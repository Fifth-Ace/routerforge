#!/bin/sh
set -e
BASE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
mkdir -p /opt/bin /opt/etc/init.d /opt/var/log /opt/share/licenses/dns-monitor
if [ -x /opt/etc/init.d/S90dns-monitor ]; then /opt/etc/init.d/S90dns-monitor stop >/dev/null 2>&1 || true; fi
cp "$BASE/dns-monitor-linux-arm64" /opt/bin/dns-monitor
chmod 0755 /opt/bin/dns-monitor
cp "$BASE/S90dns-monitor" /opt/etc/init.d/S90dns-monitor
chmod 0755 /opt/etc/init.d/S90dns-monitor
if [ -f "$BASE/LICENSE" ]; then
    cp "$BASE/LICENSE" /opt/share/licenses/dns-monitor/LICENSE
    chmod 0644 /opt/share/licenses/dns-monitor/LICENSE
fi
/opt/etc/init.d/S90dns-monitor start
printf '\nDNS Monitor installed.\nWeb: http://<router-ip>:2233\nLog: /opt/var/log/dns-monitor.log\n'
