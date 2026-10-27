# Free unnotarized release verification

## Current policy

User-requested distribution is **free and unnotarized only**. The publisher no longer imports Developer ID certificates, requests Apple credentials, calls notarization/stapling tools or requires a release environment. Ad-hoc signing remains for Apple silicon code integrity; it is not an Apple-verified developer identity.

With no Sparkle keys configured, publishing produces a DMG and checksum, omits the update feed, and leaves automatic installation disabled. Settings provides a real **Download latest release** link instead of a disabled automatic-update action. A complete optional Sparkle keypair can enable cryptographically verified updates independently of Apple notarization; partial configuration still fails closed.

## Local checks

- `go test -race ./...` and `go vet ./...`: passed.
- UI tests: 16 passed, including manual release-link fallback, authoritative saved preferences, unavailable/busy states and timeout cleanup. TypeScript checking and production Vite build passed.
- Script tests: 29 passed.
- Python metadata tests: 6 passed, including a release build with no Apple credentials or update keys, release-link routing, opt-in signing requirements, version/feed/key validation and monotonic release versions.
- Shell syntax checks, `git diff --check` and actionlint 1.7.12: passed.

## macOS CI and publication

The initial Sparkle implementation was merged in PR #13. Its first CI attempts were blocked while the source repository was private. After the owner authorized public visibility, both native and Linux checks passed:
- https://github.com/YellowFoxH4XOR/pi-designer/actions/runs/40586045947
- https://github.com/YellowFoxH4XOR/pi-designer/actions/runs/40586539498

Those runs compiled/signed the app, ran native and cryptographic signature tests, and built a development DMG. They did not publish a release.

For the free-release follow-up, `checks.yml` now exercises the actual `PIDESIGNER_RELEASE_BUILD=1` packaging path without any Apple credentials or Sparkle keys and asserts there is no unsigned `appcast.xml`. The native test checks the public manual-download destination. PR CI must pass before merging and tagging the first public release; the tag workflow repeats tests and verifies the DMG before publication.

## Coverage limits

CI verifies build, code integrity, packaging and signature rejection—not a human macOS installation or Gatekeeper override. A full keyboard/VoiceOver/visual settings pass, physical-Mac first launch and actual older-to-newer Sparkle installation remain manual checks. Automatic updates are not enabled in the initial keyless release, so no claim of a working production auto-update path is made. Existing keyless builds need a manual DMG replacement before they can trust a future update key.

The repository is now public. No billing plan or spending budget was changed. No private key or GitHub token is embedded in the app or committed.
