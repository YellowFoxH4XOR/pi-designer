import { useLayoutEffect, useRef, useState } from 'preact/hooks';
import type { RefObject } from 'preact';
import { previewURL, type FileEntry } from '../api';
import type { Frame } from '../state';
import { Icon } from './Icon';

export interface RefinementTarget { file: string; selector: string; frame: Frame }
export function ElementPicker({ iframe, project, files, frame, onPick }: {
  iframe: RefObject<HTMLIFrameElement>; project: string; files: FileEntry[]; frame: Frame; onPick: (target: RefinementTarget) => void;
}) {
  const [token, setToken] = useState('');
  const button = useRef<HTMLButtonElement>(null);
  useLayoutEffect(() => {
    if (!token) return;
    const element = iframe.current, target = element?.contentWindow;
    if (!target || !element) { setToken(''); return; }
    function cancel() { setToken(''); }
    function receive(e: MessageEvent) {
      const d = e.data;
      if (e.source !== target || e.origin !== 'null' || d?.source !== 'pidesigner-preview' || d.type !== 'element-picked' || d.token !== token) return;
      cancel();
      const file = files.find(f => /\.html?$/i.test(f.path) && (previewURL(project, f.path) === d.pathname || (f.path.endsWith('index.html') && previewURL(project, f.path).replace(/index\.html$/, '') === d.pathname)));
      if (!file || typeof d.selector !== 'string' || d.selector.length > 2048 || !/^[a-z][a-z0-9-]*:nth-of-type\([1-9]\d*\)( > [a-z][a-z0-9-]*:nth-of-type\([1-9]\d*\))*$/.test(d.selector)) {
        button.current?.focus(); return;
      }
      onPick({file: file.path, selector: d.selector, frame});
    }
    const escape = (e: KeyboardEvent) => { if (e.key === 'Escape') { cancel(); button.current?.focus(); } };
    const arm = () => target.postMessage({source:'pidesigner', type:'pick-element', token}, '*'); // sandbox has an opaque origin
    window.addEventListener('message', receive);
    window.addEventListener('keydown', escape);
    element.addEventListener('load', arm);
    arm();
    element.focus();
    return () => {
      window.removeEventListener('message', receive); window.removeEventListener('keydown', escape);
      element.removeEventListener('load', arm);
      target.postMessage({source:'pidesigner', type:'pick-element', token:null}, '*');
    };
  }, [token, iframe, project, files, frame, onPick]);
  return <button ref={button} class="fit-button pick-element" aria-label="Select element to refine" aria-pressed={!!token}
    title={token ? 'Click an element, or Tab and Enter. Escape cancels.' : 'Select an element to refine with pi'}
    onClick={() => setToken(token ? '' : crypto.randomUUID())}><Icon name="message" size={16}/>{token ? 'Picking…' : 'Refine'}</button>;
}
