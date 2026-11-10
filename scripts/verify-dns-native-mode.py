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
# DNS Lite still has a temporary compatibility controller and remains INCOMPLETE.
# Runtime has a single Svelte 5 root; do not require the deleted legacy app.js.
for name in ('dns-router-lite','dns-router-runtime'):
    f=ROOT/'modules'/name/'frontend'
    html=(f/'index.html').read_text(encoding='utf-8')
    entry=(f/'main.js').read_text(encoding='utf-8')
    component=(f/('DnsModeStatus.svelte' if name.endswith('lite') else 'DnsRuntimeMode.svelte')).read_text(encoding='utf-8')
    if name.endswith('lite'):
        gate(name+' transitional native mode mount', 'rf-dns-lite-svelte-mode' in html and 'mount(' in entry and 'flushSync()' in entry)
        gate(name+' Svelte mode controls',all(x in component for x in ('$state(', 'aria-pressed=', 'onclick=', 'onMount(', 'clearInterval(')))
        gate(name+' compatibility event', '__rfDnsLiteSvelteLoaded' in entry and '__rfDnsLiteSvelteLoaded' in (f/'DnsP22Visual.svelte').read_text(encoding='utf-8'))
        gate(name+' original controls retained', 'id="reloadButton"' in html)
    else:
        app=(f/'DnsRuntimeApp.svelte').read_text(encoding='utf-8')
        gate(name+' single native Svelte root', "mount(DnsRuntimeApp" in entry and '<DnsRuntimeMode' in app and 'rf-dns-runtime-svelte-mode' in app)
        gate(name+' no legacy controller', not (f/'app.js').exists() and 'src="app.js"' not in html)
        gate(name+' Svelte mode controls',all(x in component for x in ('aria-pressed=', 'onclick=', 'onMount(', 'clearInterval(')))
        gate(name+' original reload control retained in native root', '<DnsRuntimeConfigActions kind="reload"' in app)
    gate(name+' bundle entry', 'src="svelte-mode.js"' in html)
    gate(name+' no mode mutation API',not re.search('method.*(POST|PUT|DELETE|PATCH)',component))
    gate(name+' build input',all((f/x).exists() for x in ('package.json','vite.config.js','main.js')))
for key in ('npm run build','svelte-mode.js','for file in index.html module.css'):
    gate('DNS IPK builder '+key,key in BUILD)
print('DNS_SVELTE_NATIVE_MODE_GATE: '+('FAIL' if problems else 'PASS'),flush=True)
sys.exit(bool(problems))
