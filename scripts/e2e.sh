#!/usr/bin/env bash
# End-to-end test: create a project, run a real agent generation, stream events,
# then verify the artifact renders through the preview bridge.
#
# Runs detached so an interrupted terminal cannot kill it:
#   nohup ./scripts/e2e.sh > /tmp/e2e.out 2>&1 &
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="http://127.0.0.1:7457"
OUT=/tmp/e2e.result

j() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

echo "==> creating project"
PROJ=$(curl -s -X POST "$BASE/api/projects" -H 'Content-Type: application/json' \
  -d '{"name":"Flux Hero","designSystemId":"agentic","skillId":"web-prototype"}')
PID=$(echo "$PROJ" | j "d['project']['id']")
CID=$(echo "$PROJ" | j "d['conversationId']")
echo "project=$PID conversation=$CID"
echo "$PID" > /tmp/e2e.pid

echo "==> starting run"
RUN=$(curl -s -X POST "$BASE/api/projects/$PID/chat" -H 'Content-Type: application/json' \
  -d "{\"prompt\":\"Build a single-page landing page hero for a developer tool called Flux. Use the active design system tokens. Include a headline, subhead, primary CTA, and a subtle CSS-only animated gradient background. Write index.html and style.css.\",\"conversationId\":\"$CID\"}")
RUNID=$(echo "$RUN" | j "d['runId']")
echo "run=$RUNID"
echo "$RUNID" > /tmp/e2e.run

echo "==> streaming events to /tmp/e2e.events"
curl -sN --max-time 900 "$BASE/api/runs/$RUNID/events" > /tmp/e2e.events 2>&1 &
CURL_PID=$!

# Poll until the run leaves "running".
for i in $(seq 1 180); do
  STATUS=$(curl -s "$BASE/api/runs/$RUNID" | j "d['run']['status']" 2>/dev/null || echo unknown)
  echo "  [$i] status=$STATUS"
  case "$STATUS" in
    succeeded|failed|canceled) break ;;
  esac
  sleep 5
done
wait $CURL_PID 2>/dev/null

echo "==> final status: $STATUS"
echo "==> files"
curl -s "$BASE/api/projects/$PID/files" | python3 -c "
import json,sys
for f in json.load(sys.stdin)['files']:
    print(f\"  {f['path']:28} {f['size']:>7} bytes\")"

echo "==> artifact bundle"
curl -s "$BASE/api/projects/$PID/artifact" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('  entry:', d.get('entryFile'))
print('  files:', list(d.get('files',{}).keys()))" 2>/dev/null

echo "==> preview render check"
curl -s "$BASE/preview/$PID/" -o /tmp/e2e.preview.html -w "  http=%{http_code} bytes=%{size_download}\n"
grep -c "pidesigner-bridge" /tmp/e2e.preview.html | sed 's/^/  bridge injections: /'

echo "==> tool usage summary"
python3 - <<'PY'
import json, collections
tools = collections.Counter()
n = 0
try:
    for line in open('/tmp/e2e.events'):
        line = line.strip()
        if not line.startswith('data: '):
            continue
        try:
            d = json.loads(line[6:])
        except Exception:
            continue
        n += 1
        if d.get('type') == 'tool':
            tools[d.get('tool')] += 1
except FileNotFoundError:
    pass
print('  events:', n)
for k, v in tools.most_common():
    print(f'    {k}: {v}')
PY

echo "==> assistant message"
curl -s "$BASE/api/conversations/$CID/messages" | python3 -c "
import json,sys
msgs=json.load(sys.stdin)['messages']
for m in msgs:
    print(f\"  [{m['role']}] {len(m['content'])} chars\")
    if m['role']=='assistant':
        print('   ', m['content'][:300].replace(chr(10),' '))"

echo "E2E_DONE status=$STATUS" | tee "$OUT"
