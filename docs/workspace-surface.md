# Workspace redesign

Mode: Operate. Code-led implementation, using the user's pinned reference rather than generating alternate visual worlds.

## Direction contract

THESIS: A familiar prompt-first design workspace. Replace the developer-console arrangement with Home, Projects, Design systems, Skills, Settings, and an artifact-led Studio.

OWN-WORLD: Light lavender-gray shell, off-white canvas and charcoal ink retain the existing workspace. The distinct app mark is a vermilion tile with an ivory artboard, Bézier curve and ink pen nib; a monochrome nib identifies the native menu bar. The standalone Greek π identifies execution, while maker/provider marks identify models. Neutral system UI type and the restrained Georgia Home heading remain unchanged. Soft corners, fine rules, no glow.

STORY: Choose a workflow and design system, write a brief, then enter a studio with the conversation left and the actual preview right. Browse real existing projects and the complete system catalog.

FIRST VIEWPORT: A 224px sidebar, quiet utility header, large white rounded workspace. Home has a centered heading, a 760px composer, workflow choices, and useful example briefs. Studio removes the wide navigation rail and gives most space to the artifact.

FORM: User-pinned reference: OpenDesign Cloud home and studio, with a PiDesk-family identity. seed: none (user-pinned direction; mode-operate guidance applied).

SIGNATURE INTERACTION: The prompt's selected workflow and design system carry directly into a real project, while device frames scale the artifact without changing its CSS viewport.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints

Preserve Go and π-only execution, existing data and MCP. No cloud, media-generation, sharing, or account buttons. All navigation and tools must perform their named action. Accessible keyboard operation, readable empty/error/loading states, reduced motion, usable Mac desktop window layouts down to 900×620, and a macOS icon in the built app. macOS desktop is the only app target; web/mobile/desktop frames are artifact-preview targets, not separate clients.
