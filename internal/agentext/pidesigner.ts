/**
 * PiDesigner's Pi extension: the design-agent tools a design run needs.
 *
 * - todo_write: the live plan PiDesigner renders as a plan card.
 * - render_preview: renders a project HTML file at one of PiDesigner's preview
 *   frames with the bundled WebKit renderer and returns the screenshot to the
 *   model, so it can critique what it actually built.
 *
 * Configuration comes from the environment PiDesigner sets for the run:
 *   PIDESIGNER_RENDERER    absolute path of pidesigner-render (optional)
 *   PIDESIGNER_RENDER_DIR  directory where rendered PNGs are kept
 *
 * This file is written by the PiDesigner daemon; edits are overwritten.
 */
import { execFile } from "node:child_process";
import { mkdir, readFile, realpath, stat, unlink } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const VIEWPORTS = {
	web: { label: "Web", width: 1440, height: 900 },
	desktop: { label: "Desktop", width: 1280, height: 800 },
	mobile: { label: "Mobile", width: 390, height: 844 },
} as const;
type ViewportID = keyof typeof VIEWPORTS;

/** Renders allowed per run: primary + mobile, plus one confirming re-render each. */
const RENDER_BUDGET = 4;
/** Longest edge of the image sent to the model; the full PNG stays on disk. */
const MODEL_IMAGE_EDGE = 1568;

interface RenderReport {
	ok: boolean;
	error?: string;
	png?: string;
	width?: number;
	height?: number;
	scrollWidth?: number;
	scrollHeight?: number;
	overflowX?: boolean;
	errors?: string[];
	brokenImages?: string[];
	title?: string;
}

const TodoItem = Type.Object({
	content: Type.String({ description: "Short imperative step, e.g. 'Build the pricing section'." }),
	status: Type.Union([Type.Literal("pending"), Type.Literal("in_progress"), Type.Literal("completed")]),
});

function run(file: string, args: string[], signal?: AbortSignal): Promise<string> {
	const { promise, resolve, reject } = Promise.withResolvers<string>();
	execFile(file, args, { timeout: 45_000, signal, maxBuffer: 4 * 1024 * 1024 }, (error, stdout) => {
		// The renderer reports failures as JSON on stdout with a non-zero exit.
		if (stdout && stdout.trim()) resolve(stdout);
		else reject(error ?? new Error(`${path.basename(file)} produced no output`));
	});
	return promise;
}

function lastJSONLine(output: string): RenderReport {
	const lines = output.trim().split("\n").filter((l) => l.trim().startsWith("{"));
	if (!lines.length) throw new Error("renderer returned no report");
	return JSON.parse(lines[lines.length - 1]) as RenderReport;
}

async function modelImage(png: string, signal?: AbortSignal): Promise<{ data: string; mimeType: string }> {
	// sips ships with macOS; a JPEG bounded to the model's useful resolution keeps
	// the turn small. Fall back to the original PNG if conversion fails.
	const jpeg = png.replace(/\.png$/, ".model.jpg");
	try {
		await run("/usr/bin/sips", ["-s", "format", "jpeg", "-s", "formatOptions", "82", "-Z", String(MODEL_IMAGE_EDGE), png, "--out", jpeg], signal);
		const data = (await readFile(jpeg)).toString("base64");
		await unlink(jpeg).catch(() => {});
		return { data, mimeType: "image/jpeg" };
	} catch {
		return { data: (await readFile(png)).toString("base64"), mimeType: "image/png" };
	}
}

export default function pidesigner(pi: ExtensionAPI) {
	let renders = 0;

	pi.registerTool({
		name: "todo_write",
		label: "Plan",
		description:
			"Create or update the design plan shown to the user as a live plan card. Always send the complete list: " +
			"every item with its current status. Mark an item in_progress before starting it and completed as soon as it is done.",
		parameters: Type.Object({ todos: Type.Array(TodoItem, { description: "The complete plan, in order." }) }),
		async execute(_id, params) {
			const todos = params.todos;
			const done = todos.filter((t) => t.status === "completed").length;
			const current = todos.find((t) => t.status === "in_progress");
			const text = `Plan updated: ${done}/${todos.length} completed${current ? `; in progress: ${current.content}` : ""}.`;
			return { content: [{ type: "text", text }], details: { todos } };
		},
	});

	pi.registerTool({
		name: "render_preview",
		label: "Render preview",
		description:
			"Render a project HTML file in WebKit at one of PiDesigner's preview frames (web 1440×900, desktop 1280×800, mobile 390×844) " +
			"and return a screenshot plus measured facts: horizontal overflow, script errors, broken images, page height. " +
			`Use it to inspect your built design before delivering. Budget: ${RENDER_BUDGET} renders per run.`,
		parameters: Type.Object({
			path: Type.Optional(Type.String({ description: "Project-relative HTML file. Defaults to index.html." })),
			viewport: Type.Union([Type.Literal("web"), Type.Literal("desktop"), Type.Literal("mobile")]),
			fullPage: Type.Optional(Type.Boolean({ description: "Capture the whole page height (up to 6000px) instead of the first viewport." })),
		}),
		async execute(_id, params, signal, _onUpdate, ctx) {
			const renderer = process.env.PIDESIGNER_RENDERER;
			const renderDir = process.env.PIDESIGNER_RENDER_DIR;
			if (!renderer || !renderDir) {
				throw new Error("Rendering is unavailable in this session. Verify the layout statically instead.");
			}
			if (renders >= RENDER_BUDGET) {
				throw new Error(`Render budget used (${RENDER_BUDGET}/${RENDER_BUDGET}). Finish with static verification; do not try another way to render.`);
			}
			const viewport = VIEWPORTS[params.viewport as ViewportID];
			if (!viewport) throw new Error(`Unknown viewport ${params.viewport}. Use web, desktop, or mobile.`);

			const root = await realpath(ctx.cwd);
			const rel = (params.path || "index.html").replace(/^\/+/, "");
			let file: string;
			try {
				file = await realpath(path.resolve(root, rel));
			} catch {
				throw new Error(`${rel} does not exist in the project yet. Write it first.`);
			}
			if (file !== root && !file.startsWith(root + path.sep)) throw new Error(`${rel} is outside the project.`);
			if (!(await stat(file)).isFile()) throw new Error(`${rel} is not a file.`);

			renders++;
			await mkdir(renderDir, { recursive: true });
			const name = `${Date.now()}-${params.viewport}${params.fullPage ? "-full" : ""}.png`;
			const out = path.join(renderDir, name);
			const args = ["--url", pathToFileURL(file).href, "--width", String(viewport.width), "--height", String(viewport.height), "--out", out];
			if (params.fullPage) args.push("--full-page");
			const report = lastJSONLine(await run(renderer, args, signal));
			if (!report.ok) throw new Error(`Render failed: ${report.error ?? "unknown error"}`);

			const facts: string[] = [
				`Rendered ${path.relative(root, file)} at ${viewport.label} ${viewport.width}×${viewport.height}${params.fullPage ? " (full page)" : ""}.`,
				`Page height: ${report.scrollHeight ?? "?"}px.`,
				report.overflowX
					? `HORIZONTAL OVERFLOW: content is ${report.scrollWidth}px wide in a ${viewport.width}px viewport. Fix this before delivering.`
					: "No horizontal overflow.",
			];
			const errors = report.errors ?? [];
			const broken = report.brokenImages ?? [];
			facts.push(errors.length ? `Script errors (${errors.length}): ${errors.slice(0, 5).join(" | ")}` : "No script errors.");
			facts.push(broken.length ? `Broken images (${broken.length}): ${broken.slice(0, 5).join(", ")}` : "No broken images.");
			facts.push(`Renders used: ${renders}/${RENDER_BUDGET}.`);

			const content: Array<{ type: "text"; text: string } | { type: "image"; data: string; mimeType: string }> = [];
			const seesImages = (ctx.model?.input as readonly string[] | undefined)?.includes("image") ?? false;
			if (seesImages) {
				facts.push("Inspect the screenshot critically: hierarchy, alignment, spacing rhythm, contrast, clipped or overlapping text, empty regions.");
				content.push({ type: "text", text: facts.join("\n") });
				content.push({ type: "image", ...(await modelImage(out, signal)) });
			} else {
				facts.push("The current model cannot view images; rely on these measurements and the code.");
				content.push({ type: "text", text: facts.join("\n") });
			}

			return {
				content,
				details: {
					viewport: params.viewport,
					path: path.relative(root, file),
					file: name,
					fullPage: !!params.fullPage,
					overflowX: !!report.overflowX,
					scrollWidth: report.scrollWidth,
					scrollHeight: report.scrollHeight,
					errors: errors.length,
					brokenImages: broken.length,
				},
			};
		},
	});
}
