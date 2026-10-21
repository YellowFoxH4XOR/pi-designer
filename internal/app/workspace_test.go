package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureCatalog(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"design-systems/test/manifest.json":   `{"id":"test","name":"Test system","category":"Test","files":{"design":"DESIGN.md","tokens":"tokens.css"}}`,
		"design-systems/test/DESIGN.md":       "# Test system\nA review fixture.",
		"design-systems/test/tokens.css":      ":root { --ink: #123456; }",
		"design-systems/test/components.html": `<html><body>Component fixture</body></html>`,
		"skills/test/SKILL.md":                "---\nname: test\ndescription: Test workflow\n---\n# Workflow\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func requestJSON(t *testing.T, base, method, path string, input, output any) int {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if output != nil && json.NewDecoder(resp.Body).Decode(output) != nil {
		t.Fatal("invalid JSON response", path)
	}
	return resp.StatusCode
}

func TestWorkspaceCatalogPreferencesAndNativeDiscovery(t *testing.T) {
	opts := Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t)}
	a, err := Start(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	var discovery struct {
		URL string `json:"url"`
	}
	b, err := os.ReadFile(filepath.Join(opts.DataDir, "runtime.json"))
	if err != nil || json.Unmarshal(b, &discovery) != nil || discovery.URL != a.URL {
		t.Fatalf("discovery handshake: %s %v", b, err)
	}

	var config struct {
		Config struct {
			Servers map[string]struct {
				Args []string `json:"args"`
			} `json:"mcpServers"`
		} `json:"config"`
	}
	if requestJSON(t, a.URL, "GET", "/api/connection", nil, &config) != 200 {
		t.Fatal("connection config failed")
	}
	args := config.Config.Servers["pidesigner"].Args
	if len(args) != 2 || args[0] != "--discovery" || args[1] != filepath.Join(opts.DataDir, "runtime.json") {
		t.Fatalf("MCP must follow native port: %v", args)
	}

	preferences := map[string]string{"model": "", "thinking": "high"}
	if requestJSON(t, a.URL, "PUT", "/api/preferences", preferences, nil) != 200 {
		t.Fatal("save preferences")
	}
	var saved map[string]string
	if requestJSON(t, a.URL, "GET", "/api/preferences", nil, &saved) != 200 || saved["thinking"] != "high" || saved["model"] != "" {
		t.Fatalf("preferences not retained: %v", saved)
	}
	if requestJSON(t, a.URL, "PUT", "/api/preferences", map[string]string{"thinking": "unknown"}, nil) != 400 {
		t.Fatal("invalid thinking level accepted")
	}
	var doc map[string]string
	if requestJSON(t, a.URL, "GET", "/api/skills/test/document", nil, &doc) != 200 || !strings.Contains(doc["content"], "Workflow") {
		t.Fatal("skill resource is unreadable")
	}
	if requestJSON(t, a.URL, "GET", "/api/design-systems/missing/context", nil, nil) != 404 {
		t.Fatal("unknown catalog id accepted")
	}
	resp, err := http.Get(a.URL + "/api/design-systems/test/preview/components.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatal("component preview not served as HTML")
	}

	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(opts.DataDir, "runtime.json")); !os.IsNotExist(err) {
		t.Fatal("discovery was not removed on shutdown")
	}
	a, err = Start(opts)
	if err != nil {
		t.Fatal(err)
	}
	requestJSON(t, a.URL, "GET", "/api/preferences", nil, &saved)
	if saved["thinking"] != "high" {
		t.Fatal("preferences lost across daemon restart")
	}
}

func TestProjectControlsPersistWithoutRunningAnAgent(t *testing.T) {
	a, err := Start(Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Original", "designSystemId": "test"}, &created) != 201 {
		t.Fatal("create project")
	}
	path := "/api/projects/" + created.Project.ID
	var updated struct {
		Project struct {
			Name string `json:"name"`
		} `json:"project"`
	}
	if requestJSON(t, a.URL, "PATCH", path, map[string]string{"name": "Renamed", "designSystemId": "test"}, &updated) != 200 || updated.Project.Name != "Renamed" {
		t.Fatal("rename did not persist")
	}
	if requestJSON(t, a.URL, "PATCH", path, map[string]string{"name": "Name", "designSystemId": "not-a-system"}, nil) != 400 {
		t.Fatal("invalid design system accepted")
	}
	var runs struct {
		Runs []any `json:"runs"`
	}
	if requestJSON(t, a.URL, "GET", path+"/runs", nil, &runs) != 200 || runs.Runs == nil || len(runs.Runs) != 0 {
		t.Fatal("empty run list must be an array")
	}
}
