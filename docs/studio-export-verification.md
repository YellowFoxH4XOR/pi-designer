# Studio refinement and export — 0.4.0

- `go test -race ./...` and `go vet ./...` passed. App-level tests opened both ZIPs after a manual edit and compared the entry bytes. A separate fresh project exported a nested binary before any run; hidden files and dependencies were excluded, and an unsupported link returned an error instead of a partial ZIP.
- UI tests, TypeScript checking, Vite production build, script tests and Python release-metadata tests passed on Linux.
- The isolated fixture daemon and Chromium activity review passed all 15 checks. It dismissed the discard dialog for canvas close, version selection and a conversation version card, verified the same draft remained, then accepted discard and switched back to current files. Canvas close during an in-flight save kept the editor mounted.
- A real pointer click on a button in the scaled, sandboxed preview attached its location to an existing draft. Picking did not start a run; explicit Send included the instruction, project-relative file and structural selector. No browser runtime errors were reported.
- These checks used deterministic fixture RPC. They do not exercise a live model or the interactive AppKit Save sheet.
