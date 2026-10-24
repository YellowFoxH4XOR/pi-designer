---
name: πDesigner
description: A light local design workspace with cool neutral layers and warm coral actions
colors:
  brand-coral: "#E45F3E"
  brand-paper: "#FFF5E7"
  brand-ink: "#233137"
  accent: "#9d452d"
  accent-soft: "#f9ece6"
  action: "#bd5033"
  on-action: "#ffffff"
  shell: "#f4f5f7"
  rail: "#eef0f3"
  canvas: "#fbfbfc"
  surface: "#ffffff"
  raised: "#e9ecf0"
  ink: "#24272e"
  muted: "#626873"
  faint: "#666d78"
  line: "#dce0e6"
  subtle: "#f2f3f5"
  selected: "#e0e4eb"
  success: "#33734b"
  success-soft: "#edf6f0"
  warning: "#8b5c1b"
  warning-soft: "#fff5e5"
  error: "#a13c3a"
  error-soft: "#fdf0ee"
typography:
  display:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "40px"
    fontWeight: 550
    lineHeight: 1.18
    letterSpacing: "-1.3px"
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "25px"
    fontWeight: 620
    letterSpacing: "-0.7px"
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "18px"
    fontWeight: 600
    letterSpacing: "-0.4px"
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "14px"
    fontWeight: 400
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "12px"
    fontWeight: 550
  code:
    fontFamily: "'SFMono-Regular', Consolas, 'Liberation Mono', monospace"
    fontSize: "11px"
    lineHeight: 1.85
rounded:
  field: "6px"
  control: "7px"
  button: "9px"
  starter: "10px"
  card: "11px"
  composer: "13px"
  dialog: "15px"
spacing:
  compact: "8px"
  control: "12px"
  group: "16px"
  section: "24px"
  page: "36px"
components:
  button-primary:
    backgroundColor: "{colors.action}"
    textColor: "{colors.on-action}"
    rounded: "{rounded.button}"
    padding: "11px 17px"
    height: "40px"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.button}"
    padding: "11px 17px"
  navigation-selected:
    backgroundColor: "{colors.selected}"
    textColor: "{colors.ink}"
    rounded: "{rounded.button}"
    height: "43px"
  workflow-selected:
    textColor: "{colors.ink}"
    padding: "10px 1px"
  composer:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.composer}"
    padding: "20px 18px 14px"
---

# Design System: πDesigner

## Overview

**Creative North Star: "The light creative studio"**

A focused Mac workspace with cool gray navigation, an off-white canvas, white working surfaces, and restrained warm coral actions. The user's latest correction establishes a light theme while preserving the redesigned layout and behavior. Fine borders and tonal layers separate the brief, conversation, and artifact without glows or decorative motion.

The retained vermilion, ivory, and ink pen-nib mark gives the product its designer-tool identity. Its editable Bézier curve appears as a small authored illustration on Home. Prototype specimens may show other palettes inside their own bounds; the workspace remains consistent around them. The native window uses aqua with the shell background, so web content and AppKit form one light workspace.

**Key Characteristics:**
- Cool neutral layers and fine rules.
- Restrained warm coral actions and visible focus.
- Persistent navigation and a brief-first creation surface.
- An artifact-led Studio with faithful execution states.

## Colors

Warm coral carries action and focus; light neutral layers carry structure. The frontmatter is the normative palette, matching the CSS custom properties except for the retained logo's own identity colors.

### Primary

- **Deep Coral** (`accent`): focus rings, selected workflow underline, chosen resources, and reported working state.
- **Warm Action** (`action`, `on-action`): primary and send actions pair a warm coral fill with white text. The observed hover fill is `#ae472c`.
- **Coral Tint** (`accent-soft`): selected resources and subtle emphasis, not a broad page background.

### Secondary

- **Brand Vermilion, Ivory, and Deep Ink** (`brand-coral`, `brand-paper`, `brand-ink`): retained app-mark colors, not replacements for shell state colors.

### Neutral

- **Shell Mist** (`shell`) and **Rail Gray** (`rail`): outer window and persistent navigation.
- **Off-white Canvas** (`canvas`): internally scrolling workspace and dialogs.
- **Working White** (`surface`), **Raised Gray** (`raised`), and **Subtle Gray** (`subtle`): controls, composer, hover layers, and bounded message/code regions.
- **Selected Gray** (`selected`): active navigation.
- **Charcoal Ink** (`ink`), **Muted Slate** (`muted`), and **Faint Slate** (`faint`): primary, secondary, and tertiary content.
- **Cool Gray Rule** (`line`): panel boundaries and dividers.

Success green, warning amber, and failure rose each have a soft light counterpart. They communicate real execution outcomes alongside explicit text labels.

**The Artifact Boundary Rule.** Workspace colors stop at the prototype iframe. A selected design system controls the artifact’s colors; it never recolors the app shell.

## Typography

The Mac system stack carries navigation, headings, controls, and conversations. Monospace is reserved for source, paths, and measured dimensions. The frontmatter captures the implemented roles; Home's display role is an observed surface choice, not a requirement to use display typography on every page.

The Home heading is left aligned and steps down to 36px at the compact desktop breakpoint and 32px at the minimum desktop breakpoint. Library headings use the headline role; dialogs use the title role. Body text is 14px at the root; navigation, tools, and conversation regions use denser 11–13px working text. Long agent messages render as paragraphs, lists, emphasis, and fenced code rather than raw Markdown.

## Layout

The sidebar is 224px at the normal desktop width, 212px on narrower windows, and 190px at the minimum width. A 60px utility header and 29px footer frame a flat, internally scrolling canvas. The document itself remains viewport-bound; named panels own scrolling. AppKit owns the native minimum size so page zoom does not create document overflow.

Home uses a centered container capped at 1080px with left-aligned content and 64px horizontal padding at the normal width. Its heading and small editable-curve illustration precede workflow tabs, the full-width composer, three visual starter briefs, and recent projects. Compact windows reduce page padding; the starter grid retains three columns while recent projects become one column. Libraries use searchable grids or task-oriented lists.

Studio removes the navigation rail and has a 350px conversation column beside a flexible preview column, adjusting to 310px on narrower windows and 390px on wide windows. The native content minimum is 900×620. Breakpoints are 900px, 1190px, and 1600px; these adapt the Mac workspace, not separate clients.

Prototype frames retain their actual CSS viewport (1440×900 web, 1280×800 desktop, 390×844 mobile), then scale to fit the canvas. Scaling must not be replaced with a fluid iframe width that changes the generated layout.

## Elevation & Depth

Tonal layers and fine rules establish hierarchy. The canvas is flat; the composer has low ambient elevation, modal dialogs have stronger elevation, and prototype frames separate the artifact from light chrome. Exact shadows live in the sidecar. There are no decorative colored glows.

Focus uses a visible two-pixel accent outline with a three-pixel offset; the search field uses a two-pixel offset and the composer uses its accent border. Reduced motion disables animation and transitions while retaining status text.

**The Labeled State Rule.** Color and working dots reinforce execution labels; they never replace the reported state or transport health.

## Shapes

Compact controls use small curves, cards and panels use medium curves, and the composer and dialogs use larger curves. Most boundaries are one-pixel rules. The Home canvas is an edge-to-edge tonal plane, with bordered white working surfaces inside it. Prototype phone frames retain their device silhouette without changing artifact dimensions.

πDesigner’s retained app mark is an authored vermilion tile with an ivory artboard, an editable Bézier curve/anchor and a deep-ink pen nib. There is no π monogram in the app mark; π stays in the product name and engine attribution. A simplified transparent-cutout nib is the native menu-bar template. The native activity header uses the actual app icon. A standalone π identifies execution; model controls show the relevant locally bundled maker/provider mark. Opus and Sonnet use Claude/Anthropic, not DeepSeek. Unknown models use a neutral fallback. App, engine, model and routing-provider identity remain distinct.

## Components

### Buttons and fields

Primary actions use Warm Action with white on-action text. Secondary actions use Working White and a fine rule; hover shifts toward Subtle Gray. Disabled controls remain visible and cannot act. Search fields use the same surface and rule, with an explicit accent focus outline. A chosen design-system control uses Coral Tint with Deep Coral text.


### Navigation

Text labels accompany consistent authored line icons. Selected navigation uses the cool gray selected field and charcoal text; recent projects remain secondary. A bordered New project action sits above the main destinations. Settings and editor connection are placed at the foot of the rail.

### Composer

A white container with a deep coral focus border holds a real textarea paired with design-system selection, an optional device select, a model/provider icon and selected-model control, and a single send action. The Home composer starts a project; the compact Studio composer refines it. Empty prompts cannot submit. During generation, the send action becomes a stop control.

### Design-system library

Search and category filters scope the full bundled catalog. Grid specimens are indicative, explicitly labeled as samples. Detail views expose real guidelines, compiled CSS, palette values, and component fixtures. The detail tabs support arrow keys, Home, and End.

### pi model picker

The pi model control opens a searchable list read from the installed agent over RPC. Rows identify both model name and provider. Selection saves an exact provider/model id, checks supported thinking levels, and carries into the next run. Settings uses the same picker and only exposes supported thinking levels. Preserve loading, retry, empty-result, and failed-selection states. Credentials, endpoints, and provider headers are never model-list UI data.

### Studio and source editor

Preview/Code are distinct working views. CSS-only updates swap stylesheets without navigating the iframe; structural updates reload the preview. The source editor shows unsaved state and protects view changes, project navigation, and app quitting from accidental loss. Agent execution disables concurrent source writing.

### Observable execution

A persistent Studio strip gives the reported current action, project and elapsed time, with Stop. Expanding reveals the exact submitted brief/context, ID-correlated tool steps, bounded output snapshots, durations, errors and paginated diagnostics. Labels, not a blue dot or animation alone, distinguish starting, working, reconnecting, attention, finished, failed and stopped. No invented percentages or private reasoning.

Live activity stays at the end of the conversation, below plan, render and critique cards, so following a long design run keeps the latest execution visible. It shows the current reported action, total elapsed time and age of the last event, plus two recent completed tools and every running tool. File paths or commands are visible before expanding; each tool exposes output and arguments on demand. Running durations tick without requiring new events. View all activity opens and focuses the persistent strip’s full history. Reconnecting explicitly marks unavailable updates; silence is not presented as proof of continued progress.

The global Activity control and project statuses reflect the same durable event journal. Completed turns with unanswered questions remain attention-needed. Source editing, model identity and preview state remain separate from transport health.

### Mac background surfaces

Closing or minimizing the window retains its source draft, daemon and tasks; explicit Quit stops execution and protects unsaved source. A native menu-bar item and optional dark charcoal top-centre panel read the daemon independently of the web view. The panel appears only with active/attention-needed work while the workspace is in the background, stays below the camera/menu bar, and expands on click. Its state is available to accessibility, not only painted as a glyph.

Notifications are permission-controlled and deduplicated by run/outcome. Attention and failure are not completion. Sounds and project-name previews default off. Notification clicks restore the relevant conversation; permission denial retains menu-bar and in-app status.

### Dialogs and feedback

Use native HTML modal dialogs for focused system choice and renaming, with Escape, focus restoration, and accessible names. Errors explain what failed and preserve recoverable state. Mac menus provide standard editing, New Prototype, Settings, reload, window control, and zoom shortcuts.

## Do's and Don'ts

### Do:

- Do preserve the separation between the workspace and the generated artifact.
- Do use the real catalog and local run state, including empty and failure cases.
- Do keep pi-only execution and local storage explicit without making it the visual focus.
- Do keep native keyboard conventions and visible focus.
- Do use light neutral layers and warm coral actions consistently across shell, library, Studio, and dialogs.

### Don't:

- Don't add account, billing, publishing, or collaboration controls without working capabilities.
- Don't describe prototype frames as separate native app generators or πDesigner clients.
- Don't recolor prototype content to match the light workspace.
- Don't imply the light workspace has a complete alternate dark theme or theme-switching control.
