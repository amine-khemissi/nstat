package main

import (
	"fmt"
	"os"

	"github.com/amine-khemissi/nstat/cmd"
	"github.com/amine-khemissi/nstat/version"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nstat {start [--interval N] [--window N]|stop|status|log|graph [--hours N]|-h|-v}")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		cmd.Start(os.Args[2:])
	case "stop":
		cmd.Stop()
	case "status":
		// Check for -l flag
		if len(os.Args) > 2 && (os.Args[2] == "-l" || os.Args[2] == "--lan") {
			cmd.Status()
			cmd.RunLANDiag()
		} else {
			cmd.Status()
		}
	case "log":
		cmd.Log()
	case "graph":
		cmd.Graph(os.Args[2:])
	case "-h", "--help", "help":
		cmd.Help()
	case "-v", "--version", "version":
		fmt.Printf("nstat %s\n", version.String())
	case "_daemon":
		cmd.Daemon(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "\033[91munknown command: %s\033[0m\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: nstat {start [--interval N] [--window N]|stop|status|log|graph [--hours N]|-h|-v}")
		os.Exit(1)
	}
}
