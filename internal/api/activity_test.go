package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/store"
)

func TestCompletedEventStreamResumesByLastEventID(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "activity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := model.Project{ID: "p", Name: "Review", Slug: "review", Dir: t.TempDir(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = st.CreateProject(p); err != nil {
		t.Fatal(err)
	}
	run := model.Run{ID: "r", ProjectID: "p", Status: "running", StartedAt: time.Now()}
	st.CreateRun(run)
	b := newRunJournal(st, run, p)
	b.publish(AgentEvent{Type: "text", RunID: "r", Text: "first", At: nowMs()})
	a, _ := st.RunActivity("r")
	cursor := a.Seq
	b.publish(AgentEvent{Type: "text", RunID: "r", Text: "second", At: nowMs()})
	st.FinishRun("r", "succeeded", "")
	b.publish(AgentEvent{Type: "done", RunID: "r", At: nowMs()})
	server := &Server{Store: st}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs/{id}/events", server.handleRunEvents)
	req := httptest.NewRequest("GET", "/api/runs/r/events?after=0", nil)
	req.Header.Set("Last-Event-ID", jsonNumber(cursor))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	data, _ := io.ReadAll(rr.Result().Body)
	body := string(data)
	if strings.Contains(body, `"text":"first"`) || !strings.Contains(body, `"text":"second"`) || !strings.Contains(body, "event: stream_end") || !strings.Contains(body, "event: snapshot") {
		t.Fatal(body)
	}
	// The projection includes the complete message, even though delta replay resumes.
	if !strings.Contains(body, `"text":"firstsecond"`) {
		t.Fatal("snapshot is not authoritative")
	}
}
func jsonNumber(n int64) string { b, _ := json.Marshal(n); return string(b) }
func TestQuestionAttentionOnlyForValidUnfencedForms(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{`<question-form>{"questions":[{"label":"Choose"}]}</question-form>`, true},
		{"```html\n<question-form>{\"questions\":[{\"label\":\"Example\"}]}</question-form>\n```", false},
		{`<question-form>{"questions":[]}</question-form>`, false},
		{`<question-form>{broken}</question-form>`, false},
		{`<question-form>{"questions":[{}]}</question-form>`, false},
	} {
		var events []AgentEvent
		b := &eventJournal{persist: func(e *AgentEvent) error { events = append(events, *e); return nil }}
		publishQuestionAttention("r", tc.text, b)
		if (len(events) > 0) != tc.want {
			t.Fatalf("attention=%v for %s", len(events) > 0, tc.text)
		}
	}
}
func TestCanceledAndFailedRunsClearAttention(t *testing.T) {
	for _, kind := range []string{"canceled", "run_failed"} {
		a := model.Activity{Status: "running", Attention: true, Tools: []model.ToolActivity{{ID: "x", Status: "running"}}}
		a.Apply(model.AgentEvent{Type: kind, At: 10})
		if a.Attention || a.Tools[0].Status != "interrupted" {
			t.Fatalf("bad terminal: %+v", a)
		}
	}
}
