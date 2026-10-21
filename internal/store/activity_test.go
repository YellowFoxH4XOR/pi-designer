package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
)

func TestActivityJournalPersistsAndReplaysWithoutDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := model.Project{ID: "p", Name: "Test", Slug: "test", Dir: t.TempDir(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = st.CreateProject(p); err != nil {
		t.Fatal(err)
	}
	run := model.Run{ID: "r", ProjectID: p.ID, Status: "running", StartedAt: time.Now(), Prompt: "Exact brief"}
	if err = st.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	a := model.NewActivity(run, p)
	publish := func(kind, id, text string) {
		t.Helper()
		e := model.AgentEvent{RunID: run.ID, Type: kind, ToolCallID: id, Tool: "bash", Text: text, At: time.Now().UnixMilli()}
		if err := st.AppendRunEvent(&e, &a); err != nil {
			t.Fatal(err)
		}
	}
	publish("message_start", "", "")
	publish("text", "", "draft")
	publish("message_end", "", "final")
	publish("tool", "a", "")
	publish("tool", "b", "")
	publish("tool_end", "b", "")
	cursor := a.Seq
	// Beyond the old 1024-event replay buffer, with parallel calls out of order.
	for i := 0; i < 1200; i++ {
		publish("session_update", "", "")
	}
	publish("tool_end", "a", "")
	publish("attention", "", "")
	publish("done", "", "")
	if err = st.FinishRun(run.ID, "succeeded", ""); err != nil {
		t.Fatal(err)
	}
	if a.Text != "final" || a.Tools[0].Status != "succeeded" || a.Tools[1].Status != "succeeded" || !a.Attention || a.Label != "Needs your answer" {
		t.Fatalf("bad projection: %+v", a)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	restored, err := st.RunActivity("r")
	if err != nil || restored.Seq != a.Seq || restored.Text != "final" {
		t.Fatalf("restart lost state: %+v %v", restored, err)
	}
	total := 0
	for {
		page, err := st.RunEvents("r", cursor, 500)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if e.Seq <= cursor {
				t.Fatal("duplicate cursor")
			}
			cursor = e.Seq
			total++
		}
	}
	if total != 1203 {
		t.Fatalf("replay lost events: %d", total)
	}
	rows, err := st.LatestRuns()
	if err != nil || len(rows) != 1 || rows[0].ID != "r" {
		t.Fatalf("global discovery: %+v %v", rows, err)
	}
	upper, err := st.OutcomeCursor()
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := st.Outcomes(0, upper)
	if err != nil || len(outcomes) != 2 {
		t.Fatalf("lost attention/completion outcomes: %+v %v", outcomes, err)
	}
	publish("done", "", "") // races the already-captured high-water mark
	bounded, err := st.Outcomes(0, upper)
	if err != nil || len(bounded) != 2 {
		t.Fatal("cursor window is not bounded")
	}
	latest, _ := st.OutcomeCursor()
	next, err := st.Outcomes(upper, latest)
	if err != nil || len(next) != 1 {
		t.Fatal("racing completion was skipped")
	}
	if _, err = st.db.Exec(`DELETE FROM projects WHERE id='p'`); err != nil {
		t.Fatal(err)
	}
	events, err := st.RunEvents("r", 0, 500)
	if err != nil || len(events) != 0 {
		t.Fatal("journal did not cascade with project")
	}
}

func TestJournalFailureDoesNotAdvanceProjection(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := model.Activity{RunID: "missing", Tools: []model.ToolActivity{}}
	e := model.AgentEvent{RunID: "missing", Type: "text", Text: "not saved"}
	if err = st.AppendRunEvent(&e, &a); err == nil {
		t.Fatal("foreign key failure was hidden")
	}
	if a.Text != "" || a.Seq != 0 {
		t.Fatal("uncommitted event reached projection")
	}
}
