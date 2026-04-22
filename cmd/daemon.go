package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/amine-khemissi/nstat/config"
	"github.com/amine-khemissi/nstat/daemon"
)

// Daemon is the hidden entry point called by `nstat start` in the background.
func Daemon(args []string) {
	cfg := config.Default()
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			cfg.PingInterval = time.Duration(n) * time.Second
		}
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			cfg.RTTWindow = n
		}
	}
	daemon.Run(cfg)
}

// --- shared helpers ---------------------------------------------------------

func readPID(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscan(string(b), &pid)
	return pid, err
}

func processAlive(pid int) bool {
	return isAlive(pid)
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "\033[91m%s\033[0m\n", msg)
	os.Exit(1)
}
