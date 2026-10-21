// Command pidesigner-mcp is a stdio MCP server that exposes the design
// workspace to any coding editor.
//
// The server holds no state and never touches the filesystem: every tool
// resolves to an HTTP call against the running daemon. That keeps the server
// safe to spawn from an editor and lets the daemon remain the single owner of
// project state.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	protocolVersion  = "2025-06-18"
	fallbackProtocol = "2024-11-05"
	defaultDaemonURL = "http://127.0.0.1:7457"
	serverName       = "pidesigner"
	serverVersion    = "0.1.0"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type server struct {
	daemon    string
	discovery string
	http      *http.Client
	out       *bufio.Writer
}

func main() {
	var (
		daemon    = flag.String("daemon", envOr("PIDESIGNER_DAEMON", defaultDaemonURL), "daemon base URL")
		discovery = flag.String("discovery", "", "runtime discovery file written by the native app")
		print     = flag.Bool("print-config", false, "print an MCP client config snippet and exit")
		client    = flag.String("client", "generic", "client to print a config for (generic|claude|cursor|codex|pi)")
	)
	flag.Parse()

	if *print {
		fmt.Print(configSnippet(*client, *daemon))
		return
	}

	s := &server{
		daemon:    strings.TrimRight(*daemon, "/"),
		discovery: *discovery,
		http:      &http.Client{Timeout: 120 * time.Second},
		out:       bufio.NewWriter(os.Stdout),
	}
	if err := s.serve(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "pidesigner-mcp:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// serve reads newline-delimited JSON-RPC messages from stdin.
func (s *server) serve(ctx context.Context) error {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := readLine(in)
		if len(line) > 0 {
			s.handle(ctx, line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// readLine reads one LF-terminated record without treating U+2028/U+2029 as
// boundaries.
func readLine(r *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		buf = append(buf, chunk...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return buf, err
		}
		break
	}
	return bytes.TrimRight(buf, "\r\n"), nil
}

func (s *server) handle(ctx context.Context, line []byte) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.write(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error: " + err.Error()}})
		return
	}
	// Notifications carry no id and expect no response.
	isNotification := len(req.ID) == 0

	switch req.Method {
	case "initialize":
		s.respond(req, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools":     map[string]any{"listChanged": false},
				"resources": map[string]any{"subscribe": false, "listChanged": false},
			},
			"serverInfo": map[string]any{"name": serverName, "version": serverVersion},
			"instructions": "pidesigner exposes a local-first design workspace. " +
				"Use list_design_systems + the design-system resources for brand context, " +
				"write_file for surgical edits, and start_run to delegate a full design to the Pi agent.",
		})
	case "notifications/initialized", "notifications/cancelled":
		// No response for notifications.
	case "ping":
		s.respond(req, map[string]any{})
	case "tools/list":
		s.respond(req, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		s.callTool(ctx, req)
	case "resources/list":
		s.respond(req, map[string]any{"resources": s.resources(ctx)})
	case "resources/read":
		s.readResource(ctx, req)
	default:
		if !isNotification {
			s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found: " + req.Method}})
		}
	}
}

func (s *server) respond(req rpcRequest, result any) {
	if len(req.ID) == 0 {
		return
	}
	s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func (s *server) write(resp rpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = s.out.Write(b)
	_ = s.out.WriteByte('\n')
	_ = s.out.Flush()
}

// --------------------------------------------------------------- tool layer

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func toolDefinitions() []toolDef {
	projArg := map[string]any{
		"type":        "string",
		"description": "Project id or name substring. Optional: defaults to the active project.",
	}
	return []toolDef{
		{
			Name:        "list_projects",
			Description: "List the available design projects.",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "get_project",
			Description: "Get a project's metadata, including its design system and entry file.",
			InputSchema: objectSchema(map[string]any{"project": projArg}),
		},
		{
			Name:        "list_files",
			Description: "List the files in a project.",
			InputSchema: objectSchema(map[string]any{"project": projArg}),
		},
		{
			Name:        "get_file",
			Description: "Read one project file by relative path.",
			InputSchema: objectSchema(map[string]any{
				"project": projArg,
				"path":    map[string]any{"type": "string", "description": "Project-relative file path."},
			}),
		},
		{
			Name:        "get_artifact",
			Description: "Get the artifact entry file plus every sibling asset it references, in one call. Prefer this over multiple get_file calls.",
			InputSchema: objectSchema(map[string]any{
				"project": projArg,
				"entry":   map[string]any{"type": "string", "description": "Entry file. Optional: defaults to the project's entry file."},
			}),
		},
		{
			Name:        "search_files",
			Description: "Find project files whose contents match a query (regex or literal).",
			InputSchema: objectSchema(map[string]any{
				"project": projArg,
				"query":   map[string]any{"type": "string", "description": "Search query."},
				"limit":   map[string]any{"type": "integer", "description": "Max results (default 100)."},
			}),
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a project file. Use this for surgical edits to an existing design.",
			InputSchema: objectSchema(map[string]any{
				"project": projArg,
				"path":    map[string]any{"type": "string", "description": "Project-relative file path."},
				"content": map[string]any{"type": "string", "description": "Full file content."},
			}),
		},
		{
			Name:        "delete_file",
			Description: "Delete a project file.",
			InputSchema: objectSchema(map[string]any{
				"project": projArg,
				"path":    map[string]any{"type": "string", "description": "Project-relative file path."},
			}),
		},
		{
			Name:        "list_models",
			Description: "List models configured in the installed Pi, including exact provider/id values. Credentials remain with Pi.",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "list_skills",
			Description: "List the available design skills (prototype, motion design, website clone, ...).",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "list_design_systems",
			Description: "List the available design systems (brand packages).",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "start_run",
			Description: "Delegate a design task to the Pi agent running in the project directory. Returns a run id; poll get_run. Prefer this for full generation.",
			InputSchema: objectSchema(map[string]any{
				"project":        projArg,
				"prompt":         map[string]any{"type": "string", "description": "The design brief."},
				"conversationId": map[string]any{"type": "string", "description": "Conversation to append the turn to."},
				"designSystemId": map[string]any{"type": "string", "description": "Design system to apply."},
				"skillId":        map[string]any{"type": "string", "description": "Skill to load (web-prototype, motion-design, web-clone, ...)."},
				"model":          map[string]any{"type": "string", "description": "Exact provider/model id from list_models. Omit to use the model saved in PiDesigner."},
			}),
		},
		{
			Name:        "get_run",
			Description: "Get a run's status.",
			InputSchema: objectSchema(map[string]any{
				"runId": map[string]any{"type": "string", "description": "Run id from start_run."},
			}),
		},
		{
			Name:        "cancel_run",
			Description: "Cancel an in-flight run.",
			InputSchema: objectSchema(map[string]any{
				"runId": map[string]any{"type": "string", "description": "Run id from start_run."},
			}),
		},
	}
}

func objectSchema(props map[string]any) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
	}
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *server) callTool(ctx context.Context, req rpcRequest) {
	var p toolCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.respond(req, toolError("invalid tool call params: "+err.Error()))
		return
	}
	args := map[string]any{}
	if len(p.Arguments) > 0 {
		_ = json.Unmarshal(p.Arguments, &args)
	}

	switch p.Name {
	case "list_projects":
		s.proxy(ctx, req, http.MethodGet, "/api/projects", nil)
	case "get_project":
		s.proxy(ctx, req, http.MethodGet, "/api/projects/"+argStr(args, "project"), nil)
	case "list_files":
		s.proxy(ctx, req, http.MethodGet, "/api/projects/"+argStr(args, "project")+"/files", nil)
	case "get_file":
		s.proxy(ctx, req, http.MethodGet, "/api/projects/"+argStr(args, "project")+"/file?path="+argStr(args, "path"), nil)
	case "get_artifact":
		s.proxy(ctx, req, http.MethodGet, "/api/projects/"+argStr(args, "project")+"/artifact?entry="+argStr(args, "entry"), nil)
	case "search_files":
		s.proxy(ctx, req, http.MethodGet, "/api/projects/"+argStr(args, "project")+"/search?q="+argStr(args, "query"), nil)
	case "write_file":
		s.proxy(ctx, req, http.MethodPut, "/api/projects/"+argStr(args, "project")+"/file", map[string]any{
			"path": argStr(args, "path"), "content": argStr(args, "content"),
		})
	case "delete_file":
		s.proxy(ctx, req, http.MethodDelete, "/api/projects/"+argStr(args, "project")+"/file?path="+argStr(args, "path"), nil)
	case "list_models":
		s.proxy(ctx, req, http.MethodGet, "/api/models", nil)
	case "list_skills":
		s.proxy(ctx, req, http.MethodGet, "/api/skills", nil)
	case "list_design_systems":
		s.proxy(ctx, req, http.MethodGet, "/api/design-systems", nil)
	case "start_run":
		s.proxy(ctx, req, http.MethodPost, "/api/projects/"+argStr(args, "project")+"/chat", map[string]any{
			"prompt":         argStr(args, "prompt"),
			"conversationId": argStr(args, "conversationId"),
			"designSystemId": argStr(args, "designSystemId"),
			"skillId":        argStr(args, "skillId"),
			"model":          argStr(args, "model"),
			// Editors cannot answer PiDesigner's question forms.
			"skipDiscovery": true,
		})
	case "get_run":
		s.proxy(ctx, req, http.MethodGet, "/api/runs/"+argStr(args, "runId"), nil)
	case "cancel_run":
		s.proxy(ctx, req, http.MethodPost, "/api/runs/"+argStr(args, "runId")+"/cancel", nil)
	default:
		s.respond(req, toolError("unknown tool: "+p.Name))
	}
}

// proxy forwards a call to the daemon and returns the body as a text result.
func (s *server) proxy(ctx context.Context, req rpcRequest, method, path string, body any) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			s.respond(req, toolError(err.Error()))
			return
		}
		reader = bytes.NewReader(b)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, s.baseURL()+path, reader)
	if err != nil {
		s.respond(req, toolError(err.Error()))
		return
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.http.Do(httpReq)
	if err != nil {
		s.respond(req, toolError("daemon unreachable at "+s.baseURL()+": "+err.Error()))
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		s.respond(req, toolError(fmt.Sprintf("daemon returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))))
		return
	}
	s.respond(req, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(raw)}},
		"isError": false,
	})
}

func toolError(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": msg}},
		"isError": true,
	}
}

func argStr(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------- resource layer

func (s *server) resources(ctx context.Context) []map[string]any {
	var out []map[string]any
	resp, err := s.http.Get(s.baseURL() + "/api/design-systems")
	if err == nil {
		defer resp.Body.Close()
		var payload struct {
			DesignSystems []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"designSystems"`
		}
		if json.NewDecoder(resp.Body).Decode(&payload) == nil {
			for _, ds := range payload.DesignSystems {
				out = append(out, map[string]any{
					"uri":         "od://design-systems/" + ds.ID + "/DESIGN.md",
					"name":        ds.ID + " design system",
					"description": "Brand contract: palette, typography, voice.",
					"mimeType":    "text/markdown",
				})
			}
		}
	}
	resp2, err := s.http.Get(s.baseURL() + "/api/skills")
	if err == nil {
		defer resp2.Body.Close()
		var payload struct {
			Skills []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"skills"`
		}
		if json.NewDecoder(resp2.Body).Decode(&payload) == nil {
			for _, sk := range payload.Skills {
				out = append(out, map[string]any{
					"uri":         "od://skills/" + sk.ID + "/SKILL.md",
					"name":        sk.ID + " skill",
					"description": sk.Description,
					"mimeType":    "text/markdown",
				})
			}
		}
	}
	return out
}

func (s *server) readResource(ctx context.Context, req rpcRequest) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.respond(req, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "invalid params"}})
		return
	}
	text := ""
	switch {
	case strings.HasPrefix(p.URI, "od://design-systems/") && strings.HasSuffix(p.URI, "/DESIGN.md"):
		id := strings.TrimSuffix(strings.TrimPrefix(p.URI, "od://design-systems/"), "/DESIGN.md")
		resp, err := s.http.Get(s.baseURL() + "/api/design-systems/" + id + "/context")
		if err == nil {
			defer resp.Body.Close()
			var ctxPayload struct {
				DesignMD string `json:"designMd"`
			}
			if json.NewDecoder(resp.Body).Decode(&ctxPayload) == nil {
				text = ctxPayload.DesignMD
			}
		}
	case strings.HasPrefix(p.URI, "od://skills/"):
		id := strings.TrimSuffix(strings.TrimPrefix(p.URI, "od://skills/"), "/SKILL.md")
		resp, err := s.http.Get(s.baseURL() + "/api/skills/" + id + "/document")
		if err == nil {
			defer resp.Body.Close()
			var payload struct {
				Content string `json:"content"`
			}
			if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&payload) == nil {
				text = payload.Content
			}
		}
	}
	if text == "" {
		s.respond(req, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "resource not found: " + p.URI}})
		return
	}
	s.respond(req, map[string]any{
		"contents": []map[string]any{{"uri": p.URI, "mimeType": "text/markdown", "text": text}},
	})
}

// ------------------------------------------------------------- client setup

func configSnippet(client, daemon string) string {
	cmd := "pidesigner-mcp"
	entry := map[string]any{
		"command": cmd,
		"args":    []string{"--daemon", daemon},
	}
	switch client {
	case "claude":
		return "Add to ~/.claude.json or .mcp.json:\n\n" + prettyJSON(map[string]any{
			"mcpServers": map[string]any{"pidesigner": entry},
		})
	case "cursor":
		return "Add to ~/.cursor/mcp.json:\n\n" + prettyJSON(map[string]any{
			"mcpServers": map[string]any{"pidesigner": entry},
		})
	case "codex":
		return "Add to ~/.codex/config.toml:\n\n[mcp_servers.pidesigner]\ncommand = \"" + cmd + "\"\nargs = [\"--daemon\", \"" + daemon + "\"]\n"
	case "pi":
		return "Add to ~/.pi/agent/mcp.json:\n\n" + prettyJSON(map[string]any{
			"mcpServers": map[string]any{"pidesigner": entry},
		})
	default:
		return prettyJSON(map[string]any{
			"mcpServers": map[string]any{"pidesigner": entry},
		})
	}
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b) + "\n"
}
