#!/usr/bin/env bash
# Free, unnotarized drag-to-Applications DMG. Optional Sparkle archive/feed
# signatures are independent of Apple's paid Developer ID program.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
[ "$(uname -s)" = Darwin ] || { echo 'DMG packaging requires macOS.' >&2; exit 1; }
DEVELOPMENT=false
[ "${1:-}" = --development ] && DEVELOPMENT=true
APP="$ROOT/build/πDesigner.app"
VERSION="$(/usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' "$APP/Contents/Info.plist")"
python3 scripts/release-metadata.py check-newer "$VERSION" ''
OUT="$ROOT/build/release/v$VERSION"
[ ! -e "$OUT" ] || { echo "Output already exists: $OUT (use a clean build for a release)." >&2; exit 1; }
if ! $DEVELOPMENT; then
  [ "${PIDESIGNER_RELEASE_BUILD:-0}" = 1 ] || { echo 'Release packaging requires PIDESIGNER_RELEASE_BUILD=1.' >&2; exit 1; }
  : "${RELEASE_REPOSITORY:?Missing public release repository}"
fi
RELEASE_REPOSITORY="${RELEASE_REPOSITORY:-YellowFoxH4XOR/pi-designer}"
FEED="$(python3 scripts/release-metadata.py feed-url "$RELEASE_REPOSITORY")"
EMBEDDED_FEED="$(/usr/libexec/PlistBuddy -c 'Print SUFeedURL' "$APP/Contents/Info.plist" 2>/dev/null)" || EMBEDDED_FEED=''
PUBLIC_KEY="$(/usr/libexec/PlistBuddy -c 'Print SUPublicEDKey' "$APP/Contents/Info.plist" 2>/dev/null)" || PUBLIC_KEY=''
UPDATES=false
if [[ -n "$EMBEDDED_FEED" || -n "$PUBLIC_KEY" || -n "${SPARKLE_PRIVATE_KEY_FILE:-}" ]]; then
  [ "$EMBEDDED_FEED" = "$FEED" ] || { echo 'Embedded update feed does not match the release repository.' >&2; exit 1; }
  [ -n "$PUBLIC_KEY" ] || { echo 'Missing embedded update public key.' >&2; exit 1; }
  : "${SPARKLE_PRIVATE_KEY_FILE:?Update-enabled builds require a private signing key file}"
  UPDATES=true
fi
codesign --verify --deep --strict "$APP"
TEMP="$(mktemp -d "${TMPDIR:-/tmp}/pidesigner-package.XXXXXX")"
trap 'rm -rf "$TEMP"' EXIT
mkdir -p "$TEMP/dmg" "$OUT"
ditto "$APP" "$TEMP/dmg/πDesigner.app"
ln -s /Applications "$TEMP/dmg/Applications"
DMG="$OUT/PiDesigner-$VERSION-arm64.dmg"
hdiutil create -volname 'πDesigner' -srcfolder "$TEMP/dmg" -fs APFS -format ULFO "$DMG"
hdiutil verify "$DMG"
if $UPDATES; then
  SPARKLE_DIR="$(scripts/fetch-sparkle.sh)"
  "$SPARKLE_DIR/bin/generate_appcast" --ed-key-file "$SPARKLE_PRIVATE_KEY_FILE" \
    --download-url-prefix "https://github.com/$RELEASE_REPOSITORY/releases/download/v$VERSION/" \
    --link "https://github.com/$RELEASE_REPOSITORY/releases/tag/v$VERSION" "$OUT"
  "$SPARKLE_DIR/bin/sign_update" --ed-key-file "$SPARKLE_PRIVATE_KEY_FILE" --verify "$OUT/appcast.xml"
  swift scripts/verify-update.swift "$PUBLIC_KEY" "$DMG" "$OUT/appcast.xml"
fi
(cd "$OUT"; shasum -a 256 ./*.dmg > SHA256SUMS)
cat > "$OUT/release-notes.md" <<EOF
## πDesigner $VERSION — free, unnotarized macOS release

Download **PiDesigner-$VERSION-arm64.dmg**, open it, and drag **πDesigner.app** into **Applications**.

### macOS security notice
This app is **ad-hoc signed, not Developer ID signed or notarized by Apple**. macOS may block the first launch because it cannot verify the developer. If you trust this release, try opening the app, then use **System Settings → Privacy & Security → Open Anyway**. Do not disable Gatekeeper globally. See [Apple's guidance](https://support.apple.com/102445).

### Requirements
- Apple silicon Mac (M1 or newer), macOS 13 or later.
- The separately installed pi coding agent (1.1.0 or newer), from https://pi.dev. Launch pi and use \`/login\` to configure your model provider.

Projects and sessions remain in your local Application Support directory. App downloads do not install or update pi.
EOF
if $UPDATES; then
  printf '\n### Updates\nThis build includes a signed Sparkle update feed. Check from Settings → App updates. Automatic download/install-on-quit is optional. Update signatures are separate from Apple notarization.\n' >> "$OUT/release-notes.md"
else
  printf '\n### Updates\nAutomatic updates are **disabled** because no update-signing keys are configured. Use Settings → App updates → Download latest release to install future DMGs manually. No unsigned automatic installer is enabled.\n' >> "$OUT/release-notes.md"
fi
echo "Packaged (UNNOTARIZED): $DMG"
if $DEVELOPMENT; then echo 'Development smoke build; not published.'; fi
