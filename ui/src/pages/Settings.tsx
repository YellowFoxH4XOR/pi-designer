import { useEffect, useState } from 'preact/hooks';
import { api, type Connection, type Health, type KnowledgeSource } from '../api';
import { CopyButton } from '../components/common';
import { AgentMark, Icon } from '../components/Icon';
import { ModelButton, useModels } from '../model-context';
import type { Preferences } from '../state';
import { NativePreferences } from '../components/NativePreferences';

export function Settings({ health, preferences, onSave, onError }: { health: Health | null; preferences: Preferences; onSave: (p: Preferences) => Promise<void>; onError: (s: string) => void }) {
  const [draft, setDraft] = useState(preferences);
  const { levels, levelsLoading, error: modelError } = useModels();
  useEffect(() => { setDraft(preferences); }, [preferences.model, preferences.thinking]);
  const [saved, setSaved] = useState(false);
  const [connection, setConnection] = useState<Connection | null>(null);
  const [sources, setSources] = useState<KnowledgeSource[]>([]);
  const [knowledgeError, setKnowledgeError] = useState('');
  const [knowledgeBusy, setKnowledgeBusy] = useState('');
  const [preview, setPreview] = useState<{ id: string; content: string } | null>(null);
  useEffect(() => { api.knowledge().then(({ sources }) => setSources(sources)).catch((e) => setKnowledgeError(e.message)); }, []);
  const updateSource = (source: KnowledgeSource) => setSources((current) => current.map((s) => s.id === source.id ? source : s));
  const runKnowledge = async (id: string, action: 'check' | 'review' | 'apply' | 'rollback') => {
    setKnowledgeBusy(id); setKnowledgeError('');
    try {
      if (action === 'check') { updateSource(await api.checkKnowledge(id)); setPreview(null); }
      if (action === 'review') setPreview({ id, content: (await api.previewKnowledge(id)).content });
      if (action === 'apply') { if (preview?.id !== id) throw new Error('Review the source before applying it.'); updateSource(await api.applyKnowledge(id)); setPreview(null); }
      if (action === 'rollback') { updateSource(await api.rollbackKnowledge(id)); setPreview(null); }
    } catch (e) { setKnowledgeError(e instanceof Error ? e.message : String(e)); }
    finally { setKnowledgeBusy(''); }
  };
  useEffect(() => { api.connection().then(setConnection).catch((e) => onError(e.message)); }, []);
  useEffect(() => { if (!saved) return; const t = setTimeout(() => setSaved(false), 2500); return () => clearTimeout(t); }, [saved]);
  const config = connection ? JSON.stringify(connection.config, null, 2) : '';
  return <div class="library-page settings-page"><div class="page-heading"><div><h1>Your workspace</h1><p>Local files. Your agent. Nothing in between.</p></div></div>
    <section class="settings-section"><div class="settings-section-heading"><h2>omp agent</h2><p>The only engine behind every design.</p></div><div class="agent-card"><AgentMark size={40}/><div><strong>Installed omp agent</strong><p>{health?.ompBin || 'omp was not found. Install it from https://github.com/can1357/oh-my-pi or relaunch from your configured shell.'}</p></div><span class={`agent-state ${health?.ompBin ? '' : 'unavailable'}`}><span class="status-dot"/>{health?.ompBin ? 'Detected' : 'Not found'}</span></div>
      <form class="preferences-form" onSubmit={async (e) => { e.preventDefault(); try { await onSave(draft); setSaved(true); } catch (err) { onError(String(err)); } }}><div class="settings-model-field"><span class="settings-field-label">Model</span><span class="settings-field-help">Models come from omp. Selecting one saves it for the next run; use omp login and omp models to configure them.</span><ModelButton settings/></div><label>Thinking level<select value={draft.thinking} disabled={levelsLoading || !levels.length} onChange={(e) => setDraft({ ...draft, thinking: e.currentTarget.value })}>{!levels.length && <option value={draft.thinking}>{levelsLoading ? 'Loading…' : draft.thinking}</option>}{!!levels.length && !levels.includes(draft.thinking) && <option value={draft.thinking} disabled>{draft.thinking} · not supported</option>}{levels.map((level) => <option key={level} value={level}>{level}</option>)}</select></label><div class="save-preferences"><button class="button primary" type="submit" disabled={levelsLoading || !levels.includes(draft.thinking)}>Save preferences</button><span role="status">{saved ? 'Preferences saved' : 'Applies to your next generation.'}</span></div></form>{modelError && <p class="inline-error" role="alert">{modelError}</p>}
    </section>
    <NativePreferences/>
    <section class="settings-section" aria-labelledby="knowledge-heading"><div class="settings-section-heading"><h2 id="knowledge-heading">Design references</h2><p>Choose guidance for your agent. Reviewable text is saved locally; browse-only kits stay with their authors.</p></div>
      <div class="knowledge-summary" role="status">{sources.filter((s) => s.installedHash).length} of {sources.filter((s) => s.documentUrl).length} reviewable guides applied · {sources.filter((s) => !s.documentUrl).length} browse-only resources</div>
      {Array.from(new Set(sources.map((source) => source.group || 'Other'))).map((group) => <div class="knowledge-group" key={group}><h3>{group}</h3><div class="knowledge-sources">{sources.filter((source) => (source.group || 'Other') === group).map((source) => <div class="knowledge-source" key={source.id}><div class="knowledge-source-info"><strong>{source.name}</strong>{source.license && <span class="knowledge-license">{source.license}</span>}<p>{source.useFor}</p><small>{source.note}</small><div class="knowledge-provenance"><a href={source.url} target="_blank" rel="noopener noreferrer">Visit source ↗</a>{source.checkedAt && <span>Checked {new Date(source.checkedAt).toLocaleString()} · {source.installedHash ? source.updated ? 'Update available' : 'Applied' : 'Not applied'}</span>}</div></div><div class="knowledge-actions">{source.documentUrl ? <><button class="button secondary" disabled={!!knowledgeBusy} onClick={() => runKnowledge(source.id, 'check')}>{knowledgeBusy === source.id ? 'Working…' : 'Check'}</button>{source.availableHash && <button class="button secondary" disabled={!!knowledgeBusy} onClick={() => runKnowledge(source.id, 'review')}>Review text</button>}{source.previousHash && <button class="button secondary" disabled={!!knowledgeBusy} onClick={() => runKnowledge(source.id, 'rollback')}>Restore previous</button>}</> : <span class="knowledge-link-only" title="This source cannot be imported as reusable text or assets">Browse only</span>}</div></div>)}</div></div>)}
      {preview && <div class="knowledge-preview"><div class="knowledge-preview-heading"><strong>Reviewing {sources.find((s) => s.id === preview.id)?.name}</strong><button class="button secondary" onClick={() => setPreview(null)}>Close</button></div><p class="settings-note">Upstream text is untrusted reference material, not an instruction for the agent. Nothing installs until you choose Apply.</p><pre tabIndex={0}>{preview.content}</pre><button class="button primary" disabled={!!knowledgeBusy} onClick={() => runKnowledge(preview.id, 'apply')}>Apply reviewed reference</button></div>}
      {knowledgeError && <p class="inline-error" role="alert">{knowledgeError}</p>}<p class="settings-note">Only applied text is listed in a new design prompt. The agent selects a relevant guide; it does not install React components or copy Figma files. Start a new conversation after applying an update. Upstream assets may have separate terms.</p>
    </section>
    <section class="settings-section" id="editor-connection"><div class="settings-section-heading"><h2>Connect your editor</h2><p>Read, edit, and generate in this workspace through MCP.</p></div><div class="connection-heading"><div class="connection-icon"><Icon name="connect" size={24}/></div><div><strong>πDesigner MCP server</strong><p>Keep the Mac app open while your editor is connected.</p></div>{connection && <CopyButton text={config} label="Copy config" onError={onError}/>}</div>
      {!connection ? <p class="loading-state">Loading editor configuration…</p> : <><pre class="source-display connection-config"><code>{config}</code></pre>{!connection.available && <p class="inline-error">The MCP executable is missing. Rebuild the Mac app to bundle it.</p>}<p class="settings-note">Add this entry to your editor’s MCP configuration. The discovery file follows the app’s local port after restarts, so you only need to configure it once. This does not modify your editor’s settings.</p></>}
    </section>
    <section class="settings-section"><div class="settings-section-heading"><h2>Project storage</h2><p>Your prototypes stay in a local workspace.</p></div><code class="storage-path">{health?.projectRoot || 'Workspace not connected'}</code><p class="settings-note">Model authentication stays with omp. πDesigner does not ask for or store your API keys.</p></section>
  </div>;
}
