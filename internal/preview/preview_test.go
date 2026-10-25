package preview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/httpguard"
	"github.com/coder/websocket"
)

func TestFileHandlerAppliesSandboxHeadersToHTMLSVGAndAssets(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html":      "<!doctype html><body>preview</body>",
		"image.svg":       `<svg xmlns="http://www.w3.org/2000/svg"><script>window.bad=true</script></svg>`,
		"assets/app.js":   "window.previewLoaded = true;",
		"assets/site.css": "body { color: red; }",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	handler := NewHub().FileHandler(root)
	for _, tc := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html; charset=utf-8", contains: "preview"},
		{path: "/image.svg", contentType: "image/svg+xml", contains: "<svg"},
		{path: "/assets/app.js", contentType: "text/javascript; charset=utf-8", contains: "previewLoaded"},
		{path: "/assets/site.css", contentType: "text/css; charset=utf-8", contains: "color: red"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://localhost"+tc.path, nil)
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%q", w.Code, w.Body.String())
			}
			if got := w.Header().Get("Content-Security-Policy"); got != "sandbox allow-scripts allow-forms" {
				t.Errorf("Content-Security-Policy = %q", got)
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q", got)
			}
			if got := w.Header().Get("Content-Type"); got != tc.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tc.contentType)
			}
			if !strings.Contains(w.Body.String(), tc.contains) {
				t.Errorf("response body does not contain %q", tc.contains)
			}
		})
	}
}

func TestFileHandlerAllowsOpaqueCORSOnlyForPreviewResources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.mjs"), []byte("export const loaded = true;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<p>private preview</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := NewHub().FileHandler(root)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/app.mjs", nil)
	req.Header.Set("Origin", "null")
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "null" {
		t.Fatalf("module response CORS = %q, want null", got)
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("module response Vary = %q, want Origin", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	req.Header.Set("Origin", "null")
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("HTML response unexpectedly enables CORS: %q", got)
	}
}

func TestFileHandlerConfinesSymlinksAndTraversal(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "secret.js")
	if err := os.WriteFile(externalFile, []byte("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	externalFileRel, err := filepath.Rel(root, externalFile)
	if err != nil {
		t.Fatal(err)
	}
	externalDirRel, err := filepath.Rel(root, external)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalFileRel, filepath.Join(root, "external.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDirRel, filepath.Join(root, "external-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside", "internal.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside/internal.txt", filepath.Join(root, "internal-link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside", filepath.Join(root, "internal-dir")); err != nil {
		t.Fatal(err)
	}

	handler := NewHub().FileHandler(root)
	for _, tc := range []struct {
		path string
		want int
	}{
		{path: "/internal-link.txt", want: http.StatusOK},
		{path: "/internal-dir/internal.txt", want: http.StatusOK},
		{path: "/external.js", want: http.StatusNotFound},
		{path: "/external-dir/secret.js", want: http.StatusNotFound},
		{path: "/../outside-secret.js", want: http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://localhost"+tc.path, nil)
			handler.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%q", w.Code, tc.want, w.Body.String())
			}
		})
	}
	if got, err := os.ReadFile(externalFile); err != nil || string(got) != "outside secret" {
		t.Fatalf("outside file changed: %q, %v", got, err)
	}
}

func TestServeWSChecksOrigin(t *testing.T) {
	hub := NewHub()
	server := httptest.NewServer(httpguard.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r, "demo")
	})))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opaqueConn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"null"}},
	})
	if err == nil {
		_ = opaqueConn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("WebSocket accepted an opaque Origin")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("opaque Origin response = %#v, err=%v; want 403", response, err)
	}

	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{server.URL}},
	})
	if err != nil {
		t.Fatalf("same-origin WebSocket rejected: response=%#v, err=%v", response, err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	_, message, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading WebSocket hello: %v", err)
	}
	if !strings.Contains(string(message), "hello") {
		t.Fatalf("WebSocket hello = %q", message)
	}
}

func TestWatcherPollsOnlyWhileAClientListens(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<p>one</p>")
	hub := NewHub()
	stop := hub.Watch("demo", dir)
	defer stop()

	time.Sleep(600 * time.Millisecond)
	hub.mu.Lock()
	w := hub.watchers["demo"]
	hub.mu.Unlock()
	w.mu.Lock()
	walked := len(w.last)
	w.mu.Unlock()
	if walked != 0 {
		t.Fatalf("watcher walked %d files with no preview client", walked)
	}
	write("index.html", "<p>edited while idle</p>")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		hub.ServeWS(rw, r, "demo")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatalf("reading hello: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	write("style.css", "p{color:red}")
	_, message, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading change: %v", err)
	}
	if !strings.Contains(string(message), `"style.css"`) || strings.Contains(string(message), "index.html") {
		t.Fatalf("change message = %s; want only style.css", message)
	}
}
