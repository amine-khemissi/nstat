package cmd

import (
	"bufio"
	"fmt"
	"os"

	"nstat/config"
)

func Log() {
	cfg := config.Default()
	printLastLines(cfg.LogFile, 40)
}

func printLastLines(path string, n int) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("\033[93mno log file yet\033[0m\n")
		return
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:] {
		fmt.Println(l)
	}
}
