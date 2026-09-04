<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { getPlainDNS } from '$lib/api.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, errorCount, quality, qualityClass, latencyClass, groupBy, total } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let search='';
  let activeOnly=false;
  let plain={resolvers:[],recent:[],pending:0};
  let plainError='';
  let plainTimer=null;

  $: locale = $settings.locale || 'ru';
  $: allUpstreams=$snapshot.upstreams||[];
  $: systemUpstreams=allUpstreams.filter((u)=>u.profile==='System');
  $: protectedUpstreams=systemUpstreams.length?systemUpstreams:allUpstreams;
  $: policyUpstreams=allUpstreams.filter((u)=>u.profile!=='System');
  $: policyGroups=Object.entries(groupBy(policyUpstreams,'profile')).sort(([a],[b])=>a.localeCompare(b,locale,{numeric:true}));
  $: plainResolvers=[...(plain.resolvers||[])].sort((a,b)=>{
    const ad=String(a.source||'').toUpperCase()==='DHCP'?0:1;
    const bd=String(b.source||'').toUpperCase()==='DHCP'?0:1;
    return ad-bd||String(a.address||'').localeCompare(String(b.address||''),undefined,{numeric:true});
  });

  $: filteredProtected=protectedUpstreams.filter((u)=>{
    if(activeOnly&&!u.active)return false;
    const q=search.trim().toLowerCase();
    return !q||`${u.name} ${u.target} ${u.sni} ${u.domain}`.toLowerCase().includes(q);
  });
  $: filteredPlain=plainResolvers.filter((r)=>{
    if(activeOnly&&!plainRecentlyActive(r))return false;
    const q=search.trim().toLowerCase();
    return !q||`${r.name||''} ${r.address||''} ${r.source||''} ${r.interface||''} ${(r.domains||[]).join(' ')}`.toLowerCase().includes(q);
  });

  $: protectedRequests=total(protectedUpstreams,'requests');
  $: plainRequests=total(plainResolvers,'requests');
  $: requests=protectedRequests+plainRequests;
  $: responses=total(protectedUpstreams,'responses')+total(plainResolvers,'responses');
  $: protectedFallbacks=total(protectedUpstreams,'fallbacks');
  $: plainTimeouts=total(plainResolvers,'timeouts');
  $: protectedTimeouts=total(protectedUpstreams,'timeouts');
  $: errors=protectedUpstreams.reduce((n,u)=>n+errorCount(u)+Number(u.timeouts||0),0)+total(plainResolvers,'errors')+plainTimeouts;
  $: plainDown=plainResolvers.filter((r)=>Number(r.requests||0)>0&&Number(r.responses||0)===0&&Number(r.timeouts||0)>0).length;
  $: protectedDown=protectedUpstreams.filter((u)=>u.health_status==='DOWN').length;
  $: protectedDegraded=protectedUpstreams.filter((u)=>u.health_status==='DEGRADED').length;
  $: active=protectedUpstreams.filter((u)=>u.active).length+plainResolvers.filter(plainRecentlyActive).length;
  $: healthy=protectedUpstreams.length-protectedDown+plainResolvers.length-plainDown;
  $: serverCount=protectedUpstreams.length+plainResolvers.length;
  $: downCount=protectedDown+plainDown;
  $: degradedCount=protectedDegraded+plainResolvers.filter((r)=>Number(r.requests||0)>0&&(Number(r.errors||0)>0||Number(r.timeouts||0)>0)).length;
  $: timeoutCount=protectedTimeouts+plainTimeouts;

  const share=(u)=>requests?Number(u.requests||0)/requests*100:0;

  function plainRecentlyActive(r={}) {
    const iso=String(r.last_request||'');
    if(!iso||iso.startsWith('0001-'))return false;
    const ts=new Date(iso).getTime();
    return Number.isFinite(ts)&&Date.now()-ts<5*60*1000;
  }

  function plainStatus(r={}, lang=locale) {
    const req=Number(r.requests||0), res=Number(r.responses||0), err=Number(r.errors||0), timeouts=Number(r.timeouts||0);
    if(req>0&&res===0&&timeouts>0)return {cls:'error',label:t(lang,'dns.overview.unavailableStatus')};
    if(req>0&&(err>0||timeouts>0))return {cls:'warn',label:t(lang,'dns.overview.degradedStatus')};
    if(plainRecentlyActive(r))return {cls:'good',label:t(lang,'dns.overview.activeStatus')};
    return {cls:'neutral',label:t(lang,'dns.overview.detectedStatus')};
  }

  function plainSuccess(r={}) {
    const req=Number(r.requests||0);
    return req?Number(r.responses||0)/req*100:100;
  }

  function policyRouteState(first={}, lang=locale) {
    if(first.policy_has_default)return {cls:'good',label:t(lang,'dns.overview.hasRoute')};
    return {cls:'neutral',label:t(lang,'dns.overview.noDefaultRoute')};
  }

  async function refreshPlain() {
    try { plain=await getPlainDNS(80); plainError=''; }
    catch(error) { plainError=error?.message||t(locale,'errors.plainDnsUnavailable'); }
  }

  onMount(()=>{
    refreshPlain();
    plainTimer=setInterval(()=>{ if(!document.hidden)refreshPlain(); },2500);
    return ()=>clearInterval(plainTimer);
  });
</script>

<svelte:head><title>RouterForge — {t(locale, 'dns.overview.pageTitle')}</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head">
    <div><h1>DNS</h1><p>{t(locale,'dns.overview.subtitle')}</p></div>
    <span class="page-kicker mono">DNS / OBSERVABILITY</span>
  </div>

  <div class="toolbar">
    <div class="search-control"><span>⌕</span><input bind:value={search} placeholder={t(locale,'dns.overview.searchPlaceholder')}/></div>
    <button class="button" class:active={activeOnly} onclick={()=>activeOnly=true}>{t(locale,'dns.overview.active')}</button>
    <button class="button" class:active={!activeOnly} onclick={()=>activeOnly=false}>{t(locale,'common.all')}</button>
  </div>

  <section class="metric-grid four">
    <div class="metric-card"><span>{t(locale,'dns.overview.servers')}</span><strong>{healthy}/{serverCount}</strong><small>{t(locale,'dns.overview.activeDown',{active,down:downCount})}</small></div>
    <div class="metric-card"><span>{t(locale,'common.requests')}</span><strong>{fmtInt(requests,locale)}</strong><small>{t(locale,'dns.overview.answers',{count:fmtInt(responses,locale)})}</small></div>
    <div class="metric-card"><span>{t(locale,'common.fallback')}</span><strong>{fmtInt(protectedFallbacks,locale)}</strong><small>{protectedRequests?fmtPct(protectedFallbacks/protectedRequests*100):'0%'}</small></div>
    <div class="metric-card"><span>{t(locale,'dns.overview.problems')}</span><strong>{fmtInt(errors,locale)}</strong><small>{t(locale,'dns.overview.timeoutsDegraded',{timeouts:fmtInt(timeoutCount,locale),degraded:degradedCount})}</small></div>
  </section>

  {#if filteredPlain.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.overview.mainDns')}</strong><span>{t(locale,'dns.overview.mainDnsHint')}</span></div></div>
      <div class="table-scroll"><table>
        <thead><tr><th>DNS</th><th>{t(locale,'common.type')}</th><th>{t(locale,'common.status')}</th><th>{t(locale,'common.requests')}</th><th>{t(locale,'common.latency')}</th><th>{t(locale,'common.issues')}</th><th class="advanced-only">{t(locale,'common.responses')}</th><th class="advanced-only">{t(locale,'common.source')}</th><th class="advanced-only">{t(locale,'common.interface')}</th></tr></thead>
        <tbody>
          {#each filteredPlain as r (`${r.address}:${r.port||53}`)}
            {@const st=plainStatus(r,locale)}
            <tr>
              <td><div class="cell-title">{r.name||r.address}</div><div class="cell-sub mono">{r.address}:{r.port||53}</div></td>
              <td><span class="pill accent">DNS</span></td>
              <td><span class="state-chip {st.cls}">{st.label}</span><div class="cell-sub">{plainRecentlyActive(r)?t(locale,'dns.overview.currentUse'):fmtAgo(r.last_request,locale)}</div></td>
              <td><strong>{fmtInt(r.requests||0,locale)}</strong><div class="cell-sub">{t(locale,'dns.overview.trafficShare',{value:fmtPct(share(r))})}</div></td>
              <td>{#if Number(r.p95_latency_ms||0)}<span class="latency {latencyClass(r.p95_latency_ms)}">p95 {fmtMs(r.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(r.avg_latency_ms)}</div>{:else}—{/if}</td>
              <td><strong class={Number(r.errors||0)+Number(r.timeouts||0)>0?'warn-text':''}>{fmtInt(Number(r.errors||0)+Number(r.timeouts||0),locale)}</strong><div class="cell-sub">{fmtInt(r.timeouts||0,locale)} timeout · {fmtInt(r.nxdomain||0,locale)} NXDOMAIN</div></td>
              <td class="advanced-only"><strong class={plainSuccess(r)<95?'warn-text':'good'}>{plainSuccess(r).toFixed(1)}%</strong><div class="cell-sub">{fmtInt(r.responses||0,locale)} responses</div></td>
              <td class="advanced-only">{r.source||((r.profiles||[]).length?t(locale,'dns.overview.profileSource'):'—')}</td>
              <td class="advanced-only">{r.interface||'—'}</td>
            </tr>
          {/each}
        </tbody>
      </table></div>
    </section>
  {:else if plainError}
    <section class="panel"><div class="empty">{t(locale,'dns.overview.mainDnsError',{error:plainError})}</div></section>
  {/if}

  {#if filteredProtected.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.overview.protectedDns')}</strong><span>{t(locale,'dns.overview.protectedDnsHint')}</span></div></div>
      <div class="table-scroll"><table>
        <thead><tr><th>DNS</th><th>{t(locale,'common.type')}</th><th>{t(locale,'common.status')}</th><th>{t(locale,'common.requests')}</th><th>{t(locale,'common.latency')}</th><th>{t(locale,'common.issues')}</th><th class="advanced-only">{t(locale,'common.fallback')}</th><th class="advanced-only">{t(locale,'common.quality')}</th><th class="advanced-only">{t(locale,'common.port')}</th><th class="advanced-only">{t(locale,'common.interface')}</th></tr></thead>
        <tbody>
          {#each filteredProtected as u (u.port)}
            {@const st=statusFor(u,locale)}
            {@const win=u.stats_5m||{}}
            {@const q=quality(u)}
            <tr>
              <td><div class="cell-title">{u.name}</div><div class="cell-sub">{u.target||u.sni||'—'}{u.domain?` · ${u.domain}`:''}</div></td>
              <td><span class="pill accent">{u.protocol}</span></td>
              <td><span class="state-chip {st.cls}">{st.label}</span><div class="cell-sub">{u.active?t(locale,'dns.overview.currentUse'):fmtAgo(u.last_request,locale)}</div></td>
              <td><strong>{fmtInt(u.requests||0,locale)}</strong><div class="cell-sub">{t(locale,'dns.overview.trafficShare',{value:fmtPct(share(u))})}</div></td>
              <td>{#if Number(win.p95_latency_ms||0)}<span class="latency {latencyClass(win.p95_latency_ms)}">p95 {fmtMs(win.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(win.avg_latency_ms)}</div>{:else}—{/if}</td>
              <td><strong class={Number(win.errors||0)+Number(win.timeouts||0)>0?'warn-text':''}>{fmtInt(Number(win.errors||0)+Number(win.timeouts||0),locale)}</strong><div class="cell-sub">{fmtInt(win.timeouts||0,locale)} timeout</div></td>
              <td class="advanced-only"><strong class={Number(win.fallbacks||0)>0?'warn-text':''}>{fmtInt(win.fallbacks||0,locale)}</strong><div class="cell-sub">{fmtPct(win.fallback_pct||0)}</div></td>
              <td class="advanced-only"><strong class={qualityClass(win)}>{q.toFixed(1)}%</strong></td>
              <td class="advanced-only mono">{u.port}</td><td class="advanced-only">{u.interface||'—'}</td>
            </tr>
          {/each}
        </tbody>
      </table></div>
    </section>
  {/if}

  {#if !filteredPlain.length&&!filteredProtected.length&&!plainError}
    <section class="panel"><div class="empty">{t(locale,'dns.overview.notFound')}</div></section>
  {/if}

  {#if $settings.uiLevel==='advanced'&&policyGroups.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.overview.policyContexts')}</strong><span>{t(locale,'dns.overview.policyContextsHint')}</span></div><span class="state-chip info">{t(locale,'common.advanced').toUpperCase()}</span></div>
      <div class="table-scroll"><table>
        <thead><tr><th>{t(locale,'dns.overview.policy')}</th><th>{t(locale,'dns.overview.route')}</th><th>{t(locale,'dns.overview.dnsProxy')}</th><th>{t(locale,'dns.overview.resolvers')}</th><th>{t(locale,'dns.overview.diagnostics')}</th><th>{t(locale,'dns.overview.markTable')}</th></tr></thead>
        <tbody>
          {#each policyGroups as [name,items]}
            {@const first=items[0]||{}}
            {@const route=policyRouteState(first,locale)}
            {@const down=items.filter((u)=>u.health_status==='DOWN').length}
            <tr>
              <td><div class="cell-title">{first.policy_description||name}</div><div class="cell-sub mono">{name}</div></td>
              <td><span class="state-chip {route.cls}">{route.label}</span><div class="cell-sub">{first.policy_has_default?t(locale,'dns.overview.healthChecked'):t(locale,'dns.overview.healthSkipped')}</div></td>
              <td class="mono">:{first.profile_dns_port||'—'}</td>
              <td>{items.length} <span class="cell-sub">DoT/DoH</span></td>
              <td>{#if first.policy_has_default}<span class="state-chip {down?'warn':'good'}">{down?`${down} DOWN`:'OK'}</span>{:else}<span class="state-chip neutral">N/A</span>{/if}</td>
              <td class="mono">0x{Number(first.policy_mark||0).toString(16)} · {first.policy_table||'—'}</td>
            </tr>
          {/each}
        </tbody>
      </table></div>
    </section>
  {/if}
</div>
