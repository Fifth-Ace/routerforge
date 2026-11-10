<script>
  let {cards = [], selected = '', health = {}, onAction = () => {}} = $props();
  let warning = $state('');
  const detail = id => health[id] || null;
  const stateText = h => !h ? 'Не проверялся' : ({UP:'Доступен',DOWN:'Недоступен',STALE:'Данные устарели',UNKNOWN:'Не проверялся',UNSUPPORTED:'Keenetic'}[h.status] || 'Не проверялся');
  const route = item => !item.domains.length ? 'Без доменов' : 'Привязка: '+item.domains.slice(0,2).join(' · ')+(item.domains.length>2?' · +'+(item.domains.length-2):'');
  function action(name, id, value) {
    warning = '';
    try { onAction(name,id,value); } catch(e) { warning = String(e.message || e); }
  }
</script>

{#if warning}
  <p class="notice" role="alert">{warning}</p>
{/if}
{#if cards.length === 0}
  <p class="muted">Добавь первый DNS-сервер. Здесь появятся его списки доменов и состояние сертификата.</p>
{/if}
{#each cards as card (card.id)}
  {@const h = detail(card.id)}
  <article class="dns-card" class:is-selected={selected === card.id} data-upstream-id={card.id}>
    <div class="dns-top">
      <div class="dns-title"><strong>{card.title}</strong><small>{card.address}{card.path ? ' · '+card.path : ''}</small></div>
      <span class="dns-count" title={`${card.listCount} списков, ${card.domainCount} доменов`}>{card.listCount} сп. · {card.domainCount} дом.</span>
    </div>
    <div class="dns-health">
      <span class="dns-health-status" data-state={(h?.status || '').toLowerCase()} title={h?.last_error || ''}>{stateText(h)}</span>
      <span class="dns-rtt">{h?.status === 'UP' ? 'DNS RTT: '+h.probe_latency_ms+' мс' : 'DNS RTT: —'}</span>
      <span class="dns-p95">{h?.requests_5m > h?.failures_5m ? 'p95: '+h.p95_latency_5m_ms+' мс' : 'p95: —'}</span>
    </div>
    <div class="dns-card-metrics"><span>Списки {card.listCount}</span><span>Домены {card.domainCount}</span></div>
    <p class="dns-preview">{route(card)}</p>
    <div class="dns-card-actions">
      <button type="button" class="button dns-edit" class:primary={selected === card.id} disabled={card.native} onclick={() => action('edit',card.id)}>Списки</button>
      {#if !card.native}
        <button type="button" class="button dns-change" title="Изменить DNS / обновить TLS" onclick={() => action('change',card.id)}>Изменить</button>
      {/if}
      <button type="button" class="button dns-check" disabled={card.native} onclick={() => action('check',card.id)}>Проверить</button>
      <details class="dns-advanced">
        <summary>Ещё</summary>
        <div class="dns-bottom">
          <label class="field"><span>Резервный DNS</span>
            <select class="dns-select" disabled={card.native} value={card.fallback} onchange={(e) => action('fallback',card.id,e.currentTarget.value)}>
              <option value="">Без резерва</option>
              {#each card.fallbackOptions as option (option.id)}
                <option value={option.id}>{option.label}</option>
              {/each}
            </select>
          </label>
          <button type="button" class="button danger" onclick={() => action('remove',card.id)}>Удалить DNS</button>
        </div>
      </details>
    </div>
  </article>
{/each}
