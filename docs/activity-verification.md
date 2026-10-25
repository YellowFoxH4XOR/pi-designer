# πDesigner activity release verification

## Verdict

Ready for PR review as a local Mac build. The activity pipeline, background lifecycle and native control surfaces are implemented and exercised. This is not a notarized public release or a claim that notification permission/banner behavior has been approved on the owner's Mac.

## Passed

- `go test -race ./...` and `go vet ./...`.
- Frontend typecheck, production build, and five reducer/model-identity tests.
- Metadata-only discovery against the installed omp: 70 configured models; exact provider/model selection and supported thinking levels verified without inference or changing the default.
- Twelve browser/native-renderer integration checks against an isolated real daemon/SQLite/SSE, using deterministic fixture RPC: branding/icons, live parallel tools/output, offline/reload recovery, diagnostics, external-run discovery, supported extension questions, question-only completion, provider errors/cancellation, superseded-attention recovery, minimum-window layout, long-conversation scroll containment, variable-height error banners, and native WebKit root-height checks. No browser runtime errors.
- Eight native integration checks in the actual built app with isolated data/preferences: main-frame native preferences bridge, close continuation, notch expansion, minimize continuation, completion while minimized, menu-bar access after completion, rapid-outcome notification-ledger recovery, and attention independent of foreground WebKit.
- Ten native DTO/preferences assertions, including distinct attention/completion/failure/cancellation outcomes and private/quiet notification defaults, plus ten designer-logo raster/alpha-template assertions (20 native assertions total).
- Ad-hoc bundle signing and `codesign --verify --deep --strict build/πDesigner.app`.
- Mechanical UI detector over `ui/src`: exit 0, no findings.
- Visual inspection of the working Studio, model picker, 900×620 Studio and native notch panel. Tool status uses explicit words rather than color alone; native current action is exposed to accessibility.
- No review daemon or fixture agent left running. Existing user data and the previously running app were not migrated, replaced or stopped by the tests.

Browser and native evidence is generated under ignored `.impeccable/review/` by the repeatable scripts. Raster review captures are test artifacts, not shipping assets. Shipping icon rasters are generated from the canonical authored SVG; see `mac/ASSETS.md` for provenance.

## Unlimited-scroll regression

A 60-step plan plus a long conversation and critique reproduced a 3,826px root document inside a 900px window. Absolutely positioned `.sr-only` labels had an unbounded static position outside the conversation's scroll container. The fix explicitly anchors hidden labels, makes the conversation a containing block, bounds the app shell to its viewport, and confines scroll chaining to the appropriate inner panels. Hidden status text remains in the accessibility tree.

The regression verifies root scroll height/window scroll position, independent conversation scrolling, pinned header/composer, large error banners, and native WebKit at 1440×900, 900×620, and a reduced 900×480 CSS viewport (zoom stress). Root CSS minimums are removed; AppKit still enforces the physical window minimum. Expanded activity details can shrink to preserve the composer when an error banner takes space. The existing live app/task was not restarted by these tests.

## Inline live activity

The live conversation now exposes current action, elapsed time, last-event age, recent tool targets and running tool durations without expanding the persistent activity strip. The existing integration review checks visible file/command targets while that strip is collapsed, expands streaming output inline, and opens the full history through **View all activity**. All twelve integration checks passed, including offline/reload recovery, attention, cancellation, minimum-window containment and native WebKit rendering. The five frontend tests, TypeScript check, production build and mechanical UI detector also passed. The updated Mac bundle built and passed its ad-hoc signature verification.

A direct rendered smoke check at 900×620 with a live 60-step plan verified that the latest activity remained inside the visible conversation (183–422px within a 158–442px pane), the document stayed 900×620, the running tool timer advanced, and the last-event age increased during a quiet command. Screenshots were inspected at 1440×900 and 900×620. Verification used isolated data and deterministic RPC; no model request or installed-app replacement was performed.

The combined todo/activity view was also exercised with a live 60-step plan at 900×620: the summary reported 20/60 completed and the actual in-progress step, expanding displayed all 60 todo rows inside a 180px scrolling list, and the document remained 620px high. The activity integration suite, five frontend tests, TypeScript check, mechanical UI detector and rebuilt signed Mac bundle passed after this change.

## Audit hardening verification — 2026-10-22

- Passed `go test -race ./...` and `go vet ./...`, including rooted I/O, request-origin/Host, WebSocket, duplicate-daemon, delete retry/reservation, transcript transaction failure, cancellation and MCP error/limit regressions.
- Passed eight frontend tests, TypeScript checking and the production UI build. Nineteen script-safety tests exercised actual clone/import/E2E programs against temporary fixtures and a local mock API; no real model request was made.
- Built and ad-hoc-signature-verified the complete Mac bundle in an isolated build tree, without overwriting or restarting the user's running app. All 31 native model/preferences/notification-queue assertions passed.
- All 13 activity/browser/native-WebKit review checks passed, including a permanent delayed-save regression, reconnection, cancellation, long-plan containment and real WebKit rendering at normal, minimum and reduced heights. The general UI review also passed against a daemon with pre-existing projects; the prior project, metadata, model preferences and active selection survived unchanged.
- Direct isolated probes rejected foreign/null Origin API calls and rebinding Hosts, while preserving same-origin/native/dev-proxy authority. Embedded and standalone preview scripts could not read the API or open its WebSocket; classic JavaScript, ES modules, CSS and the main-origin watch connection still worked.
- External-symlink reads/writes/deletes/previews were denied and outside files were unchanged. Root-equivalent file deletion was rejected even while a run was active. A second OS process using the same data directory failed before changing the live run or discovery.
- An abort-acknowledging, never-settling fixture was canceled and released in approximately 0.56 seconds; a subsequent run succeeded. Nested artifact dependencies used the correct directory and PNG bytes survived a base64 roundtrip. MCP resource failures had top-level error envelopes.
- Browser delay probes confirmed file selection is disabled while saving, edits typed during the request remain dirty, a different file cannot inherit a false-clean baseline, and navigation still asks before discarding edits. Conversation selection/creation is locked during pending creation; a failed creation retains the selected conversation and restores controls.

Native OS notification authorization/banner delivery and the actual Quit confirmation dialog were not automated in this pass. Pending notification retries are intentionally in-memory and do not survive app exit. All temporary browser tabs, fixture daemons and build workspaces were removed after verification; no installed app or user data was changed.

## Runtime optimisation verification — 2026-10-25

- `/api/activity`, which the UI and native shell both poll, now reads a compact per-run summary from its own `run_summary` table (written with each journal commit and backfilled from `run_activity` on open) and run headers without prompt or work JSON. Against a copy of real data cloned to 41 projects with 182 KB projections, the handler fell from 48.7 ms to about 8.5 ms in process (104 ms per HTTP request before). The response was byte-identical before and after on the same data.
- The UI polls activity only while its window is visible and resyncs on return. Projects created or changed outside the window (MCP, another editor) appear through a project stamp in the activity response; an isolated daemon showed an API-created project in the sidebar after 1.45 s without a reload or a separate project-list poll.
- Preview watchers walk a project only while a preview client is connected and re-baseline silently on connect; previously every project was walked every 250 ms for the daemon's lifetime. A regression test covers idle, connect and change delivery.
- Run event streams check completion with a status-only query instead of loading the whole run row every 250 ms.
- Full-page renders scroll the page at the frame height and stitch one snapshot per viewport, hiding fixed and sticky chrome after the first tile. 100vh sections keep their size and scroll-triggered reveals fire. On the Ironline Freight project the previous renderer produced a 5,799 px capture that was black apart from the preloader; the new one shows every section. The independent jury now also sees a full-page capture of `pdSeek` pages after the intro.
- Passed `go test -race ./...`, `go vet ./...`, nine frontend tests, TypeScript, the production UI build, 24 script-safety tests, all 13 activity/browser/native-WebKit review checks, all nine native app review checks and `codesign --verify --deep --strict` on an isolated build. No installed app or user data was changed; benchmarks used a copy of the database.

## Manual checks / known limits

- Enable notifications in Settings, then check the macOS authorization prompt, real banner delivery and notification-click restoration. Automation deliberately did not grant permission. Permission denial does not disable in-app/menu-bar/notch activity.
- Check sleep/wake, Spaces/full-screen and physical multi-monitor/notch arrangements on the owner's setup. Positioning uses the selected screen's visible frame; only the available local arrangement was exercised.
- Active execution and user input are distinct. Legacy runs retain their conversation/plan/render data, not retroactively reconstructed tool histories. Fire-and-forget extension records are diagnostic data; arbitrary terminal custom UIs are not emulated.
- The existing Vite 5/esbuild development dependencies still report two npm audit advisories (one high, one moderate). No unrelated major build-tool upgrade was included. The shipping Mac app uses built static assets, not Vite's development server.
- The built app is `build/πDesigner.app`; the old bundle filename is not automatically deleted or installed over another running app. Bundle identifier, executable names, MCP configuration and the existing data directory remain stable.
