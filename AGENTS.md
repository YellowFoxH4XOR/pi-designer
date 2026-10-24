# Repository Guidelines

## Project Overview
πDesigner is a local-first macOS design workspace. A user describes an interface, picks a workflow (prototype, motion, slide deck, website clone) and a design system, and refines real HTML project files in Studio. A Go daemon delegates all generation to the separately installed **`pi`** coding agent. The daemon never calls model providers or holds credentials; that is `pi` and its `/login`. Mobile, web, and desktop are *preview frames* (390×844, 1440×900, 1280×800), not separate clients.

## Architecture & Data Flow
```
Preact UI (ui/src) ──HTTP JSON──▶ api.Server (internal/api)
                   ◀──SSE────── /api/runs/{id}/events   (durable SQLite journal)
                   ◀──WebSocket preview (internal/preview: HTML reload, CSS hot-swap)
api.Runner ──▶ pirun.Session: one `pi --mode rpc --no-approve --no-mcp` process per run (NDJSON over pipes)
            └─ pi loads extension <DataDir>/agent/pidesigner-extension.ts → design tools
Swift shell (mac/) spawns daemon on port 0, hosts WKWebView, polls /api/activity for menu bar/notch
pidesigner-mcp (cmd/pidesigner-mcp) finds the daemon via <DataDir>/runtime.json (loopback only)
```
- **Startup:** `cmd/pidesigner/main.go` → `app.Start` (`internal/app/app.go`) acquires an exclusive `.daemon.lock` before opening SQLite, recovering stale runs, or writing discovery. Startup rejects non-loopback bindings; shutdown drains HTTP and closes SQLite before releasing ownership. Do not unlink the lock file or introduce a second daemon for the same data root.
- **Runs:** `Runner.Start` validates input, enforces **one active run per project** (`active[projectID]`), refuses runs on projects being deleted, commits the opening user message and run atomically, then runs `execute` asynchronously. Assistant transcript failures fail the run. An acknowledged abort has a 500 ms settlement grace period before force-closing that session.
- **Sessions:** each conversation is a persistent pi session in `sessions/<conversationID>/pi/`. A conversation with no pi session yet (new, or from the earlier omp harness) gets its transcript replayed once through `prompt.TurnInput.Transcript`.
- **Prompting:** `internal/prompt/prompt.go` builds a *stable* system prompt (embedded charter, `workflow.md`, craft, and deck files, plus the active skill, design system, and installed knowledge). It writes that prompt to a temp file that is appended to pi's system prompt. Per-turn data (viewport, `SourceCatalog`, transcript, `SkipDiscovery`) goes in `<turn-context>` in `Turn()`.
- **Agent tools:** `internal/agentext/pidesigner.ts` is `//go:embed`-ed and rewritten atomically only when it changes. Tools: `todo_write`, `select_design_sources`, `study_references`, `render_preview`, `submit_critique`. It is a pi extension: import `ExtensionAPI` from `@earendil-works/pi-coding-agent` and `Type` from `typebox` (pi supplies both). Keep the default `direct` exposure so the model calls tools by name; the UI cards key on those names.

## Key Directories
| Path | Purpose |
|---|---|
| `cmd/pidesigner/` | Daemon entry and flags |
| `cmd/pidesigner-mcp/` | MCP companion for editors |
| `internal/api/` | HTTP routes, `Runner`, activity SSE, knowledge, project delete |
| `internal/httpguard/` | Full-handler loopback Host, Origin and browser Fetch Metadata checks |
| `internal/pirun/` | pi binary resolution and version check, RPC framing, model catalog |
| `internal/store/` | SQLite persistence (`modernc.org/sqlite`) |
| `internal/model/` | Persistence and event contracts (activity projection) |
| `internal/content/` | Scans `content/` for design systems, skills, and templates |
| `internal/knowledge/` | Allowlisted external design references (`Approved` list) |
| `internal/preview/` | File-polling watcher hub and WebSocket broadcast |
| `internal/project/` | Project files accessed through `os.Root`; `Resolve` is a lexical path helper, not sufficient for secure I/O |
| `ui/src/` | Preact app: `api.ts`, `state.ts` (`useRuns`), `activity.ts` (`reduceActivity`), `pages/`, `components/` |
| `mac/Sources`, `mac/Render`, `mac/Tests` | AppKit shell, WebKit renderer CLI, native tests |
| `content/` | Vendored design systems, skills, design templates, craft |
| `scripts/` | Build, review harnesses, fixtures |

## Development Commands
```bash
./scripts/build-mac.sh                 # Go + UI + Swift → build/πDesigner.app (ad-hoc signed)
./scripts/build-mac.sh --install       # also replaces /Applications/πDesigner.app
./scripts/build.sh                     # Go binaries → bin/, UI only if ui/node_modules exists
(cd ui && npm ci && npm run build)     # UI → ui/dist
go run ./cmd/pidesigner --data ./data --content ./content --ui ./ui/dist --port 7457
(cd ui && npm run dev)                 # Vite on :5174, proxies /api and /preview to :7457
go run ./cmd/pidesigner ... --pi scripts/fixtures/pi-rpc.cjs   # deterministic fake agent
```
Daemon flags worth knowing: `--port 0` (free port), `--print-url` (emits `PIDESIGNER_READY <url>`), `--exit-on-stdin-close`, `--pi <path>`.

To restart the installed dev app: quit `build/πDesigner.app`, then `open build/πDesigner.app`. Before killing it, check `/api/activity` (URL in `runtime.json`) so you don't kill a live run.

## Code Conventions & Common Patterns
- **Go:** use `gofmt`. Dependencies are injected through exported struct fields (`api.Server{Store, Catalog, Knowledge, Runner, Hub, PiExplicit, Models}`), not globals. Register routes in `Server.Routes()` as Go 1.22+ method patterns (`"DELETE /api/projects/{id}"`).
- **HTTP errors:** `writeErr(w, status, msg)` → `{"error": msg}`; success → `writeJSON`. Request decoding rejects unknown fields. Resolve the project with `s.resolveProject(w, r)` and return when `!ok`.
- **Errors:** wrap with `fmt.Errorf`. User-facing strings say "πDesigner" / "pi" (e.g. `"pi is already working in this project"`). The engine is the user's installed pi coding agent (`@earendil-works/pi-coding-agent`, needs `pirun.MinVersion` or newer); omp is no longer supported.
- **Concurrency:** `Runner` maps (`sessions`, `sessionSettled`, `journals`, `active`, `deleting`, `cancelRequested`) share `r.mu`. `uiMu` is separate, and `wg` gates shutdown. The preview `Hub` and each watcher have their own mutex and stop via `sync.Once`.
- **Persistence:** SQLite is the source of truth (WAL, foreign keys, one connection, Store mutex). Artifacts are plain files under `projects/<slug>`.
- **Security:** `httpguard.Protect` wraps the entire app handler. API/WS require same-origin browser requests; Origin-less native clients remain supported. Preview responses must retain their CSP sandbox without `allow-same-origin`, including when opened standalone. Opaque-origin CORS exceptions are only for read-only preview assets. Use `os.Root` operations, not lexical validation followed by unrestricted `os.*` calls; file deletion must reject the root.
- **Deletion:** retain the project row/slug until all filesystem cleanup succeeds. Errors leave the project reserved for retry, preventing create/delete path reuse races.
- **Artifacts:** `BuildArtifact` returns `files[path]` descriptors (`encoding`, `content`, `mimeType`), not raw strings. UTF-8 text and base64 binaries preserve bytes; resolve dependencies relative to the entry's directory or project root.
- **UI state:** SSE snapshots are authoritative. `reduceActivity` discards stale or duplicate events by sequence number, and clients reconnect via `Last-Event-ID`. Keep `ui/src/api.ts` types in sync with Go JSON.
- **Editor saves:** completion belongs to a project/file/generation and the submitted content snapshot. Keep newer edits dirty; block file selection during saves and conversation selection during create/send transitions.
- **Untrusted content:** knowledge documents and skill text are framed as *data, not instructions* in prompts. Keep that framing when editing `prompt.go`.
- **Content additions:**
  - Design system: `content/design-systems/<id>/DESIGN.md`. Optional files: `manifest.json`, `tokens.css`, `USAGE.md`, `components.html`, `preview/`.
  - Skill: `content/skills/<id>/SKILL.md` with `---` YAML frontmatter (`name`, `description`, `triggers`, `od.mode`). Invalid frontmatter silently excludes the skill.
  - Template: `content/design-templates/<id>/SKILL.md`. IDs must not collide with skill IDs.
  - Knowledge source: append to `knowledge.Approved` in `internal/knowledge/updates.go`. HTTPS only, 512 KiB cap, hash-checked, explicit apply and rollback. No `DocumentURL` means browse-only.
- Visual and UI changes must respect `DESIGN.md` (tokens, artifact boundary, accessibility, execution states) and `PRODUCT.md` scope.

## Important Files
- `cmd/pidesigner/main.go`: flags, default data dir (`~/.pidesigner` for the CLI, `~/Library/Application Support/PiDesigner` for the app)
- `internal/app/app.go`: wiring and lifecycle
- `internal/api/api.go`, `internal/api/runner.go`: API surface and run orchestration
- `internal/pirun/pi.go`: pi resolution (`PATH`, `~/.bun/bin`, `~/.local/bin`, `~/.pi/agent/bin`, Homebrew), RPC process; `version.go`: `pi --version` check against `MinVersion`
- `internal/agentext/pidesigner.ts`: agent tools (edit here; the data-dir copy is overwritten)
- `internal/prompt/prompt.go`, `internal/prompt/workflow.md`: agent instructions
- `mac/Sources/Daemon.swift`: child daemon launch, PATH, `PIDESIGNER_PI_BIN` override
- Data dir layout: `.daemon.lock`, `pidesigner.db`, `projects/`, `sessions/`, `renders/`, `agent/`, `knowledge/`, `preferences.json`, `runtime.json`
- Docs: `README.md`, `docs/activity-architecture.md`, `docs/design-reference-updates.md`, `docs/activity-verification.md`

## Runtime/Tooling Preferences
- Go **1.26** (`go.mod`). npm with `ui/package-lock.json` (no Bun, no Node engine pin). Preact + Vite + TypeScript (strict, `noEmit`).
- macOS 13+ on arm64. Swift is compiled directly with `swiftc` by `build-mac.sh`; there is no SwiftPM or Xcode project.
- There is no ESLint, Prettier, Makefile, or CI; the checks below are the gate.
- `bin/`, `build/`, `ui/dist/`, `ui/node_modules/`, and `data/` are generated and gitignored.
- `master` is the default branch. Land changes through a PR (`gh pr create` → `gh pr merge --merge`).

## Testing & QA
```bash
go test -race ./... && go vet ./...
(cd ui && npm test && npx tsc --noEmit && npm run build)   # node --test ui/tests/*.test.cjs
node --test scripts/tests/*.test.cjs    # clone/import safety and E2E failure semantics
go build -o bin/pidesigner ./cmd/pidesigner # browser review requires this generated binary
node scripts/review-activity.cjs        # real daemon + SQLite/SSE + Playwright/Chrome, fixture RPC
node scripts/review-native-activity.cjs # built-app lifecycle/accessibility review
codesign --verify --deep --strict build/πDesigner.app
```
- **Go tests:** `t.TempDir()` for the DB and files, `t.Setenv` for environment, `httptest` recorders, and custom `RoundTripper` stubs for outbound HTTP. App tests boot the real server on temp data.
- **Fixture agent:** `scripts/fixtures/pi-rpc.cjs`. Prompt markers select scenarios: `ASK_FORM`, `ASK_EXTENSION`, `FAIL_RUN`, `THINKING_TASK`, `LONG_TASK`, `FAST_TASK`, `SCROLL_STRESS`, `NATIVE_LONG_TASK`, `ACK_ABORT_NO_SETTLE`.
- **Browser reviews:** use `ui/node_modules/playwright-core` with system Chrome (`CHROME_PATH` to override). Output goes to `.impeccable/review/`.
- **Live pi tests are opt-in.** `PIDESIGNER_LIVE_PI=1 go test ./internal/pirun -run 'TestLive(PiRPC|InstalledPiModels)$' -v` checks metadata only. `PIDESIGNER_LIVE_MODEL=provider/model ...TestLiveExplicitModelPrompt` runs real inference. `scripts/e2e.sh` runs live generation against a daemon on `:7457`.
- **Review safety:** `review-ui.cjs` leaves its own fixture, never deletes a supplied project ID or rewrites metadata/preferences, and restores the prior active selection. Use isolated daemon data. E2E outputs are per-run temporary files; set `PIDESIGNER_E2E_BASE` for a test endpoint and `PIDESIGNER_E2E_KEEP_OUTPUTS=1` to retain output.
- **Design quality benchmark (real inference, opt-in):** `node scripts/design-bench.cjs --model provider/id [--judge provider/id] [--only id,…]` runs the briefs in `scripts/bench/briefs.json` through a real daemon and pi, renders each result (motion at several beats via `window.pdSeek`), and has an independent judge model score the screenshots on the Awwwards rubric (Design 40 · Usability 30 · Creativity 20 · Content 10). Report: `.impeccable/bench/<timestamp>/index.html`. Use it to compare prompt or model changes; never trust the agent's own critique score alone.
- **Renderer:** `pidesigner-render` disables WebKit occlusion throttling so requestAnimationFrame, canvas and WebGL pages run; `--wait MS` lets a live page run, `--at MS` seeks animation clocks (prefers the page's `window.pdSeek`).
- There is no coverage threshold. Add tests for consumer-visible behavior (API contracts, run lifecycle, event replay, path confinement), and record verification evidence in `docs/*-verification.md` when changing activity/UI behavior.
