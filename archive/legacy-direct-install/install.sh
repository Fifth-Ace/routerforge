#!/bin/sh
set -e
BASE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
mkdir -p /opt/bin /opt/etc/init.d /opt/etc/routerforge /opt/var/log /opt/share/licenses/routerforge-core
if [ -x /opt/etc/init.d/S90routerforge ]; then /opt/etc/init.d/S90routerforge stop >/dev/null 2>&1 || true; fi
cp "$BASE/routerforge-linux-arm64" /opt/bin/routerforge
chmod 0755 /opt/bin/routerforge
cp "$BASE/S90routerforge" /opt/etc/init.d/S90routerforge
chmod 0755 /opt/etc/init.d/S90routerforge
: > /opt/etc/routerforge/package-management.enabled
if [ -f "$BASE/LICENSE" ]; then
    cp "$BASE/LICENSE" /opt/share/licenses/routerforge-core/LICENSE
    chmod 0644 /opt/share/licenses/routerforge-core/LICENSE
fi
/opt/etc/init.d/S90routerforge start
printf '\nRouterForge installed.\nWeb: http://<router-ip>:2233\nLog: /opt/var/log/routerforge.log\n'
