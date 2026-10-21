package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeDiscoveryFollowsAppRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	s := server{daemon: "http://127.0.0.1:7457", discovery: path}
	if s.baseURL() != "http://127.0.0.1:0" {
		t.Fatal("missing app must not connect to an unrelated daemon")
	}
	for _, url := range []string{"http://127.0.0.1:49100", "http://127.0.0.1:49200"} {
		if err := os.WriteFile(path, []byte(`{"url":"`+url+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := s.baseURL(); got != url {
			t.Fatalf("stale endpoint: %s", got)
		}
	}
	_ = os.WriteFile(path, []byte(`{"url":"https://example.com"}`), 0o600)
	if s.baseURL() != "http://127.0.0.1:0" {
		t.Fatal("non-loopback discovery accepted")
	}
}
