# Optional structural examples

These examples illustrate different ways to organize content; they are not a
catalogue of required sections, paste-ready templates, or a closed set of
acceptable layouts. Use an example only when it fits the user's task, adapt it
to the actual content, or design a more suitable structure from scratch.
There is no default page order, section count, hero, navigation, or footer.

The snippets intentionally contain no visual styling, color, typography,
decorative labels, filler copy, or inactive controls. Define the project's
visual treatment separately and implement real behavior for any controls you
add. Keep semantic structure and meaningful heading relationships as you
adapt them.

## Work area with contextual information

Useful when people need to work on one primary object while consulting
secondary context. The relative emphasis and placement depend on the task;
the supporting region may be omitted or moved below the main work on narrow
screens.

```html
<div class="work-area">
  <aside aria-labelledby="context-heading">
    <h2 id="context-heading">[Relevant context]</h2>
    [Supporting information that helps with the task]
  </aside>
  <section aria-labelledby="work-heading">
    <h1 id="work-heading">[Current task or object]</h1>
    [The primary work surface and its real content]
  </section>
</div>
```

## Focused record or decision

Useful when a single item needs review, with its status and supporting facts
close to the decision it informs. Add actions only when they have real,
implemented effects.

```html
<article aria-labelledby="record-heading">
  <header>
    <h1 id="record-heading">[Record or decision]</h1>
    <p>[Current state and concise context]</p>
  </header>
  <dl>
    <div>
      <dt>[Relevant property]</dt>
      <dd>[Its actual value]</dd>
    </div>
  </dl>
  <section aria-labelledby="evidence-heading">
    <h2 id="evidence-heading">[Supporting material]</h2>
    [Evidence, history, or details needed to understand the record]
  </section>
</article>
```

## Content-led reading sequence

Useful when the user's goal is to follow an explanation, story, or set of
related topics. Let the subject determine whether content is grouped into
sections, linked as an index, or presented in another order.

```html
<article aria-labelledby="page-heading">
  <header>
    <h1 id="page-heading">[Subject]</h1>
    <p>[Context needed before reading]</p>
  </header>
  <section aria-labelledby="topic-heading">
    <h2 id="topic-heading">[First relevant topic]</h2>
    [The information that belongs to this topic]
  </section>
</article>
```

## Data comparison

Useful when the task depends on comparing values across the same attributes.
Choose a table only when row-and-column relationships aid comprehension; use
another representation when the data or task calls for it.

```html
<table>
  <caption>[What is being compared and why]</caption>
  <thead>
    <tr>
      <th scope="col">[Attribute]</th>
      <th scope="col">[Item A]</th>
      <th scope="col">[Item B]</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <th scope="row">[Relevant comparison]</th>
      <td>[Verified value]</td>
      <td>[Verified value]</td>
    </tr>
  </tbody>
</table>
```

## Adapting any structure

- Start from the primary task or message and the hierarchy established in the
  brief; do not select a familiar section sequence by habit.
- Preserve the requested brand and any explicit reference or clone fidelity.
  Distinct concepts, when useful, should differ in information structure or
  task flow while keeping the same brand and content constraints.
- Add navigation, page chrome, disclosures, forms, dialogs, and other
  interactions only when the experience needs them. Give each control a
  meaningful label and a working outcome.
- Create domain-specific sections when they express the content more clearly
  than these examples. Do not add generic content merely to complete a
  sequence.
- Keep the hierarchy and relationships understandable at narrow widths and
  with assistive technology; details belong in the prototype's implementation.
