// Typed API client for the pidesigner daemon.

export interface Project {
  id: string;
  name: string;
  slug: string;
  designSystemId?: string;
  skillId?: string;
  entryFile?: string;
  createdAt: string;
  updatedAt: string;
}

export interface FileEntry {
  name: string;
  path: string;
  size: number;
  kind: string;
  mime?: string;
}

export interface DesignSystem {
  id: string;
  name: string;
  category?: string;
  description?: string;
  slug: string;
  source?: string;
  hasPreview: boolean;
  files?: string[];
}

export interface Skill {
  id: string;
  name: string;
  description?: string;
  triggers?: string[];
  mode?: string;
  scenario?: string;
  hasScripts: boolean;
}

export interface Template {
  id: string;
  name: string;
  description?: string;
  mode?: string;
  scenario?: string;
  triggers?: string[];
}

export interface Message {
  id: string;
  conversationId: string;
  role: 'user' | 'assistant' | 'event';
  content: string;
  createdAt: string;
}

export interface Run {
  id: string;
  projectId: string;
  conversationId?: string;
  status: 'running' | 'succeeded' | 'failed' | 'canceled';
  prompt?: string;
  error?: string;
  startedAt: string;
  endedAt?: string;
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  });
  const text = await res.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    throw new Error(`bad response from ${path}: ${text.slice(0, 200)}`);
  }
  if (!res.ok) throw new Error(data?.error ?? `request failed (${res.status})`);
  return data as T;
}

export const api = {
  health: () => req<any>('/api/health'),

  listProjects: () => req<{ projects: Project[] }>('/api/projects'),
  createProject: (body: { name: string; designSystemId?: string; skillId?: string }) =>
    req<{ project: Project; conversationId: string }>('/api/projects', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  getProject: (id: string) => req<{ project: Project; conversations: any[] }>(`/api/projects/${id}`),
  deleteProject: (id: string) => req<any>(`/api/projects/${id}`, { method: 'DELETE' }),

  listFiles: (id: string) => req<{ files: FileEntry[] }>(`/api/projects/${id}/files`),
  getFile: (id: string, path: string) =>
    req<{ path: string; content: string }>(`/api/projects/${id}/file?path=${encodeURIComponent(path)}`),
  writeFile: (id: string, path: string, content: string) =>
    req<any>(`/api/projects/${id}/file`, {
      method: 'PUT',
      body: JSON.stringify({ path, content }),
    }),
  deleteFile: (id: string, path: string) =>
    req<any>(`/api/projects/${id}/file?path=${encodeURIComponent(path)}`, { method: 'DELETE' }),
  search: (id: string, q: string) =>
    req<{ files: FileEntry[] }>(`/api/projects/${id}/search?q=${encodeURIComponent(q)}`),

  listConversations: (id: string) => req<{ conversations: any[] }>(`/api/projects/${id}/conversations`),
  createConversation: (id: string, title?: string) =>
    req<{ conversation: any }>(`/api/projects/${id}/conversations`, {
      method: 'POST',
      body: JSON.stringify({ title }),
    }),
  listMessages: (convId: string) => req<{ messages: Message[] }>(`/api/conversations/${convId}/messages`),

  chat: (projectId: string, body: Record<string, unknown>) =>
    req<{ runId: string; conversationId?: string }>(`/api/projects/${projectId}/chat`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  getRun: (runId: string) => req<{ run: Run }>(`/api/runs/${runId}`),
  cancelRun: (runId: string) => req<any>(`/api/runs/${runId}/cancel`, { method: 'POST' }),

  listSkills: () => req<{ skills: Skill[] }>('/api/skills'),
  listTemplates: () => req<{ templates: Template[] }>('/api/templates'),
  listDesignSystems: () => req<{ designSystems: DesignSystem[] }>('/api/design-systems'),
};

/**
 * Subscribe to a run's agent event stream.
 * Returns an unsubscribe function.
 */
export function subscribeRun(
  runId: string,
  onEvent: (e: any) => void,
  onDone?: () => void
): () => void {
  const es = new EventSource(`/api/runs/${runId}/events`);
  const types = ['text', 'tool', 'tool_end', 'turn', 'done', 'error', 'canceled', 'stderr'];
  const handlers = types.map((t) => {
    const h = (ev: MessageEvent) => {
      try {
        onEvent(JSON.parse(ev.data));
      } catch {
        /* ignore malformed frames */
      }
      if (t === 'done' || t === 'error' || t === 'canceled') {
        onDone?.();
      }
    };
    es.addEventListener(t, h as EventListener);
    return [t, h] as const;
  });
  return () => {
    handlers.forEach(([t, h]) => es.removeEventListener(t, h as EventListener));
    es.close();
  };
}
