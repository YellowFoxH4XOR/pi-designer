# Design references: update feature

The approved source set is XIAS, Material Design 3, shadcn/ui and Figma Community kits. UIZZE is deliberately excluded. The bundled Material and shadcn design systems are still the visual contract when selected; upstream text is reference material, never an override.

## Behavior

Settings → Design references lists original sources. XIAS design QA skill text and shadcn's documentation index can be checked (manually or every 24 hours **while the app remains open**), previewed as plain text and applied with an explicit action. Check alone never installs. Updated documents are stored in `~/Library/Application Support/PiDesigner/knowledge/` (or the configured data directory), not the signed `.app` bundle. Hashes guard pending and installed content; writes use a temporary file and atomic rename. A failed fetch leaves the installed document intact. The previous installed version is retained for a one-click Restore previous action. No code, npm packages, Figma kits, fonts or images are downloaded. Material and Figma are link-only because their site pages/community assets are not redistributable design guidance with a stable plain-text endpoint and uniform reuse terms.

The agent sees a list of **reviewed reference paths**, not their bodies in the system prompt; it is explicitly told to treat them as untrusted data and read at most one relevant document. New conversations pick up new references without a build. Existing π sessions may retain earlier system prompts: start a new conversation after installing new references. The updater never runs external code or edits the bundled skills or prompts.

Endpoints on the local daemon: `GET /api/knowledge`, `POST /api/knowledge/{id}/check`, `GET /api/knowledge/{id}/preview`, `POST /api/knowledge/{id}/apply`, `POST /api/knowledge/{id}/rollback`. Only fixed allowlisted URLs and IDs are permitted. Fetches require HTTPS, are limited to 512 KiB, and reject cross-host redirects; browser writes require same Origin. Files are private to the local account.

## Next increments

- Add an optional, curated signed manifest to update approved source locations independently of app releases, with a reviewable diff and rollback, rather than following arbitrary URLs from remote text.
- Add a stable, explicitly licensed Material guidance export if available; leave Figma Community as links until individual kit authors grant compatible reuse.
- Provide a side-by-side text diff before applying; basic per-source rollback is already available. Keep updates opt-in even when background checks detect them.
