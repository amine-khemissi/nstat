//go:build !linux

package daemon

import (
	"net"
	"time"
)

// dfPing on non-Linux platforms falls back to a plain ICMP echo. The
// Don't-Fragment bit is not guaranteed here, so path-MTU detection is
// best-effort and most accurate on Linux (see mtu_linux.go). A size that
// elicits a reply is treated as passing.
func dfPing(ip net.IP, size int, timeout time.Duration) (bool, float64, error) {
	ms, err := pingWithSize(ip, size, timeout)
	if err != nil {
		return false, 0, nil
	}
	return true, ms, nil
}
