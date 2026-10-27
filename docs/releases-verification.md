# Release/update implementation verification

## Implemented surfaces

- `mac/Sources/AppUpdates.swift`: Sparkle's standard signed updater, native state/preferences and menu validation. Test/capture sessions and unconfigured local builds cannot check/install updates.
- `ui/src/components/AppUpdates.tsx`: Settings status/version, check/progress action, automatic-check and automatic-download/install-on-quit controls, timeout/unavailable state. Reuses existing accessible settings tokens/components.
- `mac/Sources/main.swift`: native menu and top-level, same-origin bridge, additionally restricted to the workspace document path. Existing asynchronous unsaved-edit/active-run quit guards are retained.
- `scripts/build-mac.sh`, `sign-mac.sh`, `package-mac.sh`: matched arm64 binaries, pinned Sparkle framework, inside-out signing, app/DMG notarization and stapling, signed appcast and independent Ed25519 verification.
- `.github/workflows/checks.yml` and `release.yml`: PR checks/development DMG smoke packaging, and stable-tag releases gated on public distribution and signing credentials.

## Completed locally (Linux)

- `go test -race ./...` and `go vet ./...`: passed.
- UI tests: 16 passed, including native-only controls, authoritative saved state, busy/unconfigured states, timeout and listener cleanup. TypeScript checking and production Vite build passed.
- Script tests: 29 passed.
- Python release metadata tests: 5 passed, covering version/feed/key validation, no plist writes on invalid input, missing release feed/key configuration, local-build defaults, public-repository routing and monotonic version checks.
- Shell syntax checking, `git diff --check`, and actionlint 1.7.12: passed.
- Sparkle 2.10.1 distribution SHA-256 was verified against the pinned upstream release-asset digest.

## Blocked native verification

PR: https://github.com/YellowFoxH4XOR/pi-designer/pull/13

The first CI run (https://github.com/YellowFoxH4XOR/pi-designer/actions/runs/40585735571) did **not start either job**. GitHub reported: "recent account payments have failed or your spending limit needs to be increased." This is a runner/billing block, not a passing build and not a compiler/test result. Re-run Checks after resolving the account restriction.

The macOS job is configured to compile/sign the actual app, run native model/updater tests, accept correctly signed archives while rejecting wrong keys/tampering with the real CryptoKit verifier, and create/verify a development DMG. None of those native results are claimed yet. No downloadable release DMG has been published.

## Release prerequisites and remaining manual coverage

The source repository remains private, and no production signing secrets or release-destination variables were configured by this change. Follow `releases.md` to choose a public binary repository (or explicitly make the source public), configure an approval-protected release environment, Apple Developer ID/notarization credentials and a persistent Sparkle keypair. Workflow-upload permission was granted separately; no token was embedded in app files or committed.

Before public release, complete an actual notarized older-to-newer installation on a Mac. Verify cancellation, download/verification errors, readonly DMG vs Applications, persisted opt-outs, automatic install-on-quit, unsaved-source and active-run termination protection, and preservation of local project/session state. A full keyboard/VoiceOver/visual settings pass remains outstanding. Old app versions without this updater need the first new DMG installed manually.
