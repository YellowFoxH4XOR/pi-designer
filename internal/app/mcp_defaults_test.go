package app

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/akki/pidesigner/internal/model"
)

func TestActiveProjectSelectionSwitchClearAndExplicitIsolation(t *testing.T) {
	a, err := Start(Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())
	var one, two struct {
		Project model.Project `json:"project"`
	}
	requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "First"}, &one)
	requestJSON(t, a.URL, "POST", "/api/projects", map[string]string{"name": "Second"}, &two)
	selectProject := func(id string) int {
		return requestJSON(t, a.URL, "PUT", "/api/active-project", map[string]string{"projectId": id}, nil)
	}
	if status := requestJSON(t, a.URL, "GET", "/api/projects/@active", nil, nil); status != http.StatusConflict {
		t.Fatalf("unselected: %d", status)
	}
	for _, p := range []model.Project{one.Project, two.Project} {
		if selectProject(p.ID) != http.StatusOK {
			t.Fatal("selection failed")
		}
		var got struct {
			Project model.Project `json:"project"`
		}
		requestJSON(t, a.URL, "GET", "/api/projects/@active", nil, &got)
		if got.Project.ID != p.ID {
			t.Fatalf("wrong active project: %s", got.Project.ID)
		}
	}
	if selectProject("nonexistent") != http.StatusNotFound {
		t.Fatal("invalid selection accepted")
	}
	var got struct {
		Project model.Project `json:"project"`
	}
	requestJSON(t, a.URL, "GET", "/api/projects/@active", nil, &got)
	if got.Project.ID != two.Project.ID {
		t.Fatal("invalid selection changed active project")
	}
	requestJSON(t, a.URL, "GET", "/api/projects/"+one.Project.ID, nil, &got)
	if got.Project.ID != one.Project.ID {
		t.Fatal("explicit selection ignored")
	}
	selectProject("")
	if status := requestJSON(t, a.URL, "GET", "/api/projects/@active/files", nil, nil); status != http.StatusConflict {
		t.Fatalf("cleared: %d", status)
	}
	if status := requestJSON(t, a.URL, "GET", "/api/projects/"+one.Project.ID+"/files", nil, nil); status != http.StatusOK {
		t.Fatalf("explicit failed after clear: %d", status)
	}
}

func TestPrototypeTemplateIsDiscoverableAndReadableAsSkill(t *testing.T) {
	root := fixtureCatalog(t)
	dir := filepath.Join(root, "design-templates", "web-prototype")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	document := "---\nname: Prototype\ndescription: Build a page\n---\n# Prototype workflow\nBuild the page.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(document), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := Start(Options{DataDir: t.TempDir(), ContentDir: root})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown(context.Background())
	var list struct {
		Skills []model.Skill `json:"skills"`
	}
	requestJSON(t, a.URL, "GET", "/api/skills", nil, &list)
	found := false
	for _, sk := range list.Skills {
		if sk.ID == "web-prototype" {
			found = true
		}
	}
	if !found {
		t.Fatal("prototype workflow not discoverable")
	}
	var body struct {
		Content string `json:"content"`
	}
	if status := requestJSON(t, a.URL, "GET", "/api/skills/web-prototype/document", nil, &body); status != http.StatusOK || body.Content != document {
		t.Fatalf("workflow document unavailable: %d %q", status, body.Content)
	}
}
