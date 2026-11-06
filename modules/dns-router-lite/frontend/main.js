import { mount, flushSync } from 'svelte';
import DnsModeStatus from './DnsModeStatus.svelte';
const target = document.getElementById('rf-dns-lite-svelte-mode');
if (!target) throw Error('DNS_LITE_SVELTE_MODE_MOUNT_MISSING');
target.replaceChildren();
window.__rfDnsLiteSvelteLoaded = true;
mount(DnsModeStatus, {target});
flushSync();
