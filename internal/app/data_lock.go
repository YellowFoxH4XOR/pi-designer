package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockDataDir holds ownership until the daemon has drained HTTP and closed its
// store. Never unlink the file: another process may already be waiting on its inode.
func lockDataDir(dir string) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(dir, ".daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open workspace lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("workspace is already in use by another πDesigner daemon: %w", err)
	}
	return file, nil
}
