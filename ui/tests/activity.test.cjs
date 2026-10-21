const { test, after } = require("node:test");
const assert = require("node:assert/strict");
const { mkdtempSync, rmSync } = require("node:fs");
const { tmpdir } = require("node:os");
const { join } = require("node:path");
const { execFileSync } = require("node:child_process");
const dir = mkdtempSync(join(tmpdir(), "omp-activity-tests-"));
execFileSync(process.execPath, [
  "node_modules/typescript/bin/tsc",
  "src/activity.ts",
  "src/model-identity.ts",
  "--module",
  "commonjs",
  "--target",
  "es2022",
  "--skipLibCheck",
  "--outDir",
  dir,
]);
const { fromActivity, reduceActivity, statusLabel } = require(
  join(dir, "activity.js"),
);
const { modelIdentity } = require(join(dir, "model-identity.js"));
after(() => rmSync(dir, { recursive: true, force: true }));
const initial = () =>
  fromActivity({
    runId: "r",
    projectId: "p",
    projectName: "Test",
    status: "running",
    phase: "starting",
    label: "Starting omp",
    startedAt: 1,
    updatedAt: 1,
    seq: 0,
    text: "",
    tools: [],
    attention: false,
  });
let seq = 0;
const event = (type, extra = {}) => ({
  type,
  runId: "r",
  seq: ++seq,
  at: 1000 + seq,
  ...extra,
});
test("authoritative message replaces deltas; replay is idempotent", () => {
  let a = initial();
  a = reduceActivity(a, event("message_start"));
  const delta = event("text", { text: "draft" });
  a = reduceActivity(a, delta);
  a = reduceActivity(a, delta);
  assert.equal(a.text, "draft");
  a = reduceActivity(a, event("message_end", { text: "final" }));
  a = reduceActivity(a, event("turn"));
  a = reduceActivity(a, event("message_start"));
  a = reduceActivity(a, event("text", { text: "next" }));
  a = reduceActivity(a, event("message_end", { text: "next final" }));
  assert.equal(a.text, "final\n\nnext final");
});
test("interleaved content blocks reconstruct in contentIndex order", () => {
  let a = reduceActivity(initial(), event("message_start"));
  a = reduceActivity(a, event("text", { contentIndex: 1, text: "second" }));
  a = reduceActivity(a, event("text", { contentIndex: 0, text: "first " }));
  a = reduceActivity(a, event("text", { contentIndex: 1, text: " block" }));
  assert.equal(a.text, "first second block");
  a = reduceActivity(a, event("message_end", { text: "final" }));
  assert.equal(a.text, "final");
});

test("parallel tools correlate by id; partial output replaces earlier snapshot", () => {
  let a = initial();
  a = reduceActivity(
    a,
    event("tool", {
      toolCallId: "a",
      tool: "bash",
      args: { command: "echo test" },
    }),
  );
  a = reduceActivity(
    a,
    event("tool", {
      toolCallId: "b",
      tool: "edit",
      args: { path: "/project/styles.css" },
    }),
  );
  a = reduceActivity(
    a,
    event("tool_update", { toolCallId: "a", tool: "bash", output: "part" }),
  );
  a = reduceActivity(
    a,
    event("tool_update", { toolCallId: "a", tool: "bash", output: "part two" }),
  );
  assert.equal(a.tools[0].output, "part two");
  a = reduceActivity(
    a,
    event("tool_end", {
      toolCallId: "b",
      tool: "edit",
      output: "saved",
      durationMs: 20,
    }),
  );
  assert.equal(a.label, "Running command");
  assert.equal(a.tools[1].status, "succeeded");
  a = reduceActivity(
    a,
    event("tool_end", {
      toolCallId: "a",
      tool: "bash",
      output: "failed",
      isError: true,
    }),
  );
  assert.equal(a.tools[0].status, "failed");
  assert.equal(a.label, "Waiting for model response");
});
test("attention is not success; cancellation interrupts tools", () => {
  let a = reduceActivity(
    initial(),
    event("attention", { data: { id: "q", method: "confirm" } }),
  );
  a = reduceActivity(a, event("done"));
  assert.equal(statusLabel(a), "Needs your answer");
  a = reduceActivity(a, event("canceled"));
  assert.equal(a.attention, false);
  assert.equal(a.request, undefined);
  let b = reduceActivity(
    initial(),
    event("tool", { tool: "bash", toolCallId: "x" }),
  );
  b = reduceActivity(b, event("run_failed", { text: "lost process" }));
  assert.equal(b.tools[0].status, "interrupted");
  assert.equal(b.error, "lost process");
});
test("model identity follows model maker rather than a routing provider", () => {
  for (const [id, provider, expected] of [
    ["gpt-5", "openai-codex", "openai"],
    ["o3-mini", "github-copilot", "openai"],
    ["claude-opus-4", "openrouter", "claude"],
    ["sonnet-4", "opencode-go", "claude"],
    ["deepseek-v3", "opencode-go", "deepseek"],
    ["gemini-2.5-pro", "google", "google-gemini"],
    ["custom-model", "opencode-go", "opencode"],
    ["custom-model", "my-server", null],
  ])
    assert.equal(modelIdentity(id, provider), expected, id);
});
