package pirun

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLivePiRPC is a live smoke test against the locally installed Pi agent.
// It is opt-in: set PIDESIGNER_LIVE_PI=1 to run it.
//
//	go test ./internal/pirun -run TestLivePiRPC -v
func TestLivePiRPC(t *testing.T) {
	if os.Getenv("PIDESIGNER_LIVE_PI") == "" {
		t.Skip("set PIDESIGNER_LIVE_PI=1 to run the live Pi RPC smoke test")
	}
	if _, err := ResolveBin(""); err != nil {
		t.Fatalf("resolve pi: %v", err)
	}

	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	sess, err := Start(ctx, Options{Dir: dir})
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

	disp, err := sess.Prompt(ctx, "Reply with exactly: PIPELINE_OK")
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if disp == "" {
		t.Fatalf("expected a disposition, got empty")
	}
	t.Logf("disposition=%s", disp)
	if disp == "handled" {
		t.Skip("prompt was handled by an input handler; no run started")
	}

	select {
	case aborted := <-settled:
		t.Logf("settled (aborted=%v)", aborted)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for agent_settled")
	}

	if !strings.Contains(text.String(), "PIPELINE_OK") {
		t.Fatalf("expected PIPELINE_OK in output, got %q", text.String())
	}
}
