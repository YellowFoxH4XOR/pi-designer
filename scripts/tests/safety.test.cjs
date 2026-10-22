const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync, execFile } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');

const initClone = path.resolve(__dirname, '../../content/skills/web-clone/scripts/init-clone.mjs');
const portScript = path.resolve(__dirname, '../port-content.sh');
const e2eScript = path.resolve(__dirname, '../e2e.sh');

function temporaryDirectory(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

function write(root, relative, content = 'fixture\n') {
  const file = path.join(root, relative);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
  return file;
}

test('in-place clone initialization preserves notes and adds ignore rules once', () => {
  const root = temporaryDirectory('pidesigner-clone-init-');
  try {
    const notes = '# User notes\nDo not replace this.\n';
    const ignore = 'vendor/\n*.cache';
    write(root, 'NOTES.md', notes);
    write(root, '.gitignore', ignore);
    const initialized = spawnSync('git', ['init', '--quiet', root], { encoding: 'utf8' });
    assert.equal(initialized.status, 0, initialized.stderr);
    let firstIgnore;

    for (let run = 0; run < 2; run += 1) {
      const result = spawnSync(process.execPath, [initClone, 'example-site', '--root', root, '--in-place'], { encoding: 'utf8' });
      assert.equal(result.status, 0, result.stderr);
      const currentIgnore = fs.readFileSync(path.join(root, '.gitignore'), 'utf8');
      if (run === 0) firstIgnore = currentIgnore;
      else assert.equal(currentIgnore, firstIgnore, 'rerunning initialization must not add duplicate rules');
    }

    assert.equal(fs.readFileSync(path.join(root, 'NOTES.md'), 'utf8'), notes);
    const finalIgnore = fs.readFileSync(path.join(root, '.gitignore'), 'utf8');
    assert.ok(finalIgnore.startsWith(ignore));
    for (const target of ['node_modules/package/index.js', '.DS_Store', 'vendor/package.js', 'build.cache']) {
      const ignored = spawnSync('git', ['-C', root, 'check-ignore', '--quiet', '--no-index', '--', target], { encoding: 'utf8' });
      assert.equal(ignored.status, 0, `${target} was not ignored: ${ignored.stderr}`);
    }
    assert.ok(fs.statSync(path.join(root, 'RECON/screenshots')).isDirectory());
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('in-place clone initialization recognizes existing anchored ignore rules', () => {
  const root = temporaryDirectory('pidesigner-clone-ignore-');
  try {
    const ignore = '# keep comments\n/custom-rule\n/node_modules\n/.DS_Store\n';
    write(root, 'NOTES.md', 'owned notes\n');
    write(root, '.gitignore', ignore);
    const result = spawnSync(process.execPath, [initClone, 'example-site', '--root', root, '--in-place'], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(fs.readFileSync(path.join(root, '.gitignore'), 'utf8'), ignore);
    assert.equal(fs.readFileSync(path.join(root, 'NOTES.md'), 'utf8'), 'owned notes\n');
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

const importedSkills = ['web-clone', 'motion-design', 'emilkowalski-motion', 'web-design-guidelines'];
const importedTemplates = [
  'web-prototype', 'web-prototype-taste-brutalist', 'web-prototype-taste-editorial',
  'web-prototype-taste-soft', 'motion-frames', 'tweaks', 'critique', 'simple-deck',
];

function makePortRepo() {
  const root = temporaryDirectory('pidesigner-port-content-');
  const scriptCopy = path.join(root, 'scripts/port-content.sh');
  fs.mkdirSync(path.dirname(scriptCopy), { recursive: true });
  fs.copyFileSync(portScript, scriptCopy);
  fs.chmodSync(scriptCopy, 0o755);

  write(root, 'open-design/design-systems/sample/manifest.json', '{}');
  write(root, 'open-design/design-systems/sample/DESIGN.md');
  write(root, 'open-design/design-systems/sample/DESIGN-fr.md');
  write(root, 'open-design/design-systems/sample/tokens.css');
  write(root, 'open-design/design-systems/sample/source/raw.json');
  write(root, 'open-design/design-systems/sample/assets/icon.svg');
  write(root, 'open-design/design-systems/_schema/schema.json', '{}');
  for (const skill of importedSkills) {
    write(root, `open-design/skills/${skill}/SKILL.md`, `upstream ${skill}\n`);
  }
  write(root, 'open-design/skills/web-clone/scripts/init-clone.mjs', 'upstream initializer\n');
  for (const template of importedTemplates) {
    write(root, `open-design/design-templates/${template}/SKILL.md`, `upstream ${template}\n`);
  }
  write(root, 'open-design/craft/law.md');
  write(root, 'open-design/prompt-templates/example.json', '{}');
  return root;
}

function runPort(root, env = process.env) {
  return spawnSync('bash', [path.join(root, 'scripts/port-content.sh')], { cwd: root, env, encoding: 'utf8' });
}

test('content import stages the update and preserves local skills and web-clone adaptations', () => {
  const root = makePortRepo();
  try {
    write(root, 'content/unmanaged.json', 'preserve me\n');
    write(root, 'content/design-systems/obsolete/old.json');
    write(root, 'content/skills/frontend-design/SKILL.md', 'separately vendored\n');
    write(root, 'content/skills/local-extension/SKILL.md', 'local skill\n');
    write(root, 'content/skills/web-clone/SKILL.md', 'πDesigner local instructions\n');
    write(root, 'content/skills/web-clone/scripts/init-clone.mjs', 'πDesigner local initializer\n');
    write(root, 'content/design-templates/obsolete/SKILL.md');
    write(root, 'content/craft/obsolete.md');
    write(root, 'content/prompt-templates/obsolete.json');

    const result = runPort(root);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(fs.readFileSync(path.join(root, 'content/unmanaged.json'), 'utf8'), 'preserve me\n');
    assert.equal(fs.readFileSync(path.join(root, 'content/skills/frontend-design/SKILL.md'), 'utf8'), 'separately vendored\n');
    assert.equal(fs.readFileSync(path.join(root, 'content/skills/local-extension/SKILL.md'), 'utf8'), 'local skill\n');
    assert.equal(fs.readFileSync(path.join(root, 'content/skills/web-clone/SKILL.md'), 'utf8'), 'πDesigner local instructions\n');
    assert.equal(fs.readFileSync(path.join(root, 'content/skills/web-clone/scripts/init-clone.mjs'), 'utf8'), 'πDesigner local initializer\n');
    assert.ok(fs.existsSync(path.join(root, 'content/design-systems/sample/manifest.json')));
    assert.ok(!fs.existsSync(path.join(root, 'content/design-systems/obsolete')));
    assert.ok(!fs.existsSync(path.join(root, 'content/design-systems/sample/DESIGN-fr.md')));
    assert.ok(fs.existsSync(path.join(root, 'content/design-systems/sample/assets/icon.svg')));
    assert.ok(fs.existsSync(path.join(root, 'content/design-templates/simple-deck/SKILL.md')));
    assert.ok(!fs.existsSync(path.join(root, 'content/design-templates/obsolete')));
    assert.ok(fs.existsSync(path.join(root, 'content/craft/law.md')));
    assert.ok(fs.existsSync(path.join(root, 'content/prompt-templates/example.json')));
    assert.deepEqual(fs.readdirSync(root).filter(name => name.startsWith('.port-content.')), []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('content import leaves the old catalog intact when required sources are missing', () => {
  const root = temporaryDirectory('pidesigner-port-missing-');
  try {
    write(root, 'scripts/port-content.sh');
    fs.copyFileSync(portScript, path.join(root, 'scripts/port-content.sh'));
    write(root, 'content/owned.json', 'old catalog\n');
    fs.mkdirSync(path.join(root, 'open-design'), { recursive: true });
    const result = runPort(root);
    assert.notEqual(result.status, 0);
    assert.equal(fs.readFileSync(path.join(root, 'content/owned.json'), 'utf8'), 'old catalog\n');
    assert.deepEqual(fs.readdirSync(root).filter(name => name.startsWith('.port-content.')), []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('content import leaves the old catalog intact after a staging copy failure', () => {
  const root = makePortRepo();
  try {
    write(root, 'content/owned.json', 'old catalog\n');
    const shim = path.join(root, 'shim');
    const cp = write(root, 'shim/cp', '#!/bin/sh\nexit 73\n');
    fs.chmodSync(cp, 0o755);
    const env = { ...process.env, PATH: `${shim}:${process.env.PATH}` };
    const result = runPort(root, env);
    assert.notEqual(result.status, 0);
    assert.equal(fs.readFileSync(path.join(root, 'content/owned.json'), 'utf8'), 'old catalog\n');
    assert.deepEqual(fs.readdirSync(root).filter(name => name.startsWith('.port-content.')), []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

function startApi(scenario = {}) {
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, 'http://127.0.0.1').pathname;
    const send = (status, body, type = 'application/json') => {
      res.writeHead(status, { 'Content-Type': type });
      res.end(typeof body === 'string' ? body : JSON.stringify(body));
    };
    if (req.method === 'POST' && pathname === '/api/projects') {
      send(scenario.createStatus || 201, scenario.createBody ?? { project: { id: 'fixture-project' }, conversationId: 'fixture-conversation' });
    } else if (req.method === 'POST' && pathname === '/api/projects/fixture-project/chat') {
      send(201, { runId: 'fixture-run' });
    } else if (req.method === 'GET' && pathname === '/api/runs/fixture-run') {
      send(200, { run: { status: scenario.status || 'succeeded' } });
    } else if (req.method === 'GET' && pathname === '/api/runs/fixture-run/events') {
      if (scenario.eventStatus) return send(scenario.eventStatus, { error: 'stream failure' });
      res.writeHead(200, { 'Content-Type': 'text/event-stream' });
      if (!scenario.keepStream) res.end(scenario.eventBody ?? 'data: {"type":"done"}\n\n');
    } else if (req.method === 'GET' && pathname === '/api/projects/fixture-project/files') {
      send(200, scenario.files ?? { files: [{ path: 'index.html', size: 32 }, { path: 'style.css', size: 8 }] });
    } else if (req.method === 'GET' && pathname === '/api/projects/fixture-project/artifact') {
      send(200, scenario.artifact ?? {
        entryFile: 'index.html',
        files: {
          'index.html': { encoding: 'utf-8', content: '<!doctype html><html><body>Flux</body></html>', mimeType: 'text/html' },
          'style.css': { encoding: 'utf-8', content: 'body{}', mimeType: 'text/css' },
        },
      });
    } else if (req.method === 'GET' && pathname === '/preview/fixture-project/') {
      send(200, scenario.preview ?? '<!doctype html><html><script data-pidesigner-bridge></script></html>', 'text/html');
    } else if (req.method === 'GET' && pathname === '/api/conversations/fixture-conversation/messages') {
      send(200, { messages: [{ role: 'assistant', content: 'Flux page is ready.' }] });
    } else {
      send(404, { error: `Unexpected request ${req.method} ${pathname}` });
    }
  });
  return new Promise(resolve => server.listen(0, '127.0.0.1', () => {
    resolve({ server, base: `http://127.0.0.1:${server.address().port}` });
  }));
}

async function runE2E(scenario, overrides = {}) {
  const { server, base } = await startApi(scenario);
  const temp = temporaryDirectory('pidesigner-e2e-case-');
  const scratch = path.join(temp, 'scratch');
  fs.mkdirSync(scratch);
  try {
    const result = await new Promise(resolve => execFile('bash', [e2eScript], {
      encoding: 'utf8',
      timeout: 15000,
      env: {
        ...process.env,
        TMPDIR: scratch,
        PIDESIGNER_E2E_BASE: base,
        PIDESIGNER_E2E_TIMEOUT_SECONDS: '2',
        PIDESIGNER_E2E_POLL_INTERVAL_SECONDS: '1',
        PIDESIGNER_E2E_STREAM_TIMEOUT_SECONDS: '10',
        PIDESIGNER_E2E_KEEP_OUTPUTS: '0',
        ...overrides,
      },
    }, (error, stdout, stderr) => resolve({
      status: error ? (Number.isInteger(error.code) ? error.code : null) : 0,
      error: error && !Number.isInteger(error.code) ? error : undefined,
      stdout,
      stderr,
    })));
    assert.equal(result.error, undefined, result.error?.message);
    assert.deepEqual(fs.readdirSync(scratch), [], 'per-run temporary output should be cleaned by the trap');
    return { ...result, output: `${result.stdout || ''}${result.stderr || ''}` };
  } finally {
    await new Promise(resolve => server.close(resolve));
    fs.rmSync(temp, { recursive: true, force: true });
  }
}

test('e2e harness succeeds only after validating the full mock API, artifact and preview', async () => {
  const result = await runE2E({});
  assert.equal(result.status, 0, result.output);
});

test('e2e harness propagates HTTP and JSON response failures', async t => {
  await t.test('HTTP failure', async () => {
    const result = await runE2E({ createStatus: 503, createBody: { error: 'fixture failure' } });
    assert.notEqual(result.status, 0);
  });
  await t.test('malformed JSON', async () => {
    const result = await runE2E({ createBody: 'not-json' });
    assert.notEqual(result.status, 0);
  });
});

test('e2e harness rejects failed, canceled, unknown and timed-out runs', async t => {
  for (const status of ['failed', 'canceled', 'unknown']) {
    await t.test(status, async () => {
      const result = await runE2E({ status });
      assert.notEqual(result.status, 0);
    });
  }
  await t.test('timeout', async () => {
    const result = await runE2E({ status: 'running', keepStream: true });
    assert.notEqual(result.status, 0);
  });
});

test('e2e harness requires valid event streams and actual artifact/preview assertions', async t => {
  await t.test('malformed SSE JSON', async () => {
    const result = await runE2E({ eventBody: 'data: not-json\n\n' });
    assert.notEqual(result.status, 0);
  });
  await t.test('SSE HTTP failure', async () => {
    const result = await runE2E({ eventStatus: 503 });
    assert.notEqual(result.status, 0);
  });
  await t.test('missing artifact stylesheet', async () => {
    const result = await runE2E({ artifact: { entryFile: 'index.html', files: {
      'index.html': { encoding: 'utf-8', content: '<html></html>', mimeType: 'text/html' },
    } } });
    assert.notEqual(result.status, 0);
  });
  await t.test('missing preview bridge', async () => {
    const result = await runE2E({ preview: '<html><body>no bridge</body></html>' });
    assert.notEqual(result.status, 0);
  });
});

