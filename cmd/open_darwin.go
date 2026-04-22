package cmd

import "os/exec"

func openSVG(path string) {
	exec.Command("open", path).Start()
}
