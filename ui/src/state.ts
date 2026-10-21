import { useEffect, useRef, useState } from 'preact/hooks';
import { api, subscribeRun, type Run } from './api';
import { fromActivity, reduceActivity, type RunState } from './activity';
export type { RunState } from './activity';

export type Frame = 'web' | 'mobile' | 'desktop';
export const frames: Record<Frame, { w: number; h: number; label: string }> = {
  web: { w: 1440, h: 900, label: 'Web' }, mobile: { w: 390, h: 844, label: 'Mobile' }, desktop: { w: 1280, h: 800, label: 'Desktop' },
};
export const workflows = [
  { id: 'web-prototype', name: 'Prototype', icon: 'web' as const, description: 'A working interface, built from your brief.' },
  { id: 'motion-design', name: 'Motion design', icon: 'motion' as const, description: 'Bring a page or interaction to life.' },
  { id: 'simple-deck', name: 'Slide deck', icon: 'deck' as const, description: 'A swipeable presentation, slide by slide.' },
  { id: 'web-clone', name: 'Website clone', icon: 'clone' as const, description: 'Rebuild a reference website with omp.' },
];
export const skillNames: Record<string, string> = {
  'web-prototype': 'Prototype', 'web-artifacts-builder': 'Prototype', 'motion-design': 'Motion design', 'web-clone': 'Website clone',
  'simple-deck': 'Slide deck', 'emilkowalski-motion': 'Interaction motion', 'web-design-guidelines': 'Interface review',
};
export interface Preferences { model: string; thinking: string }
export function loadPreferences(): Preferences {
  try { const p = JSON.parse(localStorage.getItem('pidesigner.preferences') || '{}'); return { model: p.model || '', thinking: p.thinking || 'medium' }; }
  catch { return { model: '', thinking: 'medium' }; }
}
export function useRoute() {
  const [route, setRoute] = useState(location.hash.slice(1) || 'home');
  useEffect(() => {
    let current = location.hash.slice(1) || 'home';
    const h = () => {
      const next = location.hash.slice(1) || 'home';
      if (next === current) return;
      const request = new Event('pidesigner:navigate', { cancelable: true });
      if (current.split('?')[0] !== next.split('?')[0] && !window.dispatchEvent(request)) {
        history.replaceState(null, '', `${location.pathname}${location.search}#${current}`);
        return;
      }
      current = next; setRoute(next);
    };
    window.addEventListener('hashchange', h);
    return () => window.removeEventListener('hashchange', h);
  }, []);
  return [route, (next: string) => { location.hash = next; }] as const;
}
// Discover work globally, including runs started by an editor/MCP. Each stream
// starts after an authoritative snapshot and resumes using a durable cursor.
export function useRuns(onFinished: () => void) {
  const [runs, setRuns] = useState<Record<string, RunState>>({});
  const [activityError, setActivityError] = useState('');
  const state = useRef(runs); state.current = runs;
  const subscriptions = useRef(new Map<string, () => void>());
  const loading = useRef(new Set<string>());
  const mounted = useRef(true);
  const finished = useRef(onFinished); finished.current = onFinished;

  async function monitor(run: Pick<Run, 'id'> & Partial<Run>) {
    if (subscriptions.current.has(run.id) || loading.current.has(run.id)) return;
    loading.current.add(run.id);
    try {
      const { activity } = await api.runActivity(run.id);
      if (!mounted.current) return;
      const initial = fromActivity(activity);
      setRuns(all => all[activity.projectId]?.startedAt > activity.startedAt ? all : ({ ...all, [activity.projectId]: initial }));
      if (activity.status !== 'running') return;
      const stop = subscribeRun(run.id, activity.seq, event => {
        if (!mounted.current) return;
        setRuns(all => {
          const current = all[activity.projectId];
          if (!current || current.id !== run.id) return all;
          return { ...all, [activity.projectId]: reduceActivity(current, event) };
        });
      }, snapshot => {
        if (!mounted.current) return;
        setRuns(all => {
          const current = all[snapshot.projectId];
          if (current && current.id !== run.id) return all;
          return { ...all, [snapshot.projectId]: fromActivity(snapshot) };
        });
        if (snapshot.status !== 'running') { subscriptions.current.get(run.id)?.(); subscriptions.current.delete(run.id); finished.current(); }
      }, connected => {
        if (!mounted.current) return;
        setRuns(all => { const current = all[activity.projectId]; return current?.id === run.id ? { ...all, [activity.projectId]: { ...current, connection: connected ? 'connected' : 'reconnecting' } } : all; });
      });
      subscriptions.current.set(run.id, stop);
    } catch (e) { if (mounted.current) setActivityError(String(e).replace(/^Error: /, '')); }
    finally { loading.current.delete(run.id); }
  }
  useEffect(() => {
    mounted.current = true; let pending = false;
    async function sync() {
      if (pending) return; pending = true;
      try {
        const { activities } = await api.activity();
        if (!mounted.current) return;
        setActivityError('');
        for (const a of activities) {
          const current = state.current[a.projectId];
          if (!current || current.id !== a.runId || current.status !== a.status || current.attention !== a.attention || !subscriptions.current.has(a.runId) && current.seq !== a.seq) {
            // Authoritative discovery is also the fallback when an SSE connection
            // dies before its terminal snapshot arrives.
            if (current && current.id !== a.runId) { subscriptions.current.get(current.id)?.(); subscriptions.current.delete(current.id); }
            subscriptions.current.get(a.runId)?.(); subscriptions.current.delete(a.runId);
            void monitor({ id: a.runId });
          }
        }
      } catch { if (mounted.current) setActivityError('Activity disconnected. Reconnecting…'); }
      finally { pending = false; }
    }
    void sync(); const timer = window.setInterval(sync, 1500);
    return () => { mounted.current = false; clearInterval(timer); subscriptions.current.forEach(stop => stop()); subscriptions.current.clear(); };
  }, []);
  // A deleted project's activity is gone server-side; drop it locally too.
  function forget(projectId: string) {
    const current = state.current[projectId];
    if (current) { subscriptions.current.get(current.id)?.(); subscriptions.current.delete(current.id); }
    setRuns(({ [projectId]: _removed, ...rest }) => rest);
  }
  return { runs, monitor, forget, activityError };
}
