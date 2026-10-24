import type { ComponentChildren } from 'preact';
import { RunActivity, RunProgress } from '../components/Activity';
import { fromActivity, type RunState as ActivityRun } from '../activity';
import { ModelIcon } from '../components/ModelIcon';
import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api, type ArtifactVersion, type Conversation, type DesignSystem, type FileEntry, type Message, type Project, type Render, type Run } from '../api';
import { ArtifactCard, ArtifactCardLive } from '../components/ArtifactCard';
import { AgentMessage } from '../components/AgentMessage';
import { Composer } from '../components/Composer';
import { CritiqueCard } from '../components/CritiqueCard';
import { Dialog, Empty, IconButton } from '../components/common';
import { AgentMark, Icon, Logo } from '../components/Icon';
import { RenderDialogBody, RenderStrip, renderLabel } from '../components/RenderStrip';
import { RichText } from '../components/RichText';
import { PreviewPane, type FileChange } from '../components/PreviewPane';
import { SystemPicker } from '../components/SystemPicker';
import { DeleteProjectDialog } from '../components/DeleteProject';
import { TodoCard } from '../components/TodoCard';
import { continueMessage, parseAssistantText, parseFormAnswers, type ParsedFormAnswers } from '../components/protocol';
import { skillNames, type Frame, type RunState } from '../state';

export function Studio({ projectId, systems, run, onMonitor, onStart, onHome, onDelete, onError, initialFrame = 'web', initialConversation = '', activityControl }: {
  projectId: string; systems: DesignSystem[]; run?: RunState; onMonitor: (run: Run) => void;
  onStart: (p: Project, conversationId: string, prompt: string, viewport: Frame) => Promise<void>;
  onHome: () => void; onDelete: (p: Project) => Promise<void>; onError: (s: string) => void; initialFrame?: Frame; initialConversation?: string; activityControl?: ComponentChildren;
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
  const [deleting, setDeleting] = useState(false);
  const [name, setName] = useState('');
  const [failure, setFailure] = useState('');
  const [connecting, setConnecting] = useState(false);
  const [historyRun, setHistoryRun] = useState<Run | null>(null);
  const [historyActivity, setHistoryActivity] = useState<ActivityRun | null>(null);
  const [preview, setPreview] = useState<Render | null>(null);
  const [versions, setVersions] = useState<ArtifactVersion[]>([]);
  // null shows the live project files; a number shows that saved version.
  const [viewing, setViewing] = useState<number | null>(null);
  const [canvas, setCanvas] = useState<'open' | 'closed' | 'expanded'>('open');
  const messagesPane = useRef<HTMLDivElement>(null);
  const follow = useRef(true);
  const conversationTransition = useRef(false);
  const appliedInitialConversation = useRef('');
  const busy = run?.status === 'running';
  const monitor = useRef(onMonitor); monitor.current = onMonitor;

  useEffect(() => {
    let active = true;
    Promise.all([api.project(projectId), api.files(projectId), api.runs(projectId)]).then(async ([p, f, r]) => {
      if (!active) return;
      setProject(p.project); setFiles(f.files); setConversations(p.conversations);
      if (p.conversations[0]) setConversationId(p.conversations.find(c => c.id === initialConversation)?.id || p.conversations[0].id);
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
  }, [conversationId, run?.id, run?.status]);

  useEffect(() => {
    if (!initialConversation) {
      appliedInitialConversation.current = '';
      return;
    }
    const key = `${projectId}\0${initialConversation}`;
    if (conversationTransition.current || appliedInitialConversation.current === key) return;
    if (conversations.some((c) => c.id === initialConversation)) {
      appliedInitialConversation.current = key;
      setConversationId(initialConversation);
    }
  }, [initialConversation, conversations, projectId, connecting]);
  useEffect(() => {
    let active = true;
    setHistoryActivity(null);
    if (historyRun) api.runActivity(historyRun.id).then(d => { if (active) setHistoryActivity(fromActivity(d.activity)); }).catch(() => {});
    return () => { active = false; };
  }, [historyRun?.id, run?.status]);

  // The latest finished run carries the plan and renders for a reopened project.
  useEffect(() => {
    if (!conversationId) return;
    let active = true;
    api.runs(projectId).then((d) => {
      if (!active) return;
      const latest = d.runs.find((candidate) => !candidate.conversationId || candidate.conversationId === conversationId);
      setHistoryRun(latest ?? null);
    }).catch(() => {});
    return () => { active = false; };
  }, [projectId, conversationId, run?.id, run?.status]);

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
  // A finished turn records a version; reload the history whenever a run settles.
  useEffect(() => {
    let active = true;
    api.versions(projectId).then((d) => { if (active) setVersions(d.versions); }).catch(() => {});
    return () => { active = false; };
  }, [projectId, run?.id, run?.status]);
  // A new turn always builds on the latest files, so follow it.
  useEffect(() => { if (busy) setViewing(null); }, [busy]);
  function openVersion(n: number | null) {
    setViewing(n !== null && n === versions[versions.length - 1]?.n ? null : n);
    if (canvas === 'closed') setCanvas('open');
  }
  async function restoreVersion(n: number) {
    if (!project) return;
    try {
      await api.restoreVersion(project.id, n);
      const d = await api.versions(project.id);
      setVersions(d.versions); setViewing(null);
      setFiles((await api.files(project.id)).files);
    } catch (e) { onError(e instanceof Error ? e.message : String(e)); }
  }
  useEffect(() => {
    const el = messagesPane.current;
    if (follow.current && el) el.scrollTop = el.scrollHeight;
  }, [messages, run?.phase, run?.text, run?.tools.length, run?.todos.length, run?.renders.length, run?.critiques.length]);

  // Every user turn goes through here: the composer, question forms and
  // next-step chips all submit the same way.
  async function sendText(value: string): Promise<boolean> {
    const text = value.trim();
    if (!project || !text || busy || connecting || conversationTransition.current) return false;
    conversationTransition.current = true;
    setConnecting(true);
    try {
      let id = conversationId;
      if (!id) {
        const c = await api.createConversation(project.id); id = c.conversation.id;
        setConversations((all) => [c.conversation, ...all]); setConversationId(id);
      }
      await onStart(project, id, text, frame);
      setMessages((all) => [...all, { id: `local-${Date.now()}`, conversationId: id, role: 'user', content: text, createdAt: new Date().toISOString() }]);
      follow.current = true;
      return true;
    } catch (e) { onError(String(e)); return false; }
    finally {
      conversationTransition.current = false;
      setConnecting(false);
    }
  }
  async function send() {
    if (!prompt.trim()) return;
    const text = prompt;
    setPrompt('');
    const sent = await sendText(text);
    if (!sent) setPrompt(text);
  }
  async function newConversation() {
    if (!project || busy || connecting || conversationTransition.current) return;
    conversationTransition.current = true;
    setConnecting(true);
    try {
      const c = await api.createConversation(project.id);
      setConversations((all) => [c.conversation, ...all]);
      setConversationId(c.conversation.id);
      setMessages([]);
    } catch (e) { onError(String(e)); }
    finally {
      conversationTransition.current = false;
      setConnecting(false);
    }
  }
  const answeredAt = useMemo(() => {
    const positions = new Map<string, number>();
    messages.forEach((message, index) => {
      if (message.role !== 'user') return;
      const answers = parseFormAnswers(message.content);
      if (answers) positions.set(answers.id, index);
    });
    return positions;
  }, [messages]);

  if (failure) return <div class="studio-failure"><Empty icon="folder" title="This project couldn’t be opened"><p>{failure}</p><button class="button primary" onClick={onHome}>Back to workspace</button></Empty></div>;
  if (!project) return <div class="loading-state full">Opening your project…</div>;
  const system = systems.find((s) => s.id === project.designSystemId);
  const showLive = run?.conversationId === conversationId && busy;
  const conversationRun = run && run.conversationId === conversationId ? run : undefined;
  const activeRun = showLive ? conversationRun : undefined;
  const planTodos = activeRun?.todos?.length ? activeRun.todos : historyRun?.todos?.length ? historyRun.todos : conversationRun?.todos ?? [];
  const planRenders = activeRun?.renders?.length ? activeRun.renders : historyRun?.renders?.length ? historyRun.renders : conversationRun?.renders ?? [];
  const planCritiques = activeRun?.critiques?.length ? activeRun.critiques : historyRun?.critiques?.length ? historyRun.critiques : conversationRun?.critiques ?? [];
  const latestAssistantIndex = messages.reduce((last, message, index) => (message.role === 'assistant' ? index : last), -1);
  const latestAssistant = latestAssistantIndex >= 0 ? parseAssistantText(messages[latestAssistantIndex].content) : null;
  const latestAnswers = latestAssistantIndex >= 0 ? answeredFormsFor(latestAssistantIndex) : new Map<string, ParsedFormAnswers>();
  const awaitingAnswers = latestAssistant ? latestAssistant.forms.some((form) => !form.invalid && !latestAnswers.has(form.id)) : false;
  const openTodos = planTodos.filter((todo) => todo.status !== 'completed');
  const canContinue = !busy && !connecting && openTodos.length > 0 && !awaitingAnswers;
  const displayedActivity = conversationRun || historyActivity;
  const latestVersion = versions[versions.length - 1];
  const versionByMessage = new Map(versions.filter((v) => v.messageId).map((v) => [v.messageId!, v]));
  const shownVersion = viewing ?? latestVersion?.n;

  function answeredFormsFor(index: number) {
    const message = messages[index];
    if (!message) return new Map<string, ParsedFormAnswers>();
    const forms = new Map<string, ParsedFormAnswers>();
    for (const form of parseAssistantText(message.content).forms) {
      const at = answeredAt.get(form.id);
      if (at === undefined || at <= index) continue;
      const answers = parseFormAnswers(messages[at].content);
      if (answers) forms.set(form.id, answers);
    }
    return forms;
  }

  return <div class={`studio-shell canvas-${canvas}`}>
    <header class="studio-header"><IconButton icon="home" label="Back to home" onClick={onHome}/><Logo size={26}/><button class="project-title" onClick={() => { setName(project.name); setRenaming(true); }} title="Rename project" aria-label="Rename project">{project.name}<Icon name="chevron" size={14}/></button><span class="studio-workflow">{skillNames[project.skillId || ''] || 'Prototype'}</span><div class="studio-header-end">{activityControl}<span class="local-label"><span class="status-dot"/>Local workspace</span><button class="button secondary small" onClick={() => setPickingSystem(true)} disabled={busy}><Icon name="palette" size={16}/>{system?.name || 'Design system'}</button>{canvas === 'closed' && <IconButton icon="panel" label="Show the canvas" onClick={() => setCanvas('open')}/>}<IconButton icon="trash" label="Delete project" onClick={() => setDeleting(true)}/></div></header>
    <div class="studio-content"><section class="conversation-pane" aria-label="Design conversation"><div class="conversation-toolbar"><label><span class="sr-only">Conversation</span><select value={conversationId} disabled={busy || connecting} onChange={(e) => { if (busy || connecting || conversationTransition.current) return; setConversationId(e.currentTarget.value); follow.current = true; }}>{conversations.map((c, i) => <option key={c.id} value={c.id}>{c.title === 'Main' ? 'Design conversation' : `${c.title} ${conversations.length - i}`}</option>)}</select></label><IconButton icon="plus" label="New conversation" onClick={newConversation} disabled={busy || connecting}/></div>
      {displayedActivity && <RunActivity key={displayedActivity.id} run={displayedActivity} workflow={skillNames[project.skillId || ''] || 'Prototype'} system={system?.name} onError={onError}/>}
      <div class="conversation-messages" ref={messagesPane} onScroll={(e) => { const el = e.currentTarget; follow.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100; }}>
        {!messages.length && !showLive && <div class="conversation-welcome"><Logo size={36}/><h2>Let’s make it yours.</h2><p>Describe the design you have in mind. We’ll build it here, together.</p></div>}
        {messages.map((message, index) => {
          if (message.role === 'user') {
            const answers = parseFormAnswers(message.content);
            if (answers) {
              return <article key={message.id} class="message message-user message-form-answers">
                <div class="message-byline"><span>You</span></div>
                <div class="form-answer-summary">
                  <span class="form-answer-summary-title"><Icon name="check" size={13}/>Answered</span>
                  {answers.entries.length
                    ? <ul>{answers.entries.map((entry, entryIndex) => <li key={entryIndex}><span>{entry.label}</span><strong>{entry.display}</strong></li>)}</ul>
                    : <p>All questions skipped.</p>}
                </div>
              </article>;
            }
            return <article key={message.id} class="message message-user"><div class="message-byline"><span>You</span></div><RichText text={message.content}/></article>;
          }
          return <article key={message.id} class="message message-assistant">
            <div class="message-byline"><AgentMark size={20}/><strong>pi agent</strong></div>
            <AgentMessage text={message.content} interactive={index === latestAssistantIndex && !busy && !connecting} answered={answeredFormsFor(index)} onSend={sendText}/>
            {versionByMessage.has(message.id) && (() => { const v = versionByMessage.get(message.id)!; return <ArtifactCard version={v} latest={v === latestVersion} active={canvas !== 'closed' && !busy && shownVersion === v.n} onOpen={() => openVersion(v.n)}/>; })()}
          </article>;
        })}
        {showLive && <article class="message message-assistant live-message"><div class="message-byline"><AgentMark size={20}/><strong>pi agent</strong><span class="generating-label">Working<span class="working-indicator"><i/><i/><i/></span></span></div>{run.text && <AgentMessage text={run.text} live/>}<ArtifactCardLive entry={project.entryFile} onOpen={() => openVersion(null)}/></article>}
        {run?.model && run.conversationId === conversationId && <p class="run-model"><ModelIcon id={run.modelId} provider={run.provider} size={16}/>Using {run.model}</p>}
        {planTodos.length > 0 && <TodoCard todos={planTodos} live={showLive}/>}
        {planRenders.length > 0 && <RenderStrip renders={planRenders} onOpen={setPreview}/>}
        {planCritiques.length > 0 && <CritiqueCard critiques={planCritiques} live={showLive}/>}
        {showLive && <RunProgress run={run}/>}

        {canContinue && <div class="continue-tasks"><button class="button secondary" onClick={() => void sendText(continueMessage(planTodos))} disabled={connecting}><Icon name="refresh" size={15}/>Continue remaining tasks<span class="continue-count">{openTodos.length}</span></button></div>}
        {run?.status === 'failed' && <div class="inline-error" role="alert"><strong>pi couldn’t finish this run.</strong><p>{run.error || 'Check your pi model configuration, then try again.'}</p></div>}
        {run?.status === 'canceled' && <p class="run-status">Generation stopped. Your existing files are unchanged unless pi saved an edit before stopping.</p>}
        {run?.status === 'succeeded' && !run.attention && <p class="run-status"><Icon name="check" size={15}/>{files.some((file) => /\.html?$/i.test(file.path)) ? 'Finished. Your design is ready to explore.' : 'Finished. Response saved to this conversation.'}</p>}
      </div>
      <div class="studio-composer"><Composer compact value={prompt} onChange={setPrompt} onSubmit={send} busy={busy} disabled={connecting} onStop={() => { if (run) api.cancelRun(run.id).catch((e) => onError(e.message)); }} systemName={system?.name} onSystem={() => setPickingSystem(true)} frame={frame} onFrame={setFrame}/><p>pi can edit project files and run commands on your Mac.</p></div>
    </section>{canvas !== 'closed' && <PreviewPane project={project} files={files} frame={frame} onFrame={setFrame} changes={changes} busy={busy} activityLabel={displayedActivity?.label} onError={onError} versions={versions} viewing={viewing} onView={setViewing} onRestore={restoreVersion} expanded={canvas === 'expanded'} onExpand={() => setCanvas(canvas === 'expanded' ? 'open' : 'expanded')} onClose={() => setCanvas('closed')}/>}</div>
    {pickingSystem && <SystemPicker systems={systems} selected={project.designSystemId || ''} onClose={() => setPickingSystem(false)} onSelect={async (id) => { try { const d = await api.updateProject({ ...project, designSystemId: id }); setProject(d.project); setPickingSystem(false); } catch (e) { onError(String(e)); } }}/>}
    {preview && <Dialog title={`${renderLabel(preview)} render`} onClose={() => setPreview(null)} class="render-dialog"><RenderDialogBody render={preview}/></Dialog>}
    {renaming && <Dialog title="Rename project" onClose={() => setRenaming(false)}><form class="rename-form" onSubmit={async (e) => { e.preventDefault(); try { const d = await api.updateProject({ ...project, name }); setProject(d.project); setRenaming(false); } catch (e) { onError(String(e)); } }}><label>Project name<input autoFocus value={name} onInput={(e) => setName(e.currentTarget.value)} required/></label><button class="button primary" disabled={!name.trim()}>Save name</button></form></Dialog>}
    {deleting && <DeleteProjectDialog project={project} running={busy} onClose={() => setDeleting(false)} onDelete={onDelete}/>}
  </div>;
}
