<script>
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { getHistory, getClients, getInterfaces, getClient } from '$lib/api.js';
  import { fmtInt, fmtAgo, timeOnly, groupBy } from '$lib/utils.js';
  import HistoryChart from '$lib/components/HistoryChart.svelte';

  let tab='traffic', historyMinutes=5, history={points:[]}, clients=[], interfaces=[], selectedIP='', clientDetail=null;
  let flowSearch='', flowProfile='all', fallbackOnly=false, flowPaused=false, frozenFlow=[];
  let clientSearch='', clientPaused=false, frozenClientEvents=[], clientFlowSearch='', clientOutcome='all';
  let loading=false;

  $: snapshotFlow=$snapshot.flow||[];
  $: profiles=[...new Set(($snapshot.upstreams||[]).map((u)=>u.profile).filter(Boolean))];
  $: flowSource=flowPaused?frozenFlow:snapshotFlow;
  $: flowRows=flowSource.slice().reverse()
    .filter((x)=>!flowSearch.trim()||`${x.domain||''} ${x.client_name||''} ${x.client_hostname||''} ${x.client_ip||''}`.toLowerCase().includes(flowSearch.trim().toLowerCase()))
    .filter((x)=>flowProfile==='all'||x.profile===flowProfile)
    .filter((x)=>!fallbackOnly||x.fallback)
    .slice(0,500);

  $: filteredClients=clients.filter((c)=>{
    const q=clientSearch.trim().toLowerCase();
    return !q||`${c.name||''} ${c.hostname||''} ${c.ip||''} ${c.mac||''} ${c.policy||'System'} ${c.access||''} ${c.ssid||''} ${c.ap||''}`.toLowerCase().includes(q);
  }).sort((a,b)=>{
    if(Boolean(a.active)!==Boolean(b.active))return a.active?-1:1;
    const an=String(a.name||a.hostname||a.ip||'').toLocaleLowerCase();
    const bn=String(b.name||b.hostname||b.ip||'').toLocaleLowerCase();
    return an.localeCompare(bn,'ru',{numeric:true,sensitivity:'base'})||String(a.ip||'').localeCompare(String(b.ip||''),'en',{numeric:true});
  });

  $: clientEventSource=clientPaused?frozenClientEvents:(clientDetail?.events||[]);
  $: clientEvents=clientEventSource
    .filter((e)=>!clientFlowSearch.trim()||`${e.domain||''} ${e.resolver||''} ${e.rcode||''}`.toLowerCase().includes(clientFlowSearch.trim().toLowerCase()))
    .filter((e)=>clientOutcome==='all'||e.outcome===clientOutcome);

  $: topDomains=$snapshot.top_domains||[];
  $: domainMax=topDomains[0]?.count||1;
  $: qtypeGroups=Object.entries(groupBy($snapshot.flow||[],'qtype')).sort((a,b)=>b[1].length-a[1].length);

  async function loadHistory(){loading=true;try{history=await getHistory(historyMinutes);}catch{history={points:[]};}finally{loading=false;}}
  async function loadClients(){try{clients=(await getClients()).clients||[];}catch{}}
  async function loadInterfaces(){try{interfaces=(await getInterfaces()).interfaces||[];}catch{}}
  async function loadSelectedClient(){if(!selectedIP||clientPaused)return;try{clientDetail=await getClient(selectedIP);}catch{}}

  function setTab(value){
    tab=value;
    if(value==='traffic')loadHistory();
    if(value==='devices')selectedIP?loadSelectedClient():loadClients();
    if(value==='interfaces')loadInterfaces();
  }
  function selectHistory(value){historyMinutes=value;loadHistory();}
  function toggleFlowPause(){if(!flowPaused){frozenFlow=snapshotFlow.slice();flowPaused=true;}else{flowPaused=false;frozenFlow=[];}}
  function toggleClientPause(){if(!clientPaused){frozenClientEvents=(clientDetail?.events||[]).slice();clientPaused=true;}else{clientPaused=false;frozenClientEvents=[];loadSelectedClient();}}
  function openClient(ip){selectedIP=ip;clientPaused=false;frozenClientEvents=[];clientDetail=null;loadSelectedClient();}
  function closeClient(){selectedIP='';clientPaused=false;frozenClientEvents=[];clientDetail=null;loadClients();}
  function autoPauseFlow(event){if(event.currentTarget.scrollTop>=4&&!flowPaused){frozenFlow=snapshotFlow.slice();flowPaused=true;}}
  function autoPauseClient(event){if(event.currentTarget.scrollTop>=4&&!clientPaused){frozenClientEvents=(clientDetail?.events||[]).slice();clientPaused=true;}}
  function outcomeClass(value){return value==='FORWARDED'?'good':value==='ERROR'||value==='CLIENT_TIMEOUT'?'error':'neutral';}

  onMount(()=>{
    loadHistory();
    const timer=setInterval(()=>{
      if(tab==='devices'){selectedIP?loadSelectedClient():loadClients();}
      else if(tab==='interfaces')loadInterfaces();
    },3000);
    return()=>clearInterval(timer);
  });
</script>

<svelte:head><title>RouterForge — Мониторинг</title></svelte:head>

<div class="page">
  <div class="page-head"><div><h1>Мониторинг</h1><p>DNS flow, клиенты, интерфейсы, домены и временная история.</p></div><span class="page-kicker mono">LIVE / {$snapshot.flow?.length||0} EVENTS</span></div>

  <div class="subtabs">
    <button class:active={tab==='traffic'} onclick={()=>setTab('traffic')}>Трафик</button>
    <button class:active={tab==='flow'} onclick={()=>setTab('flow')}>DNS Flow</button>
    <button class:active={tab==='devices'} onclick={()=>setTab('devices')}>Устройства</button>
    <button class:active={tab==='interfaces'} onclick={()=>setTab('interfaces')}>Интерфейсы</button>
    <button class:active={tab==='domains'} onclick={()=>setTab('domains')}>Домены</button>
  </div>

  {#if tab==='traffic'}
    <div class="toolbar"><div class="toolbar-spacer"></div><div class="segmented"><button class:active={historyMinutes===5} onclick={()=>selectHistory(5)}>5 мин</button><button class:active={historyMinutes===60} onclick={()=>selectHistory(60)}>1 час</button><button class:active={historyMinutes===180} onclick={()=>selectHistory(180)}>3 часа</button><button class:active={historyMinutes===1440} onclick={()=>selectHistory(1440)}>24 часа</button></div></div>
    <section class="panel">
      <div class="panel-head"><div><strong>DNS traffic</strong><span>{loading?'обновление…':`${history?.points?.length||0} buckets`}</span></div><div class="legend"><span><i class="requests"></i>Requests</span><span><i class="fallbacks"></i>Fallbacks</span><span><i class="errors"></i>Errors</span><span><i class="timeouts"></i>Timeouts</span></div></div>
      <div class="chart-wrap"><HistoryChart {history}/></div>
    </section>

  {:else if tab==='flow'}
    <div class="toolbar">
      <div class="search-control"><span>⌕</span><input bind:value={flowSearch} placeholder="Домен или устройство…"/></div>
      <select bind:value={flowProfile}><option value="all">Все профили</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
      <button class="button" class:active={fallbackOnly} onclick={()=>fallbackOnly=!fallbackOnly}>Fallback only</button>
      <div class="toolbar-spacer"></div><span class="live-chip {flowPaused?'paused':'running'}">{flowPaused?`ПОТОК ЗАФИКСИРОВАН · ${frozenFlow.length}`:'LIVE'}</span><button class="button" onclick={toggleFlowPause}>{flowPaused?'▶ Продолжить':'Ⅱ Пауза'}</button>
    </div>
    <section class="panel table-panel live-table" onscroll={autoPauseFlow}>
      <div class="table-scroll"><table><thead><tr><th>Время</th><th>Устройство</th><th>Домен</th><th>Профиль</th><th>DNS</th><th>Тип</th><th>Fallback</th></tr></thead><tbody>
        {#if flowRows.length}
          {#each flowRows as x (`${x.time}|${x.client_ip||''}|${x.domain}|${x.qtype}|${x.port||''}`)}
            <tr>
              <td class="mono">{timeOnly(x.time)}</td>
              <td>{#if x.client_ip}<button class="table-link" onclick={()=>{tab='devices';openClient(x.client_ip);}}><strong>{x.client_name||x.client_hostname||x.client_ip}</strong><span>{x.client_ip}{x.client_access?` · ${x.client_access}`:''}</span></button>{:else}<div class="cell-title">{x.client_name||x.client_hostname||'—'}</div>{/if}</td>
              <td><strong>{x.domain}</strong></td><td>{x.profile}</td><td>{x.upstream}</td><td><span class="pill">{x.qtype}</span></td><td>{#if x.fallback}<span class="pill warn">да</span>{:else}—{/if}</td>
            </tr>
          {/each}
        {:else}<tr><td colspan="7" class="empty">Нет подходящих запросов</td></tr>{/if}
      </tbody></table></div>
    </section>

  {:else if tab==='devices'}
    {#if selectedIP}
      <div class="toolbar"><button class="button" onclick={closeClient}>← Все устройства</button><div class="toolbar-spacer"></div><span class="live-chip {clientPaused?'paused':'running'}">{clientPaused?`ПОТОК ЗАФИКСИРОВАН · ${frozenClientEvents.length}`:'LIVE'}</span><button class="button" onclick={toggleClientPause}>{clientPaused?'▶ Продолжить':'Ⅱ Пауза'}</button></div>

      {#if clientDetail?.client}
        {@const c=clientDetail.client}
        {@const complete=Number(c.client_responses||0)+Number(c.client_timeouts||0)}
        {@const localShare=complete?Number(c.cache_local||0)/complete*100:0}
        <section class="hero-card">
          <div class="hero-head"><div><h2>{c.name||c.hostname||c.ip}</h2><p>{c.ip} · {c.mac||'—'} · {c.access||c.network||'Unknown'}</p></div><span class="state-chip {c.active?'good':'neutral'}">{c.active?'ACTIVE':'OFFLINE'}</span></div>
          <div class="hero-metrics six"><div><strong>{fmtInt(c.requests)}</strong><span>Requests</span></div><div><strong>{fmtInt(c.forwarded||0)}</strong><span>Forwarded</span></div><div><strong>{fmtInt(c.cache_local||0)}</strong><span>Cache / local</span></div><div><strong>{localShare.toFixed(1)}%</strong><span>Local share</span></div><div><strong>{Number(c.avg_client_latency_ms||0).toFixed(1)} ms</strong><span>Client avg</span></div><div><strong>{Number(c.p95_client_latency_ms||0).toFixed(1)} ms</strong><span>Client p95</span></div></div>
        </section>

        <div class="two-col">
          <section class="panel">
            <div class="panel-head"><div><strong>DNS состояние клиента</strong><span>{c.policy||'System'}</span></div></div>
            <div class="info-row"><div><strong>Policy</strong><span>Базовая политика Keenetic</span></div><div class="info-value">{c.policy||'System'}</div></div>
            <div class="info-row"><div><strong>Ошибки upstream</strong><span>SERVFAIL/REFUSED и другие</span></div><div class="info-value" class:bad={Number(c.errors||0)>0}>{fmtInt(c.errors||0)}</div></div>
            <div class="info-row"><div><strong>Timeout upstream</strong><span>Таймауты локального proxy</span></div><div class="info-value" class:bad={Number(c.timeouts||0)>0}>{fmtInt(c.timeouts||0)}</div></div>
            <div class="info-row"><div><strong>Ошибки ответа клиенту</strong><span>Client-side DNS errors</span></div><div class="info-value">{fmtInt(c.client_errors||0)}</div></div>
            <div class="info-row"><div><strong>Нет ответа клиенту</strong><span>Client timeout</span></div><div class="info-value">{fmtInt(c.client_timeouts||0)}</div></div>
            <div class="info-row"><div><strong>Fallback</strong><span>Смена resolver</span></div><div class="info-value">{fmtInt(c.fallbacks||0)}</div></div>
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>Маршрут политики</strong><span>{c.route?.mode==='policy-mark'?`${c.route?.name||'Policy'} · mark 0x${Number(c.route?.mark||0).toString(16)} · table ${fmtInt(c.route?.table||0)}`:'System · default route'}</span></div></div>
            {#if c.route?.paths?.length}
              {#each c.route.paths as path}<div class="route-path"><div><strong>{path.description||path.keenetic_interface||path.linux_interface}</strong><span>{[path.keenetic_interface,path.linux_interface,path.type].filter(Boolean).join(' · ')}</span></div>{#if path.weight}<code>weight {fmtInt(path.weight)}</code>{/if}</div>{/each}
            {:else}<div class="empty">Default/nexthop для политики не определён</div>{/if}
          </section>
        </div>

        <div class="toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientFlowSearch} placeholder="Домен / resolver / RCODE…"/></div><select bind:value={clientOutcome}><option value="all">Все исходы</option><option value="FORWARDED">FORWARDED</option><option value="CACHE_LOCAL">CACHE_LOCAL</option><option value="ERROR">ERROR</option><option value="CLIENT_TIMEOUT">CLIENT_TIMEOUT</option></select><span class="panel-meta">{clientEvents.length} событий</span></div>

        <section class="panel table-panel live-table" onscroll={autoPauseClient}>
          <div class="table-scroll"><table><thead><tr><th>Время</th><th>Домен</th><th>Исход</th><th>DNS</th><th>RCODE</th><th>Клиент</th><th>Upstream</th><th>Fallback</th></tr></thead><tbody>
            {#if clientEvents.length}
              {#each clientEvents as e (`${e.time}|${e.domain}|${e.resolver||''}|${e.qtype||''}`)}
                <tr><td class="mono">{timeOnly(e.time)}</td><td><strong>{e.domain}</strong><div class="cell-sub">{e.qtype} · {e.transport}</div></td><td><span class="state-chip {outcomeClass(e.outcome)}">{e.outcome||'—'}</span></td><td>{e.resolver||'—'}<div class="cell-sub mono">{e.resolver_port?`:${fmtInt(e.resolver_port)}`:''}</div></td><td class="mono" class:bad={e.rcode&&e.rcode!=='NOERROR'&&e.rcode!=='NXDOMAIN'}>{e.rcode||'—'}</td><td class="mono">{e.latency_ms?`${Number(e.latency_ms).toFixed(1)} ms`:'—'}</td><td class="mono">{e.upstream_latency_ms?`${Number(e.upstream_latency_ms).toFixed(1)} ms`:'—'}<div class="cell-sub">{e.upstream_timeout?'TIMEOUT':(e.upstream_rcode||'')}</div></td><td>{#if e.fallback}<span class="pill warn">да</span>{:else}—{/if}</td></tr>
              {/each}
            {:else}<tr><td colspan="8" class="empty">Пока нет завершённых DNS-запросов в этом потоке</td></tr>{/if}
          </tbody></table></div>
        </section>
      {:else}
        <section class="panel"><div class="empty">Загружаю данные устройства…</div></section>
      {/if}
    {:else}
      <div class="toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientSearch} placeholder="Устройство, IP, MAC, Policy, интерфейс…"/></div><span class="panel-meta">{filteredClients.length} из {clients.length}</span></div>
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>DNS по устройствам</strong><span>активные сначала, затем по имени</span></div></div>
        <div class="table-scroll"><table><thead><tr><th>Устройство</th><th>IP / Policy</th><th>Подключение</th><th>Запросы</th><th>Forward</th><th>Cache/local</th><th>Ошибки</th><th>Timeout</th><th>Последний</th></tr></thead><tbody>
          {#if filteredClients.length}
            {#each filteredClients as c (c.ip)}
              <tr class="clickable" onclick={()=>openClient(c.ip)}><td><strong>{c.name||c.hostname||c.ip}</strong><div class="cell-sub mono">{c.mac||''}</div></td><td><div class="mono">{c.ip}</div><div class="cell-sub">{c.policy||'System'}</div></td><td>{c.access||c.network||'Unknown'}<div class="cell-sub">{[c.ap,c.mesh_cid?`mesh ${c.mesh_cid}`:''].filter(Boolean).join(' · ')}</div></td><td class="mono">{fmtInt(c.requests)}</td><td class="mono good">{fmtInt(c.forwarded||0)}</td><td class="mono">{fmtInt(c.cache_local||0)}</td><td class="mono" class:bad={Number(c.client_errors||0)>0}>{fmtInt(c.client_errors||0)}</td><td class="mono" class:bad={Number(c.client_timeouts||0)>0}>{fmtInt(c.client_timeouts||0)}</td><td>{c.requests?fmtAgo(c.last_seen):'ожидаем DNS'}</td></tr>
            {/each}
          {:else}<tr><td colspan="9" class="empty">По фильтру устройств ничего не найдено</td></tr>{/if}
        </tbody></table></div>
      </section>
    {/if}

  {:else if tab==='interfaces'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>DNS по интерфейсам</strong><span>{interfaces.length} групп</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>Подключение</th><th>Устройств</th><th>Запросы</th><th>Ошибки</th><th>Timeout</th><th>Fallback</th><th>Top clients</th></tr></thead><tbody>
        {#if interfaces.length}
          {#each interfaces as item (item.name)}<tr><td><strong>{item.name}</strong><div class="cell-sub">{[item.network,item.ap].filter(Boolean).join(' · ')}</div></td><td class="mono">{fmtInt(item.devices)}</td><td class="mono">{fmtInt(item.requests)}</td><td class="mono">{fmtInt(item.errors)}</td><td class="mono">{fmtInt(item.timeouts)}</td><td class="mono">{fmtInt(item.fallbacks)}</td><td>{item.top_clients?.length?item.top_clients.map((c)=>`${c.name} ${fmtInt(c.count)}`).join(' · '):'—'}</td></tr>{/each}
        {:else}<tr><td colspan="7" class="empty">Нет данных по интерфейсам</td></tr>{/if}
      </tbody></table></div>
    </section>

  {:else if tab==='domains'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>Top domains</strong><span>{topDomains.length} записей</span></div></div><div class="domain-list">{#each topDomains as domain (domain.domain)}<div class="domain-row"><div><span>{domain.domain}</span><strong>{fmtInt(domain.count)}</strong></div><div class="progress"><span style={`width:${Number(domain.count||0)/domainMax*100}%`}></span></div></div>{/each}</div></section>
      <section class="panel"><div class="panel-head"><div><strong>Типы запросов</strong><span>live flow</span></div></div>{#each qtypeGroups as [type,rows]}<div class="info-row"><div><strong>{type}</strong><span>DNS qtype</span></div><div class="info-value accent-text">{fmtInt(rows.length)}</div></div>{/each}</section>
    </div>
  {/if}
</div>
