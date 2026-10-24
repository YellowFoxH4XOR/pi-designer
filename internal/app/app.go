// Package app wires the daemon together and runs it.
//
// It exists so the daemon can be started in-process by other entrypoints (the
// macOS shell, tests) rather than only from cmd/pidesigner. Start binds a
// listener immediately and reports the URL it is serving on, which lets a host
// launch the UI at the right port even when PORT is 0.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/akki/pidesigner/internal/agentext"
	"github.com/akki/pidesigner/internal/api"
	"github.com/akki/pidesigner/internal/content"
	"github.com/akki/pidesigner/internal/httpguard"
	"github.com/akki/pidesigner/internal/knowledge"
	"github.com/akki/pidesigner/internal/pirun"
	"github.com/akki/pidesigner/internal/preview"
	"github.com/akki/pidesigner/internal/store"
	"github.com/akki/pidesigner/internal/versions"
)

// Options configure the daemon.
type Options struct {
	Host       string // bind address, default 127.0.0.1
	Port       int    // 0 picks a free port
	DataDir    string // SQLite + projects/
	ContentDir string // catalog
	PiBin      string // pi executable, empty resolves automatically
	UIDir      string // built UI directory, empty disables static serving
	Quiet      bool   // suppress request logging
	// Renderer is the pidesigner-render executable. Empty looks beside the
	// daemon executable; render_preview is disabled when none is found.
	Renderer string
}

// App is a running daemon.
type App struct {
	URL     string
	Server  *http.Server
	Store   *store.Store
	Catalog *content.Catalog
	Runner  *api.Runner
	Hub     *preview.Hub

	listener      net.Listener
	piBin         string
	discoveryPath string
	stopUpdates   context.CancelFunc
	updatesDone   chan struct{}
	dataLock      *os.File
}

// PiBin reports the resolved pi executable path, or empty when unresolved.
func (a *App) PiBin() string { return a.piBin }

// Start binds the listener and serves in the background.
func Start(opts Options) (*App, error) {
	host := opts.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("πDesigner must bind to localhost or a loopback IP address")
		}
	}
	if opts.DataDir == "" {
		return nil, errors.New("DataDir is required")
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return nil, err
	}
	dataLock, err := lockDataDir(opts.DataDir)
	if err != nil {
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			_ = dataLock.Close()
		}
	}()
	projectRoot := filepath.Join(opts.DataDir, "projects")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return nil, err
	}

	st, err := store.Open(filepath.Join(opts.DataDir, "pidesigner.db"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	cat, err := content.Open(opts.ContentDir)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("open content catalog: %w", err)
	}
	cat.Scan()
	updates, err := knowledge.Open(filepath.Join(opts.DataDir, "knowledge"))
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("open design references: %w", err)
	}
	cat.Knowledge = updates

	piRuntime := pirun.Detect(context.Background(), opts.PiBin)
	if piRuntime.Error != "" {
		log.Printf("warning: %s", piRuntime.Error)
	}

	hub := preview.NewHub()
	extension, err := agentext.Install(filepath.Join(opts.DataDir, "agent"))
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("install agent extension: %w", err)
	}
	renderer := resolveRenderer(opts.Renderer)
	if renderer == "" {
		log.Printf("warning: pidesigner-render not found; the agent will verify designs without rendering them")
	}
	models := pirun.NewCatalog(opts.PiBin)
	history := &versions.Store{Root: filepath.Join(opts.DataDir, "versions")}
	runner := api.NewRunner(st, cat, opts.PiBin, hub, api.RunnerConfig{
		SessionDir: filepath.Join(opts.DataDir, "sessions"),
		RenderDir:  filepath.Join(opts.DataDir, "renders"),
		Extension:  extension,
		Renderer:   renderer,
		Versions:   history,
	})
	srv := &api.Server{
		Store:       st,
		Catalog:     cat,
		Knowledge:   updates,
		Runner:      runner,
		Hub:         hub,
		ProjectRoot: projectRoot,
		PiExplicit:  opts.PiBin,
		Models:      models,
		Versions:    history,
	}
	srv.SetPiRuntime(piRuntime)

	// Resume watching existing projects.
	if projects, err := st.ListProjects(); err == nil {
		for _, p := range projects {
			hub.Watch(p.ID, p.Dir)
		}
	}
	// A restart terminated any in-flight agent; those runs can never finish.
	if n, err := st.FailStaleRuns(); err == nil && n > 0 {
		log.Printf("marked %d stale run(s) as failed", n)
	}

	mux := srv.Routes()
	if opts.UIDir != "" {
		if _, err := os.Stat(opts.UIDir); err == nil {
			mux = withStatic(mux, opts.UIDir)
		}
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(opts.Port)))
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}
	addr := listener.Addr().(*net.TCPAddr)

	httpSrv := &http.Server{
		Handler:           httpguard.Protect(mux),
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: SSE streams stay open for the length of a run.
	}

	a := &App{
		URL:           "http://" + net.JoinHostPort(host, strconv.Itoa(addr.Port)),
		Server:        httpSrv,
		Store:         st,
		Catalog:       cat,
		Runner:        runner,
		Hub:           hub,
		listener:      listener,
		piBin:         piRuntime.Bin,
		discoveryPath: filepath.Join(opts.DataDir, "runtime.json"),
		dataLock:      dataLock,
	}
	// A stable discovery file lets MCP follow the native app across random
	// loopback ports. It contains no credentials and is private to this user.
	payload, _ := json.Marshal(map[string]string{"url": a.URL})
	if err := os.WriteFile(a.discoveryPath, payload, 0o600); err != nil {
		listener.Close()
		st.Close()
		return nil, fmt.Errorf("write runtime discovery: %w", err)
	}

	updateCtx, stopUpdates := context.WithCancel(context.Background())
	a.stopUpdates = stopUpdates
	a.updatesDone = make(chan struct{})
	go func() {
		defer close(a.updatesDone)
		runReferenceUpdates(updateCtx, updates, 24*time.Hour)
	}()
	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()
	started = true

	return a, nil
}

// Shutdown reaps agent processes, then drains HTTP.
func (a *App) Shutdown(ctx context.Context) error {
	if a.stopUpdates != nil {
		a.stopUpdates()
		<-a.updatesDone
	}
	if b, err := os.ReadFile(a.discoveryPath); err == nil {
		var v struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(b, &v) == nil && v.URL == a.URL {
			_ = os.Remove(a.discoveryPath)
		}
	}
	a.Runner.Shutdown()
	drainErr := a.Server.Shutdown(ctx)
	if drainErr != nil {
		// A stream outlived the drain deadline. Force-close connections so no
		// handler touches SQLite after it is closed, then still release it.
		drainErr = errors.Join(drainErr, a.Server.Close())
	}
	return errors.Join(drainErr, a.Store.Close(), a.dataLock.Close())
}

// withStatic serves the built UI, falling back to index.html for SPA routes.
func withStatic(next http.Handler, dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// API and preview routes always belong to the daemon -- never let the
		// SPA fallback swallow them.
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/preview/") {
			next.ServeHTTP(w, r)
			return
		}
		if p == "/" {
			fs.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			// Unknown path -> SPA fallback.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fs.ServeHTTP(w, r2)
			return
		}
		fs.ServeHTTP(w, r)
	})
}

// rendererName is the bundled WebKit renderer, installed beside the daemon.
const rendererName = "pidesigner-render"

// resolveRenderer returns an executable renderer path, or "".
func resolveRenderer(explicit string) string {
	candidates := []string{explicit}
	if explicit == "" {
		if exe, err := os.Executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				exe = resolved
			}
			candidates = []string{filepath.Join(filepath.Dir(exe), rendererName)}
		}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return c
		}
	}
	return ""
}
