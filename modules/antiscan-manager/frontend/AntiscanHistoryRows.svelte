<script>
  let { entries = [], emptyText = 'Откройте вкладку, чтобы загрузить последние события.' } = $props();
</script>
{#if !entries.length}
  <div class="empty-state">{emptyText}</div>
{:else}
  {#each entries as entry, index (index)}
    <article class="history-row" class:failed={entry.failed}>
      <div class="history-main">
        <div class="history-title">
          <span class="state" class:bad={entry.failed} class:good={!entry.failed}>{entry.failed ? 'ОШИБКА' : 'УСПЕХ'}</span>
          <strong>{entry.title}</strong><span class="mono muted">{entry.time}</span>
        </div>
        <p>{entry.summary}</p>
        {#if entry.target}<span class="mono history-target">{entry.target}</span>{/if}
        {#each entry.warnings as warning, i (i)}<small>⚠ {warning}</small>{/each}
      </div>
      <div class="history-side">
        <span class="mono">{entry.duration} мс</span><span class="mono">HTTP {entry.httpStatus}</span>
        <div class="history-flags">
          {#if entry.changed}<span class="history-flag">изменено</span>{/if}
          {#if entry.verified}<span class="history-flag good">проверено</span>{/if}
          {#if entry.rollback}<span class="history-flag bad">выполнен откат</span>{/if}
          {#if entry.restartRequired}<span class="history-flag warn">нужен перезапуск</span>{/if}
        </div>
      </div>
    </article>
  {/each}
{/if}
