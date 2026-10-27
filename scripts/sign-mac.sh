#!/usr/bin/env bash
# Free ad-hoc signing only. This supplies Apple silicon code integrity, not a
# Developer ID identity or notarization. Preserve nested helper entitlements.
set -euo pipefail
APP="${1:?Usage: sign-mac.sh path/to/app}"
FRAMEWORK="$APP/Contents/Frameworks/Sparkle.framework"
for helper in "$FRAMEWORK/Versions/B/XPCServices/Downloader.xpc" \
              "$FRAMEWORK/Versions/B/XPCServices/Installer.xpc" \
              "$FRAMEWORK/Versions/B/Updater.app"; do
  codesign --force --sign - --preserve-metadata=entitlements "$helper"
done
codesign --force --sign - "$FRAMEWORK/Versions/B/Autoupdate"
codesign --force --sign - "$FRAMEWORK"
for binary in "$APP"/Contents/MacOS/*; do codesign --force --sign - "$binary"; done
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"
