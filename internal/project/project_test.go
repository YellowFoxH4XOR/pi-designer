package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerRootOperationsAndRootDeletionGuard(t *testing.T) {
	m := New(t.TempDir())
	if err := m.Write("nested/hello.txt", "needle"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Read("nested/hello.txt")
	if err != nil || got != "needle" {
		t.Fatalf("Read() = %q, %v; want needle", got, err)
	}
	if err := m.Write("image.bin", string([]byte{0, 0xff, 1})); err != nil {
		t.Fatal(err)
	}
	binary, err := m.ReadBytes("image.bin")
	if err != nil || !bytes.Equal(binary, []byte{0, 0xff, 1}) {
		t.Fatalf("ReadBytes() = %v, %v", binary, err)
	}
	matches, err := m.Search("needle", 10)
	if err != nil || len(matches) != 1 || matches[0].Path != "nested/hello.txt" {
		t.Fatalf("Search() = %#v, %v; want nested/hello.txt", matches, err)
	}

	for _, rel := range []string{".", "/", "..", "nested/.."} {
		if err := m.Delete(rel); err == nil {
			t.Errorf("Delete(%q) unexpectedly accepted the project root", rel)
		}
	}
	if got, err := m.Read("nested/hello.txt"); err != nil || got != "needle" {
		t.Fatalf("root delete attempt changed project contents: Read() = %q, %v", got, err)
	}
	if err := m.Delete("nested/hello.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRejectsRawTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	for _, rel := range []string{"../outside.txt", "nested/../../outside.txt", "/../outside.txt"} {
		if _, err := m.Read(rel); err == nil {
			t.Errorf("Read(%q) unexpectedly accepted traversal", rel)
		}
		if err := m.Write(rel, "changed"); err == nil {
			t.Errorf("Write(%q) unexpectedly accepted traversal", rel)
		}
		if err := m.Delete(rel); err == nil {
			t.Errorf("Delete(%q) unexpectedly accepted traversal", rel)
		}
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "keep" {
		t.Fatalf("outside file changed: %q, %v", got, err)
	}
}

func TestManagerConfinesSymlinks(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "secret.txt")
	if err := os.WriteFile(externalFile, []byte("private marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	externalDirRel, err := filepath.Rel(root, external)
	if err != nil {
		t.Fatal(err)
	}
	externalFileRel, err := filepath.Rel(root, externalFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalFileRel, filepath.Join(root, "external-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDirRel, filepath.Join(root, "external-parent")); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(root, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside", "value.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside/value.txt", filepath.Join(root, "internal-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside", filepath.Join(root, "internal-parent")); err != nil {
		t.Fatal(err)
	}

	m := New(root)
	if got, err := m.Read("internal-file"); err != nil || got != "inside" {
		t.Fatalf("internal final symlink read = %q, %v", got, err)
	}
	if err := m.Write("internal-parent/new.txt", "inside too"); err != nil {
		t.Fatalf("write through internal parent symlink: %v", err)
	}

	for _, rel := range []string{"external-file", "external-parent/secret.txt"} {
		if _, err := m.ReadBytes(rel); err == nil {
			t.Errorf("ReadBytes(%q) unexpectedly followed an escaping symlink", rel)
		}
		if err := m.Write(rel, "changed"); err == nil {
			t.Errorf("Write(%q) unexpectedly followed an escaping symlink", rel)
		}
	}
	if err := m.Delete("external-parent/secret.txt"); err == nil {
		t.Fatal("Delete through an escaping parent symlink unexpectedly succeeded")
	}
	if matches, err := m.Search("private marker", 10); err != nil || len(matches) != 0 {
		t.Fatalf("Search traversed an external symlink: %#v, %v", matches, err)
	}
	if err := m.Delete("external-file"); err != nil {
		t.Fatalf("Delete final symlink: %v", err)
	}
	if got, err := os.ReadFile(externalFile); err != nil || string(got) != "private marker" {
		t.Fatalf("external target changed: %q, %v", got, err)
	}
}
