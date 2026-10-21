package omprun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeOmpCatalog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omp")
	script := `#!/bin/sh
provider=first
while [ "$#" -gt 0 ]; do
  case "$1" in --model) provider="${2%%/*}";shift 2;; *) shift;; esac
done
while IFS= read -r line; do
  id=$(printf '%s' "$line" | /usr/bin/sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
  case "$line" in
    *get_available_models*) printf '{"id":"%s","type":"response","success":true,"data":{"models":[{"provider":"first","id":"same-name","name":"First Model","reasoning":true,"headers":{"Authorization":"never-expose-this"}},{"provider":"second","id":"same-name","name":"Second Model","reasoning":false}]}}\n' "$id";;
    *get_state*) printf '{"id":"%s","type":"response","success":true,"data":{"model":{"provider":"%s","id":"same-name","name":"Selected Model"},"thinkingLevel":"medium"}}\n' "$id" "$provider";;
    *get_available_thinking_levels*)
       if [ "$provider" = second ]; then levels='["off"]';else levels='["low","medium","high"]';fi
       printf '{"id":"%s","type":"response","success":true,"data":{"levels":%s}}\n' "$id" "$levels";;
  esac
done
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfiguredModelDiscoveryAndExactProviderSelection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c := NewCatalog(fakeOmpCatalog(t))
	list, err := c.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Models) != 2 || list.Current.Ref() != "first/same-name" {
		t.Fatalf("bad discovery: %+v", list)
	}
	payload, _ := json.Marshal(list)
	if strings.Contains(string(payload), "Authorization") || strings.Contains(string(payload), "never-expose") {
		t.Fatal("raw model secrets escaped the allowlisted model fields")
	}
	// Both providers expose exactly the same id. This passes only when omp was
	// launched with the provider-qualified reference, not a bare model id.
	levels, err := c.Thinking(ctx, "second/same-name")
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 1 || levels[0] != "off" {
		t.Fatalf("wrong provider selected: %v", levels)
	}
	if _, err := c.Resolve(ctx, "unknown/model"); err == nil {
		t.Fatal("unknown model accepted")
	}
}

func TestPromptResultErrorIsTheAuthoritativeFailure(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	// A failed attempt omp still retries must not fail the run by itself.
	s.handleRecord([]byte(`{"type":"message_end","message":{"role":"assistant","stopReason":"error","errorMessage":"overloaded"}}`))
	if ev := <-s.events; ev.Type != "message_end" {
		t.Fatal("authoritative message was lost")
	}
	s.handleRecord([]byte(`{"type":"message_end","message":{"role":"assistant","stopReason":"stop"}}`))
	<-s.events // authoritative final message
	if ev := <-s.events; ev.Type != EventModelResult {
		t.Fatal("a successful retry must report a model result", ev)
	}
	s.handleRecord([]byte(`{"type":"prompt_result","id":"r2","agentInvoked":true,"status":"error","sessionSettled":true,"error":{"message":"Provider authentication failed","retryable":false}}`))
	ev := <-s.events
	if ev.Type != EventError || !strings.Contains(ev.Text, "authentication") {
		t.Fatalf("provider failure was silently ignored: %+v", ev)
	}
	if ev := <-s.events; ev.Type != EventSettled || ev.Aborted {
		t.Fatalf("errored prompt did not settle: %+v", ev)
	}
}

func TestLiveInstalledOmpModels(t *testing.T) {
	if os.Getenv("PIDESIGNER_LIVE_OMP") == "" {
		t.Skip("opt-in metadata-only test; no inference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c := NewCatalog("")
	list, err := c.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Models) == 0 || list.Current == nil {
		t.Fatal("installed omp did not return configured models")
	}
	t.Logf("omp returned %d configured models; default=%s", len(list.Models), list.Current.Ref())
	chosen := list.Models[0]
	if chosen.Ref() == list.Current.Ref() && len(list.Models) > 1 {
		chosen = list.Models[1]
	}
	levels, err := c.Thinking(ctx, chosen.Ref())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Verified exact model %s; supported thinking=%v", chosen.Ref(), levels)
	after, err := c.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if after.Current == nil || after.Current.Ref() != list.Current.Ref() {
		t.Fatal("model discovery changed the user's omp default")
	}
}
