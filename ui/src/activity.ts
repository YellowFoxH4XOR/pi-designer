import type { AgentEvent, Run, Todo, Render, Critique } from "./api";

export interface ToolActivity {
  id: string;
  tool: string;
  label: string;
  args?: Record<string, unknown>;
  output?: string;
  status: "running" | "succeeded" | "failed" | "interrupted";
  startedAt: number;
  endedAt?: number;
  durationMs?: number;
}
export interface UIRequest {
  id: string;
  method: "select" | "confirm" | "input" | "editor";
  title?: string;
  message?: string;
  options?: string[];
  prefill?: string;
  placeholder?: string;
  timeout?: number;
}
export interface Activity {
  runId: string;
  projectId: string;
  projectName: string;
  conversationId?: string;
  status: Run["status"];
  phase: string;
  label: string;
  prompt?: string;
  model?: string;
  modelId?: string;
  provider?: string;
  context?: {
    skillId?: string;
    designSystemId?: string;
    viewport?: string;
    thinking?: string;
    designRoute?: { intent: string; choices: { id: string; name: string; reason: string }[] };
  };
  startedAt: number;
  updatedAt: number;
  endedAt?: number;
  seq: number;
  text: string;
  messageText?: string;
  textBlocks?: Record<number, string>;
  tools: ToolActivity[];
  attention: boolean;
  request?: UIRequest;
  error?: string;
  todos?: Todo[];
  renders?: Render[];
  critiques?: Critique[];
}
export type RunState = Activity & {
  id: string;
  todos: Todo[];
  renders: Render[];
  critiques: Critique[];
  connection?: "connected" | "reconnecting";
};
export function fromActivity(a: Activity): RunState {
  return {
    ...a,
    id: a.runId,
    tools: a.tools || [],
    text: a.text || "",
    todos: a.todos || [],
    renders: a.renders || [],
    critiques: a.critiques || [],
    connection: "connected",
  };
}

// A single reducer for stream reconstruction; snapshots remain authoritative.
export function reduceActivity(current: RunState, e: AgentEvent): RunState {
  if (e.seq <= current.seq) return current;
  const a = {
    ...current,
    seq: e.seq,
    updatedAt: e.at,
    connection: "connected" as const,
  };
  switch (e.type) {
    case "text": {
      const prefix =
        a.messageText && a.text.endsWith(a.messageText)
          ? a.text.slice(0, -a.messageText.length)
          : a.text;
      const index = e.contentIndex || 0;
      a.textBlocks = {
        ...a.textBlocks,
        [index]: (a.textBlocks?.[index] || "") + (e.text || ""),
      };
      a.messageText = Object.keys(a.textBlocks)
        .map(Number)
        .sort((x, y) => x - y)
        .map((k) => a.textBlocks![k])
        .join("");
      a.text = prefix + a.messageText;
      a.phase = "responding";
      a.label = "Writing response";
      break;
    }
    case "message_start":
      a.textBlocks = {};
      a.messageText = "";
      a.phase = "waiting";
      a.label = "Waiting for model response";
      break;
    case "message_end":
      a.text =
        (a.messageText && a.text.endsWith(a.messageText)
          ? a.text.slice(0, -a.messageText.length)
          : a.text) + (e.text || "");
      a.messageText = e.text || "";
      break;
    case "turn":
      if (a.text && !a.text.endsWith("\n\n")) a.text += "\n\n";
      a.messageText = "";
      break;
    case "context":
      a.context = e.data;
      break;
    case "model":
      a.model = e.text;
      a.modelId = e.data?.id;
      a.provider = e.data?.provider;
      break;
    case "thinking":
      a.phase = "thinking";
      a.label = "Model is thinking";
      break;
    case "tool":
    case "tool_update":
    case "tool_end": {
      const tools = [...a.tools];
      let i = tools.findIndex((t) => t.id === e.toolCallId);
      if (i < 0) {
        tools.push({
          id: e.toolCallId || String(e.seq),
          tool: e.tool || "tool",
          label: toolLabel(e.tool || "", e.args),
          args: e.args,
          status: "running",
          startedAt: e.at,
        });
        i = tools.length - 1;
      }
      const tool = { ...tools[i] };
      if (e.args) {
        tool.args = e.args;
        tool.label = toolLabel(tool.tool, e.args);
      }
      if (e.type !== "tool") tool.output = e.output;
      a.phase = "tool";
      a.label = tool.label;
      if (e.type === "tool_end") {
        tool.status = e.isError ? "failed" : "succeeded";
        tool.endedAt = e.at;
        tool.durationMs = e.durationMs;
        a.phase = "waiting";
        a.label = "Waiting for model response";
      }
      tools[i] = tool;
      a.tools = tools;
      const active = tools.find((t) => t.status === "running");
      if (active) {
        a.phase = "tool";
        a.label = active.label;
      }
      break;
    }
    case "retry":
      a.phase = "retrying";
      a.label = "Retrying model request";
      break;
    case "compaction_start":
      a.phase = "compacting";
      a.label = "Compacting conversation";
      break;
    case "retry_end":
    case "compaction_end":
    case "agent_start":
      a.phase = "waiting";
      a.label = "Waiting for model response";
      break;
    case "attention":
      a.attention = true;
      a.request = e.data as UIRequest | undefined;
      a.phase = "attention";
      a.label = "Needs your answer";
      break;
    case "attention_resolved":
      a.attention = false;
      a.request = undefined;
      a.phase = "waiting";
      a.label = "Waiting for model response";
      break;
    case "todos":
      a.todos = e.todos || [];
      break;
    case "render":
      if (e.render && !a.renders?.some((r) => r.image === e.render?.image))
        a.renders = [...(a.renders || []), e.render];
      break;
    case "critique":
      if (e.critique)
        a.critiques = [
          ...(a.critiques || []).filter((c) => c.round !== e.critique?.round),
          e.critique,
        ].sort((x, y) => x.round - y.round);
      break;
    case "finishing":
      if (!a.attention) {
        a.phase = "finishing";
        a.label = "Finishing run";
      }
      break;
    case "error":
      a.error = e.text;
      break;
    case "done":
    case "canceled":
    case "run_failed":
      a.status =
        e.type === "done"
          ? "succeeded"
          : e.type === "canceled"
            ? "canceled"
            : "failed";
      a.phase = a.attention && e.type === "done" ? "attention" : "finished";
      a.label =
        a.phase === "attention"
          ? "Needs your answer"
          : e.type === "done"
            ? "Finished"
            : e.type === "canceled"
              ? "Stopped"
              : "Run failed";
      if (e.type !== "done") {
        a.attention = false;
        a.request = undefined;
      }
      if (e.type === "done") a.error = undefined;
      a.endedAt = e.at;
      if (e.type === "run_failed") a.error = e.text;
      a.tools = a.tools.map((t) =>
        t.status === "running"
          ? { ...t, status: "interrupted", endedAt: e.at }
          : t,
      );
      break;
  }
  return a;
}
export function toolLabel(tool: string, args?: Record<string, unknown>) {
  const path = String(args?.path || args?.file_path || "file")
    .split("/")
    .pop();
  return (
    (
      {
        read: `Reading ${path}`,
        write: `Writing ${path}`,
        edit: `Editing ${path}`,
        multi_edit: `Editing ${path}`,
        bash: "Running command",
        render_preview: "Rendering preview",
        todo_write: "Updating plan",
        submit_critique: "Reviewing design",
      } as Record<string, string>
    )[tool] || `Using ${tool}`
  );
}
export function statusLabel(
  a: Pick<Activity, "status" | "attention" | "label">,
) {
  return a.attention
    ? "Needs your answer"
    : a.status === "running"
      ? a.label
      : a.status === "succeeded"
        ? "Finished"
        : a.status === "failed"
          ? "Run failed"
          : "Stopped";
}
export function elapsed(start: number, end = Date.now()) {
  const s = Math.max(0, Math.floor((end - start) / 1000));
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`;
}
