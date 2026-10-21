---
name: πDesigner
description: A focused local design workspace with a distinct designer-tool identity
colors:
  brand-coral: "#E45F3E"
  brand-paper: "#FFF5E7"
  brand-ink: "#233137"
  primary: "#5c61b6"
  primary-soft: "#eeedfb"
  lavender: "#a8b1ff"
  shell: "#eceef3"
  rail: "#e9ebf1"
  canvas: "#fcfcfd"
  white: "#ffffff"
  ink: "#23242b"
  muted: "#646673"
  line: "#e2e3ea"
  selected: "#dedff0"
  action: "#26262d"
  success: "#3b7956"
  error: "#a13c3a"
typography:
  display:
    fontFamily: "Georgia, 'Times New Roman', serif"
    fontSize: "clamp(34px, 3.3vw, 47px)"
    fontWeight: 500
    lineHeight: 1.16
    letterSpacing: "-1.6px"
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    fontSize: "25px"
    fontWeight: 620
    letterSpacing: "-0.7px"
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
  control: "7px"
  button: "9px"
  card: "11px"
  canvas: "16px"
  composer: "17px"
spacing:
  compact: "8px"
  control: "12px"
  group: "16px"
  section: "24px"
  page: "36px"
components:
  button-primary:
    backgroundColor: "{colors.action}"
    textColor: "{colors.white}"
    rounded: "{rounded.button}"
    padding: "11px 17px"
  button-secondary:
    backgroundColor: "{colors.white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.button}"
    padding: "11px 17px"
  navigation-selected:
    backgroundColor: "{colors.selected}"
    rounded: "{rounded.button}"
    height: "43px"
  workflow-selected:
    backgroundColor: "{colors.primary-soft}"
    rounded: "{rounded.button}"
    padding: "9px 12px"
---

# Design System: πDesigner

## Overview

**Creative North Star: "A place for ideas to take shape"**

A light, operational Mac workspace with its own designer-tool identity: a pen nib shapes a Bézier curve on an ivory artboard, set against vermilion. The workspace shell keeps its existing lavender-gray tonal separation; the artifact sits on a clean canvas. The app mark deliberately departs from πDesk rather than repeating its π tile. The reference informs familiar workspace hierarchy and density, not cloud accounts, invented sharing features, or another product’s identity.

Key characteristics: persistent navigation, a prompt-first starting point, restrained typography, and an artifact-led studio. The interface is desktop-only. Device frames describe generated prototypes, not app clients.

## Colors

The app mark uses vermilion `#E45F3E`, warm ivory `#FFF5E7`, and deep ink `#233137`. These are identity colors, not replacements for workspace state colors. Lavender continues to distinguish selection, focus, and omp engine attribution. Charcoal carries primary actions; green and red communicate real execution state. The light workspace is the implemented design, not an automatic dark-mode system.

**The Artifact Boundary Rule.** Workspace colors stop at the prototype iframe. A selected design system controls the artifact’s colors; it never recolors the app shell.

## Typography

The Mac system stack carries navigation, controls, and conversations. Georgia is reserved for the Home heading. Monospace is reserved for source, paths, and measured dimensions.

The hierarchy steps from the Home display to library headings, section titles, body text, and compact labels. Desktop utility text is deliberately denser than the creation surface. Long agent messages render as paragraphs, lists, emphasis, and fenced code rather than raw Markdown.

## Layout

The sidebar is 232px at the normal desktop width, 212px on narrower windows, and 190px at the minimum width. The workspace is a rounded, internally scrolling canvas beneath a 49px utility header. Home has a centered, constrained content column; libraries use searchable grids or task-oriented lists.

Studio has a 350px conversation column and a flexible preview column, adjusting to 310px on narrower windows and 390px on wide windows. The native content minimum is 900×620. Browser and phone app distributions are not supported.

Prototype frames retain their actual CSS viewport (1440×900 web, 1280×800 desktop, 390×844 mobile), then scale to fit the canvas. Scaling must not be replaced with a fluid iframe width that changes the generated layout.

## Elevation & Depth

Tonal layers and fine rules establish hierarchy. Shadows are soft and offset: low elevation under the workspace/composer, stronger elevation for modal selection and the prototype frame. There are no decorative colored glows.

Focus is a visible two-pixel accent outline with a three-pixel offset. Reduced motion retains text and state labels while disabling the working-dot animation.

## Shapes

Controls have compact curves, cards and panels use medium curves, and the main canvas/composer have larger curves. πDesigner’s app mark is an authored vermilion tile with a folded ivory artboard, an editable Bézier curve/anchor and a deep-ink pen nib. There is no π monogram in the app mark; π stays in the product name and engine attribution. A simplified, transparent-cutout nib is the native menu-bar template. The native activity header uses the actual app icon, not a lavender π. A standalone lavender π identifies the execution engine; model controls instead show the relevant locally bundled maker/provider mark. Opus and Sonnet use Claude/Anthropic, not DeepSeek. Unknown models use a neutral fallback. App, engine, model and routing-provider identity are never conflated.

## Components

### Navigation

Text labels accompany consistent authored line icons. Selected navigation uses a lavender-gray field; recent projects remain secondary. Settings and editor connection are placed at the foot of the rail.

### Composer

A real textarea is paired with design-system selection, an optional device select, a model/provider icon and selected-model control, and a single send action. The Home composer starts a project; the compact Studio composer refines it. Empty prompts cannot submit. During generation, the send action becomes a stop control.

### Design-system library

Search and category filters scope the full bundled catalog. Grid specimens are indicative, explicitly labeled as samples. Detail views expose real guidelines, compiled CSS, palette values, and component fixtures. The detail tabs support arrow keys, Home, and End.

### omp model picker

The omp control opens a searchable list read from the installed agent over RPC. Rows identify both model name and provider. Selection saves an exact provider/model id, checks supported thinking levels, and carries into the next run. Settings uses the same picker and only exposes supported thinking levels. Preserve loading, retry, empty-result, and failed-selection states. Credentials, endpoints, and provider headers are never model-list UI data.

### Studio and source editor

Preview/Code are distinct working views. CSS-only updates swap stylesheets without navigating the iframe; structural updates reload the preview. The source editor shows unsaved state and protects view changes, project navigation, and app quitting from accidental loss. Agent execution disables concurrent source writing.

### Observable execution

A persistent Studio strip gives the reported current action, project and elapsed time, with Stop. Expanding reveals the exact submitted brief/context, ID-correlated tool steps, bounded output snapshots, durations, errors and paginated diagnostics. Labels, not a blue dot or animation alone, distinguish starting, working, reconnecting, attention, finished, failed and stopped. No invented percentages or private reasoning.

The global Activity control and project statuses reflect the same durable event journal. Completed turns with unanswered questions remain attention-needed. Source editing, model identity and preview state remain separate from transport health.

### Mac background surfaces

Closing or minimizing the window retains its source draft, daemon and tasks; explicit Quit stops execution and protects unsaved source. A native menu-bar item and optional dark charcoal top-centre panel read the daemon independently of the web view. The panel appears only with active/attention-needed work while the workspace is in the background, stays below the camera/menu bar, and expands on click. Its state is available to accessibility, not only painted as a glyph.

Notifications are permission-controlled and deduplicated by run/outcome. Attention and failure are not completion. Sounds and project-name previews default off. Notification clicks restore the relevant conversation; permission denial retains menu-bar and in-app status.

### Dialogs and feedback

Use native HTML modal dialogs for focused system choice and renaming, with Escape, focus restoration, and accessible names. Errors explain what failed and preserve recoverable state. Mac menus provide standard editing, New Prototype, Settings, reload, window control, and zoom shortcuts.

## Do's and Don'ts

- Do preserve the separation between the workspace and the generated artifact.
- Do use the real catalog and local run state, including empty and failure cases.
- Do keep omp-only execution and local storage explicit without making it the visual focus.
- Do keep native keyboard conventions and visible focus.
- Don’t add account, billing, publishing, or collaboration controls without working capabilities.
- Don’t describe prototype frames as separate native app generators or πDesigner clients.
- Don’t imply the light theme has a complete dark-mode counterpart.
