import { mount, flushSync } from 'svelte';
import DnsRuntimeMode from './DnsRuntimeMode.svelte';
const target = document.getElementById('rf-dns-runtime-svelte-mode');
if (!target) throw Error('DNS_RUNTIME_SVELTE_MODE_MOUNT_MISSING');
target.replaceChildren();
window.__rfDNSRuntimeSvelteLoaded = true;
mount(DnsRuntimeMode, {target});
flushSync();
