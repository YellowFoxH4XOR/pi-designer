import { useState } from "preact/hooks";
import { api, type AgentEvent } from "../api";
import { CopyButton } from "./common";

export function RunDiagnostics({ runId }: { runId: string }) {
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [more, setMore] = useState(true);
  const [error, setError] = useState("");
  async function load() {
    if (loading) return;
    setLoading(true);
    setError("");
    try {
      const { events: page } = await api.runLog(runId, events.at(-1)?.seq || 0);
      setEvents((all) => [...all, ...page]);
      setMore(page.length === 500);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }
  const text = events.map((e) => JSON.stringify(e)).join("\n");
  return (
    <details
      class="run-diagnostics"
      onToggle={(e) => {
        if (e.currentTarget.open && !events.length) void load();
      }}
    >
      <summary>Event log & diagnostics</summary>
      <p>
        Ordered local events, including retries, tool output and stderr. Logs
        may contain project or command content. Review before sharing.
      </p>
      {events.length > 0 && (
        <>
          <pre tabIndex={0} aria-label="Recorded run events">
            {text}
          </pre>
          <CopyButton
            text={text}
            label="Copy displayed events"
            onError={setError}
          />
        </>
      )}
      {error && (
        <p class="inline-error" role="alert">
          {error}
        </p>
      )}
      {more && (
        <button
          class="text-button"
          disabled={loading}
          onClick={() => void load()}
        >
          {loading
            ? "Loading…"
            : events.length
              ? "Load next 500 events"
              : "Retry loading events"}
        </button>
      )}
    </details>
  );
}
