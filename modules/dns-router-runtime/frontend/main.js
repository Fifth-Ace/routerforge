import { mount, flushSync } from 'svelte';
import DnsRuntimeMode from './DnsRuntimeMode.svelte';
import DnsRuntimeDiagnostics from './DnsRuntimeDiagnostics.svelte';
import DnsRuntimeTables from './DnsRuntimeTables.svelte';
const target = document.getElementById('rf-dns-runtime-svelte-mode');
if (!target) throw Error('DNS_RUNTIME_SVELTE_MODE_MOUNT_MISSING');
target.replaceChildren();
window.__rfDNSRuntimeSvelteLoaded = true;
mount(DnsRuntimeMode, {target});
flushSync();
const diagnosticsTarget=document.getElementById('rf-dns-runtime-svelte-diagnostics');
if(!diagnosticsTarget)throw Error('DNS_RUNTIME_SVELTE_DIAGNOSTICS_MOUNT_MISSING');
diagnosticsTarget.replaceChildren();
mount(DnsRuntimeDiagnostics,{target:diagnosticsTarget});
flushSync();
for(const tableTarget of ['upstreamList','ruleList']){
  const element=document.getElementById(tableTarget);
  if(!element)throw Error('DNS_RUNTIME_SVELTE_TABLE_MOUNT_MISSING: '+tableTarget);
  element.replaceChildren();
  mount(DnsRuntimeTables,{target:element,props:{target:tableTarget}});
}
flushSync();
window.__rfDNSRuntimeSvelteTablesLoaded=true;
