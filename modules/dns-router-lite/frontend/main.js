import { mount, flushSync } from 'svelte';
import DnsModeStatus from './DnsModeStatus.svelte';
import DnsLiteLog from './DnsLiteLog.svelte';
import DnsLiteCards from './DnsLiteCards.svelte';
import DnsLiteDomainEditor from './DnsLiteDomainEditor.svelte';
import DnsLiteController from './DnsLiteController.svelte';
import DnsP22Visual from './DnsP22Visual.svelte';
import DnsP25Settings from './DnsP25Settings.svelte';
const target = document.getElementById('rf-dns-lite-svelte-mode');
if (!target) throw Error('DNS_LITE_SVELTE_MODE_MOUNT_MISSING');
target.replaceChildren();
window.__rfDnsLiteSvelteLoaded = true;
mount(DnsModeStatus, {target});
flushSync();
const logTarget=document.getElementById('rf-dns-lite-svelte-log');
if(!logTarget)throw Error('DNS_LITE_SVELTE_LOG_MOUNT_MISSING');
logTarget.replaceChildren();
window.__rfDnsLiteSvelteLogLoaded=true;
mount(DnsLiteLog,{target:logTarget});
flushSync();
const cardsTarget=document.getElementById('dnsCards');
if(!cardsTarget)throw Error('DNS_LITE_SVELTE_CARDS_MOUNT_MISSING');
cardsTarget.replaceChildren();
window.__rfDnsLiteCardsLoaded=true;
mount(DnsLiteCards,{target:cardsTarget});
flushSync();
const editorTarget=document.getElementById('rf-dns-lite-svelte-editor');
if(!editorTarget)throw Error('DNS_LITE_SVELTE_DOMAIN_EDITOR_MOUNT_MISSING');
mount(DnsLiteDomainEditor,{target:editorTarget});
flushSync();

// Preserve original startup ordering after native visual components mount.
// The legacy standalone JavaScript files are no longer loaded or shipped.
for (const Controller of [DnsLiteController,DnsP22Visual,DnsP25Settings]) {
  mount(Controller,{target:document.body});
  flushSync();
}
