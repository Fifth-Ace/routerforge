<script>
  let { entries = [], emptyText = 'Набор ещё не загружен.', onUnban = () => {}, onExclude = () => {}, onDelete = () => {} } = $props();
</script>
{#if entries.length}
  {#each entries as entry, index (index)}
    <div class="entry-row" class:has-actions={entry.canUnban || entry.canExclude || entry.canDelete}>
      <span class="mono entry-value">{entry.value}</span>
      <span class="entry-meta-wrap"><span class="entry-meta">{entry.meta}</span><small>{entry.explanation}</small></span>
      {#if entry.canUnban || entry.canExclude || entry.canDelete}
        <span class="entry-actions">
          {#if entry.canUnban}<button class="button small danger" type="button" onclick={() => onUnban(entry.set, entry.value)}>Снять блокировку</button>{/if}
          {#if entry.canExclude}<button class="button small" type="button" onclick={() => onExclude(entry.value)}>В исключения</button>{/if}
          {#if entry.canDelete}<button class="button small danger" type="button" onclick={() => onDelete(entry.value)}>Удалить</button>{/if}
        </span>
      {/if}
    </div>
  {/each}
{:else}
  {emptyText}
{/if}
