#!/usr/bin/env bash
# End-to-end test: create a project, run a real agent generation, stream events,
# then verify the artifact renders through the preview bridge.
#
# Default daemon: http://127.0.0.1:7457. Override with PIDESIGNER_E2E_BASE.
# Runs detached so an interrupted terminal cannot kill it:
#   nohup ./scripts/e2e.sh > /tmp/e2e.out 2>&1 &
set -Eeuo pipefail

BASE="${PIDESIGNER_E2E_BASE:-http://127.0.0.1:7457}"
BASE="${BASE%/}"
WAIT_SECONDS="${PIDESIGNER_E2E_TIMEOUT_SECONDS:-900}"
POLL_SECONDS="${PIDESIGNER_E2E_POLL_INTERVAL_SECONDS:-5}"
STREAM_TIMEOUT_SECONDS="${PIDESIGNER_E2E_STREAM_TIMEOUT_SECONDS:-$((WAIT_SECONDS + 60))}"
KEEP_OUTPUTS="${PIDESIGNER_E2E_KEEP_OUTPUTS:-0}"

[[ "$WAIT_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "PIDESIGNER_E2E_TIMEOUT_SECONDS must be a positive integer" >&2; exit 2; }
[[ "$POLL_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "PIDESIGNER_E2E_POLL_INTERVAL_SECONDS must be a positive integer" >&2; exit 2; }
[[ "$STREAM_TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "PIDESIGNER_E2E_STREAM_TIMEOUT_SECONDS must be a positive integer" >&2; exit 2; }
[[ "$KEEP_OUTPUTS" == 0 || "$KEEP_OUTPUTS" == 1 ]] || { echo "PIDESIGNER_E2E_KEEP_OUTPUTS must be 0 or 1" >&2; exit 2; }

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pidesigner-e2e.XXXXXX")"
CURL_PID=""

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  if [[ -n "$CURL_PID" ]]; then
    kill "$CURL_PID" 2>/dev/null || true
    wait "$CURL_PID" 2>/dev/null || true
  fi
  if [[ "$KEEP_OUTPUTS" == 1 ]]; then
    printf 'E2E outputs retained at %s\n' "$WORK_DIR" >&2
  else
    rm -rf "$WORK_DIR" || true
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'code=$?; printf "e2e.sh: command failed at line %s (exit %s)\n" "$LINENO" "$code" >&2; exit "$code"' ERR

request() {
  local method="$1"
  local url="$2"
  local output="$3"
  local body="${4-}"
  local status

  : > "$output"
  if [[ -n "$body" ]]; then
    status="$(curl --silent --show-error --max-time 30 --output "$output" \
      --write-out '%{http_code}' --request "$method" \
      --header 'Content-Type: application/json' --data-binary "$body" "$url")"
  else
    status="$(curl --silent --show-error --max-time 30 --output "$output" \
      --write-out '%{http_code}' --request "$method" "$url")"
  fi
  if [[ ! "$status" =~ ^2[0-9][0-9]$ ]]; then
    printf 'HTTP %s for %s %s\n' "$status" "$method" "$url" >&2
    cat "$output" >&2
    return 1
  fi
}

json_field() {
  python3 - "$1" "$2" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
with open(sys.argv[1], encoding="utf-8") as stream:
    value = json.load(stream)
for key in path:
    if not isinstance(value, dict) or key not in value:
        raise SystemExit(f"missing JSON field: {'.'.join(path)}")
    value = value[key]
if not isinstance(value, str) or not value:
    raise SystemExit(f"JSON field must be a non-empty string: {'.'.join(path)}")
print(value)
PY
}

echo "==> creating project on $BASE"
request POST "$BASE/api/projects" "$WORK_DIR/project-create.json" \
  '{"name":"Flux Hero","designSystemId":"agentic","skillId":"web-prototype"}'
PID="$(json_field "$WORK_DIR/project-create.json" project.id)"
CID="$(json_field "$WORK_DIR/project-create.json" conversationId)"
echo "project=$PID conversation=$CID"

echo "==> starting run"
RUN_BODY="$(python3 - "$CID" <<'PY'
import json
import sys
print(json.dumps({
    "prompt": "Build a single-page landing page hero for a developer tool called Flux. Use the active design system tokens. Include a headline, subhead, primary CTA, and a subtle CSS-only animated gradient background. Write index.html and style.css.",
    "conversationId": sys.argv[1],
}))
PY
)"
request POST "$BASE/api/projects/$PID/chat" "$WORK_DIR/run-create.json" "$RUN_BODY"
RUNID="$(json_field "$WORK_DIR/run-create.json" runId)"
echo "run=$RUNID"

EVENTS="$WORK_DIR/events.sse"
EVENT_HTTP="$WORK_DIR/events.http"
EVENT_ERR="$WORK_DIR/events.stderr"
echo "==> streaming events"
curl --silent --show-error --no-buffer --max-time "$STREAM_TIMEOUT_SECONDS" \
  --output "$EVENTS" --write-out '%{http_code}' "$BASE/api/runs/$RUNID/events" \
  > "$EVENT_HTTP" 2> "$EVENT_ERR" &
CURL_PID=$!

echo "==> waiting for terminal status"
deadline=$((SECONDS + WAIT_SECONDS))
STATUS=""
while (( SECONDS <= deadline )); do
  request GET "$BASE/api/runs/$RUNID" "$WORK_DIR/run-status.json"
  STATUS="$(json_field "$WORK_DIR/run-status.json" run.status)"
  echo "  status=$STATUS"
  case "$STATUS" in
    succeeded) break ;;
    running) ;;
    failed|canceled)
      echo "Run ended with status=$STATUS; see $WORK_DIR/run-status.json" >&2
      exit 1
      ;;
    *)
      echo "Run returned unknown status: $STATUS" >&2
      exit 1
      ;;
  esac
  sleep "$POLL_SECONDS"
done
if [[ "$STATUS" != succeeded ]]; then
  echo "Run timed out after ${WAIT_SECONDS}s (last status: ${STATUS:-unknown})" >&2
  exit 1
fi

if wait "$CURL_PID"; then
  STREAM_EXIT=0
else
  STREAM_EXIT=$?
fi
CURL_PID=""
if [[ "$STREAM_EXIT" -ne 0 ]]; then
  printf 'SSE request failed with exit %s: ' "$STREAM_EXIT" >&2
  cat "$EVENT_ERR" >&2
  exit 1
fi
EVENT_STATUS="$(<"$EVENT_HTTP")"
[[ "$EVENT_STATUS" == 200 ]] || { echo "SSE endpoint returned HTTP $EVENT_STATUS" >&2; exit 1; }

echo "==> validating event stream"
python3 - "$EVENTS" <<'PY'
import json
import sys

events = []
with open(sys.argv[1], encoding="utf-8") as stream:
    for line_number, line in enumerate(stream, 1):
        line = line.strip()
        if not line.startswith("data:"):
            continue
        payload = line[5:].strip()
        if not payload:
            continue
        try:
            event = json.loads(payload)
        except json.JSONDecodeError as error:
            raise SystemExit(f"invalid SSE JSON on line {line_number}: {error}")
        if not isinstance(event, dict) or not isinstance(event.get("type"), str):
            raise SystemExit(f"malformed SSE event on line {line_number}")
        events.append(event)
if not events:
    raise SystemExit("SSE stream contained no data events")
types = {event["type"] for event in events}
if "run_failed" in types or "canceled" in types:
    raise SystemExit(f"SSE stream reported failure/cancellation: {sorted(types)}")
if "done" not in types:
    raise SystemExit(f"SSE stream did not contain the successful terminal event: {sorted(types)}")
print(f"  events: {len(events)}")
PY

echo "==> verifying generated files"
request GET "$BASE/api/projects/$PID/files" "$WORK_DIR/files.json"
python3 - "$WORK_DIR/files.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    response = json.load(stream)
files = response.get("files")
if not isinstance(files, list):
    raise SystemExit("file-list response has no files array")
by_path = {item.get("path"): item for item in files if isinstance(item, dict)}
for path in ("index.html", "style.css"):
    item = by_path.get(path)
    if item is None:
        raise SystemExit(f"required generated file is missing: {path}")
    if not isinstance(item.get("size"), int) or item["size"] <= 0:
        raise SystemExit(f"generated file is empty or has invalid size: {path}")
    print(f"  {path:28} {item['size']:>7} bytes")
PY

echo "==> verifying artifact bundle"
request GET "$BASE/api/projects/$PID/artifact" "$WORK_DIR/artifact.json"
python3 - "$WORK_DIR/artifact.json" <<'PY'
import base64
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    bundle = json.load(stream)
entry = bundle.get("entryFile")
files = bundle.get("files")
if entry != "index.html":
    raise SystemExit(f"unexpected artifact entry file: {entry!r}")
if not isinstance(files, dict):
    raise SystemExit("artifact response has no files object")
for path in (entry, "style.css"):
    item = files.get(path)
    if not isinstance(item, dict):
        raise SystemExit(f"required artifact file is missing or malformed: {path}")
    encoding = item.get("encoding")
    content = item.get("content")
    mime_type = item.get("mimeType")
    if not isinstance(content, str) or not content:
        raise SystemExit(f"artifact file has no content: {path}")
    if not isinstance(mime_type, str) or not mime_type:
        raise SystemExit(f"artifact file has no MIME type: {path}")
    if encoding == "utf-8":
        payload = content.encode("utf-8")
    elif encoding == "base64":
        try:
            payload = base64.b64decode(content, validate=True)
        except Exception as error:
            raise SystemExit(f"invalid base64 artifact payload for {path}: {error}")
    else:
        raise SystemExit(f"unsupported artifact encoding for {path}: {encoding!r}")
    if not payload:
        raise SystemExit(f"artifact file is empty: {path}")
    print(f"  {path}: {len(payload)} bytes ({encoding}, {mime_type})")
entry_item = files[entry]
entry_content = entry_item["content"]
if entry_item["encoding"] == "base64":
    entry_content = base64.b64decode(entry_content, validate=True).decode("utf-8")
if "<html" not in entry_content.lower():
    raise SystemExit("artifact entry file does not contain an HTML document")
PY

echo "==> verifying preview render"
request GET "$BASE/preview/$PID/" "$WORK_DIR/preview.html"
python3 - "$WORK_DIR/preview.html" <<'PY'
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    html = stream.read()
if "pidesigner-bridge" not in html:
    raise SystemExit("preview HTML is missing the πDesigner preview bridge")
if "<html" not in html.lower():
    raise SystemExit("preview response is not an HTML document")
print(f"  bridge injected; {len(html.encode('utf-8'))} bytes")
PY

echo "==> tool usage summary"
python3 - "$EVENTS" <<'PY'
import collections
import json
import sys

tools = collections.Counter()
for line_number, line in enumerate(open(sys.argv[1], encoding="utf-8"), 1):
    line = line.strip()
    if not line.startswith("data:"):
        continue
    event = json.loads(line[5:].strip())
    if event.get("type") == "tool":
        tools[event.get("tool", "<unknown>")] += 1
print("  tool events:", sum(tools.values()))
for name, count in tools.most_common():
    print(f"    {name}: {count}")
PY

echo "==> assistant message"
request GET "$BASE/api/conversations/$CID/messages" "$WORK_DIR/messages.json"
python3 - "$WORK_DIR/messages.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    response = json.load(stream)
messages = response.get("messages")
if not isinstance(messages, list):
    raise SystemExit("conversation response has no messages array")
assistant = [message for message in messages if message.get("role") == "assistant"]
if not assistant or not any(isinstance(message.get("content"), str) and message["content"].strip() for message in assistant):
    raise SystemExit("conversation has no non-empty assistant response")
for message in messages:
    content = message.get("content", "")
    print(f"  [{message.get('role', '<unknown>')}] {len(content)} chars")
    if message.get("role") == "assistant":
        print("   ", content[:300].replace("\n", " "))
PY

printf 'E2E_DONE status=%s project=%s run=%s\n' "$STATUS" "$PID" "$RUNID" | tee "$WORK_DIR/e2e.result"
