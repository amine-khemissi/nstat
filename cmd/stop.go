package cmd

import (
	"fmt"
	"os"

	"github.com/amine-khemissi/nstat/config"
)

func Stop() {
	cfg := config.Default()
	pid, err := readPID(cfg.PIDFile)
	if err != nil {
		fmt.Printf("\033[93mnstat is not running\033[0m\n")
		os.Exit(0)
	}
	p, err := os.FindProcess(pid)
	if err != nil || !processAlive(pid) {
		fmt.Printf("\033[93mnstat is not running (stale pid file?)\033[0m\n")
		os.Remove(cfg.PIDFile)
		os.Exit(0)
	}
	if err := sendStop(p); err != nil {
		fmt.Printf("\033[91mcould not stop pid %d: %v\033[0m\n", pid, err)
		os.Exit(1)
	}
	fmt.Printf("\033[92mnstat stopped (pid %d)\033[0m\n", pid)
}
