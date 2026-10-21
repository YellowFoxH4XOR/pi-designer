# πDesigner identities

## App mark: the drawing instrument

- Canonical source: `ui/public/brand.svg`, an original authored vector. A deep-ink pen nib draws a Bézier curve, with an editable square anchor and control handle, on a folded ivory artboard. A vermilion tile gives it its own Dock silhouette and color identity.
- Palette: vermilion `#E45F3E`, warm ivory `#FFF5E7`, deep ink `#233137`, with restrained fold/metal tones. No fonts, letters, external images, image model, or proprietary source artwork are used.
- This replaces the intermediate lavender π/charcoal tile at the user's explicit request to stop resembling πDesk. The product name remains πDesigner; the icon is about designing rather than spelling the name.
- `scripts/generate-icon.swift` rasterizes this same SVG through AppKit into ten standard icon sizes. `scripts/build-mac.sh` packages them into `πDesigner.app/Contents/Resources/AppIcon.icns`. Every shipping icon raster derives from this source; no separate bitmap drawing is maintained.
- The web favicon, workspace header and Dock share the vector. The native activity header reads `NSApp.applicationIconImage`, so it cannot silently retain a different logo.

## Menu-bar symbol

- Canonical source: `ui/public/brand-symbol.svg`, an original simplified pen-nib silhouette related to the app mark.
- The nib hole and slit are transparent geometry, not white paint. This matters because macOS template rendering uses alpha, not the source color, to tint for light/dark menu bars.
- The build copies this to `Contents/Resources/BrandTemplate.svg`. The native controller sets its size to 18×18 and `isTemplate = true`; task counts remain separate text.

## Engine and model identities

- `ui/public/pi.svg` remains the standalone lavender Greek π for execution attribution. The app logo redesign does not modify the installed agent or model/provider branding.
- Model/provider SVGs under `ui/public/model-icons/` are vendored unchanged from the OpenDesign reference checkout; provenance and Apache-2.0 license are adjacent.
- The shared registry resolves model maker first (GPT → OpenAI, Opus/Sonnet/Haiku → Claude, DeepSeek → DeepSeek, Gemini → Gemini), then known routing providers, then a neutral code icon. Marks imply no endorsement or affiliation.

## Review and compatibility

- AppKit-rendered contact sheets check the mark at 16/24/32/64/128px on light/dark grounds and a large Dock rendering. Review files are generated under ignored `.impeccable/review/`.
- Native tests check colored silhouettes at 16/32/128/512px, transparent corners, and genuinely monochrome alpha-template rendering at 18/32px. No πDesk-like glyph/color fallback is accepted as the app mark.
- Display name, bundle identifier `dev.pidesigner.app`, executable names, CLI/MCP IDs, existing preferences and `~/Library/Application Support/PiDesigner` stay compatible. Workspace layout/state colors are not part of this logo-only redesign.
