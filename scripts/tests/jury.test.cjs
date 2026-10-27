const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { EventEmitter } = require('node:events');
const ts = require('../../ui/node_modules/typescript');

// Exercise the actual embedded extension without pi, rendering, or inference.
const source = fs.readFileSync(path.join(__dirname, '../../internal/agentext/pidesigner.ts'), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ESNext, esModuleInterop: true } }).outputText;
function loadTools(review, options = {}) {
  const tools = new Map();
  const exports = {};
  const env = { PIDESIGNER_PI_BIN: '/pi', PIDESIGNER_RENDERER: '/renderer', PIDESIGNER_RENDER_DIR: '/renders', PIDESIGNER_BRIEF_FILE: '/brief', PIDESIGNER_JURY_MODEL: 'judge/model', ...options.env };
  const requireMock = (name) => {
    if (name === 'typebox') return { Type: new Proxy({}, { get: () => () => ({}) }) };
    if (name === 'node:fs/promises') return { realpath: options.realpath || (async p => p), readFile: options.readFile || (async p => p === '/brief' ? 'Ceramics launch' : '<html></html>'), mkdir: async () => {}, stat: async () => ({ isFile: () => true }) };
    if (name === 'node:child_process') return {
      execFile: (_file, args, _opts, cb) => { options.onRender?.(args); cb(null, '{"ok":true}'); },
      spawn: (_bin, args) => {
        assert.equal(args[args.indexOf('--model') + 1], 'judge/model');
        const child = new EventEmitter(); child.stdout = new EventEmitter(); child.stderr = new EventEmitter(); child.kill = () => {};
        queueMicrotask(() => { child.stdout.emit('data', JSON.stringify(review)); child.emit('close', 0); });
        return child;
      },
    };
    return require(name);
  };
  vm.runInNewContext(compiled, { exports, require: requireMock, process: { env }, Promise, setTimeout, clearTimeout, AbortSignal });
  exports.default({ registerTool: tool => tools.set(tool.name, tool) });
  return tools;
}
const load = (review, options) => loadTools(review, options).get('submit_critique');
const panel = () => ({ designer: 'A tactile studio launch', signatureMoment: 'An interactive glaze inspection in the hero', panelists: ['critic', 'brand', 'a11y', 'copy'].map((role, i) => ({ role, score: [8, 9, 9, 9][i], notes: 'Evidence from the mobile and web renders', mustFix: [] })) });
const review = score => ({ design: score, usability: score, creativity: score, content: score, verdict: 'Independent verdict', issues: ['Improve the collection imagery'] });
const execute = (tool, params = panel()) => tool.execute('test', params, undefined, undefined, { cwd: '/project', model: { provider: 'generator', id: 'model' } });

test('independent jury overrides a high self-score and caps revision rounds', async () => {
  const tool = load(review(6));
  for (let round = 1; round <= 3; round++) {
    const { details, content } = await execute(tool);
    assert.equal(details.round, round);
    assert.equal(details.composite, 6);
    assert.equal(details.selfComposite, 8.6);
    assert.equal(details.jury.model, 'judge/model');
    assert.equal(details.decision, round === 3 ? 'ship_with_issues' : 'revise');
    assert.match(content[0].text, /Jury: Improve the collection imagery/);
  }
});

test('jury passing score can ship only without jury or panel must-fix items', async () => {
  const cleanReview = { ...review(8), issues: [] };
  assert.equal((await execute(load(cleanReview))).details.decision, 'ship');
  const params = panel(); params.panelists[2].mustFix.push('Repair keyboard focus');
  assert.equal((await execute(load(cleanReview), params)).details.decision, 'revise');
  const tool = load(review(8));
  for (let round = 1; round <= 3; round++) {
    const result = await execute(tool);
    assert.equal(result.details.decision, round === 3 ? 'ship_with_issues' : 'revise');
    assert.match(result.content[0].text, /Jury: Improve the collection imagery/);
  }
});

test('none signature prevents shipping even with a passing jury', async () => {
  const params = panel(); params.signatureMoment = 'none';
  const result = await execute(load({ ...review(8), issues: [] }), params);
  assert.equal(result.details.decision, 'revise');
  assert.match(result.content[0].text, /Create a signature moment/);
});

test('jury ignores blank issue strings rather than blocking a clean review', async () => {
  const result = await execute(load({ ...review(8), issues: [' ', '\t'] }));
  assert.equal(result.details.decision, 'ship');
  assert.equal(result.details.jury.issues.length, 0);
});

test('jury renders the configured project entry, including nested files', async () => {
  const rendered = [];
  const result = await execute(load({ ...review(8), issues: [] }, {
    env: { PIDESIGNER_ENTRY_FILE: 'pages/start.html' },
    readFile: async p => {
      assert.ok(p === '/brief' || p === '/project/pages/start.html', `unexpected file ${p}`);
      return p === '/brief' ? 'Brief' : '<html></html>';
    },
    onRender: args => rendered.push(args[args.indexOf('--url') + 1]),
  }));
  assert.equal(result.details.decision, 'ship');
  assert.equal(rendered.length, 3);
  assert.ok(rendered.every(url => url === 'file:///project/pages/start.html'));
});

test('explicit critique entry overrides the configured entry', async () => {
  const rendered = [];
  await execute(load(review(6), {
    env: { PIDESIGNER_ENTRY_FILE: 'old.html' },
    onRender: args => rendered.push(args[args.indexOf('--url') + 1]),
  }), { ...panel(), path: 'new.html' });
  assert.ok(rendered.length > 0 && rendered.every(url => url === 'file:///project/new.html'));
});

test('a new project jury uses the most recently rendered entry', async () => {
  const rendered = [];
  const tools = loadTools(review(6), { onRender: args => rendered.push(args[args.indexOf('--url') + 1]) });
  await execute(tools.get('render_preview'), { path: 'pages/prototype.html', viewport: 'mobile' });
  await execute(tools.get('submit_critique'));
  assert.equal(rendered.length, 4);
  assert.ok(rendered.every(url => url === 'file:///project/pages/prototype.html'));
});

test('jury refuses entries escaping the project, including symlinks', async () => {
  for (const entry of ['../outside.html', 'linked.html']) {
    let renders = 0;
    const result = await execute(load(review(8), {
      realpath: async p => p === '/project/linked.html' ? '/outside.html' : p,
      onRender: () => renders++,
    }), { ...panel(), path: entry });
    assert.equal(renders, 0);
    assert.equal(result.details.jury, undefined);
    assert.match(result.content[0].text, /review entry must be inside the project/);
  }
});

test('malformed jury scores are disclosed, never recorded as valid reviews', async () => {
  const result = await execute(load({ ...review(8), design: 'invalid' }));
  assert.equal(result.details.jury, undefined);
  assert.match(result.content[0].text, /Independent review failed.*Invalid jury score for design/);
});
