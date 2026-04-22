//go:build windows

package cmd

import "os"

func sendStop(p *os.Process) error {
	return p.Kill()
}
