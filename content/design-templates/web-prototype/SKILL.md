---
name: web-prototype
description: |
  General-purpose self-contained HTML prototype for task interfaces, marketing,
  editorial, documentation, and other web experiences. Choose hierarchy and
  interaction from the brief; reuse the technical baseline and optional layout
  techniques without imposing a default page skeleton.
triggers:
  - "prototype"
  - "mockup"
  - "landing"
  - "single page"
  - "marketing page"
  - "homepage"
od:
  mode: prototype
  platform: desktop
  scenario: design
  preview:
    type: html
    entry: index.html
  design_system:
    requires: true
    sections: [color, typography, layout, components]
---

# Web Prototype Skill

Create a single, self-contained HTML prototype for the user's actual task. Treat
the bundled files as optional reference material, not a prescribed design
system or page skeleton. Build the information architecture, visual treatment,
and interactions that fit the brief.

## Resource map

```
web-prototype/
├── SKILL.md                ← workflow and quality bar
├── assets/
│   └── template.html       ← minimal document and accessibility baseline
└── references/
    ├── layouts.md          ← optional structural examples
    └── checklist.md        ← brief-fit and implementation checks
```

## Workflow

### Step 0 — Understand the task and content

Read the user's brief, the active DESIGN.md (if supplied in context), and any
existing prototype being refined. Establish:

- who the prototype is for and what they need to do or understand;
- the mode that fits the request (for example, a task-oriented application,
  product or marketing page, editorial experience, documentation, storefront,
  or a focused interaction);
- the information hierarchy: primary task or message, supporting content,
  important states, and secondary actions;
- explicit brand, platform, reference-fidelity, accessibility, and technical
  constraints.

Use the requested mode, not a generic marketing-page structure. A task UI
should foreground its real workflow, data, and states; it does not need a
marketing hero, feature grid, or closing CTA. Don't invent facts, metrics,
testimonials, product capabilities, or content to fill space. If details are
missing, make only safe, clearly bounded assumptions.

For a refinement, preserve working content, interactions, navigation, and
visual decisions unless the request changes them. Keep existing `data-od-id`
values stable so comment targets continue to refer to the same sections.

### Step 1 — Choose structure before styling

Plan the structure around the content hierarchy and user flow: decide what
belongs together, what deserves focus, and which actions or states need to be
visible. Use custom sections and interactions when the task calls for them;
the examples in `references/layouts.md` are optional starting points, not an
exclusive menu. The seed does not provide page chrome or a default section
sequence.

When the request is open-ended or asks for design exploration, compare
meaningfully different structural concepts under the same brand and content
constraints. Compare how they organize information and support the task, not
just their colors. Choose the best-fitting direction or show the requested
options. Do not randomize variants for their own sake. For a specific
implementation, refinement, or reference/clone request, follow that direction
instead of imposing novelty.

### Step 2 — Apply the visual system

Honor any named brand or active DESIGN.md, including its colors, typography,
spacing, components, and tone. Do not compress it into a fixed token count or
replace it with a house palette. Where the brief leaves choices open, select
type, color, density, geometry, and imagery to support this product and mode;
there is no default serif, eyebrow, accent budget, or decorative-label style.
Keep visual choices legible, consistent, and appropriate to the amount and
kind of content.

Use `assets/template.html` as a technical document baseline, not a design
template. Add the styles and any needed local interactions for the prototype.
Do not copy in chrome or content that the user did not ask for.

### Step 3 — Build the prototype

Write the requested experience in the canonical project HTML file, normally
`index.html`. Use semantic HTML and native controls where practical. Every
visible control must have a meaningful destination or working behavior;
implement important interaction states rather than showing inert decoration.
Ensure keyboard access, visible focus, clear labels, readable contrast, and
responsive behavior.

Use real, project-local assets when imagery is needed. For a named
real-world referent, use the correct real asset rather than a generated,
look-alike, or fictional substitute. Preserve content-image geometry: inspect
the intrinsic dimensions, include matching `width` and `height` attributes,
and show the full frame unless the image is explicitly a decorative,
croppable fill. Do not distort, silently crop, or add imagery merely to fill
space. Never depend on remote hotlinks or expiring asset URLs.

If sections are used, put a stable, unique `data-od-id` on each top-level
`<section>` for comment mode. Adapt identifiers when adding sections; do not
rename existing targets during unrelated refinements.

### Step 4 — Review against the brief

Use `references/checklist.md` to check both implementation and fit. Confirm
that the prototype addresses the requested task, that its structure follows
the established hierarchy, and that nothing optional has become mandatory by
habit. Fix failures before delivery.

### Step 5 — Deliver

Filesystem runs use project files as the source of truth:

```
index.html
```

OpenDesign derives the preview from the written project file. Do not also emit
a source-code `<artifact>` block for the same generation turn. After writing
the file, give one short summary naming it and describing the experience.
Do not output the full HTML source in chat.
