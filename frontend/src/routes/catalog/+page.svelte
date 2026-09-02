<script>
  import { onMount } from 'svelte';
  import { getCatalog } from '$lib/api.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';

  const acronyms={'awg-manager':'AWG',nfqws2:'NQ2',nfqws:'NQ1','hydraroute-neo':'HRN','dns-core':'DNS',marketplace:'MKT',system:'SYS',thermal:'TMP',storage:'DSK',network:'NET',admin:'ADM',profiling:'PRF'};
  let data={modules:[],integrations:[],read_only:true},search='',kind='all',category='all',plannerItem=null,loading=true;

  $: modules=data.modules||[];
  $: integrations=data.integrations||[];
  $: all=[...integrations,...modules];
  $: categories=[...new Set(all.map((x)=>x.category).filter(Boolean))].sort((a,b)=>a.localeCompare(b,'ru'));
  $: items=all.filter((item)=>{
    if(kind!=='all'&&item.kind!==kind)return false;
    if(category!=='all'&&item.category!==category)return false;
    const q=search.trim().toLowerCase();
    return !q||`${item.name} ${item.category} ${item.description} ${item.version||''} ${(item.detection?.packages||[]).join(' ')}`.toLowerCase().includes(q);
  });
  $: externalInstalled=integrations.filter((x)=>x.state==='installed_external').length;
  $: activeServices=integrations.filter((x)=>x.service_running).length;
  $: available=integrations.filter((x)=>x.state==='available').length;

  const acronym=(item)=>acronyms[item.id]||String(item.name||'EXT').replace(/[^A-Za-z0-9]/g,'').slice(0,3).toUpperCase()||'EXT';
  const packageText=(item)=>{const packages=item.install?.packages||item.detection?.packages||[];return packages.length?packages.join(', '):'—';};
  const compatibilityText=(item)=>{const hints=item.compatibility?.hints||[];if(item.compatibility?.status==='built-in')return'built-in';if(item.compatibility?.status==='planned')return'planned';return hints.length?hints.join(' · '):(item.compatibility?.status||'not evaluated');};

  async function load(){try{data=await getCatalog();}finally{loading=false;}}
  onMount(()=>{load();const timer=setInterval(load,15000);return()=>clearInterval(timer);});
</script>

<svelte:head><title>DNS Monitor — Каталог</title></svelte:head>

<div class="catalog-shell">
  <aside class="catalog-sidebar">
    <div>
      <div class="section-label">Core Modules Tree</div>
      <div class="module-tree mono"><div class="tree-root"><span class="status-dot good"></span><strong>DNS Monitor Core</strong></div>{#each modules as module,index (module.id)}{@const st=stateInfo(module)}<div class="tree-row" class:loaded={module.state==='installed'}><span>{index===modules.length-1?'└──':'├──'}</span><span class="status-dot {st.cls}"></span><span class="tree-name">{module.name}</span><span class="tree-state">[{st.label}]</span></div>{/each}</div>
      <div class="catalog-side-section"><div class="section-label">Registry Summary</div><div class="side-stat"><span>External installed</span><strong>{externalInstalled}</strong></div><div class="side-stat"><span>Services running</span><strong>{activeServices}</strong></div><div class="side-stat"><span>Available</span><strong>{available}</strong></div><div class="side-stat"><span>Catalog entries</span><strong>{all.length}</strong></div></div>
    </div>
    <div class="safety-panel mono"><div class="section-label">Marketplace Safety</div><div><span>Catalog API</span><strong class="good">ONLINE</strong></div><div><span>Mutation API</span><strong>DISABLED</strong></div><div><span>Mode</span><strong>{data.read_only?'READ-ONLY':'UNKNOWN'}</strong></div></div>
  </aside>

  <section class="catalog-main">
    <div class="catalog-toolbar"><div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск модулей, интеграций, пакетов…"/></div><div class="catalog-controls"><select bind:value={kind}><option value="all">Всё</option><option value="module">Модули</option><option value="integration">Интеграции</option></select><select bind:value={category}><option value="all">Все категории</option>{#each categories as c}<option value={c}>{c}</option>{/each}</select><span class="repo-state mono"><i></i>{loading?'SYNCING':'REGISTRY ONLINE'}</span></div></div>

    <div class="catalog-content">
      <div class="page-head"><div><h1>Marketplace</h1><p>Модули DNS Monitor и обнаруженные сторонние системы на роутере.</p></div><span class="state-chip info">READ-ONLY FOUNDATION</span></div>
      <div class="catalog-grid">
        {#if !items.length}<div class="catalog-empty">Ничего не найдено</div>{/if}
        {#each items as item (item.id)}
          {@const st=stateInfo(item)}
          <article class="catalog-card">
            <div><div class="catalog-card-head"><div class="catalog-identity"><div class="catalog-icon mono">{acronym(item)}</div><div><h3>{item.name}</h3><span class="mono">{item.source||'dns-monitor'} / {item.category||item.kind}</span></div></div><span class="state-chip {st.cls}">{st.label}</span></div><p>{item.description||''}</p><div class="tech-box mono"><div><span>Version</span><strong>{item.version?`v${item.version}`:'—'}</strong></div><div><span>Package</span><strong title={packageText(item)}>{packageText(item)}</strong></div><div><span>Service</span><strong class:good={item.service_running} class:warn={item.service&&!item.service_running}>{item.service?(item.service_running?'RUNNING':'NOT RUNNING'):'—'}</strong></div><div><span>Compat</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div></div></div>
            <div class="catalog-card-foot"><div class="catalog-actions">{#if item.installed&&item.web_port}<a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>Открыть UI :{item.web_port}</a>{/if}<button class="button" class:primary={!item.installed} onclick={()=>plannerItem=item}>{item.installed?'Подробнее':'План установки'}</button>{#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>Проект</a>{/if}</div><span class="mono muted">{item.kind||'extension'}</span></div>
          </article>
        {/each}
      </div>
    </div>
    <footer class="catalog-footer mono"><span>Generated: {data.generated_at?new Date(data.generated_at).toLocaleTimeString('ru-RU'):'—'}</span><span><strong>{items.length}</strong> visible / <strong>{all.length}</strong> total</span></footer>
  </section>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={()=>plannerItem=null}/>{/if}
