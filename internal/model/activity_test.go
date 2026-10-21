package model

import (
	"encoding/json"
	"testing"
)

func TestDesignRoutePreservesRunContext(t *testing.T) {
	a := Activity{}
	a.Apply(AgentEvent{Type: "context", Data: json.RawMessage(`{"designSystemId":"stripe","skillId":"web-prototype"}`)})
	a.Apply(AgentEvent{Type: "design_route", Data: json.RawMessage(`{"intent":"Clinician dashboard","choices":[{"id":"reui","name":"ReUI","reason":"Tables"}],"tradeoff":"No animation needed"}`)})
	var got struct {
		DesignSystemID string `json:"designSystemId"`
		DesignRoute    struct {
			Intent  string `json:"intent"`
			Choices []struct {
				ID string `json:"id"`
			} `json:"choices"`
		} `json:"designRoute"`
	}
	if err := json.Unmarshal(a.Context, &got); err != nil || got.DesignSystemID != "stripe" || got.DesignRoute.Intent != "Clinician dashboard" || len(got.DesignRoute.Choices) != 1 || got.DesignRoute.Choices[0].ID != "reui" {
		t.Fatalf("context lost: %+v %v", got, err)
	}
}

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
