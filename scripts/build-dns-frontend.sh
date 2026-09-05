#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FRONTEND="$ROOT/modules/dns/frontend"

if ! command -v npm >/dev/null 2>&1; then
    echo "npm is required to build the RouterForge DNS module frontend" >&2
    exit 1
fi

cd "$FRONTEND"
if [ ! -d node_modules ]; then
    npm install --no-audit --no-fund
fi
npm run build

test -f "$FRONTEND/build/index.html"
