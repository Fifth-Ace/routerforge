import { mount } from 'svelte';
import PortAccessStatus from './PortAccessStatus.svelte';
const target = document.getElementById('pa-svelte-status');
if (!target) throw new Error('PORT_ACCESS_SVELTE_MOUNT_MISSING');
mount(PortAccessStatus, { target });
