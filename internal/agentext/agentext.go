// Package agentext installs πDesigner's π extension, which gives design
// runs their plan (todo_write) and render-and-inspect (render_preview) tools.
package agentext

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed pidesigner.ts
var source []byte

// FileName is the installed extension file name.
const FileName = "pidesigner-extension.ts"

// Install writes the extension into dir and returns its path. The file is
// rewritten only when its content changed, so a running π process never
// observes a partially written file from an unrelated daemon start.
func Install(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, FileName)
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, source) {
		return path, nil
	}
	tmp, err := os.CreateTemp(dir, ".pidesigner-extension-*.ts")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(source); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}
