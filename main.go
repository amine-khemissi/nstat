package main

import (
	"fmt"
	"os"

	"github.com/amine-khemissi/nstat/cmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nstat {start [--interval N] [--window N]|stop|status|log|graph [--hours N]|-h}")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		cmd.Start(os.Args[2:])
	case "stop":
		cmd.Stop()
	case "status":
		cmd.Status()
	case "log":
		cmd.Log()
	case "graph":
		cmd.Graph(os.Args[2:])
	case "-h", "--help", "help":
		cmd.Help()
	case "_daemon":
		cmd.Daemon(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "\033[91munknown command: %s\033[0m\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: nstat {start [--interval N] [--window N]|stop|status|log|graph [--hours N]|-h}")
		os.Exit(1)
	}
}
