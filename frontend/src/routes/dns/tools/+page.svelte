<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import DNSInfoPanel from '$lib/components/DNSInfoPanel.svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { getSystem } from '$lib/api.js';
  import { timeOnly, fmtInt, fmtDuration, bytes } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let tab='journal', system={}, kind='all', search='', paused=false, frozenErrors=[], frozenBursts=[];

  $: locale = $settings.locale || 'ru';
  $: errorSource=paused?frozenErrors:($snapshot.errors||[]);
  $: rows=errorSource.slice().reverse()
    .filter((x)=>kind==='all'||x.kind===kind)
    .filter((x)=>!search.trim()||`${x.kind||''} ${x.profile||''} ${x.upstream||''} ${x.domain||''} ${x.message||''}`.toLowerCase().includes(search.trim().toLowerCase()));
  $: bursts=(paused?frozenBursts:($snapshot.error_bursts||[])).slice(0,8);

  function togglePause(){
    if(!paused){frozenErrors=($snapshot.errors||[]).slice();frozenBursts=($snapshot.error_bursts||[]).slice();paused=true;}
    else{paused=false;frozenErrors=[];frozenBursts=[];}
  }

  async function setTab(value){
    tab=value;
    if(value==='system'){try{system=await getSystem();}catch{}}
  }
</script>

<svelte:head><title>RouterForge — {t(locale,'dns.tools.pageTitle')}</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head"><div><h1>{t(locale,'dns.tools.pageTitle')}</h1><p>{t(locale,'dns.tools.subtitle')}</p></div><span class="page-kicker mono">READ-ONLY TOOLS</span></div>
  <div class="subtabs"><button class:active={tab==='journal'} onclick={()=>setTab('journal')}>{t(locale,'dns.tools.journal')}</button><button class:active={tab==='connections'} onclick={()=>setTab('connections')}>{t(locale,'dns.tools.connections')}</button><button class:active={tab==='dns'} onclick={()=>setTab('dns')}>{t(locale,'dns.tools.dnsInfo')}</button><button class:active={tab==='system'} onclick={()=>setTab('system')}>{t(locale,'dns.tools.system')}</button></div>

  {#if tab==='journal'}
    {#if bursts.length}
      <section class="panel table-panel"><div class="panel-head"><div><strong>{t(locale,'dns.tools.errorSummary')}</strong><span>{t(locale,'dns.tools.burstBuckets',{count:bursts.length})}</span></div></div><div class="table-scroll"><table><thead><tr><th>{t(locale,'common.time')}</th><th>DNS</th><th>{t(locale,'common.type')}</th><th>{t(locale,'common.count')}</th><th>{t(locale,'dns.tools.examples')}</th></tr></thead><tbody>
        {#each bursts as b (`${b.minute}-${b.upstream}-${b.kind}`)}<tr><td class="mono">{timeOnly(b.minute,locale)}</td><td>{b.profile} · <strong>{b.upstream}</strong></td><td><span class="pill {b.kind==='TIMEOUT'?'warn':'error'}">{b.kind}</span></td><td class="bad">{fmtInt(b.count,locale)}</td><td class="cell-sub">{(b.domains||[]).join(', ')||'—'}</td></tr>{/each}
      </tbody></table></div></section>
    {/if}
    <section class="panel">
      <div class="toolbar embedded"><span class="live-chip {paused?'paused':'running'}">{paused?t(locale,'dns.tools.journalFrozen'):'LIVE'}</span><button class="button" onclick={togglePause}>{paused?t(locale,'dns.tools.continue'):t(locale,'dns.tools.pause')}</button>{#each ['all','SERVFAIL','TIMEOUT','DOWN','RECOVERED'] as value}<button class="button" class:active={kind===value} onclick={()=>kind=value}>{value==='all'?t(locale,'common.all').toUpperCase():value.toUpperCase()}</button>{/each}<div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder={t(locale,'common.search')}/></div></div>
      <div class="log-shell">{#if rows.length}{#each rows as row (`${row.time}|${row.kind}|${row.upstream}|${row.domain}`)}<div class="log-row"><span class="mono muted">{timeOnly(row.time,locale)}</span><span class="log-kind {row.kind}">{row.kind}</span><span>{row.profile||''} · {row.upstream||''} · {row.domain||''} — {row.message||''}</span></div>{/each}{:else}<div class="empty">{t(locale,'dns.tools.noEvents')}</div>{/if}</div>
    </section>

  {:else if tab==='connections'}
    <section class="panel table-panel"><div class="panel-head"><div><strong>{t(locale,'dns.tools.localProxies')}</strong><span>{t(locale,'dns.tools.endpoints',{count:($snapshot.upstreams||[]).length})}</span></div></div><div class="table-scroll"><table><thead><tr><th>{t(locale,'common.profile')}</th><th>DNS</th><th>{t(locale,'dns.tools.protocol')}</th><th>{t(locale,'dns.tools.local')}</th><th>{t(locale,'dns.tools.target')}</th><th>{t(locale,'common.interface')}</th></tr></thead><tbody>{#each ($snapshot.upstreams||[]) as u (u.port)}<tr><td>{u.profile}</td><td><strong>{u.name}</strong></td><td><span class="pill accent">{u.protocol}</span></td><td class="mono">127.0.0.1:{u.port}</td><td class="mono">{u.target}</td><td>{u.interface||'—'}</td></tr>{/each}</tbody></table></div></section>

  {:else if tab==='dns'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>Discovery</strong><span>{t(locale,'dns.tools.discoveryState')}</span></div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.lastDiscovery')}</strong></div><div class="info-value">{$snapshot.last_discovery||'—'}</div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.discoveryError')}</strong></div><div class="info-value">{$snapshot.discovery_error||t(locale,'common.none')}</div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.captureError')}</strong></div><div class="info-value">{$snapshot.capture_error||t(locale,'common.none')}</div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.upstreamDns')}</strong></div><div class="info-value">{fmtInt($snapshot.upstream_count,locale)}</div></div></section>
      <section class="panel"><div class="panel-head"><div><strong>{t(locale,'dns.tools.counters')}</strong><span>{t(locale,'dns.tools.runtimeTotals')}</span></div></div><div class="info-row"><div><strong>{t(locale,'common.requests')}</strong></div><div class="info-value">{fmtInt($snapshot.total_requests,locale)}</div></div><div class="info-row"><div><strong>{t(locale,'common.responses')}</strong></div><div class="info-value">{fmtInt($snapshot.total_responses,locale)}</div></div><div class="info-row"><div><strong>{t(locale,'common.fallback')}</strong></div><div class="info-value">{fmtInt($snapshot.total_fallbacks,locale)}</div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.activeDown')}</strong></div><div class="info-value">{fmtInt($snapshot.active_down,locale)}</div></div></section>
    </div>

    <DNSInfoPanel />

  {:else if tab==='system'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>RouterForge</strong><span>{t(locale,'dns.tools.runtime')}</span></div></div><div class="info-row"><div><strong>{t(locale,'common.version')}</strong></div><div class="info-value">{$snapshot.version||'—'}</div></div><div class="info-row"><div><strong>Uptime</strong></div><div class="info-value">{fmtDuration($snapshot.uptime_seconds,locale)}</div></div><div class="info-row"><div><strong>{t(locale,'dns.tools.goRuntime')}</strong></div><div class="info-value mono">{system.go_version||'—'}</div></div><div class="info-row"><div><strong>{t(locale,'common.architecture')}</strong></div><div class="info-value mono">{system.goarch||'—'}</div></div></section>
      <section class="panel"><div class="panel-head"><div><strong>{t(locale,'dns.tools.process')}</strong><span>dns-monitor</span></div></div><div class="info-row"><div><strong>RSS</strong></div><div class="info-value">{bytes((system.rss_kb||0)*1024)}</div></div><div class="info-row"><div><strong>VmSize</strong></div><div class="info-value">{bytes((system.vmsize_kb||0)*1024)}</div></div><div class="info-row"><div><strong>Goroutines</strong></div><div class="info-value">{fmtInt(system.goroutines||0,locale)}</div></div><div class="info-row"><div><strong>PID</strong></div><div class="info-value mono">{fmtInt(system.pid||0,locale)}</div></div></section>
    </div>
  {/if}
</div>
