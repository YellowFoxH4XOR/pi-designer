package omprun

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// An opt-in, one-line inference check. Never runs under the ordinary test suite.
func TestLiveExplicitModelPrompt(t *testing.T) {
	ref := os.Getenv("PIDESIGNER_LIVE_MODEL")
	if ref == "" {
		t.Skip("set PIDESIGNER_LIVE_MODEL to a configured provider/model")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s, err := Start(ctx, Options{Dir: t.TempDir(), Model: ref, Thinking: "low",
		ExtraArgs: []string{"--no-skills", "--no-rules", "--no-lsp", "--no-tools"}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Model == nil || state.Model.Ref() != ref {
		t.Fatalf("wrong model: %+v", state.Model)
	}
	t.Logf("Selected exact model: %s; thinking=%s", state.Model.Ref(), state.ThinkingLevel)
	type result struct {
		text, failure string
		aborted       bool
	}
	finished := make(chan result, 1)
	go func() {
		var out strings.Builder
		failure := ""
		for e := range s.Events() {
			switch e.Type {
			case EventTextDelta:
				out.WriteString(e.Text)
			case EventError:
				failure = e.Text
			case EventModelResult:
				failure = ""
			case EventSettled:
				finished <- result{out.String(), failure, e.Aborted}
				return
			}
		}
		finished <- result{out.String(), "omp exited before settling: " + failure, false}
	}()
	if _, err := s.Prompt(ctx, "Reply with exactly MODEL_SELECTION_OK. Do not use tools."); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-finished:
		if r.failure != "" || r.aborted || !strings.Contains(r.text, "MODEL_SELECTION_OK") {
			t.Fatalf("model request failed: text=%q error=%s aborted=%v", r.text, r.failure, r.aborted)
		}
		t.Log("Selected model completed a real one-line inference")
	case <-ctx.Done():
		t.Fatal("selected model request timed out")
	}
}
