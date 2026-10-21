# πDesigner activity release verification

## Verdict

Ready for PR review as a local Mac build. The activity pipeline, background lifecycle and native control surfaces are implemented and exercised. This is not a notarized public release or a claim that notification permission/banner behavior has been approved on the owner's Mac.

## Passed

- `go test -race ./...` and `go vet ./...`.
- Frontend typecheck, production build, and five reducer/model-identity tests.
- Metadata-only discovery against the installed π: 70 configured models; exact provider/model selection and supported thinking levels verified without inference or changing the default.
- Nine browser integration checks against an isolated real daemon/SQLite/SSE, using deterministic fixture RPC: branding/icons, live parallel tools/output, offline/reload recovery, diagnostics, external-run discovery, supported extension questions, question-only completion, provider errors/cancellation, superseded-attention recovery, and minimum-window layout. No browser runtime errors.
- Eight native integration checks in the actual built app with isolated data/preferences: main-frame native preferences bridge, close continuation, notch expansion, minimize continuation, completion while minimized, menu-bar access after completion, rapid-outcome notification-ledger recovery, and attention independent of foreground WebKit.
- Ten native DTO/preferences assertions, including distinct attention/completion/failure/cancellation outcomes and private/quiet notification defaults.
- Ad-hoc bundle signing and `codesign --verify --deep --strict build/πDesigner.app`.
- Mechanical UI detector over `ui/src`: exit 0, no findings.
- Visual inspection of the working Studio, model picker, 900×620 Studio and native notch panel. Tool status uses explicit words rather than color alone; native current action is exposed to accessibility.
- No review daemon or fixture agent left running. Existing user data and the previously running app were not migrated, replaced or stopped by the tests.

Browser and native evidence is generated under ignored `.impeccable/review/` by the repeatable scripts. Raster review captures are test artifacts, not shipping assets. Shipping icon rasters are generated from the canonical authored SVG; see `mac/ASSETS.md` for provenance.

## Manual checks / known limits

- Enable notifications in Settings, then check the macOS authorization prompt, real banner delivery and notification-click restoration. Automation deliberately did not grant permission. Permission denial does not disable in-app/menu-bar/notch activity.
- Check sleep/wake, Spaces/full-screen and physical multi-monitor/notch arrangements on the owner's setup. Positioning uses the selected screen's visible frame; only the available local arrangement was exercised.
- Active execution and user input are distinct. Legacy runs retain their conversation/plan/render data, not retroactively reconstructed tool histories. Fire-and-forget extension records are diagnostic data; arbitrary terminal custom UIs are not emulated.
- The existing Vite 5/esbuild development dependencies still report two npm audit advisories (one high, one moderate). No unrelated major build-tool upgrade was included. The shipping Mac app uses built static assets, not Vite's development server.
- The built app is `build/πDesigner.app`; the old bundle filename is not automatically deleted or installed over another running app. Bundle identifier, executable names, MCP configuration and the existing data directory remain stable.
