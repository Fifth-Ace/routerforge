#!/usr/bin/env python3
"""Fail-closed P10 DNS frontend ownership and protected-action safety inventory.

Real Vite builds, JS regressions, IPK checks and Linux CI are independent gates.
"""
from pathlib import Path
import sys
R=Path(__file__).resolve().parents[1]
L=R/'modules/dns-router-lite/frontend'
E=R/'modules/dns-router-runtime/frontend'
BUILD=R/'scripts/build-dns-router-experimental.sh'
checks=[]
def text(p):return p.read_text(encoding='utf-8') if p.is_file() else ''
def gate(name,yes):
    yes=bool(yes);checks.append((name,yes))
    print(('PASS' if yes else 'FAIL')+': '+name,flush=True)
def main():
    lc=text(L/'DnsLiteCards.svelte');la=text(L/'DnsLiteController.svelte');lm=text(L/'main.js')
    ec=text(E/'DnsRuntimeTables.svelte');ea=text(E/'DnsRuntimeApp.svelte');es=text(E/'DnsRuntimeStore.svelte.js');em=text(E/'main.js')
    b=text(BUILD)
    gate('P10 Lite upstream cards owned by compiled Svelte', all(x in lm for x in (
        "import DnsLiteCards from './DnsLiteCards.svelte'",'mount(DnsLiteCards','window.__rfDnsLiteCardsLoaded=true','flushSync();')))
    gate('P10 Lite card display reactive state', all(x in lc for x in (
        'let cards = $state([])','let selected = $state(', 'let health = $state(', '{#each cards as card (card.id)}','data-upstream-id={card.id}')))
    gate('P10 Lite card actions all bound via guarded bridge', all(x in lc for x in (
        "action('edit'","action('change'","action('check'","action('fallback'","action('remove'",'__rfDnsLiteCardsAPI')))
    gate('P10 Lite fallback and list actions constrained by original controller', all(x in la for x in (
        'window.__rfDnsLiteCardsAPI={','state.config.upstreams.find(x=>x.id===id)',
        'upstream.fallbacks=value?[value]:[];mark();',"if(value&&state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))",
        'edit:id=>editList(id)','change:id=>openDNSForm(id)','remove:id=>task(()=>removeDNS(id),false)')))
    gate('P10 Lite controller delegates card data and health', all(x in la for x in (
        "if(!window.__rfDnsLiteCardsLoaded)root.replaceChildren();",'routerforge:dns-lite-cards',
        'routerforge:dns-lite-card-health','if(window.__rfDnsLiteCardsLoaded)continue;')))
    gate('P10 Svelte card uses no direct DNS network mutation', all(x not in lc for x in (
        'fetch(', 'XMLHttpRequest','innerHTML','POST','api/modules/dns','service/gate/control','service/control')))
    gate('P10 Lite source preserves DNS Gate safety flow', all(x in la for x in (
        "on('gateEnable'","on('gateDisable'","on('commitDNS'","on('applyButton'",'editorPending()')))
    gate('P10 Lite legacy DOM tests keep fallback renderer', all(x in la for x in (
        "if(!list.length){const p=document.createElement('p')", "const edit=document.createElement('button')",'root.appendChild(card);')))
    gate('P10 Runtime tables mounted by one Svelte root', all(x in ea for x in (
        "import DnsRuntimeTables from './DnsRuntimeTables.svelte'",'<DnsRuntimeTables target="upstreamList"', '<DnsRuntimeTables target="ruleList"')) and 'mount(DnsRuntimeApp' in em)
    gate('P10 Runtime reactive rule/upstream tables', all(x in ec for x in (
        'let items=$derived(', 'let rows=$derived(', '<table>', '{#each rows as values,index (index)}', 'removeRow(', 'editRow(')))
    gate('P10 Runtime guarded removal moves to native store', all(x in es for x in (
        'export function removeRow(', 'config.service?.default_upstream===id', 'config.rules.some(r=>r.upstream===id)', 'items.some(u=>(u.fallbacks||[]).includes(id))', 'markDirty(')))
    gate('P10 Runtime native tables contain no network mutation', all(x not in ec for x in (
        'fetch(', 'innerHTML', 'XMLHttpRequest', 'shadow/control', 'keenetic/domain-managed/trial')))
    gate('P10 Runtime action ownership is explicit native root', all(x in ea for x in (
        'DnsRuntimeConfigActions', 'DnsRuntimeManagedTrial', 'DnsRuntimeShadowControls', 'DnsRuntimePolicyForms')))
    gate('P10 original packaged CSS and compiled Svelte kept', all(x in b for x in (
        'svelte-mode.js', 'npm run build','for file in index.html module.css')))
    failed=[name for name,ok in checks if not ok]
    print('DNS_P10_NATIVE_EDITOR_GATES: '+('FAIL / '+', '.join(failed) if failed else 'PASS'),flush=True)
    return 1 if failed else 0
if __name__=='__main__':sys.exit(main())
