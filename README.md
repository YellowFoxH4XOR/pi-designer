# PiDesigner

A **macOS desktop design workspace**, powered only by the Pi agent already installed on your Mac.

Describe an interface, choose a design system, and refine real project files in a live studio. Web, mobile, and desktop are **prototype preview sizes inside the Mac app**—not separate PiDesigner clients.

## Open the Mac app

```bash
./scripts/build-mac.sh
open build/PiDesigner.app
```

Build prerequisites: Xcode Command Line Tools, Go at the version declared in `go.mod` or newer, and Node/npm for building the embedded interface. Pi must be installed separately. The app looks on `PATH` and in `~/.pi/agent/bin/pi`; model credentials remain with Pi.

The current build targets Apple Silicon, macOS 13 or later. It is ad-hoc signed for local use, **not Developer ID signed or notarized for public distribution**. Copying it to another Mac is not a supported installer flow yet.

Optional local installation:

```bash
./scripts/build-mac.sh --install
```

This replaces `/Applications/PiDesigner.app`. It does not modify your Pi configuration or editor configuration.

## The workspace

- **Home:** a prompt-first composer, prototype/motion/slide-deck/website-clone workflows, device target, and design-system selection.
- **Projects:** searchable local projects, recent work, and persistent conversations.
- **Design systems:** the complete bundled catalog, category/search filters, actual guidelines, CSS tokens, and component fixtures. Grid specimens are illustrative; inspect a system for its exact content.
- **Skills:** inspect the installed workflow instructions and choose one for a new project.
- **Studio:** conversation on the left; Preview/Code and the artifact on the right. Edit and save source files, rename a project, change its design system, or start another conversation.
- **Model picker:** click the actual Pi mark in Home or Studio to search models configured in your installed Pi. Exact provider/model IDs are saved and checked against the Pi process before a run.
- **Settings:** the same model picker, only supported thinking levels, installed Pi status, local project path, and the MCP configuration for your editor.

Prototype preview sizes are 1440×900 (web), 1280×800 (desktop), and 390×844 (mobile). The iframe retains that real CSS viewport and scales to the canvas. CSS-only edits update in place; structural changes reload the preview.

The Mac app supports desktop content windows from 900×620 upward. There is no phone/tablet application or hosted browser product.

## How the design agent works

Each run follows the clarify → plan → design → verify → refine loop used by Open Design and Claude's design tooling (research notes: `docs/research/`).

1. **Clarify only when it matters.** A thin brief gets one short `<question-form>` rendered as an interactive form with recommended defaults; the agent stops until you answer. Specific briefs, local edits, and editor (MCP) runs skip it.
2. **Plan visibly.** The agent's first tool call is `todo_write`; the studio shows it as a live plan card. Unfinished plans offer **Continue remaining tasks**.
3. **Design plan, reviewed.** Before code, it writes a palette, type roles, an ASCII wireframe, and principles, then revises anything that reads like a generic default (Anthropic's `frontend-design` skill, always on).
4. **Build from real resources.** The design system's DESIGN.md and `tokens.css` are bound verbatim; the workflow skill's seed, layouts, and checklist are read before writing.
5. **Look at the result.** `render_preview` renders the artifact in WebKit at the Web/Desktop/Mobile frames (or a given slide of a deck) and returns the screenshot plus measured overflow, script errors, and broken images. Renders appear in the conversation. Budget: six per run.
6. **Expert panel critique.** A critic, brand, accessibility, and copy reviewer score the rendered design through `submit_critique`; the tool computes the weighted score and decides. Below 8/10 or with must-fix items, the agent fixes and re-scores (at most three rounds); a unanimous first round with nothing to fix is sent back once as too shallow. The scorecard appears in the conversation.
7. **Deliver.** Working notes collapse under **Design notes**; the summary follows, with up to three follow-up suggestions.

Slide decks (`simple-deck`) additionally get deck discipline: keep the seed's navigation framework, one idea per slide, density limits, and computed chart bars.

Each conversation is a persistent Pi session, so follow-ups keep the agent's full context. Conversations created before sessions existed replay their transcript once.

The prompt lives in `internal/prompt` (stable instructions per project, per-turn context separately so provider prompt caches survive). The Pi tools live in `internal/agentext/pidesigner.ts`, installed into the data directory at startup and loaded with `pi --extension`.

## Local data and execution

```text
~/Library/Application Support/PiDesigner/
  pidesigner.db       projects, conversations, messages, runs (with plans and renders)
  projects/          actual project files
  sessions/          one Pi session per conversation
  renders/           previews the agent rendered, per project
  agent/             PiDesigner's Pi extension (rewritten at startup)
  preferences.json   model and thinking choices
  runtime.json       current private loopback endpoint for MCP discovery
```

The native AppKit/WKWebView shell launches a bundled Go daemon on an available loopback port. The daemon launches the installed `pi --mode rpc`; it does not call a model provider itself. Closing the app closes its daemon pipe and shuts down the agent process tree.

The internal HTTP server is implementation infrastructure for the app and MCP server. It is not intended as a public or network-exposed service.

## Connect a coding editor

1. Open PiDesigner.
2. Open **Settings → Connect your editor**.
3. Copy the generated MCP configuration into your editor’s MCP settings.

Both executables are bundled in the app. The configuration points to a runtime discovery file, so the editor follows the Mac app after a port change or restart. Generate the configuration after moving the app to its permanent location. Keep the app running while using MCP.

The MCP server supports project/file reading and writing, artifact retrieval, skills/design-system discovery, generation through Pi, polling, and cancellation. Pass an explicit project id or name when operating on a project. The editor can either edit files directly or delegate a task to Pi.

`list_models` exposes the installed Pi catalog to editors. Runs that omit a model use the choice saved in PiDesigner. A selected model is passed with an explicit provider and checked through Pi’s reported state; provider failures are reported as failures, not successful generation.

No account, billing, cloud-model proxy, image generation, or video-generation workflow is part of this app.

## Branding and design

The workspace takes structural inspiration from OpenDesign Cloud. The app icon keeps the owner’s PiDesk-family charcoal/lavender palette, but uses an artboard, editable Bézier path, and pen nib. Agent controls and messages use Pi’s actual coral/blue/gold pixel mark, derived from the locally installed agent’s logo definition.

- `PRODUCT.md`: confirmed product scope and constraints.
- `DESIGN.md` and `.impeccable/design.json`: implemented design tokens and component rules.
- `docs/design-review.md`: verification, critique, audit, and coverage limits.
- `mac/ASSETS.md`: icon source and generation provenance.

## Developer checks

```bash
go test -race ./...
go vet ./...
(cd ui && npx tsc --noEmit && npm run build)
./scripts/build-mac.sh
codesign --verify --deep --strict build/PiDesigner.app
```

The local browser review helper `scripts/review-ui.cjs` exercises the embedded interface against an isolated real daemon, without starting a model generation. It needs an existing Playwright package (`PLAYWRIGHT_PACKAGE`) and system Chrome (`CHROME_PATH`, optional on this Mac). Its temporary metadata file is provided by `PIDESIGNER_REVIEW_META`; it records the test daemon URL and fixture project id.

Native WebKit snapshots can be taken without capturing the desktop or other applications:

```bash
build/PiDesigner.app/Contents/MacOS/pidesigner-app \
  --capture /tmp/pidesigner-home.png \
  --capture-width 1280 --capture-height 800
```

## Source layout

```text
mac/                     native AppKit/WKWebView shell and bundle metadata
cmd/pidesigner/          internal Go daemon executable
cmd/pidesigner-mcp/      stateless stdio MCP bridge
mac/Render/              pidesigner-render: offscreen WebKit renderer for render_preview
internal/app/            daemon lifecycle and native runtime discovery
internal/api/            project/catalog APIs and run orchestration
internal/prompt/         design-agent prompt: charter, workflow protocol, craft rules
internal/agentext/       Pi extension providing todo_write and render_preview
internal/pirun/          installed Pi JSONL RPC integration
internal/store/          SQLite persistence
internal/content/        design-system and skill catalog
internal/project/        project file manager
ui/                      embedded Preact interface
content/                 bundled design systems, skills, templates, craft laws
scripts/                 local builds, icon generation, review helpers
```

## Content provenance

`content/` is vendored from the local reference checkout. Package manifests retain source information; upstream licenses on individual skills are retained. `content/skills/frontend-design` is Anthropic's Apache-2.0 skill from `claude-plugins-official`, kept verbatim with its `LICENSE.txt`. The protocol text in `internal/prompt` adapts Open Design's design charter (Apache-2.0). Design-system names and brands belong to their respective owners. This repository’s local usage does not imply their endorsement or grant trademark rights.
