package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/preview"
	"github.com/akki/pidesigner/internal/project"
	"github.com/akki/pidesigner/internal/versions"
)

// versionSummary is a version as the UI sees it: its file list, not blob hashes.
type versionSummary struct {
	N         int       `json:"n"`
	RunID     string    `json:"runId,omitempty"`
	MessageID string    `json:"messageId,omitempty"`
	Label     string    `json:"label,omitempty"`
	Entry     string    `json:"entry,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Changed   []string  `json:"changed"`
	Removed   []string  `json:"removed,omitempty"`
	Files     []string  `json:"files"`
}

func summarize(v versions.Version) versionSummary {
	files := make([]string, 0, len(v.Files))
	for p := range v.Files {
		files = append(files, p)
	}
	sort.Strings(files)
	return versionSummary{N: v.N, RunID: v.RunID, MessageID: v.MessageID, Label: v.Label, Entry: v.Entry, CreatedAt: v.CreatedAt, Changed: v.Changed, Removed: v.Removed, Files: files}
}

func (s *Server) versionStore(w http.ResponseWriter) (*versions.Store, bool) {
	if s.Versions == nil {
		writeErr(w, http.StatusServiceUnavailable, "Version history is unavailable")
		return nil, false
	}
	return s.Versions, true
}

func versionNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		writeErr(w, http.StatusBadRequest, "Invalid version")
		return 0, false
	}
	return n, true
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	store, ok := s.versionStore(w)
	if !ok {
		return
	}
	all, err := store.List(proj.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]versionSummary, len(all))
	for i, v := range all {
		out[i] = summarize(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": out})
}

// handleVersionPreview serves a saved version under
// /preview/{id}/@v/{n}/..., so relative links resolve inside that version.
func (s *Server) handleVersionPreview(w http.ResponseWriter, r *http.Request) {
	if s.Versions == nil {
		http.NotFound(w, r)
		return
	}
	proj, err := s.Store.GetProject(r.PathValue("id"))
	if err != nil || proj.ID != r.PathValue("id") {
		http.NotFound(w, r)
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name, ok := preview.CleanPath(r.PathValue("path"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, modTime, err := s.Versions.ReadFile(proj.ID, n, name)
	if err != nil && !strings.Contains(name, ".") {
		data, modTime, err = s.Versions.ReadFile(proj.ID, n, strings.TrimSuffix(name, "/")+"/index.html")
		name += "/index.html"
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	preview.ServeSnapshot(w, r, name, data, modTime)
}

func (s *Server) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	proj, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	n, ok := versionNumber(w, r)
	if !ok {
		return
	}
	if _, ok := s.versionStore(w); !ok {
		return
	}
	v, err := s.Runner.RestoreVersion(proj, pm, n)
	if errors.Is(err, versions.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Version not found")
		return
	}
	if errors.Is(err, ErrProjectBusy) {
		writeErr(w, http.StatusConflict, "pi is working in this project. Stop the run before restoring a version.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.Store.TouchProject(proj.ID)
	s.Hub.NotifyProjectChanged(proj.ID)
	writeJSON(w, http.StatusOK, map[string]any{"version": summarize(v)})
}

// handleDownloadVersion returns a version's files as a zip.
func (s *Server) handleDownloadVersion(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	n, ok := versionNumber(w, r)
	if !ok {
		return
	}
	store, ok := s.versionStore(w)
	if !ok {
		return
	}
	v, err := store.Get(proj.ID, n)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Version not found")
		return
	}
	paths := summarize(v).Files
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-v%d.zip"`, proj.Slug, n))
	zw := zip.NewWriter(w)
	for _, p := range paths {
		data, _, err := store.ReadFile(proj.ID, n, p)
		if err != nil {
			continue
		}
		f, err := zw.CreateHeader(&zip.FileHeader{Name: proj.Slug + "/" + p, Method: zip.Deflate, Modified: v.CreatedAt})
		if err != nil {
			return
		}
		if _, err := f.Write(data); err != nil {
			return
		}
	}
	_ = zw.Close()
}

// RestoreVersion makes the project match version n and records the result as
// a new version, so restoring is itself undoable. No run can start meanwhile.
func (r *Runner) RestoreVersion(proj model.Project, pm *project.Manager, n int) (versions.Version, error) {
	store := r.cfg.Versions
	r.mu.Lock()
	if r.active[proj.ID] != "" || r.deleting[proj.ID] || r.restoring[proj.ID] {
		r.mu.Unlock()
		return versions.Version{}, ErrProjectBusy
	}
	r.restoring[proj.ID] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.restoring, proj.ID)
		r.mu.Unlock()
	}()
	from, err := store.Get(proj.ID, n)
	if err != nil {
		return versions.Version{}, err
	}
	// Preserve manual edits made since the last agent turn before replacing
	// them. Recording only the restored result would make those edits lost.
	if _, _, err := store.Snapshot(proj.ID, pm, versions.Meta{Label: "Before restoring version " + strconv.Itoa(n), Entry: proj.EntryFile}); err != nil {
		return versions.Version{}, fmt.Errorf("save current project before restore: %w", err)
	}
	if err := store.Restore(proj.ID, n, pm); err != nil {
		return versions.Version{}, err
	}
	v, created, err := store.Snapshot(proj.ID, pm, versions.Meta{Label: fmt.Sprintf("Restored version %d", n), Entry: from.Entry})
	if err != nil {
		return versions.Version{}, err
	}
	if !created {
		return from, nil
	}
	return v, nil
}
