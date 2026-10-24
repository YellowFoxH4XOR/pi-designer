package knowledge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncInstallsUpdatesAndPreservesLastGood(t *testing.T) {
	m, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []Source{
		{ID: "first", DocumentURL: "https://example.org/first"},
		{ID: "browse", URL: "https://example.org/browse"},
		{ID: "last", DocumentURL: "https://example.org/last"},
	}
	versions := map[string]string{"/first": "# First v1\n", "/last": "# Last v1\n"}
	broken := false
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/browse" {
			t.Fatal("browse-only source fetched")
		}
		if broken && r.URL.Path == "/first" {
			return nil, errors.New("offline")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(versions[r.URL.Path])), Header: http.Header{}}, nil
	})}
	assertFile := func(name, want string) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err != nil || string(b) != want {
			t.Fatalf("%s: %q, %v; want %q", name, b, err, want)
		}
	}
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFile("first.md", "# First v1\n")
	assertFile("last.md", "# Last v1\n")
	versions["/first"] = "# First v2\n"
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFile("first.md", "# First v2\n")
	assertFile("first.previous.md", "# First v1\n")
	// Unchanged checks must not replace the rollback snapshot.
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFile("first.previous.md", "# First v1\n")
	broken = true
	versions["/last"] = "# Last v2\n"
	if err := m.Sync(context.Background()); err == nil {
		t.Fatal("offline source not reported")
	}
	assertFile("first.md", "# First v2\n")
	assertFile("last.md", "# Last v2\n")
	entries := m.List()
	if entries[0].Error == "" || entries[0].CheckedAt == "" || entries[2].Error != "" {
		t.Fatalf("source states: %+v", entries)
	}
	reopened, err := Open(m.Dir)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Sources = m.Sources
	if reopened.List()[0].Error == "" {
		t.Fatal("failure not persisted")
	}
	reopened.Client = m.Client
	m = reopened
	if _, err := reopened.Rollback("first"); err != nil {
		t.Fatal(err)
	}
	assertFile("first.md", "# First v1\n")
	broken = false
	versions["/first"] = "<html>not a guide</html>"
	if err := m.Sync(context.Background()); err == nil {
		t.Fatal("invalid document accepted")
	}
	assertFile("first.md", "# First v1\n")
	versions["/first"] = "# First v3\n"
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.List()[0].Error != "" {
		t.Fatal("recovery did not clear failure")
	}
	assertFile("first.md", "# First v3\n")
}

func TestSyncCancellationKeepsInstalledSnapshot(t *testing.T) {
	m, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []Source{{ID: "first", DocumentURL: "https://example.org/first"}, {ID: "last", DocumentURL: "https://example.org/last"}}
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("# Working\n")), Header: http.Header{}}, nil
	})}
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/first" {
			t.Error("continued after cancellation")
		}
		// Reading status during a network request must not block on its mutex.
		if m.List()[0].InstalledHash == "" {
			t.Error("missing installed state")
		}
		cancel()
		return nil, r.Context().Err()
	})}
	if err := m.Sync(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync cancellation: %v", err)
	}
	for _, entry := range m.List() {
		if entry.Error != "" {
			t.Fatalf("shutdown recorded as source failure: %+v", entry)
		}
	}
	b, err := os.ReadFile(filepath.Join(m.Dir, "first.md"))
	if err != nil || string(b) != "# Working\n" {
		t.Fatalf("installed snapshot: %q, %v", b, err)
	}
}
