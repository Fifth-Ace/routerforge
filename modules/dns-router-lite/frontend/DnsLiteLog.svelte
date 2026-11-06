<script>
  import { onMount } from 'svelte';
  let rows = $state([]);
  let upstreams = $state([]);
  let paused = $state(false);
  let busy = $state(false);
  let failure = $state('');
  let lastUpdated = $state('');
  let filter = $state('');
  let type = $state('');
  let errorsOnly = $state(false);
  let stopped = false;
  let serial = 0;
  const types = {1:'A',28:'AAAA',15:'MX',16:'TXT',5:'CNAME',12:'PTR',33:'SRV',65:'HTTPS',64:'SVCB',255:'ANY'};
  const qtype = value => typeof value === 'string' ? value : (types[value] || 'TYPE '+String(value ?? '—'));
  const elements = id => document.getElementById(id);
  const when = value => {
    const dt = new Date(value);
    return Number.isFinite(dt.getTime()) ? dt.toLocaleTimeString('ru-RU', {hour12:false}) : '—';
  };
  function endpoint(trace, field) {
    if (trace.cache) return 'Кэш · локально';
    const direct = trace[field+'_endpoint'];
    if (direct) return String(direct);
    const id = trace[field];
    if (id === 'system-local') return 'Локальный DNS Keenetic';
    if (id === 'client-acl' || id === 'refused') return 'Отказ ACL';
    const u = upstreams.find(x => x.id === id);
    return u ? `${u.server_name ? u.server_name+' ('+u.address+')' : u.address} · ${u.transport} [${u.id}]` : String(id || '—');
  }
  const route = t => {
    if (t.cache) return 'Кэш · локально';
    const assigned = endpoint(t,'assigned');
    return t.used && t.used !== t.assigned ? `${assigned} → ${endpoint(t,'used')}` : assigned;
  };
  const response = t => t.rcode === 5 ? 'REFUSED' : t.rcode === 4 ? 'NOTIMP' : t.rcode === 3 ? 'NXDOMAIN' : t.success ? (t.rcode === 0 ? 'OK' : 'RCODE '+t.rcode) : t.rcode === -1 ? 'TIMEOUT / FAIL' : 'Ошибка (RCODE '+t.rcode+')';
  const device = t => t.device_mac ? `${t.device_name || t.device_mac} · ${t.client || ''}` : `Неизвестное устройство · ${t.client || '—'}`;
  let visible = $derived(rows.slice().reverse().filter(t => {
    if (type && qtype(t.type) !== type) return false;
    if (errorsOnly && t.success) return false;
    if (!filter.trim()) return true;
    const s = filter.trim().toLowerCase();
    return [t.name,t.client,t.device_name,t.device_mac,t.assigned,t.used,t.assigned_endpoint,t.used_endpoint,endpoint(t,'assigned'),endpoint(t,'used')].some(x=>String(x||'').toLowerCase().includes(s));
  }));
  function updateLegacyCaptions() {
    const set = (id, value) => {const node=elements(id); if(node)node.textContent=String(value);};
    set('logCounter',rows.length);
    set('logVisible',`${visible.length} / ${rows.length}`);
    set('logPause',paused?'▶ Продолжить':'⏸ Пауза');
    set('logUpdated',lastUpdated ? 'Обновлено '+lastUpdated : 'Ожидание');
    set('liveBadge',paused ? 'ПАУЗА · снимок журнала' : failure ? 'LIVE · недоступен' : 'LIVE · запросы');
  }
  async function readJSON(path) {
    const response=await fetch('../'+path,{method:'GET',cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json'}});
    if (!response.ok) throw Error('HTTP '+response.status);
    if (!(response.headers.get('content-type')||'').toLowerCase().includes('json'))throw Error('Ожидался JSON');
    return response.json();
  }
  async function refresh() {
    if (stopped || paused || busy) return;
    const token=++serial;
    busy=true;
    try {
      const data=await readJSON('service/traces');
      if (stopped || serial!==token)return;
      if (!Array.isArray(data.traces))throw Error('Неверный формат DNS-журнала');
      rows=data.traces.slice(-128).filter(t=>t && typeof t==='object' && !Array.isArray(t));
      failure='';
      lastUpdated=new Date().toLocaleTimeString('ru-RU',{hour12:false});
    } catch(err) {if (!stopped && serial===token)failure=String(err.message||err);} finally {if (!stopped && serial===token){busy=false;updateLegacyCaptions();}}
  }
  function syncFilters() {
    filter=elements('logFilter')?.value || '';
    type=elements('logType')?.value || '';
    errorsOnly=Boolean(elements('onlyErrors')?.checked);
    updateLegacyCaptions();
  }
  function capturePause(event) {
    event.preventDefault(); event.stopImmediatePropagation();
    paused=!paused;
    if (!paused) void refresh();
    updateLegacyCaptions();
  }
  function captureRefresh(event) { event.preventDefault(); event.stopImmediatePropagation(); void refresh(); }
  async function captureCopy(event) {
    event.preventDefault(); event.stopImmediatePropagation();
    const data=visible.map(t=>[t.time,t.client,t.name,qtype(t.type),endpoint(t,'assigned'),endpoint(t,'used'),t.transport,t.rcode,t.success?'PASS':'FAIL',t.latency_ms].join('\t')).join('\n');
    try {if (!navigator.clipboard?.writeText)throw Error('Буфер обмена недоступен');await navigator.clipboard.writeText(data);failure='';}
    catch(e){failure=String(e.message||e);}
  }
  onMount(()=>{
    const bindings=[
      ['logFilter','input',syncFilters,false],['logType','change',syncFilters,false],
      ['onlyErrors','change',syncFilters,false],['logPause','click',capturePause,true],
      ['refreshLogs','click',captureRefresh,true],['copyLogs','click',captureCopy,true]
    ];
    for(const [id,type,fn,capture] of bindings)elements(id)?.addEventListener(type,fn,capture);
    syncFilters();
    void readJSON('config').then(d=>{if(!stopped && Array.isArray(d?.config?.upstreams))upstreams=d.config.upstreams;}).catch(()=>{});
    void refresh();
    const interval=window.setInterval(()=>{if(!document.hidden && !elements('pageLogs')?.hidden)void refresh();},2000);
    return ()=>{stopped=true;++serial;window.clearInterval(interval);for(const [id,type,fn,capture] of bindings)elements(id)?.removeEventListener(type,fn,capture);};
  });
</script>
{#if failure}<p class="muted" role="status">Журнал: {failure}</p>{/if}
{#if !visible.length}
  <p class="muted">{busy ? 'Читаем DNS-запросы…' : 'Нет подходящих запросов. Попробуй изменить фильтр.'}</p>
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
