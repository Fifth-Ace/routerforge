#!/bin/sh
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FRONTEND="$ROOT/modules/monitoring/frontend"
command -v npm >/dev/null 2>&1 || { echo "npm is required to build RouterForge Monitoring frontend" >&2; exit 1; }
cd "$FRONTEND"
[ -d node_modules ] || npm install --no-audit --no-fund
npm run build
test -f "$FRONTEND/build/index.html"