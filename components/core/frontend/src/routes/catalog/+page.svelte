<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  onMount(() => {
    goto('/apps', { replaceState: true });
  });
</script>

<div class="page">
  <div class="catalog-empty">App Center…</div>
</div>
