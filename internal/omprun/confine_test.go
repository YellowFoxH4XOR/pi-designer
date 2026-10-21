package omprun

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The sandbox is enforced by the kernel, so the test runs a real shell under
// the generated profile rather than inspecting its text.
func TestConfinementLimitsTheProcessTreeToGrantedDirectories(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Seatbelt confinement is macOS-only")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Home-directory paths: the temp dir is writable by design.
	root, err := os.MkdirTemp(home, ".pidesigner-confine-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	project := filepath.Join(root, "project")
	content := filepath.Join(root, "content")
	outside := filepath.Join(root, "other-project")
	for _, d := range []string{project, content, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(content, "skill.md"), []byte("skill"), 0o644)
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644)

	profile, err := Confinement{Writable: []string{project}, Readable: []string{content}}.profile(home)
	if err != nil {
		t.Fatal(err)
	}
	sh := func(script string) error {
		cmd := exec.Command(sandboxExec, "-p", profile, "/bin/sh", "-c", script)
		cmd.Dir = project
		return cmd.Run()
	}
	if err := sh("echo ok > inside.txt && cat inside.txt"); err != nil {
		t.Fatalf("project directory not writable: %v", err)
	}
	if err := sh("cat " + filepath.Join(content, "skill.md")); err != nil {
		t.Fatalf("granted content not readable: %v", err)
	}
	if sh("cat "+filepath.Join(outside, "secret.txt")) == nil {
		t.Fatal("read outside the granted directories was allowed")
	}
	if sh("echo x > "+filepath.Join(outside, "escape.txt")) == nil {
		t.Fatal("write outside the project was allowed")
	}
	if sh("echo x > "+filepath.Join(content, "edit.md")) == nil {
		t.Fatal("read-only content was writable")
	}
}
