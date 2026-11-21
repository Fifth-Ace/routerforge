<script>
  import { rfUi } from './rf-localization.js';
  let {cards = [], selected = '', health = {}, onAction = () => {}} = $props();
  let warning = $state('');
  const detail = id => health[id] || null;
  const stateText = h => !h ? rfUi('Не проверялся','Not checked') : ({UP:rfUi('Доступен','Available'),DOWN:rfUi('Недоступен','Unavailable'),STALE:rfUi('Данные устарели','Data outdated'),UNKNOWN:rfUi('Не проверялся','Not checked'),UNSUPPORTED:'Keenetic'}[h.status] || rfUi('Не проверялся','Not checked'));
  const route = item => !item.domains.length ? rfUi('Без доменов','No domains') : rfUi('Привязка: ','Bound to: ')+item.domains.slice(0,2).join(' · ')+(item.domains.length>2?' · +'+(item.domains.length-2):'');
  function action(name, id, value) {
    warning = '';
    try { onAction(name,id,value); } catch(e) { warning = String(e.message || e); }
  }
</script>

{#if warning}
  <p class="notice" role="alert">{warning}</p>
{/if}
{#if cards.length === 0}
  <p class="muted">{rfUi("Добавь первый DNS-сервер. Здесь появятся его списки доменов и состояние сертификата.","Add your first DNS server. Its domain lists and certificate status will appear here.")}</p>
{/if}
{#each cards as card (card.id)}
  {@const h = detail(card.id)}
  <article class="dns-card" class:is-selected={selected === card.id} data-upstream-id={card.id}>
    <div class="dns-top">
      <div class="dns-title"><strong>{card.title}</strong><small>{card.address}{card.path ? ' · '+card.path : ''}</small></div>
      <span class="dns-count" title={rfUi(`${card.listCount} списков, ${card.domainCount} доменов`,`${card.listCount} lists, ${card.domainCount} domains`)}>{card.listCount} {rfUi("сп. ·","lists ·")} {card.domainCount} {rfUi("дом.","domains")}</span>
    </div>
    <div class="dns-health">
      <span class="dns-health-status" data-state={(h?.status || '').toLowerCase()} title={h?.last_error || ''}>{stateText(h)}</span>
      <span class="dns-rtt">{h?.status === 'UP' ? 'DNS RTT: '+h.probe_latency_ms+rfUi(' мс',' ms') : 'DNS RTT: —'}</span>
      <span class="dns-p95">{h?.requests_5m > h?.failures_5m ? 'p95: '+h.p95_latency_5m_ms+rfUi(' мс',' ms') : 'p95: —'}</span>
    </div>
    <div class="dns-card-metrics"><span>{rfUi("Списки","Lists")} {card.listCount}</span><span>{rfUi("Домены","Domains")} {card.domainCount}</span></div>
    <p class="dns-preview">{route(card)}</p>
    <div class="dns-card-actions">
      <button type="button" class="button dns-edit" class:primary={selected === card.id} disabled={card.native} onclick={() => action('edit',card.id)}>{rfUi("Списки","Lists")}</button>
      {#if !card.native}
        <button type="button" class="button dns-change" title={rfUi("Изменить DNS / обновить TLS","Edit DNS / refresh TLS")} onclick={() => action('change',card.id)}>{rfUi("Изменить","Edit")}</button>
      {/if}
      <button type="button" class="button dns-check" disabled={card.native} onclick={() => action('check',card.id)}>{rfUi("Проверить","Check")}</button>
      <details class="dns-advanced">
        <summary>{rfUi("Ещё","More")}</summary>
        <div class="dns-bottom">
          <label class="field"><span>{rfUi("Резервный DNS","Backup DNS")}</span>
            <select class="dns-select" disabled={card.native} value={card.fallback} onchange={(e) => action('fallback',card.id,e.currentTarget.value)}>
              <option value="">{rfUi("Без резерва","No backup")}</option>
              {#each card.fallbackOptions as option (option.id)}
                <option value={option.id}>{option.label}</option>
              {/each}
            </select>
          </label>
          <button type="button" class="button danger" onclick={() => action('remove',card.id)}>{rfUi("Удалить DNS","Delete DNS")}</button>
        </div>
      </details>
    </div>
  </article>
{/each}
