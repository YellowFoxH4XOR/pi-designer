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
	"github.com/akki/pidesigner/internal/versions"
)

// AgentEvent is the shared durable activity contract.
type AgentEvent = model.AgentEvent

// Runner owns agent sessions and run lifecycles.
type Runner struct {
	store   *store.Store
	catalog *content.Catalog
	piBin   string
	cfg     RunnerConfig

	uiMu            sync.Mutex
	mu              sync.Mutex
	wg              sync.WaitGroup
	shuttingDown    bool
	sessionSettled  map[string]chan struct{}
	sessions        map[string]*pirun.Session
	journals        map[string]*eventJournal
	active          map[string]string
	deleting        map[string]bool
	restoring       map[string]bool
	cancelRequested map[string]bool
	hub             FileChangeNotifier
}

// RunnerConfig locates the agent's persistent state and design tools.
type RunnerConfig struct {
	// SessionDir holds one pi session directory per conversation:
	// SessionDir/<conversation id>/pi/.
	SessionDir string
	// RenderDir stores preview renders, one subdirectory per project.
	RenderDir string
	// Extension is πDesigner's pi extension (todo_write, render_preview,
	// submit_critique).
	Extension string
	// Renderer is the pidesigner-render executable; empty disables
	// render_preview.
	Renderer string
	// Versions keeps a snapshot of the project after each run; nil disables
	// version history.
	Versions *versions.Store
}

// FileChangeNotifier is notified when a run mutates project files.
type FileChangeNotifier interface {
	NotifyProjectChanged(projectID string)
}

// NewRunner creates a Runner.
func NewRunner(st *store.Store, cat *content.Catalog, piBin string, hub FileChangeNotifier, cfg RunnerConfig) *Runner {
	return &Runner{
		store:           st,
		catalog:         cat,
		piBin:           piBin,
		cfg:             cfg,
		sessions:        map[string]*pirun.Session{},
		journals:        map[string]*eventJournal{},
		active:          map[string]string{},
		deleting:        map[string]bool{},
		restoring:       map[string]bool{},
		cancelRequested: map[string]bool{},
		sessionSettled:  map[string]chan struct{}{},
		hub:             hub,
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
	RunID string
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
	if req.DesignSystemID == "" {
		req.DesignSystemID = proj.DesignSystemID
	}

	now := time.Now()
	var userMessage *model.Message
	if req.ConversationID != "" {
		userMessage = &model.Message{
			ID: newID("msg"), ConversationID: req.ConversationID,
			Role: model.RoleUser, Content: req.Prompt, CreatedAt: now,
		}
	}
	run := model.Run{
		ID:             newID("run"),
		ProjectID:      proj.ID,
		ConversationID: req.ConversationID,
		Status:         model.RunRunning,
		Prompt:         req.Prompt,
		StartedAt:      now,
	}
	r.mu.Lock()
	if r.shuttingDown {
		r.mu.Unlock()
		return nil, fmt.Errorf("πDesigner is shutting down")
	}
	if r.deleting[proj.ID] {
		r.mu.Unlock()
		return nil, fmt.Errorf("this project is being deleted")
	}
	if r.restoring[proj.ID] {
		r.mu.Unlock()
		return nil, fmt.Errorf("a version of this project is being restored")
	}
	if r.active[proj.ID] != "" {
		r.mu.Unlock()
		return nil, fmt.Errorf("pi is already working in this project")
	}
	r.active[proj.ID] = run.ID
	r.wg.Add(1)
	r.mu.Unlock()
	var persistErr error
	if userMessage != nil {
		persistErr = r.store.CreateRunWithMessage(run, userMessage)
	} else {
		persistErr = r.store.CreateRun(run)
	}
	if persistErr != nil {
		r.wg.Done()
		r.mu.Lock()
		delete(r.active, proj.ID)
		r.mu.Unlock()
		return nil, persistErr
	}

	b := newRunJournal(r.store, run, proj)
	b.publish(AgentEvent{Type: "context", RunID: run.ID, Data: modelJSON(map[string]string{"skillId": req.SkillID, "designSystemId": req.DesignSystemID, "viewport": req.Viewport, "thinking": req.Thinking}), At: nowMs()})
	r.mu.Lock()
	r.journals[run.ID] = b
	r.mu.Unlock()

	go r.execute(run, proj, req, b)
	return &RunHandle{RunID: run.ID}, nil
}

// execute runs the agent and streams its events.
func (r *Runner) execute(run model.Run, proj model.Project, req RunRequest, b *eventJournal) {
	defer r.wg.Done()
	defer func() {
		b.close()
		r.mu.Lock()
		delete(r.journals, run.ID)
		if r.active[proj.ID] == run.ID {
			delete(r.active, proj.ID)
		}
		delete(r.cancelRequested, run.ID)
		r.mu.Unlock()
	}()

	pm := project.New(proj.Dir)

	renderDir := filepath.Join(r.cfg.RenderDir, proj.ID)
	opts := pirun.Options{
		Bin:      r.piBin,
		Dir:      proj.Dir,
		Model:    req.Model,
		Thinking: req.Thinking,
		Env:      append(os.Environ(), "PIDESIGNER_RENDER_DIR="+renderDir),
	}
	if r.cfg.Extension != "" {
		opts.Extensions = []string{r.cfg.Extension}
	}
	if r.cfg.Renderer != "" {
		opts.Env = append(opts.Env, "PIDESIGNER_RENDERER="+r.cfg.Renderer)
	}
	if r.catalog.Knowledge != nil {
		opts.Env = append(opts.Env, "PIDESIGNER_RUNTIME_DISCOVERY="+filepath.Join(filepath.Dir(r.catalog.Knowledge.Dir), "runtime.json"))
	}

	turn := prompt.TurnInput{Prompt: req.Prompt, Viewport: req.Viewport, SkipDiscovery: req.SkipDiscovery}
	if r.catalog.Knowledge != nil {
		var catalog strings.Builder
		for _, src := range r.catalog.Knowledge.Sources {
			kind := "browse-only; not locally readable"
			if src.DocumentURL != "" {
				kind = "approved text reference; verified local copy when available"
			}
			fmt.Fprintf(&catalog, "- %s (%s): %s %s [%s; %s]\n", src.ID, src.Name, src.UseFor, src.Note, kind, src.License)
		}
		turn.SourceCatalog = catalog.String()
	}
	// Each conversation is one persistent pi session, so follow-up turns keep
	// the agent's full context: its plan, the files it read, its renders. pi
	// sessions live in their own subdirectory so files left by an earlier
	// agent harness are never continued. A conversation without a pi session
	// yet (new, or from an earlier harness) gets its transcript replayed once;
	// the session it starts carries the history from then on.
	if req.ConversationID != "" && r.cfg.SessionDir != "" {
		opts.SessionDir = filepath.Join(r.cfg.SessionDir, req.ConversationID, "pi")
		if !hasSession(opts.SessionDir) {
			turn.Transcript = r.priorTranscript(req.ConversationID)
		}
	}

	systemFile, cleanup, err := r.writeSystemPrompt(proj, req)
	if err != nil {
		r.completeRun(run, b, model.RunFailed, err.Error())
		return
	}
	defer cleanup()
	// The independent jury in submit_critique judges renders against the
	// brief, never against the agent's own account of its work.
	if bin, err := pirun.ResolveBin(r.piBin); err == nil {
		opts.Env = append(opts.Env, "PIDESIGNER_PI_BIN="+bin)
	}
	if jury := os.Getenv("PIDESIGNER_JURY_MODEL"); jury != "" {
		opts.Env = append(opts.Env, "PIDESIGNER_JURY_MODEL="+jury)
	}
	if briefFile, err := r.writeBrief(req); err == nil {
		defer os.Remove(briefFile)
		opts.Env = append(opts.Env, "PIDESIGNER_BRIEF_FILE="+briefFile)
	}
	// πDesigner's prompt is the system prompt: it replaces pi's coding-
	// assistant identity rather than being appended to it. A design run sees
	// only πDesigner's skills and instructions, not the user's personal pi
	// skills or AGENTS.md files.
	opts.SystemPrompt = systemFile
	// Plan-approval tools from planning packages would park an unattended run
	// waiting for sign-off; todo_write is the design plan here.
	opts.ExtraArgs = append(opts.ExtraArgs, "--no-skills", "--no-context-files", "--exclude-tools", "plannotator_*")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess, err := pirun.Start(ctx, opts)
	if err != nil {
		r.completeRun(run, b, model.RunFailed, err.Error())
		return
	}
	settled := make(chan bool, 1)
	settledSignal := make(chan struct{})
	processGone := make(chan struct{})
	events := sess.Events()
	r.mu.Lock()
	if r.shuttingDown || r.cancelRequested[run.ID] {
		r.mu.Unlock()
		sess.CloseWith()
		sess.Close()
		r.completeRun(run, b, model.RunCanceled, "")
		return
	}
	r.sessions[run.ID] = sess
	r.sessionSettled[run.ID] = settledSignal
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.sessions, run.ID)
		delete(r.sessionSettled, run.ID)
		r.mu.Unlock()
		sess.Close()
	}()

	// Subscribe before prompting so a fast completion is not missed.
	var assistantText strings.Builder
	var messageText strings.Builder
	var providerFailure string
	work := &runWork{}

	go func() {
		defer close(processGone)
		ticker := time.NewTicker(60 * time.Millisecond)
		defer ticker.Stop()
		var pending strings.Builder
		var pendingIndex int
		flush := func() {
			if pending.Len() > 0 {
				b.publish(AgentEvent{Type: "text", RunID: run.ID, Text: pending.String(), ContentIndex: pendingIndex, At: nowMs()})
				pending.Reset()
			}
		}
		defer flush()
		for {
			var ev pirun.Event
			select {
			case <-ticker.C:
				flush()
				continue
			case next, open := <-events:
				if !open {
					return
				}
				ev = next
			}
			if ev.Type != pirun.EventTextDelta || ev.ContentIndex != pendingIndex {
				flush()
			}
			switch ev.Type {
			case "message_start":
				messageText.Reset()
				b.publish(AgentEvent{Type: "message_start", RunID: run.ID, At: nowMs()})
			case "message_end":
				prefix := strings.TrimSuffix(assistantText.String(), messageText.String())
				assistantText.Reset()
				assistantText.WriteString(prefix)
				assistantText.WriteString(ev.Text)
				messageText.Reset()
				messageText.WriteString(ev.Text)
				b.publish(AgentEvent{Type: "message_end", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventThinkingDelta:
				b.publish(AgentEvent{Type: "thinking", RunID: run.ID, At: nowMs()})
			case pirun.EventTextDelta:
				messageText.WriteString(ev.Text)
				assistantText.WriteString(ev.Text)
				pendingIndex = ev.ContentIndex
				pending.WriteString(ev.Text)
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
				b.publish(AgentEvent{Type: "tool", RunID: run.ID, Tool: ev.ToolName, ToolCallID: ev.ToolCallID, Args: ev.ToolArgs, At: nowMs()})
				if ev.ToolName == "todo_write" {
					if todos, ok := parseTodos(ev.ToolArgs); ok {
						work.setTodos(todos)
						b.publish(AgentEvent{Type: "todos", RunID: run.ID, Todos: todos, At: nowMs()})
					}
				}
			case "tool_update":
				b.publish(AgentEvent{Type: "tool_update", RunID: run.ID, Tool: ev.ToolName, ToolCallID: ev.ToolCallID, Args: ev.ToolArgs, Output: ev.Output, At: nowMs()})
			case pirun.EventToolEnd:
				b.publish(AgentEvent{Type: "tool_end", RunID: run.ID, Tool: ev.ToolName, ToolCallID: ev.ToolCallID, Output: ev.Output, DurationMS: ev.DurationMS, IsError: ev.IsError, At: nowMs()})
				// Tool calls mutate the project; nudge preview watchers.
				if ev.ToolName == "write" || ev.ToolName == "edit" {
					r.hub.NotifyProjectChanged(proj.ID)
				}
				if ev.ToolName == "select_design_sources" && !ev.IsError {
					var selection struct {
						Intent  string `json:"intent"`
						Choices []struct {
							ID     string `json:"id"`
							Name   string `json:"name"`
							Reason string `json:"reason"`
						} `json:"choices"`
					}
					if json.Unmarshal(ev.ToolDetails, &selection) == nil && selection.Intent != "" && len(selection.Choices) <= 2 {
						b.publish(AgentEvent{Type: "design_route", RunID: run.ID, Data: ev.ToolDetails, At: nowMs()})
					}
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
			case "retry", "retry_end", "compaction_start", "compaction_end", "session_update":
				b.publish(AgentEvent{Type: ev.Type, RunID: run.ID, Data: ev.Data, At: nowMs()})
			case "extension_ui":
				r.handleExtensionUI(run.ID, sess, b, ev.Data)
			case "diagnostic", pirun.EventStderr:
				b.publish(AgentEvent{Type: "stderr", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventError:
				providerFailure = ev.Text
				b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventSettled:
				settled <- ev.Aborted
				close(settledSignal)
				return
			}
			if b.persistenceError() != nil {
				sess.CloseWith()
				return
			}
		}
	}()

	// Confirm the installed pi selected the exact requested provider/model.
	// CLI argument parsing must never silently substitute a similarly named one.
	checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
	state, stateErr := sess.State(checkCtx)
	checkCancel()
	if stateErr == nil && state.Model == nil {
		stateErr = fmt.Errorf("pi has no model selected. Run `pi`, use /login, or configure a model in pi first")
	}
	if stateErr == nil && req.Model != "" && state.Model.Ref() != req.Model {
		stateErr = fmt.Errorf("pi selected %s instead of %s. Choose a model from the available list", state.Model.Ref(), req.Model)
	}
	if stateErr != nil {
		r.completeRun(run, b, model.RunFailed, stateErr.Error())
		return
	}
	b.publish(AgentEvent{Type: "model", RunID: run.ID, Text: state.Model.Name + " · " + state.Model.Provider, Data: modelJSON(state.Model), At: nowMs()})

	started, err := sess.Prompt(ctx, prompt.Turn(turn))
	if err != nil {
		r.completeRun(run, b, model.RunFailed, err.Error())
		return
	}

	r.mu.Lock()
	cancelRequested := r.cancelRequested[run.ID]
	r.mu.Unlock()
	if cancelRequested {
		abortCtx, abortCancel := context.WithTimeout(ctx, 10*time.Second)
		if sess.Abort(abortCtx) != nil {
			sess.CloseWith()
		}
		abortCancel()
	}

	// pi finished the prompt locally (e.g. a slash command): no agent run
	// started, so waiting for it to settle would hang.
	if !started {
		r.completeRun(run, b, model.RunSucceeded, "")
		return
	}

	// Prefer a settled signal if one already landed: the process-exit path
	// may become ready at the same moment.
	var (
		aborted   bool
		failure   string
		loopStuck bool
	)
	runTimer := time.NewTimer(2 * time.Hour)
	defer runTimer.Stop()
	select {
	case aborted = <-settled:
	default:
		select {
		case aborted = <-settled:
		case <-processGone:
			// pi exited without settling -- surface diagnostics.
			select {
			case aborted = <-settled:
			default:
				failure = "pi exited before completing the run"
				if tail := strings.TrimSpace(sess.StderrTail()); tail != "" {
					failure += ": " + tail
				}
			}
		case <-runTimer.C:
			failure = "timeout"
			// The event loop still owns the transcript; stop pi and wait for
			// the loop to exit before reading what it accumulated.
			sess.CloseWith()
			select {
			case <-processGone:
			case <-time.After(5 * time.Second):
				// A detached descendant can hold pi's stdout open; never
				// read state the loop may still be writing.
				loopStuck = true
			}
		}
	}

	if failure == "" && !aborted && !loopStuck {
		failure = providerFailure
	}
	if err := b.persistenceError(); err != nil {
		failure = "Activity could not be saved: " + err.Error()
	}

	r.mu.Lock()
	stopping := r.shuttingDown || r.cancelRequested[run.ID]
	r.mu.Unlock()
	if stopping {
		aborted = true
		failure = ""
	}

	// Persist the plan, renders, critique rounds, and the assistant turn.
	w := work.snapshot()
	_ = r.store.SaveRunWork(run.ID, w.todos, w.renders, w.critiques)
	transcript := ""
	if !loopStuck {
		transcript = assistantText.String()
	}
	messageID := ""
	if txt := strings.TrimSpace(transcript); txt != "" && run.ConversationID != "" {
		messageID = newID("msg")
		if err := r.store.AppendMessage(model.Message{
			ID: messageID, ConversationID: run.ConversationID,
			Role: model.RoleAssistant, Content: txt, CreatedAt: time.Now(),
		}); err != nil {
			failure = "Assistant transcript could not be saved: " + err.Error()
			messageID = ""
		}
	}

	// Keep the project's entry file fresh.
	if entry := pm.FindEntryFile(); entry != "" && entry != proj.EntryFile {
		proj.EntryFile = entry
		_ = r.store.UpdateProject(proj)
	}
	// Record what this turn left behind as the next artifact version, before
	// completion is published, so a reconnecting client sees it too.
	if r.cfg.Versions != nil {
		if _, _, err := r.cfg.Versions.Snapshot(proj.ID, pm, versions.Meta{RunID: run.ID, MessageID: messageID, Entry: proj.EntryFile}); err != nil {
			b.publish(AgentEvent{Type: "stderr", RunID: run.ID, Text: "Version history: " + err.Error(), At: nowMs()})
		}
	}
	r.hub.NotifyProjectChanged(proj.ID)
	// Publish completion only after the transcript and entry metadata are
	// durable, so a reconnect sees the same result as the streaming client.
	if failure != "" {
		r.completeRun(run, b, model.RunFailed, failure)
	} else {
		if !aborted {
			publishQuestionAttention(run.ID, transcript, b)
		}
		finishRun(r, run, b, aborted)
	}
}

// Shutdown cancels every in-flight run so agent processes are reaped before
// the daemon exits.
func (r *Runner) Shutdown() {
	r.mu.Lock()
	r.shuttingDown = true
	sessions := make([]*pirun.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.mu.Unlock()

	for _, s := range sessions {
		s.CloseWith()
	}
	r.wg.Wait() // finish journal writes before the app closes SQLite
}

// Cancel aborts an in-flight run.
func (r *Runner) Cancel(runID string) error {
	r.mu.Lock()
	active := false
	for _, id := range r.active {
		if id == runID {
			active = true
			break
		}
	}
	if !active {
		r.mu.Unlock()
		return fmt.Errorf("run %s is not active", runID)
	}
	r.cancelRequested[runID] = true
	sess := r.sessions[runID]
	settled := r.sessionSettled[runID]
	r.mu.Unlock()
	if sess == nil {
		return nil
	} // execute observes cancellation before prompting
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sess.Abort(ctx); err != nil {
		sess.CloseWith()
		return nil
	}

	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-settled:
	case <-sess.Done():
	case <-timer.C:
		// An abort acknowledgment is not completion. Kill this session if
		// pi has not reported a settled prompt within the grace period.
		sess.CloseWith()
	}
	return nil
}

// writeSystemPrompt composes the project's stable design instructions into a
// temporary file that becomes pi's system prompt (no argv limits).
func (r *Runner) writeSystemPrompt(proj model.Project, req RunRequest) (string, func(), error) {
	in := prompt.Input{
		ProjectDir:     proj.Dir,
		FrontendDesign: readFile(filepath.Join(r.catalog.Root, "skills", prompt.FrontendDesignSkillID, "SKILL.md")),
		CraftDir:       filepath.Join(r.catalog.Root, "craft"),
		CraftDocs:      r.catalog.CraftIndex(),
		RenderPreview:  r.cfg.Renderer != "" && r.cfg.Extension != "",
	}

	if r.catalog.Knowledge != nil {
		in.KnowledgeDir = r.catalog.Knowledge.Dir
		in.KnowledgeDocs = r.catalog.Knowledge.Docs()
		in.KnowledgeHints = map[string]string{}
		for _, src := range r.catalog.Knowledge.Sources {
			in.KnowledgeHints[src.ID+".md"] = src.UseFor
		}
	}

	// Start already defaulted the design system and skill to the project's.
	if dsID := req.DesignSystemID; dsID != "" {
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
			Dir:            ctx.Dir,
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

	path, err := writeTemp("pidesigner-system-*.md", prompt.System(in))
	if err != nil {
		return "", nil, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

// writeTemp writes content to a new temporary file, removing it on failure.
func writeTemp(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// writeBrief saves what the user asked for — the conversation's opening
// request and this turn's message — for the independent jury.
func (r *Runner) writeBrief(req RunRequest) (string, error) {
	brief := req.Prompt
	if req.ConversationID != "" {
		if messages, err := r.store.ListMessages(req.ConversationID); err == nil {
			for _, m := range messages {
				if m.Role == model.RoleUser {
					if strings.TrimSpace(m.Content) != strings.TrimSpace(req.Prompt) {
						brief = m.Content + "\n\nLatest request: " + req.Prompt
					}
					break
				}
			}
		}
	}
	return writeTemp("pidesigner-brief-*.md", brief)
}

// resolveSkill finds a workflow skill among skills and design templates;
// both are directories with a SKILL.md.
func (r *Runner) resolveSkill(id string) *prompt.Skill {
	sk, ok := r.catalog.Skill(id)
	if !ok {
		return nil
	}
	body := readFile(filepath.Join(sk.Path, "SKILL.md"))
	if body == "" {
		return nil
	}
	return &prompt.Skill{ID: id, Name: sk.Name, Dir: sk.Path, Mode: sk.Mode, Body: body}
}

// hasSession reports whether pi already stores a session in a
// conversation's session directory ("<timestamp>_<uuid>.jsonl" files).
func hasSession(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
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
var renderFileName = regexp.MustCompile(`^[0-9]+-(web|desktop|mobile)(-full|-s[0-9]+|-t[0-9]+)?\.png$`)

// parseRender reads a finished render_preview call's details.
func parseRender(projectID string, details json.RawMessage) (model.Render, bool) {
	var d struct {
		Viewport  string `json:"viewport"`
		Path      string `json:"path"`
		File      string `json:"file"`
		FullPage  bool   `json:"fullPage"`
		Slide     int    `json:"slide"`
		At        int    `json:"at"`
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
		At:       d.At,
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

func nowMs() int64 { return time.Now().UnixMilli() }
