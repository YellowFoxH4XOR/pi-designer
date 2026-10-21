package omprun

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveOmpRPC is a live smoke test against the locally installed omp agent.
// It is opt-in: set PIDESIGNER_LIVE_OMP=1 to run it.
//
//	go test ./internal/omprun -run TestLiveOmpRPC -v
func TestLiveOmpRPC(t *testing.T) {
	if os.Getenv("PIDESIGNER_LIVE_OMP") == "" {
		t.Skip("set PIDESIGNER_LIVE_OMP=1 to run the live omp RPC smoke test")
	}
	if _, err := ResolveBin(""); err != nil {
		t.Fatalf("resolve omp: %v", err)
	}

	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	sess, err := Start(ctx, Options{Dir: dir, Tools: []string{"read"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()

	var text strings.Builder
	settled := make(chan bool, 1)
	go func() {
		for ev := range sess.Events() {
			switch ev.Type {
			case EventTextDelta:
				text.WriteString(ev.Text)
			case EventSettled:
				settled <- ev.Aborted
			case EventError:
				t.Logf("event error: %s", ev.Text)
			}
		}
	}()

	started, err := sess.Prompt(ctx, "Reply with exactly: PIPELINE_OK")
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if !started {
		t.Fatal("omp finished an ordinary prompt locally; no run started")
	}

	select {
	case aborted := <-settled:
		t.Logf("settled (aborted=%v)", aborted)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for the session to settle")
	}

	if !strings.Contains(text.String(), "PIPELINE_OK") {
		t.Fatalf("expected PIPELINE_OK in output, got %q", text.String())
	}
}
