#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FRONTEND="$ROOT/components/core/frontend"

if ! command -v npm >/dev/null 2>&1; then
    echo "npm is required to build the RouterForge Core frontend" >&2
    exit 1
fi

cd "$FRONTEND"
npm install --no-audit --no-fund
npm run check
npm run build

test -f "$FRONTEND/build/index.html"
