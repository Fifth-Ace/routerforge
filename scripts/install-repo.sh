#!/bin/sh
set -eu

VERSION="0.3.0-beta"
ARCH="aarch64-3.10"
BASE_URL="https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-beta"
SUMS="routerforge-beta-SHA256SUMS"
TMP="/opt/tmp/routerforge-bootstrap.$$"

say() { printf '%s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

case "$(uname -m 2>/dev/null || true)" in
    aarch64|arm64) ;;
    *) fail "RouterForge beta supports ARM64/aarch64 Keenetic/Netcraze routers." ;;
esac

[ -d /opt ] || fail "Entware /opt was not found."
if [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg
elif command -v opkg >/dev/null 2>&1; then OPKG="$(command -v opkg)"
else fail "opkg was not found."; fi

if command -v curl >/dev/null 2>&1; then
    fetch() { curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"; }
elif [ -x /opt/bin/curl ]; then
    fetch() { /opt/bin/curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
    fetch() { wget -q -O "$2" "$1"; }
elif [ -x /opt/bin/wget ]; then
    fetch() { /opt/bin/wget -q -O "$2" "$1"; }
else
    fail "curl/wget was not found."
fi

command -v sha256sum >/dev/null 2>&1 || fail "sha256sum was not found."

mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT HUP INT TERM

say "RouterForge beta bootstrap"
fetch "$BASE_URL/$SUMS" "$TMP/$SUMS"

install_asset() {
    package="$1"
    asset="${package}_${VERSION}_${ARCH}.ipk"
    expected="$(awk -v f="$asset" '$2 == f || $2 == "*"f {print $1; exit}' "$TMP/$SUMS")"
    [ -n "$expected" ] || fail "Checksum for $asset is missing."
    fetch "$BASE_URL/$asset" "$TMP/$asset"
    actual="$(sha256sum "$TMP/$asset" | awk '{print $1}')"
    [ "$actual" = "$expected" ] || fail "SHA256 mismatch for $asset."
    say "Installing $package..."
    "$OPKG" install "$TMP/$asset"
}

install_asset routerforge-core
install_asset routerforge-dns

say ""
say "RouterForge beta is ready."
say "Web UI: http://<router-ip>:2233"
say "Core: /opt/etc/init.d/S90routerforge"
say "Log: /opt/var/log/routerforge.log"
say "Optional capabilities are installed from Marketplace."
