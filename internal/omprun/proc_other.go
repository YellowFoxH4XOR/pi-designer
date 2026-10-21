//go:build !unix

package omprun

import "os/exec"

// setProcessGroup is a no-op on platforms without POSIX process groups.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup kills only the direct child.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
