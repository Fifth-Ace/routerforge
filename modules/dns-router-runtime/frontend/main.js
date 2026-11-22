import {mount} from 'svelte';
import DnsRuntimeApp from './DnsRuntimeApp.svelte';
const target=document.getElementById('app');
if(!target)throw Error('DNS_RUNTIME_APP_MOUNT_MISSING');
mount(DnsRuntimeApp,{target});
