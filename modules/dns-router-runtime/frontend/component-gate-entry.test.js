// Test-only Svelte mount/unmount harness. Not imported by production main.js,
// not shipped by the IPK frontend allowlist and never enabled on the router.
import {mount as mountSvelte,unmount as unmountSvelte,tick} from 'svelte';
import DnsRuntimeApp from './DnsRuntimeApp.svelte';
import {draft,simple} from './DnsRuntimeStore.svelte.js';
const target=document.getElementById('app');
if(!target)throw Error('RUNTIME_COMPONENT_TEST_TARGET_MISSING');
let instance=null;
function snapshot(){return {
  mounted:instance!==null,domChildren:target.childElementCount,
  revision:draft.revision,loaded:draft.loaded,dirty:draft.dirty,
  generation:draft.generation,domains:simple.domains
};}
window.__rfRuntimeComponentTest={
  async mount(){
    if(instance)throw Error('RUNTIME_COMPONENT_ALREADY_MOUNTED');
    instance=mountSvelte(DnsRuntimeApp,{target});
    await tick();
    return snapshot();
  },
  async unmount(){
    if(!instance)throw Error('RUNTIME_COMPONENT_NOT_MOUNTED');
    await unmountSvelte(instance);
    instance=null;
    await tick();
    return snapshot();
  },
  snapshot
};
