<script>
  import { catalog, catalogOnline, refreshCatalog } from '$lib/stores/catalog.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';

  const acronyms = {
    'awg-manager': 'AWG', nfqws2: 'NQ2', nfqws: 'NQ1', 'hydraroute-neo': 'HRN',
    'dns-core': 'DNS', marketplace: 'MKT', system: 'SYS', thermal: 'TMP',
    storage: 'DSK', network: 'NET', admin: 'ADM', profiling: 'PRF'
  };

  let search = '';
  let kind = 'all';
  let category = 'all';
  let plannerItem = null;

  $: data = $catalog || { modules: [], integrations: [], read_only: true };
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: all = [...integrations, ...modules];
  $: categories = [...new Set(all.map((x) => x.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'));
  $: items = all.filter((item) => {
    if (kind !== 'all' && item.kind !== kind) return false;
    if (category !== 'all' && item.category !== category) return false;
    const q = search.trim().toLowerCase();
    return !q || `${item.name} ${item.category} ${item.description} ${item.version || ''} ${(item.detection?.packages || []).join(' ')}`.toLowerCase().includes(q);
  });

  const acronym = (item) => acronyms[item.id]
    || String(item.name || 'EXT').replace(/[^A-Za-z0-9]/g, '').slice(0, 3).toUpperCase()
    || 'EXT';

  const packageText = (item) => {
    const packages = item.install?.packages || item.detection?.packages || [];
    return packages.length ? packages.join(', ') : '—';
  };

  const compatibilityText = (item) => {
    const hints = item.compatibility?.hints || [];
    if (item.compatibility?.status === 'built-in') return 'built-in';
    if (item.compatibility?.status === 'planned') return 'planned';
    return hints.length ? hints.join(' · ') : (item.compatibility?.status || 'not evaluated');
  };
</script>

<svelte:head><title>DNS Monitor — Каталог</title></svelte:head>

<div class="page catalog-page">
  <div class="page-head">
    <div>
      <h1>Marketplace</h1>
      <p>Модули DNS Monitor и обнаруженные сторонние системы на роутере.</p>
    </div>
    <span class="state-chip {$catalogOnline ? 'good' : 'warn'}">{$catalogOnline ? 'REGISTRY ONLINE' : 'REGISTRY OFFLINE'}</span>
  </div>

  <div class="toolbar catalog-toolbar-v2">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск модулей, интеграций, пакетов…"/></div>
    <select bind:value={kind}><option value="all">Всё</option><option value="module">Модули</option><option value="integration">Интеграции</option></select>
    <select bind:value={category}><option value="all">Все категории</option>{#each categories as c}<option value={c}>{c}</option>{/each}</select>
    <button class="button" onclick={refreshCatalog}>↻ Обновить</button>
  </div>

  <div class="catalog-grid catalog-grid-v2">
    {#if !items.length}<div class="catalog-empty">Ничего не найдено</div>{/if}

    {#each items as item (item.id)}
      {@const st = stateInfo(item)}
      <article class="catalog-card">
        <div>
          <div class="catalog-card-head">
            <div class="catalog-identity">
              <div class="catalog-icon mono">{acronym(item)}</div>
              <div><h3>{item.name}</h3><span class="mono">{item.source || 'dns-monitor'} / {item.category || item.kind}</span></div>
            </div>
            <span class="state-chip {st.cls}">{st.label}</span>
          </div>

          <p>{item.description || ''}</p>

          <div class="tech-box mono">
            <div><span>Version</span><strong>{item.version ? `v${item.version}` : '—'}</strong></div>
            <div><span>Package</span><strong title={packageText(item)}>{packageText(item)}</strong></div>
            <div><span>Service</span><strong class:good={item.service_running} class:warn={item.service && !item.service_running}>{item.service ? (item.service_running ? 'RUNNING' : 'NOT RUNNING') : '—'}</strong></div>
            <div><span>Compat</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div>
          </div>
        </div>

        <div class="catalog-card-foot">
          <div class="catalog-actions">
            {#if item.id === 'admin' && item.installed}
              <a class="button primary" href="/admin">Открыть Admin</a>
            {/if}
            {#if item.installed && item.web_port}
              <a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>Открыть UI :{item.web_port}</a>
            {/if}
            <button class="button" class:primary={!item.installed} onclick={() => plannerItem = item}>{item.installed ? 'Подробнее' : 'План установки'}</button>
            {#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>Проект</a>{/if}
          </div>
          <span class="mono muted">{item.kind || 'extension'}</span>
        </div>
      </article>
    {/each}
  </div>

  <div class="catalog-inline-footer mono">
    <span>Generated: {data.generated_at ? new Date(data.generated_at).toLocaleTimeString('ru-RU') : '—'}</span>
    <span><strong>{items.length}</strong> visible / <strong>{all.length}</strong> total</span>
  </div>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
