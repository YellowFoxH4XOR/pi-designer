import type { ArtifactVersion } from '../api';
import { Icon } from './Icon';

const changeSummary = (v: ArtifactVersion) => {
  if (v.label) return v.label;
  const n = v.changed.length + (v.removed?.length || 0);
  return n === 1 ? `Updated ${v.changed[0] || v.removed?.[0]}` : `${n} files changed`;
};

/** The artifact a turn produced, shown inline in the conversation. Opening it
 * shows that exact version in the canvas, like an artifact in a chat. */
export function ArtifactCard({ version, latest, active, onOpen }: {
  version: ArtifactVersion; latest: boolean; active: boolean; onOpen: () => void;
}) {
  const entry = version.entry || version.files.find((f) => /\.html?$/i.test(f)) || version.files[0] || 'Project files';
  return <button type="button" class={`artifact-card ${active ? 'active' : ''}`} onClick={onOpen} aria-pressed={active} aria-label={`Open ${entry}, version ${version.n}${latest ? ', latest' : ''}`}>
    <span class="artifact-card-icon"><Icon name={/\.html?$/i.test(entry) ? 'web' : 'file'} size={18}/></span>
    <span class="artifact-card-text"><strong>{entry}</strong><small>Version {version.n}{latest ? ' · Latest' : ''} · {changeSummary(version)}</small></span>
    <span class="artifact-card-open">{active ? 'Showing' : 'Open'}<Icon name="chevron" size={13}/></span>
  </button>;
}

/** While pi is working, the card shows the artifact being built. */
export function ArtifactCardLive({ entry, onOpen }: { entry?: string; onOpen: () => void }) {
  return <button type="button" class="artifact-card live" onClick={onOpen} aria-label="Show the design pi is building">
    <span class="artifact-card-icon"><Icon name="web" size={18}/></span>
    <span class="artifact-card-text"><strong>{entry || 'New design'}</strong><small>Building<span class="working-indicator"><i/><i/><i/></span></small></span>
    <span class="artifact-card-open">Watch<Icon name="chevron" size={13}/></span>
  </button>;
}
