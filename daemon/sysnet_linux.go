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
