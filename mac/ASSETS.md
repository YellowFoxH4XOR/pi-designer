# App and agent identities

## PiDesigner app icon

- Canonical source: `ui/public/brand.svg`.
- Original PiDesigner artwork: a charcoal tile, pale rotated artboard, lavender Bézier curve with square anchors, and a fountain-pen nib.
- The palette retains the owner's PiDesk-family inspiration; the design-tool imagery follows the user's request to distinguish PiDesigner.
- No generative image service was used.
- `scripts/generate-icon.swift` reads the same SVG through AppKit and renders ten standard icon sizes with a transparent macOS outer margin.
- `scripts/build-mac.sh` compiles that iconset to `PiDesigner.app/Contents/Resources/AppIcon.icns`.
- The interface, favicon, and Dock icon share the same vector source; there is no separate manually maintained drawing.

## Pi agent mark

- Asset: `ui/public/pi.svg`.
- Reproduced directly from the installed Pi 1.3.0 definition at `@earendil-works/pi-coding-agent/dist/modes/interactive/components/pi-logo.js`.
- The source defines a 4×4 pixel grid and three fixed colors: coral `#e48a7a`, blue `#4f8eb3`, and gold `#eab65d`. The SVG preserves that grid and palette, without a PiDesigner background tile.
- The Pi package README identifies the upstream project as Pi and links its logo at https://pi.dev/logo-auto.svg. The installed component was the source used here; no external asset service was required.
- Used only to identify the actual installed Pi engine in model controls, Settings, agent messages, and verified run metadata. It is not the PiDesigner application icon and does not imply endorsement.
