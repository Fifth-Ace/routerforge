<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  onMount(() => goto('/dns/tools', { replaceState: true }));
</script>
<svelte:head><title>RouterForge</title></svelte:head>
<div class="page"><section class="panel"><div class="empty">Перенаправляю…</div></section></div>
