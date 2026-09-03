#!/bin/sh
set -eu

REPO_FILE="/opt/etc/opkg/dns-monitor.conf"
if [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg; else OPKG=opkg; fi

for pkg in \
    dns-monitor-profiling \
    dns-monitor-network \
    dns-monitor-storage \
    dns-monitor-thermal \
    dns-monitor-system \
    dns-monitor-admin
do
    if "$OPKG" list-installed 2>/dev/null | grep -q "^$pkg "; then
        "$OPKG" remove "$pkg"
    fi
done

if "$OPKG" list-installed 2>/dev/null | grep -q '^dns-monitor '; then
    "$OPKG" remove dns-monitor
fi

rm -f "$REPO_FILE"
printf 'DNS Monitor packages and repository entry removed.\n'
