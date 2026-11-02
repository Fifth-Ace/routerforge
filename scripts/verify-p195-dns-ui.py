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
# Build-opkg intentionally compiles an explicit Core ABI source list.
# Both P19.5 Core helpers must be present in that exact list, not only in go test ./components/core.
core_build = Path('scripts/build-opkg.sh').read_text(encoding='utf-8')
core_sources = core_build.split('CORE_SOURCES="', 1)[1].split('"', 1)[0].splitlines()
for source in ('dns_router_catalog_runtime.go', 'dns_router_upgrade_args.go'):
    assert core_sources.count(source) == 1, f'Core IPK build missing P19.5 helper: {source}'
    assert (Path('components/core') / source).is_file(), f'missing Core helper source: {source}'
print('P195_CORE_EXPLICIT_SOURCE_LIST: PASS')
# P19.5b: the right pane must show real DNS/list data on initial render; never call a mutation API.
assert h.count('id="editorDNSChoice"') == 1
assert 'const initial=state.config.upstreams.find' in j
assert "if(!editList(chosen))$('editorDNSChoice').value=previous" in j
assert '/* P19.5b — reference-render parity.' in c
assert 'grid-template-columns: minmax(425px,1.16fr) minmax(400px,1fr)' in c
print('P195B_RENDER_PARITY_AND_READ_ONLY_SELECTION: PASS')
print('P195_DNS_UI_LIFECYCLE_SOURCE_CONTRACT: PASS')
