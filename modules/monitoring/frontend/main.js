import '@fontsource/inter/400.css';
import '@fontsource/inter/500.css';
import '@fontsource/inter/600.css';
import '@fontsource/inter/700.css';
import '@fontsource/roboto-mono/400.css';
import '@fontsource/roboto-mono/500.css';
import '@fontsource/roboto-mono/600.css';
import '../../../components/core/frontend/src/app.css';
import '../../../components/core/frontend/src/admin-shell.css';
import '../../../components/core/frontend/src/routerforge-shell.css';
import './module.css';import { refreshCatalog, startCatalogPolling } from '$lib/stores/catalog.js';
import { mount } from 'svelte';
import MonitoringModuleApp from './MonitoringModuleApp.svelte';

async function boot() {
  await refreshCatalog().catch(() => null);
  startCatalogPolling(15000);
  mount(MonitoringModuleApp, { target: document.getElementById('app') });
}

boot();
