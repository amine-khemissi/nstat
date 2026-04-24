package daemon

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

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
	SynRetries    int
	Retries2      int
	KeepaliveTime int // seconds
	KeepaliveIntvl int
	KeepaliveProbes int
	FinTimeout    int
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
