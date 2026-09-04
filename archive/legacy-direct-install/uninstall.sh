#!/bin/sh
set -e
if [ -x /opt/etc/init.d/S90routerforge ]; then /opt/etc/init.d/S90routerforge stop >/dev/null 2>&1 || true; fi
rm -f /opt/etc/init.d/S90routerforge /opt/bin/routerforge
printf 'RouterForge Core removed. Configuration under /opt/etc/routerforge is preserved.\n'
