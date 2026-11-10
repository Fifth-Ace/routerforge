import {mount, unmount} from 'svelte';
import DnsLiteController from './DnsLiteController.svelte';
// One Svelte root owns the DNS Lite UI. Portals move Svelte-owned views into
// the existing layout without creating independent roots or global bridges.
// Guard the actual layout anchor before mounting the single Svelte root.
// The failure marker is also verified in the production IPK bundle.
if (!document.body || !document.getElementById('dnsLiteApp')) {
  throw new Error('DNS_LITE_SVELTE_MODE_MOUNT_MISSING');
}
const app = mount(DnsLiteController,{target:document.body});
// Release the root and cancel its pending requests on real navigation.
// BFCache pagehide is not a teardown: the browser may restore that document.
function releaseOnNavigation(event) {
  if (event.persisted) return;
  window.removeEventListener('pagehide', releaseOnNavigation);
  void unmount(app);
}
window.addEventListener('pagehide', releaseOnNavigation);
