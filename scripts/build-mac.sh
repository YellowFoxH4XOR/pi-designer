#!/usr/bin/env bash
# Build πDesigner.app: a native macOS bundle containing
#   Contents/MacOS/pidesigner-app   the Swift/WKWebView shell
#   Contents/MacOS/pidesigner       the Go daemon (sidecar)
#   Contents/Resources/content/     design systems, skills, templates, craft
#   Contents/Resources/ui/          built web UI
#
# Usage: ./scripts/build-mac.sh [--install]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

APP_NAME="πDesigner"
BUNDLE="build/${APP_NAME}.app"
INSTALL=false
[ "${1:-}" = "--install" ] && INSTALL=true

echo "==> building Go daemon + MCP server"
mkdir -p bin
go build -trimpath -ldflags="-s -w" -o bin/pidesigner ./cmd/pidesigner
go build -trimpath -ldflags="-s -w" -o bin/pidesigner-mcp ./cmd/pidesigner-mcp

echo "==> building web UI"
if [ ! -d ui/node_modules ]; then
  (cd ui && npm install --silent)
fi
(cd ui && npm run build --silent)

echo "==> compiling Swift shell"
rm -rf "$BUNDLE"
mkdir -p "${BUNDLE}/Contents/MacOS" "${BUNDLE}/Contents/Resources"

swiftc \
  -O \
  -target arm64-apple-macos13.0 \
  -framework Cocoa \
  -framework WebKit \
  -framework SwiftUI \
  -framework UserNotifications \
  -o "${BUNDLE}/Contents/MacOS/pidesigner-app" \
  mac/Sources/*.swift

echo "==> compiling renderer CLI"
# Offscreen WKWebView renderer used by the render_preview Pi tool. Built for
# the app bundle and copied to bin/ so dev runs can use the same binary.
swiftc \
  -O \
  -target arm64-apple-macos13.0 \
  -framework Cocoa \
  -framework WebKit \
  -o bin/pidesigner-render \
  mac/Render/main.swift

echo "==> assembling bundle"
cp mac/Info.plist "${BUNDLE}/Contents/Info.plist"
cp bin/pidesigner "${BUNDLE}/Contents/MacOS/pidesigner"
cp bin/pidesigner-mcp "${BUNDLE}/Contents/MacOS/pidesigner-mcp"
cp bin/pidesigner-render "${BUNDLE}/Contents/MacOS/pidesigner-render"

# Content catalog: the canonical trio plus components and previews.
rsync -a --exclude 'source/' --exclude 'DESIGN-*.md' --exclude '.DS_Store' \
  content/ "${BUNDLE}/Contents/Resources/content/"

# Web UI bundle.
rsync -a --exclude '.DS_Store' ui/dist/ "${BUNDLE}/Contents/Resources/ui/"

echo "==> generating icon"
swiftc -O -framework Cocoa -o bin/generate-icon scripts/generate-icon.swift
bin/generate-icon build/AppIcon.iconset
iconutil -c icns build/AppIcon.iconset -o "${BUNDLE}/Contents/Resources/AppIcon.icns"

echo "==> signing (ad-hoc)"
# Ad-hoc signing is enough for local use; it makes the bundle launchable
# without a Gatekeeper prompt on the machine that built it.
codesign --force --deep --sign - "$BUNDLE"
codesign --verify --deep --strict "$BUNDLE"

echo
echo "built: $BUNDLE"
du -sh "$BUNDLE" | awk '{print "  size: "$1}'

if $INSTALL; then
  echo "==> installing to /Applications"
  rm -rf "/Applications/${APP_NAME}.app"
  cp -R "$BUNDLE" /Applications/
  echo "  /Applications/${APP_NAME}.app"
fi

cat <<'EOF'

Run it:
  open build/πDesigner.app

The daemon runs as a child process on a free loopback port.
Data: ~/Library/Application Support/PiDesigner
EOF
