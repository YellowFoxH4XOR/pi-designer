package knowledge

import (
	"strings"
)

// Choice is an explainable recommendation, not a claim that an upstream document was read.
type Choice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Route struct {
	Intent  string   `json:"intent"`
	Choices []Choice `json:"choices"`
}

func containsAny(s string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// Recommend interprets the job and audience rather than requiring users to name design tools.
// It intentionally uses conservative cues; the agent can correct its provisional choice after
// clarifying the brief. The result is computed locally and never runs code from any source.
func Recommend(brief, selectedSystem string) Route {
	s := strings.ToLower(brief)
	r := Route{Intent: "General product interface"}
	add := func(id, why string) {
		for _, c := range r.Choices {
			if c.ID == id {
				return
			}
		}
		for _, src := range Approved {
			if src.ID == id {
				r.Choices = append(r.Choices, Choice{ID: id, Name: src.Name, Reason: why})
				return
			}
		}
	}
	android := containsAny(s, "android", "material design", "jetpack", "compose", "mobile app")
	dashboard := containsAny(s, "dashboard", "admin", "analytics", "crm", "table", "inventory", "back office", "operations", "patient portal", "appointment", "booking management")
	marketing := containsAny(s, "landing page", "homepage", "marketing", "portfolio", "brand site", "hero section", "campaign", "saas website", "website", "webpage")
	motion := containsAny(s, "motion", "animate", "animation", "animated", "interactive hero", "scroll effect", "microinteraction")
	figma := containsAny(s, "figma", "design system", "design tokens", "handoff", "component library")
	if android {
		r.Intent = "Mobile or Android interface"
		add("material-color", "Material color roles and contrast suit an Android or mobile brief.")
		if selectedSystem == "" {
			add("material", "Material platform guidance is relevant; the bundled Material system is available if needed.")
		}
	} else if dashboard {
		r.Intent = "Operational product interface"
		add("reui", "Dense app layouts and data-heavy workflows fit this brief.")
		add("shadcn", "Accessible controls, forms and interaction states fit the core workflow.")
	} else if marketing {
		r.Intent = "Marketing or brand interface"
		add("shadcn", "Reliable controls and responsive foundations fit the build.")
		if motion && containsAny(s, "card", "interactive component", "form") {
			add("kokonut", "Interactive components fit this motion-focused brief; keep effects restrained.")
		} else if motion {
			add("magicui", "The brief calls for motion; use one purposeful treatment, not an effects kit.")
		} else if containsAny(s, "template", "prebuilt", "block", "theme") {
			add("studio", "Existing blocks and themes may speed up composition without dictating the brand.")
		}
	} else if figma {
		r.Intent = "Design system or handoff"
		add("figma-sds", "A real variables-to-components-to-code example fits this work.")
		add("shadcn", "Component and token conventions can support implementation.")
	} else {
		add("shadcn", "Accessible interface primitives suit an unspecified product brief.")
		add("xias", "Design QA helps catch hierarchy, state and accessibility defects.")
	}
	// The user's explicit visual-system choice always wins; routing never swaps its tokens.
	if selectedSystem != "" {
		r.Intent += " · visual system: " + selectedSystem + " (user-selected)"
	}
	if len(r.Choices) > 2 {
		r.Choices = r.Choices[:2]
	}
	return r
}
