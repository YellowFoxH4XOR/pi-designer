import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import {
  api,
  subscribeRun,
  type DesignSystem,
  type FileEntry,
  type Message,
  type Project,
  type Skill,
} from './api';

type Frame = 'web' | 'mobile' | 'desktop';

const FRAMES: Record<Frame, { w: number; h: number; label: string; scale: number }> = {
  web: { w: 1440, h: 900, label: 'Web · 1440', scale: 1 },
  desktop: { w: 1280, h: 800, label: 'Desktop · 1280', scale: 1 },
  mobile: { w: 390, h: 844, label: 'Mobile · 390', scale: 1 },
};

export function App() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [project, setProject] = useState<Project | null>(null);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [systems, setSystems] = useState<DesignSystem[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversationId, setConversationId] = useState<string>('');
  const [frame, setFrame] = useState<Frame>('web');
  const [streaming, setStreaming] = useState('');
  const [busy, setBusy] = useState(false);
  const [toolActivity, setToolActivity] = useState<string[]>([]);
  const [showNew, setShowNew] = useState(false);
  const [newName, setNewName] = useState('');
  const [newSystem, setNewSystem] = useState('');
  const [newSkill, setNewSkill] = useState('web-artifacts-builder');
  const [entry, setEntry] = useState('index.html');
  const [error, setError] = useState('');

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);

  // ------------------------------------------------------------ bootstrap
  useEffect(() => {
    (async () => {
      try {
        const [p, s, k] = await Promise.all([
          api.listProjects(),
          api.listDesignSystems(),
          api.listSkills(),
        ]);
        setProjects(p.projects);
        setSystems(s.designSystems);
        setSkills(k.skills);
        if (p.projects.length > 0) await selectProject(p.projects[0]);
      } catch (e: any) {
        setError(String(e.message ?? e));
      }
    })();
  }, []);

  const selectProject = useCallback(async (proj: Project) => {
    setProject(proj);
    setStreaming('');
    setToolActivity([]);
    setError('');
    const [f, c] = await Promise.all([
      api.listFiles(proj.id),
      api.listConversations(proj.id),
    ]);
    setFiles(f.files);
    const conv = c.conversations[0];
    if (conv) {
      setConversationId(conv.id);
      const m = await api.listMessages(conv.id);
      setMessages(m.messages);
    } else {
      setConversationId('');
      setMessages([]);
    }
    setEntry(proj.entryFile || 'index.html');
  }, []);

  // ------------------------------------------------------- file watching
  const refreshFiles = useCallback(async () => {
    if (!project) return;
    const f = await api.listFiles(project.id);
    setFiles(f.files);
  }, [project]);

  // Live preview: watch the project and hot-swap stylesheets in place.
  useEffect(() => {
    if (!project) return;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/api/projects/${project.id}/watch`);
    ws.onmessage = (ev) => {
      let msg: any;
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (msg.type !== 'changed') return;
      const changes: any[] = msg.changes ?? [];
      const css = changes.filter((c) => c.kind === 'css');
      const html = changes.filter((c) => c.kind === 'html');
      const frame = iframeRef.current;
      const win = frame?.contentWindow;
      if (win) {
        if (html.length > 0) {
          win.postMessage({ source: 'pidesigner', type: 'reload' }, '*');
        } else if (css.length > 0) {
          win.postMessage(
            {
              source: 'pidesigner',
              type: 'css-update',
              files: css.map((c) => ({ path: c.path, name: c.path.split('/').pop() })),
            },
            '*'
          );
        }
      }
      refreshFiles();
    };
    return () => ws.close();
  }, [project, refreshFiles]);

  // ------------------------------------------------------------- sending
  const send = useCallback(async () => {
    if (!project) return;
    const prompt = newName.trim();
    if (!prompt) return;
    setBusy(true);
    setStreaming('');
    setToolActivity([]);
    setNewName('');
    setError('');

    setMessages((m) => [
      ...m,
      { id: `local-${Date.now()}`, conversationId, role: 'user', content: prompt, createdAt: new Date().toISOString() },
    ]);

    try {
      const res = await api.chat(project.id, {
        prompt,
        conversationId,
        designSystemId: project.designSystemId,
        skillId: project.skillId,
      });
      const convId = res.conversationId || conversationId;
      if (!conversationId && convId) setConversationId(convId);

      let accumulated = '';
      const unsubscribe = subscribeRun(
        res.runId,
        (ev) => {
          if (ev.type === 'text') {
            accumulated += ev.text;
            setStreaming(accumulated);
          } else if (ev.type === 'tool') {
            setToolActivity((a) => [...a.slice(-6), `→ ${ev.tool}`]);
          } else if (ev.type === 'turn') {
            accumulated += '\n\n';
            setStreaming(accumulated);
          } else if (ev.type === 'error') {
            setError(ev.text ?? 'run failed');
          }
        },
        () => {
          setBusy(false);
          setStreaming('');
          if (accumulated) {
            setMessages((m) => [
              ...m,
              {
                id: `a-${Date.now()}`,
                conversationId: convId,
                role: 'assistant',
                content: accumulated,
                createdAt: new Date().toISOString(),
              },
            ]);
            setStreaming('');
          }
          refreshFiles();
        }
      );
      // Keep the unsubscribe handle so a component unmount can clean up.
      (window as any).__pdUnsub = unsubscribe;
    } catch (e: any) {
      setBusy(false);
      setError(String(e.message ?? e));
    }
  }, [project, conversationId, newName, refreshFiles]);

  const createProject = useCallback(async () => {
    if (!newName.trim()) return;
    try {
      const res = await api.createProject({
        name: newName.trim(),
        designSystemId: newSystem || undefined,
        skillId: newSkill || undefined,
      });
      setShowNew(false);
      setNewName('');
      const p = await api.listProjects();
      setProjects(p.projects);
      await selectProject(res.project);
    } catch (e: any) {
      setError(String(e.message ?? e));
    }
  }, [newName, newSystem, newSkill, selectProject]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streaming]);

  const previewSrc = useMemo(() => {
    if (!project) return '';
    return `/preview/${project.id}/${entry}?v=${project.updatedAt}`;
  }, [project, entry]);

  const currentSystem = systems.find((s) => s.id === project?.designSystemId);
  const currentSkill = skills.find((s) => s.id === project?.skillId);

  return (
    <div class="app">
      <header class="topbar">
        <div class="brand">
          <span class="mark" />
          <span>pidesigner</span>
        </div>

        <select
          class="select"
          value={project?.id ?? ''}
          onChange={(e) => {
            const p = projects.find((x) => x.id === (e.target as HTMLSelectElement).value);
            if (p) selectProject(p);
          }}
        >
          {projects.length === 0 && <option value="">no projects</option>}
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>

        <button class="btn" onClick={() => setShowNew(true)}>
          New project
        </button>

        <div class="meta">
          {currentSystem && (
            <span class="chip" title="Active design system">
              ◆ {currentSystem.name}
            </span>
          )}
          {currentSkill && <span class="chip chip-skill">/{currentSkill.name}</span>}
        </div>

        <div class="frames">
          {(Object.keys(FRAMES) as Frame[]).map((f) => (
            <button
              key={f}
              class={frame === f ? 'frame-btn active' : 'frame-btn'}
              onClick={() => setFrame(f)}
              title={FRAMES[f].label}
            >
              {f}
            </button>
          ))}
        </div>
      </header>

      {error && <div class="banner" onClick={() => setError('')}>{error} ×</div>}

      <main class="body">
        <aside class="panel files">
          <div class="panel-head">
            <span>Files</span>
            <button class="mini" onClick={refreshFiles}>
              ⟳
            </button>
          </div>
          <div class="file-list">
            {files.map((f) => (
              <button
                key={f.path}
                class={f.path === entry ? 'file active' : 'file'}
                onClick={() => setEntry(f.path)}
                title={f.path}
              >
                <span class={['dot', `dot-${f.kind}`]} />
                <span class="file-name">{f.path}</span>
                <span class="file-size">{f.size > 1024 ? `${(f.size / 1024).toFixed(1)}k` : f.size}</span>
              </button>
            ))}
            {files.length === 0 && <div class="empty">no files yet</div>}
          </div>
        </aside>

        <section class="panel chat">
          <div class="panel-head">
            <span>Chat</span>
            {busy && <span class="pulse">running</span>}
          </div>

          <div class="messages">
            {messages.map((m) => (
              <div key={m.id} class={['msg', `msg-${m.role}`]}>
                <div class="msg-role">{m.role === 'user' ? 'you' : 'agent'}</div>
                <div class="msg-body">{m.content}</div>
              </div>
            ))}
            {streaming && (
              <div class="msg msg-assistant">
                <div class="msg-role">agent</div>
                <div class="msg-body">
                  {streaming}
                  <span class="caret" />
                </div>
              </div>
            )}
            {toolActivity.length > 0 && (
              <div class="tool-activity">
                {toolActivity.map((t, i) => (
                  <div key={i}>{t}</div>
                ))}
              </div>
            )}
            <div ref={chatEndRef} />
          </div>

          <div class="composer">
            <textarea
              class="input"
              rows={3}
              placeholder={
                project
                  ? 'Describe the design to build…  (⌘↵ to send)'
                  : 'Create a project to start'
              }
              value={newName}
              disabled={!project || busy}
              onInput={(e) => setNewName((e.target as HTMLTextAreaElement).value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault();
                  send();
                }
              }}
            />
            <button class="btn primary" onClick={send} disabled={!project || busy || !newName.trim()}>
              {busy ? 'Working…' : 'Send'}
            </button>
          </div>
        </section>

        <section class="panel preview">
          <div class="panel-head">
            <span>Preview</span>
            <span class="entry-path">{entry}</span>
          </div>
          <div class={['stage', `stage-${frame}`]}>
            <div class="device" style={{ width: FRAMES[frame].w, height: FRAMES[frame].h }}>
              <iframe
                ref={iframeRef}
                src={previewSrc}
                title="preview"
                sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
              />
            </div>
          </div>
        </section>
      </main>

      {showNew && (
        <div class="modal-backdrop" onClick={() => setShowNew(false)}>
          <div class="modal" onClick={(e) => e.stopPropagation()}>
            <h2>New project</h2>
            <label>
              Name
              <input
                class="input"
                value={newName}
                autoFocus
                onInput={(e) => setNewName((e.target as HTMLInputElement).value)}
                onKeyDown={(e) => e.key === 'Enter' && createProject()}
              />
            </label>
            <label>
              Design system
              <select
                class="select wide"
                value={newSystem}
                onChange={(e) => setNewSystem((e.target as HTMLSelectElement).value)}
              >
                <option value="">none</option>
                {systems.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} — {s.category}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Skill
              <select
                class="select wide"
                value={newSkill}
                onChange={(e) => setNewSkill((e.target as HTMLSelectElement).value)}
              >
                <option value="">none</option>
                {skills.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                    {s.description ? ` — ${s.description.slice(0, 48)}…` : ''}
                  </option>
                ))}
              </select>
            </label>
            <div class="modal-actions">
              <button class="btn" onClick={() => setShowNew(false)}>
                Cancel
              </button>
              <button class="btn primary" onClick={createProject} disabled={!newName.trim()}>
                Create
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
