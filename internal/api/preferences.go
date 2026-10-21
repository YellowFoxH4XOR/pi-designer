package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type preferences struct {
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
}

// Keep preferences in the workspace, not origin-scoped localStorage: the Mac
// app binds a fresh loopback port on every launch.
func (s *Server) handlePreferences(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(filepath.Dir(s.ProjectRoot), "preferences.json")
	p := s.loadPreferences()
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, p)
		return
	}
	if err := decodeBody(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(p.Model) > 200 || strings.ContainsAny(p.Model, "\n\r\x00") {
		writeErr(w, http.StatusBadRequest, "Enter a valid Pi model pattern")
		return
	}
	switch p.Thinking {
	case "off", "minimal", "low", "medium", "high", "xhigh", "max":
	default:
		writeErr(w, http.StatusBadRequest, "Unknown thinking level")
		return
	}
	if p.Model != "" {
		if err := s.validatePreferences(r.Context(), p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	b, _ := json.Marshal(p)
	// Atomic replacement so a force-quit can't leave a partial settings file.
	temp := path + ".tmp"
	if err := os.WriteFile(temp, b, 0o600); err != nil {
		writeErr(w, 500, "Preferences could not be saved")
		return
	}
	if err := os.Rename(temp, path); err != nil {
		writeErr(w, 500, "Preferences could not be saved")
		return
	}
	writeJSON(w, http.StatusOK, p)
}
