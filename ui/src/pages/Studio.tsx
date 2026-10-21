import { useEffect, useRef, useState } from 'preact/hooks';
import { api, type Conversation, type DesignSystem, type FileEntry, type Message, type Project, type Run } from '../api';
import { Composer } from '../components/Composer';
import { Dialog, Empty, IconButton } from '../components/common';
import { Icon, Logo, PiLogo } from '../components/Icon';
import { RichText } from '../components/RichText';
import { PreviewPane, type FileChange } from '../components/PreviewPane';
import { SystemPicker } from '../components/SystemPicker';
import { skillNames, type Frame, type RunState } from '../state';

export function Studio({ projectId, systems, run, onMonitor, onStart, onHome, onError, initialFrame = 'web' }: {
  projectId: string; systems: DesignSystem[]; run?: RunState; onMonitor: (run: Run) => void;
  onStart: (p: Project, conversationId: string, prompt: string) => Promise<void>;
  onHome: () => void; onError: (s: string) => void; initialFrame?: Frame;
}) {
  const [project, setProject] = useState<Project | null>(null);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [changes, setChanges] = useState<FileChange[]>([]);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [conversationId, setConversationId] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [frame, setFrame] = useState<Frame>(initialFrame);
  const [prompt, setPrompt] = useState('');
  const [pickingSystem, setPickingSystem] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState('');
  const [failure, setFailure] = useState('');
  const [connecting, setConnecting] = useState(false);
  const messagesPane = useRef<HTMLDivElement>(null);
  const follow = useRef(true);
  const busy = run?.status === 'running';
  const monitor = useRef(onMonitor); monitor.current = onMonitor;

  useEffect(() => {
    let active = true;
    Promise.all([api.project(projectId), api.files(projectId), api.runs(projectId)]).then(async ([p, f, r]) => {
      if (!active) return;
      setProject(p.project); setFiles(f.files); setConversations(p.conversations);
      if (p.conversations[0]) setConversationId(p.conversations[0].id);
      const running = r.runs.find((r) => r.status === 'running');
      if (running) monitor.current(running);
    }).catch((e) => { if (active) setFailure(e.message); });
    return () => { active = false; };
  }, [projectId]);

  useEffect(() => {
    if (!conversationId) return;
    let active = true;
    api.messages(conversationId).then((d) => { if (active) setMessages(d.messages); }).catch((e) => { if (active) onError(e.message); });
    return () => { active = false; };
  }, [conversationId, run?.status]);

  useEffect(() => {
    let disposed = false, reconnect: number | undefined;
    let socket: WebSocket;
    const connect = () => {
      socket = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/projects/${encodeURIComponent(projectId)}/watch`);
      socket.onmessage = (e) => {
        try {
          const event = JSON.parse(e.data);
          if (event.type !== 'changed') return;
          setChanges(event.changes || []);
          api.files(projectId).then((d) => { if (!disposed) setFiles(d.files); }).catch(() => {});
        } catch { /* malformed or incomplete transport frame */ }
      };
      socket.onclose = () => { if (!disposed) reconnect = window.setTimeout(connect, 2000); };
    };
    connect();
    return () => { disposed = true; clearTimeout(reconnect); socket?.close(); };
  }, [projectId]);

  useEffect(() => {
    if (!busy) api.files(projectId).then((d) => setFiles(d.files)).catch(() => {});
  }, [busy]);
  useEffect(() => {
    const el = messagesPane.current;
    if (follow.current && el) el.scrollTop = el.scrollHeight;
  }, [messages, run?.text, run?.tools.length]);

  async function send() {
    if (!project || !prompt.trim() || busy) return;
    setConnecting(true);
    try {
      let id = conversationId;
      if (!id) {
        const c = await api.createConversation(project.id); id = c.conversation.id;
        setConversations((all) => [c.conversation, ...all]); setConversationId(id);
      }
      const text = prompt.trim();
      await onStart(project, id, text);
      setMessages((all) => [...all, { id: `local-${Date.now()}`, conversationId: id, role: 'user', content: text, createdAt: new Date().toISOString() }]);
      setPrompt(''); follow.current = true;
    } catch (e) { onError(String(e)); }
    finally { setConnecting(false); }
  }
  async function newConversation() {
    if (!project || busy) return;
    try { const c = await api.createConversation(project.id); setConversations((all) => [c.conversation, ...all]); setConversationId(c.conversation.id); setMessages([]); }
    catch (e) { onError(String(e)); }
  }
  if (failure) return <div class="studio-failure"><Empty icon="folder" title="This project couldn’t be opened"><p>{failure}</p><button class="button primary" onClick={onHome}>Back to workspace</button></Empty></div>;
  if (!project) return <div class="loading-state full">Opening your project…</div>;
  const system = systems.find((s) => s.id === project.designSystemId);
  const showLive = run?.conversationId === conversationId && busy;
  return <div class="studio-shell">
    <header class="studio-header"><IconButton icon="home" label="Back to home" onClick={onHome}/><Logo size={26}/><button class="project-title" onClick={() => { setName(project.name); setRenaming(true); }} title="Rename project" aria-label="Rename project">{project.name}<Icon name="chevron" size={14}/></button><span class="studio-workflow">{skillNames[project.skillId || ''] || 'Prototype'}</span><div class="studio-header-end"><span class="local-label"><span class="status-dot"/>Local workspace</span><button class="button secondary small" onClick={() => setPickingSystem(true)} disabled={busy}><Icon name="palette" size={16}/>{system?.name || 'Design system'}</button></div></header>
    <div class="studio-content"><section class="conversation-pane" aria-label="Design conversation"><div class="conversation-toolbar"><label><span class="sr-only">Conversation</span><select value={conversationId} disabled={busy} onChange={(e) => { setConversationId(e.currentTarget.value); follow.current = true; }}>{conversations.map((c, i) => <option key={c.id} value={c.id}>{c.title === 'Main' ? 'Design conversation' : `${c.title} ${conversations.length - i}`}</option>)}</select></label><IconButton icon="plus" label="New conversation" onClick={newConversation} disabled={busy}/></div>
      <div class="conversation-messages" ref={messagesPane} onScroll={(e) => { const el = e.currentTarget; follow.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100; }}>
        {!messages.length && !showLive && <div class="conversation-welcome"><Logo size={36}/><h2>Let’s make it yours.</h2><p>Describe the design you have in mind. We’ll build it here, together.</p></div>}
        {messages.map((m) => <article key={m.id} class={`message message-${m.role}`}><div class="message-byline">{m.role === 'assistant' ? <><PiLogo size={20}/><strong>Pi agent</strong></> : <span>You</span>}</div><RichText text={m.content}/></article>)}
        {showLive && <article class="message message-assistant live-message"><div class="message-byline"><PiLogo size={20}/><strong>Pi agent</strong><span class="generating-label">Working<span class="working-indicator"><i/><i/><i/></span></span></div>{run.text ? <RichText text={run.text}/> : <p class="thinking-label">Reading your brief and planning the design…</p>}</article>}
        {run?.model && run.conversationId === conversationId && <p class="run-model"><PiLogo size={14}/>Using {run.model}</p>}
        {run && run.tools.length > 0 && <details class="tool-log"><summary><Icon name={busy ? 'code' : 'check'} size={15}/>{busy ? `Pi is using ${run.tools[run.tools.length - 1].tool}` : `${run.tools.length} recent agent actions`}</summary><ul>{run.tools.map((event, i) => <li key={i}><code>{event.tool}</code><span>{String(event.args?.path || event.args?.command || '').slice(0, 140)}</span></li>)}</ul></details>}
        {run?.status === 'failed' && <div class="inline-error" role="alert"><strong>Pi couldn’t finish this run.</strong><p>{run.error || 'Check your Pi model configuration, then try again.'}</p></div>}
        {run?.status === 'canceled' && <p class="run-status">Generation stopped. Your existing files are unchanged unless Pi saved an edit before stopping.</p>}
        {run?.status === 'succeeded' && <p class="run-status"><Icon name="check" size={15}/>{files.some((file) => /\.html?$/i.test(file.path)) ? 'Finished. Your design is ready to explore.' : 'Finished. Response saved to this conversation.'}</p>}
      </div>
      <div class="studio-composer"><Composer compact value={prompt} onChange={setPrompt} onSubmit={send} busy={busy} disabled={connecting} onStop={() => { if (run) api.cancelRun(run.id).catch((e) => onError(e.message)); }} systemName={system?.name} onSystem={() => setPickingSystem(true)} frame={frame} onFrame={setFrame}/><p>Pi can edit project files and run commands on your Mac.</p></div>
    </section><PreviewPane project={project} files={files} frame={frame} onFrame={setFrame} changes={changes} busy={busy} onError={onError}/></div>
    {pickingSystem && <SystemPicker systems={systems} selected={project.designSystemId || ''} onClose={() => setPickingSystem(false)} onSelect={async (id) => { try { const d = await api.updateProject({ ...project, designSystemId: id }); setProject(d.project); setPickingSystem(false); } catch (e) { onError(String(e)); } }}/>}
    {renaming && <Dialog title="Rename project" onClose={() => setRenaming(false)}><form class="rename-form" onSubmit={async (e) => { e.preventDefault(); try { const d = await api.updateProject({ ...project, name }); setProject(d.project); setRenaming(false); } catch (e) { onError(String(e)); } }}><label>Project name<input autoFocus value={name} onInput={(e) => setName(e.currentTarget.value)} required/></label><button class="button primary" disabled={!name.trim()}>Save name</button></form></Dialog>}
  </div>;
}
