import { useRef, useState } from 'preact/hooks';
import type { DesignSystem, Project } from '../api';
import { Composer } from '../components/Composer';
import { Icon } from '../components/Icon';
import { relativeDate } from '../components/common';
import { workflows, type Frame } from '../state';

const starters = [
  { title: 'A calmer place to get work done', description: 'A focused dashboard for projects, notes, and the week ahead.', type: 'dashboard', prompt: 'Build a focused project dashboard with a weekly overview, project list, and notes. Use a clean visual hierarchy and make every layout work on web and mobile.' },
  { title: 'An app worth coming back to', description: 'A considered mobile experience, from onboarding to everyday use.', type: 'mobile', prompt: 'Design a mobile habit-tracking app with a thoughtful onboarding flow, a daily view, and a weekly progress screen. Make the navigation and interactions work.' },
  { title: 'A little movement, a lot of character', description: 'An expressive portfolio with purposeful, fluid interactions.', type: 'motion', prompt: 'Build a personal portfolio with large typography, a project index, and subtle scroll and hover interactions. Respect reduced motion and keep animations smooth.' },
];
export function Home({ systems, systemId, setSystemId, skillId, setSkillId, frame, setFrame, onSystem, onStart, busy, projects, onProject, onProjects, error }: {
  systems: DesignSystem[]; systemId: string; setSystemId: (s: string) => void;
  skillId: string; setSkillId: (s: string) => void; frame: Frame; setFrame: (f: Frame) => void;
  onSystem: () => void; onStart: (prompt: string) => Promise<void>; busy: boolean;
  projects: Project[]; onProject: (id: string) => void; onProjects: () => void; error?: string;
}) {
  const [prompt, setPrompt] = useState('');
  const [url, setURL] = useState('');
  const [urlError, setURLError] = useState('');
  const examplesRef = useRef<HTMLElement>(null);
  const selected = systems.find((s) => s.id === systemId);
  async function submit() {
    if (skillId === 'web-clone') {
      try { const u = new URL(url); if (!['https:', 'http:'].includes(u.protocol)) throw new Error(); }
      catch { setURLError('Enter a full website address, such as https://example.com.'); return; }
    }
    setURLError('');
    await onStart(skillId === 'web-clone' ? `Rebuild the website at ${url}.\n\n${prompt}` : prompt);
  }
  return <div class="home-page">
    <section class="home-start" aria-labelledby="home-heading">
      <h1 id="home-heading">Good ideas deserve<br/>a place to <span class="highlight">take shape.</span></h1>
      <p class="home-subtitle">Your next website, app, or interaction starts here.</p>
      <Composer value={prompt} onChange={setPrompt} onSubmit={submit} busy={busy} disabled={busy} systemName={selected?.name} onSystem={onSystem} frame={frame} onFrame={setFrame}/>
      {skillId === 'web-clone' && <div class="clone-source"><label for="clone-url"><Icon name="clone" size={17}/>Reference website</label><input id="clone-url" type="url" value={url} onInput={(e) => setURL(e.currentTarget.value)} placeholder="https://example.com" aria-invalid={!!urlError}/>{urlError && <p class="field-error" role="alert">{urlError}</p>}</div>}
      <div class="workflow-choices" aria-label="Design workflow">{workflows.map((w) => <button key={w.id} class={`workflow-choice ${skillId === w.id ? 'selected' : ''}`} aria-pressed={skillId === w.id} onClick={() => setSkillId(w.id)} title={w.description}><Icon name={w.icon} size={17}/>{w.name}</button>)}</div>
      <p class="home-caption"><span class="status-dot"/>Built on your Mac. Powered by your omp agent.</p>
      {error && <p class="inline-error" role="alert">{error}</p>}
    </section>

    {projects.length > 0 ? <section class="home-recent" aria-label="Recent projects">
      <div class="section-heading"><h2>Pick up where you left off</h2><button class="text-button" onClick={onProjects}>All projects<Icon name="chevron" size={15}/></button></div>
      <div class="recent-projects">{projects.slice(0, 3).map((p) => <button key={p.id} class="recent-project" onClick={() => onProject(p.id)}><span class="recent-project-icon"><Icon name="folder"/></span><span><strong>{p.name}</strong><small>{relativeDate(p.updatedAt)}</small></span><Icon name="chevron" size={15}/></button>)}</div>
    </section> : null}

    <section class="starter-section" ref={examplesRef} aria-label="Example briefs">
      <div class="section-heading"><h2>A few places to start</h2><span>Make it your own</span></div>
      <div class="starter-list">{starters.map((s) => <button key={s.type} class="starter-row" onClick={() => { setPrompt(s.prompt); setSkillId(s.type === 'motion' ? 'motion-design' : 'web-prototype'); setFrame(s.type === 'mobile' ? 'mobile' : 'web'); document.getElementById('home-prompt')?.focus(); }}>
        <StarterPreview type={s.type}/><span class="starter-copy"><strong>{s.title}</strong><small>{s.description}</small></span><Icon name="plus" size={18}/>
      </button>)}</div>
    </section>
  </div>;
}
function StarterPreview({ type }: { type: string }) {
  return <span class={`starter-art starter-${type}`} aria-hidden="true">{type === 'dashboard' ? <><span class="mini-sidebar"/><span class="mini-topline"/><span class="mini-chart"><i/><i/><i/><i/><i/></span></> : type === 'mobile' ? <><span class="mini-phone"><i/><i/><i/></span><span class="mini-phone offset"><i/><i/><i/></span></> : <><span class="mini-word">Make<br/><em>moves.</em></span><span class="mini-orbit"/></>}</span>;
}
