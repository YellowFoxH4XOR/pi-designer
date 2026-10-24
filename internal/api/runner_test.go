package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/content"
	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/project"
	"github.com/akki/pidesigner/internal/store"
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

func TestTimedMotionRendersAreParsedAndServed(t *testing.T) {
	got, ok := parseRender("p", []byte(`{"viewport":"mobile","file":"7-mobile-t2500.png","at":2500}`))
	if !ok || got.At != 2500 {
		t.Fatalf("timed render parsed as %+v", got)
	}
	if (&Runner{cfg: RunnerConfig{RenderDir: "/r"}}).RenderFile("p", "7-mobile-t2500.png") == "" {
		t.Fatal("timed render file is not served")
	}
}

type runnerTestHub struct{}

func (runnerTestHub) NotifyProjectChanged(string) {}

func newRunnerTest(t *testing.T) (*Runner, *store.Store, model.Project, string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pidesigner.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	proj := model.Project{
		ID: "runner-test", Name: "Runner test", Slug: "runner-test", Dir: projectDir,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := st.CreateProject(proj); err != nil {
		t.Fatal(err)
	}
	conversationID := "runner-test-conversation"
	if err := st.CreateConversation(model.Conversation{
		ID: conversationID, ProjectID: proj.ID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	catalog, err := content.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs("../../scripts/fixtures/pi-rpc.cjs")
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(st, catalog, fixture, runnerTestHub{}, RunnerConfig{})
	t.Cleanup(runner.Shutdown)
	return runner, st, proj, conversationID, dbPath
}

func addTranscriptFailureTrigger(t *testing.T, dbPath, role string) {
	t.Helper()
	if role != model.RoleUser && role != model.RoleAssistant {
		t.Fatalf("unexpected transcript role %q", role)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := fmt.Sprintf(
		"CREATE TRIGGER reject_%s_message BEFORE INSERT ON messages WHEN NEW.role='%s' BEGIN SELECT RAISE(ABORT, 'fixture %s transcript failure'); END",
		role, role, role,
	)
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func waitRunnerStatus(t *testing.T, st *store.Store, runID, status string) model.Run {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		run, err := st.GetRun(runID)
		if err == nil && run.Status == status {
			return run
		}
		if err == nil && run.Status != model.RunRunning {
			t.Fatalf("run ended as %q, wanted %q: %+v", run.Status, status, run)
		}
		time.Sleep(20 * time.Millisecond)
	}
	run, err := st.GetRun(runID)
	t.Fatalf("run did not reach %q: %+v (%v)", status, run, err)
	return model.Run{}
}

func waitForRunnerTool(t *testing.T, st *store.Store, runID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		activity, err := st.RunActivity(runID)
		if err == nil && len(activity.Tools) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not publish its fixture tool event", runID)
}

func TestStartRollsBackRunWhenUserTranscriptCannotPersist(t *testing.T) {
	runner, st, proj, conversationID, dbPath := newRunnerTest(t)
	addTranscriptFailureTrigger(t, dbPath, model.RoleUser)

	_, err := runner.Start(context.Background(), RunRequest{
		ProjectID: proj.ID, ConversationID: conversationID, Prompt: "A prompt that must be saved",
	})
	if err == nil || !strings.Contains(err.Error(), "fixture user transcript failure") {
		t.Fatalf("Start error = %v, want the real transcript write failure", err)
	}
	runs, err := st.ListRuns(proj.ID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("failed admission left runs %+v (error %v)", runs, err)
	}
	messages, err := st.ListMessages(conversationID)
	if err != nil || len(messages) != 0 {
		t.Fatalf("failed admission left messages %+v (error %v)", messages, err)
	}
	runner.mu.Lock()
	active := runner.active[proj.ID]
	runner.mu.Unlock()
	if active != "" {
		t.Fatalf("failed admission left active run %q", active)
	}
}

func TestAssistantTranscriptFailureFailsRun(t *testing.T) {
	runner, st, proj, conversationID, dbPath := newRunnerTest(t)
	addTranscriptFailureTrigger(t, dbPath, model.RoleAssistant)

	handle, err := runner.Start(context.Background(), RunRequest{
		ProjectID: proj.ID, ConversationID: conversationID, Prompt: "FAST_TASK", Model: "openai/gpt-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	run := waitRunnerStatus(t, st, handle.RunID, model.RunFailed)
	if !strings.Contains(run.Error, "fixture assistant transcript failure") {
		t.Fatalf("failed run error = %q", run.Error)
	}
}

func TestCancelForceClosesSessionThatNeverSettles(t *testing.T) {
	runner, st, proj, conversationID, _ := newRunnerTest(t)
	handle, err := runner.Start(context.Background(), RunRequest{
		ProjectID: proj.ID, ConversationID: conversationID, Prompt: "ACK_ABORT_NO_SETTLE", Model: "openai/gpt-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForRunnerTool(t, st, handle.RunID)
	if err := runner.Cancel(handle.RunID); err != nil {
		t.Fatal(err)
	}
	waitRunnerStatus(t, st, handle.RunID, model.RunCanceled)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runner.mu.Lock()
		active := runner.active[proj.ID]
		runner.mu.Unlock()
		if active == "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	runner.mu.Lock()
	active := runner.active[proj.ID]
	runner.mu.Unlock()
	if active != "" {
		t.Fatalf("canceled run left project lock %q", active)
	}

	next, err := runner.Start(context.Background(), RunRequest{
		ProjectID: proj.ID, ConversationID: conversationID, Prompt: "FAST_TASK", Model: "openai/gpt-fixture",
	})
	if err != nil {
		t.Fatalf("subsequent run was blocked: %v", err)
	}
	waitRunnerStatus(t, st, next.RunID, model.RunSucceeded)
}

func TestBuildArtifactResolvesNestedAndRootRelativeAssetsAndPreservesPNGBytes(t *testing.T) {
	root := t.TempDir()
	writeArtifactTestFile(t, root, "pages/deep/index.html", `<link href="theme.css?theme=dark#top"><link href="/assets/global.css?v=3"><script src="/scripts/app.js?module=1"></script><img src="../img/pixel.png?size=1#image"><a href="#section">section</a><img src="data:image/png;base64,AA=="><script src="https://cdn.example/app.js"></script><link href="//cdn.example/site.css">`)
	writeArtifactTestFile(t, root, "pages/deep/theme.css", `body{color:#222}`)
	writeArtifactTestFile(t, root, "theme.css", `body{color:red}`)
	writeArtifactTestFile(t, root, "assets/global.css", `body{margin:0}`)
	writeArtifactTestFile(t, root, "scripts/app.js", `console.log("ok")`)
	png := []byte{0x89, 'P', 'N', 'G', 13, 10, 26, 10, 0, 0xff, 0x80}
	writeArtifactTestFile(t, root, "pages/img/pixel.png", string(png))

	bundle, err := BuildArtifact(project.New(root), "pages/deep/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.EntryFile != "pages/deep/index.html" {
		t.Fatalf("entry file = %q", bundle.EntryFile)
	}
	if got, ok := bundle.Files["pages/deep/theme.css"]; !ok || got.Content != `body{color:#222}` {
		t.Fatalf("nested sibling was not selected: %+v", got)
	}
	for _, name := range []string{"assets/global.css", "scripts/app.js", "pages/img/pixel.png"} {
		if _, ok := bundle.Files[name]; !ok {
			t.Errorf("artifact omitted %q", name)
		}
	}
	if len(bundle.Files) != 5 {
		t.Fatalf("artifact included unrelated or missing files: %v", bundle.Files)
	}

	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip ArtifactBundle
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	entry := roundTrip.Files["pages/deep/index.html"]
	if entry.Encoding != "utf-8" || entry.MIMEType != "text/html" {
		t.Fatalf("HTML metadata = %+v", entry)
	}
	image := roundTrip.Files["pages/img/pixel.png"]
	if image.Encoding != "base64" || image.MIMEType != "image/png" {
		t.Fatalf("PNG metadata = %+v", image)
	}
	decoded, err := base64.StdEncoding.DecodeString(image.Content)
	if err != nil || string(decoded) != string(png) {
		t.Fatalf("PNG bytes did not round-trip: %x (%v)", decoded, err)
	}
}

func TestBuildArtifactReportsMissingEntryRelativeDependency(t *testing.T) {
	root := t.TempDir()
	writeArtifactTestFile(t, root, "pages/index.html", `<link href="theme.css?v=1">`)
	writeArtifactTestFile(t, root, "theme.css", `body{color:red}`)
	_, err := BuildArtifact(project.New(root), "pages/index.html")
	if err == nil || !strings.Contains(err.Error(), `pages/theme.css`) {
		t.Fatalf("missing nested dependency error = %v", err)
	}
}

func TestBuildArtifactIgnoresScriptCodeCommentsAndDataAttributes(t *testing.T) {
	root := t.TempDir()
	writeArtifactTestFile(t, root, "index.html", `<!-- <img src="old.png"> -->
<img data-src="lazy/missing.png" src="hero.png">
<script src="app.js"></script>
<script>const img = new Image(); img.src = "generated/missing.png"; el.innerHTML = '<a href="nope.html">';</script>`)
	writeArtifactTestFile(t, root, "hero.png", "png")
	writeArtifactTestFile(t, root, "app.js", "1")
	bundle, err := BuildArtifact(project.New(root), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 3 {
		t.Fatalf("artifact files = %v", bundle.Files)
	}
	for _, name := range []string{"hero.png", "app.js"} {
		if _, ok := bundle.Files[name]; !ok {
			t.Errorf("artifact omitted %q", name)
		}
	}
}

func writeArtifactTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
