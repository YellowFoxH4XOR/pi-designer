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
	"github.com/akki/pidesigner/internal/store"
)

// AgentEvent is one event in a run's event stream.
type AgentEvent struct {
	Type    string          `json:"type"`
	RunID   string          `json:"runId"`
	Text    string          `json:"text,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Args    json.RawMessage `json:"args,omitempty"`
	IsError bool            `json:"isError,omitempty"`
	Aborted bool            `json:"aborted,omitempty"`
	At      int64           `json:"at"`
}

// broker fans a run's events out to any number of SSE subscribers.
type broker struct {
	mu     sync.Mutex
	subs   map[chan AgentEvent]struct{}
	closed bool
}

func newBroker() *broker { return &broker{subs: map[chan AgentEvent]struct{}{}} }

func (b *broker) subscribe() chan AgentEvent {
	ch := make(chan AgentEvent, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan AgentEvent) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *broker) publish(e AgentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
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

	mu       sync.Mutex
	sessions map[string]*pirun.Session
	brokers  map[string]*broker
	hub      FileChangeNotifier
}

// FileChangeNotifier is notified when a run mutates project files.
type FileChangeNotifier interface {
	NotifyProjectChanged(projectID string)
}

// NewRunner creates a Runner.
func NewRunner(st *store.Store, cat *content.Catalog, piBin string, hub FileChangeNotifier) *Runner {
	return &Runner{
		store:    st,
		catalog:  cat,
		piBin:    piBin,
		sessions: map[string]*pirun.Session{},
		brokers:  map[string]*broker{},
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

	now := time.Now()
	run := model.Run{
		ID:             newID("run"),
		ProjectID:      proj.ID,
		ConversationID: req.ConversationID,
		Status:         model.RunRunning,
		Prompt:         req.Prompt,
		StartedAt:      now,
	}
	if err := r.store.CreateRun(run); err != nil {
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
	}

	// Load the selected skill so the agent picks up its methodology.
	if req.SkillID != "" {
		if sk, ok := r.catalog.Skill(req.SkillID); ok {
			opts.SkillPaths = append(opts.SkillPaths, sk.Path)
		}
	} else if req.TemplateID != "" {
		if t, ok := r.catalog.Template(req.TemplateID); ok {
			opts.SkillPaths = append(opts.SkillPaths, t.Path)
		}
	}

	// Compose the brand contract + working instructions into a temp file so
	// the context is appended to Pi's system prompt without argv limits.
	dsID := req.DesignSystemID
	if dsID == "" {
		dsID = proj.DesignSystemID
	}
	if ctxFile, cleanup, err := r.composeSystemContext(proj, dsID, req.TemplateID); err == nil {
		opts.SystemPromptFiles = append(opts.SystemPromptFiles, ctxFile)
		defer cleanup()
	}

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

	// Subscribe before prompting so a fast completion is not missed.
	events := sess.Events()
	settled := make(chan bool, 1)
	processGone := make(chan struct{})
	var assistantText strings.Builder

	go func() {
		defer close(processGone)
		for ev := range events {
			switch ev.Type {
			case pirun.EventTextDelta:
				assistantText.WriteString(ev.Text)
				b.publish(AgentEvent{Type: "text", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventAgentEnd:
				// A run spans several assistant turns. Separate them so the
				// transcript reads as paragraphs instead of running together.
				if s := assistantText.String(); s != "" && !strings.HasSuffix(s, "\n\n") {
					assistantText.WriteString("\n\n")
				}
				b.publish(AgentEvent{Type: "turn", RunID: run.ID, At: nowMs()})
			case pirun.EventToolStart:
				b.publish(AgentEvent{Type: "tool", RunID: run.ID, Tool: ev.ToolName, Args: ev.ToolArgs, At: nowMs()})
			case pirun.EventToolEnd:
				b.publish(AgentEvent{Type: "tool_end", RunID: run.ID, Tool: ev.ToolName, IsError: ev.IsError, At: nowMs()})
				// Tool calls mutate the project; nudge preview watchers.
				if ev.ToolName == "write" || ev.ToolName == "edit" || ev.ToolName == "multi_edit" {
					r.hub.NotifyProjectChanged(proj.ID)
				}
			case pirun.EventStderr:
				b.publish(AgentEvent{Type: "stderr", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventError:
				b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: ev.Text, At: nowMs()})
			case pirun.EventSettled:
				select {
				case settled <- ev.Aborted:
				default:
				}
			}
		}
	}()

	disposition, err := sess.Prompt(ctx, req.Prompt)
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
			failure = "pi exited before completing the run"
			if tail := strings.TrimSpace(sess.StderrTail()); tail != "" {
				failure += ": " + tail
			}
		case <-time.After(2 * time.Hour):
			failure = "timeout"
		}
	}

	if failure != "" {
		r.store.FinishRun(run.ID, model.RunFailed, failure)
		b.publish(AgentEvent{Type: "error", RunID: run.ID, Text: failure, At: nowMs()})
	} else {
		finishRun(r, run, b, aborted)
	}

	// Persist the assistant turn.
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

// composeSystemContext writes the brand contract and working instructions to a
// temporary file that Pi appends to its system prompt.
func (r *Runner) composeSystemContext(proj model.Project, dsID, templateID string) (string, func(), error) {
	var b strings.Builder

	b.WriteString("# Working context for this design task\n\n")
	fmt.Fprintf(&b, "Project directory: %s\n", proj.Dir)
	b.WriteString("You are working inside a design project directory. Write the artifact files there.\n")
	b.WriteString("Deliverables: a self-contained HTML artifact (entry `index.html` unless told otherwise) ")
	b.WriteString("that renders in a sandboxed preview, with CSS and assets alongside it.\n")
	b.WriteString("Use the design system's tokens and typography. Do not invent a new palette.\n")
	b.WriteString("Prefer transform/opacity for animation so the preview stays at 60fps.\n")
	b.WriteString("Do not search the filesystem for reference material -- every path you need is listed below.\n\n")

	// The preview renders the artifact in three fixed device frames. Naming the
	// exact viewports is the difference between a design that looks finished in
	// the app and one that overflows its frame.
	b.WriteString("## Viewports this design is rendered in\n\n")
	b.WriteString("The preview shows the artifact in three fixed frames. It must work at all of them:\n\n")
	b.WriteString("- Web: 1440 x 900\n")
	b.WriteString("- Desktop: 1280 x 800\n")
	b.WriteString("- Mobile: 390 x 844\n\n")
	b.WriteString("390px is a hard requirement, not an afterthought. The page must not overflow ")
	b.WriteString("horizontally at 390px: no fixed-width containers wider than the viewport, no long ")
	b.WriteString("unbreakable strings, and no content that depends on a 500px+ breakpoint. Verify the ")
	b.WriteString("390px layout before you finish.\n\n")

	if dsID != "" {
		dsDir := filepath.Join(r.catalog.Root, "design-systems", dsID)
		if ds, err := r.catalog.DesignSystemContext(dsID); err == nil {
			fmt.Fprintf(&b, "## Design system: %s\n\n", ds.Name)
			fmt.Fprintf(&b, "Package directory: %s\n\n", dsDir)
			if ds.UsageMD != "" {
				b.WriteString("### Usage guide\n\n")
				b.WriteString(ds.UsageMD)
				b.WriteString("\n\n")
			}
			b.WriteString("### DESIGN.md (brand contract)\n\n")
			b.WriteString(ds.DesignMD)
			b.WriteString("\n\n")
			if ds.TokensCSS != "" {
				b.WriteString("### tokens.css (canonical semantic tokens)\n\n")
				b.WriteString("```css\n")
				b.WriteString(ds.TokensCSS)
				b.WriteString("\n```\n\n")
			}
			if ds.ComponentHx != "" {
				b.WriteString("### Component reference\n\n")
				b.WriteString(ds.ComponentHx)
				b.WriteString("\n\n")
			}
		}
	}

	if templateID != "" {
		if t, ok := r.catalog.Template(templateID); ok {
			fmt.Fprintf(&b, "## Template: %s\n\n", t.Name)
			fmt.Fprintf(&b, "Template directory: %s\n", t.Path)
			b.WriteString("Read the template's SKILL.md and follow its structure.\n\n")
		}
	}

	// Craft laws: the shared design discipline applied to every artifact.
	craftDir := filepath.Join(r.catalog.Root, "craft")
	if laws := r.catalog.CraftIndex(); len(laws) > 0 {
		b.WriteString("## Design laws\n\n")
		fmt.Fprintf(&b, "Directory: %s\n\n", craftDir)
		b.WriteString("Read the ones that matter for this task before writing code:\n")
		for _, l := range laws {
			fmt.Fprintf(&b, "- %s\n", filepath.Join(craftDir, l))
		}
		b.WriteString("\n")
	}

	f, err := os.CreateTemp("", "pidesigner-context-*.md")
	if err != nil {
		return "", func() {}, err
	}
	path := f.Name()
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		return "", func() {}, err
	}
	_ = f.Close()
	return path, func() { _ = os.Remove(path) }, nil
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
