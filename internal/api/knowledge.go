package api

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	if s.Knowledge == nil {
		writeErr(w, 503, "design references unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"sources": s.Knowledge.List()})
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	} // CLI/MCP callers do not send Origin.
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}

func (s *Server) handleKnowledgeCheck(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "cross-origin update refused")
		return
	}
	if s.Knowledge == nil {
		writeErr(w, 503, "design references unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	e, err := s.Knowledge.Check(ctx, r.PathValue("id"))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, e)
}
func (s *Server) handleKnowledgePreview(w http.ResponseWriter, r *http.Request) {
	if s.Knowledge == nil {
		writeErr(w, 503, "design references unavailable")
		return
	}
	doc, err := s.Knowledge.Preview(r.PathValue("id"))
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"content": doc})
}
func (s *Server) handleKnowledgeApply(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "cross-origin update refused")
		return
	}
	if s.Knowledge == nil {
		writeErr(w, 503, "design references unavailable")
		return
	}
	e, err := s.Knowledge.Apply(r.PathValue("id"))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, e)
}

func (s *Server) handleKnowledgeRollback(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "cross-origin update refused")
		return
	}
	if s.Knowledge == nil {
		writeErr(w, 503, "design references unavailable")
		return
	}
	e, err := s.Knowledge.Rollback(r.PathValue("id"))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, e)
}

// CheckForUpdates queries only the two approved text sources; it never installs them.
func (s *Server) CheckForUpdates(ctx context.Context) {
	if s.Knowledge == nil {
		return
	}
	for _, src := range s.Knowledge.Sources {
		if src.DocumentURL == "" {
			continue
		}
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		_, _ = s.Knowledge.Check(c, src.ID)
		cancel()
		if ctx.Err() != nil {
			return
		}
	}
}
