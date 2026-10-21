package knowledge

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecommendFromPlainLanguage(t *testing.T) {
	cases := []struct {
		brief, system, intent string
		ids                   []string
	}{
		{"Build a booking dashboard for a small clinic", "", "Operational product interface", []string{"reui", "shadcn"}},
		{"Create a landing page for a tea brand with tasteful motion", "", "Marketing or brand interface", []string{"shadcn", "magicui"}},
		{"Prototype an Android habit tracker", "", "Mobile or Android interface", []string{"material-color", "material"}},
		{"Build a simple invoicing tool", "stripe", "General product interface", []string{"shadcn", "xias"}},
		{"Plan a design system handoff to engineers", "", "Design system or handoff", []string{"figma-sds", "shadcn"}},
	}
	for _, tc := range cases {
		r := Recommend(tc.brief, tc.system)
		if !strings.Contains(r.Intent, tc.intent) || len(r.Choices) != len(tc.ids) {
			t.Errorf("%q: %+v", tc.brief, r)
			continue
		}
		for i, c := range r.Choices {
			if c.ID != tc.ids[i] || c.Reason == "" {
				t.Errorf("%q: %+v", tc.brief, r)
			}
		}
		if tc.system != "" && !strings.Contains(r.Intent, tc.system) {
			t.Errorf("selected system lost: %+v", r)
		}
	}
}
func TestReferenceUsesVerifiedCacheWithoutApplying(t *testing.T) {
	m, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Sources = []Source{{ID: "shadcn", Name: "shadcn", DocumentURL: "https://example.org/index.txt"}, {ID: "figma", Name: "figma"}}
	calls := 0
	m.Client = &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("# Components\n- Dialog\n"))}, nil
	})}
	path, status, err := m.Reference(context.Background(), "shadcn")
	if err != nil || status != "unreviewed cache" || filepath.Base(path) != "shadcn.pending.txt" {
		t.Fatalf("path=%q status=%q err=%v", path, status, err)
	}
	if _, err = os.Stat(filepath.Join(m.Dir, "shadcn.md")); !os.IsNotExist(err) {
		t.Fatal("auto route installed an unreviewed guide")
	}
	if _, _, err = m.Reference(context.Background(), "shadcn"); err != nil || calls != 1 {
		t.Fatalf("refetched cached guide: calls=%d err=%v", calls, err)
	}
	if path, status, err = m.Reference(context.Background(), "figma"); err != nil || path != "" || status != "browse-only" {
		t.Fatalf("browse-only path=%q status=%q err=%v", path, status, err)
	}
	if _, err = m.Apply("shadcn"); err != nil {
		t.Fatal(err)
	}
	path, status, err = m.Reference(context.Background(), "shadcn")
	if err != nil || status != "reviewed" || filepath.Base(path) != "shadcn.md" {
		t.Fatalf("reviewed path=%q status=%q err=%v", path, status, err)
	}
}
