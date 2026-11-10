import {mount, unmount} from 'svelte';
import NFQWSManagerApp from './NFQWSManagerApp.svelte';

// One native Svelte root; legacy business actions run only after DOM mount.
const app = mount(NFQWSManagerApp, {target: document.body});
function teardown(event) {
  if (event.persisted) return; // BFCache restores retain the mounted DOM.
  window.removeEventListener('pagehide', teardown);
  void unmount(app);
}
window.addEventListener('pagehide', teardown);
