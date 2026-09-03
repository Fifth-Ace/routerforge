<script>
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { getPlainDNS } from '$lib/api.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, errorCount, quality, qualityClass, latencyClass, groupBy, profileOrder, total } from '$lib/utils.js';

  let search='';
  let profile='all';
  let activeOnly=false;
  let plain={resolvers:[],recent:[],pending:0};
  let plainError='';
  let plainTimer=null;

  $: upstreams=$snapshot.upstreams||[];
  $: plainResolvers=plain.resolvers||[];
  $: profiles=[...new Set([
    ...Object.keys(groupBy(upstreams,'profile')),
    ...plainResolvers.flatMap((r)=>r.profiles||[])
  ])].sort(profileOrder);
  $: filtered=upstreams.filter((u)=>{
    if(profile!=='all'&&u.profile!==profile)return false;
    if(activeOnly&&!u.active)return false;
    const q=search.trim().toLowerCase();
    return !q||`${u.name} ${u.target} ${u.profile}`.toLowerCase().includes(q);
  });
  $: filteredPlain=plainResolvers.filter((r)=>{
    if(profile!=='all'&&!(r.profiles||[]).includes(profile))return false;
    if(activeOnly&&!plainRecentlyActive(r))return false;
    const q=search.trim().toLowerCase();
    return !q||`${r.name||''} ${r.address||''} ${(r.profiles||[]).join(' ')} ${(r.domains||[]).join(' ')}`.toLowerCase().includes(q);
  });
  $: groups=Object.entries(groupBy(filtered,'profile')).sort(([a],[b])=>profileOrder(a,b));

  $: encryptedRequests=total(upstreams,'requests');
  $: plainRequests=total(plainResolvers,'requests');
  $: requests=encryptedRequests+plainRequests;
  $: responses=Number($snapshot.total_responses||0)+total(plainResolvers,'responses');
  $: plainTimeouts=total(plainResolvers,'timeouts');
  $: plainErrors=total(plainResolvers,'errors');
  $: errors=upstreams.reduce((n,u)=>n+errorCount(u)+Number(u.timeouts||0),0)+plainErrors+plainTimeouts;
  $: plainDown=plainResolvers.filter((r)=>Number(r.requests||0)>0&&Number(r.responses||0)===0&&Number(r.timeouts||0)>0).length;
  $: plainDegraded=plainResolvers.filter((r)=>Number(r.requests||0)>0&&(Number(r.errors||0)>0||Number(r.timeouts||0)>0)).length;
  $: active=upstreams.filter((u)=>u.active).length+plainResolvers.filter(plainRecentlyActive).length;
  $: healthy=upstreams.filter((u)=>u.health_status!=='DOWN').length+plainResolvers.length-plainDown;
  $: serverCount=Number($snapshot.upstream_count||0)+plainResolvers.length;
  $: downCount=Number($snapshot.down||0)+plainDown;
  $: degradedCount=Number($snapshot.active_degraded||0)+plainDegraded;
  $: timeoutCount=Number($snapshot.total_timeouts||0)+plainTimeouts;

  const share=(u)=>requests?Number(u.requests||0)/requests*100:0;

  function plainRecentlyActive(r={}) {
    const iso=String(r.last_request||'');
    if(!iso||iso.startsWith('0001-'))return false;
    const ts=new Date(iso).getTime();
    return Number.isFinite(ts)&&Date.now()-ts<5*60*1000;
  }

  function plainStatus(r={}) {
    const req=Number(r.requests||0);
    const res=Number(r.responses||0);
    const err=Number(r.errors||0);
    const timeouts=Number(r.timeouts||0);
    if(req>0&&res===0&&timeouts>0)return {cls:'error',label:'Недоступен'};
    if(req>0&&(err>0||timeouts>0))return {cls:'warn',label:'Деградация'};
    if(plainRecentlyActive(r))return {cls:'good',label:'Активен'};
    return {cls:'neutral',label:'Обнаружен'};
  }

  function plainSuccess(r={}) {
    const req=Number(r.requests||0);
    return req?Number(r.responses||0)/req*100:100;
  }

  async function refreshPlain() {
    try {
      plain=await getPlainDNS(80);
      plainError='';
    } catch(error) {
      plainError=error?.message||'Plain DNS API недоступен';
    }
  }

  onMount(()=>{
    refreshPlain();
    plainTimer=setInterval(()=>{
      if(!document.hidden)refreshPlain();
    },2500);
    return ()=>clearInterval(plainTimer);
  });
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
    <div class="metric-card"><span>DNS серверы</span><strong>{healthy}/{serverCount}</strong><small>{active} активны · DOWN {downCount}</small></div>
    <div class="metric-card"><span>Запросы</span><strong>{fmtInt(requests)}</strong><small>{fmtInt(responses)} ответов</small></div>
    <div class="metric-card"><span>Fallback</span><strong>{fmtInt($snapshot.total_fallbacks||0)}</strong><small>{encryptedRequests?fmtPct(Number($snapshot.total_fallbacks||0)/encryptedRequests*100):'0%'}</small></div>
    <div class="metric-card"><span>Ошибки</span><strong>{fmtInt(errors)}</strong><small>{fmtInt(timeoutCount)} timeout · {degradedCount} degraded</small></div>
  </section>

  {#if !groups.length&&!filteredPlain.length}<section class="panel"><div class="empty">Ничего не найдено</div></section>{/if}

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

  {#if filteredPlain.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Обычный DNS :53</strong><span>{filteredPlain.length} DNS · passive egress</span></div></div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>DNS</th><th>Статус</th><th>Тип</th><th>Трафик</th><th>Latency</th><th>Ошибки</th><th>Ответы</th><th class="expert-only">Port</th><th class="expert-only">Profiles</th></tr></thead>
          <tbody>
            {#each filteredPlain as r (`${r.address}:${r.port||53}`)}
              {@const st=plainStatus(r)}
              {@const pct=share(r)}
              {@const success=plainSuccess(r)}
              <tr>
                <td><div class="cell-title">{r.name||r.address}</div><div class="cell-sub mono">{r.address}:{r.port||53}</div></td>
                <td><span class="state-chip {st.cls}">{st.label}</span><div class="cell-sub">{plainRecentlyActive(r)?'используется сейчас':fmtAgo(r.last_request)}</div></td>
                <td><span class="pill accent">DNS</span></td>
                <td><div class="split-value"><span>{fmtInt(r.requests||0)} req</span><span class="accent-text">{fmtPct(pct)}</span></div><div class="progress"><span style={`width:${Math.min(100,pct)}%`}></span></div></td>
                <td>{#if Number(r.p95_latency_ms||0)}<span class="latency {latencyClass(r.p95_latency_ms)}">p95 {fmtMs(r.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(r.avg_latency_ms)}</div>{:else}—{/if}</td>
                <td><strong class={Number(r.errors||0)+Number(r.timeouts||0)>0?'warn-text':''}>{fmtInt(Number(r.errors||0)+Number(r.timeouts||0))}</strong><div class="cell-sub">{fmtInt(r.timeouts||0)} timeout · {fmtInt(r.nxdomain||0)} NXDOMAIN</div></td>
                <td><strong class={success<95?'warn-text':'good'}>{success.toFixed(1)}%</strong><div class="cell-sub">{fmtInt(r.responses||0)} responses</div></td>
                <td class="expert-only mono">{r.port||53}</td><td class="expert-only">{(r.profiles||[]).join(' · ')||'DHCP / system'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {:else if plainError}
    <section class="panel"><div class="empty">Обычный DNS :53 — {plainError}</div></section>
  {/if}
</div>
