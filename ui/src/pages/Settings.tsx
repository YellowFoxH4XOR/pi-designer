import { useEffect, useRef, useState } from 'preact/hooks';
import { api, type Connection, type Health, type KnowledgeSource } from '../api';
import { CopyButton } from '../components/common';
import { AgentMark, Icon } from '../components/Icon';
import { ModelButton, useModels } from '../model-context';
import type { Preferences } from '../state';
import { NativePreferences } from '../components/NativePreferences';
import { AppUpdates } from '../components/AppUpdates';

const INSTALL_COMMAND = 'curl -fsSL https://pi.dev/install.sh | sh';

function PiAgent({ health, onHealth, onError }: { health: Health | null; onHealth: (h: Health) => void; onError: (s: string) => void }) {
  const [checking, setChecking] = useState(false);
  const pi = health?.pi;
  const ready = !!pi?.supported;
  const state = !pi ? 'Checking…' : ready ? 'Ready' : pi.bin ? 'Needs an update' : 'Not found';
  const recheck = async () => {
    setChecking(true);
    try { onHealth(await api.health(true)); } catch (e) { onError(e instanceof Error ? e.message : String(e)); } finally { setChecking(false); }
  };
  return <div class={`pi-runtime ${ready ? '' : 'attention'}`}>
    <div class="pi-runtime-head"><AgentMark size={40}/><div><strong>pi coding agent</strong><p>{ready ? 'Runs every design in your project folder, with your own providers and models.' : 'πDesigner uses the pi installed on your Mac. Install or update it, then check again.'}</p></div><span class={`agent-state ${ready ? '' : 'unavailable'}`} role="status"><span class={`status-dot ${ready ? '' : 'offline'}`}/>{state}</span></div>
    <dl class="pi-runtime-facts">
      <div><dt>Executable</dt><dd><code>{pi?.bin || 'pi · not on your PATH'}</code></dd></div>
      <div><dt>Version</dt><dd><code>{pi?.version ? `pi ${pi.version}` : '—'}</code>{pi && <small>Needs {pi.minVersion} or newer</small>}</dd></div>
      <div><dt>Profile</dt><dd><code>~/.pi/agent</code><small>Providers, sign-ins and models are shared with pi in your terminal.</small></dd></div>
    </dl>
    {pi?.error && <p class="inline-error" role="alert">{pi.error}</p>}
    {!ready && <ol class="pi-steps">
      <li><span>Install pi with its installer, or with <code>npm install -g @earendil-works/pi-coding-agent</code>.</span><div class="pi-command"><code>{INSTALL_COMMAND}</code><CopyButton text={INSTALL_COMMAND} label="Copy" onError={onError}/></div></li>
      <li><span>Run <code>pi</code> in Terminal and use <code>/login</code> to sign in to a model provider.</span></li>
      <li><span>Come back and choose <strong>Check again</strong>. Nothing needs to be selected or copied.</span></li>
    </ol>}
    <div class="pi-runtime-actions"><button class="button secondary small" onClick={recheck} disabled={checking}><Icon name="refresh" size={14}/>{checking ? 'Checking…' : 'Check again'}</button><a class="text-button" href="https://pi.dev" target="_blank" rel="noopener noreferrer">pi setup guide<Icon name="external" size={13}/></a></div>
  </div>;
}

const relative = (iso?: string) => {
  if (!iso) return '';
  const minutes = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  return hours < 24 ? `${hours} h ago` : new Date(iso).toLocaleDateString();
};

function DesignReferences({ onError }: { onError: (s: string) => void }) {
  const [sources, setSources] = useState<KnowledgeSource[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const busyRef = useRef('');
  useEffect(() => {
    let active = true;
    let timer = 0;
    const refresh = async () => {
      try {
        const { sources } = await api.knowledge();
        if (active && !busyRef.current) { setSources(sources); setError(''); }
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (active) timer = window.setTimeout(() => { void refresh(); }, 3000);
      }
    };
    void refresh();
    return () => { active = false; window.clearTimeout(timer); };
  }, []);
  const run = async (key: string, action: () => Promise<void>) => {
    setBusy(key); busyRef.current = key; setError('');
    try { await action(); } catch (e) { onError(e instanceof Error ? e.message : String(e)); }
    finally { busyRef.current = ''; setBusy(''); }
  };
  const syncAll = () => run('sync', async () => setSources((await api.syncKnowledge()).sources));
  const restore = (id: string) => run(id, async () => { const updated = await api.rollbackKnowledge(id); setSources((current) => current.map((s) => s.id === id ? updated : s)); });
  const text = sources.filter((s) => s.documentUrl);
  const installed = text.filter((s) => s.installedHash).length;
  const failing = text.filter((s) => s.error).length;
  const lastSync = text.map((s) => s.checkedAt).filter(Boolean).sort().pop();
  return <section class="settings-section" aria-labelledby="knowledge-heading"><div class="settings-section-heading"><h2 id="knowledge-heading">Design references</h2><p>Guides pi can read while it designs. They update on their own when the app opens and every day after.</p></div>
    <div class="reference-sync"><span class={`status-dot ${busy === 'sync' || installed < text.length && !failing ? 'working' : failing ? 'offline' : ''}`}/><span role="status">{busy === 'sync' ? 'Syncing references…' : <><strong>{installed} of {text.length} guides ready</strong>{failing ? ` · ${failing} couldn’t update` : ''}{lastSync ? ` · synced ${relative(lastSync)}` : ''}</>}</span><button class="button secondary small" onClick={syncAll} disabled={!!busy}><Icon name="refresh" size={14}/>Sync now</button></div>
    {Array.from(new Set(sources.map((s) => s.group || 'Other'))).map((group) => <div class="knowledge-group" key={group}><h3>{group}</h3><ul class="knowledge-sources">{sources.filter((s) => (s.group || 'Other') === group).map((s) => {
      const status = !s.documentUrl ? 'Link only' : s.error ? 'Update failed' : s.installedHash ? 'Ready' : 'Waiting for sync';
      return <li class="knowledge-source" key={s.id}><div class="knowledge-source-info"><div class="knowledge-source-title"><strong>{s.name}</strong>{s.license && <span class="knowledge-license">{s.license}</span>}</div><p>{s.useFor}</p><div class="knowledge-provenance"><a href={s.url} target="_blank" rel="noopener noreferrer">Source<Icon name="external" size={11}/></a>{s.checkedAt && <span>Checked {relative(s.checkedAt)}</span>}</div>{s.error && <p class="knowledge-error">{s.installedHash ? 'Using the last good copy. ' : ''}{s.error}</p>}</div>
        <div class="knowledge-actions"><span class={`reference-state ${s.error ? 'failed' : s.installedHash ? 'ready' : ''}`}>{status}</span>{s.previousHash && <button class="text-button" disabled={!!busy} onClick={() => restore(s.id)}>{busy === s.id ? 'Restoring…' : 'Restore previous'}</button>}</div></li>;
    })}</ul></div>)}
    {error && <p class="inline-error" role="alert">{error}</p>}
    <p class="settings-note">pi treats these guides as reference material, never as instructions, and the selected design system stays in charge. Link-only kits stay with their authors; nothing installs components or copies Figma files. A restored copy lasts until the next sync.</p>
  </section>;
}

export function Settings({ health, preferences, onSave, onHealth, onError }: { health: Health | null; preferences: Preferences; onSave: (p: Preferences) => Promise<void>; onHealth: (h: Health) => void; onError: (s: string) => void }) {
  const [draft, setDraft] = useState(preferences);
  const { levels, levelsLoading, error: modelError } = useModels();
  useEffect(() => { setDraft(preferences); }, [preferences.model, preferences.thinking]);
  const [saved, setSaved] = useState(false);
  const [connection, setConnection] = useState<Connection | null>(null);
  useEffect(() => { api.connection().then(setConnection).catch((e) => onError(e.message)); }, []);
  useEffect(() => { if (!saved) return; const t = setTimeout(() => setSaved(false), 2500); return () => clearTimeout(t); }, [saved]);
  const config = connection ? JSON.stringify(connection.config, null, 2) : '';
  return <div class="library-page settings-page"><div class="page-heading"><div><h1>Your workspace</h1><p>Local files. Your pi agent. Nothing in between.</p></div></div>
    <section class="settings-section"><div class="settings-section-heading"><h2>pi agent</h2><p>The engine behind every design.</p></div><PiAgent health={health} onHealth={onHealth} onError={onError}/>
      <form class="preferences-form" onSubmit={async (e) => { e.preventDefault(); try { await onSave(draft); setSaved(true); } catch (err) { onError(String(err)); } }}><div class="settings-model-field"><span class="settings-field-label">Model</span><span class="settings-field-help">Models come from pi. Selecting one saves it for the next run; run pi and use /login to add providers.</span><ModelButton settings/></div><label>Thinking level<select value={draft.thinking} disabled={levelsLoading || !levels.length} onChange={(e) => setDraft({ ...draft, thinking: e.currentTarget.value })}>{!levels.length && <option value={draft.thinking}>{levelsLoading ? 'Loading…' : draft.thinking}</option>}{!!levels.length && !levels.includes(draft.thinking) && <option value={draft.thinking} disabled>{draft.thinking} · not supported</option>}{levels.map((level) => <option key={level} value={level}>{level}</option>)}</select></label><div class="save-preferences"><button class="button primary" type="submit" disabled={levelsLoading || !levels.includes(draft.thinking)}>Save preferences</button><span role="status">{saved ? 'Preferences saved' : 'Applies to your next generation.'}</span></div></form>{modelError && <p class="inline-error" role="alert">{modelError}</p>}
    </section>
    <AppUpdates/>
    <NativePreferences/>
    <DesignReferences onError={onError}/>
    <section class="settings-section" id="editor-connection"><div class="settings-section-heading"><h2>Connect your editor</h2><p>Read, edit, and generate in this workspace through MCP.</p></div><div class="connection-heading"><div class="connection-icon"><Icon name="connect" size={24}/></div><div><strong>πDesigner MCP server</strong><p>Keep the Mac app open while your editor is connected.</p></div>{connection && <CopyButton text={config} label="Copy config" onError={onError}/>}</div>
      {!connection ? <p class="loading-state">Loading editor configuration…</p> : <><pre class="source-display connection-config"><code>{config}</code></pre>{!connection.available && <p class="inline-error">The MCP executable is missing. Rebuild the Mac app to bundle it.</p>}<p class="settings-note">Add this entry to your editor’s MCP configuration. The discovery file follows the app’s local port after restarts, so you only need to configure it once. This does not modify your editor’s settings.</p></>}
    </section>
    <section class="settings-section"><div class="settings-section-heading"><h2>Project storage</h2><p>Your prototypes stay in a local workspace.</p></div><code class="storage-path">{health?.projectRoot || 'Workspace not connected'}</code><p class="settings-note">Model authentication stays with pi. πDesigner does not ask for or store your API keys.</p></section>
  </div>;
}
