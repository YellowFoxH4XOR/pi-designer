package versions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akki/pidesigner/internal/project"
)

func restoreFixture(t *testing.T) (*Store, *project.Manager, Version) {
	t.Helper()
	s := &Store{Root: t.TempDir()}
	pm := project.New(t.TempDir())
	if err := pm.Write("index.html", "old page"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Write("a/new.html", "new file in old version"); err != nil {
		t.Fatal(err)
	}
	v, _, err := s.Snapshot("proj", pm, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := pm.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Write("index.html", "current page"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Write("extra.txt", "current extra"); err != nil {
		t.Fatal(err)
	}
	return s, pm, v
}

func assertCurrentFiles(t *testing.T, pm *project.Manager) {
	t.Helper()
	for p, want := range map[string]string{"index.html": "current page", "extra.txt": "current extra"} {
		got, err := pm.Read(p)
		if err != nil || got != want {
			t.Fatalf("%s = %q, %v; want %q", p, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(pm.Root(), "a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restore left new directory behind: %v", err)
	}
}

func TestRestoreRejectsMissingOrCorruptBlobsWithoutMutation(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "corrupt"}[corrupt], func(t *testing.T) {
			s, pm, v := restoreFixture(t)
			blob := filepath.Join(s.Root, "proj", "blobs", v.Files["index.html"])
			var err error
			if corrupt {
				err = os.WriteFile(blob, []byte("damaged"), 0o600)
			} else {
				err = os.Remove(blob)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Restore("proj", 1, pm); err == nil {
				t.Fatal("invalid version restored")
			}
			assertCurrentFiles(t, pm)
		})
	}
}

type failingRestoreProject struct {
	*project.Manager
	writes       int
	failWrite    int
	failDelete   bool
	failRollback bool
}

func (pm *failingRestoreProject) Write(p, content string) error {
	pm.writes++
	if pm.writes == pm.failWrite || (pm.failRollback && pm.writes > pm.failWrite) {
		// Model a partially completed write, including truncation of a file.
		_ = pm.Manager.Write(p, "partial")
		return errors.New("injected write failure")
	}
	return pm.Manager.Write(p, content)
}

func (pm *failingRestoreProject) Delete(p string) error {
	if pm.failDelete {
		pm.failDelete = false
		_ = pm.Manager.Delete(p)
		return errors.New("injected delete failure")
	}
	return pm.Manager.Delete(p)
}

func TestRestoreRollsBackPartialWritesAndDeletes(t *testing.T) {
	for _, failDelete := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "delete"}[failDelete], func(t *testing.T) {
			s, pm, _ := restoreFixture(t)
			if err := os.Chmod(filepath.Join(pm.Root(), "extra.txt"), 0o755); err != nil {
				t.Fatal(err)
			}
			wrapped := &failingRestoreProject{Manager: pm, failDelete: failDelete}
			if !failDelete {
				wrapped.failWrite = 2 // The new nested file was already written.
			}
			if err := s.restore("proj", 1, wrapped); err == nil || !strings.Contains(err.Error(), "rolled back") {
				t.Fatalf("restore error = %v", err)
			}
			assertCurrentFiles(t, pm)
			info, err := os.Stat(filepath.Join(pm.Root(), "extra.txt"))
			if err != nil || info.Mode().Perm() != 0o755 {
				t.Fatalf("rollback lost original permissions: %v, %v", info, err)
			}
			left, err := filepath.Glob(filepath.Join(s.Root, "proj", ".restore-*"))
			if err != nil || len(left) != 0 {
				t.Fatalf("temporary backup not cleaned up: %v, %v", left, err)
			}
		})
	}
}

func TestRestoreRetainsRecoveryBackupWhenRollbackFails(t *testing.T) {
	s, pm, _ := restoreFixture(t)
	err := s.restore("proj", 1, &failingRestoreProject{Manager: pm, failWrite: 2, failRollback: true})
	if err == nil || !strings.Contains(err.Error(), "recovery backup retained at") {
		t.Fatalf("restore error = %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(s.Root, "proj", ".restore-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	b, err := os.ReadFile(filepath.Join(backups[0], "before", "index.html"))
	if err != nil || string(b) != "current page" {
		t.Fatalf("recovery content = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(backups[0], "recovery.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRefusesSymlinkTargetsWithoutMutation(t *testing.T) {
	s, pm, _ := restoreFixture(t)
	if err := os.Remove(filepath.Join(pm.Root(), "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("extra.txt", filepath.Join(pm.Root(), "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore("proj", 1, pm); err == nil {
		t.Fatal("restore followed a symlink")
	}
	if b, err := pm.Read("extra.txt"); err != nil || b != "current extra" {
		t.Fatalf("symlink target changed: %q, %v", b, err)
	}
}

func TestRestorePreservesExcludedStateAndOversizedFiles(t *testing.T) {
	s, pm, _ := restoreFixture(t)
	for _, p := range []string{".pi/settings.json", ".git/config", "node_modules/package/index.js"} {
		if err := pm.Write(p, "excluded state"); err != nil {
			t.Fatal(err)
		}
	}
	large, err := os.Create(filepath.Join(pm.Root(), "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	err = large.Truncate(maxFileBytes + 1)
	large.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Restore("proj", 1, pm); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".pi/settings.json", ".git/config", "node_modules/package/index.js"} {
		if b, err := pm.Read(p); err != nil || b != "excluded state" {
			t.Fatalf("excluded file %s changed: %q, %v", p, b, err)
		}
	}
	info, err := os.Stat(filepath.Join(pm.Root(), "large.bin"))
	if err != nil || info.Size() != maxFileBytes+1 {
		t.Fatalf("oversized file changed: %v, %v", info, err)
	}
}

func TestRestoreRejectsDirectoryConflictsBeforeMutation(t *testing.T) {
	s, pm, _ := restoreFixture(t)
	if err := pm.Delete("index.html"); err != nil {
		t.Fatal(err)
	}
	if err := pm.Write("index.html/notes.txt", "keep these notes"); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore("proj", 1, pm); err == nil {
		t.Fatal("directory conflict accepted")
	}
	if b, err := pm.Read("index.html/notes.txt"); err != nil || b != "keep these notes" {
		t.Fatalf("notes changed: %q, %v", b, err)
	}
	if b, err := pm.Read("extra.txt"); err != nil || b != "current extra" {
		t.Fatalf("extra file changed: %q, %v", b, err)
	}
}

func TestSafeRestorePath(t *testing.T) {
	for _, p := range []string{"", ".", "../escape", "/index.html", "a/../b", "a\\b", ".pi/settings.json", "a/node_modules/x", ".git/config"} {
		if safeRestorePath(p) {
			t.Errorf("unsafe path accepted: %q", p)
		}
	}
	if !safeRestorePath("pages/home.html") {
		t.Fatal("valid nested path rejected")
	}
}
