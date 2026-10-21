import { useEffect, useState } from 'preact/hooks';
import { api, type Connection, type Health } from '../api';
import { CopyButton } from '../components/common';
import { Icon, PiLogo } from '../components/Icon';
import { ModelButton, useModels } from '../model-context';
import type { Preferences } from '../state';
import { NativePreferences } from '../components/NativePreferences';

export function Settings({ health, preferences, onSave, onError }: { health: Health | null; preferences: Preferences; onSave: (p: Preferences) => Promise<void>; onError: (s: string) => void }) {
  const [draft, setDraft] = useState(preferences);
  const { levels, levelsLoading, error: modelError } = useModels();
  useEffect(() => { setDraft(preferences); }, [preferences.model, preferences.thinking]);
  const [saved, setSaved] = useState(false);
  const [connection, setConnection] = useState<Connection | null>(null);
  useEffect(() => { api.connection().then(setConnection).catch((e) => onError(e.message)); }, []);
  useEffect(() => { if (!saved) return; const t = setTimeout(() => setSaved(false), 2500); return () => clearTimeout(t); }, [saved]);
  const config = connection ? JSON.stringify(connection.config, null, 2) : '';
  return <div class="library-page settings-page"><div class="page-heading"><div><h1>Your workspace</h1><p>Local files. Your agent. Nothing in between.</p></div></div>
    <section class="settings-section"><div class="settings-section-heading"><h2>π agent</h2><p>The only engine behind every design.</p></div><div class="agent-card"><PiLogo size={40}/><div><strong>Installed π agent</strong><p>{health?.piBin || 'π was not found. Install it or relaunch from your configured shell.'}</p></div><span class={`agent-state ${health?.piBin ? '' : 'unavailable'}`}><span class="status-dot"/>{health?.piBin ? 'Detected' : 'Not found'}</span></div>
      <form class="preferences-form" onSubmit={async (e) => { e.preventDefault(); try { await onSave(draft); setSaved(true); } catch (err) { onError(String(err)); } }}><div class="settings-model-field"><span class="settings-field-label">Model</span><span class="settings-field-help">Choose from your installed π. Selecting a model saves it for the next run.</span><ModelButton settings/></div><label>Thinking level<select value={draft.thinking} disabled={levelsLoading || !levels.length} onChange={(e) => setDraft({ ...draft, thinking: e.currentTarget.value })}>{!levels.length && <option value={draft.thinking}>{levelsLoading ? 'Loading…' : draft.thinking}</option>}{!!levels.length && !levels.includes(draft.thinking) && <option value={draft.thinking} disabled>{draft.thinking} · not supported</option>}{levels.map((level) => <option key={level} value={level}>{level}</option>)}</select></label><div class="save-preferences"><button class="button primary" type="submit" disabled={levelsLoading || !levels.includes(draft.thinking)}>Save preferences</button><span role="status">{saved ? 'Preferences saved' : 'Applies to your next generation.'}</span></div></form>{modelError && <p class="inline-error" role="alert">{modelError}</p>}
    </section>
    <NativePreferences/>
    <section class="settings-section" id="editor-connection"><div class="settings-section-heading"><h2>Connect your editor</h2><p>Read, edit, and generate in this workspace through MCP.</p></div><div class="connection-heading"><div class="connection-icon"><Icon name="connect" size={24}/></div><div><strong>πDesigner MCP server</strong><p>Keep the Mac app open while your editor is connected.</p></div>{connection && <CopyButton text={config} label="Copy config" onError={onError}/>}</div>
      {!connection ? <p class="loading-state">Loading editor configuration…</p> : <><pre class="source-display connection-config"><code>{config}</code></pre>{!connection.available && <p class="inline-error">The MCP executable is missing. Rebuild the Mac app to bundle it.</p>}<p class="settings-note">Add this entry to your editor’s MCP configuration. The discovery file follows the app’s local port after restarts, so you only need to configure it once. This does not modify your editor’s settings.</p></>}
    </section>
    <section class="settings-section"><div class="settings-section-heading"><h2>Project storage</h2><p>Your prototypes stay in a local workspace.</p></div><code class="storage-path">{health?.projectRoot || 'Workspace not connected'}</code><p class="settings-note">Model authentication stays with π. πDesigner does not ask for or store your API keys.</p></section>
  </div>;
}
