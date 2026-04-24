package cmd

import "os/exec"

func openHTML(path string) {
	exec.Command("open", path).Start()
}
