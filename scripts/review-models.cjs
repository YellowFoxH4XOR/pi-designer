#!/usr/bin/env node
// Real installed omp metadata, real Go persistence, and an optional one-line
// inference through the app. All project data is confined to a review daemon.
const { chromium } = require(process.env.PLAYWRIGHT_PACKAGE || '../ui/node_modules/playwright-core');
const fs = require('node:fs/promises');
const assert = require('node:assert/strict');
(async () => {
  const meta = JSON.parse(await fs.readFile('/tmp/pidesigner-model-review.json', 'utf8'));
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
  const page = await browser.newPage({ viewport: { width: 1440, height: 960 }, reducedMotion: 'reduce' });
  const errors=[]; page.on('pageerror', e => errors.push(e.message));
  const checks=[]; const pass = (name) => { checks.push(name); console.log('PASS', name); };
  const request = async (route,body) => {
    const response=await fetch(meta.url+route, body ? {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)} : {});
    return {status:response.status,data:await response.json()};
  };
  try {
    await fs.mkdir('.impeccable/review', { recursive: true });
    await page.goto(meta.url);
    await page.getByRole('heading',{name:/Good ideas/}).waitFor();
    await page.getByRole('button',{name:'Choose omp model'}).waitFor();
    assert.ok(await page.locator('.model-button .model-icon').count());
    assert.equal(await page.locator('.brand .brand-mark').getAttribute('src'), '/brand.svg');
    pass('Selected-model icons and πDesigner app identity are distinct');
    await page.getByRole('button',{name:'Choose omp model'}).click();
    await page.locator('.model-row[data-model-ref]').first().waitFor({timeout:45000});
    const {data:catalog}=await request('/api/models');
    assert.ok(catalog.models.length>0);
    assert.equal(await page.locator('.model-row[data-model-ref]').count(), catalog.models.length);
    const target=catalog.models.find(m=>`${m.provider}/${m.id}`==='github-copilot/gpt-5.3-codex')||catalog.models[0];
    const ref=`${target.provider}/${target.id}`;
    await page.getByRole('searchbox',{name:'Search models or providers'}).fill(target.id);
    await page.screenshot({path:'.impeccable/review/model-picker.png'});
    await page.locator(`[data-model-ref="${ref}"]`).click();
    await page.getByRole('dialog').waitFor({state:'detached',timeout:45000});
    let prefs=(await request('/api/preferences')).data;
    assert.equal(prefs.model,ref);
    const levels=(await request('/api/models/thinking?model='+encodeURIComponent(ref))).data.levels;
    assert.ok(levels.includes(prefs.thinking));
    pass(`Real omp catalog (${catalog.models.length} models), exact selection, supported thinking, and save`);
    await page.reload();
    await page.getByRole('heading',{name:/Good ideas/}).waitFor();
    await page.waitForFunction(name=>document.querySelector('.model-button strong')?.textContent===name,target.name);
    pass('Chosen model survives app-view reload');
    await page.getByRole('link',{name:/^Settings/}).click();
    await page.getByRole('heading',{name:'Your workspace',exact:true}).waitFor();
    await page.waitForFunction(()=>!document.querySelector('.preferences-form select')?.disabled);
    const actualLevels=await page.getByLabel('Thinking level').locator('option').evaluateAll(options=>options.filter(o=>!o.disabled).map(o=>o.value));
    assert.deepEqual(actualLevels,levels);
    assert.equal((await page.locator('.agent-card .agent-mark').textContent()).trim(),'omp');
    await page.getByLabel('Thinking level').selectOption(levels.includes('low')?'low':levels[0]);
    await page.getByRole('button',{name:'Save preferences'}).click();
    await page.getByRole('status').filter({hasText:'Preferences saved'}).waitFor();
    await page.screenshot({path:'.impeccable/review/model-settings.png'});
    pass('Settings uses the omp badge and only selected-model thinking levels');
    const invalid=await request('/api/preferences',{model:'not-a-provider/not-a-model',thinking:'medium'});
    assert.equal(invalid.status,400);
    assert.equal((await request('/api/preferences')).data.model,ref);
    pass('Invalid model ids are rejected without overwriting the saved model');
    await page.goto(meta.url+'/#home');
    await page.getByRole('heading',{name:/Good ideas/}).waitFor();
    await page.screenshot({path:'.impeccable/review/model-home.png'});
    await page.setViewportSize({width:900,height:620});
    await page.waitForTimeout(150);
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
    assert.equal(overflow,false);
    await page.screenshot({path:'.impeccable/review/model-home-900.png'});
    pass('Model control fits the minimum Mac window');
    if(process.env.PIDESIGNER_LIVE_UI==='1') {
      await page.getByLabel('Describe your design').fill('Connection check only: reply exactly MODEL_SELECTION_OK. Do not read or modify files or call tools.');
      const sent=page.waitForRequest(r=>r.method()==='POST' && /\/chat$/.test(new URL(r.url()).pathname));
      await page.getByRole('button',{name:'Create prototype'}).click();
      const body=(await sent).postDataJSON();
      assert.equal(body.model,ref);
      await page.locator('.run-model').filter({hasText:target.name}).waitFor({timeout:45000});
      await page.getByText('Finished. Response saved to this conversation.',{exact:true}).waitFor({timeout:90000});
      assert.ok((await page.locator('.conversation-messages').textContent()).includes('MODEL_SELECTION_OK'));
      await page.screenshot({path:'.impeccable/review/model-run.png'});
      pass('Real UI run sent selected id, omp confirmed the model, and inference completed');
    }
    assert.deepEqual(errors,[]);pass('No browser JavaScript errors');
    await fs.writeFile('.impeccable/review/model-report.json',JSON.stringify({checks,model:ref,configuredModels:catalog.models.length,errors},null,2));
  } finally {await browser.close();}
})().catch(e=>{console.error(e.stack);process.exitCode=1;});
