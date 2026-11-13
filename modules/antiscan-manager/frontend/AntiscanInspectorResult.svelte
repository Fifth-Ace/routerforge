<script>
  let { view = null, emptyText = 'Введите IPv4-адрес. Эта проверка ничего не меняет в Antiscan.', onExclude = () => {} } = $props();
</script>
{#if view}
  <div class="inspect-verdict">
    <div><span class="mono">{view.ip}</span><strong>{view.title}</strong></div>
    <span class="state {view.stateClass}">{view.reason}</span>
  </div>
  {#if view.explanation}<div class="reason-explanation">{view.explanation}</div>{/if}
  {#if view.evidence.length}
    {#each view.evidence as item, index (index)}
      <div class="evidence-row"><div><strong>{item.reason}</strong><span class="mono">{item.set}</span></div><p>{item.summary}</p></div>
    {/each}
  {:else}
    <div class="muted inspect-empty">Совпадения в доступных наборах Antiscan не найдены.</div>
  {/if}
  {#each view.warnings as item, index (index)}<div class="inspect-warning">{item}</div>{/each}
  {#if view.canExclude}
    <div class="inspect-actions"><button class="button small" type="button" onclick={() => onExclude(view.ip)}>Добавить IP в исключения</button></div>
  {/if}
{:else}
  {emptyText}
{/if}
