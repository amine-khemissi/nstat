//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
