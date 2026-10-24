// Package pirun drives the locally installed pi coding agent over its RPC mode.
//
// pi speaks JSONL over stdin/stdout: one JSON record per line, LF-terminated.
// This package owns the process lifecycle, correlates command responses by id,
// and turns the event stream into a typed channel.
//
// Protocol notes that the implementation depends on:
//   - stdout carries protocol records only; diagnostics go to stderr.
//   - a prompt response reports data.disposition: "started" begins a run,
//     "handled" finished locally (no agent_settled follows), "queued" waits
//     behind an active run.
//   - agent_settled {aborted} marks the end of all automatic work (retries,
//     compaction recovery, queued messages). agent_end alone does not.
//   - a provider failure is the final assistant message with stopReason
//     "error"; earlier errored messages may still be retried.
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

// Event is one item from pi's event stream.
type Event struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ContentIndex int             `json:"contentIndex,omitempty"`
	ToolName     string          `json:"toolName,omitempty"`
	ToolArgs     json.RawMessage `json:"toolArgs,omitempty"`
	// ToolDetails is a finished tool's structured result details.
	ToolDetails json.RawMessage `json:"toolDetails,omitempty"`
	IsError     bool            `json:"isError,omitempty"`
	Aborted     bool            `json:"aborted,omitempty"`
	ToolCallID  string          `json:"toolCallId,omitempty"`
	Output      string          `json:"output,omitempty"`
	DurationMS  int64           `json:"durationMs,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// Options configure a pi invocation.
type Options struct {
	// Bin is the pi executable. Empty means resolve from PATH and the known
	// install locations.
	Bin string
	// Dir is the working directory -- pi's file tools operate here.
	Dir string
	// Model is an exact provider/model reference, e.g.
	// "anthropic/claude-sonnet-4-5". Empty keeps pi's default (or, for a
	// continued session, the model it was saved with).
	Model string
	// Thinking is pi's thinking level: off|minimal|low|medium|high|xhigh|max.
	Thinking string
	// SystemPrompt is a file that replaces pi's default coding-assistant
	// system prompt. Tool definitions, skills and the working directory are
	// still added by pi.
	SystemPrompt string
	// SystemPromptFiles are files appended to the system prompt.
	SystemPromptFiles []string
	// ExtraArgs are appended to the argv after the options above.
	ExtraArgs []string
	// Env overrides the child environment. Nil inherits os.Environ().
	Env []string
	// SessionDir persists the session in this directory and continues the
	// newest session already stored there. Empty runs an ephemeral session.
	SessionDir string
	// Extensions are extension files loaded explicitly for this run.
	Extensions []string
}

// Session is a running pi process.
type Session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	events       chan Event
	stopDelivery chan struct{}
	stopOnce     sync.Once
	readers      sync.WaitGroup

	mu      sync.Mutex
	writeMu sync.Mutex
	pending map[string]chan *rpcResponse
	nextID  int
	closed  bool
	done    chan struct{}
	// stderrTail keeps the last stderr lines for error reporting.
	stderrTail []string

	// Reader-goroutine state: settle tracking.
	settledEmitted       bool
	providerError        string
	lastThinkingActivity time.Time
}

// rpcResponse is a pi command response record.
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
		// Runs start in project and temp directories, so a relative path
		// must be fixed against the daemon's own working directory.
		abs, err := filepath.Abs(explicit)
		if err == nil {
			if _, err = os.Stat(abs); err == nil {
				return abs, nil
			}
		}
		return "", fmt.Errorf("pi binary %q not found", explicit)
	}
	if p, err := exec.LookPath("pi"); err == nil {
		return p, nil
	}
	// Apps launched from Finder do not inherit a login-shell PATH. Look where
	// pi's installers put the binary.
	home, err := os.UserHomeDir()
	if err == nil {
		for _, candidate := range installCandidates(home) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", errors.New("pi executable not found on PATH (also looked in ~/.bun/bin, ~/.local/bin, ~/.pi/agent/bin and Homebrew)")
}

func installCandidates(home string) []string {
	return []string{
		filepath.Join(home, ".bun", "bin", "pi"),         // bun install -g
		filepath.Join(home, ".local", "bin", "pi"),       // npm prefix ~/.local
		filepath.Join(home, ".pi", "agent", "bin", "pi"), // pi installer
		"/opt/homebrew/bin/pi",                           // npm/Homebrew on Apple silicon
		"/usr/local/bin/pi",                              // npm/Homebrew on Intel
	}
}

// args builds the pi argv for opts. Every session ignores project-local pi
// resources (--no-approve): the agent writes the project folder, so its .pi
// settings, extensions and MCP servers must never load. MCP is off too, so
// the user's MCP servers never start inside design or metadata sessions.
func (opts Options) args() []string {
	args := []string{"--mode", "rpc", "--no-approve", "--no-mcp"}
	if opts.SessionDir == "" {
		args = append(args, "--no-session")
	} else {
		args = append(args, "--session-dir", opts.SessionDir, "--continue")
	}
	if opts.Model != "" {
		// An exact provider/id; callers verify the selection with get_state.
		args = append(args, "--model", opts.Model)
	}
	if opts.Thinking != "" {
		args = append(args, "--thinking", opts.Thinking)
	}
	for _, e := range opts.Extensions {
		args = append(args, "--extension", e)
	}
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	for _, f := range opts.SystemPromptFiles {
		args = append(args, "--append-system-prompt", f)
	}
	return append(args, opts.ExtraArgs...)
}

// Start launches pi in RPC mode.
func Start(ctx context.Context, opts Options) (*Session, error) {
	bin, err := ResolveBin(opts.Bin)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(bin, opts.args()...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	// Put pi in its own process group so the whole tree (pi plus any tools
	// it spawns) can be reaped together. Without this, killing the daemon
	// leaves orphaned agent processes running to completion.
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
		cmd:          cmd,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		events:       make(chan Event, 256),
		stopDelivery: make(chan struct{}),
		pending:      map[string]chan *rpcResponse{},
		done:         make(chan struct{}),
	}

	s.readers.Add(2)
	go s.readStdout()
	go s.readStderr()
	go func() {
		s.readers.Wait()
		close(s.events)
		_ = cmd.Wait() // wait only after both pipes have been drained
		close(s.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.CloseWith()
		case <-s.done:
		}
	}()

	return s, nil
}

// Events returns the event channel. It is closed when the session ends.
func (s *Session) Events() <-chan Event { return s.events }

// Done is closed when the pi process exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// readStdout parses the JSONL protocol stream.
func (s *Session) readStdout() {
	defer s.readers.Done()

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

// handleRecord dispatches one logical protocol record.
func (s *Session) handleRecord(line []byte) {
	// Peek at the record type cheaply.
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return // ignore unparseable noise
	}
	s.dispatch(line, probe.Type)
}

func (s *Session) dispatch(line []byte, kind string) {
	if kind == "response" {
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return
		}
		s.deliverResponse(&resp)
		return
	}
	s.handleEvent(line, kind)
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

// readStderr forwards diagnostics without treating them as protocol data.
func (s *Session) readStderr() {
	defer s.readers.Done()
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

// emit honors RPC backpressure; lifecycle events and deltas are never silently lost.
func (s *Session) emit(e Event) {
	select {
	case s.events <- e:
	case <-s.stopDelivery:
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

// Prompt sends a prompt and waits until pi has admitted it.
//
// started is false when pi finished the prompt locally (disposition
// "handled", e.g. an extension command): no agent_settled follows, so callers
// must not wait for EventSettled in that case.
func (s *Session) Prompt(ctx context.Context, text string) (started bool, err error) {
	_, ch, err := s.send("prompt", map[string]any{"message": text})
	if err != nil {
		return false, err
	}
	select {
	case resp := <-ch:
		if !resp.Success {
			return false, fmt.Errorf("prompt rejected: %s", resp.Error)
		}
		var data struct {
			Disposition string `json:"disposition"`
		}
		if len(resp.Data) > 0 {
			_ = json.Unmarshal(resp.Data, &data)
		}
		return data.Disposition != "handled", nil
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.done:
		return false, errors.New("pi exited before accepting the prompt")
	}
}

// Abort asks pi to stop the current run.
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
// the process if it lingers. stdout keeps being drained meanwhile, as pi
// requires before it exits.
func (s *Session) Close() {
	s.stopOnce.Do(func() { close(s.stopDelivery) })
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
		// pi exited on its own; reap anything it left in its group.
		killProcessGroup(s.cmd)
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
	s.stopOnce.Do(func() { close(s.stopDelivery) })
	killProcessGroup(s.cmd)
}
