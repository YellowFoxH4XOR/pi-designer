import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import { api, previewURL, type FileEntry, type Project } from '../api';
import { frames, type Frame } from '../state';
import { Empty, IconButton } from './common';
import { Icon } from './Icon';

export interface FileChange { path: string; kind: string; stamp: string }
export function PreviewPane({ project, files, frame, onFrame, changes, busy, onError }: {
  project: Project; files: FileEntry[]; frame: Frame; onFrame: (f: Frame) => void;
  changes: FileChange[]; busy: boolean; onError: (s: string) => void;
}) {
  const [view, setView] = useState<'preview' | 'code'>('preview');
  const [entry, setEntry] = useState(project.entryFile || 'index.html');
  const [reload, setReload] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [fit, setFit] = useState(true);
  const [bounds, setBounds] = useState({ width: 900, height: 700 });
  const stage = useRef<HTMLDivElement>(null);
  const iframe = useRef<HTMLIFrameElement>(null);
  const htmlFiles = files.filter((f) => /\.html?$/i.test(f.path));
  const hasEntry = htmlFiles.some((f) => f.path === entry);
  useEffect(() => {
    if (!htmlFiles.length || hasEntry) return;
    setEntry(htmlFiles.find((f) => f.path === 'index.html')?.path || htmlFiles[0].path);
  }, [files]);
  useEffect(() => {
    const element = stage.current;
    if (!element) return;
    const ro = new ResizeObserver(([entry]) => setBounds({ width: entry.contentRect.width, height: entry.contentRect.height }));
    ro.observe(element); return () => ro.disconnect();
  }, []);
  useEffect(() => {
    if (!changes.length) return;
    if (changes.some((c) => c.kind !== 'css')) setReload((n) => n + 1);
    else iframe.current?.contentWindow?.postMessage({ source: 'pidesigner', type: 'css-update', files: changes.map((c) => ({ ...c, name: c.path.split('/').pop() })) }, '*');
  }, [changes]);
  const f = frames[frame];
  const scale = fit ? Math.max(.1, Math.min(1, (bounds.width - 48) / f.w, (bounds.height - 48) / f.h)) : 1;
  function switchView(next: 'preview' | 'code') {
    if (dirty && !confirm('Discard your unsaved source changes?')) return;
    setDirty(false); setView(next);
  }
  return <section class="preview-pane" aria-label="Design canvas">
    <div class="canvas-tabs"><span class="artifact-tab"><Icon name="file" size={15}/>{entry}</span><span class="canvas-tab-note">{busy ? 'Pi is working' : 'Project files'}</span></div>
    <div class="canvas-toolbar"><div class="segmented view-switch" aria-label="Canvas view">{(['preview', 'code'] as const).map((v) => <button key={v} aria-pressed={view === v} class={view === v ? 'active' : ''} onClick={() => switchView(v)}><Icon name={v === 'preview' ? 'eye' : 'code'} size={16}/>{v === 'preview' ? 'Preview' : 'Code'}</button>)}</div>
      {view === 'preview' && <><div class="toolbar-divider"/><div class="device-switch" aria-label="Preview device">{(Object.keys(frames) as Frame[]).map((id) => <button key={id} class={frame === id ? 'active' : ''} aria-label={`${frames[id].label} preview`} title={`${frames[id].label} · ${frames[id].w} × ${frames[id].h}`} aria-pressed={frame === id} onClick={() => onFrame(id)}><Icon name={id} size={17}/><span>{frames[id].label}</span></button>)}</div><div class="canvas-actions"><button class="fit-button" onClick={() => setFit(!fit)} title={fit ? 'Show at actual size' : 'Fit to canvas'}>{fit ? 'Fit' : '100%'}<span>{Math.round(scale * 100)}%</span></button><IconButton icon="refresh" label="Reload prototype" onClick={() => setReload((n) => n + 1)} disabled={!hasEntry}/>{hasEntry && <a class="icon-button" aria-label="Open preview in browser" title="Open preview in browser" href={previewURL(project.id, entry)} target="_blank" rel="noreferrer"><Icon name="external" size={17}/></a>}</div></>}
    </div>
    <div ref={stage} class={`canvas-stage ${view === 'code' ? 'hidden' : ''} ${fit ? 'fit' : 'actual-size'}`}>
      {!hasEntry ? <Empty icon="web" title={busy ? 'Your idea is taking shape' : 'A canvas for your idea'}><p>{busy ? 'The preview will appear as soon as Pi writes the first page.' : 'Describe what you want to build in the conversation.'}</p>{busy && <span class="working-indicator"><i/><i/><i/></span>}</Empty> : <div class={`device-viewport frame-${frame}`} style={{ width: f.w * scale, height: f.h * scale }}><iframe ref={iframe} title={`${f.label} prototype · ${f.w} by ${f.h}`} sandbox="allow-scripts allow-forms" src={`${previewURL(project.id, entry)}?revision=${reload}`} style={{ width: f.w, height: f.h, transform: `scale(${scale})` }}/></div>}
    </div>
    {view === 'code' && <SourceEditor projectId={project.id} files={files} defaultFile={entry} busy={busy} onDirty={setDirty} onError={onError}/>}
    <div class="canvas-status"><span><span class={`status-dot ${busy ? 'working' : ''}`}/>{busy ? 'Updating your design' : 'Local preview'}</span><span>{view === 'preview' ? `${f.w} × ${f.h}` : `${files.length} files`}<span class="status-separator"/>Changes saved in your project</span></div>
  </section>;
}
function SourceEditor({ projectId, files, defaultFile, busy, onDirty, onError }: { projectId: string; files: FileEntry[]; defaultFile: string; busy: boolean; onDirty: (b: boolean) => void; onError: (s: string) => void }) {
  const [selected, setSelected] = useState(defaultFile);
  const [content, setContent] = useState('');
  const [original, setOriginal] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const dirty = content !== original;
  const editableFiles = files.filter((f) => ['code', 'markdown', 'text'].includes(f.kind));
  useEffect(() => {
    onDirty(dirty);
    document.documentElement.dataset.unsavedChanges = String(dirty);
    const beforeNavigation = (e: Event) => {
      if (dirty && !confirm('Discard your unsaved source changes?')) e.preventDefault();
    };
    const beforeUnload = (e: BeforeUnloadEvent) => {
      if (dirty) { e.preventDefault(); e.returnValue = ''; }
    };
    window.addEventListener('pidesigner:navigate', beforeNavigation);
    window.addEventListener('beforeunload', beforeUnload);
    return () => {
      delete document.documentElement.dataset.unsavedChanges;
      window.removeEventListener('pidesigner:navigate', beforeNavigation);
      window.removeEventListener('beforeunload', beforeUnload);
    };
  }, [dirty]);
  useLayoutEffect(() => {
    let active = true;
    if (!editableFiles.some((f) => f.path === selected)) {
      if (editableFiles.length) setSelected(editableFiles[0].path);
      return;
    }
    setLoading(true); setFailure('');
    api.file(projectId, selected).then((d) => { if (active) { setContent(d.content); setOriginal(d.content); } }).catch((e) => { if (active) setFailure(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [projectId, selected]);
  async function save() {
    if (!dirty || saving || busy) return;
    setSaving(true);
    try { await api.writeFile(projectId, selected, content); setOriginal(content); }
    catch (e) { onError(String(e)); }
    finally { setSaving(false); }
  }
  return <div class="source-editor"><aside class="source-tree" aria-label="Project files"><h3>Files</h3>{files.map((f) => <button key={f.path} disabled={!editableFiles.includes(f)} class={f.path === selected ? 'active' : ''} onClick={() => { if (!dirty || confirm('Discard unsaved changes to this file?')) setSelected(f.path); }}><Icon name="file" size={14}/><span>{f.path}</span></button>)}</aside><div class="source-main"><div class="source-toolbar"><span>{selected}{dirty ? ' · Unsaved' : ''}</span><button class="button primary small" disabled={!dirty || saving || busy || loading} onClick={save}>{saving ? 'Saving…' : 'Save changes'}</button></div>{failure ? <p class="inline-error" role="alert">{failure}</p> : loading ? <div class="loading-state">Loading source…</div> : !editableFiles.length ? <Empty icon="code" title="No source files yet"><p>Files appear here as Pi builds your design.</p></Empty> : <textarea class="code-input" aria-label={`Source for ${selected}`} spellcheck={false} readOnly={busy} value={content} onInput={(e) => setContent(e.currentTarget.value)} onKeyDown={(e) => { if (e.key === 's' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); void save(); } }}/>}</div></div>;
}
