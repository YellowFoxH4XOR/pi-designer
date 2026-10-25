const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { runInNewContext } = require('node:vm');
const ts = require('typescript');

// Exercise the real hook's polling effect without a browser or live daemon.
function discoveryHarness() {
  let effect, poll, cleanup, polls = 0, refreshes = 0;
  let stamp = '1', refreshError = false;
  const listeners = new Map();
  const document = {
    visibilityState: 'visible',
    addEventListener: (name, listener) => listeners.set(name, listener),
    removeEventListener: name => listeners.delete(name),
  };
  const exports = {};
  const source = ts.transpileModule(readFileSync('src/state.ts', 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  runInNewContext(source, {
    exports, document,
    clearInterval: () => { poll = undefined; },
    window: {
      setInterval: callback => { poll = callback; return 1; },
      clearInterval: () => { poll = undefined; },
    },
    require: name => {
      if (name === 'preact/hooks') return {
        useEffect: callback => { effect = callback; },
        useRef: current => ({ current }),
        useState: initial => [initial, () => {}],
      };
      if (name === './api') return { api: {
        activity: async () => { polls++; return { activities: [], projects: stamp }; },
      } };
      if (name === './activity') return {};
      throw new Error(`Unexpected import: ${name}`);
    },
  });
  exports.useRuns(() => {}, async () => {
    refreshes++;
    if (refreshError) throw new Error('Project list temporarily unavailable');
  });
  cleanup = effect();
  const flush = () => new Promise(resolve => setImmediate(resolve));
  return {
    get polls() { return polls; },
    get refreshes() { return refreshes; },
    get listeners() { return listeners.size; },
    setStamp: value => { stamp = value; },
    failRefresh: value => { refreshError = value; },
    setVisibility: async value => {
      document.visibilityState = value;
      await listeners.get('visibilitychange')?.();
    },
    tick: async () => { await poll?.(); },
    flush, cleanup,
  };
}

test('first activity poll refreshes projects, including a creation during page load', async () => {
  const h = discoveryHarness();
  try {
    await h.flush();
    assert.equal(h.refreshes, 1);
    await h.tick();
    assert.equal(h.refreshes, 1, 'unchanged revision should not fetch the project list');
    h.setStamp('2');
    await h.tick();
    assert.equal(h.refreshes, 2);
  } finally { h.cleanup(); }
});

test('a failed project refresh is retried without requiring another mutation', async () => {
  const h = discoveryHarness();
  try {
    await h.flush();
    const refreshes = h.refreshes;
    h.setStamp('2');
    h.failRefresh(true);
    await h.tick();
    assert.equal(h.refreshes, refreshes + 1);
    h.failRefresh(false);
    await h.tick();
    assert.equal(h.refreshes, refreshes + 2);
    await h.tick();
    assert.equal(h.refreshes, refreshes + 2, 'successful refresh acknowledges the revision');
  } finally { h.cleanup(); }
});

test('hidden windows skip polling and discover changes immediately on return', async () => {
  const h = discoveryHarness();
  try {
    await h.flush();
    const polls = h.polls;
    const refreshes = h.refreshes;
    await h.setVisibility('hidden');
    h.setStamp('2');
    await h.tick();
    assert.equal(h.polls, polls);
    await h.setVisibility('visible');
    assert.equal(h.polls, polls + 1);
    assert.equal(h.refreshes, refreshes + 1);
  } finally { h.cleanup(); }
  assert.equal(h.listeners, 0);
  const polls = h.polls;
  await h.tick();
  assert.equal(h.polls, polls);
});
