<script>
  let { messages = [], emptyText = 'Дополнительных предупреждений нет.', symbol = '⚠' } = $props();
</script>
{#if messages.length}
  {#each messages as message, index (index)}
    <div class="warning-item"><span>{symbol}</span><p>{String(message)}</p></div>
  {/each}
{:else}
  <div class="ok-note">{emptyText}</div>
{/if}
