package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/akki/pidesigner/internal/model"
)

// Catalog documents and fixtures let the UI browse the actual bundled brand
// packages, rather than displaying made-up swatches or placeholder thumbnails.
func (s *Server) handleSystemPreview(w http.ResponseWriter, r *http.Request) {
	ds, ok := s.Catalog.DesignSystem(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(filepath.Join(s.Catalog.Root, "design-systems", ds.Slug))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	path := r.PathValue("path")
	if path == "" {
		path = "components.html"
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeFileFS(w, r, root.FS(), path)
}

func (s *Server) handleSkillDocument(w http.ResponseWriter, r *http.Request) {
	sk, ok := s.Catalog.Skill(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "Skill not found")
		return
	}
	b, err := os.ReadFile(filepath.Join(sk.Path, "SKILL.md"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "Skill document is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": string(b)})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	p, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	runs, err := s.Store.ListRuns(p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []model.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	exe, _ := os.Executable()
	bin := filepath.Join(filepath.Dir(exe), "pidesigner-mcp")
	_, err := os.Stat(bin)
	writeJSON(w, http.StatusOK, map[string]any{
		"available": err == nil,
		"config": map[string]any{
			"mcpServers": map[string]any{
				"pidesigner": map[string]any{
					"command": bin,
					"args":    []string{"--discovery", filepath.Join(filepath.Dir(s.ProjectRoot), "runtime.json")},
				},
			},
		},
	})
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	p, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Name           string `json:"name"`
		DesignSystemID string `json:"designSystemId"`
		SkillID        string `json:"skillId"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "Project name cannot be empty")
		return
	}
	if req.DesignSystemID != "" {
		if _, ok := s.Catalog.DesignSystem(req.DesignSystemID); !ok {
			writeErr(w, http.StatusBadRequest, "Unknown design system")
			return
		}
	}
	p.Name, p.DesignSystemID, p.SkillID = strings.TrimSpace(req.Name), req.DesignSystemID, req.SkillID
	if err := s.Store.UpdateProject(p); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p})
}
