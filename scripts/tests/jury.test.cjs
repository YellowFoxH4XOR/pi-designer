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
function load(review) {
  const tools = new Map();
  const exports = {};
  const env = { PIDESIGNER_PI_BIN: '/pi', PIDESIGNER_RENDERER: '/renderer', PIDESIGNER_RENDER_DIR: '/renders', PIDESIGNER_BRIEF_FILE: '/brief', PIDESIGNER_JURY_MODEL: 'judge/model' };
  const requireMock = (name) => {
    if (name === 'typebox') return { Type: new Proxy({}, { get: () => () => ({}) }) };
    if (name === 'node:fs/promises') return { realpath: async p => p, readFile: async p => p === '/brief' ? 'Ceramics launch' : '<html></html>', mkdir: async () => {} };
    if (name === 'node:child_process') return {
      execFile: (_file, _args, _opts, cb) => cb(null, '{"ok":true}'),
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
  return tools.get('submit_critique');
}
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

test('jury passing score can ship but cannot override panel must-fix items', async () => {
  assert.equal((await execute(load(review(8)))).details.decision, 'ship');
  const params = panel(); params.panelists[2].mustFix.push('Repair keyboard focus');
  assert.equal((await execute(load(review(8)), params)).details.decision, 'revise');
});

test('none signature prevents shipping even with a passing jury', async () => {
  const params = panel(); params.signatureMoment = 'none';
  const result = await execute(load(review(8)), params);
  assert.equal(result.details.decision, 'revise');
  assert.match(result.content[0].text, /Create a signature moment/);
});

test('malformed jury scores are disclosed, never recorded as valid reviews', async () => {
  const result = await execute(load({ ...review(8), design: 'invalid' }));
  assert.equal(result.details.jury, undefined);
  assert.match(result.content[0].text, /Independent review failed.*Invalid jury score for design/);
});
