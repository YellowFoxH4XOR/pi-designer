# πDesigner

<!-- impeccable:product-schema 1 -->

## Platform

web

The interface is implemented with web technology inside a native macOS AppKit/WKWebView shell. The product is a macOS desktop app only; the loopback server is internal infrastructure, not a supported browser product. There is no phone/tablet client.

## Users

The primary user is the owner of this local project, making web, mobile, and desktop prototypes and handing those files to a coding editor.

## Product Purpose

Turn a brief into editable interface files, preview them at device sizes, and refine them with the locally installed π coding agent.

## Operating Context

Go daemon, SQLite projects, Preact UI, installed π over JSONL RPC. The macOS app owns a bundled daemon. An MCP stdio server exposes the workspace to coding editors.

## Capabilities and Constraints

- Prototypes, interface motion, and website-clone workflows.
- Preserve the complete bundled design-system catalog, skills, existing project files, and MCP integration.
- Only the user's installed π is the generation engine. Credentials remain with π.
- No image generation, video generation, cloud accounts, billing, or fabricated collaboration features.
- Web, mobile, and desktop remain prototype preview viewports inside the Mac app. They are not separate πDesigner clients or claims of generated native applications.
- Finish and verify desktop Mac window sizes, native menus, keyboard operation, and the local app bundle. No mobile-app UI work is in scope.

## Brand Commitments

The user requested the workspace structure and usability of OpenDesign Cloud, rather than the original terminal-like three-pane shell. The app logo initially took inspiration from the PiDesk logo in `/Users/akki/Desktop/Projects/omp-ui/apps/desktop/public/app-icon.svg`. An intermediate design used an artboard/nib app icon and the upstream pixel agent mark. The user has now confirmed **πDesigner**: the Greek π replaces written “π” in the interface, with a π-led app icon retaining the family palette. A standalone π identifies execution, while maker/provider icons identify models. Keep app, engine, model and routing-provider identities distinct.

## Evidence on Hand

- Local reference screenshots under `open-design/docs/screenshots/product-tour/`.
- PiDesk SVG and application icon in the sibling `omp-ui` checkout.
- Real design-system data under `content/design-systems/`.
- Existing Go APIs for projects, files, conversations, runs, catalog, preview, and MCP.

## Product Principles

- Start with the design task, not implementation plumbing.
- Make the artifact the focus of the studio.
- Keep local execution and real run state visible through faithful, durable RPC events shared by Studio, project statuses and native Mac activity.
- Closing/minimizing the window continues tasks; menu-bar/notch activity remains available in the background. Explicit Quit stops execution.
- Notify for completion, attention and failure only with macOS permission. Keep notification previews private by default.
- Expose working features only; never fill the shell with inert product promises.

## Open Decisions

The light workspace and lavender PiDesk-family accent are this redesign's interpretation of the supplied references, not an assertion that the user requested a permanent light-only product.
