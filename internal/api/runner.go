package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/akki/pidesigner/internal/content"
	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/pirun"
	"github.com/akki/pidesigner/internal/project"
	"github.com/akki/pidesigner/internal/prompt"
	"github.com/akki/pidesigner/internal/store"
)

// AgentEvent is one event in a run's event stream.
type AgentEvent struct {
	Type     string          `json:"type"`
	RunID    string          `json:"runId"`
	Text     string          `json:"text,omitempty"`
	Tool     string          `json:"tool,omitempty"`
	Args     json.RawMessage `json:"args,omitempty"`
	IsError  bool            `json:"isError,omitempty"`
	Aborted  bool            `json:"aborted,omitempty"`
	Todos    []model.Todo    `json:"todos,omitempty"`
	Render   *model.Render   `json:"render,omitempty"`
	Critique *model.Critique `json:"critique,omitempty"`
	At       int64           `json:"at"`
}

// broker fans a run's events out to any number of SSE subscribers.
type broker struct {
	mu      sync.Mutex
	subs    map[chan AgentEvent]struct{}
	closed  bool
	history []AgentEvent
}

func newBroker() *broker { return &broker{subs: map[chan AgentEvent]struct{}{}} }

func (b *broker) subscribe() chan AgentEvent {
	ch := make(chan AgentEvent, 2048)
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range b.history {
		ch <- e
	}
	if b.closed {
		close(ch)
	} else {
		b.subs[ch] = struct{}{}
	}
	return ch
}

func (b *broker) unsubscribe(ch chan AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}

func (b *broker) publish(e AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.history = append(b.history, e)
	if len(b.history) > 1024 {
		b.history = b.history[len(b.history)-1024:]
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default: // slow subscriber: drop rather than stall the agent
		}
	}
}

func (b *broker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = map[chan AgentEvent]struct{}{}
}

// Runner owns agent sessions and run lifecycles.
type Runner struct {
	store   *store.Store
	catalog *content.Catalog
	piBin   string
	cfg     RunnerConfig

	mu       sync.Mutex
	sessions map[string]*pirun.Session
	brokers  map[string]*broker
	active   map[string]string
	hub      FileChangeNotifier
}

// RunnerConfig locates the agent's persistent state and design tools.
type RunnerConfig struct {
	// SessionDir stores one native Pi session per conversation.
	SessionDir string
	// RenderDir stores preview renders, one subdirectory per project.
	RenderDir string
	// Extension is PiDesigner's Pi extension (todo_write, render_preview).
	Extension string
	// Renderer is the pidesigner-render executable; empty disables
	// render_preview.
	Renderer string
}

// FileChangeNotifier is notified when a run mutates project files.
type FileChangeNotifier interface {
	NotifyProjectChanged(projectID string)
}

// NewRunner creates a Runner.
func NewRunner(st *store.Store, cat *content.Catalog, piBin string, hub FileChangeNotifier, cfg RunnerConfig) *Runner {
	return &Runner{
		store:    st,
		catalog:  cat,
		piBin:    piBin,
		cfg:      cfg,
		sessions: map[string]*pirun.Session{},
		brokers:  map[string]*broker{},
		active:   map[string]string{},
		hub:      hub,
	}
}

// RunRequest describes a generation request.
type RunRequest struct {
	ProjectID      string `json:"projectId"`
	ConversationID string `json:"conversationId,omitempty"`
	Prompt         string `json:"prompt"`
	DesignSystemID string `json:"designSystemId,omitempty"`
	SkillID        string `json:"skillId,omitempty"`
	TemplateID     string `json:"templateId,omitempty"`
	Model          string `json:"model,omitempty"`
	Thinking       string `json:"thinking,omitempty"`
	// Viewport is the preview frame the user is looking at.
	Viewport string `json:"viewport,omitempty"`
	// SkipDiscovery is set by clients that cannot answer question forms.
	SkipDiscovery bool `json:"skipDiscovery,omitempty"`
}

// RunHandle is a started run.
type RunHandle struct {
	RunID  string
	Broker *broker
}

// Start begins a run against a project.
func (r *Runner) Start(ctx context.Context, req RunRequest) (*RunHandle, error) {
	proj, err := r.store.GetProject(req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is empty")
	}
	if err := pirun.ValidateRef(req.Model); err != nil {
		return nil, err
	}
	if req.ConversationID != "" {
		conv, err := r.store.GetConversation(req.ConversationID)
		if err != nil || conv.ProjectID != proj.ID {
			return nil, fmt.Errorf("conversation does not belong to this project")
		}
	}
	if req.SkillID == "" {
		req.SkillID = proj.SkillID
	}

	now := time.Now()
	run := model.Run{
		ID:             newID("run"),
		ProjectID:      proj.ID,
		ConversationID: req.ConversationID,
		Status:         model.RunRunning,
		Prompt:         req.Prompt,
		StartedAt:      now,
	}
	r.mu.Lock()
	if r.active[proj.ID] != "" {
		r.mu.Unlock()
		return nil, fmt.Errorf("Pi is already working in this project")
	}
	r.active[proj.ID] = run.ID
	r.mu.Unlock()
	if err := r.store.CreateRun(run); err != nil {
		r.mu.Lock()
		delete(r.active, proj.ID)
		r.mu.Unlock()
		return nil, err
	}

	b := newBroker()
	r.mu.Lock()
	r.brokers[run.ID] = b
	r.mu.Unlock()

	// Record the user's turn.
	if req.ConversationID != "" {
		_ = r.store.AppendMessage(model.Message{
			ID: newID("msg"), ConversationID: req.ConversationID,
			Role: model.RoleUser, Content: req.Prompt, CreatedAt: now,
		})
	}

	go r.execute(run, proj, req, b)
	return &RunHandle{RunID: run.ID, Broker: b}, nil
}

// execute runs the agent and streams its events.
func (r *Runner) execute(run model.Run, proj model.Project, req RunRequest, b *broker) {
	defer func() {
		b.close()
		r.mu.Lock()
		delete(r.brokers, run.ID)
		delete(r.active, proj.ID)
		r.mu.Unlock()
	}()

	pm := project.New(proj.Dir)

	opts := pirun.Options{
		Bin:      r.piBin,
		Dir:      proj.Dir,
		Model:    req.Model,
		Thinking: req.Thinking,
		// Automated runs cannot answer Pi's project-trust prompt over RPC.
		Approve: true,
		Env:     append(os.Environ(), "PIDESIGNER_RENDER_DIR="+filepath.Join(r.cfg.RenderDir, proj.ID)),
	}
	if r.cfg.Extension != "" {
		opts.Extensions = []string{r.cfg.Extension}
	}
	if r.cfg.Renderer != "" {
		opts.Env = append(opts.Env, "PIDESIGNER_RENDERER="+r.cfg.Renderer)
	}

	turn := prompt.TurnInput{Prompt: req.Prompt, Viewport: req.Viewport, SkipDiscovery: req.SkipDiscovery}
	// Each conversation is one native Pi session, so follow-up turns keep the
	// agent's full context: its plan, the files it read, its renders. A
	// conversation that predates native sessions gets its transcript replayed
	// once; the session it starts carries the history from then on.
	if req.ConversationID != "" && r.cfg.SessionDir != "" {
		opts.Session = true
		opts.SessionDir = r.cfg.SessionDir
		opts.SessionID = req.ConversationID
		if !r.hasSession(req.ConversationID) {
			turn.Transcript = r.priorTranscript(req.ConversationID)
		}
	}

	systemFile, cleanup, err := r.writeSystemPrompt(proj, req)
	if err != nil {
		r.store.FinishRun(run.ID, model.RunFailed, err.Error())
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: err.Error(), At: nowMs()})
		return
	}
	defer cleanup()
	opts.SystemPromptFiles = []string{systemFile}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, err := pirun.Start(ctx, opts)
	if err != nil {
		r.store.FinishRun(run.ID, model.RunFailed, err.Error())
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: err.Error(), At: nowMs()})
		return
	}
	r.mu.Lock()
	r.sessions[run.ID] = sess
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.sessions, run.ID)
		r.mu.Unlock()
		sess.Close()
	}()

	// Confirm the installed Pi selected the exact requested provider/model.
	// CLI argument parsing must never silently substitute a similarly named one.
	checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
	state, stateErr := sess.State(checkCtx)
	checkCancel()
	if stateErr == nil && state.Model == nil {
		stateErr = fmt.Errorf("Pi has no model selected. Open Pi to configure a model first")
	}
	if stateErr == nil && req.Model != "" && state.Model.Ref() != req.Model {
		stateErr = fmt.Errorf("Pi selected %s instead of %s. Choose a model from the available list", state.Model.Ref(), req.Model)
	}
	if stateErr != nil {
		r.store.FinishRun(run.ID, model.RunFailed, stateErr.Error())
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: stateErr.Error(), At: nowMs()})
		return
	}
	b.publish(AgentEvent{Type: "model", RunID: run.ID, Text: state.Model.Name + " · " + state.Model.Provider, At: nowMs()})

	// Subscribe before prompting so a fast completion is not missed.
	events := sess.Events()
	settled := make(chan bool, 1)
	processGone := make(chan struct{})
	var assistantText strings.Builder
	var providerFailure string
	work := &runWork{}

	go func() {
		defer close(processGone)
		for ev := range events {
			switch ev.Type {
			case pirun.EventTextDelta:
				assistantText.WriteString(ev.Text)
				b.publish(AgentEvent{Type: "text", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventAgentEnd, pirun.EventModelResult:
				if ev.Type == pirun.EventModelResult {
					// A successful retry clears a previous transient provider error.
					providerFailure = ""
				}
				// A run spans several assistant messages separated by tool
				// calls. Separate them so the transcript reads as paragraphs
				// instead of running together mid-sentence.
				if s := assistantText.String(); s != "" && !strings.HasSuffix(s, "\n\n") {
					assistantText.WriteString("\n\n")
					b.publish(AgentEvent{Type: "turn", RunID: run.ID, At: nowMs()})
				}
			case pirun.EventToolStart:
				b.publish(AgentEvent{Type: "tool", RunID: run.ID, Tool: ev.ToolName, Args: ev.ToolArgs, At: nowMs()})
				if ev.ToolName == "todo_write" {
					if todos, ok := parseTodos(ev.ToolArgs); ok {
						work.setTodos(todos)
						b.publish(AgentEvent{Type: "todos", RunID: run.ID, Todos: todos, At: nowMs()})
					}
				}
			case pirun.EventToolEnd:
				b.publish(AgentEvent{Type: "tool_end", RunID: run.ID, Tool: ev.ToolName, IsError: ev.IsError, At: nowMs()})
				// Tool calls mutate the project; nudge preview watchers.
				if ev.ToolName == "write" || ev.ToolName == "edit" || ev.ToolName == "multi_edit" {
					r.hub.NotifyProjectChanged(proj.ID)
				}
				if ev.ToolName == "render_preview" && !ev.IsError {
					if render, ok := parseRender(proj.ID, ev.ToolDetails); ok {
						work.addRender(render)
						b.publish(AgentEvent{Type: "render", RunID: run.ID, Render: &render, At: nowMs()})
					}
				}
				if ev.ToolName == "submit_critique" && !ev.IsError {
					var c model.Critique
					if json.Unmarshal(ev.ToolDetails, &c) == nil && c.Round > 0 {
						work.addCritique(c)
						b.publish(AgentEvent{Type: "critique", RunID: run.ID, Critique: &c, At: nowMs()})
					}
				}
			case pirun.EventStderr:
				b.publish(AgentEvent{Type: "stderr", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventError:
				providerFailure = ev.Text
				b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventSettled:
				settled <- ev.Aborted
				return
			}
		}
	}()

	disposition, err := sess.Prompt(ctx, prompt.Turn(turn))
	if err != nil {
		r.store.FinishRun(run.ID, model.RunFailed, err.Error())
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: err.Error(), At: nowMs()})
		return
	}

	// "handled" means an input handler consumed the prompt -- no run started,
	// so waiting for agent_settled would hang.
	if disposition == "handled" {
		r.store.FinishRun(run.ID, model.RunSucceeded, "")
		b.publish(AgentEvent{Type: "done", RunID: run.ID, At: nowMs()})
		return
	}

	// Prefer a settled signal if one already landed: the process-exit path
	// may become ready at the same moment.
	var (
		aborted bool
		failure string
	)
	select {
	case aborted = <-settled:
	default:
		select {
		case aborted = <-settled:
		case <-processGone:
			// Pi exited without emitting agent_settled -- surface diagnostics.
			select {
			case aborted = <-settled:
			default:
				failure = "Pi exited before completing the run"
				if tail := strings.TrimSpace(sess.StderrTail()); tail != "" {
					failure += ": " + tail
				}
			}
		case <-time.After(2 * time.Hour):
			failure = "timeout"
		}
	}

	if failure == "" && !aborted {
		failure = providerFailure
	}

	// Persist the plan, renders, critique rounds, and the assistant turn.
	w := work.snapshot()
	_ = r.store.SaveRunWork(run.ID, w.todos, w.renders, w.critiques)
	if txt := strings.TrimSpace(assistantText.String()); txt != "" && run.ConversationID != "" {
		_ = r.store.AppendMessage(model.Message{
			ID: newID("msg"), ConversationID: run.ConversationID,
			Role: model.RoleAssistant, Content: txt, CreatedAt: time.Now(),
		})
	}

	// Keep the project's entry file fresh.
	if entry := pm.FindEntryFile(); entry != "" && entry != proj.EntryFile {
		proj.EntryFile = entry
		_ = r.store.UpdateProject(proj)
	}
	r.hub.NotifyProjectChanged(proj.ID)
	// Publish completion only after the transcript and entry metadata are
	// durable, so a reconnect sees the same result as the streaming client.
	if failure != "" {
		r.store.FinishRun(run.ID, model.RunFailed, failure)
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: failure, At: nowMs()})
	} else {
		finishRun(r, run, b, aborted)
	}
}

// Shutdown cancels every in-flight run so agent processes are reaped before
// the daemon exits.
func (r *Runner) Shutdown() {
	r.mu.Lock()
	sessions := make([]*pirun.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.mu.Unlock()

	for _, s := range sessions {
		s.CloseWith()
	}
}

// Cancel aborts an in-flight run.
func (r *Runner) Cancel(runID string) error {
	r.mu.Lock()
	sess, ok := r.sessions[runID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("run %s is not active", runID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return sess.Abort(ctx)
}

// Subscribe returns the event channel for a run.
func (r *Runner) Subscribe(runID string) (chan AgentEvent, bool) {
	r.mu.Lock()
	b, ok := r.brokers[runID]
	r.mu.Unlock()
	if !ok {
		return nil, false
	}
	return b.subscribe(), true
}

// finishRun marks a run terminal and notifies subscribers.
func finishRun(r *Runner, run model.Run, b *broker, aborted bool) {
	if aborted {
		r.store.FinishRun(run.ID, model.RunCanceled, "")
		b.publish(AgentEvent{Type: "canceled", RunID: run.ID, At: nowMs()})
		return
	}
	r.store.FinishRun(run.ID, model.RunSucceeded, "")
	b.publish(AgentEvent{Type: "done", RunID: run.ID, At: nowMs()})
}

// writeSystemPrompt composes the project's stable design instructions into a
// temporary file that Pi appends to its system prompt (no argv limits).
func (r *Runner) writeSystemPrompt(proj model.Project, req RunRequest) (string, func(), error) {
	in := prompt.Input{
		ProjectDir:     proj.Dir,
		FrontendDesign: readFile(filepath.Join(r.catalog.Root, "skills", prompt.FrontendDesignSkillID, "SKILL.md")),
		CraftDir:       filepath.Join(r.catalog.Root, "craft"),
		CraftDocs:      r.catalog.CraftIndex(),
		RenderPreview:  r.cfg.Renderer != "" && r.cfg.Extension != "",
	}

	dsID := req.DesignSystemID
	if dsID == "" {
		dsID = proj.DesignSystemID
	}
	if dsID != "" {
		ctx, err := r.catalog.DesignSystemContext(dsID)
		if err != nil {
			return "", nil, fmt.Errorf("design system: %w", err)
		}
		name := ctx.Name
		if ds, ok := r.catalog.DesignSystem(dsID); ok && ds.Name != "" {
			name = ds.Name
		}
		in.DesignSystem = &prompt.DesignSystem{
			Name:           name,
			Dir:            filepath.Join(r.catalog.Root, "design-systems", dsID),
			DesignMD:       ctx.DesignMD,
			TokensCSS:      ctx.TokensCSS,
			UsageMD:        ctx.UsageMD,
			ComponentsHTML: ctx.ComponentHx,
		}
	}

	skillID := req.SkillID
	if skillID == "" {
		skillID = req.TemplateID
	}
	if skillID != "" {
		in.Skill = r.resolveSkill(skillID)
	}

	f, err := os.CreateTemp("", "pidesigner-system-*.md")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	if _, err := f.WriteString(prompt.System(in)); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

// resolveSkill finds a workflow skill among skills and design templates;
// both are directories with a SKILL.md.
func (r *Runner) resolveSkill(id string) *prompt.Skill {
	name, dir, mode := "", "", ""
	if sk, ok := r.catalog.Skill(id); ok {
		name, dir, mode = sk.Name, sk.Path, sk.Mode
	} else if t, ok := r.catalog.Template(id); ok {
		name, dir, mode = t.Name, t.Path, t.Mode
	} else {
		return nil
	}
	body := readFile(filepath.Join(dir, "SKILL.md"))
	if body == "" {
		return nil
	}
	return &prompt.Skill{ID: id, Name: name, Dir: dir, Mode: mode, Body: body}
}

// hasSession reports whether Pi already stores a session for a conversation.
// Pi names session files "<timestamp>_<session id>.jsonl".
func (r *Runner) hasSession(conversationID string) bool {
	matches, _ := filepath.Glob(filepath.Join(r.cfg.SessionDir, "*_"+conversationID+".jsonl"))
	return len(matches) > 0
}

// maxReplayMessages bounds the transcript replayed into a new session.
const maxReplayMessages = 24

// priorTranscript returns the conversation before the current user turn.
func (r *Runner) priorTranscript(conversationID string) []prompt.Message {
	messages, err := r.store.ListMessages(conversationID)
	if err != nil {
		return nil
	}
	// Start appended the current prompt; it is sent separately.
	if n := len(messages); n > 0 && messages[n-1].Role == model.RoleUser {
		messages = messages[:n-1]
	}
	if len(messages) > maxReplayMessages {
		messages = messages[len(messages)-maxReplayMessages:]
	}
	out := make([]prompt.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == model.RoleUser || m.Role == model.RoleAssistant {
			out = append(out, prompt.Message{Role: m.Role, Content: m.Content})
		}
	}
	return out
}

// runWork accumulates the plan, renders, and critique rounds of a run.
type runWork struct {
	mu        sync.Mutex
	todos     []model.Todo
	renders   []model.Render
	critiques []model.Critique
}

func (w *runWork) setTodos(t []model.Todo) {
	w.mu.Lock()
	w.todos = t
	w.mu.Unlock()
}

func (w *runWork) addRender(r model.Render) {
	w.mu.Lock()
	w.renders = append(w.renders, r)
	w.mu.Unlock()
}

func (w *runWork) addCritique(c model.Critique) {
	w.mu.Lock()
	w.critiques = append(w.critiques, c)
	w.mu.Unlock()
}

// snapshot returns copies safe to use after the event loop ends.
func (w *runWork) snapshot() *runWork {
	w.mu.Lock()
	defer w.mu.Unlock()
	return &runWork{
		todos:     append([]model.Todo(nil), w.todos...),
		renders:   append([]model.Render(nil), w.renders...),
		critiques: append([]model.Critique(nil), w.critiques...),
	}
}

// parseTodos reads a todo_write call's arguments.
func parseTodos(args json.RawMessage) ([]model.Todo, bool) {
	var in struct {
		Todos []model.Todo `json:"todos"`
	}
	if json.Unmarshal(args, &in) != nil || in.Todos == nil {
		return nil, false
	}
	todos := make([]model.Todo, 0, len(in.Todos))
	for _, t := range in.Todos {
		if strings.TrimSpace(t.Content) == "" {
			continue
		}
		switch t.Status {
		case "pending", "in_progress", "completed":
		default:
			t.Status = "pending"
		}
		todos = append(todos, t)
	}
	return todos, true
}

// renderFileName matches the PNG names the extension writes.
var renderFileName = regexp.MustCompile(`^[0-9]+-(web|desktop|mobile)(-full|-s[0-9]+)?\.png$`)

// parseRender reads a finished render_preview call's details.
func parseRender(projectID string, details json.RawMessage) (model.Render, bool) {
	var d struct {
		Viewport  string `json:"viewport"`
		Path      string `json:"path"`
		File      string `json:"file"`
		FullPage  bool   `json:"fullPage"`
		Slide     int    `json:"slide"`
		OverflowX bool   `json:"overflowX"`
		Errors    int    `json:"errors"`
	}
	if json.Unmarshal(details, &d) != nil || !renderFileName.MatchString(d.File) {
		return model.Render{}, false
	}
	return model.Render{
		Viewport: d.Viewport,
		Path:     d.Path,
		Image:    "/api/projects/" + projectID + "/renders/" + d.File,
		FullPage: d.FullPage,
		Slide:    d.Slide,
		// A swiping deck's slide strip is wide by design, not an overflow.
		OverflowX: d.OverflowX && d.Slide == 0,
		Errors:    d.Errors,
	}, true
}

// RenderFile resolves a render image of a project, or "" when the name is not
// a render the extension could have written.
func (r *Runner) RenderFile(projectID, name string) string {
	if !renderFileName.MatchString(name) || r.cfg.RenderDir == "" {
		return ""
	}
	return filepath.Join(r.cfg.RenderDir, projectID, name)
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// ArtifactBundle is the full set of files an artifact needs.
type ArtifactBundle struct {
	EntryFile string            `json:"entryFile"`
	Files     map[string]string `json:"files"`
}

// BuildArtifact collects the entry file plus every sibling it references.
func BuildArtifact(pm *project.Manager, entry string) (*ArtifactBundle, error) {
	if entry == "" {
		entry = pm.FindEntryFile()
	}
	if entry == "" {
		return nil, fmt.Errorf("no artifact entry file found")
	}
	html, err := pm.Read(entry)
	if err != nil {
		return nil, err
	}
	files := map[string]string{entry: html}

	// Pull in referenced local assets (css, js, images, fonts).
	for _, ref := range referencedAssets(html) {
		if _, ok := files[ref]; ok {
			continue
		}
		if content, err := pm.Read(ref); err == nil {
			files[ref] = content
		}
	}
	return &ArtifactBundle{EntryFile: entry, Files: files}, nil
}

// referencedAssets extracts same-directory asset references from an HTML doc.
func referencedAssets(html string) []string {
	// Matches href/src with a quoted or unquoted value.
	re := regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"'` + "`" + `]+))`)

	var out []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		raw := m[1]
		if raw == "" {
			raw = m[2]
		}
		if raw == "" {
			raw = m[3]
		}
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") ||
			strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "#") ||
			strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") {
			continue
		}
		raw = strings.SplitN(raw, "?", 2)[0]
		raw = strings.SplitN(raw, "#", 2)[0]
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, raw)
	}
	return out
}

func nowMs() int64 { return time.Now().UnixMilli() }
