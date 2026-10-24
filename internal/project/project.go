// Package project manages project directories on disk: file listing, reads,
// writes, deletes and search.
//
// Every path that arrives from a client is resolved against the project root
// and rejected if it escapes it.
package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/akki/pidesigner/internal/model"
)

// Manager operates on a single project directory.
type Manager struct {
	root string
}

// New returns a Manager for the given project root.
func New(root string) *Manager { return &Manager{root: root} }

// Root returns the project directory.
func (m *Manager) Root() string { return m.root }

// EnsureDir creates the project directory.
func (m *Manager) EnsureDir() error { return os.MkdirAll(m.root, 0o755) }

// Resolve turns a client-supplied relative path into an absolute path inside
// the project, or an error if it would escape.
func (m *Manager) Resolve(rel string) (string, error) {
	clean, err := rootedPath(rel)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(m.root, filepath.FromSlash(clean))
	relCheck, err := filepath.Rel(m.root, abs)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the project directory", rel)
	}
	return abs, nil
}

// rootedPath cleans a project-relative path without allowing leading ".."
// components to be normalized away. Leading slashes remain root-relative for
// compatibility with Resolve's historical behavior.
func rootedPath(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	input := rel
	if filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("path %q escapes the project directory", input)
	}
	rel = strings.TrimLeft(filepath.ToSlash(rel), "/")
	depth := 0
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case "", ".":
		case "..":
			if depth == 0 {
				return "", fmt.Errorf("path %q escapes the project directory", input)
			}
			depth--
		default:
			depth++
		}
	}
	return path.Clean(rel), nil
}

// The configured project root is trusted; OpenRoot establishes confinement
// beneath it, including when following links below the root.
func (m *Manager) openRoot() (*os.Root, error) {
	return os.OpenRoot(m.root)
}

// Rel converts an absolute path back to a project-relative slash path.
func (m *Manager) Rel(abs string) string {
	rel, err := filepath.Rel(m.root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// List walks the project tree and returns file metadata.
func (m *Manager) List() ([]model.File, error) {
	root, err := m.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	var out []model.File
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than failing the whole listing
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".pi", ".DS_Store", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		out = append(out, model.File{
			Name: d.Name(),
			Path: rel,
			Size: info.Size(),
			Kind: kindOf(rel),
			Mime: mimeOf(rel),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Read returns a file's contents.
func (m *Manager) Read(rel string) (string, error) {
	b, err := m.ReadBytes(rel)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadBytes returns a file's contents without converting binary data to text.
func (m *Manager) ReadBytes(rel string) ([]byte, error) {
	name, err := rootedPath(rel)
	if err != nil {
		return nil, err
	}
	root, err := m.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(name)
}

// Write creates or overwrites a file, creating parent directories as needed.
func (m *Manager) Write(rel, content string) error {
	name, err := rootedPath(rel)
	if err != nil {
		return err
	}
	root, err := m.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return err
	}
	return root.WriteFile(name, []byte(content), 0o644)
}

// Delete removes a file or directory, but never the project root itself.
func (m *Manager) Delete(rel string) error {
	name, err := rootedPath(rel)
	if err != nil {
		return err
	}
	if name == "." {
		return fmt.Errorf("cannot delete the project root")
	}
	root, err := m.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(name)
}

// Search returns files whose contents match the query (case-insensitive).
// When query looks like a regex, it is used as one; otherwise it is a literal.
func (m *Manager) Search(query string, limit int) ([]model.File, error) {
	if limit <= 0 {
		limit = 100
	}
	var re *regexp.Regexp
	if rx, err := regexp.Compile("(?i)" + query); err == nil {
		re = rx
	}
	files, err := m.List()
	if err != nil {
		return nil, err
	}
	root, err := m.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()

	var out []model.File
	for _, f := range files {
		if len(out) >= limit {
			break
		}
		if f.Kind == "code" || f.Kind == "text" || f.Kind == "markdown" {
			b, err := root.ReadFile(f.Path)
			if err != nil {
				continue
			}
			hit := false
			if re != nil {
				hit = re.Match(b)
			} else {
				hit = strings.Contains(strings.ToLower(string(b)), strings.ToLower(query))
			}
			if hit {
				out = append(out, f)
			}
		}
	}
	return out, nil
}

// FindEntryFile picks the artifact entry point for a project: an explicit
// entry file if set, else index.html, else the first HTML file found.
func (m *Manager) FindEntryFile() string {
	files, err := m.List()
	if err != nil {
		return ""
	}
	var fallback string
	for _, f := range files {
		p := f.Path
		if p == "index.html" {
			return p
		}
		if fallback == "" && strings.HasSuffix(p, ".html") && !strings.Contains(p, "/") {
			fallback = p
		}
	}
	return fallback
}

// kindOf buckets a path into a coarse file kind used by the UI.
func kindOf(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".html", ".htm":
		return "code"
	case ".css":
		return "code"
	case ".js", ".mjs", ".jsx", ".ts", ".tsx":
		return "code"
	case ".json":
		return "code"
	case ".md", ".txt":
		return "markdown"
	case ".svg":
		return "image"
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return "image"
	case ".woff", ".woff2", ".ttf", ".otf":
		return "font"
	case ".mp4", ".webm", ".mov":
		return "video"
	case ".mp3", ".wav":
		return "audio"
	default:
		return "file"
	}
}

func mimeOf(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}
