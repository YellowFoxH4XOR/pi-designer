package omprun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// πDesigner suppresses command/stdio entries from omp's native user MCP
// configuration and the project's .omp/mcp.json before startup. HTTP/SSE
// definitions remain available. This does not filter additional sources
// imported internally by omp (third-party tool configs or extensions).
//
// omp has no per-process transport filter, but a project-level
// .omp/mcp.json entry overrides a same-named user entry.

// projectMCPFile is omp's project-level MCP config, relative to the cwd.
var projectMCPFile = filepath.Join(".omp", "mcp.json")

// userMCPFile is the user's omp MCP config: the file `/mcp list` reports as
// [user]. PI_CODING_AGENT_DIR relocates omp's agent directory.
func userMCPFile() (string, error) {
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return filepath.Join(dir, "mcp.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".omp", "agent", "mcp.json"), nil
}

// LocalMCPServers names the enabled stdio servers in the user's omp MCP
// config. It reads the file instead of asking omp, because any omp session
// (even `/mcp list`) launches every configured server first.
func LocalMCPServers() ([]string, error) {
	path, err := userMCPFile()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names, err := parseLocalMCP(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return names, nil
}

// parseLocalMCP applies omp's transport inference: an explicit type wins,
// otherwise a command entry is stdio and a URL entry is http.
func parseLocalMCP(b []byte) ([]string, error) {
	var cfg struct {
		MCPServers map[string]struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Enabled *bool  `json:"enabled"`
		} `json:"mcpServers"`
		DisabledServers []string `json:"disabledServers"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	disabled := map[string]bool{}
	for _, name := range cfg.DisabledServers {
		disabled[name] = true
	}
	names := []string{}
	for name, s := range cfg.MCPServers {
		local := s.Type == "stdio" || (s.Type == "" && s.Command != "")
		if local && !disabled[name] && (s.Enabled == nil || *s.Enabled) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// WriteLocalMCPPolicy writes dir/.omp/mcp.json so an omp session started in
// dir launches none of the user's local MCP servers. The file is owned by
// πDesigner and rewritten before every session.
func WriteLocalMCPPolicy(dir string) error {
	local, err := LocalMCPServers()
	if err != nil {
		return err
	}
	servers := map[string]any{}
	for _, name := range local {
		servers[name] = map[string]any{"type": "stdio", "command": "/usr/bin/false", "enabled": false}
	}
	path := filepath.Join(dir, projectMCPFile)
	// Preserve project-specific remote servers and settings. Never replace a
	// project's remote definitions with the daemon's local-server denylist.
	config := map[string]json.RawMessage{}
	projectServers := map[string]json.RawMessage{}
	if current, readErr := os.ReadFile(path); readErr == nil {
		if err := json.Unmarshal(current, &config); err != nil {
			return fmt.Errorf("read project MCP config: %w", err)
		}
		if raw, ok := config["mcpServers"]; ok {
			if err := json.Unmarshal(raw, &projectServers); err != nil {
				return fmt.Errorf("read project MCP servers: %w", err)
			}
		}
		for name, raw := range projectServers {
			var entry struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			}
			if err := json.Unmarshal(raw, &entry); err != nil {
				return fmt.Errorf("read project MCP server %s: %w", name, err)
			}
			if entry.Type == "stdio" || entry.Command != "" {
				servers[name] = map[string]any{"type": "stdio", "command": "/usr/bin/false", "enabled": false}
			} else {
				servers[name] = raw
			}
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	config["mcpServers"] = encodedServers
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mcp-*.json")
	if err != nil {
		return err
	}
	if _, err = tmp.Write(append(b, '\n')); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
