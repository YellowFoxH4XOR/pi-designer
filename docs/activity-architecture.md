# πDesigner activity and Mac background lifecycle

## Ownership

- `internal/pirun/pi.go`: pi process lifetime, strict LF-only JSONL framing, command correlation and shutdown. Backpressure replaces silently dropping events. Metadata-only sessions explicitly drain their startup events.
- `internal/pirun/events.go`: protocol decoding. Preserves tool-call IDs, latest tool result snapshots, durations, final messages and text-block indices. Exposes only a thinking-state marker, not private thinking content; binary/image tool content stays out of text logs.
- `internal/model/activity.go`: version-1 event contract and materialized activity projection. Execution status and attention are distinct; a completed turn may still need an answer.
- `internal/store/activity.go`: atomic journal/projection commits in SQLite, replay cursors, per-project discovery. Project deletion cascades into its journal. Legacy runs have no retroactive tool log.
- `internal/api/activity_journal.go`: serialized journal ownership, no lossy subscriber buffers.
- `internal/api/run_lifecycle.go`: close/reap the old process before declaring completion and releasing a project for another prompt. Explicit cancellation and daemon shutdown persist a stopped outcome. An acknowledged abort gets a 500 ms settlement grace period before the same session is force-closed. Run admission commits the opening transcript and run atomically; assistant transcript failures fail the run. Shutdown waits for outstanding run writes before closing SQLite.
- `internal/api/activity.go`: activity snapshots, paginated diagnostics, resumable SSE, supported extension-dialog responses and valid unfenced question-form attention detection.
- `ui/src/activity.ts`: deterministic stream reducer, matching authoritative message/partial-tool semantics. The global run hook discovers editor/MCP-started work, hydrates a snapshot, then streams strictly after its cursor. Replayed events are idempotent; discovery is also a terminal-state fallback.
- `ui/src/components/Activity.tsx`: current-action strip, exact submitted brief/context, tool details, extension questions, workspace activity and project labels. Diagnostics and native preferences have their own components.
- `ui/src/model-identity.ts`: maker-before-router identity resolution; local assets only. Unknown identities remain neutral.
- `mac/Sources/Daemon.swift`: owns the bundled sidecar and its stdin parent-death guard.
- `mac/Sources/ActivityModels.swift`: native DTOs and presentation preferences.
- `mac/Sources/ActivityService.swift`: foreground-independent daemon discovery/polling, menu bar, notch lifecycle and notification delivery/deep links.
- `mac/Sources/ActivitySurface.swift`: native SwiftUI activity surface. The physical notch/menu bar are never modified.
- `mac/Sources/main.swift`: window/navigation/menus and a main-frame, exact-origin allowlisted preferences bridge. Generated iframe content cannot use it.
- `internal/httpguard`: loopback Host and same-origin browser checks around the entire app handler, including WebSocket upgrades. Opaque preview origins cannot call the API; only read-only preview resources receive a narrow exception.
- `internal/app/data_lock.go`: exclusive data-directory ownership before opening SQLite or recovering stale runs. The lock remains held until HTTP drains and the store closes.

## API and consistency

- `GET /api/activity`: active/latest project summaries, excluding prompts, message blocks, tool logs and request payloads. Native clients pass `after=<cursor>` to also receive durable attention/terminal outcomes; a captured high-water mark prevents rapid successive completions from being overwritten by newer runs or skipped by a poll race.
- `GET /api/runs/:id/activity`: complete materialized state for one run.
- `GET /api/runs/:id/log?after=<seq>`: at most 500 ordered events per page.
- `GET /api/runs/:id/events?after=<seq>`: SSE with durable `id` fields and `Last-Event-ID` resumption. Completed runs remain readable. A terminal snapshot and `stream_end` close the connection, including legacy/interrupted runs.
- `POST /api/runs/:id/ui-response`: validates the still-pending request ID and select options before sending a supported extension response to pi.

Text events are coalesced over 60 ms without crossing content-block boundaries or dropping text. Output previews are limited to 16 KB per tool; tool partial results replace their previous preview instead of duplicating cumulative output. Tools correlate by ID, never by name or presumed order. Final assistant messages replace streamed drafts. Lifecycle waits for omp's `session_settled` (or a `prompt_result` that reports a settled session), not `agent_end`; `prompt_result.status` is the authoritative completed/aborted/error outcome. Records larger than 1 MiB arrive losslessly through negotiated RPC protocol v2 `rpc_chunk` reassembly.

SSE reads durable pages every 250 ms with keepalives. Workspace discovery polls every 1.5 seconds; the native controller polls summaries every second independently of WKWebView. These are bounded observation delays, not separate execution state machines. Connection failures are labeled as disconnected/reconnecting rather than inventing progress.

Extension select/confirm/input/editor requests are actionable. Fire-and-forget extension records are retained in diagnostics; custom terminal rendering is not emulated. Upstream RPC limitations still apply.

## Mac behavior and privacy

Closing hides the workspace and retains source drafts; minimizing also keeps work running. Explicit Quit checks unsaved source and fetches current daemon activity rather than relying on the polling snapshot. Confirmation is checked again for newly started runs; unavailable state requires an explicit warning/quit decision. The private daemon remains bound to loopback; no cloud service or new model engine is introduced.

The notch-style panel appears below the camera/menu-bar area on the workspace's display when work is active or needs attention and the workspace is not foreground-visible. It expands on click and collapses on Escape/outside click. The menu-bar entry remains after task completion and when the notch preference is disabled.

Notifications require user-granted macOS permission. Completion, attention and failure are separate semantic outcomes. Enabled outcomes are recorded as delivered only after successful OS scheduling; failed scheduling stays in a bounded in-memory queue with backoff and journal-cursor backpressure. Disabled preferences deliberately suppress their outcomes. Successful delivery/suppression is deduplicated across polling/relaunch; pending retries do not survive app exit. A question-only turn is not a task-complete notification. Sounds and project-name previews default off. Notification routing stores project/conversation IDs rather than trusting text or executable URLs. Denied permissions retain all other status surfaces.

Diagnostic logs are local but can contain project/command content; review before sharing. This feature does not collect telemetry, copy credentials or expose full model objects with provider headers/endpoints.

## Verification

- Go tests/race detector: protocol lifecycle correlation, output snapshots, final-message authority, thinking privacy, >64 KB/Unicode JSONL framing, backpressure, >1024-event replay, restart recovery, transaction failure, attention parsing, cancellation and shutdown ordering.
- Frontend reducer/icon/save-state tests: replay deduplication, content-block ordering, parallel tool completion, partial output, attention/failure/cancel states, maker/router identity, file-scoped save completion and edits typed during a save.
- `scripts/review-activity.cjs`: actual daemon/API/SQLite/SSE and system Chrome with a deterministic RPC fixture. Covers model icons, live activity, offline/reload recovery, diagnostics, externally started work, extension dialogs, question-only turns, failure/cancel, the 900×620 window, and delayed source-save/file-selection regression. No provider inference.
- Native model/preferences tests in `mac/Tests/main.swift`.
- `scripts/review-native-activity.cjs`: actual built app, isolated data/preferences, explicitly PID-scoped Accessibility actions. Covers close/minimize continuation, notch status/expansion, menu-bar access after completion and attention outside foreground WebKit. It never grants system permissions. Screenshot artifacts live under ignored `.impeccable/review/`.

Manual coverage still needed: notification authorization/banner delivery/clicks on the owner's Mac, sleep/wake and display/Spaces transitions. No public distribution signing/notarization is claimed; the app remains an ad-hoc-signed local build. The repository's existing Vite/esbuild development-tool audit advisories are not fixed by this feature.
