# πDesigner identities

## App and engine marks

- `ui/public/brand.svg`: original authored vector, a lavender Greek π on a charcoal rounded tile. Retains the owner's PiDesk-family palette; replaces the intermediate artboard/nib icon at the user's request.
- `ui/public/pi.svg`: original authored standalone lavender π for engine attribution. Replaces the intermediate coral/blue/gold pixel mark; no upstream agent files are modified.
- No generative image service was used. SVG geometry is the canonical source.
- `scripts/generate-icon.swift` renders the app SVG into ten standard icon sizes through AppKit; `scripts/build-mac.sh` compiles them into `πDesigner.app/Contents/Resources/AppIcon.icns`. The favicon, UI and Dock share this source.
- The native menu bar and notch use the Greek π with the Mac system font. They are operational status surfaces, not a system-notch replacement.

## Model/provider marks

- Local SVG assets under `ui/public/model-icons/`, vendored unchanged from the OpenDesign reference checkout; provenance and Apache-2.0 license are included alongside them.
- The shared registry resolves the model maker first (GPT → OpenAI, Opus/Sonnet/Haiku → Claude, DeepSeek → DeepSeek, Gemini → Gemini), then known routing providers, then a neutral authored code icon.
- Provider labels remain separate from model-maker identity. Marks identify the configured models/providers and imply no endorsement or affiliation.

## Compatibility

Display name and app-bundle filename are πDesigner. `dev.pidesigner.app`, executable names, CLI/MCP IDs, existing preferences and `~/Library/Application Support/PiDesigner` remain unchanged to preserve projects and permissions.
