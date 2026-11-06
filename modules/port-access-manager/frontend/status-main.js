import { mount } from 'svelte';
import PortAccessStatus from './PortAccessStatus.svelte';
import PortAccessOperationLog from './PortAccessOperationLog.svelte';
const target = document.getElementById('pa-svelte-status');
if (!target) throw new Error('PORT_ACCESS_SVELTE_MOUNT_MISSING');
mount(PortAccessStatus, { target });
const logTarget = document.getElementById('pa-svelte-log');
if (!logTarget) throw new Error('PORT_ACCESS_SVELTE_LOG_MOUNT_MISSING');
mount(PortAccessOperationLog, { target: logTarget });
