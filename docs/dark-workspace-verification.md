# Dark workspace verification

Verified 2026-10-22 for the user-selected dark creative workspace.

## Presentation changes

- Graphite navigation, canvas, composer, dialogs, libraries, settings and execution states share semantic dark tokens. Coral identifies primary actions, focus and selected workflows.
- Home uses a left-aligned heading, a small Bézier illustration, workflow tabs above the composer, three visual starter briefs, and recent projects below.
- New project is directly accessible in the sidebar. The native window uses Dark Aqua and the same shell background.
- Model marks retain their source artwork on a light tile. Prototype frames and component fixtures retain their own backgrounds and CSS viewports.

## Automated checks

- `go test -race ./...` and `go vet ./...`: pass. Integration checks were run outside the restricted sandbox to permit temporary loopback listeners.
- UI: 8 tests, TypeScript checking and Vite production build pass.
- Script safety suite: 19 tests pass.
- `scripts/review-ui.cjs`: pass at 1440×960, 1280×800 and 900×620. Covers clone URL validation, catalog selection, Escape/focus restoration, keyboard tabs, source saves, unsaved navigation protection, repeated CSS hot-swap, rename and conversation creation. Artifact CSS widths remain 1440 and 390.
- `scripts/review-activity.cjs`: pass. Covers active/attention/failure/cancellation states, reconnect/reload, durable tool history, pending saves and long-content containment. Its native WebKit scroll checks also pass.
- Additional browser smoke: each starter sets the expected prompt, workflow and viewport and focuses the textarea; Projects, Design systems, Skills and the model picker render; minimum-window internal scrolling works. No browser runtime errors.
- Impeccable source detector: no findings in App, Home, Composer and the three shared stylesheets. The later finish-review cleanup removed the question-form eyebrow and its unused style, retaining the form's own title and a visible fallback heading.
- Shared text contrast: ink/canvas 14.65:1, muted/surface 6.47:1, accent/selected background 6.20:1, primary action 7.92:1. Success, warning and error text on their semantic backgrounds exceed 6.6:1. These are token-pair checks, not a claim of an exhaustive accessibility audit.
- Mac bundle builds and passes `codesign --verify --deep --strict`. Native model/icon tests pass. Native app snapshots were inspected at 1440×900 and 900×620 using isolated data and a fixture agent.
- Independent finish review found one material cleanup in the question-form heading. The refreshed capture shows it resolved with the choices and actions intact; the verdict pass returned `ship` for that scored fix. UI checks and the activity review passed again after the cleanup, and the bundled UI was refreshed and re-signed.

## Visual evidence

Local, generated review artifacts live under `.impeccable/review/`:

- `dark-home.png`, `home-1280.png`, `home-900.png`, `dark-home-900-scrolled.png`
- `dark-projects.png`, `dark-systems.png`, `dark-skills.png`, `system-detail.png`
- `dark-model-picker.png`, `settings-desktop.png`, `dark-settings-900.png`
- `studio-desktop.png`, `studio-900.png`, `studio-source.png`, `studio-mobile-frame.png`
- `activity-working.png`, `activity-attention.png`, `activity-finished.png`
- `dark-native-home.png`, `dark-native-home-900.png`

Chrome reviews used isolated daemon data at `/tmp/pidesigner-ui-redesign`; native captures used `/tmp/pidesigner-native-dark-review`. No live model inference, user projects, credentials or model configuration were needed. This pass does not verify notification permission/banner delivery or a new menu-bar/notch lifecycle run.
