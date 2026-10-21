# App identity and omp model selection

## Changes

- App and agent identities are separate. `brand.svg` is πDesigner’s design-centric artboard/Bézier/pen-nib icon. The execution engine is identified by the official omp logo (`ui/public/omp.svg`, vendored unchanged from can1357/oh-my-pi `assets/icon.svg`, MIT) on its native dark tile.
- Native icons are generated directly from the canonical app SVG. The Dock explicitly loads the bundled icon, and the Mac bundle version is 0.1.1.
- Home, Studio, and Settings use a shared searchable model picker.
- The model list comes from the installed omp’s `get_available_models` RPC command, not a hard-coded provider list or a direct provider API.
- Selection saves an exact provider/model id; the CLI gets separate provider and model arguments. Before sending a prompt, the runner checks the model omp actually reports.
- Thinking options come from the selected model’s `get_available_thinking_levels`, with a supported fallback when switching models.
- Invalid model ids are rejected without overwriting saved preferences.
- Provider errors now affect the final run result; a successful retry can clear a transient error.
- MCP adds `list_models`, and omitted model/thinking fields use the app’s saved preferences.
- Model API responses contain only display/selection metadata. Provider headers, endpoints, and credentials are excluded.

## Verification

All tests used isolated temporary project data. The user’s omp credentials were not read by this app, and no global omp model default was changed.

- Installed omp discovery returned 70 configured models.
- Metadata-only test verified exact selection of `github-copilot/gpt-5.3-codex`, including its supported thinking levels, and checked that the omp default was unchanged.
- An opt-in real RPC request completed with `MODEL_SELECTION_OK`.
- The actual UI was used to choose that model, persist it across a reload, save a supported thinking level, create an isolated connection-check project, and complete a real request through the Go runner.
- The UI displayed omp’s verified model, not merely the requested selection.
- Eight focused model/identity checks and the 23 existing desktop interaction checks passed.
- Native WebKit captures verified the updated Home and Settings/model picker. No whole-desktop screenshots were required.
- Unit tests cover duplicate model ids across different providers, metadata allowlisting, unknown model rejection, provider-error events, and retry-result clearing.
- Go race tests, Go vet, TypeScript checks, Vite build, and strict local code-signature verification passed.
- The scoped design-source scan returned no findings.

## Scope and limits

This remains a macOS desktop-only local app. The checks establish the scoped selector/identity fix; they do not certify every model’s authentication, provider limits, cost, or output quality. Models are configured by the user’s installed omp and may become unavailable externally. The live inference test covered the named configured model, not all 70 models.

Visual review was performed in-session, not by an independent reviewer. Native captures and the browser checks are recorded under `.impeccable/review/` (gitignored). No paid image-generation service was used.
