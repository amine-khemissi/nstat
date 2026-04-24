package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/amine-khemissi/nstat/dim"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// pingOnce sends a single ICMP echo to target and returns the RTT in ms.
// Tries unprivileged UDP-ICMP first (works on most Linux without root),
// falls back to raw ICMP (needs CAP_NET_RAW).
func pingOnce(target string, timeout time.Duration) (float64, error) {
	ip, err := resolveIP(target)
	if err != nil {
		return 0, err
	}

	network := "udp4"
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		network = "ip4:icmp"
		conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			return 0, fmt.Errorf("icmp listen: %w (needs elevated privileges; on Linux try: sysctl net.ipv4.ping_group_range)", err)
		}
	}
	defer conn.Close()

	var dst net.Addr
	if network == "udp4" {
		dst = &net.UDPAddr{IP: ip}
	} else {
		dst = &net.IPAddr{IP: ip}
	}

	id := os.Getpid() & 0xffff
	seq := int(time.Now().UnixNano() & 0xffff)

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("nstat"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	if _, err = conn.WriteTo(wb, dst); err != nil {
		return 0, err
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	rb := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(rb)
		if err != nil {
			return 0, err
		}
		rm, err := icmp.ParseMessage(1, rb[:n])
		if err != nil {
			continue
		}
		if rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		// For unprivileged udp4 sockets on Linux the kernel replaces the ICMP
		// ID with its own and filters replies per-socket, so we only check seq.
		// For raw ip4:icmp sockets we keep the ID we set.
		if echo, ok := rm.Body.(*icmp.Echo); ok {
			if network == "ip4:icmp" && echo.ID != id {
				continue
			}
			if echo.Seq == seq {
				return float64(time.Since(start).Microseconds()) / 1000.0, nil
			}
		}
	}
}

// tcpCheck measures the time to complete a TCP handshake in ms.
func tcpCheck(host string, port int, timeout time.Duration) (float64, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, nil
}

// tcpCheckWithReason measures TCP handshake time and categorizes failures.
func tcpCheckWithReason(host string, port int, timeout time.Duration) (float64, dim.TCPFailReason, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		reason := classifyTCPError(err)
		return 0, reason, err
	}
	conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, dim.TCPFailNone, nil
}

// classifyTCPError determines the failure reason from a TCP dial error.
func classifyTCPError(err error) dim.TCPFailReason {
	if err == nil {
		return dim.TCPFailNone
	}

	errStr := err.Error()

	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return dim.TCPFailTimeout
	}
	if strings.Contains(errStr, "i/o timeout") || strings.Contains(errStr, "deadline exceeded") {
		return dim.TCPFailTimeout
	}

	// Check for connection refused (RST)
	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if sysErr.Err == syscall.ECONNREFUSED {
				return dim.TCPFailRefused
			}
			if sysErr.Err == syscall.ECONNRESET {
				return dim.TCPFailReset
			}
		}
	}
	if strings.Contains(errStr, "connection refused") {
		return dim.TCPFailRefused
	}
	if strings.Contains(errStr, "connection reset") {
		return dim.TCPFailReset
	}

	// Check for DNS failure
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "lookup") {
		return dim.TCPFailDNS
	}

	return dim.TCPFailOther
}

// MTUSizes defines the packet sizes to test for MTU discovery.
var MTUSizes = []int{1500, 1472, 1400, 1300, 1200, 576}

// mtuProbe sends ICMP echo with Don't Fragment flag to detect MTU issues.
// Returns the largest size that succeeded and the latency.
func mtuProbe(target string, timeout time.Duration) (int, float64, error) {
	ip, err := resolveIP(target)
	if err != nil {
		return 0, 0, err
	}

	// Try each size from largest to smallest
	for _, size := range MTUSizes {
		ms, err := pingWithSize(ip, size, timeout)
		if err == nil {
			return size, ms, nil
		}
	}

	return 0, 0, fmt.Errorf("all MTU sizes failed")
}

// pingWithSize sends an ICMP echo with a specific payload size.
func pingWithSize(ip net.IP, size int, timeout time.Duration) (float64, error) {
	network := "udp4"
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		network = "ip4:icmp"
		conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			return 0, fmt.Errorf("icmp listen: %w", err)
		}
	}
	defer conn.Close()

	var dst net.Addr
	if network == "udp4" {
		dst = &net.UDPAddr{IP: ip}
	} else {
		dst = &net.IPAddr{IP: ip}
	}

	id := os.Getpid() & 0xffff
	seq := int(time.Now().UnixNano() & 0xffff)

	// Calculate payload size: total size - IP header (20) - ICMP header (8)
	payloadSize := size - 28
	if payloadSize < 0 {
		payloadSize = 0
	}
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: payload,
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	if _, err = conn.WriteTo(wb, dst); err != nil {
		return 0, err
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	rb := make([]byte, 2000)
	for {
		n, _, err := conn.ReadFrom(rb)
		if err != nil {
			return 0, err
		}
		rm, err := icmp.ParseMessage(1, rb[:n])
		if err != nil {
			continue
		}
		if rm.Type == ipv4.ICMPTypeDestinationUnreachable {
			// Fragmentation needed but DF set
			return 0, fmt.Errorf("fragmentation needed for size %d", size)
		}
		if rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		if echo, ok := rm.Body.(*icmp.Echo); ok {
			if network == "ip4:icmp" && echo.ID != id {
				continue
			}
			if echo.Seq == seq {
				return float64(time.Since(start).Microseconds()) / 1000.0, nil
			}
		}
	}
}

// dnsCheck measures the time to resolve google.com using the given server in ms.
func dnsCheck(server string, timeout time.Duration) (float64, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	if _, err := r.LookupHost(ctx, "google.com"); err != nil {
		return 0, err
	}
	return float64(time.Since(start).Microseconds()) / 1000.0, nil
}

func resolveIP(host string) (net.IP, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.To4(), nil
	}
	addrs, err := net.LookupHost(host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("cannot resolve %s", host)
	}
	return net.ParseIP(addrs[0]).To4(), nil
}
