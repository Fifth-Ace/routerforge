#!/usr/bin/env python3
"""DNS Lite shared reactive state + guarded controller split; read-only static checks."""
from pathlib import Path
import subprocess
r=Path(__file__).resolve().parents[1]
f=r/'modules/dns-router-lite/frontend'
s=(f/'DnsLiteController.svelte').read_text(encoding='utf-8')
j=(f/'DnsLiteControllerLogic.js').read_text(encoding='utf-8')
e=(f/'main.js').read_text(encoding='utf-8')
assert len(s)<12000 and len(j)>50000, 'LITE_CONTROLLER_EXTRACTION_TRUNCATED'
assert 'RF_ACTUAL_CONTROLLER_START' not in s
assert 'const ui = $state({' in s and 'const state = $state({' in s
assert 'onMount(() => {' in s and 'startDnsLiteShellBridge()' in s and 'startDnsLiteController({state,ui,presentation' in s
assert 'startDnsLiteController' in j and 'return () => cleanup()' in j
assert 'shellBridge.dispose()' in s and 'apiClient.dispose()' in j and 'document.getElementById' not in j
assert 'ui.cardsView = list.map(' in j and '<DnsLiteCards cards={ui.cardsView}' in s
assert 'ui.initialized=true' in j and '{#if ui.initialized}' in s
for name in ("'gate-preflight':()=>task","'gate-enable':()=>task","'gate-disable':()=>task","'dns-commit':()=>task","'apply':()=>task"):
    assert name in j, 'LITE_GUARDED_ACTION_MISSING: '+name
for value in ('If-Match','X-RouterForge-Action', 'editorPending()','apiClient.dispose()','clearInterval(timer)'):
    assert value in j, 'LITE_PROTECTED_LIFECYCLE_MISSING: '+value
for forbidden in ("method:'POST'", 'fetch(', 'XMLHttpRequest', '.innerHTML'):
    assert forbidden not in s, 'LITE_SVELTE_VIEW_HAS_MUTATION_CODE: '+forbidden
assert 'mount(DnsLiteController' in e
subprocess.run(['node','--check',str(f/'DnsLiteControllerLogic.js')],check=True)
print('DNS_LITE_SVELTE5_SHARED_CONTROLLER: PASS / 1 reactive root / guarded actions preserved')
print('DNS_LITE_SOURCE_UI_OWNERSHIP: PASS / controller uses no getElementById; Svelte owns visible controls')
print('DNS_LITE_L07_L08_SIGNOFF: PENDING / real browser, backend IPK and upgrade evidence')
