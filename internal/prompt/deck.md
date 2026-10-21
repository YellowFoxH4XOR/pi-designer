## Slide deck discipline

The active skill builds a slide deck. HTML is the vehicle; think like a slide designer, not a web designer.

### Keep the deck framework

Copy the skill's seed verbatim first, then fill slides. The seed's navigation script, slide sizing, scroll/scale logic, keyboard handling, position persistence, and any print rules are load-bearing: do not rewrite, "tidy", or replace them, and never redefine the seed's slide visibility or sizing selectors. Add your own classes for slide content instead. If the brief needs something the framework cannot do, say so and ask before forking it.

### Plan the arc before writing slides

State the slide list with each slide's single idea and theme rhythm before any slide HTML, so the user can redirect cheaply. One idea per slide: two ideas are two slides; more slides cost nothing.

### Density and overflow (the most common cause of ugly decks)

1. Cover and title headlines: at most ~8 words and 3 lines. If it does not fit, the slide is the wrong shape — split it rather than shrinking the type and packing more in.
2. Body slides: at most 3 short paragraphs, lead text at most ~56ch, about 12 words per line.
3. Reserve a footer safe zone: flow content must never run into an absolutely positioned footer.
4. Every slide's content must fit inside its own slide box at every preview frame, including 390×844. A deck that swipes horizontally scrolls sideways by design, so `render_preview` reporting horizontal overflow for the slide strip is expected — check instead that no slide's content is clipped, overlapped, or spilling past its own slide.

### Charts

Bar lengths are computed, never eyeballed: give every bar its real value as a unitless `--v`, declare `--max` once per chart, and size with `calc(var(--v) / var(--max) * 100%)` (give vertical tracks an explicit height). Every data point gets a visible category label and a value label outside the bar. One baseline per chart. Nested diagrams may share a center but not text: at most one short KPI in the center, every other label in a legend or callout.

### Verifying a deck

`render_preview` shows the first slide. To check later slides, temporarily open the file at a slide anchor only if the seed supports one; otherwise review the remaining slides statically against the density rules above. Spend renders on the cover and the densest slide.
