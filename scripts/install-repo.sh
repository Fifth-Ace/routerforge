#!/bin/sh
set -eu

REPO_NAME="dns-monitor"
REPO_URL="https://raw.githubusercontent.com/Fifth-Ace/dns-monitor/opkg"
REPO_FILE="/opt/etc/opkg/${REPO_NAME}.conf"
PKG="dns-monitor"

say() { printf '%s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

case "$(uname -m 2>/dev/null || true)" in
    aarch64|arm64) ;;
    *) fail "DNS Monitor currently supports only ARM64/aarch64 Keenetic/Netcraze routers." ;;
esac

[ -d /opt ] || fail "Entware /opt was not found. Install Entware first."

if [ -x /opt/bin/opkg ]; then
    OPKG=/opt/bin/opkg
elif command -v opkg >/dev/null 2>&1; then
    OPKG="$(command -v opkg)"
else
    fail "opkg was not found. Install Entware first."
fi

if command -v wget >/dev/null 2>&1; then
    WGET="$(command -v wget)"
elif [ -x /opt/bin/wget ]; then
    WGET=/opt/bin/wget
else
    fail "wget was not found."
fi

say "DNS Monitor repository installer"
say "Target: Keenetic / Netcraze ARM64"
say "Repository: ${REPO_URL}"

TMP="/tmp/dns-monitor-Packages.gz.$$"
trap 'rm -f "$TMP"' EXIT HUP INT TERM
"$WGET" -q -O "$TMP" "${REPO_URL}/Packages.gz" || fail "Cannot reach DNS Monitor repository."
[ -s "$TMP" ] || fail "Repository index is empty."
rm -f "$TMP"
trap - EXIT HUP INT TERM

mkdir -p /opt/etc/opkg
printf 'src/gz %s %s\n' "$REPO_NAME" "$REPO_URL" > "$REPO_FILE"
say "Repository added: ${REPO_FILE}"

# Other third-party feeds may occasionally fail. Do not abort if our feed was
# reachable and opkg reports an unrelated repository error.
"$OPKG" update || say "WARNING: opkg update reported an error in one or more feeds; continuing."

if "$OPKG" list-installed 2>/dev/null | grep -q '^dns-monitor '; then
    say "DNS Monitor is already installed; upgrading..."
    "$OPKG" upgrade "$PKG" || "$OPKG" install "$PKG"
else
    say "Installing DNS Monitor..."
    "$OPKG" install "$PKG"
fi

say ""
say "DNS Monitor is ready."
say "Web UI: http://<router-ip>:2233"
say "Service: /opt/etc/init.d/S90dns-monitor"
say "Log: /opt/var/log/dns-monitor.log"
