package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/akki/pidesigner/internal/model"
)

// ErrProjectBusy rejects deleting a project while omp is working in it: the
// agent would keep writing files and activity for a project that is gone.
var ErrProjectBusy = errors.New("omp is working in this project. Stop the run before deleting it.")

// DeleteProject removes a project and everything πDesigner keeps for it: the
// database rows (conversations, messages, runs and their activity cascade),
// the project files, its preview renders and each conversation's omp session.
// No run can start in the project while it is being deleted.
func (r *Runner) DeleteProject(proj model.Project) error {
	r.mu.Lock()
	if r.active[proj.ID] != "" {
		r.mu.Unlock()
		return ErrProjectBusy
	}
	if r.deleting[proj.ID] {
		r.mu.Unlock()
		return errors.New("this project is already being deleted")
	}
	r.deleting[proj.ID] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.deleting, proj.ID)
		r.mu.Unlock()
	}()

	// Keep the row (and its unique slug) until every directory is removed.
	// A concurrent create must not reuse the path while RemoveAll is running.
	convs, err := r.store.ListConversations(proj.ID)
	if err != nil {
		return err
	}

	dirs := []string{proj.Dir}
	if r.cfg.RenderDir != "" {
		dirs = append(dirs, filepath.Join(r.cfg.RenderDir, proj.ID))
	}
	if r.cfg.SessionDir != "" {
		for _, c := range convs {
			dirs = append(dirs, filepath.Join(r.cfg.SessionDir, c.ID))
		}
	}
	var errs []error
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("delete project files (project retained for retry): %w", err)
	}
	return r.store.DeleteProject(proj.ID)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	// Other routes accept names and unique substrings; a destructive call
	// must name exactly one project.
	if proj.ID != r.PathValue("id") {
		writeErr(w, http.StatusBadRequest, "Deleting a project requires its exact id ("+proj.ID+").")
		return
	}
	// The directory comes from the database; never recursively remove a path
	// outside the workspace's project root.
	if !insideDir(s.ProjectRoot, proj.Dir) {
		writeErr(w, http.StatusInternalServerError, "The project directory is outside the workspace; nothing was deleted.")
		return
	}
	if err := s.Runner.DeleteProject(proj); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrProjectBusy) {
			status = http.StatusConflict
		}
		writeErr(w, status, err.Error())
		return
	}
	s.forgetProject(proj.ID)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": proj.ID})
}

// forgetProject drops daemon state that refers to a deleted project.
func (s *Server) forgetProject(id string) {
	s.Hub.Unwatch(id)
	s.activeMu.Lock()
	if s.activeProject == id {
		s.activeProject = ""
	}
	s.activeMu.Unlock()
}

// insideDir reports whether path is strictly below root.
func insideDir(root, path string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
