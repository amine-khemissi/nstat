package cmd

import "os/exec"

func openSVG(path string) {
	exec.Command("cmd", "/c", "start", "", path).Start()
}
