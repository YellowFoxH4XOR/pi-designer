package prompt

import (
	"strings"
	"testing"
)

func TestSystemBindsDesignSystemAsTheDirection(t *testing.T) {
	with := System(Input{ProjectDir: "/p", DesignSystem: &DesignSystem{Name: "Linear", Dir: "/ds", DesignMD: "# Linear\n\nCalm.", TokensCSS: ":root{--accent:#5e6ad2}"}})
	if !strings.Contains(with, ":root{--accent:#5e6ad2}") || !strings.Contains(with, "it is the visual direction") {
		t.Fatal("active design system tokens or direction lock missing")
	}
	if strings.Contains(with, "No design system is active") {
		t.Fatal("active design system prompt also claims none is active")
	}
	// The brand document's headings must not become top-level prompt sections.
	if strings.Contains(with, "\n# Linear") {
		t.Fatal("design system headings were not demoted")
	}

	without := System(Input{ProjectDir: "/p"})
	if !strings.Contains(without, "No design system is active") {
		t.Fatal("missing own-direction instruction without a design system")
	}
}

func TestSystemPreflightNamesOnlyFilesTheSkillUses(t *testing.T) {
	body := "---\nname: web-prototype\n---\n# Web\nRead `assets/template.html` then `references/checklist.md`."
	s := System(Input{ProjectDir: "/p", Skill: &Skill{ID: "web-prototype", Name: "web-prototype", Dir: "/skills/wp", Body: body}})
	if !strings.Contains(s, "`/skills/wp/assets/template.html`") || !strings.Contains(s, "`/skills/wp/references/checklist.md`") {
		t.Fatal("pre-flight does not name the skill's seed and checklist")
	}
	if strings.Contains(s, "/skills/wp/references/layouts.md") {
		t.Fatal("pre-flight names a file the skill never references")
	}
	if strings.Contains(s, "name: web-prototype") {
		t.Fatal("skill frontmatter leaked into the prompt")
	}
}

func TestFrontendDesignIsNotRepeatedAsTheActiveSkill(t *testing.T) {
	fd := "---\nname: frontend-design\n---\nWork in two passes."
	s := System(Input{ProjectDir: "/p", FrontendDesign: fd, Skill: &Skill{ID: FrontendDesignSkillID, Name: "frontend-design", Dir: "/fd", Body: fd}})
	if strings.Count(s, "Work in two passes.") != 1 {
		t.Fatalf("frontend-design appears %d times", strings.Count(s, "Work in two passes."))
	}
}

func TestTurnLocksSubmittedFormAnswers(t *testing.T) {
	answer := "[form answers — discovery]\n- What are we making?: Landing [value: landing]"
	msg := Turn(TurnInput{Prompt: answer, Viewport: "mobile"})
	if !strings.Contains(msg, "form answers; they are authoritative") {
		t.Fatal("form answers are not locked")
	}
	if !strings.HasSuffix(msg, answer) {
		t.Fatal("user request must close the message verbatim")
	}
	if !strings.Contains(msg, "Mobile frame (390 × 844)") {
		t.Fatal("primary viewport missing")
	}
	plain := Turn(TurnInput{Prompt: "Make the hero darker"})
	if strings.Contains(plain, "authoritative") || !strings.Contains(plain, "Web frame") {
		t.Fatal("ordinary turn got the form override or lost the default viewport")
	}
}

func TestTurnReplaysTruncatedTranscriptAndHonorsSkipDiscovery(t *testing.T) {
	long := strings.Repeat("x", maxReplayChars+50)
	msg := Turn(TurnInput{Prompt: "Next", SkipDiscovery: true, Transcript: []Message{{Role: "user", Content: "Build a page"}, {Role: "assistant", Content: long}}})
	if !strings.Contains(msg, "[user]\nBuild a page") || !strings.Contains(msg, "[truncated 50 characters]") {
		t.Fatal("transcript replay missing or untruncated")
	}
	if !strings.Contains(msg, "Do not emit a `<question-form>`") {
		t.Fatal("skip-discovery override missing")
	}
}

func TestDeckDisciplineOnlyForDeckSkills(t *testing.T) {
	deckSkill := &Skill{ID: "simple-deck", Name: "simple-deck", Dir: "/d", Mode: DeckMode, Body: "# Deck\nCopy `assets/template.html`."}
	if s := System(Input{ProjectDir: "/p", Skill: deckSkill}); !strings.Contains(s, "## Slide deck discipline") {
		t.Fatal("deck skill did not get deck discipline")
	}
	page := &Skill{ID: "web-prototype", Name: "web-prototype", Dir: "/w", Mode: "prototype", Body: "# Web"}
	if s := System(Input{ProjectDir: "/p", Skill: page}); strings.Contains(s, "## Slide deck discipline") {
		t.Fatal("page skill got deck discipline")
	}
}
