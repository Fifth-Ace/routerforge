#!/usr/bin/env python3
"""P11 source contracts. Real npm, UI regressions and Linux CI remain mandatory."""
from pathlib import Path
import sys

root=Path(__file__).resolve().parents[1]
lite=root/'modules/dns-router-lite/frontend'
runtime=root/'modules/dns-router-runtime/frontend'
checks=[]
def src(p):return p.read_text(encoding='utf-8') if p.is_file() else ''
def gate(name,passed):
    passed=bool(passed)
    checks.append((name,passed))
    print(('PASS' if passed else 'FAIL')+': '+name,flush=True)

lm=src(lite/'main.js');la=src(lite/'DnsLiteController.svelte');lh=src(lite/'index.html');ls=src(lite/'DnsLiteDomainEditor.svelte')
rm=src(runtime/'main.js');ra=src(runtime/'app.js');rh=src(runtime/'index.html');rs=src(runtime/'DnsRuntimePolicyForms.svelte')
gate('Lite native list editor mounted',all(x in lm for x in ('DnsLiteDomainEditor.svelte','mount(DnsLiteDomainEditor','flushSync()')) and 'rf-dns-lite-svelte-editor' in lh)
gate('Lite Svelte complete list actions',all(x in ls for x in ('createList','renameList','saveList','selectList','domainInput','deleteListButton','importDomains','exportDomains','closeEditor')))
gate('Lite draft controller only',all(x in ls for x in ('newListButton','renameListButton','saveDomains','editorDomains','listChoice','newListName')) and all(x not in ls for x in ('fetch(',"method:'POST'","method:'PUT'",'innerHTML')))
gate('Lite old unsaved text gates retained',all(x in la for x in ('const editorPending=()',"if(editorPending()&&!confirm(",'function saveList(){','function renameList(){')))
gate('Lite state is read-only snapshot',"window.__rfDnsLiteEditorSnapshot=()=>" in la and 'routerforge:dns-lite-editor-state' in la)
gate('Lite native and legacy fallback',"window.__rfDnsLiteNativeEditorLoaded" in la and 'id="listEditor"' in lh)
gate('Runtime Svelte policy editors mounted',all(x in rm for x in ('DnsRuntimePolicyForms.svelte','mount(DnsRuntimePolicyForms','props:{scope}')) and 'rf-dns-runtime-svelte-upstream-form' in rh and 'rf-dns-runtime-svelte-rule-form' in rh and 'rf-dns-runtime-svelte-listener-form' in rh)
gate('Runtime forms reactively bound',all(x in rs for x in ('$state(', '$props()', 'bind:value={upstream}', 'bind:value={poolAddress}', 'bind:value={listenerAddress}', 'onsubmit={saveRule}', 'onsubmit={saveListener}')))
gate('Runtime controller submit gates retained',all(x in ra for x in ("el('ruleForm').addEventListener('submit'", "el('listenerForm').addEventListener('submit'", "'If-Match':'\"'+revision+'\"'")))
gate('Runtime forms do not mutate DNS without explicit submit',all(x in rs for x in ('requestSubmit()', 'event.preventDefault()', 'submitLegacy(')) and not any(x in rs for x in ('fetch(', 'XMLHttpRequest','innerHTML','localStorage')))
gate('Runtime native upstream form retains Keenetic selector',all(x in rs for x in ('selectPoolTransport',"poolTransport==='native'","submitLegacy('upstreamForm'")) and 'id="p85NativePicker"' in rh)
gate('Runtime read-only state bridge',"window.__rfDNSRuntimeFormsSnapshot=()=>" in ra and 'routerforge:dns-runtime-form-state' in ra)
gate('Runtime native picker untouched',all(x in rh for x in ('id="p85NativePicker"','id="p85NativeScan"','id="p87NativeProbe"')))
errors=[name for name,ok in checks if not ok]
print('DNS_P11_DRAFT_EDITOR_SOURCE_GATES: '+('FAIL / '+','.join(errors) if errors else 'PASS'))
sys.exit(bool(errors))
