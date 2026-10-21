package omprun

import (
	"encoding/json"
	"strings"
)

// Protocol adaptation lives here, separate from process/command ownership.
// Keep authoritative message_end content and toolCallId. Tool partialResult is
// a snapshot, not an append-only string. Never expose thinking text as a log.
func (s *Session) handleEvent(line []byte, kind string) {
	switch kind {
	case "agent_start":
		s.settledEmitted = false
		s.emit(Event{Type: EventAgentStart})
	case "agent_end":
		s.emit(Event{Type: EventAgentEnd})
	case "prompt_result":
		var e struct {
			AgentInvoked   bool   `json:"agentInvoked"`
			Status         string `json:"status"`
			SessionSettled bool   `json:"sessionSettled"`
			Error          *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &e) != nil {
			return
		}
		s.promptAborted = e.Status == "aborted"
		if e.Status == "error" {
			message := ""
			if e.Error != nil {
				message = e.Error.Message
			}
			if message == "" {
				message = "The selected omp model could not complete its request. Check its authentication and availability."
			}
			s.emit(Event{Type: EventError, Text: message})
		}
		// A prompt that never reached the agent gets no session_settled; one
		// whose session is already quiet may be torn down right away.
		if !e.AgentInvoked || e.SessionSettled {
			s.settle()
		}
	case "session_settled":
		s.settle()
	case "message_start":
		var e struct {
			Message struct {
				Role string `json:"role"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) == nil && e.Message.Role == "assistant" {
			s.emit(Event{Type: "message_start"})
		}
	case "message_update":
		var e struct {
			AssistantMessageEvent struct {
				Type         string `json:"type"`
				Delta        string `json:"delta"`
				ContentIndex int    `json:"contentIndex"`
			} `json:"assistantMessageEvent"`
		}
		if json.Unmarshal(line, &e) != nil {
			return
		}
		if e.AssistantMessageEvent.Type == "text_delta" {
			s.emit(Event{Type: EventTextDelta, Text: e.AssistantMessageEvent.Delta, ContentIndex: e.AssistantMessageEvent.ContentIndex})
		}
		if e.AssistantMessageEvent.Type == "thinking_start" {
			s.emit(Event{Type: EventThinkingDelta})
		}
	case "message_end":
		var e struct {
			Message struct {
				Role       string `json:"role"`
				StopReason string `json:"stopReason"`
				Content    []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) != nil || e.Message.Role != "assistant" {
			return
		}
		var text strings.Builder
		for _, c := range e.Message.Content {
			if c.Type == "text" {
				text.WriteString(c.Text)
			}
		}
		s.emit(Event{Type: "message_end", Text: text.String()})
		// A provider error here may still be retried; prompt_result reports
		// the authoritative outcome once omp's retries are exhausted.
		if e.Message.StopReason != "error" {
			s.emit(Event{Type: EventModelResult})
		}
	case "tool_execution_start", "tool_execution_update", "tool_execution_end":
		var e struct {
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			Args       json.RawMessage `json:"args"`
			IsError    bool            `json:"isError"`
			DurationMS int64           `json:"durationMs"`
			Result     toolResult      `json:"result"`
			Partial    toolResult      `json:"partialResult"`
		}
		if json.Unmarshal(line, &e) != nil {
			return
		}
		event := Event{ToolCallID: e.ToolCallID, ToolName: e.ToolName, ToolArgs: e.Args, IsError: e.IsError, DurationMS: e.DurationMS}
		switch kind {
		case "tool_execution_start":
			event.Type = EventToolStart
		case "tool_execution_update":
			event.Type = "tool_update"
			event.Output = resultText(e.Partial)
		case "tool_execution_end":
			event.Type = EventToolEnd
			event.Output = resultText(e.Result)
			event.ToolDetails = e.Result.Details
		}
		s.emit(event)
	case "auto_retry_start":
		s.emit(Event{Type: "retry", Data: line})
	case "auto_retry_end":
		s.emit(Event{Type: "retry_end", Data: line})
	case "auto_compaction_start":
		s.emit(Event{Type: "compaction_start", Data: line})
	case "auto_compaction_end":
		s.emit(Event{Type: "compaction_end", Data: line})
	case "extension_ui_request":
		var e struct {
			Method string `json:"method"`
		}
		// Dialogs need an answer and notify is user-facing text. Widget,
		// status and title updates are omp terminal presentation state with
		// no meaning in πDesigner, and would flood the durable event log.
		if json.Unmarshal(line, &e) == nil && forwardedUIMethods[e.Method] {
			s.emit(Event{Type: "extension_ui", Data: line})
		}
	case "extension_error":
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(line, &e) == nil {
			s.emit(Event{Type: "diagnostic", Text: e.Error})
		}
	case "rpc_frame_error":
		s.emit(Event{Type: "diagnostic", Text: "omp dropped an event that exceeded its transport limit"})
	case "queue_update", "session_info_update", "thinking_level_changed", "model_changed":
		s.emit(Event{Type: "session_update", Data: line})
	}
}

var forwardedUIMethods = map[string]bool{"select": true, "confirm": true, "input": true, "editor": true, "notify": true}

// settle emits EventSettled once per stretch of agent activity, carrying
// whether the last prompt was aborted.
func (s *Session) settle() {
	if s.settledEmitted {
		return
	}
	s.settledEmitted = true
	s.emit(Event{Type: EventSettled, Aborted: s.promptAborted})
}

type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Details json.RawMessage `json:"details"`
}

func resultText(r toolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(c.Text)
		}
	}
	text := b.String()
	// Binary/image blocks never enter the log. The complete result still belongs
	// to the omp session; the UI explicitly labels bounded text previews.
	if len(text) > 16384 {
		text = text[:16384] + "\n[Output preview truncated; complete output remains in the omp session.]"
	}
	return strings.ToValidUTF8(text, "�")
}

// RespondUI is not an RPC command: extension responses have no command reply.
func (s *Session) RespondUI(response map[string]any) error {
	response["type"] = "extension_ui_response"
	b, err := json.Marshal(response)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.stdin.Write(append(b, '\n'))
	return err
}
