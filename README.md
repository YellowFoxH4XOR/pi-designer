# pidesigner

A local-first design workspace in Go. Describe a design; your installed **Pi agent** builds it as a real HTML artifact that renders live in a sandboxed preview at 60fps. Works from the UI or from any coding editor over MCP.

- **Prototypes** — web, mobile and desktop frames
- **Motion design** — agent-authored CSS/GSAP motion with a hot-swappable preview
- **Website clone** — the `web-clone` skill's CDP recon methodology, ported from OpenDesign
- **152 design systems** — Dropped in as data; each is a `DESIGN.md` brand contract plus `tokens.css`
- **MCP server** — connect Claude Code, Cursor, Codex, or Pi itself
- **Pi-only** — no cloud accounts, no API keys in this process, no media generation

Built as a focused reimplementation of [OpenDesign](https://github.com/nexu-io/open-design), cutting ~1.6M lines of TS down to ~3.5K lines of Go plus a 9KB UI.

---

## Quick start

```bash
./scripts/build.sh          # builds bin/pidesigner, bin/pidesigner-mcp, ui/dist
./bin/pidesigner            # http://127.0.0.1:7457
```

First time only, for the UI bundle:

```bash
cd ui && npm install && cd ..
```

Requires Go 1.24+ and a working `pi` on your `PATH` (or `--pi /path/to/pi`).

---

## How it works

```
┌─ UI (Preact, 9KB gzip) ───────────────────────────┐
│  chat · files · preview iframe · device frames     │
└───────────────┬───────────────────────────────────┘
                │ REST + SSE + WebSocket
┌───────────────▼───────────────────────────────────┐
│  Go daemon (single binary)                        │
│   projects · files · SQLite · static preview       │
│   Pi RPC runner ──► spawns `pi --mode rpc`         │
│   design-systems + skills + craft resolver        │
└───────────────┬───────────────────────────────────┘
                │ MCP over stdio
      your coding editor ──► write_file / start_run
```

The daemon never talks to a model provider directly. It spawns the Pi CLI in the project directory, so Pi's own tools (`read`, `write`, `edit`, `bash`) do the writing and Pi's own `~/.pi/agent/auth.json` holds the credentials.

### Streaming

Pi speaks JSONL-RPC on stdin/stdout. `internal/pirun` owns the process lifecycle, correlates command responses by id, and converts the event stream into typed events. The daemon fans those out over SSE.

Four protocol details that matter:

| Detail | Why it matters |
|---|---|
| Wait for `agent_settled`, not `agent_end` | `agent_end` closes one low-level run; retries, compaction and follow-ups still follow |
| `disposition: "handled"` means no run started | Waiting for `agent_settled` would hang forever |
| Subscribe before prompting | A fast completion otherwise races the listener |
| Frame on LF only | Pi's docs warn that `readline` splits on U+2028/U+2029, which are legal inside JSON strings |

We use `bufio.Reader.ReadSlice('\n')` rather than `bufio.Scanner` for the same reason we don't use a line reader: no 64KB token cap, no Unicode separator bugs.

### Preview performance

The preview does **not** reload the iframe on every change. A websocket watcher diffs the project directory at 250ms; CSS-only edits are pushed to an injected bridge script that swaps stylesheet `href`s in place. HTML edits fall back to a reload. This is the difference between a design tool that feels live and one that flashes white every time the agent saves.

### Path safety

Every client-supplied path is resolved against the project root in `project.Resolve`. Traversal attempts are clamped into the project directory rather than escaping it; both the file API and the preview server enforce this independently.

---

## Configuration

```
pidesigner [flags]

  --port 7457         HTTP port
  --host 127.0.0.1    bind address
  --data ~/.pidesigner
                      data directory (SQLite + projects/)
  --content ./content content catalog (design systems, skills, templates, craft)
  --pi <path>         pi executable (default: PATH, then ~/.pi/agent/bin/pi)
  --ui <dir>          built UI directory to serve
```

---

## HTTP API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/health` | daemon status, catalog counts |
| `GET/POST` | `/api/projects` | list / create |
| `GET/DELETE` | `/api/projects/{id}` | fetch / delete |
| `GET` | `/api/projects/{id}/files` | list project files |
| `GET/PUT/DELETE` | `/api/projects/{id}/file?path=` | read / write / delete one file |
| `GET` | `/api/projects/{id}/artifact?entry=` | entry file + every referenced sibling |
| `GET` | `/api/projects/{id}/search?q=` | content search |
| `POST` | `/api/projects/{id}/chat` | start an agent run → `{runId}` |
| `GET` | `/api/runs/{id}` | run status |
| `GET` | `/api/runs/{id}/events` | **SSE** agent event stream |
| `POST` | `/api/runs/{id}/cancel` | abort a run |
| `GET` | `/api/projects/{id}/watch` | **WebSocket** file changes |
| `GET` | `/api/design-systems` | catalog |
| `GET` | `/api/design-systems/{id}/context` | full brand context |
| `GET` | `/api/skills`, `/api/templates`, `/api/craft` | catalog |
| `GET` | `/preview/{id}/{path}` | preview render with bridge injected |

---

## MCP

The MCP server is stateless: it holds no state and never touches the filesystem. Every tool call becomes an HTTP request to the running daemon, so the daemon stays the single owner of project state.

```bash
./bin/pidesigner-mcp --print-config claude   # or cursor | codex | pi | generic
```

**Tools** — `list_projects`, `get_project`, `list_files`, `get_file`, `get_artifact`, `search_files`, `write_file`, `delete_file`, `list_skills`, `list_design_systems`, `start_run`, `get_run`, `cancel_run`

**Resources** — `od://design-systems/<id>/DESIGN.md`, `od://skills/<id>/SKILL.md`

Two write paths, both supported:

- `write_file` — the editor's agent edits files surgically
- `start_run` — delegate a full design to the Pi agent, then `get_run` to poll

---

## Content catalog

`content/` is vendored data, not code. `scripts/port-content.sh` refreshes it from an upstream OpenDesign checkout.

```
content/
  design-systems/     152 packages — manifest.json + DESIGN.md + tokens.css
                      + components.html, design-tokens.json, preview/
  skills/             web-clone, motion-design, web-artifacts-builder,
                      emilkowalski-motion, web-design-guidelines, agent-browser
  design-templates/   web-prototype (+3 taste variants), motion-frames,
                      tweaks, critique
  craft/              typography, color, motion discipline, anti-AI-slop, a11y, RTL
  prompt-templates/
```

Stripped from the upstream copy: `source/` (importer audit trail, 4.7MB) and 391 localized `DESIGN-<lang>.md` files (8.1MB). Neither is a runtime input.

Dropping a new folder into `content/design-systems/` is picked up on the next `/api/design-systems` request — no restart.

---

## Layout

```
cmd/pidesigner           daemon entrypoint
cmd/pidesigner-mcp       MCP stdio server
internal/model           domain types
internal/store           SQLite: projects, conversations, messages, runs
internal/content         catalog: design systems, skills, templates, craft
internal/project         project-file manager (path safety, search, entry resolution)
internal/pirun           Pi RPC client
internal/preview         file watcher, hot-swap bridge, preview server
internal/api             HTTP surface + run orchestrator
ui/                      Preact + Vite
content/                 vendored catalog
scripts/                 build, port-content, e2e
```

---

## Tests

```bash
go test ./...                              # unit tests
PIDESIGNER_LIVE_PI=1 go test ./internal/pirun -v
                                           # live RPC smoke test against installed pi
./scripts/e2e.sh                           # full create → generate → verify run
```

---

## License

The daemon and UI are original work. `content/` is derived from OpenDesign (Apache-2.0) and its upstream sources; each design-system package records provenance in `manifest.json`. Design-system names are trademarks of their respective owners — fine for local use, a separate question if you publish.
