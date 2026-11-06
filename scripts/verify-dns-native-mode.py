#!/usr/bin/env python3
"""Read-only source gate for DNS native Svelte 5 mode and status boundaries."""
from pathlib import Path
import re, sys
ROOT=Path(__file__).resolve().parent.parent
BUILD=(ROOT/'scripts/build-dns-router-experimental.sh').read_text(encoding='utf-8')
problems=[]
def gate(label,ok):
    print(('PASS' if ok else 'FAIL')+': '+label,flush=True)
    if not ok:problems.append(label)
for name, marker, event in [
    ('dns-router-lite','rf-dns-lite-svelte-mode','__rfDnsLiteSvelteLoaded'),
    ('dns-router-runtime','rf-dns-runtime-svelte-mode','__rfDNSRuntimeSvelteLoaded'),
]:
    f=ROOT/'modules'/name/'frontend'
    html=(f/'index.html').read_text(encoding='utf-8')
    entry=(f/'main.js').read_text(encoding='utf-8')
    component=(f/('DnsModeStatus.svelte' if name.endswith('lite') else 'DnsRuntimeMode.svelte')).read_text(encoding='utf-8')
    gate(name+' native mode mount',f'id="{marker}"' in html and marker in entry and 'mount(' in entry and 'flushSync()' in entry)
    gate(name+' bundle is shipped before standalone legacy', 'src="svelte-mode.js"' in html and (name.endswith('lite') or html.index('src="svelte-mode.js"') < html.index('src="app.js"')))
    gate(name+' Svelte owns buttons',all(x in component for x in ('$state(', 'aria-pressed=', 'onclick=', 'onMount(', 'clearInterval(')))
    gate(name+' compatibility fallback', event in entry and event in (f/('DnsP22Visual.svelte' if name.endswith('lite') else 'app.js')).read_text(encoding='utf-8'))
    gate(name+' no mode mutation APIs',not re.search(r'''method\s*:\s*['"](?:POST|PUT|DELETE|PATCH)['"]''',component))
    gate(name+' fetch read-only', "method:'GET'" in component or "fetch('../shadow/status'" in component)
    gate(name+' build input',all((f/x).exists() for x in ('package.json','vite.config.js','main.js')))
    gate(name+' original action controls intact','id="reloadButton"' in html)
for key in ('npm run build','svelte-mode.js','for file in index.html module.css'):
    gate('DNS IPK builder '+key,key in BUILD)
print('DNS_SVELTE_NATIVE_MODE_GATE: '+('FAIL' if problems else 'PASS'),flush=True)
sys.exit(bool(problems))
