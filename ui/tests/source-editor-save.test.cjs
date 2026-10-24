const { test, after } = require("node:test");
const assert = require("node:assert/strict");
const { mkdtempSync, rmSync } = require("node:fs");
const { tmpdir } = require("node:os");
const { join } = require("node:path");
const { execFileSync } = require("node:child_process");

const dir = mkdtempSync(join(tmpdir(), "pi-source-editor-tests-"));
execFileSync(process.execPath, [
  "node_modules/typescript/bin/tsc",
  "src/source-editor-save.ts",
  "--module",
  "commonjs",
  "--target",
  "es2022",
  "--skipLibCheck",
  "--outDir",
  dir,
]);
const { completeSourceSave } = require(join(dir, "source-editor-save.js"));
after(() => rmSync(dir, { recursive: true, force: true }));

test("a stale save cannot mark another file's matching edit clean", () => {
  const saved = {
    projectId: "project",
    filePath: "a.ts",
    generation: 1,
    content: "same text",
  };
  const current = {
    projectId: "project",
    filePath: "b.ts",
    generation: 2,
    content: "same text",
    original: "loaded B",
  };

  const completed = completeSourceSave(current, saved);

  assert.equal(completed, current);
  assert.equal(completed.content, saved.content);
  assert.notEqual(completed.content, completed.original);
  assert.equal(completed.original, "loaded B");
});

test("an older editor generation cannot complete into a later visit to the same file", () => {
  const saved = {
    projectId: "project",
    filePath: "a.ts",
    generation: 3,
    content: "saved snapshot",
  };
  const current = {
    projectId: "project",
    filePath: "a.ts",
    generation: 5,
    content: "new visit edit",
    original: "new visit baseline",
  };

  const completed = completeSourceSave(current, saved);

  assert.equal(completed, current);
  assert.equal(completed.content, "new visit edit");
  assert.equal(completed.original, "new visit baseline");
  assert.notEqual(completed.content, completed.original);
});

test("saving a snapshot keeps edits typed while the save was pending dirty", () => {
  const saved = {
    projectId: "project",
    filePath: "a.ts",
    generation: 6,
    content: "saved snapshot",
  };
  const current = {
    projectId: "project",
    filePath: "a.ts",
    generation: 6,
    content: "newer edit typed during save",
    original: "old baseline",
  };

  const completed = completeSourceSave(current, saved);

  assert.equal(completed.content, "newer edit typed during save");
  assert.equal(completed.original, "saved snapshot");
  assert.notEqual(completed.content, completed.original);
});

