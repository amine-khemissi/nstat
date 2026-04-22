package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"nstat/config"
)

func Start(args []string) {
	cfg := config.Default()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interval":
			if i+1 >= len(args) {
				fatal("--interval requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fatal("--interval must be a positive integer")
			}
			cfg.PingInterval = time.Duration(n) * time.Second
			i++
		case "--window":
			if i+1 >= len(args) {
				fatal("--window requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fatal("--window must be a positive integer")
			}
			cfg.RTTWindow = n
			i++
		default:
			fatal("unknown option: " + args[i])
		}
	}

	// check if already running
	if pid, err := readPID(cfg.PIDFile); err == nil {
		if processAlive(pid) {
			fmt.Printf("\033[93mnstat already running (pid %d)\033[0m\n", pid)
			os.Exit(1)
		}
	}

	exe, err := os.Executable()
	if err != nil {
		fatal("cannot find executable: " + err.Error())
	}

	cmd := exec.Command(exe, "_daemon",
		strconv.Itoa(int(cfg.PingInterval.Seconds())),
		strconv.Itoa(cfg.RTTWindow),
	)
	setSysProcAttr(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fatal("failed to start daemon: " + err.Error())
	}
	cmd.Process.Release()

	// wait up to 2s for PID file
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if pid, err := readPID(cfg.PIDFile); err == nil && processAlive(pid) {
			fmt.Printf("\033[92mnstat started — pid %d\033[0m\n", pid)
			fmt.Printf("  interval: %ds  window: %d pings\n",
				int(cfg.PingInterval.Seconds()), cfg.RTTWindow)
			fmt.Printf("  log:      %s\n", cfg.LogFile)
			fmt.Printf("  status:   nstat status\n")
			return
		}
	}

	fmt.Printf("\033[91mfailed to start — check %s\033[0m\n", cfg.LogFile)
	os.Exit(1)
}
