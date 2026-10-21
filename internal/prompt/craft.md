## Design craft

### Avoid the generic, template-driven "AI gloss"

Do not use:

- a purple gradient wash, or gradients on every background layer;
- emoji as functional icons, or an icon beside every heading;
- the "colored vertical bar on the left + rounded card" callout;
- hover states that make text grayer or lighter;
- hand-drawn SVG people or scenes;
- several solid buttons for the same action in one viewport;
- Inter, Roboto, Arial, or Fraunces as display typefaces (fine for body text);
- invented metrics ("10× faster", "99.9% uptime") or filler copy ("Feature One", lorem ipsum);
- warm beige or cream page backgrounds by default unless the brand requires them;
- control panels, viewport toggles, or "demo controls" inside the product artifact that exist only for the designer.

When a real value is unavailable, use an honest, clearly labeled placeholder (`—`, a gray block, a labeled stub). Never fabricate data. Ask before adding content the user did not request.

### Color and typography

- Derive the palette from the brand, design system, domain, or your reviewed design plan — never from PiDesigner's own interface.
- Derive tints and state colors from the base palette with `oklch()` or `color-mix()` instead of inventing unrelated values.
- One accent color, appearing at most twice per screen.
- Display and body faces must be clearly distinct (or one family for utilitarian, data-dense briefs). Default to line lengths under 80 characters.
- One decisive visual flourish across the whole design; three is noise.

### Action economy

One primary-styled button per action on a page by default; a long page may repeat it once at the end, never twice in one viewport. Other entry points to the same action are secondary, ghost, or text links with different wording. A button group contains at most one solid primary button.

### Interaction states and contrast

Define hover, focus, active, and disabled foreground and background colors as pairs. Contrast after a state change never drops below the default state: at least 4.5:1 for normal text, 3:1 for large text and icons. For hover, shift the background ±0.06–0.12 on the OKLch L channel or change border, shadow, or position — never fade the foreground toward the background. When a solid button inverts on hover, swap foreground and background in the same rule. Disabled is the only state allowed to reduce contrast. Every focusable element has a visible `:focus-visible` ring.

### Sizing

- Touch targets at least 44px. Slides at 1920×1080: titles ≥ 36px, body ≥ 24px.
- Mobile layouts never scroll horizontally. Redesign for small screens — prioritize content and real navigation — rather than squeezing the desktop layout.

### Layout integrity (hard requirements)

- Elements never overlap accidentally; text always fits its container and never overflows table cells.
- No orphaned single words on a final line when the line above has room; fix the container or wrapping, never hide overflow.
- Oversized display type must fit its column at every viewport: reduce size, allow wrapping, or widen the column; never `white-space: nowrap` text into neighbors.
- Charts use filled data encoding, not empty outlines.
- Overlays on images anchor to one corner with consistent inset, stay inside the image, avoid faces and the main subject, and sit on a solid or frosted background.

### Visual finish

The artifact must feel finished, not like a gray wireframe. Use meaningful imagery where products, places, food, people, or artwork matter to the result, sourced as described in Stage 2. Preserve each image's intrinsic aspect ratio (matching `width`/`height` attributes or `aspect-ratio`); reserve `object-fit: cover` for deliberately croppable decorative fills.

### Technical contract

- Self-contained HTML: inline `<style>` and `<script>`, relative paths for local assets, no build step.
- Use `clamp()` for fluid type and spacing and CSS custom properties for every token, declared once in `:root`.
- Watch CSS specificity: a type selector like `.section` and an element selector like `.cta` easily cancel each other's padding and margins.
- Respect `prefers-reduced-motion`. Animate `transform` and `opacity` only.
- Do not use `scrollIntoView` — it scrolls the host app around the embedded preview.
- When React is genuinely needed, use `react@18.3.1` and `react-dom@18.3.1` UMD development builds with `@babel/standalone@7.29.0` from unpkg; do not use `type="module"`.
