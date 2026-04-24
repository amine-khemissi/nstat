package cmd

import "os/exec"

func openHTML(path string) {
	exec.Command("xdg-open", path).Start()
}
