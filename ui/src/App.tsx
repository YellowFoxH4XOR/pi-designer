import { useCallback, useEffect, useState } from 'preact/hooks';
import { api, type DesignSystem, type Health, type Project, type Skill } from './api';
import { Home } from './pages/Home';
import { Projects, DesignSystems, Skills } from './pages/Library';
import { Settings } from './pages/Settings';
import { Studio } from './pages/Studio';
import { Icon, Logo, type IconName } from './components/Icon';
import { IconButton, Empty } from './components/common';
import { SystemPicker } from './components/SystemPicker';
import { useRoute, useRuns, type Frame, type Preferences } from './state';
import { useModels } from './model-context';

const navigation: { id: string; label: string; icon: IconName }[] = [
  { id: 'home', label: 'Home', icon: 'home' }, { id: 'projects', label: 'All projects', icon: 'projects' },
  { id: 'systems', label: 'Design systems', icon: 'palette' }, { id: 'skills', label: 'Skills', icon: 'skills' },
];
export function App() {
  const [route, navigate] = useRoute();
  const [projects, setProjects] = useState<Project[]>([]);
  const [systems, setSystems] = useState<DesignSystem[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [health, setHealth] = useState<Health | null>(null);
  const { preferences, ready: preferencesReady, save: saveModelPreferences } = useModels();
  const [systemId, setSystemId] = useState('');
  const [skillId, setSkillId] = useState('web-artifacts-builder');
  const [frame, setFrame] = useState<Frame>('web');
  const [pickingSystem, setPickingSystem] = useState(false);
  const [starting, setStarting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [connectionError, setConnectionError] = useState('');
  const [error, setError] = useState('');
  const report = useCallback((e: string) => setError(e.replace(/^Error: /, '')), []);
  const refreshProjects = useCallback(() => { api.projects().then((d) => setProjects(d.projects)).catch((e) => report(e.message)); }, [report]);
  const { runs, monitor } = useRuns(refreshProjects);
  const projectId = route.startsWith('project/') ? route.slice('project/'.length) : '';

  useEffect(() => {
    let active = true;
    Promise.all([api.projects(), api.systems(), api.skills(), api.health()]).then(([p, s, k, h]) => {
      if (!active) return;
      setProjects(p.projects); setSystems(s.designSystems); setSkills(k.skills); setHealth(h);
    }).catch((e) => { if (active) setConnectionError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);
  useEffect(() => { setError(''); }, [route]);
  useEffect(() => {
    const title = projectId ? projects.find((p) => p.id === projectId)?.name || 'Studio' : navigation.find((n) => n.id === route)?.label || 'Settings';
    document.title = `${title} — PiDesigner`;
  }, [route, projects]);

  async function startRun(project: Project, conversationId: string, prompt: string) {
    if (!preferencesReady) throw new Error('Pi model preferences are still loading. Try again in a moment.');
    if (runs[project.id]?.status === 'running') throw new Error('Pi is already working in this project. Stop that run before starting another.');
    const { runId } = await api.chat(project.id, { prompt, conversationId, designSystemId: project.designSystemId, skillId: project.skillId, model: preferences.model, thinking: preferences.thinking });
    monitor({ id: runId, projectId: project.id, conversationId, status: 'running', startedAt: new Date().toISOString() });
  }
  async function createAndStart(prompt: string) {
    if (starting) return;
    if (!health?.piBin) { report('Pi was not found. Open Settings to check the agent installation.'); return; }
    setStarting(true); setError('');
    try {
      const name = prompt.replace(/^(build|create|design)\s+(an?\s+)?/i, '').split(/[.!?\n]/)[0].trim().slice(0, 56) || 'Untitled prototype';
      const { project, conversationId } = await api.createProject({ name: name.charAt(0).toUpperCase() + name.slice(1), designSystemId: systemId, skillId });
      setProjects((all) => [project, ...all]);
      navigate(`project/${project.id}`);
      try { await startRun(project, conversationId, `${prompt}\n\nPrimary preview: ${frame}. Keep it responsive at all three supported device sizes.`); }
      catch (e) { report(`The project was created, but Pi couldn’t start. ${String(e).replace(/^Error: /, '')}`); }
    } catch (e) { report(String(e)); }
    finally { setStarting(false); }
  }
  function newProject() { navigate('home'); requestAnimationFrame(() => document.getElementById('home-prompt')?.focus()); }
  async function savePreferences(p: Preferences) {
    await saveModelPreferences(p);
  }
  const errorBanner = error && <div class="error-banner" role="alert"><span>{error}</span><IconButton icon="close" label="Dismiss message" onClick={() => setError('')}/></div>;
  if (projectId) return <>{errorBanner}<Studio key={projectId} projectId={projectId} systems={systems} run={runs[projectId]} onMonitor={monitor} onStart={startRun} onHome={() => { refreshProjects(); navigate('home'); }} onError={report} initialFrame={frame}/></>;

  return <div class="workspace">
    <aside id="workspace-sidebar" class="sidebar" aria-label="Workspace navigation"><a class="brand" href="#home"><Logo size={35}/><span>PiDesigner<small>Your local design studio</small></span></a>
      <button class="workspace-switch" onClick={() => navigate('settings')}><span class="workspace-avatar">P</span><span>Personal workspace<small>On this Mac</small></span><Icon name="chevron" size={15}/></button>
      <nav class="primary-nav">{navigation.map((n) => <a key={n.id} class={route === n.id ? 'active' : ''} href={`#${n.id}`} aria-current={route === n.id ? 'page' : undefined}><Icon name={n.icon} size={19}/>{n.label}{n.id === 'systems' && systems.length > 0 && <span class="nav-count">{systems.length}</span>}</a>)}</nav>
      <div class="sidebar-recents"><div class="sidebar-section-label"><span>Recent projects</span><IconButton icon="plus" label="Create a project" onClick={newProject}/></div>{projects.length ? projects.slice(0, 6).map((p) => <a key={p.id} href={`#project/${p.id}`} title={p.name}><Icon name="folder" size={17}/><span>{p.name}</span>{runs[p.id]?.status === 'running' && <span class="status-dot working"/>}</a>) : <p class="sidebar-hint">Your ideas will live here.</p>}</div>
      <div class="sidebar-bottom"><a href="#settings" class="connect-editor"><Icon name="connect" size={19}/><span>Connect your editor<small>Bring your workspace to MCP</small></span><Icon name="chevron" size={14}/></a><a href="#settings" class={`settings-link ${route === 'settings' ? 'active' : ''}`} aria-current={route === 'settings' ? 'page' : undefined}><Icon name="settings" size={18}/>Settings<span class="version-label">v0.1</span></a></div>
    </aside>
    <div class="workspace-body"><header class="workspace-header"><span>{navigation.find((n) => n.id === route)?.label || 'Settings'}</span><div class="workspace-health"><span class={`status-dot ${!health?.piBin ? 'offline' : ''}`}/>{loading ? 'Connecting…' : health?.piBin ? 'Pi detected' : 'Pi not found'}<span class="header-rule"/><span class="local-only">Local workspace</span></div></header>
      {errorBanner}
      <main class="workspace-canvas" id="main-content">{loading ? <div class="loading-state full">Opening your workspace…</div> : connectionError ? <Empty icon="connect" title="The workspace isn’t connected"><p>{connectionError}</p><button class="button primary" onClick={() => location.reload()}>Try again</button></Empty> : route === 'home' ? <Home systems={systems} systemId={systemId} setSystemId={setSystemId} skillId={skillId} setSkillId={setSkillId} frame={frame} setFrame={setFrame} onSystem={() => setPickingSystem(true)} onStart={createAndStart} busy={starting} projects={projects} onProject={(id) => navigate(`project/${id}`)} onProjects={() => navigate('projects')}/> : route === 'projects' ? <Projects projects={projects} onOpen={(id) => navigate(`project/${id}`)} onNew={newProject}/> : route === 'systems' ? <DesignSystems systems={systems} onUse={(id) => { setSystemId(id); newProject(); }} onError={report}/> : route === 'skills' ? <Skills skills={skills} onUse={(id) => { setSkillId(id); newProject(); }} onError={report}/> : <Settings health={health} preferences={preferences} onSave={savePreferences} onError={report}/>}</main>
      <footer class="workspace-footer"><Logo size={14}/><span>Made for the way you think.</span><span>Pi does the building. You make it yours.</span></footer>
    </div>
    {pickingSystem && <SystemPicker systems={systems} selected={systemId} onClose={() => setPickingSystem(false)} onSelect={(id) => { setSystemId(id); setPickingSystem(false); }}/>}
  </div>;
}
