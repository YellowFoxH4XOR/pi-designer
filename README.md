# πDesigner

A **macOS desktop design workspace**, powered only by the pi coding agent already installed on your Mac.

Describe an interface, choose a design system, and refine real project files in a live studio. Web, mobile, and desktop are **prototype preview sizes inside the Mac app**—not separate πDesigner clients.

## Open the Mac app

```bash
./scripts/build-mac.sh
open build/πDesigner.app
```

Build prerequisites: Xcode Command Line Tools, Go at the version declared in `go.mod` or newer, and Node/npm for building the embedded interface. pi 1.1.0 or newer must be installed separately (`curl -fsSL https://pi.dev/install.sh | sh` or `npm install -g @earendil-works/pi-coding-agent`; see https://pi.dev). Because a Finder launch does not inherit a login-shell `PATH`, the app searches `~/.bun/bin`, `~/.local/bin`, `~/.pi/agent/bin` and Homebrew locations before the inherited `PATH`. Settings → pi agent shows the executable, version and setup steps, and re-checks after you install or update pi.

The current build targets Apple Silicon, macOS 13 or later. It is ad-hoc signed for local use, **not Developer ID signed or notarized for public distribution**. Copying it to another Mac is not a supported installer flow yet.

Optional local installation:

```bash
./scripts/build-mac.sh --install
```

This replaces `/Applications/πDesigner.app`. It does not modify your pi configuration or editor configuration.

## The workspace

- **Home:** a prompt-first composer, prototype/motion/slide-deck/website-clone workflows, device target, and design-system selection.
- **Projects:** searchable local projects, recent work, and persistent conversations.
- **Design systems:** the complete bundled catalog, category/search filters, actual guidelines, CSS tokens, and component fixtures. Grid specimens are illustrative; inspect a system for its exact content.
- **Skills:** inspect the installed workflow instructions and choose one for a new project.
- **Studio:** conversation on the left; Preview/Code and the artifact on the right. Edit and save source files, rename a project, change its design system, or start another conversation.
- **Model picker:** click the model/provider icon in Home or Studio to search models configured in your installed pi. Exact provider/model IDs are saved and checked against the pi process before a run.
- **Settings:** the pi agent card (executable, version, profile, setup steps and Check again), the same model picker, only supported thinking levels, local project path, the MCP configuration for your editor, and design references that sync automatically with Sync now and Restore previous. Models and credentials are managed by pi (run `pi` and use `/login`); πDesigner never reads or stores keys.

Prototype preview sizes are 1440×900 (web), 1280×800 (desktop), and 390×844 (mobile). The iframe retains that real CSS viewport and scales to the canvas. CSS-only edits update in place; structural changes reload the preview.

The Mac app supports desktop content windows from 900×620 upward. There is no phone/tablet application or hosted browser product.

## How the design agent works

Each run follows the clarify → plan → design → verify → refine loop used by Open Design and Claude's design tooling (research notes: `docs/research/`).

1. **Clarify only when it matters.** A thin brief gets one short `<question-form>` rendered as an interactive form with recommended defaults; the agent stops until you answer. Specific briefs, local edits, and editor (MCP) runs skip it.
2. **Plan visibly.** The agent's first tool call is `todo_write`; the studio shows it as a live plan card. Unfinished plans offer **Continue remaining tasks**.
3. **Structure before styling.** Before code, the agent identifies the visitor's job and content hierarchy, compares two structurally different concepts under the same brand constraints, chooses one, and sketches its first viewport, primary interaction, and mobile reflow. It checks whether the design still fits the brief with the colors and product name removed. Tokens and type support that choice; they are not a substitute for it.
4. **Reuse mechanics, not a page recipe.** The selected design system's tokens and component rules remain authoritative. The general-purpose `web-prototype` seed is a minimal document baseline: no required hero, serif display face, feature grid, CTA sequence, or closed section menu. Its layout examples are optional techniques. Explicit reference matches and intentionally selected fixed-format workflows retain their structure.
5. **Look at the result.** `render_preview` renders the artifact in WebKit at the Web/Desktop/Mobile frames (or a given slide of a deck) and returns the screenshot, its saved PNG path, and measured overflow, script errors, and broken images. If pi shortens the tool result and omits the image, the agent reads the saved PNG instead of spending another render. Metrics alone do not count as visual inspection. Renders appear in the conversation. Budget: six per run.
6. **Expert panel and independent jury.** A critic, brand, accessibility, and copy reviewer score the rendered design through `submit_critique`. A fresh pi invocation then judges new screenshots against the brief, without the designer's conversation or tools, using Design 40 · Usability 30 · Creativity 20 · Content 10. Its score decides the round: at least 7.5/10 and no panel or jury must-fix items are needed to ship. Jury feedback guides revisions, with at most three rounds; unresolved work ships with issues explicitly listed. If independent review is unavailable or fails, the tool discloses that and falls back to the panel's 8/10 threshold. Both scores appear in the conversation. The jury uses the configured project entry, then the most recently rendered file, then `index.html`; `submit_critique.path` explicitly selects a different project-relative entry.
7. **Deliver.** Working notes collapse under **Design notes**; the summary follows, with up to three follow-up suggestions.

Slide decks (`simple-deck`) additionally get deck discipline: keep the seed's navigation framework, one idea per slide, density limits, and computed chart bars.

Each conversation is a persistent pi session (stored under `sessions/<conversationId>/pi/`), so follow-ups keep the agent's full context. Conversations started under an earlier harness (pi or the original π) replay their transcript once into a new pi session.

Use a new conversation after updating generation guidance so an existing session's earlier design decisions do not anchor the next task. Local refinements preserve the current design; a request for a new direction reopens composition.

The prompt lives in `internal/prompt` (stable instructions per project, per-turn context separately so provider prompt caches survive). The πDesigner tools live in `internal/agentext/pidesigner.ts`, installed into the data directory at startup and loaded with `pi --extension`. Runs launch `pi --mode rpc` with pi's tools in the project directory and your normal pi profile (`~/.pi/agent`); pi does not ask for tool approval, and there is no extra filesystem sandbox.

Runs and model discovery start pi with `--no-approve`, so project-local `.pi/` settings, extensions and MCP servers written into a project folder never load, and with `--no-mcp`, so none of your MCP servers start inside design runs. Your global pi configuration is unchanged. `.pi/` is hidden from the project file list. This is a startup policy, not a filesystem sandbox: it does not stop the agent from running commands itself.

## Design quality

pi is held to an Awwwards bar: a one-sentence concept, one signature moment, typography and art-directed imagery over decoration, choreographed motion, and usability intact (`content/craft/award-grade.md`). Before planning a new showcase, campaign, portfolio or motion piece, pi studies 2–3 live award-level references with `study_references` (screenshots, principles only — never copied). People and objects are never drawn from primitive shapes. Motion pieces expose `window.pdSeek(ms)` so `render_preview` can inspect the opening, key transformation and ending, and every new build records at least one critique scored on the award scale.

Model choice matters most: use a strong model in Settings → Model. The in-loop jury uses the active model by default; set `PIDESIGNER_JURY_MODEL=provider/id` on the daemon to choose another. `node scripts/design-bench.cjs --model provider/id --judge provider/id` measures quality with an independent final judge and uses that same judge model for the in-loop jury. Reports retain per-round jury feedback and the temporary data directory/run IDs for recovery. These are real, billable pi model calls, not fixture tests.

## Observable work and Mac activity

- **Current action:** the Studio activity strip shows what pi actually reports, elapsed time, and Stop. Live activity also appears directly in the conversation, below the plan and review cards: current action, total time, time since the last reported event, the two most recent completed tools, and all running tools with file/command targets and live durations. Expand a tool for output and arguments, or choose **View all activity** for the exact brief, submitted context and full tool history. Reconnection and model waits are explicit; the UI does not invent progress during silence.
- **Todo alongside activity:** live activity includes the current in-progress todo (or the next pending step) and completed/total count. **Show todo list** expands the full plan in place with completed, in-progress and pending states; long plans scroll internally. The original full plan card remains available in the conversation.
- **Thinking activity:** reasoning-stream activity refreshes the live status at most once per second, even before a visible response or tool call. The UI shows **Model is thinking** and the age of the last thinking event. No private reasoning text is stored or displayed; silence still means no new activity was received, not proof that the provider is working.
- **Durable events:** RPC decoding preserves tool-call IDs, partial-result snapshots and authoritative final messages. A SQLite event journal and materialized activity snapshot drive every surface. SSE resumes after a cursor instead of replaying/duplicating a bounded tail. Text deltas are batched for 60 ms without dropping content. Tool-output previews are capped at 16 KB; image payloads and private thinking text are not log content.
- **Workspace activity:** project status labels and the Activity control discover work across projects, including editor/MCP runs. Questions remain “Needs your answer”, not “Design complete”. Supported pi extension dialogs (select, confirm, input, editor) can be answered or canceled in the activity panel.
- **Diagnostics:** a paginated local event log includes retries, compaction, stderr and tool events. Logs can contain project/command content; inspect them before sharing. Existing runs from before this feature keep their conversation/plan/render history but have no retroactive tool journal.
- **Menu bar/notch:** the native controller reads the daemon independently of WebKit, so background throttling cannot hide task state. The optional top-centre panel stays below the menu bar/camera area, expands into project controls and supports non-notched/external displays. It is not a modification to the system notch. Disable it in Settings if desired; the menu-bar item remains.
- **Notifications:** Settings → Mac activity & notifications requests macOS permission and controls completion, attention, failure, sound and project-name previews. Previews and sound default off. Clicking a notification restores the relevant project/conversation. Permission denial retains in-app/menu-bar feedback; automatic permission granting is never attempted.

App display/bundle names change to πDesigner; the existing bundle identifier, `pidesigner` binaries, MCP names, preferences and data directory remain compatible. No project migration or credential copying is needed.

## Local data and execution

```text
~/Library/Application Support/PiDesigner/
  pidesigner.db       projects, conversations, messages, runs (with plans and renders)
  projects/          actual project files
  sessions/          one pi session directory per conversation
  renders/           previews the agent rendered, per project
  agent/             πDesigner's pi extension (rewritten at startup)
  preferences.json   model and thinking choices
  runtime.json       current private loopback endpoint for MCP discovery
  .daemon.lock       exclusive ownership of this data directory (not deleted on shutdown)
```

`sessions/<conversationId>/` is one pi session directory per conversation; pi writes `<timestamp>_<uuid>.jsonl` files inside it.

Only one daemon may own a data directory. A second instance fails before opening the database, recovering runs, or replacing discovery. A failed project-file cleanup retains its database row and slug for retry; a new project cannot reuse a directory while deletion is in progress.

Restoring a version first saves any current edits into version history, validates all target paths and blob hashes, and stages the target plus a backup before changing project files. Write/delete failures trigger rollback. If rollback fails, the error identifies a retained recovery directory under `versions/<projectID>/.restore-*` containing `before/`, `after/`, and `recovery.json`. Symlink and file/directory conflicts are refused rather than followed or overwritten. This is not a crash-atomic filesystem transaction; recovery after abrupt termination is manual.

The native AppKit/WKWebView shell launches a bundled Go daemon on an available loopback port. The daemon launches the installed `pi --mode rpc`; it does not call a model provider itself. Closing or minimizing the workspace keeps the daemon and active tasks running. The menu bar and optional notch panel remain available in the background. Explicit Quit closes the daemon pipe and shuts down the agent process tree.

The internal HTTP server is local-only implementation infrastructure for the app and MCP server. Bind addresses and request Hosts must be loopback; foreign and opaque browser Origins are rejected for API and WebSocket requests. Native HTTP/MCP clients without an Origin remain supported. These checks protect the browser boundary, not against other processes already running as your user.

Project HTTP/MCP file operations and preview serving use rooted filesystem handles; links escaping the configured project root are rejected, and file deletion cannot remove that root. Embedded and standalone preview documents both have a response-level sandbox without same-origin privileges. Read-only preview asset requests have narrowly scoped opaque-origin support for scripts, modules, styles, fonts, and media; that exception never grants API access. These restrictions do not sandbox the separately launched, auto-approved pi agent.

## Connect a coding editor

1. Open πDesigner.
2. Open **Settings → Connect your editor**.
3. Copy the generated MCP configuration into your editor’s MCP settings.

Both executables are bundled in the app. The configuration points to a runtime discovery file, so the editor follows the Mac app after a port change or restart. Generate the configuration after moving the app to its permanent location. Keep the app running while using MCP.

The MCP server supports project/file reading and writing, artifact retrieval, skills/design-system discovery, generation through pi, polling, and cancellation. An omitted `project` uses the project currently open in Studio; explicit project IDs or names override that selection. Leaving Studio clears the default, and calls without a selection return an actionable error rather than choosing an arbitrary project. The selection is daemon-session state, not persisted across restarts. The editor can either edit files directly or delegate a task to pi. `pidesigner-mcp --print-config --client pi` prints the config for `~/.pi/agent/mcp.json`.

`get_artifact` returns canonical project-relative keys in `files`. Each value is `{ "encoding": "utf-8" | "base64", "content": "...", "mimeType": "..." }`: text is UTF-8, binary assets are base64, and decoding preserves the original bytes. HTML references resolve against the entry document's directory (or the project root for leading `/`); missing local dependencies return an error instead of an incomplete bundle. Resource-read failures use top-level JSON-RPC errors, and `search_files.limit` is honored.

`list_models` exposes the installed pi catalog to editors. Runs that omit a model use the choice saved in πDesigner. A selected model is passed with an explicit provider and checked through pi’s reported state; provider failures are reported as failures, not successful generation.

`list_skills` and the skill resources include runnable design-template workflows such as `web-prototype`, matching the workflows the runner can actually load. Existing project skill IDs do not need migration.

No account, billing, cloud-model proxy, image generation, or video-generation workflow is part of this app.

## Branding and design

The app is **πDesigner**, using the Greek letter π in its name. Its independent designer-tool mark is a deep-ink pen nib drawing a Bézier curve on an ivory artboard, inside a vermilion tile—not another πDesk-style π icon. The same vector drives the Dock, favicon and workspace brand; a monochrome nib identifies the menu bar and the native activity panel uses the app icon. The execution engine is identified by the official pi logo (`ui/public/pi.svg`, vendored unchanged from can1357/oh-my-pi `assets/icon.svg`, MIT) on its native dark tile. Locally bundled model/provider marks distinguish OpenAI, Claude/Anthropic, DeepSeek, OpenCode, Gemini and OpenRouter. Model maker and routing provider remain separate identities.

- `PRODUCT.md`: confirmed product scope and constraints.
- `DESIGN.md` and `.impeccable/design.json`: implemented design tokens and component rules.
- `docs/design-review.md`: verification, critique, audit, and coverage limits.
- `mac/ASSETS.md`: icon source and generation provenance.

## Developer checks

```bash
go test -race ./...
go vet ./...
(cd ui && npm test && npx tsc --noEmit && npm run build)
node --test scripts/tests/*.test.cjs
go build -o bin/pidesigner ./cmd/pidesigner
node scripts/review-activity.cjs
swiftc -o /tmp/pidesigner-native-tests mac/Sources/ActivityModels.swift mac/Tests/main.swift
/tmp/pidesigner-native-tests
./scripts/build-mac.sh
codesign --verify --deep --strict build/πDesigner.app
```

The browser helper `scripts/review-ui.cjs` connects to the daemon URL in the JSON file supplied by `PIDESIGNER_REVIEW_META` (default `/tmp/pidesigner-design-review.json`). Use an isolated test daemon. The helper creates and edits its own labeled fixture; it never deletes a supplied project ID or rewrites the metadata file, leaves model preferences unchanged, and restores the previous active-project selection. Its fixture remains afterward. It starts no model generation and needs `playwright-core` (`PLAYWRIGHT_PACKAGE` to override) and system Chrome (`CHROME_PATH` to override).

`scripts/e2e.sh` does run real inference against `PIDESIGNER_E2E_BASE` (default `http://127.0.0.1:7457`). It exits nonzero on request, stream, run, artifact, or preview failures. Outputs are isolated per run and removed on exit; set `PIDESIGNER_E2E_KEEP_OUTPUTS=1` to retain the printed directory. `scripts/port-content.sh` validates and stages a complete upstream catalog before replacement, preserving separately vendored/local skills, πDesigner's web-clone adaptations, and the locally adapted general-purpose prototype skill, seed, layouts, and checklist. Other upstream resources continue updating.

Native WebKit snapshots can be taken without capturing the desktop or other applications:

```bash
build/πDesigner.app/Contents/MacOS/pidesigner-app \
  --capture /tmp/pidesigner-home.png \
  --capture-width 1280 --capture-height 800
```

The activity browser review uses `playwright-core` and system Chrome with deterministic fixture RPC, not a model provider. Native lifecycle checks use `node scripts/review-native-activity.cjs` after building; they require already-granted Accessibility/Screen Recording access, use an isolated data directory/preferences suite, and do not request notification permission. Actual notification authorization, banner delivery/clicks, sleep/wake, Spaces and multi-monitor behavior still need a manual Mac pass.

Developer-only environment overrides: `PIDESIGNER_DATA_DIR`, `PIDESIGNER_PI_BIN`, and `PIDESIGNER_PREFERENCES_SUITE` isolate native smoke tests. Normal launches continue using the existing data root and installed `pi` executable.

### Generation verification — 2026-10-23

- Go race tests, vet, content-import safety tests (including local prototype preservation and first import), and the signed Mac build passed. After the screenshot-recovery change, the affected prompt/API/app tests and app build passed again.
- Two isolated live briefs used `anthropic/claude-haiku-5-5` with high thinking and the same Shadcn system. Mend produced session selection beside a local booking form; Margin produced a filtered collection and a separate single-column reader. These demonstrate different task structures under shared brand constraints, not a guarantee of diverse output for every brief.
- Browser checks exercised booking, confirmation and release; filtering, opening a story, changing actual text size from 16px to 24px, and returning with filters preserved. Both artifacts fit 1440px and 390px without horizontal overflow; no page-script exceptions were recorded.
- The initial grouped renderer results lost image attachments when omp shortened tool output. Saved PNG paths now let the agent recover the actual image. A separate live RPC smoke used one render, read the saved PNG, correctly described the visible two-column arrangement, and settled successfully.
- Limits: Mend's generation run succeeded; Margin wrote its final response but did not settle and was canceled when the isolated daemon shut down. The artifacts were inspected by the browser, but this is not a claim that both full agent critique loops completed. A mechanical scan also reported a 4.48:1 helper-text contrast advisory in Mend and the brand-pinned Geist font in both samples. Temporary smoke projects were removed.


## Source layout

```text
mac/                     native AppKit/WKWebView shell and bundle metadata
cmd/pidesigner/          internal Go daemon executable
cmd/pidesigner-mcp/      stateless stdio MCP bridge
mac/Render/              pidesigner-render: offscreen WebKit renderer for render_preview
internal/app/            daemon lifecycle and native runtime discovery
internal/api/            project/catalog APIs and run orchestration
internal/prompt/         design-agent prompt: charter, workflow protocol, craft rules
internal/agentext/       pi extension providing todo_write and render_preview
internal/pirun/          pi JSONL RPC integration
internal/store/          SQLite persistence
internal/content/        design-system and skill catalog
internal/project/        project file manager
ui/                      embedded Preact interface
content/                 bundled design systems, skills, templates, craft laws
scripts/                 local builds, icon generation, review helpers
```

## Content provenance

`content/` is vendored from the local reference checkout. Package manifests retain source information; upstream licenses on individual skills are retained. `content/skills/frontend-design` is Anthropic's Apache-2.0 skill from `claude-plugins-official`, kept verbatim with its `LICENSE.txt`. The protocol text in `internal/prompt` adapts Open Design's design charter (Apache-2.0). Design-system names and brands belong to their respective owners. This repository’s local usage does not imply their endorsement or grant trademark rights.
