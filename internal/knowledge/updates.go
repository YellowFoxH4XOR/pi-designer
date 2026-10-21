// Package knowledge manages reviewed, data-only design references outside the signed app bundle.
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Source is an allowlisted reference. Link-only resources are never copied or treated as licensed assets.
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	DocumentURL string `json:"documentUrl,omitempty"`
	Note        string `json:"note"`
	Group       string `json:"group,omitempty"`
	UseFor      string `json:"useFor,omitempty"`
	License     string `json:"license,omitempty"`
}

var Approved = []Source{
	{ID: "xias", Name: "XIAS · design QA", Group: "Design guidance", URL: "https://github.com/plugin87/ux-ui-agent-skills", DocumentURL: "https://raw.githubusercontent.com/plugin87/ux-ui-agent-skills/main/.claude/skills/design-qa/SKILL.md", Note: "One selected QA skill from XIAS; not its full 26-skill collection.", UseFor: "Review hierarchy, states and accessibility after a draft is built.", License: "MIT"},
	{ID: "material", Name: "Material Design 3", Group: "Design guidance", URL: "https://m3.material.io/", Note: "The bundled Material design system is selectable under Design systems. Official site remains browse-only.", UseFor: "Android patterns, adaptive layouts and Material components."},
	{ID: "material-color", Name: "Material Color Utilities", Group: "Design guidance", URL: "https://github.com/material-foundation/material-color-utilities", DocumentURL: "https://raw.githubusercontent.com/material-foundation/material-color-utilities/main/README.md", Note: "Official color-system documentation, not the full Material 3 guidelines.", UseFor: "Dynamic color schemes, semantic color roles and contrast.", License: "Apache-2.0"},
	{ID: "shadcn", Name: "shadcn/ui", Group: "shadcn ecosystem", URL: "https://ui.shadcn.com/docs", DocumentURL: "https://ui.shadcn.com/llms.txt", Note: "The bundled shadcn design system is selectable under Design systems. Component code is not installed.", UseFor: "Accessible product UI foundations and component selection.", License: "MIT (code)"},
	{ID: "reui", Name: "ReUI", Group: "shadcn ecosystem", URL: "https://github.com/keenthemes/reui", DocumentURL: "https://raw.githubusercontent.com/keenthemes/reui/main/README.md", Note: "Project overview, not the full component catalog or Pro blocks.", UseFor: "Dense app UI, dashboards and operational workflows.", License: "MIT (repo)"},
	{ID: "magicui", Name: "Magic UI", Group: "shadcn ecosystem", URL: "https://github.com/magicuidesign/magicui", DocumentURL: "https://raw.githubusercontent.com/magicuidesign/magicui/main/README.md", Note: "Project overview; animated components are not installed.", UseFor: "One purposeful animated moment in a marketing interface.", License: "MIT (repo)"},
	{ID: "kokonut", Name: "Kokonut UI", Group: "shadcn ecosystem", URL: "https://github.com/kokonut-labs/kokonutui", DocumentURL: "https://raw.githubusercontent.com/kokonut-labs/kokonutui/main/README.md", Note: "Project overview; animated components are not installed.", UseFor: "Modern interactive components for a React/Tailwind project.", License: "MIT (repo)"},
	{ID: "aceternity", Name: "Aceternity UI", Group: "shadcn ecosystem", URL: "https://ui.aceternity.com/", Note: "Browse free components at source. Redistribution and Pro terms differ; no mirroring.", UseFor: "Expressive landing-page interaction; check the component's terms."},
	{ID: "studio", Name: "shadcn/studio", Group: "shadcn ecosystem", URL: "https://github.com/shadcnstudio/shadcn-studio", DocumentURL: "https://raw.githubusercontent.com/shadcnstudio/shadcn-studio/main/README.md", Note: "Repo overview only; check terms for specific templates and blocks.", UseFor: "Find composable blocks and themes before making new ones.", License: "MIT (repo); check assets"},
	{ID: "figma", Name: "Figma Community kits", Group: "Figma kits", URL: "https://www.figma.com/community/ui-kits", Note: "Community files have individual authors and terms; browse, do not mirror.", UseFor: "Inspect real layout and component precedents before designing."},
	{ID: "figma-sds", Name: "Figma Simple Design System", Group: "Figma kits", URL: "https://github.com/figma/sds", DocumentURL: "https://raw.githubusercontent.com/figma/sds/main/README.md", Note: "Official open-source SDS overview, not the entire Figma Community.", UseFor: "Connect responsive components, variables and code handoff.", License: "MIT (repo)"},
	{ID: "obra", Name: "Obra shadcn/ui Community kit", Group: "Figma kits", URL: "https://www.figma.com/community/file/1514746685758799870/shadcn-ui-kit-community-edition", Note: "Free Community Edition; consult the author for file reuse terms.", UseFor: "Preview shadcn components in Figma before implementation."},
}

const maxDocument = 512 << 10

type Entry struct {
	Source
	InstalledHash string `json:"installedHash,omitempty"`
	PreviousHash  string `json:"previousHash,omitempty"`
	AvailableHash string `json:"availableHash,omitempty"`
	CheckedAt     string `json:"checkedAt,omitempty"`
	Updated       bool   `json:"updated"`
	Error         string `json:"error,omitempty"`
}

type state struct {
	Entries map[string]*Entry `json:"entries"`
}

// Manager is serialized so checking and applying cannot race or write partial snapshots.
type Manager struct {
	mu      sync.Mutex
	Dir     string
	Client  *http.Client
	Sources []Source
	data    state
}

func Open(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	m := &Manager{Dir: dir, Client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 2 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
			return errors.New("cross-origin redirect refused")
		}
		return nil
	}}, Sources: append([]Source(nil), Approved...), data: state{Entries: map[string]*Entry{}}}
	if raw, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
		if err = json.Unmarshal(raw, &m.data); err != nil {
			return nil, fmt.Errorf("invalid knowledge state: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if m.data.Entries == nil {
		m.data.Entries = map[string]*Entry{}
	}
	return m, nil
}

func (m *Manager) listLocked() []Entry {
	out := make([]Entry, 0, len(m.Sources))
	for _, src := range m.Sources {
		e := Entry{Source: src}
		if saved := m.data.Entries[src.ID]; saved != nil {
			e.InstalledHash = saved.InstalledHash
			e.PreviousHash = saved.PreviousHash
			e.AvailableHash = saved.AvailableHash
			e.CheckedAt = saved.CheckedAt
			e.Error = saved.Error
			e.Updated = e.AvailableHash != "" && e.AvailableHash != e.InstalledHash
		}
		out = append(out, e)
	}
	return out
}
func (m *Manager) List() []Entry { m.mu.Lock(); defer m.mu.Unlock(); return m.listLocked() }
func hash(b []byte) string       { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func atomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (m *Manager) saveLocked() error {
	raw, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return atomic(filepath.Join(m.Dir, "state.json"), raw)
}
func (m *Manager) source(id string) (Source, bool) {
	for _, s := range m.Sources {
		if s.ID == id {
			return s, true
		}
	}
	return Source{}, false
}
func (m *Manager) Check(ctx context.Context, id string) (Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src, ok := m.source(id)
	if !ok || src.DocumentURL == "" {
		return Entry{}, errors.New("source cannot be checked")
	}
	if !strings.HasPrefix(src.DocumentURL, "https://") {
		return Entry{}, errors.New("source URL must use HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", src.DocumentURL, nil)
	if err != nil {
		return Entry{}, err
	}
	req.Header.Set("User-Agent", "PiDesigner-Knowledge/1")
	res, err := m.Client.Do(req)
	if err != nil {
		return Entry{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Entry{}, fmt.Errorf("upstream returned %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxDocument+1))
	if err != nil {
		return Entry{}, err
	}
	if len(b) == 0 || len(b) > maxDocument {
		return Entry{}, errors.New("document empty or too large")
	}
	// READMEs may start with image markup; reject full HTML/error pages instead.
	lower := strings.ToLower(strings.TrimSpace(string(b)))
	if strings.Contains(res.Header.Get("Content-Type"), "text/html") || strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") || !strings.Contains(string(b), "# ") {
		return Entry{}, errors.New("unexpected document format")
	}
	if m.data.Entries[id] == nil {
		m.data.Entries[id] = &Entry{}
	}
	entry := m.data.Entries[id]
	entry.AvailableHash = hash(b)
	entry.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	entry.Error = ""
	// Staging remains data-only, with a fixed filename and size limit.
	if err = atomic(filepath.Join(m.Dir, id+".pending.txt"), b); err != nil {
		return Entry{}, err
	}
	if err = m.saveLocked(); err != nil {
		return Entry{}, err
	}
	for _, v := range m.listLocked() {
		if v.ID == id {
			return v, nil
		}
	}
	return Entry{}, errors.New("source disappeared")
}
func (m *Manager) Preview(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src, ok := m.source(id)
	if !ok || src.DocumentURL == "" {
		return "", errors.New("unknown source")
	}
	raw, err := os.ReadFile(filepath.Join(m.Dir, id+".pending.txt"))
	if err != nil {
		return "", errors.New("check source first")
	}
	if m.data.Entries[id] == nil || len(raw) > maxDocument || hash(raw) != m.data.Entries[id].AvailableHash {
		return "", errors.New("staged document failed verification")
	}
	return string(raw), nil
}
func (m *Manager) Apply(id string) (Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src, ok := m.source(id)
	if !ok || src.DocumentURL == "" {
		return Entry{}, errors.New("unknown source")
	}
	e := m.data.Entries[id]
	if e == nil || e.AvailableHash == "" {
		return Entry{}, errors.New("check source first")
	}
	raw, err := os.ReadFile(filepath.Join(m.Dir, id+".pending.txt"))
	if err != nil {
		return Entry{}, err
	}
	if len(raw) > maxDocument || hash(raw) != e.AvailableHash {
		return Entry{}, errors.New("staged document failed verification")
	}
	installed := filepath.Join(m.Dir, id+".md")
	if e.InstalledHash != "" && e.InstalledHash != e.AvailableHash {
		old, readErr := os.ReadFile(installed)
		if readErr != nil || hash(old) != e.InstalledHash {
			return Entry{}, errors.New("installed document failed verification")
		}
		if err = atomic(filepath.Join(m.Dir, id+".previous.md"), old); err != nil {
			return Entry{}, err
		}
		e.PreviousHash = e.InstalledHash
	}
	if err = atomic(installed, raw); err != nil {
		return Entry{}, err
	}
	e.InstalledHash = e.AvailableHash
	if err = m.saveLocked(); err != nil {
		return Entry{}, err
	}
	for _, v := range m.listLocked() {
		if v.ID == id {
			return v, nil
		}
	}
	return Entry{}, errors.New("source disappeared")
}

// Rollback restores the prior reviewed version without fetching upstream.
func (m *Manager) Rollback(id string) (Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.source(id); !ok {
		return Entry{}, errors.New("unknown source")
	}
	e := m.data.Entries[id]
	if e == nil || e.PreviousHash == "" {
		return Entry{}, errors.New("no previous version")
	}
	previous, err := os.ReadFile(filepath.Join(m.Dir, id+".previous.md"))
	if err != nil || len(previous) > maxDocument || hash(previous) != e.PreviousHash {
		return Entry{}, errors.New("previous version failed verification")
	}
	current, err := os.ReadFile(filepath.Join(m.Dir, id+".md"))
	if err != nil || hash(current) != e.InstalledHash {
		return Entry{}, errors.New("installed version failed verification")
	}
	if err = atomic(filepath.Join(m.Dir, id+".md"), previous); err != nil {
		return Entry{}, err
	}
	if err = atomic(filepath.Join(m.Dir, id+".previous.md"), current); err != nil {
		return Entry{}, err
	}
	e.InstalledHash, e.PreviousHash = e.PreviousHash, e.InstalledHash
	if err = m.saveLocked(); err != nil {
		return Entry{}, err
	}
	for _, v := range m.listLocked() {
		if v.ID == id {
			return v, nil
		}
	}
	return Entry{}, errors.New("source disappeared")
}

// Docs lists installed references by fixed filename, verifying they still match approved hashes.
func (m *Manager) Docs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, s := range m.Sources {
		e := m.data.Entries[s.ID]
		if e == nil || e.InstalledHash == "" {
			continue
		}
		name := s.ID + ".md"
		b, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err == nil && len(b) <= maxDocument && hash(b) == e.InstalledHash {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
