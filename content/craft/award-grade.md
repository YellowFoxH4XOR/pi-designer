# Award-grade craft

The bar is an Awwwards Site of the Day, not "a clean page". Juries score
**Design 40% · Usability 30% · Creativity 20% · Content 10%**; Honorable
Mentions start at 6.5/10 and Site of the Day sits above roughly 7. A page that
is merely correct scores 5–6. Use this reference for every new showcase,
persuasion, portfolio, campaign or motion build. It is about execution
quality; the active design system and brand still own tokens and voice.

## 1. One idea, one signature moment

- Write the concept as one sentence that only this subject could own
  ("the menu is a tasting flight you scroll through", not "a modern coffee site").
- Design exactly one **signature moment** the visitor will remember and
  describe to someone else: a hero that transforms on scroll, type that
  assembles into the product, a cursor that becomes a lens, a pinned chapter
  that reveals the process. Spend most of the effort here.
- Everything else is quiet and precise so the signature moment can land.
  Three competing effects read as a template; one confident one reads as authored.

## 2. Typography is the image

- Scale contrast is the fastest route to drama: a display size 8–15× the body
  size (`clamp(4rem, 12vw, 14rem)`), tight leading (0.85–0.95), negative
  tracking (−0.02 to −0.05em) on large sizes, generous tracking on tiny
  uppercase labels (0.08–0.16em).
- Pick a display face with character (a high-contrast serif, a grotesk with
  quirks, a variable face whose axes you animate). Load it with `<link>` from
  Google Fonts or Fontshare and give a system fallback. Pair with a restrained
  text face. Never Inter, Roboto or Arial for display.
- Typeset like an editor: hang punctuation, set numerals deliberately, break
  headlines by meaning (`<br>` or `text-wrap: balance`), never by accident.
  Mix one italic or weight shift inside a headline for emphasis, not color.

## 3. Grid with intent, then break it once

- Build on a visible logic: a 12-column grid with wide gutters, or a strict
  editorial measure. Align everything to it; misalignment by a few pixels is
  what separates amateur from award work.
- Break the grid deliberately in one or two places (an image bleeding off the
  edge, a headline crossing columns, overlapping layers with clear z-order).
- Use whitespace as a material: sections of very different density create
  rhythm. Avoid uniform section heights and equal three-card rows.

## 4. Art direction and imagery

- Real, specific imagery beats illustration. Acquire it as the imagery rules
  say, then **art-direct it**: consistent crop ratios, a shared treatment
  (duotone, grain, desaturation, color grade via `mix-blend-mode` or a
  `filter`), masks and reveals instead of rectangles dropped on a page.
- Texture adds depth without color noise: a subtle SVG `feTurbulence` grain
  at 3–6% opacity, soft gradients only where light would be, real shadows
  (layered, low-opacity, offset by elevation).
- **Figurative subjects** (people, animals, vehicles, objects) must never be
  built from circles, rectangles or freehand SVG paths. Choose a
  representation you can execute professionally: (a) real photography or
  footage, treated and masked; (b) typography as the image; (c) abstraction
  and metaphor — light, water, particles, form — that conveys the idea without
  drawing a body; (d) a silhouette made by masking a real photograph. Name the
  choice in the design plan.

## 5. Motion is choreography

- **Load sequence:** the first second sets the tone. Stagger the hero in
  (mask-reveal headlines line by line, 60–90ms stagger, 0.8–1.2s, ease
  `cubic-bezier(0.16, 1, 0.3, 1)`), then let the page settle. No spinners for
  a page that loads instantly.
- **Scroll storytelling:** pin a chapter and scrub a transformation, reveal
  content as it enters (translate 24–60px plus opacity, never a bounce),
  parallax at most two depth layers at different speeds.
- **Micro-interactions:** magnetic or underline-draw links, image hover that
  scales the image inside a fixed mask (1.04–1.08), buttons whose label rolls.
  A custom cursor only when it does real work for the concept.
- Durations: 150–250ms for UI feedback, 600–1200ms for reveals, longer only
  for narrative scrubs. Ease out on entry, ease in on exit. Animate
  `transform`, `opacity`, `clip-path` and `filter` only.
- Tools: CSS for simple reveals; GSAP + ScrollTrigger for timelines, pins and
  scrubs; Lenis for smooth scrolling when scroll-driven work needs it. Download
  libraries into the project and reference them relatively so the design works
  offline. Every motion respects `prefers-reduced-motion` with a calm,
  complete static state.

## 6. Usability is 30% of the score

- The signature moment never hides navigation, the primary action or reading.
  Content is reachable without waiting for animation.
- Real navigation, visible focus, 44px targets, AA contrast, semantic
  headings. Mobile is redesigned, not shrunk: rethink the signature moment
  for touch and a 390px column.
- Performance is part of the craft: no layout shift, lazy-load below-the-fold
  media, keep the first view light.

## 7. Content earns its place

Specific copy in the brand's voice, real names and facts from the brief,
short enough to set large. Never invent claims a visitor could rely on —
metrics, testimonials, awards, clients. Do build the world the brief leaves
open: name the pieces, the features, the place, the hours, so the page feels
real; list those inventions in the summary. Credits and "prototype" notes stay
off the canvas.

## Self-score honestly

Before shipping, ask: would a jury of designers remember this tomorrow?
Could an unrelated product reuse this page by swapping nouns and colors? If
yes, the concept is not done — change the hierarchy, the content format or
the signature moment, not the decoration.
