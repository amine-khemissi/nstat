package cmd

import "os/exec"

func openSVG(path string) {
	exec.Command("xdg-open", path).Start()
}
