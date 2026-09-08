<script>
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import { page } from '$app/stores';
  $: view = $page.url.searchParams.get('tab') || 'system';
</script>
<svelte:head><title>RouterForge — Monitoring</title></svelte:head>
<ModuleFrame moduleId="monitoring" {view} />