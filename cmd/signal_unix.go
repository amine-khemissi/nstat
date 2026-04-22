//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

func sendStop(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
