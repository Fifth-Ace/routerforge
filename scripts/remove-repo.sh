#!/bin/sh
set -eu

if [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg; else OPKG=opkg; fi

for pkg in \
    routerforge-profiling routerforge-network routerforge-storage routerforge-thermal \
    routerforge-system routerforge-admin routerforge-dns \
    dns-monitor-profiling dns-monitor-network dns-monitor-storage dns-monitor-thermal \
    dns-monitor-system dns-monitor-admin
do
    if "$OPKG" list-installed 2>/dev/null | grep -q "^$pkg "; then
        "$OPKG" remove "$pkg" || true
    fi
done

if "$OPKG" list-installed 2>/dev/null | grep -q '^routerforge-core '; then
    "$OPKG" remove routerforge-core || true
fi
if "$OPKG" list-installed 2>/dev/null | grep -q '^dns-monitor '; then
    "$OPKG" remove dns-monitor || true
fi

rm -f /opt/etc/opkg/dns-monitor.conf /opt/etc/opkg/routerforge.conf
printf 'RouterForge packages removed. Configuration directories were preserved.\n'
