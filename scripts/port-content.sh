#!/usr/bin/env bash
# Port the content layer (design systems, skills, templates, craft laws) out of
# the upstream OpenDesign checkout into ./content.
#
# The existing content tree is copied to same-filesystem staging, updated there,
# then swapped in only after every required source has been copied successfully.
# Local skills not imported from OpenDesign, including frontend-design, survive.
set -Eeuo pipefail

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

fail() {
  printf 'port-content: %s\n' "$*" >&2
  exit 1
}

require_dir() {
  [[ -d "$1" && -r "$1" ]] || fail "required source directory is missing or unreadable: $1"
}

require_file() {
  [[ -f "$1" && -r "$1" ]] || fail "required source file is missing or unreadable: $1"
}

require_nonempty_dir() {
  local first_file
  first_file="$(find "$1" -type f -print -quit)"
  [[ -n "$first_file" ]] || fail "required source directory contains no files: $1"
}

require_dir "$UP/design-systems"
require_dir "$UP/skills"
require_dir "$UP/design-templates"
require_dir "$UP/craft"
require_dir "$UP/prompt-templates"
require_nonempty_dir "$UP/craft"
require_nonempty_dir "$UP/prompt-templates"

system_count=0
for pkg in "$UP"/design-systems/*/; do
  [[ -d "$pkg" ]] || continue
  [[ "$(basename "$pkg")" == "_schema" ]] && continue
  require_file "$pkg/manifest.json"
  require_file "$pkg/DESIGN.md"
  require_file "$pkg/tokens.css"
  system_count=$((system_count + 1))
done
(( system_count > 0 )) || fail "no complete design-system packages found in $UP/design-systems"

for skill in "${SKILLS[@]}"; do
  require_dir "$UP/skills/$skill"
  require_file "$UP/skills/$skill/SKILL.md"
done
for template in "${TEMPLATES[@]}"; do
  require_dir "$UP/design-templates/$template"
  require_file "$UP/design-templates/$template/SKILL.md"
done


[[ ! -L "$DEST" ]] || fail "refusing to replace symbolic-link content directory: $DEST"
if [[ -e "$DEST" && ! -d "$DEST" ]]; then
  fail "content destination is not a directory: $DEST"
fi

TMP="$(mktemp -d "$ROOT/.port-content.XXXXXX")"
STAGED="$TMP/content"
BACKUP="$TMP/content.previous"

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  if [[ -e "$BACKUP" && ! -e "$DEST" ]]; then
    if ! mv "$BACKUP" "$DEST"; then
      printf 'port-content: failed to restore the previous catalog; backup retained at %s\n' "$BACKUP" >&2
      status=1
      exit "$status"
    fi
  fi
  if ! rm -rf "$TMP"; then
    printf 'port-content: failed to remove temporary staging directory %s\n' "$TMP" >&2
    status=1
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir "$STAGED"
if [[ -d "$DEST" ]]; then
  cp -R "$DEST/." "$STAGED/"
fi

echo "==> design systems"
rm -rf "$STAGED/design-systems"
mkdir -p "$STAGED/design-systems"
for pkg in "$UP"/design-systems/*/; do
  [[ -d "$pkg" ]] || continue
  slug="$(basename "$pkg")"
  [[ "$slug" == "_schema" ]] && continue
  out="$STAGED/design-systems/$slug"
  mkdir -p "$out"
  (cd "$pkg" && find . -mindepth 1 -maxdepth 1 \
      ! -name 'source' ! -name 'assets' ! -name 'fonts' \
      -exec cp -R {} "$out/" \;)
  for keepdir in assets fonts; do
    [[ ! -d "$pkg/$keepdir" ]] || cp -R "$pkg/$keepdir" "$out/"
  done
  find "$out" -maxdepth 1 -name 'DESIGN-*.md' -delete
done

echo "==> skills"
mkdir -p "$STAGED/skills"
PRESERVE="$TMP/preserved-web-clone"
mkdir -p "$PRESERVE/scripts"
if [[ -f "$STAGED/skills/web-clone/SKILL.md" ]]; then
  cp -p "$STAGED/skills/web-clone/SKILL.md" "$PRESERVE/SKILL.md"
fi
if [[ -f "$STAGED/skills/web-clone/scripts/init-clone.mjs" ]]; then
  cp -p "$STAGED/skills/web-clone/scripts/init-clone.mjs" "$PRESERVE/scripts/init-clone.mjs"
fi
for skill in "${SKILLS[@]}"; do
  rm -rf "$STAGED/skills/$skill"
  cp -R "$UP/skills/$skill" "$STAGED/skills/$skill"
done
if [[ -f "$PRESERVE/SKILL.md" ]]; then
  cp -p "$PRESERVE/SKILL.md" "$STAGED/skills/web-clone/SKILL.md"
fi
if [[ -f "$PRESERVE/scripts/init-clone.mjs" ]]; then
  mkdir -p "$STAGED/skills/web-clone/scripts"
  cp -p "$PRESERVE/scripts/init-clone.mjs" "$STAGED/skills/web-clone/scripts/init-clone.mjs"
fi

PROTOTYPE_PRESERVE="$TMP/preserved-web-prototype"
PROTOTYPE_ADAPTERS=(
  SKILL.md
  assets/template.html
  references/layouts.md
  references/checklist.md
)
for adapter in "${PROTOTYPE_ADAPTERS[@]}"; do
  source="$STAGED/design-templates/web-prototype/$adapter"
  if [[ -f "$source" ]]; then
    mkdir -p "$PROTOTYPE_PRESERVE/$(dirname "$adapter")"
    cp -p "$source" "$PROTOTYPE_PRESERVE/$adapter"
  fi
done

echo "==> design templates"
rm -rf "$STAGED/design-templates"
mkdir -p "$STAGED/design-templates"
for template in "${TEMPLATES[@]}"; do
  cp -R "$UP/design-templates/$template" "$STAGED/design-templates/$template"
done
for adapter in "${PROTOTYPE_ADAPTERS[@]}"; do
  preserved="$PROTOTYPE_PRESERVE/$adapter"
  if [[ -f "$preserved" ]]; then
    destination="$STAGED/design-templates/web-prototype/$adapter"
    mkdir -p "$(dirname "$destination")"
    cp -p "$preserved" "$destination"
  fi
done


echo "==> craft laws"
rm -rf "$STAGED/craft"
cp -R "$UP/craft" "$STAGED/craft"

echo "==> prompt templates"
rm -rf "$STAGED/prompt-templates"
mkdir -p "$STAGED/prompt-templates"
cp -R "$UP/prompt-templates/." "$STAGED/prompt-templates/"

if [[ -e "$DEST" ]]; then
  mv "$DEST" "$BACKUP"
fi
if ! mv "$STAGED" "$DEST"; then
  if [[ -e "$BACKUP" ]]; then
    mv "$BACKUP" "$DEST" || fail "could not restore $DEST after the staged catalog replacement failed"
  fi
  fail "could not replace $DEST with the completed staged catalog"
fi

echo
echo "==> done"
du -sh "$DEST"/design-systems "$DEST"/skills "$DEST"/design-templates "$DEST"/craft || true
printf 'design systems: %s packages\n' "$(find "$DEST/design-systems" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
