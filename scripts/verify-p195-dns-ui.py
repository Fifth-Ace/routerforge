#!/usr/bin/env python3
from pathlib import Path
h=Path('modules/dns-router-lite/frontend/index.html').read_text()
j=Path('modules/dns-router-lite/frontend/DnsLiteController.svelte').read_text()
logic=Path('modules/dns-router-lite/frontend/DnsLiteControllerLogic.js').read_text()
cards=Path('modules/dns-router-lite/frontend/DnsLiteCards.svelte').read_text()
controls=Path('modules/dns-router-lite/frontend/DnsLiteControls.svelte').read_text()
c=Path('modules/dns-router-lite/frontend/module.css').read_text()
g=Path('components/core/catalog.go').read_text()
m=Path('components/core/marketplace_install.go').read_text()
assert 'target="_top"' in h and 'id="nativeDNSLink"' in h
assert 'id="gateShell"' in h and 'id="gateCompactBadge"' in h
assert 'id="healthOverview"' in h and 'class="health-overview"' in h
assert 'refreshLogs().catch(()=>{})' in logic and "state.page!=='dns'" in logic
# P19.5 cards are Svelte-owned; the controller only supplies reactive props.
assert 'class="dns-card-metrics"' in cards and '{card.listCount}' in cards and '{card.domainCount}' in cards, 'P195_DNS_METRICS_NOT_OWNED_BY_SVELTE_CARDS'
assert "use:portal={'dnsCards'}" in j and '<DnsLiteCards cards=' in j, 'P195_SVELTE_CARD_MOUNT_MISSING'
assert 'startDnsLiteController' in j and 'startDnsLiteController' in logic, 'P195_DNS_SVELTE_SHARED_CONTROLLER_NOT_MOUNTED'
assert '.gate-shell' in c, 'P195_GATE_SHELL_STYLING_MISSING'
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
# Svelte controls own the one visible editor selector; index.html owns only its portal.
assert h.count('id="editorDNSChoice"') == 0, 'P195_DUPLICATE_LEGACY_EDITOR_SELECTOR'
assert controls.count('id="editorDNSChoice"') == 1, 'P195_SVELTE_EDITOR_SELECTOR_NOT_UNIQUE'
assert 'id="rf-lite-editor-choice-slot"' in h and "use:portal={'rf-lite-editor-choice-slot'}" in controls, 'P195_EDITOR_SELECTOR_PORTAL_MISSING'
# Require rejected-change rollback on the SINGLE Svelte-owned dropdown.
# The legacy unguarded string was an obsolete P19.5 snapshot, not the safe UI contract.
from p195_dns_editor_selector_contract import verify_editor_selector_contract
verify_editor_selector_contract(controls)
assert 'const initial=state.config.upstreams.find' in logic
assert "'editor-choice':(value)" in logic and 'value={chosen}' in controls
assert 'document.getElementById' not in logic, 'P195_LEGACY_DOM_SELECTOR_OWNERSHIP_REMAINS'
assert '/* P19.5b — reference-render parity.' in c
assert 'grid-template-columns: minmax(425px,1.16fr) minmax(400px,1fr)' in c
print('P195B_RENDER_PARITY_AND_READ_ONLY_SELECTION: PASS')
print('P195_DNS_UI_LIFECYCLE_SOURCE_CONTRACT: PASS')
