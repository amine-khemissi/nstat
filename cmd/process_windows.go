package cmd

import "syscall"

// isAlive checks whether a process with the given PID is still running.
// On Windows we open a handle and check GetExitCodeProcess — exit code 259
// (STILL_ACTIVE) means the process has not yet terminated.
func isAlive(pid int) bool {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == 259 // STILL_ACTIVE
}
