//go:build linux

package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

// dfPing sends ICMP echoes of the given total IPv4 packet size with the
// Don't-Fragment bit set, retrying a few times to ride out transient ICMP loss.
//
// Returns ok=true with the RTT (ms) if the packet reaches the target
// unfragmented. A packet too large for the path — EMSGSIZE on send (local
// interface MTU) or an ICMP fragmentation-needed reply (a downstream hop) —
// returns ok=false with a nil error. Only unexpected failures (e.g. the kernel
// refusing to open an unprivileged ICMP socket) return a non-nil error.
func dfPing(ip net.IP, size int, timeout time.Duration) (bool, float64, error) {
	const tries = 3
	for i := 0; i < tries; i++ {
		pass, ms, definitive, err := dfPingOnce(ip, size, timeout)
		if err != nil {
			return false, 0, err
		}
		if pass {
			return true, ms, nil
		}
		if definitive {
			return false, 0, nil
		}
		// transient (read timeout): retry
	}
	return false, 0, nil
}

// dfPingOnce performs a single DF-set echo. definitive is true when the result
// is conclusive (a reply arrived, or the path provably can't carry the size);
// it is false for a bare read timeout, which the caller may retry.
func dfPingOnce(ip net.IP, size int, timeout time.Duration) (pass bool, ms float64, definitive bool, err error) {
	// Unprivileged ICMP datagram socket (SOCK_DGRAM/IPPROTO_ICMP); the kernel
	// rewrites the echo ID to the socket's port, so we match replies by Seq.
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_ICMP)
	if err != nil {
		return false, 0, false, fmt.Errorf("icmp socket (try: sysctl net.ipv4.ping_group_range): %w", err)
	}
	// Force the kernel to set DF and never fragment locally.
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO); err != nil {
		unix.Close(fd)
		return false, 0, false, fmt.Errorf("set DF (IP_MTU_DISCOVER): %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{}); err != nil {
		unix.Close(fd)
		return false, 0, false, fmt.Errorf("bind icmp socket: %w", err)
	}
	f := os.NewFile(uintptr(fd), "icmp-df")
	defer f.Close()
	conn, err := net.FilePacketConn(f)
	if err != nil {
		return false, 0, false, fmt.Errorf("wrap icmp socket: %w", err)
	}
	defer conn.Close()

	payloadSize := size - 28 // IPv4 header (20) + ICMP header (8)
	if payloadSize < 0 {
		payloadSize = 0
	}
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	seq := int(time.Now().UnixNano() & 0xffff)
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: os.Getpid() & 0xffff, Seq: seq, Data: payload},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return false, 0, false, err
	}

	start := time.Now()
	if _, err := conn.WriteTo(wb, &net.UDPAddr{IP: ip}); err != nil {
		if errors.Is(err, syscall.EMSGSIZE) {
			// Too big for the local path MTU with DF set: conclusive failure.
			return false, 0, true, nil
		}
		return false, 0, false, fmt.Errorf("send: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	rb := make([]byte, 2000)
	for {
		n, _, rerr := conn.ReadFrom(rb)
		if rerr != nil {
			return false, 0, false, nil // timeout: transient, allow retry
		}
		rm, perr := icmp.ParseMessage(1, rb[:n])
		if perr != nil {
			continue
		}
		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			if echo, ok := rm.Body.(*icmp.Echo); ok && echo.Seq == seq {
				return true, float64(time.Since(start).Microseconds()) / 1000.0, true, nil
			}
		case ipv4.ICMPTypeDestinationUnreachable:
			// Fragmentation needed but DF set (downstream hop): conclusive.
			return false, 0, true, nil
		}
		// Any other message: keep reading until the deadline.
	}
}
