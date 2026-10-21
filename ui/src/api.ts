export interface Project {
  id: string; name: string; slug: string; designSystemId?: string; skillId?: string;
  entryFile?: string; createdAt: string; updatedAt: string;
}
export interface FileEntry { name: string; path: string; size: number; kind: string; mime?: string }
export interface DesignSystem {
  id: string; name: string; category?: string; description?: string; slug: string;
  source?: string; hasPreview: boolean; files?: string[];
}
export interface SystemContext {
  id: string; name: string; designMd: string; tokensCss?: string; usageMd?: string;
}
export interface Skill { id: string; name: string; description?: string; mode?: string; hasScripts: boolean }
export interface Message { id: string; conversationId: string; role: string; content: string; createdAt: string }
export interface Conversation { id: string; title: string; projectId: string }
export interface Run {
  id: string; projectId: string; conversationId?: string;
  status: 'running' | 'succeeded' | 'failed' | 'canceled'; error?: string; startedAt: string;
}
export interface Health { ok: boolean; piBin: string; projectRoot: string; designSystems: number; skills: number }
export interface PiModel { id: string; provider: string; name: string; reasoning: boolean; contextWindow?: number }
export interface ModelCatalog { models: PiModel[]; current?: PiModel; thinkingLevels: string[] }
export interface Connection { available: boolean; config: Record<string, unknown> }
export interface AgentEvent { type: string; text?: string; tool?: string; args?: Record<string, unknown>; isError?: boolean }

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { 'Content-Type': 'application/json', ...init?.headers } });
  const contentType = res.headers.get('content-type') || '';
  if (!contentType.includes('application/json')) throw new Error('The workspace is unavailable. Reopen the app and try again.');
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status}). Please try again.`);
  return data as T;
}
const json = (body: unknown, method = 'POST') => ({ method, body: JSON.stringify(body) });
const projectPath = (id: string) => `/api/projects/${encodeURIComponent(id)}`;
export const api = {
  health: () => req<Health>('/api/health'),
  connection: () => req<Connection>('/api/connection'),
  preferences: () => req<{ model: string; thinking: string }>('/api/preferences'),
  savePreferences: (p: { model: string; thinking: string }) => req<{ model: string; thinking: string }>('/api/preferences', json(p, 'PUT')),
  models: (refresh = false) => req<ModelCatalog>(`/api/models${refresh ? '?refresh=1' : ''}`),
  thinkingLevels: (model: string) => req<{ levels: string[] }>(`/api/models/thinking?model=${encodeURIComponent(model)}`),
  projects: () => req<{ projects: Project[] }>('/api/projects'),
  createProject: (body: { name: string; designSystemId?: string; skillId?: string }) =>
    req<{ project: Project; conversationId: string }>('/api/projects', json(body)),
  project: (id: string) => req<{ project: Project; conversations: Conversation[] }>(projectPath(id)),
  updateProject: (p: Project) => req<{ project: Project }>(projectPath(p.id), json(pickProject(p), 'PATCH')),
  files: (id: string) => req<{ files: FileEntry[] }>(`${projectPath(id)}/files`),
  file: (id: string, path: string) => req<{ content: string }>(`${projectPath(id)}/file?path=${encodeURIComponent(path)}`),
  writeFile: (id: string, path: string, content: string) => req(`${projectPath(id)}/file`, json({ path, content }, 'PUT')),
  conversations: (id: string) => req<{ conversations: Conversation[] }>(`${projectPath(id)}/conversations`),
  createConversation: (id: string) => req<{ conversation: Conversation }>(`${projectPath(id)}/conversations`, json({ title: 'New conversation' })),
  messages: (id: string) => req<{ messages: Message[] }>(`/api/conversations/${encodeURIComponent(id)}/messages`),
  chat: (id: string, body: Record<string, unknown>) => req<{ runId: string }>(`${projectPath(id)}/chat`, json(body)),
  runs: (id: string) => req<{ runs: Run[] }>(`${projectPath(id)}/runs`),
  run: (id: string) => req<{ run: Run }>(`/api/runs/${encodeURIComponent(id)}`),
  cancelRun: (id: string) => req(`/api/runs/${encodeURIComponent(id)}/cancel`, { method: 'POST' }),
  systems: () => req<{ designSystems: DesignSystem[] }>('/api/design-systems'),
  system: (id: string) => req<SystemContext>(`/api/design-systems/${encodeURIComponent(id)}/context`),
  skills: () => req<{ skills: Skill[] }>('/api/skills'),
  skillDocument: (id: string) => req<{ content: string }>(`/api/skills/${encodeURIComponent(id)}/document`),
};
function pickProject(p: Project) { return { name: p.name, designSystemId: p.designSystemId || '', skillId: p.skillId || '' }; }

// Polling is also used by the run monitor: an EventSource reconnect or a very
// fast run cannot strand the UI in a permanent "working" state.
export function subscribeRun(runId: string, onEvent: (event: AgentEvent) => void) {
  const es = new EventSource(`/api/runs/${encodeURIComponent(runId)}/events`);
  for (const type of ['text', 'tool', 'tool_end', 'turn', 'model', 'done', 'error', 'canceled', 'stderr']) {
    es.addEventListener(type, ((e: MessageEvent) => {
      if (!e.data) return; // browser transport errors are not agent errors
      try { onEvent(JSON.parse(e.data)); } catch { /* ignore incomplete transport frames */ }
    }) as EventListener);
  }
  return () => es.close();
}

export function previewURL(id: string, entry = 'index.html') {
  return `/preview/${encodeURIComponent(id)}/${entry.split('/').map(encodeURIComponent).join('/')}`;
}
