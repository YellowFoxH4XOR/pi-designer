package versions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/project"
)

// restoreProject permits deterministic write/delete failure tests without
// weakening the rooted filesystem boundary used by the real project manager.
type restoreProject interface {
	Root() string
	List() ([]model.File, error)
	ReadBytes(string) ([]byte, error)
	Write(string, string) error
	Delete(string) error
}

// Restore validates and stages the entire version before touching the project.
// A disk backup supports rollback on failure; if rollback itself fails, it is
// retained and its location is returned in the error. Hidden agent state and
// oversized files omitted by snapshots are left alone.
func (s *Store) Restore(id string, n int, pm *project.Manager) error {
	return s.restore(id, n, pm)
}

func (s *Store) restore(id string, n int, pm restoreProject) error {
	v, err := s.Get(id, n)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.dir(id)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(pm.Root())
	if err != nil {
		return err
	}
	defer root.Close()

	paths := make([]string, 0, len(v.Files))
	missingDirs := map[string]bool{}
	for p := range v.Files {
		if !safeRestorePath(p) {
			return fmt.Errorf("version %d has unsafe path %q", n, p)
		}
		// Refuse symlink aliases and file/directory conflicts before mutation.
		// In particular, a restore must not overwrite an excluded directory.
		for part := p; part != "."; part = path.Dir(part) {
			if part != p {
				if _, conflict := v.Files[part]; conflict {
					return fmt.Errorf("version %d has conflicting paths %q and %q", n, part, p)
				}
			}
			info, err := root.Lstat(part)
			if errors.Is(err, os.ErrNotExist) {
				if part != p {
					missingDirs[part] = true
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect %s before restore: %w", part, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || (part == p && !info.Mode().IsRegular()) || (part != p && !info.IsDir()) {
				return fmt.Errorf("cannot safely restore over %s", part)
			}
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)

	stage, err := os.MkdirTemp(dir, ".restore-")
	if err != nil {
		return err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(stage)
		}
	}()
	for _, name := range []string{"before", "after"} {
		if err := os.Mkdir(filepath.Join(stage, name), 0o700); err != nil {
			return err
		}
	}
	before := project.New(filepath.Join(stage, "before"))
	after := project.New(filepath.Join(stage, "after"))
	var total int
	for _, p := range paths {
		h := v.Files[p]
		if len(h) != sha256.Size*2 || strings.Trim(h, "0123456789abcdef") != "" {
			return fmt.Errorf("version %d has invalid blob for %s", n, p)
		}
		b, err := os.ReadFile(filepath.Join(dir, "blobs", h))
		if err != nil {
			return fmt.Errorf("version %d is missing %s: %w", n, p, err)
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != h {
			return fmt.Errorf("version %d has corrupt blob for %s", n, p)
		}
		total += len(b)
		if len(b) > maxFileBytes || total > maxTotalBytes {
			return fmt.Errorf("version %d exceeds restore size limits", n)
		}
		if err := after.Write(p, string(b)); err != nil {
			return fmt.Errorf("stage %s: %w", p, err)
		}
	}

	current, err := pm.List()
	if err != nil {
		return err
	}
	backedUp := map[string]bool{}
	backupModes := map[string]os.FileMode{}
	var backupBytes int
	for _, f := range current {
		if f.Size > maxFileBytes {
			if _, overwrite := v.Files[f.Path]; overwrite {
				return fmt.Errorf("cannot back up oversized file %s before restore", f.Path)
			}
			continue
		}
		info, err := root.Lstat(f.Path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("cannot safely back up %s before restore", f.Path)
		}
		b, err := pm.ReadBytes(f.Path)
		if err != nil {
			return fmt.Errorf("back up %s before restore: %w", f.Path, err)
		}
		backupBytes += len(b)
		if len(b) > maxFileBytes || backupBytes > maxTotalBytes {
			return fmt.Errorf("project exceeds restore backup size limits")
		}
		if err := before.Write(f.Path, string(b)); err != nil {
			return fmt.Errorf("back up %s before restore: %w", f.Path, err)
		}
		backedUp[f.Path] = true
		backupModes[f.Path] = info.Mode().Perm()
	}
	// Listings may omit unreadable entries. Never treat such a file as new.
	for _, p := range paths {
		if _, err := root.Lstat(p); err == nil && !backedUp[p] {
			return fmt.Errorf("%s was not backed up before restore", p)
		}
	}
	metadata, err := json.MarshalIndent(struct {
		Project string                 `json:"project"`
		Version int                    `json:"version"`
		Files   map[string]bool        `json:"files"`
		Modes   map[string]os.FileMode `json:"modes"`
	}{pm.Root(), n, backedUp, backupModes}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(stage, "recovery.json"), metadata); err != nil {
		return err
	}

	var touched []string
	rollback := func(cause error) error {
		var failures []error
		for i := len(touched) - 1; i >= 0; i-- {
			p := touched[i]
			if backedUp[p] {
				b, err := before.ReadBytes(p)
				if err == nil {
					err = pm.Write(p, string(b))
				}
				if err == nil {
					err = root.Chmod(p, backupModes[p])
				}
				if err != nil {
					failures = append(failures, fmt.Errorf("roll back %s: %w", p, err))
				}
			} else if err := pm.Delete(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove restored %s: %w", p, err))
			}
		}
		// Remove only directories created by this attempt, never existing ones.
		dirs := make([]string, 0, len(missingDirs))
		for p := range missingDirs {
			dirs = append(dirs, p)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
		for _, p := range dirs {
			if err := root.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove restored directory %s: %w", p, err))
			}
		}
		if len(failures) > 0 {
			keepBackup = true
			return fmt.Errorf("restore failed; recovery backup retained at %s: %w", stage, errors.Join(append([]error{cause}, failures...)...))
		}
		return fmt.Errorf("restore failed; project files rolled back: %w", cause)
	}
	for _, p := range paths {
		b, err := after.ReadBytes(p)
		if err != nil {
			return rollback(err)
		}
		// Include a failed write: it may already have truncated the old file.
		touched = append(touched, p)
		if err := pm.Write(p, string(b)); err != nil {
			return rollback(err)
		}
	}
	for _, f := range current {
		if _, keep := v.Files[f.Path]; !keep && backedUp[f.Path] {
			touched = append(touched, f.Path)
			if err := pm.Delete(f.Path); err != nil {
				return rollback(err)
			}
		}
	}
	return nil
}

func safeRestorePath(p string) bool {
	if p == "." || !filepath.IsLocal(p) || path.Clean(p) != p || strings.Contains(p, "\\") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		switch part {
		case ".git", ".pi", "node_modules", ".DS_Store", "__pycache__":
			return false
		}
	}
	return true
}
