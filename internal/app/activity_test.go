package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/store"
)

func TestQuitDrainsActivityBeforeClosingTheDatabase(t *testing.T) {
	fixture, err := filepath.Abs("../../scripts/fixtures/omp-rpc.cjs")
	if err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	a, err := Start(Options{DataDir: data, ContentDir: fixtureCatalog(t), OmpBin: fixture})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())
	var p struct {
		Project        model.Project `json:"project"`
		ConversationID string        `json:"conversationId"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Shutdown fixture"}, &p) != 201 {
		t.Fatal("create")
	}
	var run struct {
		RunID string `json:"runId"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects/"+p.Project.ID+"/chat", map[string]string{"prompt": "LONG_TASK", "conversationId": p.ConversationID, "model": "openai/gpt-fixture"}, &run) != 202 {
		t.Fatal("run")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		activity, err := a.Store.RunActivity(run.RunID)
		if err == nil && len(activity.Tools) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(filepath.Join(data, "pidesigner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	r, err := restored.GetRun(run.RunID)
	if err != nil || r.Status != "canceled" {
		t.Fatalf("quit did not persist cancellation: %+v %v", r, err)
	}
	activity, err := restored.RunActivity(run.RunID)
	if err != nil || activity.Status != "canceled" {
		t.Fatalf("journal disagrees after quit: %+v %v", activity, err)
	}
}
