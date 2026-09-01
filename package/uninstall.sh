#!/bin/sh
set -e
if [ -x /opt/etc/init.d/S90dns-monitor ]; then /opt/etc/init.d/S90dns-monitor stop >/dev/null 2>&1 || true; fi
rm -f /opt/etc/init.d/S90dns-monitor /opt/bin/dns-monitor
printf 'DNS Monitor removed. Logs were kept in /opt/var/log/dns-monitor.log\n'
