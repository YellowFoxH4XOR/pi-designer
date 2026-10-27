#!/usr/bin/env bash
# Package a built app as a drag-to-Applications DMG. Release mode additionally
# notarizes/staples both app and DMG, and generates a signed Sparkle appcast.
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
  : "${MACOS_SIGNING_IDENTITY:?Missing Developer ID signing identity}"
  : "${NOTARY_PROFILE:?Missing notarization credential profile}"
  : "${NOTARY_KEYCHAIN:?Missing notarization keychain path}"
  : "${SPARKLE_PRIVATE_KEY_FILE:?Missing Sparkle private key file}"
  : "${RELEASE_REPOSITORY:?Missing public release repository}"
  [[ "$MACOS_SIGNING_IDENTITY" == 'Developer ID Application: '* ]] || { echo 'A Developer ID Application identity is required.' >&2; exit 1; }
  FEED="$(python3 scripts/release-metadata.py feed-url "$RELEASE_REPOSITORY")"
  [ "$(/usr/libexec/PlistBuddy -c 'Print SUFeedURL' "$APP/Contents/Info.plist")" = "$FEED" ] || { echo 'The embedded update feed does not match the release repository.' >&2; exit 1; }
fi
codesign --verify --deep --strict "$APP"
TEMP="$(mktemp -d "${TMPDIR:-/tmp}/pidesigner-package.XXXXXX")"
trap 'rm -rf "$TEMP"' EXIT
notarize() {
  xcrun notarytool submit "$1" --keychain-profile "$NOTARY_PROFILE" --keychain "$NOTARY_KEYCHAIN" --wait
}
if ! $DEVELOPMENT; then
  ditto -c -k --sequesterRsrc --keepParent "$APP" "$TEMP/PiDesigner-notary.zip"
  notarize "$TEMP/PiDesigner-notary.zip"
  xcrun stapler staple "$APP"
  xcrun stapler validate "$APP"
  spctl --assess --type execute --verbose=2 "$APP"
fi
mkdir -p "$TEMP/dmg" "$OUT"
ditto "$APP" "$TEMP/dmg/πDesigner.app"
ln -s /Applications "$TEMP/dmg/Applications"
DMG="$OUT/PiDesigner-$VERSION-arm64.dmg"
hdiutil create -volname 'πDesigner' -srcfolder "$TEMP/dmg" -fs APFS -format ULFO "$DMG"
hdiutil verify "$DMG"
if ! $DEVELOPMENT; then
  codesign --force --sign "$MACOS_SIGNING_IDENTITY" --timestamp "$DMG"
  notarize "$DMG"
  xcrun stapler staple "$DMG"
  xcrun stapler validate "$DMG"
  spctl --assess --type open --context context:primary-signature --verbose=2 "$DMG"
  SPARKLE_DIR="$(scripts/fetch-sparkle.sh)"
  "$SPARKLE_DIR/bin/generate_appcast" --ed-key-file "$SPARKLE_PRIVATE_KEY_FILE" \
    --download-url-prefix "https://github.com/$RELEASE_REPOSITORY/releases/download/v$VERSION/" \
    --link "https://github.com/$RELEASE_REPOSITORY/releases/tag/v$VERSION" "$OUT"
  "$SPARKLE_DIR/bin/sign_update" --ed-key-file "$SPARKLE_PRIVATE_KEY_FILE" --verify "$OUT/appcast.xml"
  PUBLIC_KEY="$(/usr/libexec/PlistBuddy -c 'Print SUPublicEDKey' "$APP/Contents/Info.plist")"
  swift scripts/verify-update.swift "$PUBLIC_KEY" "$DMG" "$OUT/appcast.xml"
fi
(cd "$OUT"; shasum -a 256 ./*.dmg > SHA256SUMS)
echo "Packaged: $DMG"
if $DEVELOPMENT; then echo 'DEVELOPMENT ONLY: not notarized and not suitable for publishing or automatic updates.'; fi
