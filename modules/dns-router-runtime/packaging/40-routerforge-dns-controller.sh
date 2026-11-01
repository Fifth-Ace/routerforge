#!/bin/sh
# RouterForge DNS Controller P19. Notify only; never edit firewall or start DNS.
# NDM may hold xtables.lock while this hook runs, so reconciliation is queued.
BIN=/opt/bin/routerforge-dns-router
SOCK=/opt/var/run/routerforge-dns-router.sock
[ -x "$BIN" ] && [ -S "$SOCK" ] || exit 0
"$BIN" --socket "$SOCK" --gate-notify >/dev/null 2>&1 || :
exit 0
