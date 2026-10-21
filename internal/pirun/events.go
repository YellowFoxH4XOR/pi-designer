package pirun

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
		s.emit(Event{Type: EventAgentStart})
	case "agent_end":
		s.emit(Event{Type: EventAgentEnd})
	case "agent_settled":
		var e struct {
			Aborted bool `json:"aborted"`
		}
		if json.Unmarshal(line, &e) == nil {
			s.emit(Event{Type: EventSettled, Aborted: e.Aborted})
		}
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
				Role         string `json:"role"`
				StopReason   string `json:"stopReason"`
				ErrorMessage string `json:"errorMessage"`
				Content      []struct {
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
		if e.Message.StopReason == "error" {
			message := e.Message.ErrorMessage
			if message == "" {
				message = "The selected π model could not complete its request. Check its authentication and availability."
			}
			s.emit(Event{Type: EventError, Text: message})
		} else {
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
	case "auto_retry_start", "summarization_retry_scheduled", "summarization_retry_attempt_start":
		s.emit(Event{Type: "retry", Data: line})
	case "auto_retry_end", "summarization_retry_finished":
		s.emit(Event{Type: "retry_end", Data: line})
	case "compaction_start", "compaction_end":
		s.emit(Event{Type: kind, Data: line})
	case "extension_ui_request":
		var e struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(line, &e) == nil {
			s.emit(Event{Type: "extension_ui", Data: line})
		}
	case "extension_error":
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(line, &e) == nil {
			s.emit(Event{Type: "diagnostic", Text: e.Error})
		}
	case "queue_update", "session_info_changed", "thinking_level_changed":
		s.emit(Event{Type: "session_update", Data: line})
	}
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
	// to the π session; the UI explicitly labels bounded text previews.
	if len(text) > 16384 {
		text = text[:16384] + "\n[Output preview truncated; complete output remains in the π session.]"
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
