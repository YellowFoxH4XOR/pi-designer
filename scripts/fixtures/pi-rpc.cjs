#!/usr/bin/env node
// Deterministic RPC fixture, never a model provider. Exercises the real daemon,
// store, stream, web UI and native sidecar without spending inference credits.
const fs = require('node:fs');
const path = require('node:path');
const readline = require('node:readline');
const argv = process.argv;
const arg = (name, fallback) => argv.includes(name) ? argv[argv.indexOf(name) + 1] : fallback;
const models = [
  { id: 'gpt-fixture', provider: 'openai', name: 'GPT fixture', reasoning: true },
  { id: 'claude-opus-fixture', provider: 'anthropic', name: 'Claude Opus fixture', reasoning: true },
  { id: 'deepseek-fixture', provider: 'opencode-go', name: 'DeepSeek fixture', reasoning: true },
  { id: 'custom-fixture', provider: 'opencode-go', name: 'OpenCode fixture', reasoning: false },
];
if (argv.includes('--version')) { console.log('fixture 1.0'); process.exit(0); }
const provider = arg('--provider', 'openai'); const id = arg('--model', 'gpt-fixture');
const model = models.find(m => m.id === id && m.provider === provider) || models[0];
const sessionDir = arg('--session-dir'); const sessionId = arg('--session-id');
if (sessionDir && sessionId) { fs.mkdirSync(sessionDir, { recursive: true }); fs.writeFileSync(path.join(sessionDir, `fixture_${sessionId}.jsonl`), ''); }
const send = obj => process.stdout.write(JSON.stringify(obj) + '\n');
const response = (req, data = {}) => send({ type: 'response', command: req.type, id: req.id, success: true, data });
let aborted = false; let active = false; let resolveUI;
const sleep = ms => new Promise(r => setTimeout(r, ms));
const message = text => {
  send({ type: 'message_start', message: { role: 'assistant' } });
  send({ type: 'message_update', assistantMessageEvent: { type: 'text_delta', delta: text } });
  send({ type: 'message_end', message: { role: 'assistant', stopReason: 'stop', content: [{ type: 'text', text }] } });
};
async function run(text) {
  active = true; aborted = false;
  send({ type: 'agent_start' });
  if (text.includes('ASK_EXTENSION')) {
    send({ type: 'extension_ui_request', id: 'fixture-confirm', method: 'confirm', title: 'Continue the fixture design?', message: 'This is a deterministic test request.' });
    await new Promise(resolve => { resolveUI = resolve; });
  }
  if (aborted) return;
  if (text.includes('ASK_FORM')) {
    message('<question-form id="fixture-question" title="Choose a direction">{"questions":[{"id":"direction","label":"Visual direction","type":"radio","options":["Quiet","Expressive"],"default":"Quiet"}]}</question-form>');
    send({ type: 'agent_end' }); send({ type: 'agent_settled', aborted: false }); active = false; return;
  }
  if (text.includes('FAIL_RUN')) {
    send({ type: 'message_end', message: { role: 'assistant', stopReason: 'error', errorMessage: 'Fixture provider failed', content: [] } });
    send({ type: 'agent_end' }); send({ type: 'agent_settled', aborted: false }); active = false; return;
  }
  message('Reading the project and building a focused interface.');
  if (text.includes('SCROLL_STRESS')) {
    message(Array.from({length:120},(_,i)=>`Review note ${i+1}: keep the conversation inside its own scroll area while the canvas and composer remain fixed.`).join('\n\n'));
    const todos=Array.from({length:60},(_,i)=>({content:`Design step ${i+1}: verify a detailed layout, interaction, and accessibility state.`,status:i<20?'completed':i===20?'in_progress':'pending'}));
    send({type:'tool_execution_start',toolCallId:'plan-stress',toolName:'todo_write',args:{todos}});
    send({type:'tool_execution_end',toolCallId:'plan-stress',toolName:'todo_write',result:{content:[{type:'text',text:'Plan recorded'}]},isError:false});
    const critique={round:1,composite:7.5,decision:'revise',panelists:['critic','brand','a11y','copy'].map(role=>({role,score:7.5,notes:'Review this fixture carefully. '.repeat(20),mustFix:['Keep long content inside the correct scroll panel.']}))};
    send({type:'tool_execution_start',toolCallId:'critique-stress',toolName:'submit_critique',args:{}});
    send({type:'tool_execution_end',toolCallId:'critique-stress',toolName:'submit_critique',result:{content:[{type:'text',text:'Critique recorded'}],details:critique},isError:false});
  }
  send({ type: 'auto_retry_start', attempt: 1, maxAttempts: 3, delayMs: 20, errorMessage: 'Fixture transient error' });
  await sleep(20); send({ type: 'auto_retry_end', success: true, attempt: 1 });
  send({ type: 'compaction_start', reason: 'threshold' }); send({ type: 'compaction_end', reason: 'threshold', aborted: false });
  send({ type: 'tool_execution_start', toolCallId: 'read-1', toolName: 'read', args: { path: 'index.html' } });
  send({ type: 'tool_execution_start', toolCallId: 'bash-1', toolName: 'bash', args: { command: 'printf "Checking design"' } });
  send({ type: 'tool_execution_update', toolCallId: 'bash-1', toolName: 'bash', partialResult: { content: [{ type: 'text', text: 'Checking…' }] } });
  send({ type: 'tool_execution_end', toolCallId: 'read-1', toolName: 'read', result: { content: [{ type: 'text', text: 'Read existing page' }] }, durationMs: 15, isError: false });
  const duration = text.includes('NATIVE_LONG_TASK') ? 12000 : text.includes('LONG_TASK') ? 7000 : text.includes('FAST_TASK') ? 0 : 350;
  await sleep(duration);
  if (aborted) return;
  send({ type: 'tool_execution_update', toolCallId: 'bash-1', toolName: 'bash', partialResult: { content: [{ type: 'text', text: 'Checking…\nChecks passed' }] } });
  send({ type: 'tool_execution_end', toolCallId: 'bash-1', toolName: 'bash', result: { content: [{ type: 'text', text: 'Checks passed' }] }, durationMs: duration, isError: false });
  const html = '<!doctype html><html lang="en"><meta charset="utf-8"><title>Activity review fixture</title><style>*{box-sizing:border-box}body{margin:0;background:#f2eee7;color:#313a32;font-family:Georgia,serif;padding:70px 8%}small{font:13px system-ui}h1{font-weight:400;font-size:clamp(44px,7vw,100px);line-height:1.05;letter-spacing:-3px;max-width:10ch}p{font:18px/1.7 system-ui;max-width:46ch}button{padding:15px 24px;border:0;background:#344936;color:white}</style><small>OBSERVABLE / STUDIO</small><h1>Good work.<br>In plain sight.</h1><p>A deterministic test artifact for checking live activity, reconnection, native status and project handoff.</p><button>Explore the work</button></html>';
  send({ type: 'tool_execution_start', toolCallId: 'write-1', toolName: 'write', args: { path: 'index.html' } });
  fs.writeFileSync('index.html', html);
  send({ type: 'tool_execution_end', toolCallId: 'write-1', toolName: 'write', result: { content: [{ type: 'text', text: 'Saved index.html' }] }, durationMs: 12, isError: false });
  process.stderr.write('Fixture diagnostic: preview files saved\n');
  message('<pd-done/>Fixture complete. The design is ready to explore.<pd-next>Refine the typography</pd-next>');
  send({ type: 'agent_end' }); send({ type: 'agent_settled', aborted: false }); active = false;
}
readline.createInterface({ input: process.stdin }).on('line', line => {
  let req; try { req = JSON.parse(line); } catch { return; }
  if (req.type === 'get_state') response(req, { model, isStreaming: active, thinkingLevel: 'medium' });
  else if (req.type === 'get_available_models') response(req, { models });
  else if (req.type === 'get_available_thinking_levels') response(req, { levels: model.reasoning ? ['off', 'medium', 'high'] : ['off'] });
  else if (req.type === 'prompt') { response(req, { disposition: 'started' }); void run(req.message); }
  else if (req.type === 'abort') { aborted = true; active = false; resolveUI?.(); send({ type: 'agent_settled', aborted: true }); response(req); }
  else if (req.type === 'extension_ui_response') { resolveUI?.(); resolveUI = undefined; }
  else response(req);
}).on('close', () => process.exit(0));
