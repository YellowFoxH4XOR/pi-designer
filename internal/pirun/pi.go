// Package pirun drives the locally installed Pi agent over its RPC mode.
//
// Pi speaks JSONL over stdin/stdout: one JSON record per line, LF-terminated.
// This package owns the process lifecycle, correlates command responses by id,
// and turns the event stream into a typed channel.
//
// Protocol notes that the implementation depends on:
//   - stdout carries protocol records only; diagnostics go to stderr.
//   - a prompt response's data.disposition may be "handled", meaning no run
//     started, so waiting for agent_settled would hang forever.
//   - agent_settled (not agent_end) marks the end of automatic work.
//   - records are framed by LF only; line-splitting must not treat U+2028 or
//     U+2029 as boundaries.
package pirun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Event types emitted on a Session's event channel.
const (
	EventAgentStart    = "agent_start"
	EventTextDelta     = "text_delta"
	EventThinkingDelta = "thinking_delta"
	EventToolStart     = "tool_start"
	EventToolEnd       = "tool_end"
	EventAgentEnd      = "agent_end"
	EventSettled       = "settled"
	EventStderr        = "stderr"
	EventError         = "error"
	EventModelResult   = "model_result"
)

// Event is one item from Pi's event stream.
type Event struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ToolName string          `json:"toolName,omitempty"`
	ToolArgs json.RawMessage `json:"toolArgs,omitempty"`
	// ToolDetails is a finished tool's structured result details.
	ToolDetails json.RawMessage `json:"toolDetails,omitempty"`
	IsError     bool            `json:"isError,omitempty"`
	Aborted     bool            `json:"aborted,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// Options configure a Pi invocation.
type Options struct {
	// Bin is the pi executable. Empty means resolve from PATH / ~/.pi.
	Bin string
	// Dir is the working directory -- Pi's Read/Write/Edit tools operate here.
	Dir string
	// Model is a model pattern, e.g. "anthropic/claude-sonnet-4-5".
	Model string
	// Thinking is Pi's thinking level: off|minimal|low|medium|high|xhigh|max.
	Thinking string
	// SkillPaths are skill files or directories to load for this run.
	SkillPaths []string
	// SystemPromptFiles are files appended to the system prompt.
	SystemPromptFiles []string
	// SystemPromptText is appended to the system prompt verbatim.
	SystemPromptText string
	// ExtraArgs are appended to the argv before the RPC mode flags.
	ExtraArgs []string
	// Env overrides the child environment. Nil inherits os.Environ().
	Env []string
	// Session enables session persistence (off by default: runs are ephemeral).
	Session bool
	// SessionDir is where sessions are persisted when Session is true.
	SessionDir string
	// SessionID opens or creates the exact session with this id. Requires
	// Session.
	SessionID string
	// Extensions are extension files loaded explicitly for this run.
	Extensions []string
	// AllowTools restricts the tool set, e.g. ["read","write","edit","bash"].
	AllowTools []string
	// Approve grants a one-time project-trust decision for the run. Automated
	// runs need this: Pi's default trust policy is "ask", which cannot be
	// answered over RPC.
	Approve bool
	// NoApprove explicitly denies project-trust.
	NoApprove bool
}

// Session is a running Pi process.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	events chan Event

	mu      sync.Mutex
	writeMu sync.Mutex
	pending map[string]chan *rpcResponse
	nextID  int
	closed  bool
	done    chan struct{}
	// stderrTail keeps the last stderr lines for error reporting.
	stderrTail []string
}

// rpcResponse is a Pi command response record.
type rpcResponse struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

// ResolveBin locates the pi executable.
func ResolveBin(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("pi binary %q not found", explicit)
	}
	if p, err := exec.LookPath("pi"); err == nil {
		return p, nil
	}
	// Pi's own installer keeps the binary here.
	home, err := os.UserHomeDir()
	if err == nil {
		candidate := filepath.Join(home, ".pi", "agent", "bin", "pi")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("pi executable not found on PATH (looked for ~/.pi/agent/bin/pi)")
}

// Start launches Pi in RPC mode.
func Start(ctx context.Context, opts Options) (*Session, error) {
	bin, err := ResolveBin(opts.Bin)
	if err != nil {
		return nil, err
	}

	args := []string{"--mode", "rpc"}
	if !opts.Session {
		args = append(args, "--no-session")
	} else {
		if opts.SessionDir != "" {
			args = append(args, "--session-dir", opts.SessionDir)
		}
		if opts.SessionID != "" {
			args = append(args, "--session-id", opts.SessionID)
		}
	}
	if opts.Model != "" {
		// Separate provider and model id so duplicate names across providers
		// cannot resolve through a fuzzy CLI match.
		parts := strings.SplitN(opts.Model, "/", 2)
		if len(parts) == 2 {
			args = append(args, "--provider", parts[0], "--model", parts[1])
		} else {
			args = append(args, "--model", opts.Model)
		}
	}
	if opts.Thinking != "" {
		args = append(args, "--thinking", opts.Thinking)
	}
	for _, s := range opts.SkillPaths {
		args = append(args, "--skill", s)
	}
	for _, e := range opts.Extensions {
		args = append(args, "--extension", e)
	}
	for _, f := range opts.SystemPromptFiles {
		args = append(args, "--append-system-prompt", f)
	}
	if opts.SystemPromptText != "" {
		args = append(args, "--append-system-prompt", opts.SystemPromptText)
	}
	if len(opts.AllowTools) > 0 {
		args = append(args, "--tools", strings.Join(opts.AllowTools, ","))
	}
	if opts.Approve {
		args = append(args, "--approve")
	}
	if opts.NoApprove {
		args = append(args, "--no-approve")
	}
	args = append(args, opts.ExtraArgs...)

	cmd := exec.Command(bin, args...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	// Put Pi in its own process group so the whole tree (Pi plus any tools it
	// spawns) can be reaped together. Without this, killing the daemon leaves
	// orphaned agent processes running to completion.
	setProcessGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start pi: %w", err)
	}

	s := &Session{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		events:  make(chan Event, 256),
		pending: map[string]chan *rpcResponse{},
		done:    make(chan struct{}),
	}

	go s.readStdout()
	go s.readStderr()
	go func() {
		select {
		case <-ctx.Done():
			s.CloseWith()
		case <-s.done:
		}
	}()
	go func() {
		_ = cmd.Wait()
		close(s.done)
	}()

	return s, nil
}

// Events returns the event channel. It is closed when the session ends.
func (s *Session) Events() <-chan Event { return s.events }

// Done is closed when the Pi process exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// readStdout parses the JSONL protocol stream.
func (s *Session) readStdout() {
	defer close(s.events)

	// bufio.Reader (not Scanner) so long records are not capped at 64KB.
	br := bufio.NewReaderSize(s.stdout, 1<<16)
	for {
		line, err := readLine(br)
		if len(line) > 0 {
			s.handleRecord(line)
		}
		if err != nil {
			if err != io.EOF {
				s.emit(Event{Type: EventError, Text: "stdout read: " + err.Error()})
			}
			return
		}
	}
}

// readLine reads one LF-terminated record. It tolerates CRLF and does not
// split on U+2028 / U+2029.
func readLine(br *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := br.ReadSlice('\n')
		buf = append(buf, chunk...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return buf, err
		}
		break
	}
	return trimCR(buf), nil
}

func trimCR(b []byte) []byte {
	b = bytesTrimSuffix(b, '\n')
	b = bytesTrimSuffix(b, '\r')
	return b
}

func bytesTrimSuffix(b []byte, c byte) []byte {
	if len(b) > 0 && b[len(b)-1] == c {
		return b[:len(b)-1]
	}
	return b
}

// handleRecord dispatches one protocol record.
func (s *Session) handleRecord(line []byte) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return
	}
	// Peek at the record type cheaply.
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return // ignore unparseable noise
	}

	if probe.Type == "response" {
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return
		}
		s.deliverResponse(&resp)
		return
	}

	s.handleEvent(line, probe.Type)
}

// deliverResponse hands a response to whoever is waiting on its id.
func (s *Session) deliverResponse(resp *rpcResponse) {
	s.mu.Lock()
	ch, ok := s.pending[resp.ID]
	if ok {
		delete(s.pending, resp.ID)
	}
	s.mu.Unlock()
	if ok {
		ch <- resp
		close(ch)
	}
}

// handleEvent converts an event record into a typed Event.
func (s *Session) handleEvent(line []byte, recType string) {
	switch recType {
	case "agent_start":
		s.emit(Event{Type: EventAgentStart})
	case "agent_end":
		s.emit(Event{Type: EventAgentEnd})
	case "agent_settled":
		var e struct {
			Aborted bool `json:"aborted"`
		}
		_ = json.Unmarshal(line, &e)
		s.emit(Event{Type: EventSettled, Aborted: e.Aborted})
	case "message_update":
		var e struct {
			AssistantMessageEvent struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			} `json:"assistantMessageEvent"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			return
		}
		switch e.AssistantMessageEvent.Type {
		case "text_delta":
			s.emit(Event{Type: EventTextDelta, Text: e.AssistantMessageEvent.Delta})
		case "thinking_delta":
			s.emit(Event{Type: EventThinkingDelta, Text: e.AssistantMessageEvent.Delta})
		}
	case "message_end":
		var e struct {
			Message struct {
				Role         string `json:"role"`
				StopReason   string `json:"stopReason"`
				ErrorMessage string `json:"errorMessage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) == nil && e.Message.Role == "assistant" && e.Message.StopReason == "error" {
			message := e.Message.ErrorMessage
			if message == "" {
				message = "The selected Pi model could not complete its request. Check its authentication and availability in Pi."
			}
			s.emit(Event{Type: EventError, Text: message})
		} else if e.Message.Role == "assistant" {
			s.emit(Event{Type: EventModelResult})
		}
	case "tool_execution_start":
		var e struct {
			ToolName string          `json:"toolName"`
			Args     json.RawMessage `json:"args"`
		}
		_ = json.Unmarshal(line, &e)
		s.emit(Event{Type: EventToolStart, ToolName: e.ToolName, ToolArgs: e.Args})
	case "tool_execution_end":
		var e struct {
			ToolName string `json:"toolName"`
			IsError  bool   `json:"isError"`
			Result   struct {
				Details json.RawMessage `json:"details"`
			} `json:"result"`
		}
		_ = json.Unmarshal(line, &e)
		s.emit(Event{Type: EventToolEnd, ToolName: e.ToolName, IsError: e.IsError, ToolDetails: e.Result.Details})
	}
}

// readStderr forwards diagnostics without treating them as protocol data.
func (s *Session) readStderr() {
	br := bufio.NewReaderSize(s.stderr, 1<<16)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			txt := strings.TrimRight(line, "\r\n")
			s.mu.Lock()
			s.stderrTail = append(s.stderrTail, txt)
			if len(s.stderrTail) > 40 {
				s.stderrTail = s.stderrTail[len(s.stderrTail)-40:]
			}
			s.mu.Unlock()
			s.emit(Event{Type: EventStderr, Text: txt})
		}
		if err != nil {
			return
		}
	}
}

// emit pushes an event without blocking the reader goroutine indefinitely.
func (s *Session) emit(e Event) {
	defer func() { _ = recover() }() // channel may be closed concurrently
	select {
	case s.events <- e:
	default:
		// Drop the event rather than stalling Pi's stdout reader. Dropping
		// deltas is preferable to blocking the agent.
	}
}

// StderrTail returns recent stderr output, for error messages.
func (s *Session) StderrTail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.stderrTail, "\n")
}

// send writes a command record and returns its id.
func (s *Session) send(cmdType string, extra map[string]any) (string, chan *rpcResponse, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", nil, errors.New("session closed")
	}
	s.nextID++
	id := fmt.Sprintf("r%d", s.nextID)
	ch := make(chan *rpcResponse, 1)
	s.pending[id] = ch
	s.mu.Unlock()

	rec := map[string]any{"id": id, "type": cmdType}
	for k, v := range extra {
		rec[k] = v
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", nil, err
	}
	b = append(b, '\n')
	s.writeMu.Lock()
	_, writeErr := s.stdin.Write(b)
	s.writeMu.Unlock()
	if err := writeErr; err != nil {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return "", nil, fmt.Errorf("write to pi: %w", err)
	}
	return id, ch, nil
}

// Prompt sends a prompt and waits until Pi has accepted it.
//
// The returned disposition mirrors Pi's own vocabulary: "started", "queued"
// or "handled". "handled" means an input handler consumed the prompt and no
// run started -- callers must not wait for agent_settled in that case.
func (s *Session) Prompt(ctx context.Context, text string) (string, error) {
	_, ch, err := s.send("prompt", map[string]any{"message": text})
	if err != nil {
		return "", err
	}
	select {
	case resp := <-ch:
		if !resp.Success {
			return "", fmt.Errorf("prompt rejected: %s", resp.Error)
		}
		var data struct {
			Disposition string `json:"disposition"`
		}
		if len(resp.Data) > 0 {
			_ = json.Unmarshal(resp.Data, &data)
		}
		return data.Disposition, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-s.done:
		return "", errors.New("pi exited before accepting the prompt")
	}
}

// Abort asks Pi to stop the current run.
func (s *Session) Abort(ctx context.Context) error {
	_, ch, err := s.send("abort", nil)
	if err != nil {
		return err
	}
	select {
	case resp := <-ch:
		if !resp.Success {
			return fmt.Errorf("abort failed: %s", resp.Error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errors.New("pi exited")
	}
}

// Close shuts the session down: close stdin for an orderly exit, then kill
// the process if it lingers.
func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	_ = s.stdin.Close()

	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		killProcessGroup(s.cmd)
		select {
		case <-s.done:
		case <-time.After(1 * time.Second):
		}
	}
}

// CloseWith kills the process group immediately.
func (s *Session) CloseWith() {
	killProcessGroup(s.cmd)
}
