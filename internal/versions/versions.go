// Package versions keeps a numbered snapshot of a project's files after each
// agent turn, so a design can be viewed, compared and restored like an
// artifact's version history.
//
// Snapshots are content-addressed: file bodies live once under
// <root>/<project>/blobs/<sha256>, and each version is a small JSON manifest
// mapping project-relative paths to blob hashes.
package versions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/akki/pidesigner/internal/project"
)

// Limits keep a runaway project (a vendored dependency tree, video exports)
// from filling the disk with snapshots.
const (
	maxFileBytes  = 20 << 20
	maxTotalBytes = 200 << 20
)

// ErrNotFound reports an unknown version.
var ErrNotFound = errors.New("version not found")

// Version is one snapshot's metadata. Files maps paths to blob hashes.
type Version struct {
	N         int               `json:"n"`
	RunID     string            `json:"runId,omitempty"`
	MessageID string            `json:"messageId,omitempty"`
	Label     string            `json:"label,omitempty"`
	Entry     string            `json:"entry,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	Changed   []string          `json:"changed"`
	Removed   []string          `json:"removed,omitempty"`
	Files     map[string]string `json:"files"`
}

// Meta describes what produced a snapshot.
type Meta struct {
	RunID, MessageID, Label, Entry string
}

// Store owns every project's version history beneath Root.
type Store struct {
	Root string
	mu   sync.Mutex
}

var projectID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (s *Store) dir(id string) (string, error) {
	if !projectID.MatchString(id) {
		return "", fmt.Errorf("invalid project id %q", id)
	}
	return filepath.Join(s.Root, id), nil
}

// Snapshot records the project's current files as a new version, unless they
// are identical to the latest version. created reports whether one was added.
func (s *Store) Snapshot(id string, pm *project.Manager, meta Meta) (v Version, created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.dir(id)
	if err != nil {
		return Version{}, false, err
	}
	files, err := pm.List()
	if err != nil {
		return Version{}, false, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o700); err != nil {
		return Version{}, false, err
	}
	manifest := map[string]string{}
	var total int64
	for _, f := range files {
		if f.Size > maxFileBytes {
			continue
		}
		if total += f.Size; total > maxTotalBytes {
			return Version{}, false, fmt.Errorf("project is larger than %d MB; version not saved", maxTotalBytes>>20)
		}
		b, err := pm.ReadBytes(f.Path)
		if err != nil {
			continue // removed mid-walk
		}
		sum := sha256.Sum256(b)
		h := hex.EncodeToString(sum[:])
		blob := filepath.Join(dir, "blobs", h)
		if _, err := os.Stat(blob); errors.Is(err, os.ErrNotExist) {
			if err := writeAtomic(blob, b); err != nil {
				return Version{}, false, err
			}
		}
		manifest[f.Path] = h
	}
	all, err := s.listLocked(dir)
	if err != nil {
		return Version{}, false, err
	}
	previous := map[string]string{}
	if len(all) > 0 {
		previous = all[len(all)-1].Files
	}
	v = Version{N: 1, RunID: meta.RunID, MessageID: meta.MessageID, Label: meta.Label, Entry: meta.Entry, CreatedAt: time.Now().UTC(), Files: manifest, Changed: []string{}}
	if len(all) > 0 {
		v.N = all[len(all)-1].N + 1
	}
	for p, h := range manifest {
		if previous[p] != h {
			v.Changed = append(v.Changed, p)
		}
	}
	for p := range previous {
		if _, ok := manifest[p]; !ok {
			v.Removed = append(v.Removed, p)
		}
	}
	sort.Strings(v.Changed)
	sort.Strings(v.Removed)
	if len(v.Changed) == 0 && len(v.Removed) == 0 {
		if len(all) > 0 {
			return all[len(all)-1], false, nil
		}
		if len(manifest) == 0 {
			return Version{}, false, nil
		}
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return Version{}, false, err
	}
	if err := writeAtomic(filepath.Join(dir, strconv.Itoa(v.N)+".json"), raw); err != nil {
		return Version{}, false, err
	}
	return v, true, nil
}

// List returns a project's versions, oldest first.
func (s *Store) List(id string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.dir(id)
	if err != nil {
		return nil, err
	}
	return s.listLocked(dir)
}

func (s *Store) listLocked(dir string) ([]Version, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Version{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Version{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}
		if _, err := strconv.Atoi(name[:len(name)-5]); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var v Version
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("version %s: %w", name, err)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out, nil
}

// Get returns one version.
func (s *Store) Get(id string, n int) (Version, error) {
	all, err := s.List(id)
	if err != nil {
		return Version{}, err
	}
	for _, v := range all {
		if v.N == n {
			return v, nil
		}
	}
	return Version{}, ErrNotFound
}

// ReadFile returns one file as it was in version n.
func (s *Store) ReadFile(id string, n int, path string) ([]byte, time.Time, error) {
	v, err := s.Get(id, n)
	if err != nil {
		return nil, time.Time{}, err
	}
	h, ok := v.Files[path]
	if !ok {
		return nil, time.Time{}, os.ErrNotExist
	}
	dir, err := s.dir(id)
	if err != nil {
		return nil, time.Time{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "blobs", h))
	return b, v.CreatedAt, err
}

// Restore makes the project's files match version n: it rewrites changed
// files and removes files the version did not have. Hidden agent state
// skipped by project listings (.pi, .git, node_modules) is left alone.
func (s *Store) Restore(id string, n int, pm *project.Manager) error {
	v, err := s.Get(id, n)
	if err != nil {
		return err
	}
	dir, err := s.dir(id)
	if err != nil {
		return err
	}
	current, err := pm.List()
	if err != nil {
		return err
	}
	for _, f := range current {
		if _, keep := v.Files[f.Path]; !keep && f.Size <= maxFileBytes {
			if err := pm.Delete(f.Path); err != nil {
				return err
			}
		}
	}
	for p, h := range v.Files {
		b, err := os.ReadFile(filepath.Join(dir, "blobs", h))
		if err != nil {
			return fmt.Errorf("version %d is missing %s: %w", n, p, err)
		}
		if err := pm.Write(p, string(b)); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a project's whole history.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.dir(id)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
