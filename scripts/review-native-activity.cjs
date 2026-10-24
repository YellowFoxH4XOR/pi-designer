#!/usr/bin/env node
// Native lifecycle integration. Requires already-granted Accessibility and
// Screen Recording access; never grants permissions or drives another app.
const { spawn, execFileSync } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const assert = require('node:assert/strict');
(async()=>{
 const root=path.resolve(__dirname,'..');const data=await fs.mkdtemp(path.join(os.tmpdir(),'pi-native-review-'));
 const out=path.join(root,'.impeccable/review');await fs.mkdir(out,{recursive:true});
 const driver=path.join(data,'accessibility');execFileSync('swiftc',['-o',driver,path.join(root,'scripts/native-accessibility.swift')]);
 const suite='dev.pidesigner.review.'+Date.now();
 for(const category of ['completed','attention','failure'])execFileSync('defaults',['write',suite,'activity.'+category,'-bool','NO']);
 const executable=path.join(root,'build/πDesigner.app/Contents/MacOS/pidesigner-app');
 const child=spawn(executable,[],{env:{...process.env,PIDESIGNER_DATA_DIR:data,PIDESIGNER_PI_BIN:path.join(root,'scripts/fixtures/pi-rpc.cjs'),PIDESIGNER_PREFERENCES_SUITE:suite},stdio:['ignore','pipe','pipe']});
 let logs='';child.stderr.on('data',b=>logs+=b);const checks=[];const pass=name=>{checks.push(name);console.log('PASS',name)};
 const sleep=ms=>new Promise(r=>setTimeout(r,ms));
 const ax=(action,...args)=>execFileSync(driver,[String(child.pid),action,...args],{encoding:'utf8'});
 try{
  let base;for(let i=0;i<80;i++){try{base=JSON.parse(await fs.readFile(path.join(data,'runtime.json'),'utf8')).url;break;}catch{await sleep(150)}}assert.ok(base,'native daemon never became ready');await sleep(1700);
  async function api(route,body){const r=await fetch(base+route,body?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{});assert.ok(r.ok,route+': '+r.status);return r.json();}
  ax('click','Settings…'); let settingsTree='';
  for(let i=0;i<20;i++){await sleep(150);settingsTree=ax('dump');if(settingsTree.includes('Show notch activity when the workspace is in the background'))break;}
  assert.ok(settingsTree.includes('Show notch activity when the workspace is in the background'), 'native preferences bridge did not resolve: '+settingsTree);
  pass('Native preferences bridge exposes notification/notch settings without requesting permission');
  const project=await api('/api/projects',{name:'Native background review',skillId:'web-prototype'});
  const run=await api(`/api/projects/${project.project.id}/chat`,{prompt:'NATIVE_LONG_TASK native background',conversationId:project.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await sleep(1100);assert.ok(ax('windows').includes('πDesigner'));ax('close');await sleep(1300);
  assert.equal((await api('/api/runs/'+run.runId)).run.status,'running');
  let tree=ax('dump');await fs.writeFile(path.join(out,'native-background-tree.txt'),tree);
  assert.ok(tree.includes('Running command')&&tree.includes('Native background review'),tree);
  pass('Closing the native window keeps the daemon/task alive and shows current action in the notch');
  ax('click','πDesigner activity. Click to expand or collapse');await sleep(400);tree=ax('dump');
  assert.ok(tree.includes('Stop task')&&tree.includes('Open workspace'),tree);
  pass('Clicking the notch expands project status and task controls');
  const code=`import Cocoa; import CoreGraphics; import Foundation; let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly,.excludeDesktopElements],kCGNullWindowID) as? [[String:Any]] ?? []; let ids=windows.filter{($0[kCGWindowOwnerPID as String] as? Int)==${child.pid}}.compactMap{$0[kCGWindowNumber as String] as? Int}; print(ids.map(String.init).joined(separator:","))`;
  const ids=execFileSync('swift',['-e',code],{encoding:'utf8'}).trim().split(',').filter(Boolean);
  if(!ax('dump').includes('Open workspace')) { ax('click','πDesigner activity. Click to expand or collapse'); await sleep(150); }
  for(let i=0;i<ids.length;i++)execFileSync('screencapture',['-x','-l',ids[i],path.join(out,`native-notch-${i}.png`)]);
  ax('click','Show Workspace');await sleep(400);ax('minimize');await sleep(600);
  assert.equal((await api('/api/runs/'+run.runId)).run.status,'running');assert.ok(ax('dump').includes('Running command'));
  pass('Minimizing the native window retains background visibility');
  for(let i=0;i<80;i++){if((await api('/api/runs/'+run.runId)).run.status==='succeeded')break;await sleep(150)}
  assert.equal((await api('/api/runs/'+run.runId)).run.status,'succeeded');
  pass('The task completes while the workspace is minimized');
  await sleep(1300);ax('click','πDesigner activity');
  let menuTree=''; for(let i=0;i<20;i++){await sleep(100);menuTree=ax('dump');if(menuTree.includes('Open workspace'))break;}
  await fs.writeFile(path.join(out,'native-menu-tree.txt'),menuTree);
  assert.ok(menuTree.includes('Open workspace'),menuTree);
  pass('Menu-bar activity remains available after completion');
  ax('click','Show Workspace');await sleep(300);
  const fast=[];
  for(let n=0;n<2;n++) { const task=await api(`/api/projects/${project.project.id}/chat`,{prompt:'FAST_TASK notification cursor',conversationId:project.conversationId,model:'openai/gpt-fixture',thinking:'medium'});fast.push(task.runId);for(let i=0;i<60;i++){if((await api('/api/runs/'+task.runId)).run.status==='succeeded')break;await sleep(40);} }
  await sleep(1400);const ledger=execFileSync('defaults',['read',suite,'activity.delivered'],{encoding:'utf8'});for(const id of fast)assert.ok(ledger.includes(id),`missed rapid outcome ${id}`);
  pass('Rapid successive completions reach the native notification ledger without being overwritten by a newer run');
  const attention=await api(`/api/projects/${project.project.id}/chat`,{prompt:'ASK_EXTENSION',conversationId:project.conversationId,model:'openai/gpt-fixture',thinking:'medium'});
  await sleep(1000);ax('minimize');await sleep(1000);assert.ok(ax('dump').includes('Needs your answer'));
  await api('/api/runs/'+attention.runId+'/cancel',{});
  pass('Attention state is visible natively without relying on foreground WebKit');
  await fs.writeFile(path.join(out,'native-activity-report.json'),JSON.stringify({checks,notifications:'OS permission/banner delivery intentionally not requested by automation'},null,2));
 }finally{child.kill('SIGTERM');await new Promise(resolve=>{child.once('exit',resolve);setTimeout(()=>{child.kill('SIGKILL');resolve()},5000).unref()});execFileSync('defaults',['delete',suite]);await fs.writeFile(path.join(out,'native-activity.log'),logs);await fs.rm(data,{recursive:true,force:true});}
})().catch(e=>{console.error(e.stack);process.exitCode=1});
