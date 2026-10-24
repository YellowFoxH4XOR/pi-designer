package app

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
)

func TestRunsRecordArtifactVersionsThatPreviewAndRestore(t *testing.T) {
	fixture, err := filepath.Abs("../../scripts/fixtures/pi-rpc.cjs")
	if err != nil {
		t.Fatal(err)
	}
	a, err := Start(Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t), PiBin: fixture})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())
	var p struct {
		Project        model.Project `json:"project"`
		ConversationID string        `json:"conversationId"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Versions"}, &p) != 201 {
		t.Fatal("create")
	}
	base := "/api/projects/" + p.Project.ID
	var run struct {
		RunID string `json:"runId"`
	}
	if requestJSON(t, a.URL, "POST", base+"/chat", map[string]string{"prompt": "FAST_TASK", "conversationId": p.ConversationID, "model": "openai/gpt-fixture"}, &run) != 202 {
		t.Fatal("run")
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var got struct {
			Run model.Run `json:"run"`
		}
		requestJSON(t, a.URL, "GET", "/api/runs/"+run.RunID, nil, &got)
		if got.Run.Status == model.RunSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run status %s", got.Run.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	var list struct {
		Versions []struct {
			N         int      `json:"n"`
			RunID     string   `json:"runId"`
			MessageID string   `json:"messageId"`
			Files     []string `json:"files"`
			Changed   []string `json:"changed"`
		} `json:"versions"`
	}
	if requestJSON(t, a.URL, "GET", base+"/versions", nil, &list) != 200 || len(list.Versions) != 1 {
		t.Fatalf("versions = %+v", list)
	}
	v := list.Versions[0]
	if v.N != 1 || v.RunID != run.RunID || v.MessageID == "" || strings.Join(v.Changed, ",") != "index.html" {
		t.Fatalf("version 1 = %+v", v)
	}

	resp, err := http.Get(a.URL + "/preview/" + p.Project.ID + "/@v/1/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "Good work") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") || strings.Contains(resp.Header.Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatalf("version preview: %d %q %s", resp.StatusCode, resp.Header.Get("Content-Security-Policy"), body)
	}

	if requestJSON(t, a.URL, "PUT", base+"/file", map[string]string{"path": "index.html", "content": "<p>edited</p>"}, nil) != 200 {
		t.Fatal("edit")
	}
	if requestJSON(t, a.URL, "POST", base+"/versions/1/restore", nil, nil) != 200 {
		t.Fatal("restore")
	}
	var file struct {
		Content string `json:"content"`
	}
	requestJSON(t, a.URL, "GET", base+"/file?path=index.html", nil, &file)
	if !strings.Contains(file.Content, "Good work") {
		t.Fatalf("restored content = %q", file.Content)
	}
	if requestJSON(t, a.URL, "POST", base+"/versions/9/restore", nil, nil) != 404 {
		t.Fatal("unknown version restored")
	}
	resp, err = http.Get(a.URL + base + "/versions/1/download")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("download: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
