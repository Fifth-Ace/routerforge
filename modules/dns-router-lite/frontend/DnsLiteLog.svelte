<script>
  // A view of the single DNS Lite controller snapshot. No independent polling,
  // global bridge, or capture-phase handlers for the surrounding toolbar.
  let {view = {}} = $props();
  import {visibleLiteLogs,qtypeName,liteLogEndpoint} from './DnsLiteLogModel.js';
  const qtype=qtypeName;
  let rows = $derived(Array.isArray(view.rows) ? view.rows : []);
  let upstreams = $derived(Array.isArray(view.upstreams) ? view.upstreams : []);
  let filter = $derived(String(view.filter || '').trim().toLowerCase());
  let type = $derived(String(view.type || ''));
  let errorsOnly = $derived(Boolean(view.errorsOnly));
  let visible = $derived(visibleLiteLogs(rows,{filter,type,errorsOnly},upstreams));
  const endpoint=(trace,field)=>liteLogEndpoint(trace,field,upstreams);
  const route = t => t.cache ? 'Кэш · локально' : t.used && t.used !== t.assigned ? `${endpoint(t,'assigned')} → ${endpoint(t,'used')}` : endpoint(t,'assigned');
  const response = t => t.rcode === 5 ? 'REFUSED' : t.rcode === 4 ? 'NOTIMP' : t.rcode === 3 ? 'NXDOMAIN' : t.success ? (t.rcode === 0 ? 'OK' : 'RCODE '+t.rcode) : t.rcode === -1 ? 'TIMEOUT / FAIL' : 'Ошибка (RCODE '+t.rcode+')';
  const device = t => t.device_mac ? `${t.device_name || t.device_mac} · ${t.client || ''}` : `Неизвестное устройство · ${t.client || '—'}`;
  function when(value) {const dt=new Date(value);return Number.isFinite(dt.getTime()) ? dt.toLocaleTimeString('ru-RU',{hour12:false}) : '—';}
</script>
{#if view.error}<p class="muted" role="status">Журнал: {view.error}</p>{/if}
{#if !visible.length}
  <p class="muted">Нет подходящих запросов. Попробуй изменить фильтр.</p>
{:else}
  {#each visible as t, index (index)}
    <div class:error={!t.success} class="log-row">
      <span>{when(t.time)}</span>
      <span title={[t.device_mac,t.identity_source,t.identity_reason,t.policy_layer,t.route_reason].filter(Boolean).join(' · ')}>{device(t)}</span>
      <span class="col-domain">{t.name || '—'}</span>
      <span>{qtype(t.type)}</span>
      <span class="col-route" title={t.used ? 'Фактический DNS: '+endpoint(t,'used') : ''}>{route(t)}</span>
      <span class="col-result">{response(t)}</span>
      <span>{String(t.latency_ms ?? 0)} мс</span>
    </div>
  {/each}
{/if}
