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
