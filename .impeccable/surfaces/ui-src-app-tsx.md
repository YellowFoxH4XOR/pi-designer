---
version: 1
slug: "ui-src-app-tsx"
primary_target: "ui/src/App.tsx"
related_targets: ["ui/src/styles.css","mac/Sources/main.swift"]
---

# Workspace redesign

Mode: Operate. Code-led implementation of the redesigned workspace with the user's latest correction: light themed. OpenDesign Cloud remains a historical workspace-structure reference; the light palette supersedes the intermediate dark treatment while preserving the redesigned geometry and behavior.

## Direction contract

THESIS: A familiar prompt-first design workspace. Replace the developer-console arrangement with Home, Projects, Design systems, Skills, Settings, and an artifact-led Studio.

OWN-WORLD: Cool gray rail, off-white canvas, white composer, charcoal system typography and restrained warm coral actions. Fine rules, compact curves, no glows or decorative motion. Retain the vermilion tile with ivory artboard, Bézier curve and ink nib; the native menu bar keeps its monochrome nib. A standalone Greek π identifies execution while maker/provider marks identify models. Native aqua and the shell background unify the window.

STORY: Choose a workflow and design system, write a brief, then enter a studio with the conversation left and the actual preview right. Browse real existing projects and the complete system catalog.

FIRST VIEWPORT: A 224px sidebar with New project, quiet 60px utility header, off-white canvas and compact footer. Home has a left-aligned 40px system heading (36px/32px on narrower windows), small editable-curve illustration, workflow tabs before the wide composer, three visual starter briefs, then recent projects. Studio removes the wide navigation rail and gives most space to the artifact.

FORM: The user's newest light-theme preference overrides the intermediate dark palette; preserve the redesigned layout and behavior. Code-led; seed:767e11340272f4dd. The authored visual gesture borrows the retained app mark's Bézier curve. No new images or dependencies.

SIGNATURE INTERACTION: The prompt's selected workflow and design system carry into a real project. Ordered, durable omp RPC events drive the current-action strip, expandable tool timeline, project statuses and native menu-bar/notch panel. Closing or minimizing the workspace keeps work running; explicit Quit stops it. Device frames scale the artifact without changing its CSS viewport.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints

Preserve Go and omp-only execution, existing data and MCP. No cloud, media-generation, sharing, or account buttons. All navigation and tools must perform their named action. Accessible keyboard operation, readable empty/error/loading states, reduced motion, usable Mac desktop window layouts down to 900×620, and a macOS icon in the built app. macOS desktop is the only app target; web/mobile/desktop frames are artifact-preview targets, not separate clients.

Keep the preview sandbox and artifact boundary intact. Visual starter briefs populate the real prompt, workflow and frame; they do not start generation automatically. Preserve editor save ownership, unsaved-change protection, durable execution state, and native background-task behavior.
