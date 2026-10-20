// Command pidesigner runs the design workspace daemon: projects, files, chat
// runs driven by the locally installed Pi agent, and the live preview.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/akki/pidesigner/internal/api"
	"github.com/akki/pidesigner/internal/content"
	"github.com/akki/pidesigner/internal/pirun"
	"github.com/akki/pidesigner/internal/preview"
	"github.com/akki/pidesigner/internal/store"
)

func main() {
	var (
		port       = flag.Int("port", 7457, "HTTP port")
		host       = flag.String("host", "127.0.0.1", "bind address")
		dataDir    = flag.String("data", defaultDataDir(), "data directory")
		contentDir = flag.String("content", defaultContentDir(), "content catalog directory (design systems, skills, templates)")
		piBin      = flag.String("pi", "", "path to the pi executable (default: resolve from PATH)")
		uiDir      = flag.String("ui", defaultUIDir(), "built UI directory to serve")
	)
	flag.Parse()

	log.SetFlags(log.Ltime)

	if err := run(*port, *host, *dataDir, *contentDir, *piBin, *uiDir); err != nil {
		log.Fatal(err)
	}
}

func run(port int, host, dataDir, contentDir, piBin, uiDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	projectRoot := filepath.Join(dataDir, "projects")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return err
	}

	st, err := store.Open(filepath.Join(dataDir, "pidesigner.db"))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	cat, err := content.Open(contentDir)
	if err != nil {
		return fmt.Errorf("open content catalog: %w", err)
	}
	cat.Scan()

	resolvedPi, err := pirun.ResolveBin(piBin)
	if err != nil {
		log.Printf("warning: %v", err)
		log.Printf("warning: chat runs will fail until pi is installed or --pi is set")
	} else {
		log.Printf("pi agent: %s", resolvedPi)
	}

	hub := preview.NewHub()
	runner := api.NewRunner(st, cat, piBin, hub)
	srv := &api.Server{
		Store:       st,
		Catalog:     cat,
		Runner:      runner,
		Hub:         hub,
		ProjectRoot: projectRoot,
		PiBin:       resolvedPi,
	}

	// Resume watching existing projects.
	if projects, err := st.ListProjects(); err == nil {
		for _, p := range projects {
			hub.Watch(p.ID, p.Dir)
		}
	}

	// A daemon restart kills any in-flight agent runs; mark them failed so
	// the UI does not show them as running forever.
	if n, err := st.FailStaleRuns(); err == nil && n > 0 {
		log.Printf("marked %d stale run(s) as failed", n)
	}

	mux := srv.Routes()
	if uiDir != "" {
		if _, err := os.Stat(uiDir); err == nil {
			mux = withStatic(mux, uiDir)
			log.Printf("ui: serving %s", uiDir)
		}
	}

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: SSE streams stay open for the length of a run.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on http://%s:%d", host, port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	// Reap agent processes first, then drain HTTP.
	runner.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
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

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pidesigner"
	}
	return filepath.Join(home, ".pidesigner")
}

func defaultContentDir() string {
	exe, err := os.Executable()
	if err == nil {
		// Binaries live in <root>/bin or <root>; content lives in <root>/content.
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "content"),
			filepath.Join(filepath.Dir(dir), "content"),
			filepath.Join(dir, "..", "content"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return filepath.Clean(candidate)
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(wd, "content")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "content"
}

func defaultUIDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "ui/dist"
	}
	dir := filepath.Dir(exe)
	for _, candidate := range []string{
		filepath.Join(dir, "ui"),
		filepath.Join(filepath.Dir(dir), "ui", "dist"),
		filepath.Join(dir, "..", "ui", "dist"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}
	return ""
}
