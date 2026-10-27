#!/usr/bin/env bash
# Sign nested executables inside-out. Never use --deep to sign a release.
set -euo pipefail
APP="${1:?Usage: sign-mac.sh path/to/app}"
IDENTITY="${MACOS_SIGNING_IDENTITY:--}"
if [ "${PIDESIGNER_RELEASE_BUILD:-0}" = 1 ] && [[ "$IDENTITY" != 'Developer ID Application: '* ]]; then
  echo 'Release builds require MACOS_SIGNING_IDENTITY (Developer ID Application).' >&2
  exit 1
fi
sign() {
  if [ "$IDENTITY" = - ]; then
    codesign --force --sign - "$@"
  else
    codesign --force --sign "$IDENTITY" --options runtime --timestamp "$@"
  fi
}
FRAMEWORK="$APP/Contents/Frameworks/Sparkle.framework"
# Preserve the upstream helper entitlements (notably Downloader's sandbox).
for helper in "$FRAMEWORK/Versions/B/XPCServices/Downloader.xpc" \
              "$FRAMEWORK/Versions/B/XPCServices/Installer.xpc" \
              "$FRAMEWORK/Versions/B/Updater.app"; do
  sign --preserve-metadata=entitlements "$helper"
done
sign "$FRAMEWORK/Versions/B/Autoupdate"
sign "$FRAMEWORK"
for binary in "$APP"/Contents/MacOS/*; do sign "$binary"; done
sign "$APP"
codesign --verify --deep --strict "$APP"
