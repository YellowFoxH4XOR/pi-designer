import { useEffect, useMemo, useState } from 'preact/hooks';
import { api, type DesignSystem, type Project, type Skill, type SystemContext } from '../api';
import { Empty, SearchField, relativeDate, CopyButton } from '../components/common';
import { Icon } from '../components/Icon';
import { SystemBadge, systemColors } from '../components/SystemPicker';
import { RichText } from '../components/RichText';
import { skillNames, workflows } from '../state';

export function Projects({ projects, onOpen, onNew }: { projects: Project[]; onOpen: (s: string) => void; onNew: () => void }) {
  const [search, setSearch] = useState('');
  const filtered = projects.filter((p) => p.name.toLowerCase().includes(search.toLowerCase()));
  return <div class="library-page">
    <div class="page-heading"><div><h1>All projects</h1><p>A place for every idea, and every iteration.</p></div><button class="button primary" onClick={onNew}><Icon name="plus" size={17}/>New project</button></div>
    <div class="library-toolbar"><span>{projects.length} {projects.length === 1 ? 'project' : 'projects'}</span><SearchField value={search} onInput={setSearch} placeholder="Search projects"/></div>
    {!filtered.length ? <Empty icon="projects" title={search ? 'No projects found' : 'A clean canvas awaits'}><p>{search ? 'Try another project name.' : 'Start with a brief. Pi will take it from there.'}</p>{!search && <button class="button primary" onClick={onNew}>Create a project<Icon name="plus" size={17}/></button>}</Empty> : <div class="project-grid">{filtered.map((p) => <button key={p.id} class="project-card" onClick={() => onOpen(p.id)}>
      <div class="project-cover"><Icon name={p.skillId === 'web-clone' ? 'clone' : p.skillId === 'motion-design' ? 'motion' : 'web'} size={36}/><span>{p.entryFile ? 'Open prototype' : 'Ready for your first brief'}<Icon name="chevron" size={14}/></span></div>
      <div class="project-card-meta"><strong>{p.name}</strong><small>{relativeDate(p.updatedAt)}</small></div><p>{skillNames[p.skillId || ''] || 'Prototype'}{p.designSystemId ? ` · ${p.designSystemId.replace(/-/g, ' ')}` : ''}</p>
    </button>)}</div>}
  </div>;
}

export function DesignSystems({ systems, onUse, onError }: { systems: DesignSystem[]; onUse: (id: string) => void; onError: (s: string) => void }) {
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('All categories');
  const [selected, setSelected] = useState<DesignSystem | null>(null);
  const categories = useMemo(() => ['All categories', ...new Set(systems.map((s) => s.category || 'Other'))].sort((a, b) => a === 'All categories' ? -1 : b === 'All categories' ? 1 : a.localeCompare(b)), [systems]);
  const filtered = systems.filter((s) => `${s.name} ${s.description} ${s.id}`.toLowerCase().includes(search.toLowerCase()) && (category === 'All categories' || (s.category || 'Other') === category));
  if (selected) return <SystemDetail system={selected} onBack={() => setSelected(null)} onUse={onUse} onError={onError}/>;
  return <div class="library-page systems-page">
    <div class="page-heading"><div><h1>Design systems</h1><p>Start with a visual language. Make something unmistakably yours.</p></div><span class="count-label">{systems.length} bundled systems</span></div>
    <div class="library-toolbar"><label class="category-select"><Icon name="palette" size={16}/><select value={category} aria-label="Design system category" onChange={(e) => setCategory(e.currentTarget.value)}>{categories.map((c) => <option key={c}>{c}</option>)}</select></label><SearchField value={search} onInput={setSearch} placeholder="Search design systems"/></div>
    <p class="result-count">{filtered.length} {filtered.length === 1 ? 'system' : 'systems'}{search && ` matching “${search}”`}<span class="catalog-preview-note">Preview samples · open a system for its exact tokens and components</span></p>
    {!filtered.length ? <Empty icon="search" title="No matching systems"><p>Try a different name or category.</p><button class="text-button" onClick={() => { setSearch(''); setCategory('All categories'); }}>Clear filters</button></Empty> : <div class="systems-grid">{filtered.map((s) => <button key={s.id} class="system-card" onClick={() => setSelected(s)} style={{ '--brand-color': systemColors[s.id] || '#656185' }}>
      <div class="system-card-art"><span class="specimen-letter">Aa</span><span class="specimen-lines"><i/><i/><i/></span><span class="specimen-button"/></div>
      <div class="system-card-label"><SystemBadge system={s}/><span><strong>{s.name}</strong><small>{s.category || 'Design system'}</small></span><Icon name="chevron" size={16}/></div>
    </button>)}</div>}
  </div>;
}
function SystemDetail({ system, onBack, onUse, onError }: { system: DesignSystem; onBack: () => void; onUse: (s: string) => void; onError: (s: string) => void }) {
  const [data, setData] = useState<(SystemContext & { componentsHtml?: string }) | null>(null);
  const [failure, setFailure] = useState('');
  const [tab, setTab] = useState('Overview');
  useEffect(() => { let active = true; api.system(system.id).then((d) => { if (active) setData(d); }).catch((e) => { if (active) setFailure(e.message); }); return () => { active = false; }; }, [system.id]);
  const colors = [...new Set(data?.tokensCss?.match(/#[a-f\d]{6}\b/gi) || [])].slice(0, 9);
  return <div class="library-page system-detail">
    <button class="text-button back-link" onClick={onBack}><Icon name="back" size={17}/>All design systems</button>
    <div class="page-heading"><div class="system-title"><SystemBadge system={system} size="large"/><div><h1>{system.name}</h1><p>{system.category || 'Bundled design system'}</p></div></div><button class="button primary" onClick={() => onUse(system.id)}>Use this system<Icon name="arrow" size={17}/></button></div>
    {system.description && <p class="system-description">{system.description}</p>}
    <div class="tabs detail-tabs" role="tablist" aria-label="Design system details">{['Overview', 'Guidelines', 'Tokens'].map((t) => <button key={t} role="tab" id={`system-tab-${t}`} aria-controls="system-content" aria-selected={t === tab} class={t === tab ? 'active' : ''} tabIndex={t === tab ? 0 : -1} onKeyDown={(e) => { const tabs = ['Overview', 'Guidelines', 'Tokens']; const index = tabs.indexOf(tab); let next = index; if (e.key === 'ArrowRight') next = (index + 1) % tabs.length; else if (e.key === 'ArrowLeft') next = (index + tabs.length - 1) % tabs.length; else if (e.key === 'Home') next = 0; else if (e.key === 'End') next = tabs.length - 1; else return; e.preventDefault(); setTab(tabs[next]); requestAnimationFrame(() => document.getElementById(`system-tab-${tabs[next]}`)?.focus()); }} onClick={() => setTab(t)}>{t}</button>)}</div>
    {failure ? <p class="inline-error" role="alert">{failure}</p> : !data ? <div class="loading-state">Loading design system…</div> : <section id="system-content" role="tabpanel" aria-labelledby={`system-tab-${tab}`} class="system-content">
      {tab === 'Overview' && <><div class="system-overview-grid"><div><h2>A ready-to-use foundation</h2><p>Pi reads this system’s brand guidelines and semantic tokens when it builds your prototype.</p><dl class="system-facts"><div><dt>Brand contract</dt><dd>DESIGN.md</dd></div><div><dt>Semantic tokens</dt><dd>tokens.css</dd></div><div><dt>Source</dt><dd>{system.source || 'Bundled package'}</dd></div></dl>{colors.length > 0 && <><h3>From the token palette</h3><div class="swatches">{colors.map((color) => <div key={color}><span style={{ background: color }}/><code>{color.toUpperCase()}</code></div>)}</div></>}</div><div class="type-specimen"><span>Aa</span><p>{system.name}</p><small>The full component language is shown below.</small></div></div>
        {data.componentsHtml && <section class="component-section"><h2>Component preview</h2><iframe sandbox="" title={`${system.name} component preview`} src={`/api/design-systems/${encodeURIComponent(system.id)}/preview/components.html`}/></section>}</>}
      {tab === 'Guidelines' && <RichText text={data.designMd} class="guidelines"/>}
      {tab === 'Tokens' && <><div class="token-heading"><p>The exact CSS context passed to Pi.</p><CopyButton text={data.tokensCss || ''} label="Copy CSS" onError={onError}/></div><pre class="source-display"><code>{data.tokensCss || 'This package has no compiled tokens.'}</code></pre></>}
    </section>}
  </div>;
}

const skillDescriptions: Record<string, string> = {
  'frontend-design': 'Always on: plan a distinctive direction, review it against generic defaults, then critique the build.',
  'motion-design': 'Shape animation, timing, typography, and visual continuity.',
  'web-clone': 'Inspect a reference website and rebuild its structure and interactions.',
  'emilkowalski-motion': 'Refine the feel of interface transitions and micro-interactions.',
  'web-design-guidelines': 'Review an interface for usability, accessibility, and visual hierarchy.',
  'agent-browser': 'Let Pi inspect and verify a prototype in the browser.',
};
export function Skills({ skills, onUse, onError }: { skills: Skill[]; onUse: (s: string) => void; onError: (s: string) => void }) {
  const [document, setDocument] = useState<{ title: string; content: string } | null>(null);
  return <div class="library-page"><div class="page-heading"><div><h1>A skill for the task</h1><p>Focused workflows for your installed Pi agent. No extra engine required.</p></div><span class="count-label">{skills.length} installed skills</span></div>
    {document ? <><button class="text-button back-link" onClick={() => setDocument(null)}><Icon name="back" size={17}/>Back to skills</button><RichText text={document.content} class="guidelines"/></> : <div class="skills-list">{skills.map((s) => <article class="skill-row" key={s.id}><span class="skill-icon"><Icon name={workflows.find((w) => w.id === s.id)?.icon || 'skills'} size={24}/></span><div><h2>{skillNames[s.id] || s.name}</h2><p>{skillDescriptions[s.id] || s.description}</p><button class="text-button" onClick={async () => { try { const d = await api.skillDocument(s.id); setDocument({ title: s.name, content: d.content }); } catch (e) { onError(String(e)); } }}>Read workflow<Icon name="chevron" size={14}/></button></div><button class="button secondary" onClick={() => onUse(s.id)}>Use skill<Icon name="arrow" size={16}/></button></article>)}</div>}
  </div>;
}
