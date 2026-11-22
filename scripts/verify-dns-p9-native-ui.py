#!/usr/bin/env python3
"""P9 fail-closed static gates; real build, UI and IPK tests remain separate."""
from pathlib import Path
import sys
ROOT=Path(__file__).resolve().parents[1]
L=ROOT/'modules/dns-router-lite/frontend'
R=ROOT/'modules/dns-router-runtime/frontend'
B=ROOT/'scripts/build-dns-router-experimental.sh'
checks=[]
def gate(name,ok):
    checks.append((name,bool(ok)))
    print(('PASS' if ok else 'FAIL')+': '+name,flush=True)
def source(p):return p.read_text(encoding='utf-8') if p.is_file() else ''
def main():
    la=source(L/'DnsLiteController.svelte'); lh=source(L/'index.html'); lm=source(L/'main.js'); ls=source(L/'DnsLiteLog.svelte')
    rh=source(R/'index.html'); rm=source(R/'main.js'); rs=source(R/'DnsRuntimeDiagnostics.svelte'); ra=source(R/'DnsRuntimeApp.svelte'); build=source(B)
    gate('P9 native Lite log mounted before legacy app', 'src="svelte-mode.js"' in lh and 'src="app.js"' not in lh and 'rf-dns-lite-svelte-log' in lh and 'mount(DnsLiteLog' in lm)
    gate('P9 legacy Lite log renderer delegates to Svelte', 'if(window.__rfDnsLiteSvelteLogLoaded)return;' in la and la.count('if(window.__rfDnsLiteSvelteLogLoaded)return;')==2)
    gate('P9 log display and bounded state implemented by Svelte', all(x in ls for x in ('let rows = $state(','.slice(-128)','let visible = $derived(',"get('service/traces')".replace("get(","readJSON("))))
    gate('P9 log updates real assigned and used DNS endpoints', all(x in ls for x in ('assigned_endpoint','endpoint(t,\'used\')','trace[field+\'_endpoint\']','Кэш · локально')))
    gate('P9 log retains filters pause refresh copy and cleanup', all(x in ls for x in ("'logFilter'","'logType'","'onlyErrors'","'logPause'","'copyLogs'","'refreshLogs'",'stopImmediatePropagation()','clearInterval(interval)','removeEventListener')))
    gate('P9 trace read-only fail-closed with no mutation', "method:'GET'" in ls and "readJSON('service/traces')" in ls and 'POST' not in ls and 'innerHTML' not in ls)
    gate('P9 runtime diagnostics owned by single Svelte root', 'rf-dns-runtime-svelte-diagnostics' in ra and '<DnsRuntimeDiagnostics' in ra and 'mount(DnsRuntimeApp' in rm)
    gate('P9 runtime diagnostics no mutation endpoints', all(x in rs for x in ("get('shadow/status')","get('runtime/resources')","get('keenetic/discovery')","method:'GET'")) and 'POST' not in rs and 'innerHTML' not in rs)
    gate('P9 runtime diagnostics cleanup', all(x in rs for x in ('Promise.allSettled','clearInterval(id)','stopped=true','sequence')))
    gate('P9 Lite unchanged; Runtime native single entry', all(x in la for x in ("on('gateEnable'","on('gateDisable'","on('commitDNS'")) and 'DnsP22Visual.svelte' in lm and 'DnsP25Settings.svelte' in lm and 'src="svelte-mode.js"' in rh and 'app.js' not in rh and not (R/'app.js').exists())
    gate('P9 both modes compile into the same shipped IPK JS', 'svelte-mode.js' in build and 'test -s "$UI_SOURCE/build/svelte-mode.js"' in build)
    failed=[name for name,ok in checks if not ok]
    print('DNS_P9_NATIVE_UI_GATES: '+('FAIL / '+','.join(failed) if failed else 'PASS'),flush=True)
    return bool(failed)
if __name__=='__main__':sys.exit(main())
