package pirun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLivePiRPC is a live smoke test against the locally installed pi agent.
// It is opt-in: set PIDESIGNER_LIVE_PI=1 to run it.
//
//	go test ./internal/pirun -run TestLivePiRPC -v
func TestLivePiRPC(t *testing.T) {
	if os.Getenv("PIDESIGNER_LIVE_PI") == "" {
		t.Skip("set PIDESIGNER_LIVE_PI=1 to run the live pi RPC smoke test")
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

	started, err := sess.Prompt(ctx, "Reply with exactly: PIPELINE_OK")
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if !started {
		t.Fatal("pi finished an ordinary prompt locally; no run started")
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

func TestArgsIgnoreProjectResourcesAndMCP(t *testing.T) {
	got := strings.Join(Options{SessionDir: "/s", Model: "p/m", Thinking: "high", Extensions: []string{"/x.ts"}, SystemPrompt: "/designer.md", SystemPromptFiles: []string{"/sys.md"}}.args(), " ")
	want := "--mode rpc --no-approve --no-mcp --session-dir /s --continue --model p/m --thinking high --extension /x.ts --system-prompt /designer.md --append-system-prompt /sys.md"
	if got != want {
		t.Fatalf("args = %q\nwant  %q", got, want)
	}
	if got := strings.Join(Options{}.args(), " "); got != "--mode rpc --no-approve --no-mcp --no-session" {
		t.Fatalf("ephemeral args = %q", got)
	}
}

func TestVersionAtLeast(t *testing.T) {
	for _, c := range []struct {
		v    string
		want bool
	}{{"1.1.0", true}, {"1.4.0", true}, {"2.0.0", true}, {"1.0.9", false}, {"0.99.0", false}, {"1.1.0-beta", true}, {"1.1", false}, {"x", false}} {
		if got := VersionAtLeast(c.v, MinVersion); got != c.want {
			t.Errorf("VersionAtLeast(%q) = %v", c.v, got)
		}
	}
}

func TestDetectReadsVersionWithoutStartingASession(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old-pi")
	if err := os.WriteFile(old, []byte("#!/bin/sh\necho 1.0.2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if rt := Detect(context.Background(), old); rt.Version != "1.0.2" || rt.Supported || rt.Error == "" {
		t.Fatalf("old pi: %+v", rt)
	}
	current := filepath.Join(dir, "pi")
	if err := os.WriteFile(current, []byte("#!/bin/sh\necho 'pi 1.4.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if rt := Detect(context.Background(), current); rt.Version != "1.4.0" || !rt.Supported || rt.Error != "" || rt.Bin != current {
		t.Fatalf("current pi: %+v", rt)
	}
}
