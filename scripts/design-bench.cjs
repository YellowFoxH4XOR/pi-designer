#!/usr/bin/env node
// Design benchmark: real generations scored by an independent judge.
//
// Runs fixed briefs (scripts/bench/briefs.json) through a real πDesigner
// daemon and the installed pi, renders each result with pidesigner-render, and
// asks a separate judge model — no tools, no view of how the design was made —
// to score the screenshots on the Awwwards rubric. The weighted score is
// computed here, not by the judge. Uses real inference on your pi accounts.
//
//   node scripts/design-bench.cjs --model openai-codex/gpt-6.1-sol \
//     [--judge github-copilot/gemini-4-argon] [--only ceramics-launch,drowning-film]
//   node scripts/design-bench.cjs --judge provider/id --from <data dir of an earlier run>
//     re-renders and re-judges earlier generations without new ones.
//
// Output: .impeccable/bench/<timestamp>/{report.json,index.html,*.png}
const { spawn, execFile } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');

const root = path.resolve(__dirname, '..');
const arg = (name, fallback) => { const i = process.argv.indexOf(name); return i > 0 ? process.argv[i + 1] : fallback; };
const model = arg('--model');
const judge = arg('--judge', model);
const only = arg('--only') ? new Set(arg('--only').split(',')) : null;
const thinking = arg('--thinking', 'high');
const runMinutes = Number(arg('--minutes', '25'));
const from = arg('--from');
if (!model && !(from && judge)) { console.error('usage: design-bench.cjs --model provider/id [--judge provider/id] [--only a,b] | --judge provider/id --from <data dir>'); process.exit(2); }

const WEIGHTS = { design: 0.4, usability: 0.3, creativity: 0.2, content: 0.1 };
const JUDGE_PROMPT = `You are an Awwwards jury member. Score the design in the attached screenshots against the brief, as a site submitted for Site of the Day.
Criteria (0-10 each, one decimal): design (visual quality, typography, layout, art direction), usability (clarity, navigation, readability, responsive behaviour visible across the screenshots), creativity (originality of concept and signature moment, not a template), content (specific, credible copy and imagery for this brief).
Calibrate: 9-10 would win Site of the Day; 7-8 Honorable Mention; 5-6 competent but forgettable template work; 4 or less broken, generic or crudely executed.
Be specific and severe; do not reward effort. Reply with only JSON:
{"design":n,"usability":n,"creativity":n,"content":n,"verdict":"one sentence","strengths":["..."],"issues":["most important first, max 5"]}`;

const run = (file, args, opts = {}) => new Promise((resolve, reject) => {
  execFile(file, args, { maxBuffer: 16 << 20, timeout: 300_000, ...opts }, (err, stdout, stderr) => err && !stdout ? reject(new Error(stderr || err.message)) : resolve(stdout));
});
const sleep = ms => new Promise(r => setTimeout(r, ms));
// pi's print mode reads piped stdin, so it must be closed or pi waits forever.
const runPi = (args, cwd) => new Promise((resolve, reject) => {
  const p = spawn('pi', args, { cwd, stdio: ['ignore', 'pipe', 'pipe'] });
  let out = '', err = '';
  p.stdout.on('data', d => { out += d; }); p.stderr.on('data', d => { err += d; });
  const t = setTimeout(() => { p.kill(); reject(new Error('judge timed out')); }, 300_000);
  p.on('close', code => { clearTimeout(t); code === 0 ? resolve(out) : reject(new Error(err.slice(-400) || `pi exited ${code}`)); });
});

(async () => {
  const daemon = path.join(root, 'bin/pidesigner'), renderer = path.join(root, 'bin/pidesigner-render');
  for (const f of [daemon, renderer]) await fs.access(f).catch(() => { throw new Error(`Missing ${f}. Run ./scripts/build-mac.sh first.`); });
  const briefs = JSON.parse(await fs.readFile(path.join(__dirname, 'bench/briefs.json'), 'utf8')).filter(b => !only || only.has(b.id));
  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  const out = path.join(root, '.impeccable/bench', stamp);
  await fs.mkdir(out, { recursive: true });
  const data = from ? path.resolve(from) : await fs.mkdtemp(path.join(os.tmpdir(), 'pi-bench-'));
  console.log(`data: ${data}`);
  const child = spawn(daemon, ['--port', '0', '--print-url', '--data', data, '--content', path.join(root, 'content'), '--ui', path.join(root, 'ui/dist')], { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, PIDESIGNER_JURY_MODEL: judge } });
  // Drain stderr so a verbose daemon cannot block on a full pipe.
  child.stderr.on('data', c => process.stderr.write(c));
  const base = await new Promise((resolve, reject) => {
    let s = ''; const t = setTimeout(() => reject(new Error('daemon startup timeout')), 20000);
    child.stdout.on('data', c => { s += c; const m = s.match(/PIDESIGNER_READY (\S+)/); if (m) { clearTimeout(t); resolve(m[1]); } });
  });
  const api = async (route, body) => {
    const r = await fetch(base + route, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {});
    const j = await r.json(); if (!r.ok) throw new Error(`${route}: ${j.error || r.status}`); return j;
  };
  const results = [];
  try {
    for (const brief of briefs) {
      const started = Date.now();
      console.log(`▶ ${brief.id}`);
      const result = { id: brief.id, prompt: brief.prompt, model, judge, dataDir: data };
      // Persist the in-flight run too, so an interruption leaves a recovery trail.
      await fs.writeFile(path.join(out, 'report.json'), JSON.stringify({ model, judge, thinking, results: [...results, result] }, null, 2));
      try {
        // Projects are named after the brief id, so their folder is data/projects/<id>.
        const projectDir = path.join(data, 'projects', brief.id);
        if (!from) {
        const { project, conversationId } = await api('/api/projects', { name: brief.id, skillId: brief.skill });
        const { runId } = await api(`/api/projects/${project.id}/chat`, { conversationId, prompt: brief.prompt, model, thinking, skipDiscovery: true, viewport: brief.viewport });
        result.projectId = project.id;
        result.runId = runId;
        await fs.writeFile(path.join(out, 'report.json'), JSON.stringify({ model, judge, thinking, results: [...results, result] }, null, 2));
        let runState;
        for (;;) {
          ({ run: runState } = await api(`/api/runs/${runId}`));
          if (runState.status !== 'running') break;
          if (Date.now() - started > runMinutes * 60_000) { await api(`/api/runs/${runId}/cancel`, {}); throw new Error(`timed out after ${runMinutes} min`); }
          await sleep(5000);
        }
        const { activity } = await api(`/api/runs/${runId}/activity`);
        result.status = runState.status; result.error = runState.error;
        result.minutes = +((Date.now() - started) / 60000).toFixed(1);
        result.tools = activity.tools.map(t => t.tool);
        result.critiques = runState.critiques || [];
        const { project: p } = await api(`/api/projects/${project.id}`);
        result.entry = p.entryFile || 'index.html';
        }
        const entry = path.join(projectDir, result.entry || 'index.html');
        await fs.access(entry);
        const shots = brief.motion
          ? [1500, 5000, 9000, 14000].map(at => ({ name: `${brief.id}-t${at}`, args: ['--width', '390', '--height', '844', '--at', String(at)] }))
          : [{ name: `${brief.id}-desktop`, args: ['--width', '1440', '--height', '900', '--wait', '1500'] },
             { name: `${brief.id}-full`, args: ['--width', '1440', '--height', '900', '--full-page', '--wait', '1500'] },
             { name: `${brief.id}-mobile`, args: ['--width', '390', '--height', '844', '--wait', '1500'] }];
        result.shots = [];
        for (const s of shots) {
          const png = path.join(out, `${s.name}.png`);
          await run(renderer, ['--url', 'file://' + entry, '--out', png, ...s.args]);
          result.shots.push(path.basename(png));
        }
        // The judge sees only the brief and the screenshots.
        const judged = await runPi(['-p', '--no-session', '--no-tools', '--no-skills', '--no-context-files', '--no-mcp', '--no-approve', '--model', judge,
          '--system-prompt', JUDGE_PROMPT, ...result.shots.map(s => '@' + path.join(out, s)), `Brief: ${brief.prompt}`], out);
        const json = JSON.parse(judged.slice(judged.indexOf('{'), judged.lastIndexOf('}') + 1));
        result.scores = Object.fromEntries(Object.keys(WEIGHTS).map(k => {
          const score = json[k];
          if (typeof score !== 'number' || !Number.isFinite(score) || score < 0 || score > 10) throw new Error(`Invalid judge score for ${k}`);
          return [k, score];
        }));
        result.overall = +Object.entries(WEIGHTS).reduce((sum, [k, w]) => sum + w * result.scores[k], 0).toFixed(2);
        Object.assign(result, { verdict: json.verdict, strengths: json.strengths, issues: json.issues });
        console.log(`  ${result.overall}/10 · ${result.verdict}`);
      } catch (e) {
        result.status = result.status || 'error'; result.error = String(e.message || e);
        console.log(`  ✗ ${result.error}`);
      }
      results.push(result);
      await fs.writeFile(path.join(out, 'report.json'), JSON.stringify({ model, judge, thinking, results }, null, 2));
    }
  } finally {
    child.kill();
  }
  const scored = results.filter(r => r.overall !== undefined);
  const mean = scored.length ? +(scored.reduce((s, r) => s + r.overall, 0) / scored.length).toFixed(2) : null;
  const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
  await fs.writeFile(path.join(out, 'index.html'), `<!doctype html><meta charset="utf-8"><title>Design benchmark</title>
<style>body{font:14px/1.5 -apple-system,sans-serif;margin:32px;background:#f4f5f7;color:#24272e}h1{font-weight:600}section{background:#fff;border:1px solid #dce0e6;border-radius:12px;padding:20px;margin:0 0 20px}.shots{display:flex;gap:10px;overflow-x:auto}.shots img{height:320px;border:1px solid #dce0e6;border-radius:6px}.score{font-size:28px;font-weight:600}small{color:#626873}</style>
<h1>Design benchmark</h1><p>Generator <b>${esc(model)}</b> · judge <b>${esc(judge)}</b> · mean <b>${mean ?? '—'}</b>/10 (Design 40 · Usability 30 · Creativity 20 · Content 10)</p>
${results.map(r => `<section><h2>${esc(r.id)} <span class="score">${r.overall ?? '—'}</span></h2><p>${esc(r.prompt)}</p>
${r.scores ? `<p><small>design ${r.scores.design} · usability ${r.scores.usability} · creativity ${r.scores.creativity} · content ${r.scores.content} · ${r.minutes} min · critique rounds ${esc(JSON.stringify((r.critiques || []).map(c => ({ round: c.round, panel: c.selfComposite ?? c.composite, jury: c.jury?.overall, decision: c.decision }))))}</small></p><p><b>${esc(r.verdict)}</b></p><ul>${(r.issues || []).map(i => `<li>${esc(i)}</li>`).join('')}</ul>` : `<p>${esc(r.error)}</p>`}
${r.critiques?.length ? `<details><summary>Per-round jury feedback</summary>${r.critiques.map(c => `<h3>Round ${esc(c.round)} · ${esc(c.decision)}</h3><p>Panel ${esc(c.selfComposite ?? c.composite)} · jury ${esc(c.jury?.overall ?? 'unavailable')}</p><p>${esc(c.jury?.verdict)}</p><ul>${(c.jury?.issues || []).map(i => `<li>${esc(i)}</li>`).join('')}</ul>`).join('')}</details>` : ''}
<div class="shots">${(r.shots || []).map(s => `<img src="${esc(s)}" alt="">`).join('')}</div></section>`).join('')}`);
  console.log(`\nMean ${mean ?? '—'}/10 over ${scored.length}/${results.length} briefs\n${path.join(out, 'index.html')}`);
})().catch(e => { console.error(e); process.exit(1); });
