import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import { api, previewURL, versionDownloadURL, versionPreviewURL, type ArtifactVersion, type FileEntry, type Project } from '../api';
import { frames, type Frame } from '../state';
import { copyText, Empty, IconButton } from './common';
import { Icon } from './Icon';
import { completeSourceSave } from '../source-editor-save';

export interface FileChange { path: string; kind: string; stamp: string }
export function PreviewPane({ project, files, frame, onFrame, changes, busy, activityLabel, onError, versions = [], viewing = null, onView, onRestore, expanded = false, onExpand, onClose }: {
  project: Project; files: FileEntry[]; frame: Frame; onFrame: (f: Frame) => void;
  changes: FileChange[]; busy: boolean; activityLabel?: string; onError: (s: string) => void;
  /** Saved versions, oldest first; viewing is a version number or null for the live files. */
  versions?: ArtifactVersion[]; viewing?: number | null; onView?: (n: number | null) => void;
  onRestore?: (n: number) => Promise<void>; expanded?: boolean; onExpand?: () => void; onClose?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const latest = versions[versions.length - 1];
  const shown = viewing !== null ? versions.find((v) => v.n === viewing) : undefined;
  const position = shown ? versions.indexOf(shown) : versions.length - 1;
  useEffect(() => { if (!copied) return; const t = window.setTimeout(() => setCopied(false), 1600); return () => clearTimeout(t); }, [copied]);
  const [view, setView] = useState<'preview' | 'code'>('preview');
  const [entry, setEntry] = useState(project.entryFile || 'index.html');
  const [reload, setReload] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [fit, setFit] = useState(true);
  const [bounds, setBounds] = useState({ width: 900, height: 700 });
  const stage = useRef<HTMLDivElement>(null);
  const iframe = useRef<HTMLIFrameElement>(null);
  const htmlFiles = shown ? shown.files.filter((f) => /\.html?$/i.test(f)).map((path) => ({ path })) : files.filter((f) => /\.html?$/i.test(f.path));
  const hasEntry = htmlFiles.some((f) => f.path === entry);
  useEffect(() => {
    if (!htmlFiles.length || hasEntry) return;
    setEntry(htmlFiles.find((f) => f.path === (shown?.entry || 'index.html'))?.path || htmlFiles.find((f) => f.path === 'index.html')?.path || htmlFiles[0].path);
  }, [files, viewing]);
  useEffect(() => {
    const element = stage.current;
    if (!element) return;
    const ro = new ResizeObserver(([entry]) => setBounds({ width: entry.contentRect.width, height: entry.contentRect.height }));
    ro.observe(element); return () => ro.disconnect();
  }, []);
  useEffect(() => {
    if (!changes.length || shown) return;
    if (changes.some((c) => c.kind !== 'css')) setReload((n) => n + 1);
    else iframe.current?.contentWindow?.postMessage({ source: 'pidesigner', type: 'css-update', files: changes.map((c) => ({ ...c, name: c.path.split('/').pop() })) }, '*');
  }, [changes]);
  const f = frames[frame];
  const scale = fit ? Math.max(.1, Math.min(1, (bounds.width - 48) / f.w, (bounds.height - 48) / f.h)) : 1;
  function switchView(next: 'preview' | 'code') {
    if (dirty && !confirm('Discard your unsaved source changes?')) return;
    setDirty(false); setView(next);
  }
  const src = shown ? versionPreviewURL(project.id, shown.n, entry) : `${previewURL(project.id, entry)}?revision=${reload}`;
  const step = (delta: number) => {
    const next = versions[position + delta];
    if (next) onView?.(next === latest ? null : next.n);
  };
  async function copySource() {
    try {
      const text = shown ? await (await fetch(versionPreviewURL(project.id, shown.n, entry))).text() : (await api.file(project.id, entry)).content;
      await copyText(text); setCopied(true);
    } catch (e) { onError(e instanceof Error ? e.message : String(e)); }
  }
  async function restore() {
    if (!shown || !onRestore) return;
    setRestoring(true);
    try { await onRestore(shown.n); } finally { setRestoring(false); }
  }
  const downloadN = shown?.n ?? latest?.n;
  return <section class={`preview-pane ${expanded ? 'expanded' : ''}`} aria-label="Design canvas">
    <div class="canvas-tabs artifact-header">
      <span class="artifact-title"><Icon name="file" size={15}/>{htmlFiles.length > 1
        ? <select aria-label="Artifact file" value={entry} onChange={(e) => setEntry(e.currentTarget.value)}>{htmlFiles.map((f) => <option key={f.path} value={f.path}>{f.path}</option>)}</select>
        : <span>{entry}</span>}</span>
      {versions.length > 0 && <div class="version-nav" role="group" aria-label="Versions">
        <IconButton icon="chevron-left" label="Previous version" onClick={() => step(-1)} disabled={position <= 0}/>
        <select aria-label="Version" value={shown ? String(shown.n) : 'live'} onChange={(e) => { const v = e.currentTarget.value; onView?.(v === 'live' || Number(v) === latest?.n ? null : Number(v)); }}>
          {[...versions].reverse().map((v) => <option key={v.n} value={v === latest ? 'live' : String(v.n)}>Version {v.n}{v === latest ? ' · Latest' : ''}</option>)}
        </select>
        <IconButton icon="chevron" label="Next version" onClick={() => step(1)} disabled={!shown}/>
      </div>}
      <span class="canvas-tab-note">{busy ? activityLabel || 'Starting pi' : shown ? `Saved ${new Date(shown.createdAt).toLocaleString()}` : 'Project files'}</span>
      <div class="artifact-actions">
        <IconButton icon={copied ? 'check' : 'copy'} label={copied ? 'Copied' : `Copy ${entry} source`} onClick={() => void copySource()} disabled={!hasEntry}/>
        {downloadN ? <a class="icon-button" href={versionDownloadURL(project.id, downloadN)} download aria-label={`Download version ${downloadN}`} title={`Download version ${downloadN} (.zip)`}><Icon name="download" size={17}/></a> : <IconButton icon="download" label="Nothing to download yet" onClick={() => {}} disabled/>}
        {onExpand && <IconButton icon={expanded ? 'collapse' : 'expand'} label={expanded ? 'Show the conversation' : 'Expand the canvas'} onClick={onExpand}/>}
        {onClose && <IconButton icon="close" label="Close the canvas" onClick={onClose}/>}
      </div>
    </div>
    {shown && <div class="version-banner" role="status"><Icon name="history" size={15}/><span>You’re viewing version {shown.n} of {versions.length}{shown.label ? ` · ${shown.label}` : ''}. pi’s next turn starts from the latest version.</span><button class="button secondary small" onClick={() => onView?.(null)}>Back to latest</button><button class="button primary small" onClick={() => void restore()} disabled={busy || restoring}>{restoring ? 'Restoring…' : 'Restore this version'}</button></div>}
    <div class="canvas-toolbar"><div class="segmented view-switch" aria-label="Canvas view">{(['preview', 'code'] as const).map((v) => <button key={v} aria-pressed={view === v} class={view === v ? 'active' : ''} onClick={() => switchView(v)}><Icon name={v === 'preview' ? 'eye' : 'code'} size={16}/>{v === 'preview' ? 'Preview' : 'Code'}</button>)}</div>
      {view === 'preview' && <><div class="toolbar-divider"/><div class="device-switch" aria-label="Preview device">{(Object.keys(frames) as Frame[]).map((id) => <button key={id} class={frame === id ? 'active' : ''} aria-label={`${frames[id].label} preview`} title={`${frames[id].label} · ${frames[id].w} × ${frames[id].h}`} aria-pressed={frame === id} onClick={() => onFrame(id)}><Icon name={id} size={17}/><span>{frames[id].label}</span></button>)}</div><div class="canvas-actions"><button class="fit-button" onClick={() => setFit(!fit)} title={fit ? 'Show at actual size' : 'Fit to canvas'}>{fit ? 'Fit' : '100%'}<span>{Math.round(scale * 100)}%</span></button><IconButton icon="refresh" label="Reload prototype" onClick={() => setReload((n) => n + 1)} disabled={!hasEntry || !!shown}/>{hasEntry && <a class="icon-button" aria-label="Open preview in browser" title="Open preview in browser" href={shown ? versionPreviewURL(project.id, shown.n, entry) : previewURL(project.id, entry)} target="_blank" rel="noreferrer"><Icon name="external" size={17}/></a>}</div></>}
    </div>
    <div ref={stage} class={`canvas-stage ${view === 'code' ? 'hidden' : ''} ${fit ? 'fit' : 'actual-size'}`}>
      {!hasEntry ? <Empty icon="web" title={busy ? 'Your idea is taking shape' : 'A canvas for your idea'}><p>{busy ? 'The preview will appear as soon as pi writes the first page.' : 'Describe what you want to build in the conversation.'}</p>{busy && <span class="working-indicator"><i/><i/><i/></span>}</Empty> : <div class={`device-viewport frame-${frame}`} style={{ width: f.w * scale, height: f.h * scale }}><iframe ref={iframe} title={`${f.label} prototype · ${f.w} by ${f.h}${shown ? ` · version ${shown.n}` : ''}`} sandbox="allow-scripts allow-forms" src={src} style={{ width: f.w, height: f.h, transform: `scale(${scale})` }}/></div>}
    </div>
    {view === 'code' && (shown ? <VersionSource projectId={project.id} version={shown} defaultFile={entry} onError={onError}/> : <SourceEditor projectId={project.id} files={files} defaultFile={entry} busy={busy} onDirty={setDirty} onError={onError}/>)}
    <div class="canvas-status"><span><span class={`status-dot ${busy ? 'working' : ''}`}/>{busy ? 'Updating your design' : shown ? `Version ${shown.n} · read-only` : 'Local preview'}</span><span>{view === 'preview' ? `${f.w} × ${f.h}` : `${shown ? shown.files.length : files.length} files`}<span class="status-separator"/>{shown ? 'Saved version' : 'Changes saved in your project'}</span></div>
  </section>;
}
/** Read-only source of a saved version: it cannot be edited, only restored. */
function VersionSource({ projectId, version, defaultFile, onError }: { projectId: string; version: ArtifactVersion; defaultFile: string; onError: (s: string) => void }) {
  const textFiles = version.files.filter((f) => /\.(html?|css|m?js|json|svg|md|txt)$/i.test(f));
  const [selected, setSelected] = useState(textFiles.includes(defaultFile) ? defaultFile : textFiles[0] || '');
  const [content, setContent] = useState('');
  useEffect(() => {
    if (!selected) return;
    let active = true;
    fetch(versionPreviewURL(projectId, version.n, selected)).then((r) => { if (!r.ok) throw new Error(`Could not read ${selected} from version ${version.n}.`); return r.text(); })
      .then((t) => { if (active) setContent(t); }).catch((e) => { if (active) onError(e.message); });
    return () => { active = false; };
  }, [projectId, version.n, selected]);
  return <div class="source-editor"><aside class="source-tree" aria-label={`Files in version ${version.n}`}><h3>Version {version.n}</h3>{textFiles.map((f) => <button key={f} class={f === selected ? 'active' : ''} onClick={() => setSelected(f)}><Icon name="file" size={14}/><span>{f}</span></button>)}</aside><div class="source-main"><div class="source-toolbar"><span>{selected} · read-only</span></div><textarea class="code-input" aria-label={`Source for ${selected} in version ${version.n}`} spellcheck={false} readOnly value={content}/></div></div>;
}
function SourceEditor({ projectId, files, defaultFile, busy, onDirty, onError }: { projectId: string; files: FileEntry[]; defaultFile: string; busy: boolean; onDirty: (b: boolean) => void; onError: (s: string) => void }) {
  const [selected, setSelected] = useState(defaultFile);
  const [content, setContent] = useState('');
  const [original, setOriginal] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const projectIdRef = useRef(projectId);
  projectIdRef.current = projectId;
  const contentRef = useRef(content);
  contentRef.current = content;
  const originalRef = useRef(original);
  originalRef.current = original;
  const editorGeneration = useRef(0);
  const savingRef = useRef(false);
  const mountedRef = useRef(true);
  const dirty = content !== original;
  const editableFiles = files.filter((f) => ['code', 'markdown', 'text'].includes(f.kind));
  useEffect(() => () => { mountedRef.current = false; }, []);
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
  function chooseFile(path: string) {
    if (savingRef.current || path === selectedRef.current) return;
    if (dirty && !confirm('Discard unsaved changes to this file?')) return;
    selectedRef.current = path;
    editorGeneration.current += 1;
    setSelected(path);
  }
  useLayoutEffect(() => {
    editorGeneration.current += 1;
    let active = true;
    if (!editableFiles.some((f) => f.path === selected)) {
      if (editableFiles.length) setSelected(editableFiles[0].path);
      return;
    }
    setLoading(true); setFailure('');
    api.file(projectId, selected).then((d) => {
      if (active) {
        contentRef.current = d.content;
        originalRef.current = d.content;
        setContent(d.content);
        setOriginal(d.content);
      }
    }).catch((e) => { if (active) setFailure(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [projectId, selected]);
  async function save() {
    if (!dirty || savingRef.current || busy) return;
    const request = {
      projectId,
      filePath: selected,
      generation: editorGeneration.current,
      content,
    };
    savingRef.current = true;
    setSaving(true);
    try {
      await api.writeFile(request.projectId, request.filePath, request.content);
      if (!mountedRef.current) return;
      const current = {
        projectId: projectIdRef.current,
        filePath: selectedRef.current,
        generation: editorGeneration.current,
        content: contentRef.current,
        original: originalRef.current,
      };
      const completed = completeSourceSave(current, request);
      if (completed !== current) {
        originalRef.current = completed.original;
        setOriginal(completed.original);
      }
    } catch (e) {
      if (mountedRef.current) onError(String(e));
    } finally {
      savingRef.current = false;
      if (mountedRef.current) setSaving(false);
    }
  }
  return <div class="source-editor"><aside class="source-tree" aria-label="Project files"><h3>Files</h3>{files.map((f) => <button key={f.path} disabled={!editableFiles.includes(f) || saving} class={f.path === selected ? 'active' : ''} onClick={() => chooseFile(f.path)}><Icon name="file" size={14}/><span>{f.path}</span></button>)}</aside><div class="source-main"><div class="source-toolbar"><span>{selected}{dirty ? ' · Unsaved' : ''}</span><button class="button primary small" disabled={!dirty || saving || busy || loading} onClick={save}>{saving ? 'Saving…' : 'Save changes'}</button></div>{failure ? <p class="inline-error" role="alert">{failure}</p> : loading ? <div class="loading-state">Loading source…</div> : !editableFiles.length ? <Empty icon="code" title="No source files yet"><p>Files appear here as pi builds your design.</p></Empty> : <textarea class="code-input" aria-label={`Source for ${selected}`} spellcheck={false} readOnly={busy} value={content} onInput={(e) => { contentRef.current = e.currentTarget.value; setContent(e.currentTarget.value); }} onKeyDown={(e) => { if (e.key === 's' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); void save(); } }}/>}</div></div>;
}
