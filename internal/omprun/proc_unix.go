//go:build unix

package omprun

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so the entire tree
// can be signalled at once.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup signals the child's whole process group.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return
	}
	// Negative pid == "signal the process group".
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
