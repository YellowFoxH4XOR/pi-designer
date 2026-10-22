package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/store"
)

func TestDeleteProjectReservesSlugUntilAllFilesAreRemoved(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	proj := model.Project{ID: "proj-delete", Name: "Reserved name", Slug: "reserved-name", Dir: filepath.Join(root, "reserved-name"), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := os.MkdirAll(proj.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(proj); err != nil {
		t.Fatal(err)
	}
	conv := model.Conversation{ID: "conv-delete", ProjectID: proj.ID, Title: "Keep until cleanup", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatal(err)
	}
	// A NUL path reliably fails on all supported filesystems, without requiring
	// permission assumptions or racing a large directory traversal.
	runner := NewRunner(st, nil, "", nil, RunnerConfig{RenderDir: filepath.Join(root, "invalid\x00renders")})
	if err := runner.DeleteProject(proj); err == nil {
		t.Fatal("cleanup unexpectedly accepted an invalid directory")
	}
	if kept, err := st.GetProject(proj.Slug); err != nil || kept.ID != proj.ID {
		t.Fatalf("failed cleanup released the project slug: %+v, %v", kept, err)
	}
	if _, err := st.GetConversation(conv.ID); err != nil {
		t.Fatalf("failed cleanup cascaded conversation history: %v", err)
	}
	runner.cfg.RenderDir = ""
	if err := runner.DeleteProject(proj); err != nil {
		t.Fatalf("cleanup could not be retried: %v", err)
	}
	if _, err := st.GetProject(proj.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("successful cleanup retained the project: %v", err)
	}
}
