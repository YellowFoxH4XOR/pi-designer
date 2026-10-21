package api

import (
	"testing"
)

func TestRenderNamesCannotEscapeTheRenderDirectory(t *testing.T) {
	r := &Runner{cfg: RunnerConfig{RenderDir: "/data/renders"}}
	if got := r.RenderFile("proj_1", "1712345-mobile.png"); got != "/data/renders/proj_1/1712345-mobile.png" {
		t.Fatalf("valid render resolved to %q", got)
	}
	for _, name := range []string{"../pidesigner.db", "1-mobile.png/../../x", "x.png", "1-tablet.png"} {
		if got := r.RenderFile("proj_1", name); got != "" {
			t.Fatalf("%q resolved to %q", name, got)
		}
	}
	if _, ok := parseRender("proj_1", []byte(`{"viewport":"web","file":"../../etc/passwd"}`)); ok {
		t.Fatal("render details with a traversal file name were accepted")
	}
	got, ok := parseRender("proj_1", []byte(`{"viewport":"mobile","path":"index.html","file":"9-mobile-full.png","fullPage":true,"overflowX":true,"errors":2}`))
	if !ok || got.Image != "/api/projects/proj_1/renders/9-mobile-full.png" || !got.OverflowX || got.Errors != 2 {
		t.Fatalf("render details parsed as %+v", got)
	}
}

func TestTodoSnapshotsNormalizeStatusAndDropEmptyItems(t *testing.T) {
	todos, ok := parseTodos([]byte(`{"todos":[{"content":"Plan","status":"completed"},{"content":" ","status":"pending"},{"content":"Build","status":"doing"}]}`))
	if !ok || len(todos) != 2 || todos[1].Status != "pending" {
		t.Fatalf("todos parsed as %+v", todos)
	}
	if _, ok := parseTodos([]byte(`{"plan":[]}`)); ok {
		t.Fatal("arguments without todos were treated as a plan")
	}
}

func TestDeckSlideRendersAreNotPageOverflow(t *testing.T) {
	got, ok := parseRender("p", []byte(`{"viewport":"web","file":"5-web-s3.png","slide":3,"overflowX":true}`))
	if !ok || got.Slide != 3 || got.OverflowX {
		t.Fatalf("slide render parsed as %+v", got)
	}
	if (&Runner{cfg: RunnerConfig{RenderDir: "/r"}}).RenderFile("p", "5-web-s3.png") == "" {
		t.Fatal("slide render file is not served")
	}
}
