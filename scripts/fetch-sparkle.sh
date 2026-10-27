#!/usr/bin/env bash
# Pinned Sparkle binary distribution, including framework and release tools.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION=2.10.1
SHA256=1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27
CACHE="$ROOT/build/deps"
ARCHIVE="$CACHE/Sparkle-$VERSION.tar.xz"
DEST="$CACHE/Sparkle-$VERSION"
mkdir -p "$CACHE"
if [ ! -f "$ARCHIVE" ]; then
  curl --fail --location --retry 3 --proto '=https' --proto-redir '=https' \
    "https://github.com/sparkle-project/Sparkle/releases/download/$VERSION/Sparkle-$VERSION.tar.xz" -o "$ARCHIVE.tmp"
  mv "$ARCHIVE.tmp" "$ARCHIVE"
fi
printf '%s  %s\n' "$SHA256" "$ARCHIVE" | shasum -a 256 -c - >&2
if [ ! -f "$DEST/Sparkle.framework/Sparkle" ]; then
  mkdir -p "$DEST"
  tar -xf "$ARCHIVE" -C "$DEST"
fi
printf '%s\n' "$DEST"
