#!/bin/sh
set -eu

CHANNEL="${ROUTERFORGE_CHANNEL:-stable}"

case "$CHANNEL" in
    stable|beta) ;;
    *) printf 'ERROR: ROUTERFORGE_CHANNEL must be stable or beta.\n' >&2; exit 1 ;;
esac

BASE="https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-$CHANNEL"
ASSET="routerforge-$CHANNEL-bootstrap.sh"
TMP="/opt/tmp/routerforge-bootstrap-launcher.$$"

fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"
    elif [ -x /opt/bin/curl ]; then
        /opt/bin/curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$2" "$1"
    elif [ -x /opt/bin/wget ]; then
        /opt/bin/wget -q -O "$2" "$1"
    else
        printf 'ERROR: curl/wget was not found.\n' >&2
        exit 1
    fi
}

mkdir -p /opt/tmp
trap 'rm -f "$TMP"' EXIT HUP INT TERM

printf 'RouterForge %s launcher\n' "$CHANNEL"
fetch "$BASE/$ASSET" "$TMP"
sh "$TMP"
