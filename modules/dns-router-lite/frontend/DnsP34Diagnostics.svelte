<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import DnsPolicyAudit from './DnsPolicyAudit.svelte';

  let loading = $state(false);
  let error = $state('');
  let upstreams = $state([]);
  let traces = $state([]);
  let serviceActive = $state(false);
  let updated = $state('');
  let nameFilter = $state('');
  let clientFilter = $state('');
  let disposed = false;
  let current = null;

  let visibleTraces = $derived(traces.filter(row =>
    String(row.name||'').toLowerCase().includes(nameFilter.trim().toLowerCase()) &&
    [row.client,row.device_mac,row.device_name].some(value =>
      String(value||'').toLowerCase().includes(clientFilter.trim().toLowerCase()))
  ).slice(-30).reverse());

  const pct = (x,n) => n > 0 ? (100*x/n).toFixed(1)+'%' : '—';
  const ms = v => Number.isFinite(Number(v)) ? Number(v).toFixed(1)+' ms' : '—';
  function detail(row) {
    return [row.policy_layer,row.route_reason,row.rule,row.device_group].filter(Boolean).join(' · ') || rfUi('Общее правило',"Shared rule");
  }
  async function refresh() {
    if (loading) return;
    loading=true;error='';
    const ctl=new AbortController();current=ctl;
    try {
      const [historyResponse,traceResponse] = await Promise.all([
        fetch('../service/diagnostics',{method:'GET',cache:'no-store',signal:ctl.signal}),
        fetch('../service/traces',{method:'GET',cache:'no-store',signal:ctl.signal})
      ]);
      if (!historyResponse.ok || !traceResponse.ok) throw Error('HTTP diagnostics '+historyResponse.status+' / traces '+traceResponse.status);
      const [diagnostics, recent] = await Promise.all([historyResponse.json(),traceResponse.json()]);
      if (disposed || ctl.signal.aborted) return;
      upstreams = Array.isArray(diagnostics.upstreams) ? diagnostics.upstreams : [];
      serviceActive = Boolean(diagnostics.active);
      traces = Array.isArray(recent.traces) ? recent.traces.slice(-128) : [];
      updated=new Date().toLocaleTimeString('ru-RU');
    } catch (e) {
      if (!disposed && !ctl.signal.aborted) error=String(e.message||e);
    } finally {if (current===ctl)current=null;loading=false;}
  }
  onMount(() => {void refresh(); return () => {disposed=true;current?.abort();};});
</script>

<div class="p34-diagnostics" data-p34-native="true">
  <DnsPolicyAudit />
  <p class="p25-settings-info">{rfUi("История запросов хранится только в памяти до перезапуска DNS. Окна приблизительные: последние 4 или 96 пятнадцатиминутных интервалов. Альтернативный DNS не обязательно означает аварию — это может быть балансировка.","Query history is held in memory until DNS restarts. Time windows are approximate: the latest 4 or 96 fifteen-minute intervals. Using another DNS does not necessarily indicate an outage; it may be load balancing.")}</p>
  <div class="actions"><button class="button" type="button" disabled={loading} onclick={refresh}>{loading?'Обновляем…':'↻ Обновить диагностику'}</button><span role="status">{error||('Последнее обновление: '+(updated||'—'))}</span></div>
  {#if !serviceActive}<p class="notice">{rfUi("DNS Lite остановлен: показатели появятся после запуска сервиса и обработки запросов.","DNS Lite is stopped. Metrics will appear after the service starts and handles queries.")}</p>{/if}
  <div class="p34-comparison" style="overflow-x:auto">
    <table aria-label={rfUi("Сравнение DNS за 24 часа","24-hour DNS comparison")} style="width:100%;min-width:520px;text-align:left">
      <thead><tr><th>{rfUi("DNS / протокол","DNS / protocol")}</th><th>{rfUi("~1 ч · запросы","~1 h · queries")}</th><th>{rfUi("~1 ч · ошибки","~1 h · errors")}</th><th>{rfUi("~1 ч · RTT","~1 h · RTT")}</th><th>{rfUi("~24 ч · запросы","~24 h · queries")}</th><th>{rfUi("~24 ч · ошибки","~24 h · errors")}</th><th>{rfUi("Альтернативный выбор","Alternative selection")}</th></tr></thead>
      <tbody>
        {#each upstreams as row (row.id)}
          <tr><td><strong>{row.id}</strong><small> · {row.transport}</small></td><td>{row.last_four_buckets?.requests||0}</td><td>{pct(row.last_four_buckets?.errors||0,row.last_four_buckets?.requests||0)}</td><td>{ms(row.last_four_buckets?.avg_rtt_ms||0)}</td><td>{row.last_96_buckets?.requests||0}</td><td>{pct(row.last_96_buckets?.errors||0,row.last_96_buckets?.requests||0)}</td><td>{row.last_96_buckets?.alternate_choices||0}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if !upstreams.length}<p class="muted">{rfUi("Нет измерений для применённых DNS-серверов.","No measurements for active DNS servers.")}</p>{/if}
  <details><summary>{rfUi("История по интервалам (15 минут)","History by 15-minute intervals")}</summary>
    {#each upstreams as server (server.id)}
      <details><summary>{server.id} · {server.history?.length||0} {rfUi("интервалов","intervals")}</summary>
        <div style="overflow-x:auto"><table style="width:100%;text-align:left"><thead><tr><th>UTC</th><th>{rfUi("Запросы","Queries")}</th><th>{rfUi("Ошибки","Errors")}</th><th>RTT</th><th>{rfUi("Другой DNS","Other DNS")}</th></tr></thead><tbody>
          {#each server.history||[] as bucket (bucket.start)}
            <tr><td>{bucket.start}</td><td>{bucket.requests}</td><td>{bucket.errors}</td><td>{ms(bucket.avg_rtt_ms)}</td><td>{bucket.alternate_choices}</td></tr>
          {/each}
        </tbody></table></div>
      </details>
    {/each}
  </details>
  <h4>{rfUi("Последние решения маршрутизации","Recent routing decisions")}</h4>
  <p class="p25-settings-info">{rfUi("Только последние 128 событий сервиса. Фильтры работают локально; MAC отображается лишь при подтверждённом определении клиента. Для проверки правила без сетевого запроса используй «Объяснить маршрут» в разделе устройств.","Only the last 128 service events are shown. Filters run locally; MAC addresses appear only for verified clients. To inspect a rule without a network request, use Explain route in the Devices section.")}</p>
  <div class="fields two">
    <label class="field"><span>{rfUi("Домен","Domain")}</span><input type="text" maxlength="253" placeholder="example.org" bind:value={nameFilter}/></label>
    <label class="field"><span>{rfUi("Клиент / MAC","Client / MAC")}</span><input type="text" maxlength="80" placeholder="02:11:22:33:44:55" bind:value={clientFilter}/></label>
  </div>
  <div style="overflow-x:auto"><table style="width:100%;min-width:550px;text-align:left"><thead><tr><th>{rfUi("Время","Time")}</th><th>{rfUi("Домен","Domain")}</th><th>{rfUi("Клиент","Client")}</th><th>{rfUi("Назначен → использован","Assigned → used")}</th><th>{rfUi("Решение","Decision")}</th><th>{rfUi("Итог","Result")}</th></tr></thead><tbody>
    {#each visibleTraces as row, index (`${row.time}-${row.name}-${row.client}-${index}`)}
      <tr><td>{row.time ? new Date(row.time).toLocaleTimeString('ru-RU') : '—'}</td><td>{row.name}</td><td>{row.device_name||row.device_mac||row.client||'—'}</td><td>{row.assigned||'—'} → {row.used||'—'}</td><td>{detail(row)}</td><td>{row.success?'PASS':'FAIL'}{row.cache?' · cache':''}</td></tr>
    {/each}
  </tbody></table></div>
  {#if !visibleTraces.length}<p class="muted">{rfUi("Нет событий по фильтру.","No events match the filter.")}</p>{/if}
</div>
