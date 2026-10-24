import type { Todo } from '../api';

// Pure parsing of the pi assistant-text protocol. No DOM, no React, no
// side effects: the same text always produces the same result, which keeps
// streaming renders stable and makes the parser testable in isolation.
//
// Markers: <question-form …>JSON</question-form>, <pd-done/>, <pd-next>…</pd-next>.
// Anything inside a ``` fence is prose, never a marker.

export interface FormOption { label: string; value: string; description?: string }
export type QuestionType =
  | 'radio' | 'checkbox' | 'select' | 'text' | 'textarea' | 'number' | 'range'
  | 'color' | 'switch' | 'url' | 'email' | 'date';
export interface Question {
  id: string; label: string; type: string; required?: boolean;
  default?: unknown; defaultValue?: unknown;
  options?: FormOption[]; placeholder?: string;
  maxSelections?: number; allowCustom?: boolean;
  min?: number; max?: number; step?: number;
}
export interface QuestionFormData {
  id: string; title?: string; submitLabel?: string;
  questions: Question[];
  /** A complete block that could not be parsed. */
  invalid?: boolean;
}
export interface ParsedAssistant {
  /** Working notes before <pd-done/> (empty when the marker is absent). */
  narration: string;
  /** The user-facing answer; the whole prose when there is no marker. */
  answer: string;
  forms: QuestionFormData[];
  next: string[];
  /** An unterminated <question-form …> is still streaming in. */
  pendingForm: boolean;
  hasDone: boolean;
}

const OPEN_FORM = '<question-form';

function asText(value: unknown): string | undefined {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return undefined;
}

/** Replace fenced code with spaces of equal length, preserving offsets. */
function maskFences(source: string): string {
  const lines = source.split('\n');
  const masked: string[] = [];
  let fenced = false;
  for (const line of lines) {
    if (/^\s*```/.test(line)) {
      fenced = !fenced;
      masked.push(' '.repeat(line.length));
    } else {
      masked.push(fenced ? ' '.repeat(line.length) : line);
    }
  }
  return masked.join('\n');
}

function attributes(source: string): Record<string, string> {
  const out: Record<string, string> = {};
  const re = /([a-zA-Z][\w-]*)\s*=\s*"([^"]*)"|([a-zA-Z][\w-]*)\s*=\s*'([^']*)'/g;
  for (const match of source.matchAll(re)) {
    const name = (match[1] || match[3] || '').toLowerCase();
    if (name) out[name] = match[2] ?? match[4] ?? '';
  }
  return out;
}

function normalizeOption(value: unknown): FormOption | null {
  if (typeof value === 'string') return value ? { label: value, value } : null;
  if (typeof value === 'number') return { label: String(value), value: String(value) };
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  const optionValue = asText(raw.value) ?? asText(raw.label);
  if (optionValue === undefined) return null;
  const label = asText(raw.label) ?? optionValue;
  const description = asText(raw.description);
  return description ? { label, value: optionValue, description } : { label, value: optionValue };
}

function normalizeQuestion(value: unknown, index: number): Question | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  const label = asText(raw.label) ?? asText(raw.id);
  if (!label) return null;
  const id = asText(raw.id) ?? `q${index + 1}`;
  const options = Array.isArray(raw.options)
    ? raw.options.map(normalizeOption).filter((option): option is FormOption => option !== null)
    : undefined;
  const question: Question = { id, label, type: asText(raw.type) ?? 'text' };
  if (raw.required === true) question.required = true;
  if (raw.default !== undefined) question.default = raw.default;
  if (raw.defaultValue !== undefined) question.defaultValue = raw.defaultValue;
  if (options && options.length) question.options = options;
  const placeholder = asText(raw.placeholder);
  if (placeholder) question.placeholder = placeholder;
  if (typeof raw.maxSelections === 'number' && Number.isFinite(raw.maxSelections) && raw.maxSelections > 0) {
    question.maxSelections = raw.maxSelections;
  }
  if (raw.allowCustom === false) question.allowCustom = false;
  for (const key of ['min', 'max', 'step'] as const) {
    const numeric = raw[key];
    if (typeof numeric === 'number' && Number.isFinite(numeric)) question[key] = numeric;
  }
  return question;
}

function parseForm(attrs: string, body: string, ordinal: number): QuestionFormData {
  const meta = attributes(attrs);
  const id = meta.id || `form-${ordinal + 1}`;
  const title = meta.title || undefined;
  const submitLabel = meta['submit-label'] || meta.submitlabel || undefined;
  const start = body.indexOf('{');
  const end = body.lastIndexOf('}');
  let data: unknown;
  if (start >= 0 && end > start) {
    try { data = JSON.parse(body.slice(start, end + 1)); } catch { data = undefined; }
  }
  const rawQuestions = typeof data === 'object' && data !== null && Array.isArray((data as { questions?: unknown }).questions)
    ? (data as { questions: unknown[] }).questions
    : null;
  const questions = rawQuestions
    ? rawQuestions.map(normalizeQuestion).filter((question): question is Question => question !== null)
    : [];
  if (!questions.length) return { id, title, submitLabel, questions: [], invalid: true };
  return {
    id,
    title: title || asText((data as { title?: unknown }).title),
    submitLabel: submitLabel || asText((data as { submitLabel?: unknown }).submitLabel),
    questions,
  };
}

/** Index of a trailing, still-streaming marker; -1 when the tail is prose. */
function pendingFormIndex(source: string): number {
  const open = source.search(/<question-form\b/i);
  if (open >= 0) return open;
  const lt = source.lastIndexOf('<');
  if (lt < 0) return -1;
  const tail = source.slice(lt).toLowerCase();
  return ['<question-form', '<pd-done', '<pd-next>'].some((candidate) => candidate.startsWith(tail)) ? lt : -1;
}

function cleanProse(source: string): string {
  return source
    .replace(/<\/?(?:question-form|pd-next|pd-done)\b[^>]*>/gi, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

export function parseAssistantText(input: string): ParsedAssistant {
  const source = input.replace(/\r\n?/g, '\n');
  const masked = maskFences(source);
  const removals: { start: number; end: number }[] = [];
  const forms: QuestionFormData[] = [];
  const suggestions: string[] = [];

  const formPattern = /<question-form\b([^>]*)>([\s\S]*?)<\/question-form\s*>/gi;
  for (const match of masked.matchAll(formPattern)) {
    const start = match.index ?? 0;
    const openEnd = match[0].indexOf('>');
    const closeLength = match[0].length - match[0].lastIndexOf('</question-form');
    const bodyStart = start + openEnd + 1;
    const bodyEnd = start + match[0].length - closeLength;
    forms.push(parseForm(source.slice(start + OPEN_FORM.length, start + openEnd), source.slice(bodyStart, bodyEnd), forms.length));
    removals.push({ start, end: start + match[0].length });
  }

  const nextPattern = /<pd-next>([\s\S]*?)<\/pd-next>/gi;
  for (const match of masked.matchAll(nextPattern)) {
    const start = match.index ?? 0;
    const body = source.slice(start + '<pd-next>'.length, start + match[0].length - '</pd-next>'.length).trim();
    if (body) suggestions.push(body);
    removals.push({ start, end: start + match[0].length });
  }

  let doneAt = -1;
  const doneMatch = /<pd-done\s*\/>/i.exec(masked);
  if (doneMatch) {
    doneAt = doneMatch.index ?? 0;
    removals.push({ start: doneAt, end: doneAt + doneMatch[0].length });
  }

  removals.sort((a, b) => a.start - b.start);
  const merged: { start: number; end: number }[] = [];
  for (const span of removals) {
    const previous = merged[merged.length - 1];
    if (previous && span.start <= previous.end) previous.end = Math.max(previous.end, span.end);
    else merged.push({ ...span });
  }

  let cleaned = '';
  let cleanedMasked = '';
  let cursor = 0;
  let split = -1;
  for (const span of merged) {
    if (doneAt >= cursor && doneAt <= span.start) split = cleaned.length + (doneAt - cursor);
    cleaned += source.slice(cursor, span.start);
    cleanedMasked += masked.slice(cursor, span.start);
    cursor = Math.max(cursor, span.end);
  }
  if (doneAt >= 0 && split < 0 && doneAt >= cursor) split = cleaned.length + (doneAt - cursor);
  cleaned += source.slice(cursor);
  cleanedMasked += masked.slice(cursor);

  const pendingAt = pendingFormIndex(cleanedMasked);
  const pending = pendingAt >= 0;
  if (pending) cleaned = cleaned.slice(0, pendingAt);
  if (split < 0 || split > cleaned.length) split = -1;

  const narration = split >= 0 ? cleaned.slice(0, split) : '';
  const answer = split >= 0 ? cleaned.slice(split) : cleaned;
  return {
    narration: cleanProse(narration),
    answer: cleanProse(answer),
    forms,
    next: [...new Set(suggestions)].slice(0, 3),
    pendingForm: pending,
    hasDone: split >= 0,
  };
}

// ---- Form answers -------------------------------------------------------

export type FormValue = string | string[] | boolean | number | null | undefined;
export type FormAnswers = Record<string, FormValue>;
export interface ParsedFormAnswers { id: string; entries: { label: string; display: string }[] }

export const OTHER_VALUE = '\u0000other';

export function resolveDefault(question: Question): FormValue {
  const raw = question.default !== undefined ? question.default : question.defaultValue;
  const type = question.type;
  if (type === 'checkbox') {
    if (Array.isArray(raw)) return raw.map((value) => String(value));
    if (typeof raw === 'string' && raw.trim()) return [raw];
    return [];
  }
  if (type === 'switch') {
    if (typeof raw === 'boolean') return raw;
    if (raw === 'true') return true;
    if (raw === 'false') return false;
    return false;
  }
  if (type === 'number' || type === 'range') {
    const numeric = typeof raw === 'number' ? raw : Number(raw);
    return Number.isFinite(numeric) ? numeric : '';
  }
  if (type === 'radio' || type === 'select' || type === 'text' || type === 'textarea' || type === 'color' || type === 'url' || type === 'email' || type === 'date') {
    if (raw === undefined || raw === null) return '';
    if (Array.isArray(raw)) return raw.length ? String(raw[0]) : '';
    return String(raw);
  }
  if (raw === undefined || raw === null) return '';
  if (Array.isArray(raw)) return raw.map((value) => String(value));
  return typeof raw === 'boolean' ? raw : String(raw);
}

function optionLabel(question: Question, value: string): string {
  return question.options?.find((option) => option.value === value)?.label ?? value;
}

export function displayValue(question: Question, value: FormValue): string {
  const type = question.type;
  if (type === 'switch') return value === true ? 'Yes' : value === false ? 'No' : '(skipped)';
  if (type === 'checkbox') {
    const values = Array.isArray(value) ? value.map(String).filter((entry) => entry.trim()) : [];
    if (!values.length) return '(skipped)';
    const labels = values.map((entry) => optionLabel(question, entry));
    const differs = values.some((entry, index) => labels[index] !== entry);
    return differs ? `${labels.join(', ')} [value: ${values.join(', ')}]` : labels.join(', ');
  }
  if (type === 'radio' || type === 'select') {
    if (typeof value !== 'string' || !value) return '(skipped)';
    const label = optionLabel(question, value);
    return label === value ? label : `${label} [value: ${value}]`;
  }
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') return String(value);
  if (Array.isArray(value)) return value.length ? value.join(', ') : '(skipped)';
  return value && value.trim() ? value.trim() : '(skipped)';
}

/** True when the form's own defaults answer every required question. */
export function isAnswered(question: Question, value: FormValue): boolean {
  if (question.type === 'switch') return typeof value === 'boolean';
  if (question.type === 'checkbox') return Array.isArray(value) && value.some((entry) => String(entry).trim());
  if (question.type === 'number' || question.type === 'range') return value !== '' && value !== null && value !== undefined && Number.isFinite(Number(value));
  return typeof value === 'string' ? value.trim().length > 0 : value !== null && value !== undefined;
}

/** Exact `[form answers — id]` message body for a form. */
export function formatFormAnswers(form: QuestionFormData, answers: FormAnswers): string {
  const lines = [`[form answers — ${form.id}]`];
  for (const question of form.questions) {
    const value = answers[question.id];
    lines.push(`- ${question.label}: ${displayValue(question, value === OTHER_VALUE ? '' : value)}`);
  }
  return lines.join('\n');
}

export function parseFormAnswers(content: string): ParsedFormAnswers | null {
  const normalized = content.replace(/\r\n?/g, '\n').trim();
  if (!normalized.startsWith('[form answers')) return null;
  const [header, ...rest] = normalized.split('\n');
  const match = /^\[form answers\s*(?:—|–|-)\s*([^\]]*)\]\s*$/.exec(header.trim());
  if (!match) return null;
  const id = match[1].trim();
  if (!id) return null;
  const entries = rest
    .map((line) => /^\s*[-*]\s*(.+?):\s*(.*)$/.exec(line))
    .filter((entry): entry is RegExpExecArray => entry !== null)
    // `[value: …]` is the machine id for the agent; people read the label.
    .map((entry) => ({ label: entry[1].trim(), display: entry[2].replace(/\s*\[value:[^\]]*\]\s*$/, '').trim() || '(skipped)' }));
  return { id, entries };
}

/** The continuation message for a run with unfinished todos (contract §6). */
export function continueMessage(todos: Todo[]): string {
  const open = todos.filter((todo) => todo.status !== 'completed');
  return [
    'Continue the remaining unfinished tasks from the previous run. Do not redo completed work. Focus only on these unfinished todos:',
    ...open.map((todo) => `- ${todo.content}`),
    'Before making changes, inspect the current project files as needed. Update todo_write as you complete each remaining task.',
  ].join('\n');
}
