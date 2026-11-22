import { mount } from 'svelte';
import PortAccessLayout from './PortAccessLayout.svelte';

// One root, one lifecycle: no moving mounted DOM or global action bridge.
const target = document.getElementById('pa-svelte-layout');
if (!target) throw new Error('PORT_ACCESS_LAYOUT_TARGET_MISSING');
mount(PortAccessLayout, { target });
