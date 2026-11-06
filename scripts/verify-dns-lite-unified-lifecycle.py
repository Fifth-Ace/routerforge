#!/usr/bin/env python3
from pathlib import Path
import sys
root=Path(__file__).resolve().parents[1]
f=root/'modules/dns-router-lite/frontend'
html=(f/'index.html').read_text(encoding='utf-8')
entry=(f/'main.js').read_text(encoding='utf-8')
for old in ('app.js','p22-visual.js','p25-settings.js'):
    assert not (f/old).exists(), 'LEGACY_SOURCE_STILL_PRESENT: '+old
    assert 'src="'+old+'"' not in html, 'LEGACY_SCRIPT_STILL_REFERENCED: '+old
for name in ('DnsLiteController','DnsP22Visual','DnsP25Settings'):
    s=(f/(name+'.svelte')).read_text(encoding='utf-8')
    assert s.count('RF_ACTUAL_CONTROLLER_START')==1
    assert s.count('RF_ACTUAL_CONTROLLER_END')==1
    assert "import {onMount} from 'svelte'" in s
    # Svelte 5 reserves a bare `$` identifier even inside onMount. Keep the
    # extracted business controllers compilable before expensive npm gates.
    assert 'const $ =' not in s and '$(' not in s, 'SVELTE_RESERVED_DOLLAR_IDENTIFIER: '+name
    assert 'const byId = id => document.getElementById(id)' in s, 'DOM_ACCESSOR_MISSING: '+name
    assert 'mount(' in entry and name in entry
assert 'src="svelte-mode.js"' in html
assert 'gateEnable' in (f/'DnsLiteController.svelte').read_text(encoding='utf-8')
print('DNS_LITE_UNIFIED_SVELTE_LIFECYCLE_GATE: PASS')
print('NOTE: IMPERATIVE_CONTROLLER_STILL_EMBEDDED / FULL_MIGRATION_INCOMPLETE')
