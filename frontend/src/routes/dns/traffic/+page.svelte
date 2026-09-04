<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { getHistory, getClients, getInterfaces, getClient } from '$lib/api.js';
  import { fmtInt, fmtAgo, timeOnly, groupBy } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
  import HistoryChart from '$lib/components/HistoryChart.svelte';

  let tab='traffic', historyMinutes=5, history={points:[]}, clients=[], interfaces=[], selectedIP='', clientDetail=null;
  let flowSearch='', flowProfile='all', fallbackOnly=false, flowPaused=false, frozenFlow=[];
  let clientSearch='', clientPaused=false, frozenClientEvents=[], clientFlowSearch='', clientOutcome='all';
  let loading=false;

  $: locale = $settings.locale || 'ru';
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
    return an.localeCompare(bn,locale,{numeric:true,sensitivity:'base'})||String(a.ip||'').localeCompare(String(b.ip||''),'en',{numeric:true});
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
  function outcomeLabel(value, lang){return value?t(lang,`dns.traffic.outcomes.${value}`):'—';}

  onMount(()=>{
    loadHistory();
    const timer=setInterval(()=>{
      if(tab==='devices'){selectedIP?loadSelectedClient():loadClients();}
      else if(tab==='interfaces')loadInterfaces();
    },3000);
    return()=>clearInterval(timer);
  });
</script>

<svelte:head><title>RouterForge — {t(locale,'dns.traffic.pageTitle')}</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head"><div><h1>{t(locale,'dns.traffic.title')}</h1><p>{t(locale,'dns.traffic.subtitle')}</p></div><span class="page-kicker mono">LIVE / {$snapshot.flow?.length||0} EVENTS</span></div>

  <div class="subtabs">
    <button class:active={tab==='traffic'} onclick={()=>setTab('traffic')}>{t(locale,'dns.traffic.traffic')}</button>
    <button class:active={tab==='flow'} onclick={()=>setTab('flow')}>{t(locale,'dns.traffic.flow')}</button>
    <button class:active={tab==='devices'} onclick={()=>setTab('devices')}>{t(locale,'dns.traffic.devices')}</button>
    <button class:active={tab==='interfaces'} onclick={()=>setTab('interfaces')}>{t(locale,'dns.traffic.interfaces')}</button>
    <button class:active={tab==='domains'} onclick={()=>setTab('domains')}>{t(locale,'dns.traffic.domains')}</button>
  </div>

  {#if tab==='traffic'}
    <div class="toolbar"><div class="toolbar-spacer"></div><div class="segmented"><button class:active={historyMinutes===5} onclick={()=>selectHistory(5)}>{t(locale,'common.minutesShort',{count:5})}</button><button class:active={historyMinutes===60} onclick={()=>selectHistory(60)}>{t(locale,'common.hourShort')}</button><button class:active={historyMinutes===180} onclick={()=>selectHistory(180)}>{t(locale,'common.hours3')}</button><button class:active={historyMinutes===1440} onclick={()=>selectHistory(1440)}>{t(locale,'common.hours24')}</button></div></div>
    <section class="panel">
      <div class="panel-head"><div><strong>DNS traffic</strong><span>{loading?t(locale,'dns.traffic.updating'):t(locale,'dns.traffic.buckets',{count:history?.points?.length||0})}</span></div><div class="legend"><span><i class="requests"></i>{t(locale,'common.requests')}</span><span><i class="fallbacks"></i>{t(locale,'common.fallback')}</span><span><i class="errors"></i>{t(locale,'common.errors')}</span><span><i class="timeouts"></i>{t(locale,'common.timeouts')}</span></div></div>
      <div class="chart-wrap"><HistoryChart {history}/></div>
    </section>

  {:else if tab==='flow'}
    <div class="toolbar">
      <div class="search-control"><span>⌕</span><input bind:value={flowSearch} placeholder={t(locale,'dns.traffic.domainOrDevice')}/></div>
      <select bind:value={flowProfile}><option value="all">{t(locale,'dns.traffic.allProfiles')}</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
      <button class="button" class:active={fallbackOnly} onclick={()=>fallbackOnly=!fallbackOnly}>{t(locale,'dns.traffic.fallbackOnly')}</button>
      <div class="toolbar-spacer"></div><span class="live-chip {flowPaused?'paused':'running'}">{flowPaused?t(locale,'dns.traffic.flowFrozen',{count:frozenFlow.length}):'LIVE'}</span><button class="button" onclick={toggleFlowPause}>{flowPaused?t(locale,'dns.traffic.continue'):t(locale,'dns.traffic.pause')}</button>
    </div>
    <section class="panel table-panel live-table" onscroll={autoPauseFlow}>
      <div class="table-scroll"><table><thead><tr><th>{t(locale,'common.time')}</th><th>{t(locale,'common.device')}</th><th>{t(locale,'common.domain')}</th><th>{t(locale,'common.profile')}</th><th>DNS</th><th>{t(locale,'common.type')}</th><th>{t(locale,'common.fallback')}</th></tr></thead><tbody>
        {#if flowRows.length}
          {#each flowRows as x (`${x.time}|${x.client_ip||''}|${x.domain}|${x.qtype}|${x.port||''}`)}
            <tr>
              <td class="mono">{timeOnly(x.time,locale)}</td>
              <td>{#if x.client_ip}<button class="table-link" onclick={()=>{tab='devices';openClient(x.client_ip);}}><strong>{x.client_name||x.client_hostname||x.client_ip}</strong><span>{x.client_ip}{x.client_access?` · ${x.client_access}`:''}</span></button>{:else}<div class="cell-title">{x.client_name||x.client_hostname||'—'}</div>{/if}</td>
              <td><strong>{x.domain}</strong></td><td>{x.profile}</td><td>{x.upstream}</td><td><span class="pill">{x.qtype}</span></td><td>{#if x.fallback}<span class="pill warn">{t(locale,'common.yes')}</span>{:else}—{/if}</td>
            </tr>
          {/each}
        {:else}<tr><td colspan="7" class="empty">{t(locale,'dns.traffic.noMatching')}</td></tr>{/if}
      </tbody></table></div>
    </section>

  {:else if tab==='devices'}
    {#if selectedIP}
      <div class="toolbar"><button class="button" onclick={closeClient}>{t(locale,'dns.traffic.allDevices')}</button><div class="toolbar-spacer"></div><span class="live-chip {clientPaused?'paused':'running'}">{clientPaused?t(locale,'dns.traffic.flowFrozen',{count:frozenClientEvents.length}):'LIVE'}</span><button class="button" onclick={toggleClientPause}>{clientPaused?t(locale,'dns.traffic.continue'):t(locale,'dns.traffic.pause')}</button></div>

      {#if clientDetail?.client}
        {@const c=clientDetail.client}
        {@const complete=Number(c.client_responses||0)+Number(c.client_timeouts||0)}
        {@const localShare=complete?Number(c.cache_local||0)/complete*100:0}
        <section class="hero-card">
          <div class="hero-head"><div><h2>{c.name||c.hostname||c.ip}</h2><p>{c.ip} · {c.mac||'—'} · {c.access||c.network||t(locale,'dns.traffic.unknown')}</p></div><span class="state-chip {c.active?'good':'neutral'}">{c.active?t(locale,'dns.traffic.active'):t(locale,'dns.traffic.offline')}</span></div>
          <div class="hero-metrics six"><div><strong>{fmtInt(c.requests,locale)}</strong><span>{t(locale,'dns.traffic.requestsMetric')}</span></div><div><strong>{fmtInt(c.forwarded||0,locale)}</strong><span>{t(locale,'dns.traffic.forwardedMetric')}</span></div><div><strong>{fmtInt(c.cache_local||0,locale)}</strong><span>{t(locale,'dns.traffic.cacheLocalMetric')}</span></div><div><strong>{localShare.toFixed(1)}%</strong><span>{t(locale,'dns.traffic.localShareMetric')}</span></div><div><strong>{Number(c.avg_client_latency_ms||0).toFixed(1)} ms</strong><span>{t(locale,'dns.traffic.clientAvgMetric')}</span></div><div><strong>{Number(c.p95_client_latency_ms||0).toFixed(1)} ms</strong><span>{t(locale,'dns.traffic.clientP95Metric')}</span></div></div>
        </section>

        <div class="two-col">
          <section class="panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.traffic.dnsClientState')}</strong><span>{c.policy||'System'}</span></div></div>
            <div class="info-row"><div><strong>Policy</strong><span>{t(locale,'dns.traffic.basePolicy')}</span></div><div class="info-value">{c.policy||'System'}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.traffic.upstreamErrors')}</strong><span>{t(locale,'dns.traffic.upstreamErrorsHint')}</span></div><div class="info-value" class:bad={Number(c.errors||0)>0}>{fmtInt(c.errors||0,locale)}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.traffic.upstreamTimeout')}</strong><span>{t(locale,'dns.traffic.upstreamTimeoutHint')}</span></div><div class="info-value" class:bad={Number(c.timeouts||0)>0}>{fmtInt(c.timeouts||0,locale)}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.traffic.clientErrors')}</strong><span>{t(locale,'dns.traffic.clientErrorsHint')}</span></div><div class="info-value">{fmtInt(c.client_errors||0,locale)}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.traffic.clientTimeout')}</strong><span>{t(locale,'dns.traffic.clientTimeoutHint')}</span></div><div class="info-value">{fmtInt(c.client_timeouts||0,locale)}</div></div>
            <div class="info-row"><div><strong>Fallback</strong><span>{t(locale,'dns.traffic.fallbackHint')}</span></div><div class="info-value">{fmtInt(c.fallbacks||0,locale)}</div></div>
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.traffic.policyRoute')}</strong><span>{c.route?.mode==='policy-mark'?`${c.route?.name||'Policy'} · mark 0x${Number(c.route?.mark||0).toString(16)} · table ${fmtInt(c.route?.table||0,locale)}`:t(locale,'dns.traffic.systemDefaultRoute')}</span></div></div>
            {#if c.route?.paths?.length}
              {#each c.route.paths as path}<div class="route-path"><div><strong>{path.description||path.keenetic_interface||path.linux_interface}</strong><span>{[path.keenetic_interface,path.linux_interface,path.type].filter(Boolean).join(' · ')}</span></div>{#if path.weight}<code>{t(locale,'dns.traffic.weight',{value:fmtInt(path.weight,locale)})}</code>{/if}</div>{/each}
            {:else}<div class="empty">{t(locale,'dns.traffic.noPolicyRoute')}</div>{/if}
          </section>
        </div>

        <div class="toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientFlowSearch} placeholder={t(locale,'dns.traffic.clientSearch')}/></div><select bind:value={clientOutcome}><option value="all">{t(locale,'dns.traffic.allOutcomes')}</option><option value="FORWARDED">{t(locale,'dns.traffic.outcomes.FORWARDED')}</option><option value="CACHE_LOCAL">{t(locale,'dns.traffic.outcomes.CACHE_LOCAL')}</option><option value="ERROR">{t(locale,'dns.traffic.outcomes.ERROR')}</option><option value="CLIENT_TIMEOUT">{t(locale,'dns.traffic.outcomes.CLIENT_TIMEOUT')}</option></select><span class="panel-meta">{t(locale,'common.eventCount',{count:clientEvents.length})}</span></div>

        <section class="panel table-panel live-table" onscroll={autoPauseClient}>
          <div class="table-scroll"><table><thead><tr><th>{t(locale,'common.time')}</th><th>{t(locale,'common.domain')}</th><th>{t(locale,'dns.traffic.outcome')}</th><th>DNS</th><th>RCODE</th><th>{t(locale,'dns.traffic.client')}</th><th>{t(locale,'dns.traffic.upstream')}</th><th>{t(locale,'common.fallback')}</th></tr></thead><tbody>
            {#if clientEvents.length}
              {#each clientEvents as e (`${e.time}|${e.domain}|${e.resolver||''}|${e.qtype||''}`)}
                <tr><td class="mono">{timeOnly(e.time,locale)}</td><td><strong>{e.domain}</strong><div class="cell-sub">{e.qtype} · {e.transport}</div></td><td><span class="state-chip {outcomeClass(e.outcome)}">{outcomeLabel(e.outcome,locale)}</span></td><td>{e.resolver||'—'}<div class="cell-sub mono">{e.resolver_port?`:${fmtInt(e.resolver_port,locale)}`:''}</div></td><td class="mono" class:bad={e.rcode&&e.rcode!=='NOERROR'&&e.rcode!=='NXDOMAIN'}>{e.rcode||'—'}</td><td class="mono">{e.latency_ms?`${Number(e.latency_ms).toFixed(1)} ms`:'—'}</td><td class="mono">{e.upstream_latency_ms?`${Number(e.upstream_latency_ms).toFixed(1)} ms`:'—'}<div class="cell-sub">{e.upstream_timeout?'TIMEOUT':(e.upstream_rcode||'')}</div></td><td>{#if e.fallback}<span class="pill warn">{t(locale,'common.yes')}</span>{:else}—{/if}</td></tr>
              {/each}
            {:else}<tr><td colspan="8" class="empty">{t(locale,'dns.traffic.noCompleted')}</td></tr>{/if}
          </tbody></table></div>
        </section>
      {:else}
        <section class="panel"><div class="empty">{t(locale,'dns.traffic.loadingDevice')}</div></section>
      {/if}
    {:else}
      <div class="toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientSearch} placeholder={t(locale,'dns.traffic.deviceSearch')}/></div><span class="panel-meta">{t(locale,'dns.traffic.shownOf',{shown:filteredClients.length,total:clients.length})}</span></div>
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{t(locale,'dns.traffic.dnsByDevices')}</strong><span>{t(locale,'dns.traffic.devicesOrderHint')}</span></div></div>
        <div class="table-scroll"><table><thead><tr><th>{t(locale,'common.device')}</th><th>IP / Policy</th><th>{t(locale,'dns.traffic.connection')}</th><th>{t(locale,'common.requests')}</th><th>{t(locale,'dns.traffic.forward')}</th><th>{t(locale,'dns.traffic.cacheLocal')}</th><th>{t(locale,'common.errors')}</th><th>{t(locale,'common.timeout')}</th><th>{t(locale,'dns.traffic.last')}</th></tr></thead><tbody>
          {#if filteredClients.length}
            {#each filteredClients as c (c.ip)}
              <tr class="clickable" onclick={()=>openClient(c.ip)}><td><strong>{c.name||c.hostname||c.ip}</strong><div class="cell-sub mono">{c.mac||''}</div></td><td><div class="mono">{c.ip}</div><div class="cell-sub">{c.policy||'System'}</div></td><td>{c.access||c.network||t(locale,'dns.traffic.unknown')}<div class="cell-sub">{[c.ap,c.mesh_cid?`mesh ${c.mesh_cid}`:''].filter(Boolean).join(' · ')}</div></td><td class="mono">{fmtInt(c.requests,locale)}</td><td class="mono good">{fmtInt(c.forwarded||0,locale)}</td><td class="mono">{fmtInt(c.cache_local||0,locale)}</td><td class="mono" class:bad={Number(c.client_errors||0)>0}>{fmtInt(c.client_errors||0,locale)}</td><td class="mono" class:bad={Number(c.client_timeouts||0)>0}>{fmtInt(c.client_timeouts||0,locale)}</td><td>{c.requests?fmtAgo(c.last_seen,locale):t(locale,'dns.traffic.waitingDns')}</td></tr>
            {/each}
          {:else}<tr><td colspan="9" class="empty">{t(locale,'dns.traffic.noDevices')}</td></tr>{/if}
        </tbody></table></div>
      </section>
    {/if}

  {:else if tab==='interfaces'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.traffic.dnsByInterfaces')}</strong><span>{t(locale,'common.groupCount',{count:interfaces.length})}</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>{t(locale,'dns.traffic.connection')}</th><th>{t(locale,'common.devices')}</th><th>{t(locale,'common.requests')}</th><th>{t(locale,'common.errors')}</th><th>{t(locale,'common.timeout')}</th><th>{t(locale,'common.fallback')}</th><th>{t(locale,'dns.traffic.topClients')}</th></tr></thead><tbody>
        {#if interfaces.length}
          {#each interfaces as item (item.name)}<tr><td><strong>{item.name}</strong><div class="cell-sub">{[item.network,item.ap].filter(Boolean).join(' · ')}</div></td><td class="mono">{fmtInt(item.devices,locale)}</td><td class="mono">{fmtInt(item.requests,locale)}</td><td class="mono">{fmtInt(item.errors,locale)}</td><td class="mono">{fmtInt(item.timeouts,locale)}</td><td class="mono">{fmtInt(item.fallbacks,locale)}</td><td>{item.top_clients?.length?item.top_clients.map((c)=>`${c.name} ${fmtInt(c.count,locale)}`).join(' · '):'—'}</td></tr>{/each}
        {:else}<tr><td colspan="7" class="empty">{t(locale,'dns.traffic.noInterfaces')}</td></tr>{/if}
      </tbody></table></div>
    </section>

  {:else if tab==='domains'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>{t(locale,'dns.traffic.topDomains')}</strong><span>{t(locale,'common.entryCount',{count:topDomains.length})}</span></div></div><div class="domain-list">{#each topDomains as domain (domain.domain)}<div class="domain-row"><div><span>{domain.domain}</span><strong>{fmtInt(domain.count,locale)}</strong></div><div class="progress"><span style={`width:${Number(domain.count||0)/domainMax*100}%`}></span></div></div>{/each}</div></section>
      <section class="panel"><div class="panel-head"><div><strong>{t(locale,'dns.traffic.queryTypes')}</strong><span>{t(locale,'dns.traffic.liveFlow')}</span></div></div>{#each qtypeGroups as [type,rows]}<div class="info-row"><div><strong>{type}</strong><span>DNS qtype</span></div><div class="info-value accent-text">{fmtInt(rows.length,locale)}</div></div>{/each}</section>
    </div>
  {/if}
</div>
