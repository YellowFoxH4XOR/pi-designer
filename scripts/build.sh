#!/usr/bin/env bash
# Build pidesigner: the Go daemon, the MCP server, and the UI bundle.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> go build"
mkdir -p bin
go build -trimpath -ldflags="-s -w" -o bin/pidesigner ./cmd/pidesigner
go build -trimpath -ldflags="-s -w" -o bin/pidesigner-mcp ./cmd/pidesigner-mcp

echo "==> ui build"
if [ -d ui/node_modules ]; then
  (cd ui && npm run build --silent)
else
  echo "  ui/node_modules missing; run: cd ui && npm install"
fi

echo
echo "built:"
ls -lh bin/ | tail -n +2 | awk '{printf "  %-22s %s\n", $9, $5}'
[ -d ui/dist ] && du -sh ui/dist | awk '{print "  ui/dist                "$1}'
