// Command pidesigner runs the design workspace daemon: projects, files, chat
// runs driven by the locally installed pi agent, and the live preview.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/akki/pidesigner/internal/app"
)

func main() {
	var (
		port       = flag.Int("port", 7457, "HTTP port (0 picks a free port)")
		host       = flag.String("host", "127.0.0.1", "bind address")
		dataDir    = flag.String("data", defaultDataDir(), "data directory")
		contentDir = flag.String("content", defaultContentDir(), "content catalog directory")
		piBin      = flag.String("pi", "", "path to the pi executable (default: resolve from PATH and pi install locations)")
		uiDir      = flag.String("ui", defaultUIDir(), "built UI directory to serve")
		ready      = flag.Bool("print-url", false, "print the serving URL to stdout as soon as it is bound")
		exitOnEOF  = flag.Bool("exit-on-stdin-close", false, "exit when stdin closes (parent-death guard for app shells)")
		renderer   = flag.String("renderer", "", "path to pidesigner-render (default: beside this executable)")
	)
	flag.Parse()

	log.SetFlags(log.Ltime)

	a, err := app.Start(app.Options{
		Host:       *host,
		Port:       *port,
		DataDir:    *dataDir,
		ContentDir: *contentDir,
		PiBin:      *piBin,
		UIDir:      *uiDir,
		Renderer:   *renderer,
	})
	if err != nil {
		log.Fatal(err)
	}

	if *ready {
		// A host process (such as the macOS shell) waits for this exact line.
		fmt.Printf("PIDESIGNER_READY %s\n", a.URL)
		_ = os.Stdout.Sync()
	}
	log.Printf("listening on %s", a.URL)
	log.Printf("pi agent: %s", a.PiBin())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// When a GUI shell owns this process, its stdin is a pipe. If the shell dies
	// (even by force-quit) the pipe closes, and we shut down instead of leaking
	// a daemon that keeps spawning agents.
	if *exitOnEOF {
		go func() {
			buf := make([]byte, 256)
			for {
				if _, err := os.Stdin.Read(buf); err != nil {
					log.Printf("stdin closed; exiting")
					stop()
					return
				}
			}
		}()
	}

	<-ctx.Done()

	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
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
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "content"),
			filepath.Join(filepath.Dir(dir), "content"),
			filepath.Join(dir, "..", "content"),
			filepath.Join(dir, "..", "Resources", "content"),
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
		return ""
	}
	dir := filepath.Dir(exe)
	for _, candidate := range []string{
		filepath.Join(dir, "ui"),
		filepath.Join(dir, "..", "Resources", "ui"),
		filepath.Join(filepath.Dir(dir), "ui", "dist"),
		filepath.Join(dir, "..", "ui", "dist"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}
	return ""
}
