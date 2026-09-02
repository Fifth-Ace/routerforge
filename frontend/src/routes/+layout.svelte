<script>
  import '@fontsource/inter/400.css';
  import '@fontsource/inter/500.css';
  import '@fontsource/inter/600.css';
  import '@fontsource/inter/700.css';
  import '@fontsource/roboto-mono/400.css';
  import '@fontsource/roboto-mono/500.css';
  import '@fontsource/roboto-mono/600.css';
  import '../app.css';

  import { onMount } from 'svelte';
  import AppHeader from '$lib/components/AppHeader.svelte';
  import { snapshot, startSnapshotStream } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';

  let stopStream = null;
  let lastInterval = 0;

  onMount(() => {
    const unsubscribe = settings.subscribe((value) => {
      const interval = Number(value.refreshMs || 2000);
      if (interval === lastInterval && stopStream) return;
      lastInterval = interval;
      stopStream?.();
      stopStream = startSnapshotStream(interval);
    });
    return () => {
      unsubscribe();
      stopStream?.();
    };
  });
</script>

<AppHeader />

{#if $snapshot.discovery_error || $snapshot.capture_error || $snapshot.client_registry_error || $snapshot.client_capture_error}
  <div class="global-alerts">
    {#if $snapshot.discovery_error}<div class="global-alert">Discovery: {$snapshot.discovery_error}</div>{/if}
    {#if $snapshot.capture_error}<div class="global-alert">Capture: {$snapshot.capture_error}</div>{/if}
    {#if $snapshot.client_registry_error}<div class="global-alert">Clients: {$snapshot.client_registry_error}</div>{/if}
    {#if $snapshot.client_capture_error}<div class="global-alert">Client capture: {$snapshot.client_capture_error}</div>{/if}
  </div>
{/if}

<main class="app-main"><slot /></main>
