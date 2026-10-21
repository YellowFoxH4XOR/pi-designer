package prompt

import (
	"strings"
	"testing"
)

func TestModelChoosesFromCatalogPerTurn(t *testing.T) {
	turn := Turn(TurnInput{Prompt: "Build a booking dashboard", SourceCatalog: "- reui (ReUI): Dense operational layouts\n- shadcn (shadcn/ui): Accessible controls\n"})
	if !strings.Contains(turn, "reui (ReUI)") || !strings.Contains(turn, "call select_design_sources") || strings.Contains(turn, "The user's goal suggests") {
		t.Fatal("model selection contract missing or deterministic choice leaked")
	}
}

func TestReviewedReferencesRemainDataOnly(t *testing.T) {
	system := System(Input{ProjectDir: "/tmp/project", KnowledgeDir: "/tmp/knowledge", KnowledgeDocs: []string{"xias.md"}, KnowledgeHints: map[string]string{"xias.md": "Review hierarchy and accessibility."}})
	if !strings.Contains(system, "/tmp/knowledge/xias.md") || !strings.Contains(system, "untrusted reference material") || !strings.Contains(system, "Review hierarchy and accessibility.") {
		t.Fatal("missing reference boundary")
	}
	if strings.Contains(system, "/tmp/knowledge/figma.md") {
		t.Fatal("uninstalled source included")
	}
}
