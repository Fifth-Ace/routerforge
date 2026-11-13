<script>
  let { checks = [], emptyText = 'Проверки не вернули данных.' } = $props();
  const view = state => ({pass:['good','ГОТОВО'],warn:['warn','ВНИМАНИЕ'],fail:['bad','ОШИБКА'],info:['info','ИНФО']})[state] || ['neutral','НЕИЗВЕСТНО'];
</script>
{#if !checks.length}
  <div class="empty-state">{emptyText}</div>
{:else}
  {#each checks as check, index (index)}
    <article class="diagnostic-row">
      <div class="diagnostic-main">
        <div class="diagnostic-title"><strong>{check.label || check.id || 'Проверка'}</strong><span class="state {view(check.state)[0]}">{view(check.state)[1]}</span></div>
        <p>{check.summary || '—'}</p>
        {#each check.details || [] as detail, i (i)}<small class="mono">{detail}</small>{/each}
      </div>
    </article>
  {/each}
{/if}
