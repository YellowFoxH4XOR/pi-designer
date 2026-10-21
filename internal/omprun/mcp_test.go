package omprun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectMCPPolicyKeepsRemoteDefinitionsAndDisablesLocal(t *testing.T) {
	agent := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agent)
	user := []byte(`{"mcpServers":{"local":{"command":"npx"},"remote":{"type":"http","url":"https://example.com/mcp"}}}`)
	if err := os.WriteFile(filepath.Join(agent, "mcp.json"), user, 0600); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".omp"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, projectMCPFile)
	remote := `{"type":"sse","url":"https://project.example/sse","headers":{"Authorization":"test-header"}}`
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"project-remote":`+remote+`,"project-local":{"command":"node"}},"disabledServers":["unrelated"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := WriteLocalMCPPolicy(project); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			MCPServers      map[string]json.RawMessage `json:"mcpServers"`
			DisabledServers []string                   `json:"disabledServers"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			t.Fatal(err)
		}
		var entry map[string]any
		for _, name := range []string{"local", "project-local"} {
			if err := json.Unmarshal(cfg.MCPServers[name], &entry); err != nil {
				t.Fatal(err)
			}
			if entry["enabled"] != false || entry["command"] != "/usr/bin/false" {
				t.Fatalf("local server remains executable: %s", cfg.MCPServers[name])
			}
		}
		var got, want any
		json.Unmarshal(cfg.MCPServers["project-remote"], &got)
		json.Unmarshal([]byte(remote), &want)
		a, _ := json.Marshal(got)
		z, _ := json.Marshal(want)
		if string(a) != string(z) {
			t.Fatal("remote endpoint or headers changed")
		}
		if len(cfg.DisabledServers) != 1 || cfg.DisabledServers[0] != "unrelated" {
			t.Fatal("project settings lost")
		}
	}
	b, err := os.ReadFile(filepath.Join(agent, "mcp.json"))
	if err != nil || string(b) != string(user) {
		t.Fatal("user MCP config changed")
	}
}
