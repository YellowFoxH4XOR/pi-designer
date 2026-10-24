# Web prototype review checklist

Review the prototype against the brief, then check its semantics and behavior.
P0 items are required before delivery; P1 items are recommended when relevant;
P2 items are optional refinements, not extra design constraints.

## P0 — required

- [ ] **The requested task is clear.** The prototype serves the user's actual
  audience and mode. Its primary message or task is easy to find, and the
  content follows the hierarchy established from the brief. It has not been
  turned into a generic marketing page or padded with irrelevant sections.
- [ ] **Explicit constraints are honored.** Named brand, platform,
  reference/clone fidelity, and requested content or interactions are reflected
  in the result. A refinement preserves existing working details that the
  request did not ask to change.
- [ ] **The document is semantic and navigable.** Use appropriate landmarks,
  headings, lists, tables, and form elements for their content. Heading levels
  and labels make relationships understandable; no section or control is
  present only for visual filler.
- [ ] **Section targets are stable.** Each top-level `<section>` has a
  meaningful, unique `data-od-id`. During refinement, keep existing values
  stable for unchanged sections.
- [ ] **Every control works.** Links have real destinations; buttons, forms,
  and other controls perform their stated action and expose their actual state.
  No `href="#"`, dead-end navigation, fake CTA, or decorative control that
  looks actionable without doing anything.
- [ ] **Keyboard and assistive-technology use works.** Interactive elements
  are keyboard reachable, focus is visible, controls have meaningful labels,
  images have appropriate alternative text (or empty alt when decorative),
  and meaning is not conveyed by color alone.
- [ ] **Responsive layout preserves usability.** Check the intended desktop
  size and narrow viewports. Content and controls remain readable and usable,
  order remains sensible, and no element causes unintended horizontal
  scrolling, clipping, or overlap.
- [ ] **Content and assets are truthful.** No invented metrics, testimonials,
  claims, records, or capabilities. Any named real-world visual referent uses
  the correct real asset, not a generated or look-alike substitute.
- [ ] **Image use preserves geometry.** Use project-local assets when imagery
  is needed; do not hotlink remote or expiring URLs. For content images,
  inspect intrinsic dimensions, include matching `width` and `height`, retain
  the full frame, and avoid distortion. Crop only an intentionally decorative
  image.
- [ ] **No implementation scaffolding ships as content.** Remove instructions,
  unfilled placeholders, temporary wireframes, broken links, and sample data
  that could be mistaken for real product content.

## P1 — recommended when relevant

- [ ] **The structure fits the task's information density.** Work surfaces
  foreground the work; content-led pages foreground their subject. Include
  navigation, chrome, status, secondary context, and calls to action only
  where the brief or task needs them.
- [ ] **Exploration compares structure, not just styling.** When the brief is
  open-ended or asks for concepts, compare genuinely different information
  structures or task flows under the same brand and content constraints. For
  a specified design or reference-fidelity task, do not introduce alternatives
  or novelty that conflict with the request.
- [ ] **Visual choices express the intended brand and mode.** Typography,
  palette, spacing, density, and imagery are consistent and support the
  content. They come from the named design system or deliberate choices for
  this brief, not an assumed seed style.
- [ ] **Relevant interaction states are covered.** Where the experience needs
  them, represent or implement empty, loading, success, validation, and error
  states with clear feedback. Do not fabricate states or data.
- [ ] **Motion respects preferences.** If transitions or animations are used,
  check the reduced-motion preference and avoid motion that obscures task
  progress or essential information.
- [ ] **The preview is self-contained.** Styles, behavior, and required assets
  are available to the project without a remote runtime or dependency.

## P2 — optional refinement

- [ ] Review at additional viewport sizes and with longer or shorter realistic
  content to catch layout pressure not visible in the initial example.
- [ ] Review dense content and interaction flows for scanning, hierarchy,
  feedback, and recovery from mistakes.
- [ ] Remove styling or implementation that no longer serves the chosen
  structure; keep the prototype maintainable and easy to refine.
