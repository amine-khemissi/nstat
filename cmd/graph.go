package cmd

import (
	"fmt"
	"os"
	"strconv"

	"nstat/config"
	"nstat/graph"
)

func Graph(args []string) {
	var hours float64
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--hours":
			if i+1 >= len(args) {
				fatal("--hours requires a value")
			}
			var err error
			hours, err = strconv.ParseFloat(args[i+1], 64)
			if err != nil || hours < 0 {
				fatal("--hours must be a non-negative number")
			}
			i++
		default:
			fatal("unknown option: " + args[i])
		}
	}

	cfg := config.Default()
	panels := graph.BuildPanels(graph.DefaultPanels, cfg.Dir, hours)

	title := "nstat · Network Monitor"
	if hours > 0 {
		title += fmt.Sprintf(" — last %.0fh", hours)
	}
	opts := graph.Options{Title: title}

	if err := graph.Generate(panels, opts, cfg.GraphFile); err != nil {
		fmt.Fprintf(os.Stderr, "graph: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\033[92mgraph saved: \033[1m%s\033[0m\n", cfg.GraphFile)

	openSVG(cfg.GraphFile)
}
