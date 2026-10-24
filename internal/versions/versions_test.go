package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akki/pidesigner/internal/project"
)

func TestSnapshotsDedupeAndRestore(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	pm := project.New(t.TempDir())
	write := func(p, c string) {
		t.Helper()
		if err := pm.Write(p, c); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<h1>v1</h1>")
	write("style.css", "body{}")
	v1, created, err := s.Snapshot("proj_1", pm, Meta{RunID: "run_1", Entry: "index.html"})
	if err != nil || !created || v1.N != 1 || strings.Join(v1.Changed, ",") != "index.html,style.css" {
		t.Fatalf("v1 = %+v, %v, %v", v1, created, err)
	}
	if _, created, _ := s.Snapshot("proj_1", pm, Meta{RunID: "run_2"}); created {
		t.Fatal("unchanged files created a version")
	}
	write("index.html", "<h1>v2</h1>")
	write("about.html", "<p>about</p>")
	if err := pm.Delete("style.css"); err != nil {
		t.Fatal(err)
	}
	v2, created, err := s.Snapshot("proj_1", pm, Meta{RunID: "run_3", MessageID: "msg_3"})
	if err != nil || !created || v2.N != 2 || strings.Join(v2.Changed, ",") != "about.html,index.html" || strings.Join(v2.Removed, ",") != "style.css" {
		t.Fatalf("v2 = %+v, %v, %v", v2, created, err)
	}
	if b, _, err := s.ReadFile("proj_1", 1, "index.html"); err != nil || string(b) != "<h1>v1</h1>" {
		t.Fatalf("v1 index = %q, %v", b, err)
	}
	if _, _, err := s.ReadFile("proj_1", 1, "about.html"); err == nil {
		t.Fatal("v1 served a file it did not have")
	}
	// Unchanged bodies share one blob.
	blobs, _ := os.ReadDir(filepath.Join(s.Root, "proj_1", "blobs"))
	if len(blobs) != 4 {
		t.Fatalf("blobs = %d, want 4", len(blobs))
	}

	if err := s.Restore("proj_1", 1, pm); err != nil {
		t.Fatal(err)
	}
	if b, _ := pm.Read("index.html"); b != "<h1>v1</h1>" {
		t.Fatalf("restored index = %q", b)
	}
	if b, _ := pm.Read("style.css"); b != "body{}" {
		t.Fatalf("restored style = %q", b)
	}
	if _, err := pm.Read("about.html"); err == nil {
		t.Fatal("restore kept a file the version did not have")
	}
	all, err := s.List("proj_1")
	if err != nil || len(all) != 2 {
		t.Fatalf("list = %d, %v", len(all), err)
	}
}

func TestRejectsUnsafeProjectIDs(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	for _, id := range []string{"../x", "a/b", ""} {
		if _, err := s.List(id); err == nil {
			t.Fatalf("List(%q) accepted", id)
		}
	}
}
