import { useMemo, useState } from 'preact/hooks';
import type { DesignSystem } from '../api';
import { Dialog, SearchField } from './common';
import { Icon } from './Icon';

export const systemColors: Record<string, string> = {
  'stripe': '#635bff', 'linear-app': '#5e6ad2', 'vercel': '#18181b', 'apple': '#89898e', 'airbnb': '#ff385c',
  'claude': '#b76546', 'spotify': '#16833e', 'notion': '#37352f', 'agentic': '#ff5701', 'figma': '#9747ff',
  'shopify': '#5b831a', 'github': '#24292f', 'openai': '#108176', 'default': '#59657b', 'supabase': '#27855c',
};
export function SystemBadge({ system, size = 'normal' }: { system: DesignSystem; size?: 'normal' | 'large' }) {
  return <span aria-hidden="true" class={`system-badge ${size}`} style={{ '--brand-color': systemColors[system.id] || '#656185' }}>{system.name.replace(/[^\p{L}\p{N}]/gu, '').slice(0, 1)}</span>;
}
export function SystemPicker({ systems, selected, onSelect, onClose }: {
  systems: DesignSystem[]; selected: string; onSelect: (id: string) => void; onClose: () => void;
}) {
  const [query, setQuery] = useState('');
  const filtered = useMemo(() => systems.filter((s) => `${s.name} ${s.category} ${s.id}`.toLowerCase().includes(query.toLowerCase())), [systems, query]);
  return <Dialog title="Choose a design system" onClose={onClose} class="system-picker">
    <p class="dialog-intro">Give Pi a palette, typography, and component language to work with.</p>
    <SearchField value={query} onInput={setQuery} placeholder={`Search ${systems.length} design systems`}/>
    <div class="picker-list">
      <button class={`picker-row ${!selected ? 'selected' : ''}`} onClick={() => onSelect('')}><span class="system-badge neutral"><Icon name="palette"/></span><span><strong>No preset</strong><small>Let your brief guide the design</small></span>{!selected && <Icon name="check" size={17}/>}</button>
      {filtered.map((s) => <button key={s.id} class={`picker-row ${selected === s.id ? 'selected' : ''}`} onClick={() => onSelect(s.id)}><SystemBadge system={s}/><span><strong>{s.name}</strong><small>{s.category || 'Design system'}</small></span>{selected === s.id && <Icon name="check" size={17}/>}</button>)}
      {!filtered.length && <p class="no-results">No systems match “{query}”. Try a different name.</p>}
    </div>
  </Dialog>;
}
