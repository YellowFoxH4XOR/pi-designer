package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akki/pidesigner/internal/knowledge"
)

type stubTransport func(*http.Request) (*http.Response, error)

func (f stubTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSelectedReferenceIsAllowlistedAndUnreviewed(t *testing.T) {
	mgr, err := knowledge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr.Sources = []knowledge.Source{{ID: "shadcn", Name: "shadcn", DocumentURL: "https://example.org/guide.md"}, {ID: "figma", Name: "figma"}}
	mgr.Client = &http.Client{Transport: stubTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("# Approved text\nSome design guidance."))}, nil
	})}
	s := &Server{Knowledge: mgr}
	for _, tc := range []struct {
		id   string
		code int
		want string
	}{
		{"shadcn", 200, "unreviewed cache"}, {"figma", 200, "browse-only"}, {"unknown", 502, "design reference unavailable"},
	} {
		req := httptest.NewRequest("POST", "/api/knowledge/"+tc.id+"/reference", nil)
		req.SetPathValue("id", tc.id)
		w := httptest.NewRecorder()
		s.handleKnowledgeReference(w, req)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: %d %s", tc.id, w.Code, w.Body.String())
		}
	}
	req := httptest.NewRequest("POST", "/api/knowledge/shadcn/reference", nil)
	req.SetPathValue("id", "shadcn")
	req.Header.Set("Origin", "https://foreign.example")
	w := httptest.NewRecorder()
	s.handleKnowledgeReference(w, req)
	if w.Code != 403 {
		t.Fatalf("foreign origin accepted: %d", w.Code)
	}
	if got := mgr.Docs(); len(got) != 0 {
		t.Fatalf("unreviewed reference was installed: %v", got)
	}
}
