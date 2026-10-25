package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
)

func TestRunActivitySummaryTracksProjectionWithoutTranscript(t *testing.T) {
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
	for _, e := range []model.AgentEvent{{Type: "message_start"}, {Type: "text", Text: "draft"}, {Type: "tool", ToolCallID: "a", Tool: "bash"}, {Type: "attention"}} {
		e.RunID, e.At = run.ID, time.Now().UnixMilli()
		if err = st.AppendRunEvent(&e, &a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.RunActivitySummary(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := a.Summary(); got.Seq != want.Seq || got.Phase != want.Phase || got.Label != want.Label || !got.Attention || got.Text != "" || got.Prompt != "" || got.Tools != nil {
		t.Fatalf("summary = %+v; want %+v", got, want)
	}

	// Projections written before the summary table are backfilled on open.
	if _, err = st.db.Exec(`DELETE FROM run_summary`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	backfilled, err := st.RunActivitySummary(run.ID)
	if err != nil || backfilled.Seq != a.Seq || !backfilled.Attention || backfilled.Text != "" {
		t.Fatalf("backfilled summary = %+v, err=%v", backfilled, err)
	}
}

func TestProjectsStampChangesWithProjectList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	// Clients compare each poll with the previous one.
	previous := ""
	record := func(step string) {
		t.Helper()
		stamp, err := st.ProjectsStamp()
		if err != nil {
			t.Fatal(err)
		}
		if stamp == previous {
			t.Fatalf("stamp %q unchanged after %s", stamp, step)
		}
		previous = stamp
	}
	record("open")
	p := model.Project{ID: "p", Name: "One", Slug: "one", Dir: t.TempDir(), CreatedAt: time.UnixMilli(1), UpdatedAt: time.UnixMilli(1)}
	if err = st.CreateProject(p); err != nil {
		t.Fatal(err)
	}
	record("create")
	// Multiple writes can share a timestamp; each must invalidate the list.
	for _, name := range []string{"Renamed", "Renamed again"} {
		if _, err = st.db.Exec(`UPDATE projects SET name=? WHERE id=?`, name, p.ID); err != nil {
			t.Fatal(err)
		}
		record("rename without timestamp change")
	}
	if err = st.TouchProject(p.ID); err != nil {
		t.Fatal(err)
	}
	record("touch")
	if err = st.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	record("delete")
	// Delete + recreate between polls must not recreate an earlier stamp.
	if err = st.CreateProject(p); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	record("create and delete between polls")
	// Triggers participate in the transaction: failed mutations cannot leave
	// a revision change, and normal daemon restarts cannot reset the token.
	tx, err := st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO projects(id,name,slug,dir,created_at,updated_at) VALUES('rolled-back','X','x','x',1,1)`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	st = reopened
	if stamp, err := st.ProjectsStamp(); err != nil || stamp != previous {
		t.Fatalf("revision changed after rollback/reopen: %q, %v; want %q", stamp, err, previous)
	}
}
