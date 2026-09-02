<script>
  import { snapshot } from '$lib/stores/snapshot.js';
  import { getSystem } from '$lib/api.js';
  import { timeOnly, fmtInt, fmtDuration, bytes } from '$lib/utils.js';

  let tab='journal', system={}, kind='all', search='', paused=false, frozenErrors=[], frozenBursts=[];

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

<svelte:head><title>DNS Monitor — Инструменты</title></svelte:head>

<div class="page">
  <div class="page-head"><div><h1>Инструменты</h1><p>Журнал ошибок, локальные DNS proxy и процесс DNS Monitor.</p></div><span class="page-kicker mono">READ-ONLY TOOLS</span></div>
  <div class="subtabs"><button class:active={tab==='journal'} onclick={()=>setTab('journal')}>Журнал</button><button class:active={tab==='connections'} onclick={()=>setTab('connections')}>Соединения</button><button class:active={tab==='dns'} onclick={()=>setTab('dns')}>Сведения о DNS</button><button class:active={tab==='system'} onclick={()=>setTab('system')}>Система</button></div>

  {#if tab==='journal'}
    {#if bursts.length}
      <section class="panel table-panel"><div class="panel-head"><div><strong>Сводка ошибок за 1 час</strong><span>{bursts.length} burst buckets</span></div></div><div class="table-scroll"><table><thead><tr><th>Время</th><th>DNS</th><th>Тип</th><th>Кол-во</th><th>Примеры доменов</th></tr></thead><tbody>
        {#each bursts as b (`${b.minute}-${b.upstream}-${b.kind}`)}<tr><td class="mono">{timeOnly(b.minute)}</td><td>{b.profile} · <strong>{b.upstream}</strong></td><td><span class="pill {b.kind==='TIMEOUT'?'warn':'error'}">{b.kind}</span></td><td class="bad">{fmtInt(b.count)}</td><td class="cell-sub">{(b.domains||[]).join(', ')||'—'}</td></tr>{/each}
      </tbody></table></div></section>
    {/if}
    <section class="panel">
      <div class="toolbar embedded"><span class="live-chip {paused?'paused':'running'}">{paused?'ЖУРНАЛ ЗАФИКСИРОВАН':'LIVE'}</span><button class="button" onclick={togglePause}>{paused?'▶ Продолжить':'Ⅱ Пауза'}</button>{#each ['all','SERVFAIL','TIMEOUT','DOWN','RECOVERED'] as value}<button class="button" class:active={kind===value} onclick={()=>kind=value}>{value.toUpperCase()}</button>{/each}<div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск…"/></div></div>
      <div class="log-shell">{#if rows.length}{#each rows as row (`${row.time}|${row.kind}|${row.upstream}|${row.domain}`)}<div class="log-row"><span class="mono muted">{timeOnly(row.time)}</span><span class="log-kind {row.kind}">{row.kind}</span><span>{row.profile||''} · {row.upstream||''} · {row.domain||''} — {row.message||''}</span></div>{/each}{:else}<div class="empty">Событий нет</div>{/if}</div>
    </section>

  {:else if tab==='connections'}
    <section class="panel table-panel"><div class="panel-head"><div><strong>Локальные DNS proxy</strong><span>{($snapshot.upstreams||[]).length} endpoints</span></div></div><div class="table-scroll"><table><thead><tr><th>Профиль</th><th>DNS</th><th>Протокол</th><th>Local</th><th>Target</th><th>Interface</th></tr></thead><tbody>{#each ($snapshot.upstreams||[]) as u (u.port)}<tr><td>{u.profile}</td><td><strong>{u.name}</strong></td><td><span class="pill accent">{u.protocol}</span></td><td class="mono">127.0.0.1:{u.port}</td><td class="mono">{u.target}</td><td>{u.interface||'—'}</td></tr>{/each}</tbody></table></div></section>

  {:else if tab==='dns'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>Discovery</strong><span>Keenetic state</span></div></div><div class="info-row"><div><strong>Последний discovery</strong></div><div class="info-value">{$snapshot.last_discovery||'—'}</div></div><div class="info-row"><div><strong>Discovery error</strong></div><div class="info-value">{$snapshot.discovery_error||'нет'}</div></div><div class="info-row"><div><strong>Capture error</strong></div><div class="info-value">{$snapshot.capture_error||'нет'}</div></div><div class="info-row"><div><strong>Upstream DNS</strong></div><div class="info-value">{fmtInt($snapshot.upstream_count)}</div></div></section>
      <section class="panel"><div class="panel-head"><div><strong>Счётчики</strong><span>runtime totals</span></div></div><div class="info-row"><div><strong>Requests</strong></div><div class="info-value">{fmtInt($snapshot.total_requests)}</div></div><div class="info-row"><div><strong>Responses</strong></div><div class="info-value">{fmtInt($snapshot.total_responses)}</div></div><div class="info-row"><div><strong>Fallbacks</strong></div><div class="info-value">{fmtInt($snapshot.total_fallbacks)}</div></div><div class="info-row"><div><strong>Active DOWN</strong></div><div class="info-value">{fmtInt($snapshot.active_down)}</div></div></section>
    </div>

  {:else if tab==='system'}
    <div class="two-col">
      <section class="panel"><div class="panel-head"><div><strong>DNS Monitor</strong><span>runtime</span></div></div><div class="info-row"><div><strong>Версия</strong></div><div class="info-value">{$snapshot.version||'—'}</div></div><div class="info-row"><div><strong>Uptime</strong></div><div class="info-value">{fmtDuration($snapshot.uptime_seconds)}</div></div><div class="info-row"><div><strong>Go runtime</strong></div><div class="info-value mono">{system.go_version||'—'}</div></div><div class="info-row"><div><strong>Архитектура</strong></div><div class="info-value mono">{system.goarch||'—'}</div></div></section>
      <section class="panel"><div class="panel-head"><div><strong>Процесс</strong><span>dns-monitor</span></div></div><div class="info-row"><div><strong>RSS</strong></div><div class="info-value">{bytes((system.rss_kb||0)*1024)}</div></div><div class="info-row"><div><strong>VmSize</strong></div><div class="info-value">{bytes((system.vmsize_kb||0)*1024)}</div></div><div class="info-row"><div><strong>Goroutines</strong></div><div class="info-value">{fmtInt(system.goroutines||0)}</div></div><div class="info-row"><div><strong>PID</strong></div><div class="info-value mono">{fmtInt(system.pid||0)}</div></div></section>
    </div>
  {/if}
</div>
