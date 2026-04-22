package cmd

import "syscall"

func isAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
