# macOS releases and updates

πDesigner ships an Apple silicon (`arm64`), macOS 13+ application. A stable tag such as `v0.3.0` triggers `.github/workflows/release.yml` to build a Developer ID signed, notarized, stapled DMG and a Sparkle-signed appcast. No release is published unless signing, notarization and signature verification succeed.

## One-time setup

### 1. Choose a public download destination

The source repository is currently private. GitHub does not expose private release assets anonymously; embedding a GitHub token in a desktop app is not acceptable.

Either explicitly make the source repository public, or create a **separate public, initialized repository** for binaries (for example, an owner-selected `pi-designer-releases`). This workflow never changes repository visibility or creates that repository for you.

In the source repository, create a GitHub Actions environment named `release` (recommended: require approval). Add these environment variables:

| Variable | Value |
| --- | --- |
| `RELEASE_REPOSITORY` | `owner/public-binary-repository`; omit only if the source itself is public |
| `MACOS_SIGNING_IDENTITY` | Full `Developer ID Application: … (TEAMID)` identity |
| `SPARKLE_PUBLIC_KEY` | Public Ed25519 key printed by Sparkle's `generate_keys` |

If the release destination is a different repository, add `RELEASE_TOKEN`: a fine-grained token restricted to that destination with Contents read/write. The built app never receives it. For same-repository releases, the workflow's temporary `GITHUB_TOKEN` is used instead.

### 2. Apple signing and notarization

Public releases require an Apple Developer Program membership and a **Developer ID Application** certificate (not an Apple Development certificate). Export that identity **including its private key** as a password-protected `.p12` from Keychain Access.

Add these secrets to the `release` environment through GitHub's encrypted secret UI/CLI, never as committed files:

| Secret | Value |
| --- | --- |
| `MACOS_CERTIFICATE_BASE64` | Base64 contents of the exported `.p12` |
| `MACOS_CERTIFICATE_PASSWORD` | Its export password |
| `APPLE_ID` | Account used for notarization |
| `APPLE_TEAM_ID` | Apple developer team ID |
| `APPLE_APP_PASSWORD` | App-specific password for notarization |

The workflow imports the certificate into an ephemeral keychain, stores a notarytool credential profile there, signs nested Sparkle helpers inside-out (preserving their entitlements), signs the app with hardened runtime, notarizes/staples the app, builds/signs/notarizes/staples the DMG, and deletes signing material in an `always()` cleanup step. Notarization credentials and certificate keys never enter the app bundle.

### 3. Sparkle signing keys

On a trusted Mac:

```bash
SPARKLE_DIR="$(./scripts/fetch-sparkle.sh)"
"$SPARKLE_DIR/bin/generate_keys"
# Export to a protected location outside this checkout, following the tool's help:
"$SPARKLE_DIR/bin/generate_keys" -x /secure/location/pidesigner-update-key
```

Set the printed **public** key as the `SPARKLE_PUBLIC_KEY` Actions variable. Store the exported private-key file contents as the `SPARKLE_PRIVATE_KEY` environment secret, for example via secure file input:

```bash
gh secret set SPARKLE_PRIVATE_KEY --env release < /secure/location/pidesigner-update-key
```

Back up this key securely and keep it stable across releases. Do not regenerate it per build. The public key is embedded in `Info.plist`; only the private key signs updates in CI. Losing/changing the key can break updates for existing users; follow [Sparkle's key-rotation instructions](https://sparkle-project.org/documentation/#rotating-signing-keys).

The workflow uses Sparkle's `generate_appcast`, verifies its signed feed, then independently verifies the DMG signature against the app's embedded public key using CryptoKit. Mismatched public/private configuration fails the release.

## Publish a release

Merge the release implementation into `main`, configure the environment above, and tag a commit already on `main`:

```bash
git switch main
git pull --ff-only
git tag v0.3.0
git push origin v0.3.0
```

Use a strictly increasing stable `major.minor.patch`; prerelease suffixes are intentionally rejected. The tag controls both `CFBundleVersion` and the display version, so a plist-only version bump is not required for future releases. The workflow rejects versions not newer than the latest published stable release.

Assets:
- `PiDesigner-<version>-arm64.dmg` — app plus an Applications shortcut.
- `appcast.xml` — signed feed referencing that version's immutable DMG URL.
- `SHA256SUMS` — DMG checksum (computed after signing/stapling).

All assets are uploaded to a draft first; only then is the release published and marked latest. The stable feed URL embedded in the app is:

```text
https://github.com/<release-owner>/<release-repo>/releases/latest/download/appcast.xml
```

Do not edit the signed appcast, replace published DMGs, or mark older releases latest. Publish a new higher version for corrections. A failed publication can leave a draft; inspect it before deleting/retrying. The feed currently advertises the latest version only; historical/delta update feeds and Intel builds are not implemented. If a future release raises minimum macOS requirements, retain compatible items in the feed before publishing it.

## User experience

1. Download the DMG, open it and drag **πDesigner.app** into Applications before launching. Running from the mounted DMG is read-only and cannot be updated in place.
2. Install pi separately from https://pi.dev and configure its provider with `/login`. App updates do not install/update pi or change its credentials.
3. **Settings → App updates** and the native **πDesigner → Check for Updates…** menu open Sparkle's normal update UI. It checks, downloads, verifies, installs and relaunches the app; no manual app replacement is needed.
4. Daily update checks default on. Automatic downloads/install-on-quit default off and can be enabled in Settings. These are Sparkle-backed preferences, not duplicate web-storage flags. Updates never silently force a quit; the existing unsaved-edit/active-task termination guards remain in effect.
5. Local project/database/session state in Application Support is outside the app bundle and is not replaced by an update.

Developer builds still work without signing configuration. They embed Sparkle but omit the feed/public key and explicitly disable update controls. Capture tests disable the updater; other isolated native tests can set `PIDESIGNER_DISABLE_UPDATES=1`. Existing pre-updater app installations need the first new DMG installed manually.

## Local development and verification

```bash
./scripts/build-mac.sh
./scripts/test-mac.sh
./scripts/package-mac.sh --development
```

The last command makes a **non-notarized smoke-test DMG**, not a public release. Output is under `build/release/v<version>/`; packaging refuses to overwrite an existing output directory. Remove only that generated directory before rebuilding it.

`checks.yml` runs native build/signature/DMG smoke checks on macOS and tests/typechecks on Linux. It never uses release secrets or publishes a release from a PR. Before the first public release, test a real signed older-to-newer update on a Mac: feed/signature failures, cancel/retry, automatic install-on-quit, user opt-out across relaunch, install from Applications vs a mounted DMG, unsaved edits, active runs and notification behavior. CI smoke tests cannot certify the full notarized upgrade/relaunch flow.

Sources: [Sparkle setup](https://sparkle-project.org/documentation/), [programmatic setup](https://sparkle-project.org/documentation/programmatic-setup/), [publishing](https://sparkle-project.org/documentation/publishing/), [settings](https://sparkle-project.org/documentation/preferences-ui/).
