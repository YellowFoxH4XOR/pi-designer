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

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReviewApplyAndTamper(t *testing.T) {
	dir := t.TempDir()
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []Source{{ID: "xias", Name: "XIAS", URL: "https://example.org", DocumentURL: "https://example.org/doc.md"}, {ID: "figma", Name: "Figma", URL: "https://figma.com"}}
	version := "# First version\n"
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(version)), Header: http.Header{}}, nil
	})}
	if _, err = m.Check(context.Background(), "figma"); err == nil {
		t.Fatal("link-only source should be rejected")
	}
	first, err := m.Check(context.Background(), "xias")
	if err != nil || !first.Updated {
		t.Fatalf("check: %+v %v", first, err)
	}
	if _, err = m.Apply("xias"); err != nil {
		t.Fatal(err)
	}
	if got := m.Docs(); len(got) != 1 || got[0] != "xias.md" {
		t.Fatalf("docs %v", got)
	}
	version = "# Second version\n"
	if _, err = m.Check(context.Background(), "xias"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "xias.pending.txt"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Apply("xias"); err == nil {
		t.Fatal("tampered staging applied")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "xias.md"))
	if string(b) != "# First version\n" {
		t.Fatal("old version not preserved")
	}
	if _, err = m.Check(context.Background(), "xias"); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Apply("xias"); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Rollback("xias"); err != nil {
		t.Fatal(err)
	}
	rolled, _ := os.ReadFile(filepath.Join(dir, "xias.md"))
	if string(rolled) != "# First version\n" {
		t.Fatal("rollback did not restore previous version")
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Docs(); len(got) != 1 {
		t.Fatalf("reopened %v", got)
	}
	if err = os.WriteFile(filepath.Join(dir, "xias.md"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if len(reopened.Docs()) != 0 {
		t.Fatal("tampered installed file listed")
	}
}
func TestLimitsAndFailures(t *testing.T) {
	m, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []Source{{ID: "xias", DocumentURL: "https://example.org/doc.md"}}
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxDocument+1))), Header: http.Header{}}, nil
	})}
	if _, err = m.Check(context.Background(), "xias"); err == nil {
		t.Fatal("oversized response accepted")
	}
	m.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	if _, err = m.Check(context.Background(), "xias"); err == nil {
		t.Fatal("offline failure ignored")
	}
	if _, err = m.Preview("../state"); err == nil {
		t.Fatal("arbitrary file read accepted")
	}
}
