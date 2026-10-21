package api

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Server) loadPreferences() preferences {
	p := preferences{Thinking: "medium"}
	path := filepath.Join(filepath.Dir(s.ProjectRoot), "preferences.json")
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	return p
}
