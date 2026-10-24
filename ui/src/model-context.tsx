import { createContext, type ComponentChildren } from 'preact';
import { useCallback, useContext, useEffect, useRef, useState } from 'preact/hooks';
import { api, type ModelCatalog } from './api';
import type { Preferences } from './state';
import { Dialog, SearchField } from './components/common';
import { Icon } from './components/Icon';
import { ModelIcon } from './components/ModelIcon';

export const modelRef = (m: { provider: string; id: string }) => `${m.provider}/${m.id}`;
export function providerName(id: string) {
  const names: Record<string, string> = { 'github-copilot': 'GitHub Copilot', 'opencode-go': 'OpenCode Go', 'openai-codex': 'OpenAI Codex', 'anthropic': 'Anthropic', 'openai': 'OpenAI', 'google': 'Google', 'openrouter': 'OpenRouter', 'vercel-ai-gateway': 'Vercel AI Gateway' };
  return names[id] || id.replace(/(^|[-_])([a-z])/g, (_, space, c) => `${space ? ' ' : ''}${c.toUpperCase()}`);
}
interface ModelContextValue {
  preferences: Preferences; ready: boolean; catalog: ModelCatalog | null;
  levels: string[]; levelsLoading: boolean; error: string; loading: boolean;
  open: () => void; refresh: () => Promise<void>; save: (p: Preferences) => Promise<void>;
}
const ModelContext = createContext<ModelContextValue | null>(null);
export function useModels() {
  const value = useContext(ModelContext);
  if (!value) throw new Error('pi model controls require the model provider');
  return value;
}
export function ModelProvider({ children }: { children: ComponentChildren }) {
  const [preferences, setPreferences] = useState<Preferences>({ model: '', thinking: 'medium' });
  const [ready, setReady] = useState(false);
  const [catalog, setCatalog] = useState<ModelCatalog | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [levels, setLevels] = useState<string[]>([]);
  const [levelsLoading, setLevelsLoading] = useState(false);
  const [opened, setOpened] = useState(false);
  const [selecting, setSelecting] = useState<string | null>(null);
  const [selectionError, setSelectionError] = useState('');
  const mounted = useRef(true);
  const refresh = useCallback(async (force = true) => {
    setLoading(true); setError('');
    try { const data = await api.models(force); if (mounted.current) setCatalog(data); }
    catch (e) { if (mounted.current) setError(String(e).replace(/^Error: /, '')); }
    finally { if (mounted.current) setLoading(false); }
  }, []);
  useEffect(() => {
    mounted.current = true;
    api.preferences().then((p) => { if (mounted.current) { setPreferences(p); setReady(true); } }).catch((e) => { if (mounted.current) setError(e.message); });
    void refresh(false);
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    if (!catalog) return;
    let active = true; setLevelsLoading(true);
    api.thinkingLevels(preferences.model).then((r) => { if (active) setLevels(r.levels); }).catch((e) => { if (active) setError(e.message); }).finally(() => { if (active) setLevelsLoading(false); });
    return () => { active = false; };
  }, [catalog, preferences.model]);
  async function save(p: Preferences) {
    const stored = await api.savePreferences(p);
    setPreferences(stored); setReady(true);
  }
  async function choose(ref: string) {
    if (selecting !== null) return;
    setSelecting(ref); setSelectionError('');
    try {
      const { levels } = await api.thinkingLevels(ref);
      const thinking = levels.includes(preferences.thinking) ? preferences.thinking : levels.includes('medium') ? 'medium' : levels[0];
      if (!thinking) throw new Error('pi returned no thinking levels for this model.');
      await save({ model: ref, thinking });
      setLevels(levels); setOpened(false);
    } catch (e) { setSelectionError(String(e).replace(/^Error: /, '')); }
    finally { setSelecting(null); }
  }
  return <ModelContext.Provider value={{ preferences, ready, catalog, levels, levelsLoading, error, loading, refresh, save, open: () => { setSelectionError(''); setOpened(true); } }}>
    {children}
    {opened && <ModelDialog catalog={catalog} selected={preferences.model} loading={loading} error={selectionError || error} selecting={selecting} onRefresh={refresh} onSelect={choose} onClose={() => { if (selecting === null) setOpened(false); }}/>}
  </ModelContext.Provider>;
}
export function ModelButton({ compact = false, disabled = false, settings = false }: { compact?: boolean; disabled?: boolean; settings?: boolean }) {
  const { preferences, catalog, open, ready } = useModels();
  const selected = catalog?.models.find((m) => modelRef(m) === preferences.model);
  const label = preferences.model ? selected?.name || preferences.model.split('/').slice(1).join('/') : 'pi default';
  const provider = selected ? providerName(selected.provider) : catalog?.current ? `${catalog.current.name} · ${providerName(catalog.current.provider)}` : 'Your installed pi';
  return <button type="button" class={`model-button ${compact ? 'compact' : ''} ${settings ? 'settings-model' : ''}`} onClick={open} disabled={disabled || !ready} aria-label="Choose pi model" aria-haspopup="dialog" title={`${label} · ${provider}`}><ModelIcon id={selected?.id || (preferences.model ? preferences.model.split('/').slice(1).join('/') : catalog?.current?.id)} provider={selected?.provider || (preferences.model ? preferences.model.split('/')[0] : catalog?.current?.provider)} size={settings ? 28 : 22}/><span><strong>{label}</strong>{settings && <small>{provider}</small>}</span><Icon name="chevron" size={13} class="model-chevron"/></button>;
}
function ModelDialog({ catalog, selected, loading, error, selecting, onRefresh, onSelect, onClose }: { catalog: ModelCatalog | null; selected: string; loading: boolean; error: string; selecting: string | null; onRefresh: () => void; onSelect: (ref: string) => void; onClose: () => void }) {
  const [query, setQuery] = useState('');
  const filtered = (catalog?.models || []).filter((m) => `${m.name} ${m.provider} ${m.id}`.toLowerCase().includes(query.toLowerCase()));
  const current = catalog?.current;
  return <Dialog title="Choose an pi model" onClose={onClose} class="model-picker">
    <p class="dialog-intro">Available models from your installed pi. Accounts and credentials stay with pi.</p>
    <SearchField value={query} onInput={setQuery} placeholder="Search models or providers"/>
    <div class="model-picker-info"><span>{catalog ? `${catalog.models.length} models available` : 'Reading your pi configuration…'}</span><button class="text-button" onClick={onRefresh} disabled={loading || selecting !== null}><Icon name="refresh" size={13}/>{loading ? 'Refreshing…' : 'Refresh'}</button></div>
    {error && <p class="inline-error" role="alert">{error}</p>}
    {!catalog && loading ? <div class="loading-state">Starting pi to discover your models…</div> : <div class="model-list">
      {!query && <button class={`model-row ${!selected ? 'selected' : ''}`} onClick={() => onSelect('')} disabled={selecting !== null}><ModelIcon id={current?.id} provider={current?.provider} size={27}/><span><strong>pi default</strong><small>{current ? `${current.name} · ${providerName(current.provider)}` : 'Use the default configured in pi'}</small></span>{!selected && <Icon name="check" size={17}/>}</button>}
      {filtered.map((m) => { const ref = modelRef(m); return <button key={ref} data-model-ref={ref} class={`model-row ${ref === selected ? 'selected' : ''}`} onClick={() => onSelect(ref)} disabled={selecting !== null}><ModelIcon id={m.id} provider={m.provider} size={27}/><span><strong>{m.name}</strong><small>{providerName(m.provider)}<span class="model-id">{m.id}</span></small></span><span class="model-row-meta">{selecting === ref ? 'Selecting…' : ref === selected ? <Icon name="check" size={17}/> : m.reasoning ? 'Reasoning' : ''}</span></button>; })}
      {catalog && !filtered.length && <p class="no-results">No models match “{query}”. Try a model or provider name.</p>}
    </div>}
    {selecting !== null && <p class="model-select-status" role="status">Checking this model’s supported thinking levels…</p>}
  </Dialog>;
}
