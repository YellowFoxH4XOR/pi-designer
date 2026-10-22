package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akki/pidesigner/internal/api"
	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/store"
)

func newTestMCPServer(daemon string, client *http.Client) (*server, *bytes.Buffer) {
	var output bytes.Buffer
	return &server{
		daemon: daemon,
		http:   client,
		out:    bufio.NewWriter(&output),
	}, &output
}

func invokeMCP(t *testing.T, s *server, output *bytes.Buffer, request any) map[string]json.RawMessage {
	t.Helper()
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	s.handle(context.Background(), line)
	var response map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode serialized response %q: %v", output.String(), err)
	}
	return response
}

func resourceRequest(t *testing.T, params any) rpcRequest {
	t.Helper()
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return rpcRequest{JSONRPC: "2.0", ID: json.RawMessage("7"), Method: "resources/read", Params: b}
}

func assertRPCError(t *testing.T, response map[string]json.RawMessage, wantCode int) {
	t.Helper()
	if string(response["jsonrpc"]) != `"2.0"` || string(response["id"]) != "7" {
		t.Fatalf("invalid JSON-RPC error envelope: %v", response)
	}
	if _, ok := response["result"]; ok {
		t.Fatalf("error response unexpectedly contains result: %s", response["result"])
	}
	var got struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response["error"], &got); err != nil {
		t.Fatalf("decode top-level JSON-RPC error: %v (response %s)", err, response["error"])
	}
	if got.Code != wantCode {
		t.Fatalf("error code = %d, want %d (message %q)", got.Code, wantCode, got.Message)
	}
}

func TestResourcesReadJSONRPCErrors(t *testing.T) {
	t.Run("invalid params are protocol errors", func(t *testing.T) {
		s, output := newTestMCPServer("", http.DefaultClient)
		response := invokeMCP(t, s, output, resourceRequest(t, map[string]any{}))
		assertRPCError(t, response, -32602)
	})

	t.Run("unknown URI is a resource error", func(t *testing.T) {
		s, output := newTestMCPServer("", http.DefaultClient)
		response := invokeMCP(t, s, output, resourceRequest(t, map[string]any{"uri": "od://unknown/item"}))
		assertRPCError(t, response, -32002)
	})

	t.Run("daemon retrieval failure is an internal error", func(t *testing.T) {
		daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
		}))
		defer daemon.Close()
		s, output := newTestMCPServer(daemon.URL, daemon.Client())
		response := invokeMCP(t, s, output, resourceRequest(t, map[string]any{"uri": "od://skills/demo/SKILL.md"}))
		assertRPCError(t, response, -32603)
	})
}

func TestResourcesReadPreservesSuccessfulEmptyResource(t *testing.T) {
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/design-systems/brand/context" {
			t.Errorf("daemon path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"designMd":""}`))
	}))
	defer daemon.Close()
	s, output := newTestMCPServer(daemon.URL, daemon.Client())
	uri := "od://design-systems/brand/DESIGN.md"
	response := invokeMCP(t, s, output, resourceRequest(t, map[string]any{"uri": uri}))

	if _, ok := response["error"]; ok {
		t.Fatalf("successful resource returned an error: %s", response["error"])
	}
	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(response["result"], &result); err != nil {
		t.Fatalf("decode resource result: %v", err)
	}
	if len(result.Contents) != 1 || result.Contents[0].URI != uri || result.Contents[0].MIMEType != "text/markdown" || result.Contents[0].Text != "" {
		t.Fatalf("resource success = %+v", result.Contents)
	}
}

func TestSearchFilesForwardsLimitThroughDaemonHandler(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{"a.html", "b.html", "c.html"} {
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte("needle"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	db, err := store.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	if err := db.CreateProject(model.Project{
		ID: "project-1", Name: "Search fixture", Slug: "search-fixture", Dir: projectDir,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	daemon := httptest.NewServer((&api.Server{Store: db}).Routes())
	defer daemon.Close()
	s, output := newTestMCPServer(daemon.URL, daemon.Client())

	call := func(arguments map[string]any) struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} {
		t.Helper()
		output.Reset()
		args, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		params, err := json.Marshal(toolCallParams{Name: "search_files", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		response := invokeMCP(t, s, output, rpcRequest{
			JSONRPC: "2.0", ID: json.RawMessage("8"), Method: "tools/call", Params: params,
		})
		var result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(response["result"], &result); err != nil {
			t.Fatalf("decode tool result: %v (response %s)", err, response["result"])
		}
		return result
	}
	decodeFiles := func(result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}) []string {
		t.Helper()
		if result.IsError || len(result.Content) != 1 {
			t.Fatalf("search tool result = %+v", result)
		}
		var payload struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
			t.Fatalf("decode daemon search response %q: %v", result.Content[0].Text, err)
		}
		paths := make([]string, len(payload.Files))
		for i, file := range payload.Files {
			paths[i] = file.Path
		}
		return paths
	}

	limited := decodeFiles(call(map[string]any{"project": "project-1", "query": "needle", "limit": 1}))
	if len(limited) != 1 || limited[0] != "a.html" {
		t.Fatalf("limited search results = %v, want [a.html]", limited)
	}
	defaulted := decodeFiles(call(map[string]any{"project": "project-1", "query": "needle"}))
	if got := strings.Join(defaulted, ","); got != "a.html,b.html,c.html" {
		t.Fatalf("search without limit = %v, want all fixture matches", defaulted)
	}

	invalid := call(map[string]any{"project": "project-1", "query": "needle", "limit": "1"})
	if !invalid.IsError || len(invalid.Content) != 1 || !strings.Contains(invalid.Content[0].Text, "must be an integer") {
		t.Fatalf("invalid limit result = %+v", invalid)
	}
}
