package model

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

// AgentEvent is the versioned, durable boundary between RPC, the UI and macOS.
// Seq is a database cursor, not a per-connection counter.
type AgentEvent struct {
	Version      int             `json:"version"`
	ContentIndex int             `json:"contentIndex,omitempty"`
	Seq          int64           `json:"seq"`
	Type         string          `json:"type"`
	RunID        string          `json:"runId"`
	Text         string          `json:"text,omitempty"`
	Tool         string          `json:"tool,omitempty"`
	ToolCallID   string          `json:"toolCallId,omitempty"`
	Args         json.RawMessage `json:"args,omitempty"`
	Output       string          `json:"output,omitempty"`
	DurationMS   int64           `json:"durationMs,omitempty"`
	IsError      bool            `json:"isError,omitempty"`
	Aborted      bool            `json:"aborted,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	Todos        []Todo          `json:"todos,omitempty"`
	Render       *Render         `json:"render,omitempty"`
	Critique     *Critique       `json:"critique,omitempty"`
	At           int64           `json:"at"`
}

type ToolActivity struct {
	ID         string          `json:"id"`
	Tool       string          `json:"tool"`
	Label      string          `json:"label"`
	Args       json.RawMessage `json:"args,omitempty"`
	Output     string          `json:"output,omitempty"`
	Status     string          `json:"status"`
	StartedAt  int64           `json:"startedAt"`
	EndedAt    int64           `json:"endedAt,omitempty"`
	DurationMS int64           `json:"durationMs,omitempty"`
}

// Activity is a materialized projection of the event journal. All clients use
// this snapshot; native surfaces never infer execution state from web chrome.
type Activity struct {
	RunID          string          `json:"runId"`
	ProjectID      string          `json:"projectId"`
	ProjectName    string          `json:"projectName"`
	ConversationID string          `json:"conversationId,omitempty"`
	Status         string          `json:"status"`
	Phase          string          `json:"phase"`
	Label          string          `json:"label"`
	Prompt         string          `json:"prompt,omitempty"`
	Context        json.RawMessage `json:"context,omitempty"`
	Model          string          `json:"model,omitempty"`
	ModelID        string          `json:"modelId,omitempty"`
	Provider       string          `json:"provider,omitempty"`
	StartedAt      int64           `json:"startedAt"`
	UpdatedAt      int64           `json:"updatedAt"`
	EndedAt        int64           `json:"endedAt,omitempty"`
	Seq            int64           `json:"seq"`
	Text           string          `json:"text"`
	MessageText    string          `json:"messageText,omitempty"`
	TextBlocks     map[int]string  `json:"textBlocks,omitempty"`
	Tools          []ToolActivity  `json:"tools"`
	Attention      bool            `json:"attention"`
	AttentionSeq   int64           `json:"attentionSeq,omitempty"`
	Request        json.RawMessage `json:"request,omitempty"`
	Error          string          `json:"error,omitempty"`
	Todos          []Todo          `json:"todos,omitempty"`
	Renders        []Render        `json:"renders,omitempty"`
	Critiques      []Critique      `json:"critiques,omitempty"`
}

func NewActivity(r Run, p Project) Activity {
	return Activity{RunID: r.ID, ProjectID: r.ProjectID, ProjectName: p.Name, ConversationID: r.ConversationID, Status: r.Status, Phase: "starting", Label: "Starting π", Prompt: r.Prompt, StartedAt: r.StartedAt.UnixMilli(), UpdatedAt: r.StartedAt.UnixMilli(), Tools: []ToolActivity{}}
}

// ToolLabel is descriptive, not a claim about what a command will accomplish.
func ToolLabel(name string, args json.RawMessage) string {
	var a map[string]any
	_ = json.Unmarshal(args, &a)
	target, _ := a["path"].(string)
	if target == "" {
		target, _ = a["file_path"].(string)
	}
	target = filepath.Base(target)
	if target == "." {
		target = "file"
	}
	switch name {
	case "read":
		return "Reading " + target
	case "write":
		return "Writing " + target
	case "edit", "multi_edit":
		return "Editing " + target
	case "bash":
		return "Running command"
	case "render_preview":
		return "Rendering preview"
	case "todo_write":
		return "Updating plan"
	case "submit_critique":
		return "Reviewing design"
	default:
		return "Using " + name
	}
}

func (a *Activity) Apply(e AgentEvent) {
	a.Seq, a.UpdatedAt = e.Seq, e.At
	switch e.Type {
	case "text":
		prefix := strings.TrimSuffix(a.Text, a.MessageText)
		if a.TextBlocks == nil {
			a.TextBlocks = map[int]string{}
		}
		a.TextBlocks[e.ContentIndex] += e.Text
		keys := make([]int, 0, len(a.TextBlocks))
		for k := range a.TextBlocks {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		var message strings.Builder
		for _, k := range keys {
			message.WriteString(a.TextBlocks[k])
		}
		a.MessageText = message.String()
		a.Text = prefix + a.MessageText
		a.Phase = "responding"
		a.Label = "Writing response"
	case "message_start":
		a.TextBlocks = map[int]string{}
		a.MessageText = ""
		a.Phase = "waiting"
		a.Label = "Waiting for model response"
	case "message_end":
		// The final message replaces deltas rather than duplicating them.
		a.Text = strings.TrimSuffix(a.Text, a.MessageText) + e.Text
		a.MessageText = e.Text
	case "turn":
		if a.Text != "" && !strings.HasSuffix(a.Text, "\n\n") {
			a.Text += "\n\n"
		}
		a.MessageText = ""
	case "context":
		a.Context = e.Data
	case "design_route":
		var fields map[string]json.RawMessage
		if json.Unmarshal(a.Context, &fields) != nil || fields == nil {
			fields = map[string]json.RawMessage{}
		}
		fields["designRoute"] = e.Data
		a.Context, _ = json.Marshal(fields)
	case "model":
		a.Model = e.Text
		var m struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		}
		_ = json.Unmarshal(e.Data, &m)
		a.ModelID, a.Provider = m.ID, m.Provider
	case "thinking":
		a.Phase = "thinking"
		a.Label = "Model is thinking"
	case "tool", "tool_update", "tool_end":
		i := -1
		for n := range a.Tools {
			if a.Tools[n].ID == e.ToolCallID {
				i = n
				break
			}
		}
		if i < 0 {
			a.Tools = append(a.Tools, ToolActivity{ID: e.ToolCallID, Tool: e.Tool, Label: ToolLabel(e.Tool, e.Args), Args: e.Args, Status: "running", StartedAt: e.At})
			i = len(a.Tools) - 1
		}
		t := &a.Tools[i]
		if len(e.Args) > 0 {
			t.Args = e.Args
			t.Label = ToolLabel(e.Tool, e.Args)
		}
		if e.Type != "tool" {
			t.Output = e.Output
		}
		a.Phase = "tool"
		a.Label = t.Label
		if e.Type == "tool_end" {
			t.Status = "succeeded"
			if e.IsError {
				t.Status = "failed"
			}
			t.EndedAt = e.At
			t.DurationMS = e.DurationMS
			a.Phase = "waiting"
			a.Label = "Waiting for model response"
			for _, active := range a.Tools {
				if active.Status == "running" {
					a.Phase = "tool"
					a.Label = active.Label
					break
				}
			}
		}
	case "retry":
		a.Phase = "retrying"
		a.Label = "Retrying model request"
	case "compaction_start":
		a.Phase = "compacting"
		a.Label = "Compacting conversation"
	case "compaction_end", "retry_end", "agent_start":
		a.Phase = "waiting"
		a.Label = "Waiting for model response"
	case "attention":
		a.AttentionSeq = e.Seq
		a.Attention = true
		a.Request = e.Data
		a.Phase = "attention"
		a.Label = "Needs your answer"
	case "attention_resolved":
		a.Attention = false
		a.Request = nil
		a.Phase = "waiting"
		a.Label = "Waiting for model response"
	case "todos":
		a.Todos = e.Todos
	case "render":
		if e.Render != nil {
			a.Renders = append(a.Renders, *e.Render)
		}
	case "critique":
		if e.Critique != nil {
			found := false
			for i, c := range a.Critiques {
				if c.Round == e.Critique.Round {
					a.Critiques[i] = *e.Critique
					found = true
				}
			}
			if !found {
				a.Critiques = append(a.Critiques, *e.Critique)
			}
		}
	case "finishing":
		if !a.Attention {
			a.Phase = "finishing"
			a.Label = "Finishing run"
		}
	case "error":
		a.Error = e.Text
	case "done", "canceled", "run_failed":
		a.Status = "succeeded"
		if e.Type == "done" {
			a.Error = ""
		}
		a.Phase = "finished"
		a.Label = "Finished"
		if e.Type == "canceled" {
			a.Status = "canceled"
			a.Label = "Stopped"
		}
		if e.Type == "run_failed" {
			a.Status = "failed"
			a.Label = "Run failed"
			a.Error = e.Text
		}
		if e.Type != "done" {
			a.Attention = false
			a.Request = nil
		}
		if a.Attention && e.Type == "done" {
			a.Phase = "attention"
			a.Label = "Needs your answer"
		}
		a.EndedAt = e.At
		for i := range a.Tools {
			if a.Tools[i].Status == "running" {
				a.Tools[i].Status = "interrupted"
				a.Tools[i].EndedAt = e.At
			}
		}
	}
}
