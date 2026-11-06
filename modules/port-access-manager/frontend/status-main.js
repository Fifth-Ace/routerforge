import { mount, flushSync } from 'svelte';
import PortAccessEngineWorkspace from './PortAccessEngineWorkspace.svelte';
import PortAccessEngineering from './PortAccessEngineering.svelte';
import PortAccessStatus from './PortAccessStatus.svelte';
import PortAccessOdin from './PortAccessOdin.svelte';
import PortAccessOperationLog from './PortAccessOperationLog.svelte';
const target = document.getElementById('pa-svelte-status');
if (!target) throw new Error('PORT_ACCESS_SVELTE_MOUNT_MISSING');
mount(PortAccessStatus, { target });
const logTarget = document.getElementById('pa-svelte-log');
if (!logTarget) throw new Error('PORT_ACCESS_SVELTE_LOG_MOUNT_MISSING');
mount(PortAccessOperationLog, { target: logTarget });
const engineTarget = document.getElementById('pa-svelte-engine-workspace');
if (!engineTarget) throw new Error('PORT_ACCESS_ENGINE_WORKSPACE_MOUNT_MISSING');
mount(PortAccessEngineWorkspace, { target: engineTarget });
const odinTarget = document.getElementById('pa-svelte-odin');
if (!odinTarget) throw new Error('PORT_ACCESS_ODIN_MOUNT_MISSING');
mount(PortAccessOdin, { target: odinTarget });
const engineeringTarget = document.getElementById('pa-svelte-engineering');
if (!engineeringTarget) throw new Error('PORT_ACCESS_ENGINEERING_MOUNT_MISSING');
mount(PortAccessEngineering, { target: engineeringTarget });
// The retained layout controller follows this bundle in document order.
// Ensure all Svelte-owned sections exist before it rearranges them.
flushSync();
