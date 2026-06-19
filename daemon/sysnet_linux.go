package daemon

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// defaultIface returns the interface carrying the default route, or "".
func defaultIface() string {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 2 && f[1] == "00000000" {
			return f[0]
		}
	}
	return ""
}

// readDHCPLease reads the current DHCP lease for the default-route interface,
// trying NetworkManager (nmcli), then systemd-networkd, then ISC dhclient lease
// files. Returns avail=false if none yield a lease, so callers degrade
// gracefully ("n/a") rather than alarming.
func readDHCPLease() (server string, expiry, leaseTime int64, avail bool) {
	iface := defaultIface()
	if iface == "" {
		return "", 0, 0, false
	}
	for _, src := range []func(string) (string, int64, int64, bool){
		leaseFromNMCLI,
		leaseFromNetworkd,
		leaseFromDhclient,
	} {
		if s, e, l, ok := src(iface); ok {
			return s, e, l, true
		}
	}
	return "", 0, 0, false
}

// leaseFromNMCLI reads the lease from NetworkManager via nmcli.
func leaseFromNMCLI(iface string) (server string, expiry, leaseTime int64, ok bool) {
	out, err := exec.Command("nmcli", "-t", "-f", "DHCP4.OPTION", "device", "show", iface).Output()
	if err != nil {
		return "", 0, 0, false
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		// Format: DHCP4.OPTION[3]:expiry = 1781964619
		c := strings.Index(line, ":")
		if c < 0 {
			continue
		}
		kv := strings.SplitN(line[c+1:], "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch key {
		case "dhcp_server_identifier":
			server = val
		case "expiry":
			expiry, _ = strconv.ParseInt(val, 10, 64)
		case "dhcp_lease_time":
			leaseTime, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	return server, expiry, leaseTime, expiry > 0
}

// leaseFromNetworkd reads the systemd-networkd lease from
// /run/systemd/netif/leases/<ifindex>. That file carries no absolute expiry, so
// it is approximated from the file's mtime (rewritten on each renewal) + LIFETIME.
func leaseFromNetworkd(iface string) (server string, expiry, leaseTime int64, ok bool) {
	idx, err := os.ReadFile("/sys/class/net/" + iface + "/ifindex")
	if err != nil {
		return "", 0, 0, false
	}
	path := "/run/systemd/netif/leases/" + strings.TrimSpace(string(idx))
	fi, err := os.Stat(path)
	if err != nil {
		return "", 0, 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, false
	}
	return parseNetworkdLease(string(data), fi.ModTime())
}

// parseNetworkdLease parses a systemd-networkd lease file (KEY=VALUE lines).
func parseNetworkdLease(content string, mtime time.Time) (server string, expiry, leaseTime int64, ok bool) {
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		kv := strings.SplitN(sc.Text(), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch key {
		case "SERVER_ADDRESS":
			server = val
		case "LIFETIME":
			leaseTime, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	if leaseTime <= 0 {
		return "", 0, 0, false
	}
	return server, mtime.Unix() + leaseTime, leaseTime, true
}

// leaseFromDhclient scans common ISC dhclient lease files and returns the most
// recent lease for the interface.
func leaseFromDhclient(iface string) (server string, expiry, leaseTime int64, ok bool) {
	patterns := []string{
		"/var/lib/dhcp/dhclient*.leases",
		"/var/lib/dhclient/dhclient*.leases",
		"/var/lib/NetworkManager/dhclient-*.lease",
	}
	for _, p := range patterns {
		files, _ := filepath.Glob(p)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if s, e, l, found := parseDhclientLeases(string(data), iface); found && e > expiry {
				server, expiry, leaseTime, ok = s, e, l, true
			}
		}
	}
	return server, expiry, leaseTime, ok
}

// parseDhclientLeases parses ISC dhclient `lease { ... }` blocks and returns the
// matching lease with the latest expiry. A block with no interface line still
// matches (single-interface lease files omit it).
func parseDhclientLeases(content, iface string) (server string, expiry, leaseTime int64, ok bool) {
	sc := bufio.NewScanner(strings.NewReader(content))
	var bIface, bServer string
	var bExpiry, bLease int64
	inBlock := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "lease {"):
			inBlock = true
			bIface, bServer, bExpiry, bLease = "", "", 0, 0
		case inBlock && line == "}":
			inBlock = false
			if (bIface == "" || bIface == iface) && bExpiry > 0 && bExpiry > expiry {
				server, expiry, leaseTime, ok = bServer, bExpiry, bLease, true
			}
		case inBlock:
			parseDhclientField(line, &bIface, &bServer, &bExpiry, &bLease)
		}
	}
	return server, expiry, leaseTime, ok
}

func parseDhclientField(line string, iface, server *string, expiry, lease *int64) {
	switch {
	case strings.HasPrefix(line, "interface "):
		*iface = trimDhclientVal(line[len("interface "):])
	case strings.HasPrefix(line, "option dhcp-server-identifier "):
		*server = trimDhclientVal(line[len("option dhcp-server-identifier "):])
	case strings.HasPrefix(line, "option dhcp-lease-time "):
		*lease, _ = strconv.ParseInt(trimDhclientVal(line[len("option dhcp-lease-time "):]), 10, 64)
	case strings.HasPrefix(line, "expire "):
		*expiry = parseDhclientTime(line[len("expire "):])
	}
}

func trimDhclientVal(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";")
	return strings.Trim(s, "\"")
}

// parseDhclientTime parses a dhclient `expire`/`renew` value of the form
// `<weekday> 2026/06/20 12:30:19;` (UTC).
func parseDhclientTime(s string) int64 {
	fields := strings.Fields(strings.TrimRight(strings.TrimSpace(s), ";"))
	if len(fields) < 3 {
		return 0
	}
	t, err := time.ParseInLocation("2006/01/02 15:04:05", fields[1]+" "+fields[2], time.UTC)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func detectDNS() string {
	for _, path := range []string{
		"/run/systemd/resolve/resolv.conf", // real upstream, not the stub
		"/etc/resolv.conf",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if !strings.HasPrefix(line, "nameserver ") {
				continue
			}
			server := strings.TrimSpace(strings.TrimPrefix(line, "nameserver "))
			if server != "" && server != "127.0.0.53" && server != "127.0.0.1" {
				return server
			}
		}
	}
	return "127.0.0.53"
}

func detectGateway() (string, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		gw, err := hexToIPv4LE(fields[2])
		if err != nil || gw == "0.0.0.0" {
			continue
		}
		return gw, nil
	}
	return "", fmt.Errorf("no default gateway in /proc/net/route")
}

func hexToIPv4LE(s string) (string, error) {
	if len(s) != 8 {
		return "", fmt.Errorf("unexpected hex length %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[3], b[2], b[1], b[0]), nil
}

// TCPTuning holds the kernel TCP timeout configuration.
type TCPTuning struct {
	SynRetries      int
	Retries2        int
	KeepaliveTime   int // seconds
	KeepaliveIntvl  int
	KeepaliveProbes int
	FinTimeout      int
}

// ReadTCPTuning reads TCP timeout settings from sysctl.
func ReadTCPTuning() (*TCPTuning, error) {
	t := &TCPTuning{}

	readInt := func(path string) int {
		data, err := os.ReadFile(path)
		if err != nil {
			return -1
		}
		var v int
		fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &v)
		return v
	}

	t.SynRetries = readInt("/proc/sys/net/ipv4/tcp_syn_retries")
	t.Retries2 = readInt("/proc/sys/net/ipv4/tcp_retries2")
	t.KeepaliveTime = readInt("/proc/sys/net/ipv4/tcp_keepalive_time")
	t.KeepaliveIntvl = readInt("/proc/sys/net/ipv4/tcp_keepalive_intvl")
	t.KeepaliveProbes = readInt("/proc/sys/net/ipv4/tcp_keepalive_probes")
	t.FinTimeout = readInt("/proc/sys/net/ipv4/tcp_fin_timeout")

	return t, nil
}

// IsFastFail returns true if TCP settings are tuned for fast failure.
func (t *TCPTuning) IsFastFail() bool {
	return t.SynRetries <= 3 && t.Retries2 <= 8 && t.KeepaliveTime <= 300
}

// ReadKernelTCPStats reads TCP statistics from /proc/net/snmp.
// Returns: retransSegs, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab
func ReadKernelTCPStats() (int64, int64, int64, int64, int64, int64, int64, int64, error) {
	data, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	var headers, values []string

	for i, line := range lines {
		if strings.HasPrefix(line, "Tcp:") {
			headers = strings.Fields(line)
			if i+1 < len(lines) {
				values = strings.Fields(lines[i+1])
			}
			break
		}
	}

	if len(headers) == 0 || len(values) == 0 || len(headers) != len(values) {
		return 0, 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("could not parse /proc/net/snmp")
	}

	// Build a map of header -> value
	stats := make(map[string]int64)
	for i, h := range headers {
		var v int64
		fmt.Sscanf(values[i], "%d", &v)
		stats[h] = v
	}

	return stats["RetransSegs"], stats["OutSegs"], stats["InSegs"],
		stats["InErrs"], stats["OutRsts"], stats["AttemptFails"],
		stats["EstabResets"], stats["CurrEstab"], nil
}
