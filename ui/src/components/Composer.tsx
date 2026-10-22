import { useRef } from 'preact/hooks';
import { Icon } from './Icon';
import { ModelButton } from '../model-context';
import { IconButton } from './common';
import { frames, type Frame } from '../state';

export function Composer({ value, onChange, onSubmit, busy = false, onStop, systemName, onSystem, frame, onFrame, compact = false, disabled = false }: {
  value: string; onChange: (v: string) => void; onSubmit: () => void; busy?: boolean; onStop?: () => void;
  systemName?: string; onSystem: () => void; frame: Frame; onFrame: (v: Frame) => void; compact?: boolean; disabled?: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  return <form class={`composer ${compact ? 'composer-compact' : ''}`} onSubmit={(e) => { e.preventDefault(); if (!busy && value.trim() && !disabled) onSubmit(); }}>
    <label class="sr-only" for={compact ? 'studio-prompt' : 'home-prompt'}>{compact ? 'Refine your design' : 'Describe your design'}</label>
    <textarea ref={ref} id={compact ? 'studio-prompt' : 'home-prompt'} rows={compact ? 3 : 4} value={value}
      placeholder={compact ? 'What would you like to change?' : 'Describe a website, app, or interaction you have in mind…'}
      disabled={disabled} onInput={(e) => onChange(e.currentTarget.value)}
      onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); if (!busy && value.trim() && !disabled) onSubmit(); } }}/>
    <div class="composer-tools">
      <button type="button" class={`composer-system ${systemName ? 'chosen' : ''}`} onClick={onSystem} title="Choose a design system" aria-label="Choose a design system"><Icon name="palette" size={18}/><span>{systemName || 'Design system'}</span></button>
      {!compact && <label class="surface-select"><Icon name={frame} size={17}/><select aria-label="Prototype device" value={frame} onChange={(e) => onFrame(e.currentTarget.value as Frame)}>{Object.entries(frames).map(([id, f]) => <option key={id} value={id}>{f.label}</option>)}</select></label>}
      <ModelButton compact={compact} disabled={busy || disabled}/>
      {busy && onStop ? <IconButton icon="stop" label="Stop generation" onClick={onStop} class="send-button"/> : <button class={`send-button ${!compact ? 'create-button' : ''}`} type="submit" aria-label={compact ? 'Send message' : 'Create prototype'} title="Send · ⌘ Enter" disabled={disabled || busy || !value.trim()}>{!compact && <span>{busy ? 'Creating…' : 'Create'}</span>}<Icon name="arrow" size={compact ? 23 : 18}/></button>}
    </div>
  </form>;
}
