//go:build unix

package pirun

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so the entire tree
// can be signalled at once.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup signals the child's whole process group. setProcessGroup
// makes the child its group leader, so the group id is its pid; that stays
// valid after pi itself has exited and lets leftover descendants (servers or
// watchers a tool started in the background) be reaped too.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Negative pid == "signal the process group".
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
