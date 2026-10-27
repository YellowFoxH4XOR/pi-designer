# Free, unnotarized macOS releases

πDesigner distributes **free, unnotarized** Apple silicon (`arm64`), macOS 13+ DMGs. No Apple Developer membership, Developer ID certificate, notarization account, payment method or signing secrets are required to publish a manual-download release.

The app uses free **ad-hoc code signing** for Apple silicon executable integrity. This is not an Apple-verified developer identity and is not notarization. macOS can block the first launch.

## Download and install

1. Open [GitHub Releases](https://github.com/YellowFoxH4XOR/pi-designer/releases/latest).
2. Download `PiDesigner-<version>-arm64.dmg` and open it.
3. Drag **πDesigner.app** into **Applications** before launching.
4. If macOS blocks the app and you trust this release, try opening it, then use **System Settings → Privacy & Security → Open Anyway**. See [Apple's guidance](https://support.apple.com/102445). Do not disable Gatekeeper globally.
5. Install pi separately from https://pi.dev and configure your provider using `/login`. App downloads do not install/update pi or change its credentials.

Saved projects, conversations and sessions live in Application Support, outside the app bundle. Replacing the app does not replace those files. Quit the old app first, saving edits and letting active work finish.

## Publish a release

The repository is public and standard GitHub-hosted runner usage is free. Merge changes through a PR into `main`, then push a strictly increasing stable version tag:

```bash
git switch main
git pull --ff-only
git tag v0.3.0
git push origin v0.3.0
```

`.github/workflows/release.yml` then:
- Validates that the tagged commit is on `main` and the version is newer than the latest release.
- Runs Go/UI/script/native tests and builds matching arm64 executables.
- Ad-hoc signs the app inside-out, preserving Sparkle helper entitlements.
- Creates and verifies the DMG with an Applications shortcut.
- Uploads the DMG and `SHA256SUMS` to a draft, then publishes it as latest.

The tag controls both the display version and build version. Prerelease suffixes are intentionally rejected. No `APPLE_*`, certificate, notarization or Developer ID configuration is used. The workflow uses the repository's temporary `GITHUB_TOKEN`; no personal token or release environment is needed for same-repository publication.

Default assets:
- `PiDesigner-<version>-arm64.dmg`
- `SHA256SUMS`

The release notes explicitly disclose the unnotarized status, requirements, first-launch instructions and update availability. Do not replace already published DMGs or mark older versions latest; publish a higher version for corrections. A failed publication can leave a draft; inspect it before deleting/retrying.

For another **public** binary repository, set repository variable `RELEASE_REPOSITORY=owner/repository` and repository secret `RELEASE_TOKEN` restricted to that destination with Contents read/write. Never embed it in the app. Private release assets cannot be downloaded anonymously.

## Updates: manual by default, signed automation optional

Without update-signing keys, **automatic installation is disabled**. Settings → App updates offers **Download latest release**, opening the configured GitHub release page. Install the new DMG manually. The app does not download or execute unsigned automatic updates.

Secure automatic updates can be enabled later with a persistent Sparkle Ed25519 keypair. These keys are **free and independent of Apple notarization**; configuring them does not turn the app into a notarized app or remove Gatekeeper warnings.

On a trusted Mac, use the pinned Sparkle tools:

```bash
SPARKLE_DIR="$(./scripts/fetch-sparkle.sh)"
"$SPARKLE_DIR/bin/generate_keys"
"$SPARKLE_DIR/bin/generate_keys" -x /secure/location/pidesigner-update-key
```

Set the printed public key as repository variable `SPARKLE_PUBLIC_KEY`, and store the exported private-key file contents as repository secret `SPARKLE_PRIVATE_KEY`, for example:

```bash
gh secret set SPARKLE_PRIVATE_KEY < /secure/location/pidesigner-update-key
```

Keep a secure backup and reuse the keypair across releases. Do not generate a new key per build. With both configured, the workflow embeds the public key/feed URL, generates and verifies the signed appcast, verifies the DMG against the embedded public key with CryptoKit, and publishes `appcast.xml` as an additional asset. Partial/mismatched key configuration fails instead of silently publishing an insecure feed. The stable feed is `https://github.com/<owner>/<repo>/releases/latest/download/appcast.xml`.

Once a user installs an update-enabled build manually, Settings and the native **Check for Updates…** menu can use Sparkle. Daily checks default on, automatic download/install-on-quit is opt-in, and the existing unsaved-edit/active-task quit guards remain in effect. The private key and GitHub token never enter the app bundle. Existing builds without the key must be replaced manually once; they cannot gain trust in a new key remotely.

No keys are configured for the initial manual-download release. Native signed older-to-newer installation, opt-out persistence, cancellation and active-run behavior still require a real Mac upgrade test before treating the optional automated path as fully verified. The appcast currently retains only the latest version; keep compatible entries if future versions raise the minimum macOS requirement.

## Development and checks

```bash
./scripts/build-mac.sh
./scripts/test-mac.sh
./scripts/package-mac.sh --development
# Exercise the actual no-secrets release packaging path locally:
PIDESIGNER_RELEASE_BUILD=1 RELEASE_REPOSITORY=YellowFoxH4XOR/pi-designer ./scripts/package-mac.sh
```

Packaging refuses to overwrite `build/release/v<version>/`; remove only that generated output before repeating it. Both modes produce unnotarized DMGs. `--development` skips only the release invocation guard; it does not disable signature verification when update keys are present.

PR checks exercise the real no-credentials packaging path on macOS. Capture/test sessions disable update checks; isolated native tests can also set `PIDESIGNER_DISABLE_UPDATES=1`. Local builds without a key have the same explicit manual-download fallback as release builds.

Sources: [Sparkle setup](https://sparkle-project.org/documentation/), [publishing](https://sparkle-project.org/documentation/publishing/), [settings](https://sparkle-project.org/documentation/preferences-ui/).
