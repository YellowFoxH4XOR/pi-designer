package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
)

func TestProjectReferencesMatchWildcardsLiterally(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	if err := st.CreateProject(model.Project{ID: "proj_1", Name: "Landing page", Slug: "landing-page", Dir: t.TempDir(), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"%", "_", "land%", "landing_page"} {
		if p, err := st.GetProject(ref); err != ErrNotFound {
			t.Fatalf("GetProject(%q) = %s, %v; want not found", ref, p.ID, err)
		}
	}
	if p, err := st.GetProject("ding-pa"); err != nil || p.ID != "proj_1" {
		t.Fatalf("substring lookup: %s, %v", p.ID, err)
	}
	if taken, err := st.SlugExists("Landing-Page"); err != nil || !taken {
		t.Fatalf("SlugExists exact: %v, %v", taken, err)
	}
	if taken, err := st.SlugExists("landing"); err != nil || taken {
		t.Fatalf("SlugExists substring: %v, %v", taken, err)
	}
}
