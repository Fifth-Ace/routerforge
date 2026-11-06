#!/bin/sh
set -eu
MODULE="${1:?standalone module required}"
case "$MODULE" in
    crowdsec-manager|network-tools|antiscan-manager) ;;
    *) echo "UNSUPPORTED_SVELTE_MODULE: $MODULE" >&2; exit 1 ;;
esac
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FRONTEND="$ROOT/modules/$MODULE/frontend"
[ -f "$FRONTEND/package.json" ] || { echo "MISSING_FRONTEND_PACKAGE $MODULE" >&2; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "NPM_NOT_FOUND" >&2; exit 1; }
# Project-local dependencies, never npm -g and never a CLI agent.
if [ ! -d "$FRONTEND/node_modules" ]; then
    (cd "$FRONTEND" && npm install --no-save --package-lock=false --ignore-scripts --no-audit --no-fund)
fi
(cd "$FRONTEND" && npm run build)
python3 "$ROOT/scripts/verify-standalone-svelte-ui.py" --module "$MODULE" --dir "$FRONTEND/build"
