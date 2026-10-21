import type { Activity } from './activity';

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
export type TodoStatus = 'pending' | 'in_progress' | 'completed';
export interface Todo { content: string; status: TodoStatus }
export interface Render {
  viewport: 'web' | 'desktop' | 'mobile' | string; path?: string; image: string;
  overflowX?: boolean; errors?: number; slide?: number;
}
export type CritiqueRole = 'designer' | 'critic' | 'brand' | 'a11y' | 'copy';
export type CritiqueDecision = 'revise' | 'ship' | 'ship_with_issues';
export interface CritiquePanelist { role: CritiqueRole; score?: number; notes: string; mustFix: string[] }
export interface Critique { round: number; composite: number; decision: CritiqueDecision; panelists: CritiquePanelist[] }
export interface Run {
  id: string; projectId: string; conversationId?: string;
  status: 'running' | 'succeeded' | 'failed' | 'canceled'; error?: string; startedAt: string;
  todos?: Todo[]; renders?: Render[]; critiques?: Critique[];
}
export interface Health { ok: boolean; piBin: string; projectRoot: string; designSystems: number; skills: number }
export interface PiModel { id: string; provider: string; name: string; reasoning: boolean; contextWindow?: number }
export interface ModelCatalog { models: PiModel[]; current?: PiModel; thinkingLevels: string[] }
export interface Connection { available: boolean; config: Record<string, unknown> }
export interface KnowledgeSource { id: string; name: string; url: string; documentUrl?: string; note: string; installedHash?: string; previousHash?: string; availableHash?: string; checkedAt?: string; updated: boolean; error?: string }
export interface AgentEvent {
  version?: number; contentIndex?: number; seq: number; runId: string; at: number; type: string; text?: string; tool?: string; toolCallId?: string; args?: Record<string, unknown>; isError?: boolean;
  output?: string; durationMs?: number; data?: any;
  todos?: Todo[]; render?: Render; critique?: Critique;
}

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
  activity: () => req<{ activities: Activity[] }>('/api/activity'),
  runLog: (id: string, after = 0) => req<{ events: AgentEvent[] }>(`/api/runs/${encodeURIComponent(id)}/log?after=${after}`),
  runActivity: (id: string) => req<{ activity: Activity }>(`/api/runs/${encodeURIComponent(id)}/activity`),
  respondUI: (id: string, body: { id: string; value?: string; confirmed?: boolean; cancelled?: boolean }) => req(`/api/runs/${encodeURIComponent(id)}/ui-response`, json(body)),
  connection: () => req<Connection>('/api/connection'),
  knowledge: () => req<{ sources: KnowledgeSource[] }>('/api/knowledge'),
  checkKnowledge: (id: string) => req<KnowledgeSource>(`/api/knowledge/${encodeURIComponent(id)}/check`, { method: 'POST' }),
  previewKnowledge: (id: string) => req<{ content: string }>(`/api/knowledge/${encodeURIComponent(id)}/preview`),
  applyKnowledge: (id: string) => req<KnowledgeSource>(`/api/knowledge/${encodeURIComponent(id)}/apply`, { method: 'POST' }),
  rollbackKnowledge: (id: string) => req<KnowledgeSource>(`/api/knowledge/${encodeURIComponent(id)}/rollback`, { method: 'POST' }),
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

export function subscribeRun(runId: string, after: number, onEvent: (event: AgentEvent) => void, onSnapshot: (a: Activity) => void, onConnection: (connected: boolean) => void) {
  const es = new EventSource(`/api/runs/${encodeURIComponent(runId)}/events?after=${after}`);
  es.onopen = () => onConnection(true); es.onerror = () => onConnection(false);
  for (const type of ['starting', 'finishing', 'context', 'text', 'message_start', 'message_end', 'thinking', 'tool', 'tool_update', 'tool_end', 'turn', 'model', 'done', 'run_failed', 'error', 'canceled', 'stderr', 'todos', 'render', 'critique', 'retry', 'retry_end', 'compaction_start', 'compaction_end', 'attention', 'attention_resolved', 'extension_ui', 'session_update']) {
    es.addEventListener(type, ((e: MessageEvent) => { if (e.data) { try { onEvent(JSON.parse(e.data)); } catch { /* native transport error, not a protocol event */ } } }) as EventListener);
  }
  es.addEventListener('snapshot', ((e: MessageEvent) => { try { onSnapshot(JSON.parse(e.data)); } catch { /* reconnect retrieves a fresh projection */ } }) as EventListener);
  es.addEventListener('stream_end', () => es.close());
  return () => es.close();
}

export function previewURL(id: string, entry = 'index.html') {
  return `/preview/${encodeURIComponent(id)}/${entry.split('/').map(encodeURIComponent).join('/')}`;
}
