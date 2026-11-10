import {mount, unmount} from 'svelte';
import DnsLiteController from './DnsLiteController.svelte';
// One Svelte root owns the DNS Lite UI. Portals move Svelte-owned views into
// the existing layout without creating independent roots or global bridges.
const app = mount(DnsLiteController,{target:document.body});
// Release the root and cancel its pending requests on real navigation.
// BFCache pagehide is not a teardown: the browser may restore that document.
function releaseOnNavigation(event) {
  if (event.persisted) return;
  window.removeEventListener('pagehide', releaseOnNavigation);
  void unmount(app);
}
window.addEventListener('pagehide', releaseOnNavigation);
