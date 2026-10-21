import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import type { ComponentChildren } from 'preact';
import { Icon, type IconName } from './Icon';

export function IconButton({ icon, label, onClick, disabled = false, class: className = '' }: {
  icon: IconName; label: string; onClick: () => void; disabled?: boolean; class?: string;
}) {
  return <button class={`icon-button ${className}`} onClick={onClick} title={label} aria-label={label} disabled={disabled}><Icon name={icon} /></button>;
}
export function Empty({ icon, title, children }: { icon: IconName; title: string; children?: ComponentChildren }) {
  return <div class="empty-state"><Icon name={icon} size={30}/><h3>{title}</h3>{children}</div>;
}
export function Dialog({ title, children, onClose, class: className = '' }: { title: string; children: ComponentChildren; onClose: () => void; class?: string }) {
  const ref = useRef<HTMLDialogElement>(null);
  useLayoutEffect(() => { const dialog = ref.current; dialog?.showModal(); return () => dialog?.close(); }, []);
  return <dialog ref={ref} class={`dialog ${className}`} aria-label={title} onCancel={(e) => { e.preventDefault(); onClose(); }} onClick={(e) => { if (e.target === ref.current) onClose(); }}>
    <div class="dialog-head"><h2>{title}</h2><IconButton icon="close" label="Close dialog" onClick={onClose}/></div>{children}
  </dialog>;
}
export async function copyText(text: string) {
  if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(text); return; }
  const field = document.createElement('textarea'); field.value = text;
  field.style.cssText = 'position:fixed;left:-9999px'; document.body.appendChild(field); field.select();
  const ok = document.execCommand('copy'); field.remove();
  if (!ok) throw new Error('Copy was blocked. Select and copy the text instead.');
}
export function CopyButton({ text, label = 'Copy', onError }: { text: string; label?: string; onError?: (s: string) => void }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => { if (!copied) return; const t = window.setTimeout(() => setCopied(false), 1800); return () => clearTimeout(t); }, [copied]);
  return <button class="button secondary" onClick={async () => { try { await copyText(text); setCopied(true); } catch (e) { onError?.(String(e)); } }}><Icon name={copied ? 'check' : 'copy'} size={16}/>{copied ? 'Copied' : label}</button>;
}
export function SearchField({ value, onInput, placeholder }: { value: string; onInput: (v: string) => void; placeholder: string }) {
  return <div class="search-field"><Icon name="search" size={17}/><input type="search" aria-label={placeholder} placeholder={placeholder} value={value} onInput={(e) => onInput(e.currentTarget.value)}/></div>;
}
export function relativeDate(value: string) {
  const diff = Math.max(0, Date.now() - new Date(value).getTime());
  if (diff < 60_000) return 'Just now';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  if (diff < 604_800_000) return `${Math.floor(diff / 86_400_000)}d ago`;
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
