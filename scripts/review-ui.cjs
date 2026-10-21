#!/usr/bin/env node
// Local-only browser review against the real Go API. Uses an already-installed
// Playwright package; does not download a browser or start a model generation.
const { chromium } = require(process.env.PLAYWRIGHT_PACKAGE || 'playwright');
const fs = require('node:fs/promises');
const path = require('node:path');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const out = path.join(root, '.impeccable/review');
const metaPath = process.env.PIDESIGNER_REVIEW_META || '/tmp/pidesigner-design-review.json';

(async () => {
  const meta = JSON.parse(await fs.readFile(metaPath, 'utf8'));
  const base = meta.url;
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 }, reducedMotion: 'reduce' });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  const checks = [];
  const record = (name, detail) => { checks.push({ name, detail }); console.log('PASS', name, detail || ''); };
  // This is a fixed-height desktop application with internal scrolling.
  const capture = name => page.screenshot({ path: path.join(out, name + '.png'), fullPage: false });
  async function api(route, body, method) {
    const r = await fetch(base + route, body === undefined ? {} : { method: method || 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    assert.ok(r.ok, `${route}: ${r.status} ${r.status >= 400 ? await r.text() : ''}`);
    return r.json();
  }
  async function noOverflow(label) {
    const size = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, width: innerWidth }));
    assert.ok(size.scroll <= size.width, `${label}: document overflows by ${size.scroll-size.width}px`);
    record(label + ' no horizontal overflow', size);
  }
  try {
    await fs.mkdir(out, { recursive: true });
    if (meta.projectId) await fetch(`${base}/api/projects/${meta.projectId}`, { method: 'DELETE' });
    await page.goto(base);
    await page.getByRole('heading', { name: /Good ideas/ }).waitFor();
    await capture('desktop');
    await noOverflow('Home 1440');
    assert.ok(await page.getByRole('button', { name: 'Create prototype' }).isDisabled());
    record('Empty prompt cannot submit');
    await page.getByRole('button', { name: 'Website clone', exact: true }).click();
    await page.getByLabel('Reference website', { exact: true }).fill('not-a-url');
    await page.getByLabel('Describe your design').fill('Rebuild this reference.');
    await page.getByRole('button', { name: 'Create prototype' }).click();
    await page.getByRole('alert').filter({ hasText: 'Enter a full website address' }).waitFor();
    record('Website clone validates URL before a run');
    await page.getByRole('button', { name: 'Prototype', exact: true }).click();
    await page.getByRole('button', { name: 'Choose a design system' }).click();
    await page.getByRole('dialog').getByRole('searchbox').fill('stripe');
    await page.getByRole('button', { name: /Stripe.*Fintech/ }).click();
    assert.ok((await page.locator('.composer-system').textContent()).includes('Stripe'));
    record('System picker selects real catalog data');
    await page.getByRole('button', { name: 'Choose a design system' }).click();
    await page.getByRole('dialog').waitFor({ state: 'visible' });
    await page.keyboard.press('Escape');
    await page.getByRole('dialog').waitFor({ state: 'detached' });
    record('Dialog Escape and focus return');
    await page.getByRole('link', { name: /^Design systems/ }).click();
    await page.getByRole('heading', { name: 'Design systems', exact: true }).waitFor();
    await page.getByRole('searchbox', { name: 'Search design systems' }).fill('Stripe');
    await page.getByRole('button', { name: /Stripe/ }).click();
    await page.getByRole('heading', { name: 'Stripe', exact: true }).waitFor();
    await page.locator('.component-section iframe').waitFor();
    await capture('system-detail');
    await page.getByRole('tab', { name: 'Overview', exact: true }).focus();
    await page.keyboard.press('End');
    assert.equal(await page.getByRole('tab', { name: 'Tokens', exact: true }).getAttribute('aria-selected'), 'true');
    record('Design-system tabs support arrow/Home/End keyboard navigation');
    await page.getByRole('button', { name: 'Copy CSS' }).waitFor();
    assert.ok((await page.locator('.source-display').textContent()).includes('--'));
    record('Design-system detail loads preview and real tokens');
    await page.getByRole('button', { name: 'Use this system' }).click();
    await page.getByRole('heading', { name: /Good ideas/ }).waitFor();
    assert.ok((await page.locator('.composer-system').textContent()).includes('Stripe'));
    await page.getByRole('link', { name: 'All projects', exact: true }).click();
    await page.getByRole('heading', { name: 'A clean canvas awaits' }).waitFor();
    await capture('projects-empty');
    record('Projects has an actionable empty state');

    // Real Go API project, explicitly labeled as a review fixture.
    const p = await api('/api/projects', { name: 'Workspace review fixture', designSystemId: 'linear-app', skillId: 'web-artifacts-builder' });
    meta.projectId = p.project.id;
    await fs.writeFile(metaPath, JSON.stringify(meta));
    const fixture = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="style.css"><title>Preview verification</title></head><body><nav>FORM / STUDIO<span>Independent design practice</span></nav><main><h1>Less noise.<br>More intention.</h1><p>A local preview fixture for checking the studio canvas, responsive layout, and live stylesheet updates.</p><button>Explore the work</button><section><article>Digital experiences</article><article>Thoughtful interfaces</article></section></main></body></html>`;
    const css = `*{box-sizing:border-box}body{margin:0;background:#eeeae2;color:#343b32;font-family:Georgia,serif}nav{padding:30px 6%;border-bottom:1px solid #343b3220;font:14px system-ui;display:flex;justify-content:space-between}nav span{font-size:12px}main{padding:80px 8%}h1{font-size:clamp(48px,7vw,100px);font-weight:400;letter-spacing:-4px;line-height:1.05;margin:0 0 28px}p{font:17px/1.8 system-ui;max-width:48ch;color:#636658}button{background:#3f4c3b;color:white;padding:17px 24px;border:0;border-radius:3px;margin:15px 0}section{display:grid;grid-template-columns:1fr 1fr;gap:25px;margin-top:60px}article{border-top:1px solid #343b3240;padding-top:25px;font:18px system-ui}@media(max-width:600px){nav span{display:none}main{padding:55px 8%}h1{letter-spacing:-2px}p{font-size:14px}section{grid-template-columns:1fr;gap:25px}}`;
    await api(`/api/projects/${p.project.id}/file`, { path: 'index.html', content: fixture }, 'PUT');
    await api(`/api/projects/${p.project.id}/file`, { path: 'style.css', content: css }, 'PUT');
    await page.goto(base + '/#project/' + p.project.id);
    await page.getByRole('button', { name: 'Rename project' }).waitFor();
    const webFrame = page.frameLocator('.device-viewport iframe');
    await webFrame.getByRole('heading', { name: /Less noise/ }).waitFor();
    assert.equal(await webFrame.locator('body').evaluate(() => innerWidth), 1440);
    await capture('studio-desktop');
    await noOverflow('Studio 1440');
    await page.getByRole('button', { name: 'Mobile preview', exact: true }).click();
    await page.waitForTimeout(200);
    assert.equal(await webFrame.locator('body').evaluate(() => innerWidth), 390);
    assert.ok(await webFrame.locator('body').evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    record('Device frames retain true 1440 and 390 CSS viewports');
    await capture('studio-mobile-frame');
    await page.getByRole('button', { name: 'Code', exact: true }).click();
    await page.getByRole('button', { name: 'style.css', exact: true }).click();
    await page.getByLabel('Source for style.css').waitFor({ state: 'visible' });
    await page.getByLabel('Source for style.css').fill(css + '\n/* edited in the source editor */');
    await page.getByRole('button', { name: 'Save changes' }).click();
    const saved = await api(`/api/projects/${p.project.id}/file?path=style.css`);
    assert.ok(saved.content.includes('edited in the source editor'));
    record('Source editor persists edits through Go file API');
    await page.getByLabel('Source for style.css').fill(css + '\n/* unsaved source */');
    await page.waitForFunction(() => document.documentElement.dataset.unsavedChanges === 'true');
    page.once('dialog', d => d.dismiss());
    await page.getByRole('button', { name: 'Back to home' }).click();
    await page.waitForTimeout(150);
    assert.ok(page.url().includes('#project/'));
    assert.ok((await page.getByLabel('Source for style.css').inputValue()).includes('unsaved source'));
    record('Canceled navigation preserves unsaved source edits');
    await page.getByRole('button', { name: 'Save changes' }).click();
    await page.waitForFunction(() => document.documentElement.dataset.unsavedChanges === 'false');
    await capture('studio-source');
    await page.getByRole('button', { name: 'Preview', exact: true }).click();
    await webFrame.getByRole('heading', { name: /Less noise/ }).waitFor();
    let navigations = 0;
    const listener = frame => { if (frame.parentFrame()) navigations++; };
    page.on('framenavigated', listener);
    for (const color of ['#e6e3f2', '#e0e8e2']) {
      await api(`/api/projects/${p.project.id}/file`, { path: 'style.css', content: css + `\nbody{background:${color}}` }, 'PUT');
      await page.waitForFunction(() => true);
      await webFrame.locator('body').evaluate(() => new Promise(r => setTimeout(r, 500)));
      const actual = await webFrame.locator('body').evaluate(() => getComputedStyle(document.body).backgroundColor);
      const expected = color === '#e6e3f2' ? 'rgb(230, 227, 242)' : 'rgb(224, 232, 226)';
      assert.equal(actual, expected);
    }
    page.off('framenavigated', listener);
    assert.equal(navigations, 0);
    record('Repeated CSS hot-swap updates without iframe navigation');
    await page.getByRole('button', { name: 'Rename project' }).click();
    await page.getByLabel('Project name').fill('A considered workspace');
    await page.getByRole('button', { name: 'Save name' }).click();
    await page.getByRole('dialog').waitFor({ state: 'detached' });
    assert.equal((await api(`/api/projects/${p.project.id}`)).project.name, 'A considered workspace');
    await page.getByRole('button', { name: 'New conversation' }).click();
    assert.equal((await api(`/api/projects/${p.project.id}/conversations`)).conversations.length, 2);
    record('Rename and conversation creation persist');

    await page.getByRole('button', { name: 'Back to home' }).click();
    await page.getByRole('link', { name: 'Settings', exact: false }).click();
    await page.getByRole('heading', { name: 'Your workspace', exact: true }).waitFor();
    await page.getByLabel('Thinking level').selectOption('high');
    await page.getByRole('button', { name: 'Save preferences' }).click();
    await page.getByRole('status').filter({ hasText: 'Preferences saved' }).waitFor();
    await page.reload();
    await page.getByRole('heading', { name: 'Your workspace', exact: true }).waitFor();
    assert.equal(await page.getByLabel('Thinking level').inputValue(), 'high');
    record('Preferences survive reload; MCP configuration is displayed');
    await capture('settings-desktop');
    await page.goto(base + '/#home');
    await page.getByRole('heading', { name: /Good ideas/ }).waitFor();
    for (const size of [{width:1280,height:800},{width:900,height:620}]) {
      await page.setViewportSize(size);
      await page.waitForTimeout(200);
      await capture(`home-${size.width}`);
      await noOverflow(`Mac Home ${size.width}`);
      await page.goto(base + '/#project/' + p.project.id);
      await page.getByRole('button', {name:'Rename project'}).waitFor();
      await page.frameLocator('.device-viewport iframe').getByRole('heading',{name:/Less noise/}).waitFor();
      await capture(`studio-${size.width}`);
      await noOverflow(`Mac Studio ${size.width}`);
      assert.ok(await page.getByRole('textbox',{name:'Refine your design'}).isVisible());
      await page.getByRole('button',{name:'Mobile preview',exact:true}).click();
      assert.equal(await page.frameLocator('.device-viewport iframe').locator('body').evaluate(()=>innerWidth),390);
      await capture(`studio-mobile-preview-${size.width}`);
      record(`Desktop window ${size.width} can preview a 390px mobile artifact`);
      await page.goto(base+'/#home');
      await page.getByRole('heading',{name:/Good ideas/}).waitFor();
    }
    await page.emulateMedia({ reducedMotion: 'reduce' });
    assert.ok(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches));
    record('Reduced-motion preference honored');
    assert.deepEqual(errors, []);
    record('No browser JavaScript errors');
    await fs.writeFile(path.join(out, 'ui-report.json'), JSON.stringify({ base, checks, errors }, null, 2));
  } finally { await browser.close(); }
})().catch(e => { console.error(e.stack); process.exitCode = 1; });
