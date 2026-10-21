import { useEffect, useRef, useState } from 'preact/hooks';
import { api, subscribeRun, type Run, type AgentEvent, type Render, type Todo } from './api';

export type Frame = 'web' | 'mobile' | 'desktop';
export const frames: Record<Frame, { w: number; h: number; label: string }> = {
  web: { w: 1440, h: 900, label: 'Web' }, mobile: { w: 390, h: 844, label: 'Mobile' }, desktop: { w: 1280, h: 800, label: 'Desktop' },
};
export const workflows = [
  { id: 'web-prototype', name: 'Prototype', icon: 'web' as const, description: 'A working interface, built from your brief.' },
  { id: 'motion-design', name: 'Motion design', icon: 'motion' as const, description: 'Bring a page or interaction to life.' },
  { id: 'web-clone', name: 'Website clone', icon: 'clone' as const, description: 'Rebuild a reference website with Pi.' },
];
export const skillNames: Record<string, string> = {
  'web-prototype': 'Prototype', 'web-artifacts-builder': 'Prototype', 'motion-design': 'Motion design', 'web-clone': 'Website clone',
  'emilkowalski-motion': 'Interaction motion', 'web-design-guidelines': 'Interface review', 'agent-browser': 'Browser verification',
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
      if (!window.dispatchEvent(request)) {
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
export interface RunState {
  id: string; projectId: string; status: Run['status']; text: string; tools: AgentEvent[];
  todos: Todo[]; renders: Render[];
  error?: string; conversationId?: string; model?: string;
}
export function useRuns(onFinished: () => void) {
  const [runs, setRuns] = useState<Record<string, RunState>>({});
  const subscriptions = useRef(new Map<string, () => void>());
  const finished = useRef(onFinished); finished.current = onFinished;
  useEffect(() => () => { subscriptions.current.forEach((stop) => stop()); }, []);

  function monitor(run: Run) {
    if (subscriptions.current.has(run.id)) return;
    setRuns((all) => ({
      ...all,
      [run.projectId]: {
        id: run.id, projectId: run.projectId, status: run.status, text: '', tools: [],
        todos: run.todos ?? [], renders: run.renders ?? [], conversationId: run.conversationId,
      },
    }));
    let stopped = false, polling = false;
    const stopEvents = subscribeRun(run.id, (ev) => {
      if (stopped) return;
      setRuns((all) => {
        const current = all[run.projectId];
        if (!current || current.id !== run.id) return all;
        const next = { ...current };
        if (ev.type === 'text') next.text += ev.text || '';
        if (ev.type === 'model') next.model = ev.text || '';
        if (ev.type === 'turn' && !next.text.endsWith('\n\n')) next.text += '\n\n';
        if (ev.type === 'tool') next.tools = [...next.tools.slice(-29), ev];
        if (ev.type === 'todos' && Array.isArray(ev.todos)) next.todos = ev.todos;
        if (ev.type === 'render' && ev.render?.image && !next.renders.some((render) => render.image === ev.render?.image)) {
          next.renders = [...next.renders, ev.render];
        }
        return { ...all, [run.projectId]: next };
      });
    });
    const stop = () => { stopped = true; stopEvents(); clearInterval(timer); subscriptions.current.delete(run.id); };
    const poll = async () => {
      if (stopped || polling) return;
      polling = true;
      try {
        const { run: latest } = await api.run(run.id);
        if (latest.status !== 'running' && !stopped) {
          stop();
          setRuns((all) => {
            const current = all[run.projectId];
            return {
              ...all,
              [run.projectId]: {
                ...current,
                status: latest.status,
                error: latest.error,
                todos: latest.todos?.length ? latest.todos : current.todos,
                renders: latest.renders?.length ? latest.renders : current.renders,
              },
            };
          });
          finished.current();
        }
      } catch { /* preserve the run while the daemon is temporarily unreachable */ }
      finally { polling = false; }
    };
    const timer = window.setInterval(poll, 1500);
    subscriptions.current.set(run.id, stop);
    void poll();
  }
  return { runs, monitor };
}
