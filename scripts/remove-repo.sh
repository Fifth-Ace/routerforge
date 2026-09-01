#!/bin/sh
set -eu
REPO_FILE="/opt/etc/opkg/dns-monitor.conf"
if [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg; else OPKG=opkg; fi

if "$OPKG" list-installed 2>/dev/null | grep -q '^dns-monitor '; then
    "$OPKG" remove dns-monitor
fi
rm -f "$REPO_FILE"
printf 'DNS Monitor package and repository entry removed.\n'
