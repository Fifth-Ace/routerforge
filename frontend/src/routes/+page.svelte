<script>
  import { snapshot } from '$lib/stores/snapshot.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, errorCount, quality, qualityClass, latencyClass, groupBy, profileOrder, total } from '$lib/utils.js';

  let search='';
  let profile='all';
  let activeOnly=false;

  $: upstreams=$snapshot.upstreams||[];
  $: profiles=Object.keys(groupBy(upstreams,'profile')).sort(profileOrder);
  $: filtered=upstreams.filter((u)=>{
    if(profile!=='all'&&u.profile!==profile)return false;
    if(activeOnly&&!u.active)return false;
    const q=search.trim().toLowerCase();
    return !q||`${u.name} ${u.target} ${u.profile}`.toLowerCase().includes(q);
  });
  $: groups=Object.entries(groupBy(filtered,'profile')).sort(([a],[b])=>profileOrder(a,b));
  $: requests=total(upstreams,'requests');
  $: errors=upstreams.reduce((n,u)=>n+errorCount(u)+Number(u.timeouts||0),0);
  $: active=upstreams.filter((u)=>u.active).length;
  $: healthy=upstreams.filter((u)=>u.health_status!=='DOWN').length;

  const share=(u)=>requests?Number(u.requests||0)/requests*100:0;
</script>

<svelte:head><title>DNS Monitor — Обзор</title></svelte:head>

<div class="page">
  <div class="page-head">
    <div><h1>Обзор</h1><p>Живое состояние DNS resolver'ов Keenetic и качество маршрутов.</p></div>
    <span class="page-kicker mono">CORE / DNS OBSERVABILITY</span>
  </div>

  <div class="toolbar">
    <div class="search-control"><span>⌕</span><input bind:value={search} placeholder="Поиск DNS или профиля…"/></div>
    <select bind:value={profile}><option value="all">Все профили</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
    <button class="button" class:active={activeOnly} onclick={()=>activeOnly=true}>Активные</button>
    <button class="button" class:active={!activeOnly} onclick={()=>activeOnly=false}>Все</button>
  </div>

  <section class="metric-grid four">
    <div class="metric-card"><span>DNS серверы</span><strong>{healthy}/{Number($snapshot.upstream_count||0)}</strong><small>{active} активны · DOWN {Number($snapshot.down||0)}</small></div>
    <div class="metric-card"><span>Запросы</span><strong>{fmtInt(requests)}</strong><small>{fmtInt($snapshot.total_responses||0)} ответов</small></div>
    <div class="metric-card"><span>Fallback</span><strong>{fmtInt($snapshot.total_fallbacks||0)}</strong><small>{requests?fmtPct(Number($snapshot.total_fallbacks||0)/requests*100):'0%'}</small></div>
    <div class="metric-card"><span>Ошибки</span><strong>{fmtInt(errors)}</strong><small>{fmtInt($snapshot.total_timeouts||0)} timeout · {Number($snapshot.active_degraded||0)} degraded</small></div>
  </section>

  {#if !groups.length}<section class="panel"><div class="empty">Ничего не найдено</div></section>{/if}

  {#each groups as [groupName,items]}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{groupName}</strong><span>{items.length} DNS</span></div></div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>DNS</th><th>Статус</th><th>Тип</th><th>Трафик</th><th>Latency 5m</th><th>Fallback</th><th>Качество 5m</th><th class="expert-only">Port</th><th class="expert-only">Interface</th></tr></thead>
          <tbody>
            {#each items as u (u.port)}
              {@const st=statusFor(u)}
              {@const win=u.stats_5m||{}}
              {@const pct=share(u)}
              {@const q=quality(u)}
              <tr>
                <td><div class="cell-title">{u.name}</div><div class="cell-sub">{u.target||u.sni||'—'}</div></td>
                <td><span class="state-chip {st.cls}">{st.label}</span><div class="cell-sub">{u.active?'используется сейчас':fmtAgo(u.last_request)}</div></td>
                <td><span class="pill accent">{u.protocol}</span></td>
                <td><div class="split-value"><span>{fmtInt(u.requests)} req</span><span class="accent-text">{fmtPct(pct)}</span></div><div class="progress"><span style={`width:${Math.min(100,pct)}%`}></span></div></td>
                <td>{#if Number(win.p95_latency_ms||0)}<span class="latency {latencyClass(win.p95_latency_ms)}">p95 {fmtMs(win.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(win.avg_latency_ms)}</div>{:else}—{/if}</td>
                <td><strong class={Number(win.fallbacks||0)>0 ? 'warn-text' : ''}>{fmtInt(win.fallbacks||0)}</strong><div class="cell-sub">{fmtPct(win.fallback_pct||0)}</div></td>
                <td><strong class={qualityClass(win)}>{q.toFixed(1)}%</strong><div class="cell-sub">{fmtInt(Number(win.errors||0)+Number(win.timeouts||0))} err · {fmtInt(win.timeouts||0)} timeout</div></td>
                <td class="expert-only mono">{u.port}</td><td class="expert-only">{u.interface||'—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/each}
</div>
