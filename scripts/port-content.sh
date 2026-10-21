#!/usr/bin/env bash
# Port the content layer (design systems, skills, templates, craft laws) out of
# the upstream OpenDesign checkout into ./content.
#
# This is a one-shot vendoring step. Upstream lives in ./open-design.
# We strip the parts that cost bytes and buy us nothing:
#   - source/            importer audit trail, not a runtime input
#   - DESIGN-<lang>.md   391 localized design docs (i18n we don't ship)
#
# Everything else -- the canonical trio (manifest.json / DESIGN.md / tokens.css),
# component fixtures, design-tokens.json, tailwind mapping, preview pages --
# is copied verbatim so all 154 design systems stay intact.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UP="$ROOT/open-design"
DEST="$ROOT/content"

SKILLS=(
  web-clone
  motion-design
  emilkowalski-motion
  web-design-guidelines
)

TEMPLATES=(
  web-prototype
  web-prototype-taste-brutalist
  web-prototype-taste-editorial
  web-prototype-taste-soft
  motion-frames
  tweaks
  critique
  simple-deck
)

echo "==> design systems"
mkdir -p "$DEST/design-systems"
rm -rf "$DEST/design-systems"/*
for pkg in "$UP"/design-systems/*/; do
  slug="$(basename "$pkg")"
  [ "$slug" = "_schema" ] && continue
  out="$DEST/design-systems/$slug"
  mkdir -p "$out"
  # copy everything except the two stripped trees
  (cd "$pkg" && find . -mindepth 1 -maxdepth 1 \
      ! -name 'source' ! -name 'assets' ! -name 'fonts' \
      -exec cp -R {} "$out/" \;)
  # assets/fonts only when present, but skip large binaries we can't use
  for keepdir in assets fonts; do
    [ -d "$pkg/$keepdir" ] && cp -R "$pkg/$keepdir" "$out/"
  done
  # strip localized design docs
  find "$out" -maxdepth 1 -name 'DESIGN-*.md' -delete
done

echo "==> skills"
mkdir -p "$DEST/skills"
# frontend-design is Anthropic's always-on design skill, vendored separately
# from claude-plugins-official; it is not part of the OpenDesign checkout.
find "$DEST/skills" -mindepth 1 -maxdepth 1 ! -name frontend-design -exec rm -rf {} +
for s in "${SKILLS[@]}"; do
  [ -d "$UP/skills/$s" ] || { echo "  skip (missing): $s"; continue; }
  cp -R "$UP/skills/$s" "$DEST/skills/$s"
done

echo "==> design templates"
mkdir -p "$DEST/design-templates"
rm -rf "$DEST/design-templates"/*
for t in "${TEMPLATES[@]}"; do
  [ -d "$UP/design-templates/$t" ] || { echo "  skip (missing): $t"; continue; }
  cp -R "$UP/design-templates/$t" "$DEST/design-templates/$t"
done

echo "==> craft laws"
rm -rf "$DEST/craft"
cp -R "$UP/craft" "$DEST/craft"

echo "==> prompt templates"
rm -rf "$DEST/prompt-templates"
mkdir -p "$DEST/prompt-templates"
cp -R "$UP/prompt-templates/." "$DEST/prompt-templates/" 2>/dev/null || true

echo
echo "==> done"
du -sh "$DEST"/design-systems "$DEST"/skills "$DEST"/design-templates "$DEST"/craft 2>/dev/null
echo "design systems: $(find "$DEST/design-systems" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ') packages"
