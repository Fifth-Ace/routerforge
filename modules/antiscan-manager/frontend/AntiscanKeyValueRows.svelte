<script>
  // Read-only Svelte owner: text nodes, never API-provided HTML.
  let { rows = [] } = $props();
</script>
{#each rows as row, index (index)}
  <div class="kv"><span>{row[0]}</span><strong>{String(row[1] ?? '—')}</strong></div>
{/each}
