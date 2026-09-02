<script>
  import { snapshot } from '$lib/stores/snapshot.js';
  import { fmtInt, groupBy, profileOrder } from '$lib/utils.js';

  let profile='all';
  let minutes=60;

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

<svelte:head><title>DNS Monitor — Маршрутизация</title></svelte:head>

<div class="page">
  <div class="page-head"><div><h1>Маршрутизация</h1><p>Fallback-переходы и распределение запросов между профилями Keenetic.</p></div><span class="page-kicker mono">POLICY / FALLBACK</span></div>
  <div class="toolbar">
    <select bind:value={profile}><option value="all">Все профили</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
    <div class="segmented"><button class:active={minutes===5} onclick={()=>minutes=5}>5 мин</button><button class:active={minutes===60} onclick={()=>minutes=60}>1 час</button><button class:active={minutes===1440} onclick={()=>minutes=1440}>24 часа</button></div>
  </div>

  <div class="two-col">
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Fallback маршруты</strong><span>{fmtInt(edgeCount)} за период</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>Откуда</th><th></th><th>Куда</th><th>Профиль</th><th>Срабатываний</th></tr></thead><tbody>
        {#if edges.length}
          {#each edges as e (`${e.from_port}-${e.to_port}-${e.from_profile}`)}<tr><td><div class="cell-title">{e.from_upstream}</div><div class="cell-sub mono">:{e.from_port}</div></td><td class="accent-text">→</td><td><div class="cell-title">{e.to_upstream}</div><div class="cell-sub mono">:{e.to_port}</div></td><td>{e.from_profile}</td><td><strong class="good">{fmtInt(e.count)}</strong></td></tr>{/each}
        {:else}<tr><td colspan="5" class="empty">Fallback переходов за выбранный период нет</td></tr>{/if}
      </tbody></table></div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><strong>Профили Keenetic</strong><span>Текущие discovery данные</span></div></div>
      {#each profileStats as item (item.name)}
        <div class="info-row"><div><strong>{item.name}</strong><span>{item.count} DNS · {item.active} active</span></div><div class="info-value accent-text">{fmtInt(item.requests)} req</div></div>
      {/each}
    </section>
  </div>
</div>
