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
rm=src(runtime/'main.js');ra=src(runtime/'DnsRuntimeApp.svelte');rh=src(runtime/'index.html');rs=src(runtime/'DnsRuntimePolicyForms.svelte');store=src(runtime/'DnsRuntimeStore.svelte.js')
gate('Lite native list editor mounted',all(x in lm for x in ('DnsLiteDomainEditor.svelte','mount(DnsLiteDomainEditor','flushSync()')) and 'rf-dns-lite-svelte-editor' in lh)
gate('Lite Svelte complete list actions',all(x in ls for x in ('createList','renameList','saveList','selectList','domainInput','deleteListButton','importDomains','exportDomains','closeEditor')))
gate('Lite draft controller only',all(x in ls for x in ('newListButton','renameListButton','saveDomains','editorDomains','listChoice','newListName')) and all(x not in ls for x in ('fetch(',"method:'POST'","method:'PUT'",'innerHTML')))
gate('Lite old unsaved text gates retained',all(x in la for x in ('const editorPending=()',"if(editorPending()&&!confirm(",'function saveList(){','function renameList(){')))
gate('Lite state is read-only snapshot',"window.__rfDnsLiteEditorSnapshot=()=>" in la and 'routerforge:dns-lite-editor-state' in la)
gate('Lite native and legacy fallback',"window.__rfDnsLiteNativeEditorLoaded" in la and 'id="listEditor"' in lh)
gate('Runtime Svelte policy editors mounted by single root',all(x in ra for x in ('DnsRuntimePolicyForms.svelte', '<DnsRuntimePolicyForms scope="upstream"', '<DnsRuntimePolicyForms scope="rule"','<DnsRuntimePolicyForms scope="listener"')) and 'mount(DnsRuntimeApp' in rm)
gate('Runtime forms use reactive state and explicit submit',all(x in rs for x in ('$state(', '$props()', 'bind:value={upstream}', 'bind:value={poolAddress}', 'bind:value={listenerAddress}', 'onsubmit={saveRule}', 'onsubmit={saveListener}')))
gate('Runtime form controller moved to reactive draft owner',all(x in rs for x in ('createDraftEditor(', 'readDraft', 'commitDraft', 'event.preventDefault()')) and 'export function commitDraft' in store)
gate('Runtime forms stage only, never call backend on mount',all(x in rs for x in ('onsubmit={saveUpstream}', 'onsubmit={saveRule}', 'onsubmit={saveListener}', "apply('upstream'", "apply('listener'")) and not any(x in rs for x in ('fetch(', 'XMLHttpRequest', 'innerHTML', 'localStorage')))
gate('Runtime native upstream form retains Keenetic selector',all(x in rs for x in ('DnsRuntimeNative',"poolTransport==='native'",'onsubmit={saveUpstream}')))
gate('Runtime source has no permanent global forms bridge', 'window.__rfDNSRuntimeFormsSnapshot' not in store and 'window.__rfDNSRuntimeFormsSnapshot' not in rs)
gate('Runtime native picker owned by native child', (runtime/'DnsRuntimeNative.svelte').exists() and 'DnsRuntimeNative' in rs and 'src="svelte-mode.js"' in rh and 'app.js' not in rh)
errors=[name for name,ok in checks if not ok]
print('DNS_P11_DRAFT_EDITOR_SOURCE_GATES: '+('FAIL / '+','.join(errors) if errors else 'PASS'))
sys.exit(bool(errors))
