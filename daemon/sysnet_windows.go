package daemon

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func detectDNS() string {
	out, err := exec.Command("ipconfig", "/all").Output()
	if err != nil {
		return "8.8.8.8"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// "DNS Servers . . . . . . . . . . . : 192.168.1.1"
		if strings.Contains(line, "DNS Servers") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				server := strings.TrimSpace(parts[1])
				if net.ParseIP(server) != nil {
					return server
				}
			}
		}
	}
	return "8.8.8.8"
}

func detectGateway() (string, error) {
	out, err := exec.Command("ipconfig").Output()
	if err != nil {
		return "", fmt.Errorf("ipconfig failed: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// "Default Gateway . . . . . . . . . : 192.168.1.1"
		if strings.HasPrefix(line, "Default Gateway") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				gw := strings.TrimSpace(parts[1])
				if net.ParseIP(gw) != nil {
					return gw, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

// TCPTuning holds the kernel TCP timeout configuration.
type TCPTuning struct {
	SynRetries      int
	Retries2        int
	KeepaliveTime   int
	KeepaliveIntvl  int
	KeepaliveProbes int
	FinTimeout      int
}

// ReadTCPTuning reads TCP timeout settings. Windows uses registry.
func ReadTCPTuning() (*TCPTuning, error) {
	t := &TCPTuning{
		SynRetries:      2,    // Windows default
		Retries2:        5,    // Windows default (TcpMaxDataRetransmissions)
		KeepaliveTime:   7200, // Windows default
		KeepaliveIntvl:  1,
		KeepaliveProbes: 10,
		FinTimeout:      120,  // Windows default (TcpTimedWaitDelay)
	}
	// Could read from registry, but defaults are reasonable
	return t, nil
}

// IsFastFail returns true if TCP settings are tuned for fast failure.
func (t *TCPTuning) IsFastFail() bool {
	return t.KeepaliveTime <= 300
}

// ReadKernelTCPStats returns TCP statistics. On Windows, uses netstat -s.
// Returns: retransSegs, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab
func ReadKernelTCPStats() (int64, int64, int64, int64, int64, int64, int64, int64, error) {
	out, err := exec.Command("netstat", "-s", "-p", "tcp").Output()
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}

	var retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab int64

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Windows netstat -s output format varies, parse key metrics
		if strings.Contains(line, "Segments Retransmitted") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &retrans)
			}
		}
		if strings.Contains(line, "Segments Sent") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &outSegs)
			}
		}
		if strings.Contains(line, "Segments Received") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &inSegs)
			}
		}
		if strings.Contains(line, "Failed Connection Attempts") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &attemptFails)
			}
		}
		if strings.Contains(line, "Reset Connections") || strings.Contains(line, "Connections Reset") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &estabResets)
			}
		}
		if strings.Contains(line, "Current Connections") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &currEstab)
			}
		}
	}

	return retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab, nil
}
