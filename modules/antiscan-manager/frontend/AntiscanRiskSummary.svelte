<script>
  let { view = {detected:false} } = $props();
</script>
{#if !view.detected}
  <div class="risk-empty">Antiscan не обнаружен — анализировать нечего.</div>
{:else}
  <div class="risk-verdict" class:warn={view.risk} class:good={!view.risk}>
    <span>{view.risk ? '!' : '✓'}</span>
    <div>
      <strong>{view.risk ? 'Есть риск ложной /24-блокировки' : 'Явного риска ложной /24-блокировки не видно'}</strong>
      <p>{view.risk ? `Порог ${view.threshold} адресов сочетается с хранением кандидатов ${view.storage}. Для мобильных пулов это может быть слишком агрессивно.` : 'Текущее сочетание порога и времени хранения кандидатов не попало под встроенный профиль повышенного риска.'}</p>
    </div>
  </div>
  <div class="risk-pairs">
    <div><span>Порог /24</span><strong>{view.threshold || '—'}</strong></div>
    <div><span>Кандидаты хранятся</span><strong>{view.storage}</strong></div>
    <div><span>Блокировка подсети</span><strong>{view.subnetBan}</strong></div>
    <div><span>Порог новых подключений</span><strong>{view.recentHitcount || '—'}</strong></div>
  </div>
{/if}
