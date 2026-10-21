#!/usr/bin/env node
// Real API + browser, isolated data, fixture RPC. No accounts or inference.
const { chromium } = require('../ui/node_modules/playwright-core');
const { spawn, execFileSync } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const assert = require('node:assert/strict');
(async () => {
 const root = path.resolve(__dirname, '..');
 const data = await fs.mkdtemp(path.join(os.tmpdir(), 'omp-activity-review-'));
 const out = path.join(root, '.impeccable/review'); await fs.mkdir(out, { recursive: true });
 const child = spawn(path.join(root, 'bin/pidesigner'), ['--port','0','--print-url','--data',data,'--content',path.join(root,'content'),'--ui',path.join(root,'ui/dist'),'--omp',path.join(root,'scripts/fixtures/omp-rpc.cjs')], { stdio: ['ignore','pipe','pipe'] });
 let logs = ''; child.stderr.on('data', chunk => { logs += chunk; });
 let browser;
 const checks = []; const pass = name => { checks.push(name); console.log('PASS',name); };
 try {
  const base = await new Promise((resolve,reject) => { let stdout=''; const timeout=setTimeout(()=>reject(Error('daemon startup timeout')),15000); child.stdout.on('data', chunk => {stdout+=chunk;const match=stdout.match(/PIDESIGNER_READY (http:\/\/[^\s]+)/);if(match){clearTimeout(timeout);resolve(match[1]);}}); child.once('error',reject); });
  async function api(route,body) { const r=await fetch(base+route,body?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{}); assert.ok(r.ok,`${route}: ${r.status} ${r.ok?'':await r.text()}`); return r.json(); }
  async function waitForRun(id,status) { for(let i=0;i<120;i++){const {run}=await api('/api/runs/'+id);if(run.status===status)return run;await new Promise(r=>setTimeout(r,150));}throw Error(`run ${id} never reached ${status}`); }
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
  const context=await browser.newContext({viewport:{width:1440,height:900},reducedMotion:'reduce'});
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base);await page.getByRole('heading',{name:/Good ideas/}).waitFor();
  assert.ok((await page.title()).includes('πDesigner'));await page.locator('.model-button img').waitFor();
  assert.equal(await page.locator('.model-button img').getAttribute('src'),'/model-icons/openai.svg');
  await page.getByRole('button',{name:'Choose omp model'}).click();await page.locator('[data-model-ref="anthropic/claude-opus-fixture"]').waitFor();
  for(const [ref,icon]of[['anthropic/claude-opus-fixture','claude'],['opencode-go/deepseek-fixture','deepseek'],['opencode-go/custom-fixture','opencode']])assert.equal(await page.locator(`[data-model-ref="${ref}"] img`).getAttribute('src'),`/model-icons/${icon}.svg`);
  await page.screenshot({path:path.join(out,'activity-model-picker.png')});
  await page.locator('[data-model-ref="opencode-go/deepseek-fixture"]').click();await page.getByRole('dialog').waitFor({state:'detached'});
  pass('omp brand and real model/provider icon mapping');
  await page.getByLabel('Describe your design').fill('LONG_TASK Build an observable design fixture');
  await page.getByRole('button',{name:'Create prototype'}).click();await page.getByRole('button',{name:'Rename project'}).waitFor();
  const projectId=new URL(page.url()).hash.slice('#project/'.length);
  const {runs}=await api(`/api/projects/${projectId}/runs`);const runId=runs[0].id;
  await page.locator('.run-activity-toggle').click();await page.locator('.activity-step').first().waitFor();
  await page.locator('.activity-step').filter({hasText:'Running command'}).locator('summary').first().click();
  await page.getByText('Checking…',{exact:true}).waitFor();await page.screenshot({path:path.join(out,'activity-working.png')});
  pass('Live current action, parallel tool identity and streaming output');
  await context.setOffline(true);await page.waitForTimeout(400);await context.setOffline(false);
  await page.reload();await page.getByRole('button',{name:'Rename project'}).waitFor();
  await waitForRun(runId,'succeeded');await page.getByText('Finished. Your design is ready to explore.',{exact:true}).waitFor();
  assert.equal(await page.locator('.message-assistant').getByText('Fixture complete. The design is ready to explore.',{exact:true}).count(),1);
  const {activity}=await api(`/api/runs/${runId}/activity`);
  assert.equal(activity.tools.length,3);assert.ok(activity.tools.every(t=>t.status==='succeeded'));assert.ok(activity.text.includes('Fixture complete'));
  await page.locator('.run-activity-toggle').click();await page.getByText('Event log & diagnostics',{exact:true}).click();await page.getByLabel('Recorded run events').waitFor();
  assert.ok((await page.getByLabel('Recorded run events').textContent()).includes('Fixture diagnostic'));
  pass('Offline/reload recovery, no duplicate messages, durable tool history and stderr diagnostics');
  await page.screenshot({path:path.join(out,'activity-finished.png')});
  const p=await api('/api/projects',{name:'Attention review fixture',skillId:'web-prototype'});
  const start=await api(`/api/projects/${p.project.id}/chat`,{prompt:'ASK_EXTENSION',conversationId:p.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await page.getByRole('button',{name:/Activity|working|needs attention/}).first().click();await page.getByRole('dialog',{name:'Workspace activity'}).waitFor();
  await page.getByRole('dialog').getByRole('button').filter({hasText:'Attention review fixture'}).click();
  await page.getByRole('heading',{name:'Continue the fixture design?'}).waitFor();
  await page.getByRole('button',{name:'Confirm',exact:true}).click();await waitForRun(start.runId,'succeeded');
  pass('Global discovery of externally started work and actionable extension UI attention');
  const form=await api(`/api/projects/${p.project.id}/chat`,{prompt:'ASK_FORM',conversationId:p.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await waitForRun(form.runId,'succeeded');
  for(let i=0;i<30;i++){const r=await api(`/api/runs/${form.runId}/activity`);if(r.activity.attention)break;await page.waitForTimeout(100);}
  assert.equal((await api(`/api/runs/${form.runId}/activity`)).activity.attention,true);
  await page.reload();await page.getByRole('heading',{name:'Choose a direction'}).waitFor();
  await page.screenshot({path:path.join(out,'activity-attention.png')});
  pass('Question-only completion remains Needs your answer, including after reload');
  const failed=await api(`/api/projects/${p.project.id}/chat`,{prompt:'FAIL_RUN',conversationId:p.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await waitForRun(failed.runId,'failed');assert.equal((await api(`/api/runs/${failed.runId}/activity`)).activity.status,'failed');
  const canceled=await api(`/api/projects/${p.project.id}/chat`,{prompt:'LONG_TASK cancellation',conversationId:p.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  for(let i=0;i<30;i++){const r=await api(`/api/runs/${canceled.runId}/activity`);if(r.activity.tools.length)break;await page.waitForTimeout(100);}
  await api(`/api/runs/${canceled.runId}/cancel`,{});await waitForRun(canceled.runId,'canceled');
  pass('Provider failure and cancellation are terminal, not false success');
  await page.waitForTimeout(1700);
  assert.ok(!(await page.locator('.activity-center-button').textContent()).includes('needs attention'), 'superseded questions must not remain in global activity');
  pass('Superseded attention clears from the global activity control');
  await page.setViewportSize({width:900,height:620});await page.goto(base+'/#project/'+projectId+'?activity=1');await page.getByRole('button',{name:'Rename project'}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  assert.ok(await page.getByLabel('Refine your design').isVisible());await page.screenshot({path:path.join(out,'activity-studio-900.png')});
  pass('Activity, composer and preview fit the minimum Mac window');
  const stress=await api('/api/projects',{name:'Scroll containment fixture',skillId:'web-prototype'});
  const stressRun=await api(`/api/projects/${stress.project.id}/chat`,{prompt:'SCROLL_STRESS',conversationId:stress.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await waitForRun(stressRun.runId,'succeeded');
  const stressURL=base+'/#project/'+stress.project.id+'?activity=1';
  await page.goto(stressURL);await page.locator('.todo-item').last().waitFor();
  assert.equal(await page.locator('.todo-item').count(),60);
  for(const size of [{width:1440,height:900},{width:900,height:620},{width:900,height:480}]) {
    await page.setViewportSize(size);await page.waitForTimeout(200);
    const dimensions=await page.evaluate(()=>({height:innerHeight,scroll:document.scrollingElement.scrollHeight}));
    assert.ok(dimensions.scroll<=dimensions.height+1,`Document grew beyond viewport with long plan/critique: ${JSON.stringify(dimensions)}`);
    await page.evaluate(()=>{window.scrollTo(0,100000);const pane=document.querySelector('.conversation-messages');pane.scrollTop=pane.scrollHeight;});
    assert.equal(await page.evaluate(()=>scrollY),0,'App shell must not scroll into blank space');
    assert.ok(await page.locator('.conversation-messages').evaluate(e=>e.scrollTop>1000),'Conversation must remain independently scrollable');
    const bounds=await page.locator('.conversation-messages').boundingBox();await page.mouse.move(bounds.x+30,bounds.y+bounds.height-5);await page.mouse.wheel(0,10000);await page.waitForTimeout(100);
    assert.equal(await page.evaluate(()=>scrollY),0,'Reaching the conversation end must not scroll the app shell');
    const fixed=await page.evaluate(()=>({header:document.querySelector('.studio-header').getBoundingClientRect().top,composer:document.querySelector('.studio-composer').getBoundingClientRect().bottom,height:innerHeight}));
    assert.ok(fixed.header>=0&&fixed.composer<=fixed.height+1,'Header and composer must remain visible');
    assert.ok((await page.locator('.todo-card').ariaSnapshot()).includes('Pending'),'Visually hidden plan statuses must stay accessible');
    await page.screenshot({path:path.join(out,`scroll-contained-${size.width}.png`)});
  }
  pass('Long plans and critiques cannot expand or scroll the document; inner conversation and accessible statuses remain intact');
  await page.evaluate(()=>{const banner=document.createElement('div');banner.className='error-banner';banner.id='scroll-error-fixture';const text=document.createElement('span');text.textContent='Long recoverable error. '.repeat(100)+'x'.repeat(3000);banner.append(text);document.getElementById('app').prepend(banner);});
  const errorLayout=await page.evaluate(()=>({height:innerHeight,root:document.scrollingElement.scrollHeight,banner:document.getElementById('scroll-error-fixture').getBoundingClientRect().height,shell:document.querySelector('.studio-shell').getBoundingClientRect().bottom,composer:document.querySelector('.studio-composer').getBoundingClientRect().bottom}));
  assert.ok(errorLayout.root<=errorLayout.height+1&&errorLayout.shell<=errorLayout.height+1&&errorLayout.composer<=errorLayout.height+1&&errorLayout.banner<=errorLayout.height*.25+1,JSON.stringify(errorLayout));
  await page.locator('#scroll-error-fixture').evaluate(e=>e.remove());
  pass('Variable-height error banners preserve the bounded Studio and composer');
  if(process.platform==='darwin') {
    const renderer=path.join(root,'bin/pidesigner-render');let available=true;try{await fs.access(renderer);}catch{available=false;}
    if(available){for(const size of [{width:1440,height:900},{width:900,height:620},{width:900,height:480}]) {
      const report=JSON.parse(execFileSync(renderer,['--url',stressURL,'--width',String(size.width),'--height',String(size.height),'--out',path.join(out,`scroll-webkit-${size.width}.png`),'--timeout','20'],{encoding:'utf8'}));
      assert.ok(report.ok&&report.scrollHeight<=size.height+1,`WebKit document overflow: ${JSON.stringify(report)}`);assert.ok(report.title.includes('Scroll containment fixture'),'WebKit must load the actual Studio, not an empty page');
    }pass('Native WebKit contains long conversations at normal, minimum, and reduced-height viewports');}
    else console.log('SKIP native WebKit scroll check: build bin/pidesigner-render first');
  }
  assert.deepEqual(errors,[]);pass('No browser runtime errors');
  await fs.writeFile(path.join(out,'activity-report.json'),JSON.stringify({checks,errors,data},null,2));
 } finally {
  if(browser)await browser.close();child.kill('SIGTERM');await new Promise(resolve=>{child.once('exit',resolve);setTimeout(()=>{child.kill('SIGKILL');resolve();},5000).unref();});
  await fs.writeFile(path.join(out,'activity-daemon.log'),logs);await fs.rm(data,{recursive:true,force:true});
 }
})().catch(error=>{console.error(error.stack);process.exitCode=1;});
