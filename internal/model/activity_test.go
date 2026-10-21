package model

import "testing"

func TestTextContentBlocksAreReconstructedByIndex(t *testing.T) {
	a := Activity{Tools: []ToolActivity{}}
	for _, e := range []AgentEvent{
		{Type: "message_start"},
		{Type: "text", ContentIndex: 1, Text: "second"},
		{Type: "text", ContentIndex: 0, Text: "first "},
		{Type: "text", ContentIndex: 1, Text: " block"},
	} {
		a.Apply(e)
	}
	if a.Text != "first second block" {
		t.Fatalf("out-of-order content blocks: %q", a.Text)
	}
	a.Apply(AgentEvent{Type: "message_end", Text: "final authoritative text"})
	if a.Text != "final authoritative text" {
		t.Fatal("final message did not replace partial blocks")
	}
	a.Apply(AgentEvent{Type: "turn"})
	a.Apply(AgentEvent{Type: "message_start"})
	a.Apply(AgentEvent{Type: "text", Text: "next"})
	if a.Text != "final authoritative text\n\nnext" {
		t.Fatalf("next turn duplicated blocks: %q", a.Text)
	}
}
