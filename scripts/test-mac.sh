#!/usr/bin/env bash
# Native tests use no installed app, user data, permissions or update feed.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
SPARKLE_DIR="$(scripts/fetch-sparkle.sh)"
TEMP="$(mktemp -d "${TMPDIR:-/tmp}/pidesigner-tests.XXXXXX")"
trap 'rm -rf "$TEMP"' EXIT
swiftc -F "$SPARKLE_DIR" -framework Sparkle -Xlinker -rpath -Xlinker "$SPARKLE_DIR" \
  -o "$TEMP/native-tests" mac/Sources/ActivityModels.swift mac/Sources/AppUpdates.swift mac/Tests/main.swift
"$TEMP/native-tests"
swift scripts/tests/update-signature-test.swift
