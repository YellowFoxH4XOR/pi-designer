# Independent jury verification — 2026-10-24

## Scope

Completed the interrupted independent-jury benchmark work. The daemon still delegates all inference to the installed pi; no provider credentials or direct provider calls were added. No installed app was replaced or restarted, and no user project data was changed.

The benchmark now sets the daemon's `PIDESIGNER_JURY_MODEL` to its configured judge, preserves full per-round reviews, and writes an initial recovery record with data directory/project/run IDs before polling. Its HTML report distinguishes panel scores from jury scores and exposes per-round feedback. Both jury paths reject malformed, missing or out-of-range scores rather than silently accepting them. The critique schema now permits the documented `none` signature response.

## Checks

- `go test -race ./...` and `go vet ./...`: passed.
- `(cd ui && npm test && npx tsc --noEmit && npm run build)`: passed, 9 UI tests.
- `node --test scripts/tests/*.test.cjs`: passed, 24 tests including four new jury regressions exercising the actual embedded extension with mocked process/render boundaries (no inference).
- Regressions cover independent scores overriding self-scores, revision/three-round limits, panel must-fix and missing-signature gates, and disclosed fallback on malformed jury scores.
- Chrome review of the live isolated Studio: independent model identity, all four scores, verdict and issues were present; no page errors. At 900×620, document dimensions remained 900×620. Screenshots: `.impeccable/review/independent-jury-visible-{1440,900}.png`. This was not a full keyboard or VoiceOver audit.

## Real inference

Command:

```sh
node scripts/design-bench.cjs --model openai-codex/gpt-6.1-sol \
  --judge github-copilot/gemini-4-argon --only ceramics-launch --minutes 35
```

Generation settled successfully in 17.2 minutes. The in-loop jury scored 7.1, 5.7, then 7.0; the last decision was **ship_with_issues**, correctly below the 7.5 independent ship bar. A fresh final judging call scored **6.97/10** (Design 7.2 · Usability 7.5 · Creativity 5.8 · Content 6.8).

Earlier single-brief scores were 6.42 at baseline and 6.66 after prompt tuning. The new result is +0.31 over the latter, but this is one stochastic brief, not evidence of a reliable general improvement or award-level output. Revisions were not monotonically better. Collection imagery, original concept, studio-visit detail and mobile navigation remain weaknesses. Business details invented during generation are not verified facts; proposed logistics need owner approval before publication.

Report and final desktop/full-page/mobile captures:
`.impeccable/bench/2026-10-24T18-57-59-988Z/index.html`.
The adjacent `report.json` preserves complete per-round evidence and the temporary data directory. The benchmark daemon exited after completion; no benchmark process was left running.

## Limits

Independent review adds real inference cost and latency. If it cannot run or its response is invalid, the extension explicitly discloses fallback to the self-panel; this is not a fail-closed certification gate. At the time of this pass, the jury assumed an `index.html` entry (corrected in the review follow-up below) and used screenshot-visible evidence, not interactive browser execution. No broad benchmark suite or motion-quality comparison was run in this pass.

## Review follow-up — restore safety and jury correctness

Changes:
- Jury issues now prevent `ship`, even with a passing overall score. After three unresolved rounds the result remains `ship_with_issues`. Blank issue strings do not create artificial blockers.
- The runner passes the configured project entry to the extension. Independent review uses it, or the most recently rendered file for a new project, with `index.html` as the final default. `submit_critique.path` can select another entry. Realpath confinement rejects traversal and escaping symlinks.
- Version restore validates paths, blob hashes and size limits and stages the entire target before mutation. Current files are backed up on disk. A write/delete failure rolls back touched files, original permissions and newly created directories; rollback failures retain the backup and return its recovery path.
- The API saves pre-restore manual edits as a version, making those edits recoverable after a successful restore.

Verification:
- `go test -race ./...` and `go vet ./...`: passed using a temporary Go 1.26.0 toolchain.
- UI tests, TypeScript checking and production build: passed (12 UI tests).
- `node --test scripts/tests/*.test.cjs`: passed (29 tests). The embedded extension is exercised with mocked rendering/process boundaries, including passing-score jury issues, round limits, nested/configured/explicit/rendered entries, and path confinement.
- Go regressions cover missing/corrupt blobs without project mutation, injected partial-write and delete failures with rollback, retained backups when rollback fails, symlink/directory conflicts, preserved excluded state/oversized files, and pre-restore edit history through the real HTTP API.

Limits: no live inference, native WebKit, macOS bundle or browser visual verification was performed for these backend/tool changes. Restore is not crash-atomic and does not coordinate with external filesystem writers. Symlinks and file/directory shape conflicts are conservatively rejected. An abrupt termination can leave a `.restore-*` directory; automatic startup recovery is not implemented. For manual recovery, stop project writers, preserve a copy of the project and recovery directory, inspect `recovery.json`, remove newly introduced files recorded only in `after/`, and restore original files from `before/` (permissions are recorded in `modes`). Do not replace the entire project directory: excluded `.pi`, `.git`, and dependency state are intentionally outside the backup.
