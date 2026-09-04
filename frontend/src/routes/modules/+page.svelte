<script>
  import { page } from '$app/stores';
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  $: moduleId = $page.url.searchParams.get('module') || '';
  $: view = $page.url.searchParams.get('view') || 'overview';
</script>
<svelte:head><title>RouterForge — Module</title></svelte:head>
{#if moduleId}
  <ModuleFrame {moduleId} {view} />
{:else}
  <div class="page"><section class="panel"><div class="empty">Module id is required.</div></section></div>
{/if}
