---
version: 1
slug: "app-identity"
primary_target: "ui/public/brand.svg"
related_targets: ["ui/public/brand-symbol.svg", "scripts/generate-icon.swift", "scripts/build-mac.sh", "mac/Sources/ActivityService.swift", "mac/Sources/ActivitySurface.swift"]
---

# Designer-specific app identity

Mode: Operate. Local app-mark replacement, not a workspace redesign. The user rejected the πDesk-like lavender π tile, kept the πDesigner name, and delegated the visual decision.

## Direction contract

THESIS: A mark about shaping an interface, not a second π monogram: a pen nib draws a Bézier curve on an artboard.

OWN-WORLD: Vermilion tile, ivory folded artboard, deep ink nib, precise vector anchors. Authored geometry, no lettering or purple π tile. Workspace colors and engine/model identities stay unchanged.

STORY: Read as a design tool before its name is read. The Dock mark explains the craft; a simplified nib remains clear in the menu bar.

FIRST VIEWPORT: One canonical vector drives Dock, favicon, workspace header and native activity panel. A monochrome alpha-template nib identifies the menu bar. Verify 16/24/32/64/128/256/512px on light and dark grounds.

FORM: Authored vector identity; seed: none (narrow asset redesign, direction delegated by user).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Finish review

Verdict: accepted. Reviewed inline because no sub-agent tool is exposed. The app mark has a distinct pen/artboard/curve silhouette rather than a πDesk-like letter tile. The first curve was revised because it read as a question mark; the final curve reads as a Bézier arch ending at the nib. AppKit renders preserve the coral/paper/ink silhouette at 16/32/128/512px; contact sheets cover 16/24/32/64/128px on light/dark grounds. The 18/32px menu symbol is genuinely black/transparent, so system template tinting remains correct.

The actual native activity-panel capture shows the new app icon. Frontend tests/typecheck/build, 20 native asset/state assertions, native lifecycle checks, browser/WebKit integration checks, icon detector and bundle signing pass. DESIGN.md and mac/ASSETS.md document the new mark and shipping raster provenance. Layout, model/provider marks, engine attribution and user data remain unchanged.
