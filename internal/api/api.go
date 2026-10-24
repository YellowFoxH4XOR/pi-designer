// Package api exposes the daemon's HTTP surface: projects, files, chat runs,
// the content catalog and the live preview.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akki/pidesigner/internal/content"
	"github.com/akki/pidesigner/internal/knowledge"
	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/pirun"
	"github.com/akki/pidesigner/internal/preview"
	"github.com/akki/pidesigner/internal/project"
	"github.com/akki/pidesigner/internal/store"
	"github.com/akki/pidesigner/internal/versions"
)

// Server holds the daemon's dependencies.
type Server struct {
	Store       *store.Store
	Catalog     *content.Catalog
	Knowledge   *knowledge.Manager
	Runner      *Runner
	Hub         *preview.Hub
	ProjectRoot string
	// PiExplicit is the configured pi path; empty resolves automatically.
	PiExplicit    string
	Models        *pirun.Catalog
	Versions      *versions.Store
	activeMu      sync.RWMutex
	activeProject string
	piMu          sync.Mutex
	pi            pirun.Runtime
}

// SetPiRuntime records the detected pi installation.
func (s *Server) SetPiRuntime(rt pirun.Runtime) {
	s.piMu.Lock()
	s.pi = rt
	s.piMu.Unlock()
}

func (s *Server) piRuntime() pirun.Runtime {
	s.piMu.Lock()
	defer s.piMu.Unlock()
	return s.pi
}

// piReady reports why a run cannot start with the detected pi, or nil.
func (s *Server) piReady() error {
	rt := s.piRuntime()
	if rt.Supported {
		return nil
	}
	if rt.Error != "" {
		return fmt.Errorf("%s. Open Settings to check your pi installation", rt.Error)
	}
	return fmt.Errorf("pi was not found. Open Settings to check your pi installation")
}

// Routes builds the HTTP mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/models/thinking", s.handleThinkingLevels)
	mux.HandleFunc("GET /api/connection", s.handleConnection)
	mux.HandleFunc("GET /api/preferences", s.handlePreferences)
	mux.HandleFunc("PUT /api/preferences", s.handlePreferences)

	// Projects
	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("PUT /api/active-project", s.handleActiveProject)
	mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)

	// Files
	mux.HandleFunc("GET /api/projects/{id}/files", s.handleListFiles)
	mux.HandleFunc("GET /api/projects/{id}/file", s.handleGetFile)
	mux.HandleFunc("PUT /api/projects/{id}/file", s.handleWriteFile)
	mux.HandleFunc("DELETE /api/projects/{id}/file", s.handleDeleteFile)
	mux.HandleFunc("GET /api/projects/{id}/search", s.handleSearch)
	mux.HandleFunc("GET /api/projects/{id}/artifact", s.handleArtifact)
	mux.HandleFunc("GET /api/projects/{id}/versions", s.handleListVersions)
	mux.HandleFunc("POST /api/projects/{id}/versions/{n}/restore", s.handleRestoreVersion)
	mux.HandleFunc("GET /api/projects/{id}/versions/{n}/download", s.handleDownloadVersion)

	// Conversations
	mux.HandleFunc("GET /api/projects/{id}/conversations", s.handleListConversations)
	mux.HandleFunc("POST /api/projects/{id}/conversations", s.handleCreateConversation)
	mux.HandleFunc("GET /api/conversations/{id}/messages", s.handleListMessages)

	// Runs
	mux.HandleFunc("POST /api/projects/{id}/chat", s.handleChat)
	mux.HandleFunc("GET /api/activity", s.handleActivity)
	mux.HandleFunc("GET /api/runs/{id}/activity", s.handleRunActivity)
	mux.HandleFunc("GET /api/runs/{id}/log", s.handleRunLog)
	mux.HandleFunc("POST /api/runs/{id}/ui-response", s.handleUIResponse)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/projects/{id}/runs", s.handleRuns)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancelRun)
	mux.HandleFunc("GET /api/runs/{id}/events", s.handleRunEvents)
	mux.HandleFunc("GET /api/projects/{id}/renders/{name}", s.handleRender)

	// Catalog
	mux.HandleFunc("GET /api/skills", s.handleSkills)
	mux.HandleFunc("GET /api/skills/{id}/document", s.handleSkillDocument)
	mux.HandleFunc("GET /api/templates", s.handleTemplates)
	mux.HandleFunc("GET /api/design-systems", s.handleDesignSystems)
	mux.HandleFunc("GET /api/design-systems/{id}/context", s.handleDesignSystemContext)
	mux.HandleFunc("GET /api/design-systems/{id}/preview/{path...}", s.handleSystemPreview)
	mux.HandleFunc("GET /api/craft", s.handleCraft)
	mux.HandleFunc("GET /api/knowledge", s.handleKnowledge)
	mux.HandleFunc("POST /api/knowledge/sync", s.handleKnowledgeSync)
	mux.HandleFunc("POST /api/knowledge/{id}/check", s.handleKnowledgeCheck)
	mux.HandleFunc("GET /api/knowledge/{id}/preview", s.handleKnowledgePreview)
	mux.HandleFunc("POST /api/knowledge/{id}/apply", s.handleKnowledgeApply)
	mux.HandleFunc("POST /api/knowledge/{id}/rollback", s.handleKnowledgeRollback)
	mux.HandleFunc("POST /api/knowledge/{id}/reference", s.handleKnowledgeReference)

	// Preview
	mux.HandleFunc("GET /api/projects/{id}/watch", s.handleWatch)
	mux.HandleFunc("GET /preview/{id}", s.handlePreview)
	mux.HandleFunc("GET /preview/{id}/", s.handlePreview)
	mux.HandleFunc("GET /preview/{id}/@v/{n}/{path...}", s.handleVersionPreview)

	return logMiddleware(mux)
}

// ------------------------------------------------------------------ helpers

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasPrefix(r.URL.Path, "/preview/") {
			log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid body: %w", err)
	}
	return nil
}

// resolveProject loads a project and its file manager.
func (s *Server) resolveProject(w http.ResponseWriter, r *http.Request) (model.Project, *project.Manager, bool) {
	ref := r.PathValue("id")
	if ref == "@active" {
		s.activeMu.RLock()
		ref = s.activeProject
		s.activeMu.RUnlock()
		if ref == "" {
			writeErr(w, http.StatusConflict, "No active project. Open a project in πDesigner or pass an explicit project id or name.")
			return model.Project{}, nil, false
		}
	}
	proj, err := s.Store.GetProject(ref)
	if err != nil {
		if err == store.ErrNotFound {
			writeErr(w, http.StatusNotFound, "project not found: "+ref)
		} else {
			writeErr(w, http.StatusBadRequest, err.Error())
		}
		return model.Project{}, nil, false
	}
	return proj, project.New(proj.Dir), true
}

// ------------------------------------------------------------------ handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// "Check again" in Settings re-detects pi after the user installs or
	// upgrades it; runs resolve the binary themselves each time.
	if r.URL.Query().Get("recheck") == "1" {
		s.SetPiRuntime(pirun.Detect(r.Context(), s.PiExplicit))
	}
	rt := s.piRuntime()
	piBin := ""
	if rt.Supported {
		piBin = rt.Bin
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"piBin":         piBin,
		"pi":            rt,
		"projectRoot":   s.ProjectRoot,
		"designSystems": len(s.Catalog.DesignSystems()),
		"skills":        len(s.Catalog.Skills()),
		"templates":     len(s.Catalog.Templates()),
		"version":       preview.BridgeVersion(),
	})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.Store.ListProjects()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []model.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

type createProjectRequest struct {
	Name           string `json:"name"`
	DesignSystemID string `json:"designSystemId,omitempty"`
	SkillID        string `json:"skillId,omitempty"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.DesignSystemID != "" {
		if _, ok := s.Catalog.DesignSystem(req.DesignSystemID); !ok {
			writeErr(w, http.StatusBadRequest, "Unknown design system")
			return
		}
	}

	slug := store.Slugify(req.Name)
	if slug == "" {
		slug = "project"
	}
	// Ensure slug uniqueness. Match slugs exactly: GetProject's name and
	// substring fallbacks would reserve unrelated slugs.
	base := slug
	for i := 2; ; i++ {
		taken, err := s.Store.SlugExists(slug)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !taken {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}

	now := time.Now()
	dir := filepath.Join(s.ProjectRoot, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	p := model.Project{
		ID:             newID("proj"),
		Name:           req.Name,
		Slug:           slug,
		Dir:            dir,
		DesignSystemID: req.DesignSystemID,
		SkillID:        req.SkillID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.Store.CreateProject(p); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	conv := model.Conversation{
		ID: newID("conv"), ProjectID: p.ID, Title: "Main",
		CreatedAt: now, UpdatedAt: now,
	}
	_ = s.Store.CreateConversation(conv)

	s.Hub.Watch(p.ID, p.Dir)
	writeJSON(w, http.StatusCreated, map[string]any{"project": p, "conversationId": conv.ID})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	convs, _ := s.Store.ListConversations(proj.ID)
	writeJSON(w, http.StatusOK, map[string]any{"project": proj, "conversations": convs})
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	_, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	files, err := pm.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []model.File{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	_, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	content, err := pm.Read(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "content": content})
}

type writeFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	proj, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	var req writeFileRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := pm.Write(req.Path, req.Content); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.Store.TouchProject(proj.ID)
	s.Hub.NotifyProjectChanged(proj.ID)
	writeJSON(w, http.StatusOK, map[string]any{"path": req.Path, "ok": true})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	proj, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := pm.Delete(path); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Hub.NotifyProjectChanged(proj.ID)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": path, "ok": true})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	_, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		writeErr(w, http.StatusBadRequest, "q is required")
		return
	}
	limit := 100
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	files, err := pm.Search(q, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []model.File{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files, "query": q})
}

func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	proj, pm, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	entry := r.URL.Query().Get("entry")
	if entry == "" {
		entry = proj.EntryFile
	}
	bundle, err := BuildArtifact(pm, entry)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	convs, err := s.Store.ListConversations(proj.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if convs == nil {
		convs = []model.Conversation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": convs})
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	_ = decodeBody(r, &req) // body optional
	now := time.Now()
	c := model.Conversation{
		ID: newID("conv"), ProjectID: proj.ID, Title: req.Title,
		CreatedAt: now, UpdatedAt: now,
	}
	if c.Title == "" {
		c.Title = "New chat"
	}
	if err := s.Store.CreateConversation(c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": c})
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msgs, err := s.Store.ListMessages(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []model.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	var req RunRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.piReady(); err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	req.ProjectID = proj.ID
	defaults := s.loadPreferences()
	if req.Model == "" {
		req.Model = defaults.Model
	}
	if req.Thinking == "" {
		req.Thinking = defaults.Thinking
	}

	handle, err := s.Runner.Start(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"runId":          handle.RunID,
		"conversationId": req.ConversationID,
	})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.Store.GetRun(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run})
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	if err := s.Runner.Cancel(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
}

// handleRender serves a preview render the agent took of a project.
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	path := s.Runner.RenderFile(proj.ID, r.PathValue("name"))
	if path == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"skills": s.Catalog.Skills()})
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": s.Catalog.Templates()})
}

func (s *Server) handleDesignSystems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"designSystems": s.Catalog.DesignSystems()})
}

func (s *Server) handleDesignSystemContext(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.Catalog.DesignSystem(r.PathValue("id")); !ok {
		writeErr(w, http.StatusNotFound, "Design system not found")
		return
	}
	ds, err := s.Catalog.DesignSystemContext(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

func (s *Server) handleCraft(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"docs": s.Catalog.CraftIndex()})
}

// handleWatch upgrades to a websocket and streams file change events.
func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	proj, _, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	s.Hub.ServeWS(w, r, proj.ID)
}

// handlePreview serves a project's files for the live preview iframe.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := s.Store.GetProject(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Rewrite the URL so the file handler sees a project-relative path.
	prefix := "/preview/" + id
	rel := strings.TrimPrefix(r.URL.Path, prefix)
	if rel == "" {
		rel = "/"
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = rel
	s.Hub.FileHandler(proj.Dir).ServeHTTP(w, r2)
}
