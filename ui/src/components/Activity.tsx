import { useEffect, useState } from "preact/hooks";
import { api } from "../api";
import {
  elapsed,
  statusLabel,
  type RunState,
  type ToolActivity,
  type UIRequest,
} from "../activity";
import { Dialog } from "./common";
import { Icon, PiLogo } from "./Icon";
import { ModelIcon } from "./ModelIcon";
import { RunDiagnostics } from "./RunDiagnostics";
import { skillNames } from "../state";

function useClock(active: boolean) {
  const [, tick] = useState(0);
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(timer);
  }, [active]);
}
export function ProjectStatus({ run }: { run?: RunState }) {
  if (!run) return null;
  return (
    <span
      class={`project-status is-${run.attention ? "attention" : run.status}`}
    >
      <Icon
        name={
          run.attention
            ? "message"
            : run.status === "running"
              ? "activity"
              : run.status === "failed"
                ? "warning"
                : run.status === "canceled"
                  ? "stop"
                  : "check"
        }
        size={13}
      />
      <span>{statusLabel(run)}</span>
    </span>
  );
}
export function ActivityCenter({
  runs,
  error,
  onOpen,
}: {
  runs: Record<string, RunState>;
  error?: string;
  onOpen: (run: RunState) => void;
}) {
  const [open, setOpen] = useState(false);
  const items = Object.values(runs).sort(
    (a, b) =>
      Number(b.attention || b.status === "running") -
        Number(a.attention || a.status === "running") ||
      b.startedAt - a.startedAt,
  );
  const working = items.filter((a) => a.status === "running").length;
  const attention = items.filter((a) => a.attention).length;
  useClock(open && working > 0);
  return (
    <>
      <button
        class={`activity-center-button ${attention ? "needs-attention" : ""}`}
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
      >
        <Icon name="activity" size={16} />
        <span>
          {error ? "Activity disconnected" : attention
            ? `${attention} needs attention`
            : working
              ? `${working} working`
              : "Activity"}
        </span>
      </button>
      {open && (
        <Dialog
          title="Workspace activity"
          onClose={() => setOpen(false)}
          class="activity-dialog"
        >
          <p class="dialog-intro">
            Live work, questions, and recent results. Tasks keep running when
            you close the window.
          </p>
          {error && (
            <p class="inline-error" role="status">
              {error}
            </p>
          )}
          {!items.length ? (
            <p class="activity-empty">
              Nothing is running. Start with a brief on Home.
            </p>
          ) : (
            <ul class="activity-project-list">
              {items.map((a) => (
                <li key={a.id}>
                  <button
                    onClick={() => {
                      setOpen(false);
                      onOpen(a);
                    }}
                  >
                    <span class="activity-project-title">
                      {a.projectName}
                      <Icon name="chevron" size={14} />
                    </span>
                    <ProjectStatus run={a} />
                    <span class="activity-project-meta">
                      {a.model || "π agent"} · {elapsed(a.startedAt, a.endedAt)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Dialog>
      )}
    </>
  );
}
function ToolStep({ tool }: { tool: ToolActivity }) {
  const path = tool.args?.path || tool.args?.file_path;
  const command = tool.args?.command;
  return (
    <details class={`activity-step is-${tool.status}`}>
      <summary>
        <Icon
          name={
            tool.status === "running"
              ? "activity"
              : tool.status === "failed"
                ? "warning"
                : tool.status === "interrupted"
                  ? "stop"
                  : "check"
          }
          size={14}
        />
        <span>{tool.label}</span>
        <small>
          {tool.status === "running" ? "Running" : tool.status === "failed" ? "Failed" : tool.status === "interrupted" ? "Interrupted" : "Done"}
          {tool.status !== "running" && tool.durationMs !== undefined && ` · ${(tool.durationMs / 1000).toFixed(1)}s`}
        </small>
      </summary>
      <div class="activity-step-body">
        {path && <code>{String(path)}</code>}
        {command && (
          <pre>
            <code>{String(command)}</code>
          </pre>
        )}
        {tool.output ? (
          <pre>{tool.output}</pre>
        ) : (
          <p>
            {tool.status === "running"
              ? "Waiting for tool output…"
              : "No text output returned."}
          </p>
        )}
        <details class="activity-arguments">
          <summary>Tool arguments · {tool.tool}</summary>
          <pre>{JSON.stringify(tool.args || {}, null, 2)}</pre>
        </details>
      </div>
    </details>
  );
}
function ExtensionQuestion({
  request,
  runId,
  onError,
}: {
  request: UIRequest;
  runId: string;
  onError: (s: string) => void;
}) {
  const [value, setValue] = useState(request.prefill || "");
  const [pending, setPending] = useState(false);
  async function answer(cancelled = false, confirmed?: boolean) {
    setPending(true);
    try {
      await api.respondUI(runId, {
        id: request.id,
        value,
        confirmed,
        cancelled,
      });
    } catch (e) {
      onError(String(e));
      setPending(false);
    }
  }
  return (
    <form
      class="extension-question"
      onSubmit={(e) => {
        e.preventDefault();
        void answer(false, request.method === "confirm" ? true : undefined);
      }}
    >
      <h3>{request.title || "π needs your answer"}</h3>
      {request.message && <p>{request.message}</p>}
      {request.method === "select" ? (
        <label>
          Choose an option
          <select
            value={value}
            onChange={(e) => setValue(e.currentTarget.value)}
            required
          >
            <option value="" disabled>
              Select…
            </option>
            {request.options?.map((o) => (
              <option key={o}>{o}</option>
            ))}
          </select>
        </label>
      ) : (
        request.method !== "confirm" && (
          <label>
            Your answer
            <textarea
              value={value}
              placeholder={request.placeholder}
              onInput={(e) => setValue(e.currentTarget.value)}
            />
          </label>
        )
      )}
      <div>
        <button
          class="button primary small"
          disabled={pending || (request.method === "select" && !value)}
        >
          {pending
            ? "Sending…"
            : request.method === "confirm"
              ? "Confirm"
              : "Send answer"}
        </button>
        {request.method === "confirm" && (
          <button
            class="button secondary small"
            type="button"
            disabled={pending}
            onClick={() => void answer(false, false)}
          >
            No
          </button>
        )}
        <button
          class="text-button"
          type="button"
          disabled={pending}
          onClick={() => void answer(true)}
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
export function RunActivity({
  run,
  workflow,
  system,
  onError,
}: {
  run: RunState;
  workflow: string;
  system?: string;
  onError: (s: string) => void;
}) {
  useClock(run.status === "running");
  const [open, setOpen] = useState(() =>
    new URLSearchParams(location.hash.split("?")[1] || "").has("activity"),
  );
  const [stopping, setStopping] = useState(false);
  const [failure, setFailure] = useState("");
  useEffect(() => {
    const reveal = () => {
      setOpen(true);
      requestAnimationFrame(() =>
        document.getElementById("run-activity-summary")?.focus(),
      );
    };
    window.addEventListener("pidesigner:show-activity", reveal);
    return () => window.removeEventListener("pidesigner:show-activity", reveal);
  }, []);
  useEffect(() => {
    if (run.attention) setOpen(true);
  }, [run.attention]);
  async function stop() {
    setStopping(true);
    try {
      await api.cancelRun(run.id);
    } catch (e) {
      setStopping(false);
      setFailure(String(e));
    }
  }
  return (
    <section
      class={`run-activity ${run.attention ? "needs-attention" : ""}`}
      aria-label="Run activity"
    >
      <div class="run-activity-top">
        <button
          id="run-activity-summary"
          class="run-activity-toggle"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
          aria-controls={open ? "run-activity-details" : undefined}
        >
          <PiLogo size={24} />
          <span>
            <strong aria-live="polite">
              {run.connection === "reconnecting"
                ? "Reconnecting to activity…"
                : statusLabel(run)}
            </strong>
            <small>
              {run.projectName} · {elapsed(run.startedAt, run.endedAt)}
            </small>
          </span>
          <Icon name="chevron" size={14} class={open ? "expanded" : ""} />
        </button>
        {run.status === "running" && (
          <button
            class="run-stop"
            onClick={stop}
            disabled={stopping}
            title="Stop generation"
          >
            <Icon name="stop" size={14} />
            {stopping ? "Stopping…" : "Stop"}
          </button>
        )}
      </div>
      {open && (
        <div id="run-activity-details" class="run-activity-details">
          {run.model && (
            <p class="activity-model">
              <ModelIcon id={run.modelId} provider={run.provider} size={18} />
              <span>
                {run.model}
                <small>Executed by π</small>
              </span>
            </p>
          )}
          <details class="brief-details">
            <summary>Your brief</summary>
            <p>{run.prompt}</p>
            <dl>
              <div>
                <dt>Workflow</dt>
                <dd>
                  {skillNames[run.context?.skillId || ""] ||
                    run.context?.skillId ||
                    workflow}
                </dd>
              </div>
              <div>
                <dt>Design system</dt>
                <dd>
                  {run.context
                    ? run.context.designSystemId || "Not selected"
                    : system || "Not selected"}
                </dd>
              </div>
              <div>
                <dt>Thinking</dt>
                <dd>{run.context?.thinking || "Default"}</dd>
              </div>
              <div>
                <dt>Submitted</dt>
                <dd>{new Date(run.startedAt).toLocaleString()}</dd>
              </div>
            </dl>
          </details>
          {run.request && (
            <ExtensionQuestion
              key={run.request.id}
              request={run.request}
              runId={run.id}
              onError={onError}
            />
          )}
          <div class="activity-timeline" aria-label="Tool execution history">
            {run.tools.length ? (
              run.tools.map((tool) => <ToolStep key={tool.id} tool={tool} />)
            ) : (
              <p class="activity-empty">
                {run.status === "running"
                  ? "Waiting for the first reported action…"
                  : "No tool actions reported in this run."}
              </p>
            )}
          </div>
          {run.error && <p class="inline-error">{run.error}</p>}
          <RunDiagnostics runId={run.id} />
          <p class="activity-log-note">
            Exact reported actions. Output previews are limited to 16 KB per
            tool and stored locally.
          </p>
        </div>
      )}
      {failure && (
        <p class="inline-error" role="alert">
          {failure}
        </p>
      )}
    </section>
  );
}
