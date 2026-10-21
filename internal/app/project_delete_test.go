package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteProjectRemovesFilesRendersSessionsAndHistory(t *testing.T) {
	data := t.TempDir()
	a, err := Start(Options{DataDir: data, ContentDir: fixtureCatalog(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())

	var created struct {
		Project struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"project"`
		ConversationID string `json:"conversationId"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Doomed landing"}, &created) != 201 {
		t.Fatal("create project")
	}
	var keep struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Kept"}, &keep) != 201 {
		t.Fatal("create second project")
	}
	id := created.Project.ID
	path := "/api/projects/" + id
	if requestJSON(t, a.URL, "PUT", path+"/file", map[string]string{"path": "index.html", "content": "<h1>hi</h1>"}, nil) != 200 {
		t.Fatal("write file")
	}
	renders := filepath.Join(data, "renders", id)
	session := filepath.Join(data, "sessions", created.ConversationID)
	for _, f := range []string{filepath.Join(renders, "1-web.png"), filepath.Join(session, "session.jsonl")} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if requestJSON(t, a.URL, "PUT", "/api/active-project", map[string]string{"projectId": id}, nil) != 200 {
		t.Fatal("select project")
	}

	// Names resolve elsewhere, but deletion must not act on a fuzzy match.
	if requestJSON(t, a.URL, "DELETE", "/api/projects/doomed", nil, nil) != 400 {
		t.Fatal("deleted a project by partial name")
	}
	if requestJSON(t, a.URL, "DELETE", path, nil, nil) != 200 {
		t.Fatal("delete project")
	}

	for _, dir := range []string{filepath.Join(data, "projects", created.Project.Slug), renders, session} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s survived deletion: %v", dir, err)
		}
	}
	if requestJSON(t, a.URL, "GET", path, nil, nil) != 404 {
		t.Fatal("project still resolvable")
	}
	if convs, err := a.Store.ListConversations(id); err != nil || len(convs) != 0 {
		t.Fatalf("conversations survived: %v %v", convs, err)
	}
	if requestJSON(t, a.URL, "GET", "/api/projects/@active/files", nil, nil) != 409 {
		t.Fatal("deleted project stayed active for MCP defaults")
	}
	if requestJSON(t, a.URL, "GET", "/api/projects/"+keep.Project.ID, nil, nil) != 200 {
		t.Fatal("deleting one project affected another")
	}
}
