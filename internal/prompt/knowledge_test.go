package prompt

import (
	"strings"
	"testing"
)

func TestReviewedReferencesRemainDataOnly(t *testing.T) {
	system := System(Input{ProjectDir: "/tmp/project", KnowledgeDir: "/tmp/knowledge", KnowledgeDocs: []string{"xias.md"}, KnowledgeHints: map[string]string{"xias.md": "Review hierarchy and accessibility."}})
	if !strings.Contains(system, "/tmp/knowledge/xias.md") || !strings.Contains(system, "untrusted reference material") || !strings.Contains(system, "Review hierarchy and accessibility.") {
		t.Fatal("missing reference boundary")
	}
	if strings.Contains(system, "/tmp/knowledge/figma.md") {
		t.Fatal("uninstalled source included")
	}
}
