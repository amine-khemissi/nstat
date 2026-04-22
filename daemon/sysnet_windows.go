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
