package app

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/model"
)

func TestDataDirOwnershipProtectsLiveRunsAndDiscovery(t *testing.T) {
	opts := Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t)}
	first, err := Start(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Shutdown(context.Background()) })
	var created struct {
		Project model.Project `json:"project"`
	}
	if requestJSON(t, first.URL, "POST", "/api/projects", map[string]string{"name": "Live project"}, &created) != 201 {
		t.Fatal("create project")
	}
	run := model.Run{ID: "run-owned", ProjectID: created.Project.ID, Status: model.RunRunning, Prompt: "Still working", StartedAt: time.Now()}
	if err := first.Store.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	discovery := filepath.Join(opts.DataDir, "runtime.json")
	before, err := os.ReadFile(discovery)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Start(opts); err == nil {
		_ = second.Shutdown(context.Background())
		t.Fatal("second daemon acquired an owned workspace")
	}
	stored, err := first.Store.GetRun(run.ID)
	if err != nil || stored.Status != model.RunRunning {
		t.Fatalf("rejected startup changed live run: %+v, %v", stored, err)
	}
	after, err := os.ReadFile(discovery)
	if err != nil || string(after) != string(before) {
		t.Fatalf("rejected startup replaced discovery: %q, %v", after, err)
	}
	if err := first.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted, err := Start(opts)
	if err != nil {
		t.Fatalf("workspace lock survived completed shutdown: %v", err)
	}
	defer restarted.Shutdown(context.Background())
	stored, err = restarted.Store.GetRun(run.ID)
	if err != nil || stored.Status != model.RunFailed {
		t.Fatalf("exclusive restart did not recover genuinely stale run: %+v, %v", stored, err)
	}
}

func TestFailedStartupReleasesDataDirOwnership(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	opts := Options{DataDir: t.TempDir(), ContentDir: fixtureCatalog(t), Port: listener.Addr().(*net.TCPAddr).Port}
	if failed, err := Start(opts); err == nil {
		_ = failed.Shutdown(context.Background())
		t.Fatal("startup unexpectedly acquired occupied port")
	}
	opts.Port = 0
	a, err := Start(opts)
	if err != nil {
		t.Fatalf("failed startup kept workspace lock: %v", err)
	}
	defer a.Shutdown(context.Background())
}

func TestDaemonRejectsNonLoopbackBindings(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "192.0.2.1", "attacker.invalid"} {
		t.Run(host, func(t *testing.T) {
			a, err := Start(Options{Host: host, DataDir: t.TempDir(), ContentDir: fixtureCatalog(t)})
			if err == nil {
				_ = a.Shutdown(context.Background())
				t.Fatal("accepted a non-loopback bind address")
			}
		})
	}
}
