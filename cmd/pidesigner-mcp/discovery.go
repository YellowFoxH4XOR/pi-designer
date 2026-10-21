package main

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
)

// Resolve for every request: the native app can restart on a different port
// without requiring the editor to restart its MCP process.
func (s *server) baseURL() string {
	if s.discovery == "" {
		return s.daemon
	}
	b, err := os.ReadFile(s.discovery)
	if err != nil {
		return "http://127.0.0.1:0"
	}
	var v struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(b, &v) != nil {
		return "http://127.0.0.1:0"
	}
	u, err := url.Parse(v.URL)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		return "http://127.0.0.1:0"
	}
	return strings.TrimRight(v.URL, "/")
}
