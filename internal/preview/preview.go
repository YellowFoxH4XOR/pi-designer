// Package preview serves project files for the live preview and pushes change
// notifications to connected clients.
//
// The preview deliberately avoids reloading the iframe for CSS-only edits:
// the injected bridge script swaps stylesheet hrefs in place, so a token or
// layout change lands without a white flash, a scroll reset, or a dropped
// frame. HTML edits fall back to a reload.
package preview

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Change describes one file mutation detected by a watcher.
type Change struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"` // "css" | "html" | "other"
	Stamp string `json:"stamp"`
}

// Message is pushed to preview clients over the websocket.
type Message struct {
	Type    string   `json:"type"` // "hello" | "changed"
	Project string   `json:"project,omitempty"`
	Changes []Change `json:"changes,omitempty"`
	Version string   `json:"version,omitempty"`
}

// Client is one connected preview browser.
type Client struct {
	Project string
	ws      *websocket.Conn
	mu      sync.Mutex
}

// Hub tracks watchers and websocket clients per project.
type Hub struct {
	mu       sync.Mutex
	watchers map[string]*watcher
	clients  map[string]map[*Client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		watchers: map[string]*watcher{},
		clients:  map[string]map[*Client]struct{}{},
	}
}

// watcher polls a project directory and reports mutations.
type watcher struct {
	project string
	dir     string
	hub     *Hub
	stop    chan struct{}
	once    sync.Once
	last    map[string]fileStamp
	mu      sync.Mutex
}

type fileStamp struct {
	size int64
	mod  time.Time
}

// Watch starts polling a project directory. Call stop() to end it.
func (h *Hub) Watch(project, dir string) (stop func()) {
	h.mu.Lock()
	if _, exists := h.watchers[project]; exists {
		h.mu.Unlock()
		return func() {}
	}
	w := &watcher{
		project: project,
		dir:     dir,
		hub:     h,
		stop:    make(chan struct{}),
		last:    map[string]fileStamp{},
	}
	h.watchers[project] = w
	h.mu.Unlock()

	w.scan() // prime the baseline
	go w.loop()
	return func() { w.once.Do(func() { close(w.stop) }) }
}

// Unwatch stops polling a project directory, e.g. once the project is deleted.
func (h *Hub) Unwatch(project string) {
	h.mu.Lock()
	w := h.watchers[project]
	delete(h.watchers, project)
	h.mu.Unlock()
	if w != nil {
		w.once.Do(func() { close(w.stop) })
	}
}

func (w *watcher) loop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

// scan diffs the directory against the last known state.
func (w *watcher) scan() {
	w.mu.Lock()
	defer w.mu.Unlock()
	seen := map[string]fileStamp{}
	_ = filepath.WalkDir(w.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(w.dir, path)
		if err != nil {
			return nil
		}
		seen[filepath.ToSlash(rel)] = fileStamp{size: info.Size(), mod: info.ModTime()}
		return nil
	})

	var changes []Change
	for p, st := range seen {
		old, ok := w.last[p]
		if !ok || old.size != st.size || !old.mod.Equal(st.mod) {
			changes = append(changes, Change{Path: p, Kind: changeKind(p), Stamp: stampOf(p, st)})
		}
	}
	for p := range w.last {
		if _, ok := seen[p]; !ok {
			changes = append(changes, Change{Path: p, Kind: changeKind(p), Stamp: stampOf(p, fileStamp{})})
		}
	}
	w.last = seen

	if len(changes) == 0 {
		return
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	w.hub.broadcast(w.project, Message{Type: "changed", Project: w.project, Changes: changes})
}

func changeKind(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".css":
		return "css"
	case ".html", ".htm":
		return "html"
	default:
		return "other"
	}
}

func stampOf(path string, st fileStamp) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d", path, st.size, st.mod.UnixNano())))
	return hex.EncodeToString(h[:])[:12]
}

// NotifyProjectChanged forces an immediate rescan of a project's watcher so
// preview clients see the change without waiting for the next poll tick.
func (h *Hub) NotifyProjectChanged(project string) {
	h.mu.Lock()
	w, ok := h.watchers[project]
	h.mu.Unlock()
	if ok {
		w.scan()
	}
}

// AddClient registers a websocket client for a project.
func (h *Hub) AddClient(project string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[project] == nil {
		h.clients[project] = map[*Client]struct{}{}
	}
	h.clients[project][c] = struct{}{}
}

// RemoveClient drops a websocket client.
func (h *Hub) RemoveClient(project string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[project]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, project)
		}
	}
}

// broadcast pushes a message to every client of a project.
func (h *Hub) broadcast(project string, msg Message) {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients[project]))
	for c := range h.clients[project] {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.mu.Lock()
		_ = writeMessage(c.ws, msg)
		c.mu.Unlock()
	}
}

// ServeWS upgrades an HTTP request to a websocket and streams change events.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, project string) {
	// Keep the library's default same-origin Origin verification enabled.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	c := &Client{Project: project, ws: conn}
	h.AddClient(project, c)
	defer func() {
		h.RemoveClient(project, c)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	_ = writeMessage(conn, Message{Type: "hello", Project: project})

	// Drain inbound frames so close/ping handling works.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
	}
}

// FileHandler serves a project directory with the hot-swap bridge injected
// into HTML responses.
func (h *Hub) FileHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-forms")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		name, ok := cleanPreviewPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		// The configured project root is trusted; OpenRoot confines access
		// beneath it while allowing only links that stay within the boundary.
		root, err := os.OpenRoot(dir)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer root.Close()

		file, err := root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			_ = file.Close()
			name = path.Join(name, "index.html")
			file, err = root.Open(name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			info, err = file.Stat()
			if err != nil {
				_ = file.Close()
				http.NotFound(w, r)
				return
			}
		}
		defer file.Close()
		if !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}

		setPreviewCORS(w, r, name)
		if typ := previewContentType(name); typ != "" {
			w.Header().Set("Content-Type", typ)
		}
		if isHTML(name) {
			doc, err := io.ReadAll(file)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(InjectBridge(doc)))
			return
		}
		http.ServeContent(w, r, name, info.ModTime(), file)
	})
}

func cleanPreviewPath(raw string) (string, bool) {
	rel := filepath.ToSlash(raw)
	rel = strings.TrimPrefix(rel, "/")
	if strings.HasPrefix(rel, "/") {
		return "", false
	}
	if rel == "" {
		return "index.html", true
	}
	depth := 0
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case "", ".":
		case "..":
			if depth == 0 {
				return "", false
			}
			depth--
		default:
			depth++
		}
	}
	name := path.Clean(rel)
	if name == "." {
		name = "index.html"
	}
	return name, true
}

func isHTML(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm":
		return true
	default:
		return false
	}
}

func previewContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs", ".cjs":
		return "text/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return mime.TypeByExtension(filepath.Ext(name))
	}
}

func setPreviewCORS(w http.ResponseWriter, r *http.Request, name string) {
	origins := r.Header.Values("Origin")
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || len(origins) != 1 || origins[0] != "null" {
		return
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".css", ".js", ".mjs", ".cjs", ".json", ".woff", ".woff2", ".ttf", ".otf", ".eot", ".wasm":
		w.Header().Set("Access-Control-Allow-Origin", "null")
		w.Header().Add("Vary", "Origin")
	}
}

// bridgeScript is injected into every previewed HTML document. It listens for
// postMessage hot-swap instructions from the parent UI.
const bridgeScript = `<script data-pidesigner-bridge>
(function () {
  var VERSION = "__BRIDGE_VERSION__";
  function swapCSS(files) {
    (files || []).forEach(function (f) {
      var links = document.querySelectorAll('link[rel="stylesheet"]');
      var matched = false;
      for (var i = 0; i < links.length; i++) {
        var href = links[i].getAttribute("href") || "";
        if (href.indexOf(f.path) !== -1 || href.split("?")[0].split("/").pop() === f.name) {
          links[i].setAttribute("href", href.split("?")[0] + "?v=" + encodeURIComponent(f.stamp || Date.now()));
          matched = true;
        }
      }
      if (!matched) {
        var link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = f.path + "?v=" + encodeURIComponent(f.stamp || Date.now());
        document.head.appendChild(link);
      }
    });
  }
  window.addEventListener("message", function (ev) {
    var d = ev.data;
    if (ev.source !== window.parent || !d || d.source !== "pidesigner") return;
    if (d.type === "css-update") {
      swapCSS(d.files);
    } else if (d.type === "reload") {
      location.reload();
    }
  });
  window.__pidesigner = { version: VERSION, swapCSS: swapCSS };
})();
</script>`

var bridgeVersion = fmt.Sprintf("%d", time.Now().UnixNano())

// InjectBridge inserts the hot-swap bridge into an HTML document.
func InjectBridge(doc []byte) []byte {
	script := strings.ReplaceAll(bridgeScript, "__BRIDGE_VERSION__", bridgeVersion)
	if i := strings.LastIndex(strings.ToLower(string(doc)), "</body>"); i >= 0 {
		out := make([]byte, 0, len(doc)+len(script))
		out = append(out, doc[:i]...)
		out = append(out, script...)
		out = append(out, doc[i:]...)
		return out
	}
	out := append([]byte{}, doc...)
	out = append(out, script...)
	return out
}

// BridgeVersion is the current hot-swap cache-buster.
func BridgeVersion() string { return bridgeVersion }

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error"}`)
	}
	return b
}

// writeMessage sends one message, giving up on a stalled client after 5s.
func writeMessage(ws *websocket.Conn, msg Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ws.Write(ctx, websocket.MessageText, mustJSON(msg))
}

// CleanPath turns a preview URL path into a project-relative file name,
// defaulting to index.html; ok is false when it would escape the project.
func CleanPath(raw string) (string, bool) { return cleanPreviewPath(raw) }

// ServeSnapshot serves one file of a saved version with the same sandbox as a
// live preview. Versions never change, so there is no reload bridge and the
// response may be cached.
func ServeSnapshot(w http.ResponseWriter, r *http.Request, name string, data []byte, modTime time.Time) {
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-forms")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	setPreviewCORS(w, r, name)
	if typ := previewContentType(name); typ != "" {
		w.Header().Set("Content-Type", typ)
	}
	http.ServeContent(w, r, name, modTime, bytes.NewReader(data))
}
