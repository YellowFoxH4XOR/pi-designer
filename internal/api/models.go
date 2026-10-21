package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/akki/pidesigner/internal/pirun"
)

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.Models.List(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "Could not read models from your installed Pi: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
}
func (s *Server) handleThinkingLevels(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("model")
	levels, err := s.Models.Thinking(r.Context(), ref)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"levels": levels})
}
func (s *Server) validatePreferences(ctx context.Context, p preferences) error {
	if err := pirun.ValidateRef(p.Model); err != nil {
		return err
	}
	levels, err := s.Models.Thinking(ctx, p.Model)
	if err != nil {
		return err
	}
	if !pirun.ContainsLevel(levels, p.Thinking) {
		return fmt.Errorf("Thinking level %s is not supported by the selected Pi model", p.Thinking)
	}
	return nil
}
