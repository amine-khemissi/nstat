package cmd

import "syscall"

func isAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
