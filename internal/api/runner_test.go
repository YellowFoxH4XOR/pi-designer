package api

import (
	"sync"
	"testing"
)

func TestBrokerReplaysEarlyEventsAndClosesSafely(t *testing.T) {
	b := newBroker()
	b.publish(AgentEvent{Type: "text", Text: "Before subscription"})
	ch := b.subscribe()
	if event := <-ch; event.Text != "Before subscription" {
		t.Fatalf("lost initial stream: %+v", event)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); b.close() }()
	go func() { defer wg.Done(); b.unsubscribe(ch) }()
	wg.Wait()
	if _, open := <-ch; open {
		t.Fatal("subscription remained open")
	}
	// Late reconnects get the bounded history, then EOF, not an orphan channel.
	late := b.subscribe()
	if event := <-late; event.Text != "Before subscription" {
		t.Fatal("lost replay")
	}
	if _, open := <-late; open {
		t.Fatal("closed broker accepted a live subscriber")
	}
}

func TestBrokerReplayBound(t *testing.T) {
	b := newBroker()
	for i := 0; i < 1300; i++ {
		b.publish(AgentEvent{Type: "tool", At: int64(i)})
	}
	ch := b.subscribe()
	b.close()
	count := 0
	for range ch {
		count++
	}
	if count != 1024 {
		t.Fatalf("history is unbounded: %d", count)
	}
}

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
