package app

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/knowledge"
)

type referenceTransport func(*http.Request) (*http.Response, error)

func (f referenceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReferenceWorkerStartsImmediatelyRepeatsAndCancels(t *testing.T) {
	m, err := knowledge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []knowledge.Source{{ID: "guide", DocumentURL: "https://example.org/guide"}}
	started, repeated := make(chan struct{}), make(chan struct{})
	calls := 0
	m.Client = &http.Client{Transport: referenceTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			close(started)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("# Startup guide\n")), Header: http.Header{}}, nil
		}
		close(repeated)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); runReferenceUpdates(ctx, m, time.Millisecond) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("startup check did not start")
	}
	select {
	case <-repeated:
	case <-time.After(2 * time.Second):
		t.Fatal("periodic check did not run")
	}
	b, err := os.ReadFile(filepath.Join(m.Dir, "guide.md"))
	if err != nil || string(b) != "# Startup guide\n" {
		t.Fatalf("startup did not install guide: %q, %v", b, err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker ignored shutdown")
	}
	if m.List()[0].Error != "" {
		t.Fatal("shutdown persisted as source failure")
	}
}
