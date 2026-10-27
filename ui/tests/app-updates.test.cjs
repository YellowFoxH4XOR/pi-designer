const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');

function harness(native = true) {
  const state = [], listeners = new Map(), timers = new Map(), sent = [];
  let cursor = 0, mounted = false, cleanup;
  const exports = {};
  const source = ts.transpileModule(readFileSync('src/components/AppUpdates.tsx', 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, jsx: ts.JsxEmit.ReactJSX, jsxImportSource: 'preact' },
  }).outputText;
  const jsx = (type, props) => ({ type, props });
  runInNewContext(source, {
    exports,
    window: {
      __pidesignerNative: native,
      webkit: { messageHandlers: { pidesigner: { postMessage: m => sent.push(JSON.parse(JSON.stringify(m))) } } },
      addEventListener: (name, handler) => listeners.set(name, handler),
      removeEventListener: name => listeners.delete(name),
      setTimeout: callback => { timers.set(1, callback); return 1; },
      clearTimeout: id => timers.delete(id),
    },
    require: name => {
      if (name === 'preact/jsx-runtime') return { jsx, jsxs: jsx };
      if (name === 'preact/hooks') return {
        useState: initial => { const index = cursor++; if (!(index in state)) state[index] = initial; return [state[index], value => { state[index] = value; }]; },
        useEffect: effect => { if (!mounted) cleanup = effect(); },
      };
      throw new Error(`Unexpected import ${name}`);
    },
  });
  const render = () => { cursor = 0; const tree = exports.AppUpdates(); mounted = true; return tree; };
  const receive = value => listeners.get('pidesigner:update-settings')?.({ detail: value });
  return { render, receive, sent, timers, listeners, cleanup: () => cleanup?.() };
}
function elements(tree, type) {
  if (!tree || typeof tree !== 'object') return [];
  if (Array.isArray(tree)) return tree.flatMap(node => elements(node, type));
  return [...(tree.type === type ? [tree] : []), ...elements(tree.props?.children, type)];
}
function text(tree) {
  if (Array.isArray(tree)) return tree.map(text).join('');
  if (tree && typeof tree === 'object') return text(tree.props?.children);
  return tree == null || typeof tree === 'boolean' ? '' : String(tree);
}
const ready = { version: '0.3.0', configured: true, canCheck: true, busy: false, automaticChecks: true, automaticDownloads: false, message: 'Ready to check.' };

test('update settings are native-only and subscribe before requesting authoritative state', () => {
  const browser = harness(false);
  assert.match(text(browser.render()), /available in the πDesigner Mac app/);
  assert.equal(browser.sent.length, 0);
  const h = harness();
  h.render();
  assert.equal(h.listeners.size, 1);
  assert.equal(h.sent[0].action, 'getUpdateSettings');
  h.receive(ready);
  assert.match(text(h.render()), /0.3.0/);
  assert.equal(h.timers.size, 0);
  h.cleanup();
  assert.equal(h.listeners.size, 0);
});

test('check button and automatic-update controls call the native bridge without optimistic success', () => {
  const h = harness(); h.render(); h.receive(ready);
  const tree = h.render();
  elements(tree, 'button')[0].props.onClick();
  assert.equal(h.sent.at(-1).action, 'checkForUpdates');
  elements(tree, 'input')[1].props.onChange({ currentTarget: { checked: true } });
  assert.deepEqual(h.sent.at(-1), { action: 'updateUpdateSettings', preferences: { automaticDownloads: true } });
  assert.equal(elements(h.render(), 'input')[1].props.checked, false, 'host state, not the click, determines saved state');
  h.receive({ ...ready, busy: true });
  assert.match(text(elements(h.render(), 'button')[0]), /Show update progress/);
  h.cleanup();
});

test('unconfigured builds cannot check/install and disabled checks disable automatic downloads', () => {
  const h = harness(); h.render();
  h.receive({ ...ready, configured: false, canCheck: false, message: 'No signed feed.' });
  assert.equal(elements(h.render(), 'button')[0].props.disabled, true);
  assert.ok(elements(h.render(), 'input').every(input => input.props.disabled));
  assert.match(text(h.render()), /No signed feed/);
  h.receive({ ...ready, automaticChecks: false });
  assert.equal(elements(h.render(), 'input')[1].props.disabled, true);
  h.cleanup();
});

test('an unavailable native update service reports a timeout rather than spinning forever', () => {
  const h = harness(); h.render(); h.timers.get(1)();
  assert.match(text(h.render()), /did not respond/);
  h.cleanup();
  assert.equal(h.timers.size, 0);
});
