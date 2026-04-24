package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

func detectDNS() string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if !strings.HasPrefix(line, "nameserver ") {
				continue
			}
			server := strings.TrimSpace(strings.TrimPrefix(line, "nameserver "))
			if server != "" && server != "127.0.0.1" {
				return server
			}
		}
	}
	return "8.8.8.8"
}

func detectGateway() (string, error) {
	// Try `route -n get default` first (most reliable on macOS)
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "gateway:") {
				gw := strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
				if net.ParseIP(gw) != nil {
					return gw, nil
				}
			}
		}
	}

	// Fallback: parse netstat -rn
	out, err = exec.Command("netstat", "-rn").Output()
	if err != nil {
		return "", fmt.Errorf("cannot detect gateway: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "default" || fields[0] == "0.0.0.0/0" {
			if net.ParseIP(fields[1]) != nil {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

// ReadKernelTCPStats returns TCP statistics. On macOS, uses netstat -s.
// Returns: retransSegs, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab
func ReadKernelTCPStats() (int64, int64, int64, int64, int64, int64, int64, int64, error) {
	out, err := exec.Command("netstat", "-s", "-p", "tcp").Output()
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}

	var retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets int64

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Parse lines like "12345 data packets (67890 bytes) retransmitted"
		if strings.Contains(line, "retransmitted") && strings.Contains(line, "packets") {
			fmt.Sscanf(line, "%d", &retrans)
		}
		if strings.Contains(line, "packets sent") {
			fmt.Sscanf(line, "%d", &outSegs)
		}
		if strings.Contains(line, "packets received") && !strings.Contains(line, "ack") {
			fmt.Sscanf(line, "%d", &inSegs)
		}
		if strings.Contains(line, "bad checksum") || strings.Contains(line, "packets with bad") {
			var v int64
			fmt.Sscanf(line, "%d", &v)
			inErrs += v
		}
		if strings.Contains(line, "connection reset") {
			fmt.Sscanf(line, "%d", &estabResets)
		}
		if strings.Contains(line, "failed connection") || strings.Contains(line, "connections dropped") {
			var v int64
			fmt.Sscanf(line, "%d", &v)
			attemptFails += v
		}
	}

	return retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, 0, nil
}
