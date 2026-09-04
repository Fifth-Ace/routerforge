<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { fmtInt, groupBy, profileOrder } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let profile='all';
  let minutes=60;

  $: locale = $settings.locale || 'ru';
  $: upstreams=$snapshot.upstreams||[];
  $: profiles=Object.keys(groupBy(upstreams,'profile')).sort(profileOrder);
  $: source=minutes===5?($snapshot.fallback_edges_5m||[]):minutes===1440?($snapshot.fallback_edges_24h||[]):($snapshot.fallback_edges_1h||[]);
  $: edges=source.filter((e)=>profile==='all'||e.from_profile===profile);
  $: profileStats=profiles.map((p)=>({
    name:p,
    requests:upstreams.filter((u)=>u.profile===p).reduce((sum,u)=>sum+Number(u.requests||0),0),
    active:upstreams.filter((u)=>u.profile===p&&u.active).length,
    count:upstreams.filter((u)=>u.profile===p).length
  }));
  $: edgeCount=edges.reduce((sum,e)=>sum+Number(e.count||0),0);
</script>

<svelte:head><title>RouterForge — {t(locale,'dns.routing.pageTitle')}</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head"><div><h1>{t(locale,'dns.routing.pageTitle')}</h1><p>{t(locale,'dns.routing.subtitle')}</p></div><span class="page-kicker mono">POLICY / FALLBACK</span></div>
  <div class="toolbar">
    <select bind:value={profile}><option value="all">{t(locale,'dns.routing.allProfiles')}</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
    <div class="segmented"><button class:active={minutes===5} onclick={()=>minutes=5}>{t(locale,'common.minutesShort',{count:5})}</button><button class:active={minutes===60} onclick={()=>minutes=60}>{t(locale,'common.hourShort')}</button><button class:active={minutes===1440} onclick={()=>minutes=1440}>{t(locale,'common.hours24')}</button></div>
  </div>

  <div class="two-col">
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.routing.fallbackRoutes')}</strong><span>{t(locale,'dns.routing.periodCount',{count:fmtInt(edgeCount,locale)})}</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>{t(locale,'dns.routing.from')}</th><th></th><th>{t(locale,'dns.routing.to')}</th><th>{t(locale,'common.profile')}</th><th>{t(locale,'dns.routing.triggers')}</th></tr></thead><tbody>
        {#if edges.length}
          {#each edges as e (`${e.from_port}-${e.to_port}-${e.from_profile}`)}<tr><td><div class="cell-title">{e.from_upstream}</div><div class="cell-sub mono">:{e.from_port}</div></td><td class="accent-text">→</td><td><div class="cell-title">{e.to_upstream}</div><div class="cell-sub mono">:{e.to_port}</div></td><td>{e.from_profile}</td><td><strong class="good">{fmtInt(e.count,locale)}</strong></td></tr>{/each}
        {:else}<tr><td colspan="5" class="empty">{t(locale,'dns.routing.noFallbacks')}</td></tr>{/if}
      </tbody></table></div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.routing.keeneticProfiles')}</strong><span>{t(locale,'dns.routing.discoveryData')}</span></div></div>
      {#each profileStats as item (item.name)}
        <div class="info-row"><div><strong>{item.name}</strong><span>{t(locale,'dns.routing.profileSummary',{count:item.count,active:item.active})}</span></div><div class="info-value accent-text">{t(locale,'dns.routing.requestsShort',{count:fmtInt(item.requests,locale)})}</div></div>
      {/each}
    </section>
  </div>
</div>
