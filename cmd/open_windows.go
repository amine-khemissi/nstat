package cmd

import "os/exec"

func openHTML(path string) {
	exec.Command("cmd", "/c", "start", "", path).Start()
}
