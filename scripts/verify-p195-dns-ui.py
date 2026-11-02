#!/usr/bin/env python3
from pathlib import Path
h=Path('modules/dns-router-lite/frontend/index.html').read_text()
j=Path('modules/dns-router-lite/frontend/app.js').read_text()
c=Path('modules/dns-router-lite/frontend/module.css').read_text()
g=Path('components/core/catalog.go').read_text()
m=Path('components/core/marketplace_install.go').read_text()
assert 'target="_top"' in h and 'id="nativeDNSLink"' in h
assert 'id="gateShell"' in h and 'id="gateCompactBadge"' in h
assert 'id="healthOverview"' in h and 'class="health-overview"' in h
assert 'refreshLogs().catch(()=>{})' in j and "state.page!=='dns'" in j
assert 'dns-card-metrics' in j and '.gate-shell' in c
assert 'dnsRouterCatalogEngineActive()' in g
assert m.count('routerForgeIPKOpkgArgs(action,') == 2
assert 'service_enabled' in Path('modules/dns-router-lite/intent.go').read_text()
assert 'case "maintenance":' in Path('modules/dns-router-lite/lifecycle.go').read_text()
print('P195_DNS_UI_LIFECYCLE_SOURCE_CONTRACT: PASS')
