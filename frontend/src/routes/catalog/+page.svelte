<script>
  import { catalog, catalogOnline, refreshCatalog } from '$lib/stores/catalog.js';
  import { catalogAction } from '$lib/api.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';
  import RemoveConfirm from '$lib/components/RemoveConfirm.svelte';

  const acronyms = {
    'awg-manager': 'AWG', nfqws2: 'NQ2', nfqws: 'NQ1', 'nfqws-web': 'NQW', 'hydraroute-neo': 'HRN',
    'routerforge-core': 'RFC', dns: 'DNS', marketplace: 'MKT', system: 'SYS', thermal: 'TMP',
    storage: 'DSK', network: 'NET', admin: 'ADM', profiling: 'PRF',
    xkeen: 'XKN', 'xkeen-ui': 'XUI', 'keen-pbr': 'PBR', kvas: 'KVS', 'bypass-keenetic': 'BYP',
    'traffic-via-vpn': 'VPN', 'adguardhome-keenetic': 'AGH', skeen: 'SKN', 'chur-keenetic': 'CHR', 'keenetic-sing-box-ui': 'SBU',
    'keenetic-entware-extras': 'KEE'
  };

  let search = '';
  let statusFilter = 'all';
  let category = 'all';
  let plannerItem = null;
  let removeItem = null;
  let busyId = '';
  let busyAction = '';
  let actionNotice = null;
  let updatingAll = false;

  $: data = $catalog || { modules: [], integrations: [], read_only: true, package_management_enabled: false };
  $: packageMode = Boolean(data.package_management_enabled ?? data['install_test_mode']);
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: all = [...modules, ...integrations];
  $: installedCount = all.filter((item) => item.installed).length;
  $: notInstalledCount = all.length - installedCount;
  $: routerForgeUpdates = modules.filter((item) =>
    item.installed
    && item.actions?.update
    && (item.id === 'routerforge-core' || item.publisher?.id === 'routerforge')
  );
  $: categories = [...new Set(all.map((x) => x.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'));
  $: visibleModules = filterAndSort(modules, statusFilter, category, search);
  $: visibleIntegrations = filterAndSort(integrations, statusFilter, category, search);
  $: sections = [
    { id: 'modules', title: 'Модули RouterForge', subtitle: 'Core и optional-модули платформы. Неустановленные IPK не хранятся на роутере.', items: visibleModules },
    { id: 'integrations', title: 'Сторонние проекты', subtitle: 'Lifecycle приходит из RouterForge Registry и должен соответствовать официальной инструкции разработчика.', items: visibleIntegrations }
  ];

  function filterAndSort(items, selectedStatus, selectedCategory, query) {
    const q = String(query || '').trim().toLowerCase();
    return items
      .filter((item) => {
        if (selectedStatus === 'installed' && !item.installed) return false;
        if (selectedStatus === 'not-installed' && item.installed) return false;
        if (selectedCategory !== 'all' && item.category !== selectedCategory) return false;
        return !q || `${item.name} ${item.category} ${item.description} ${item.version || ''} ${(item.detection?.packages || []).join(' ')} ${item.publisher?.name || ''}`.toLowerCase().includes(q);
      })
      .map((item, index) => ({ item, index }))
      .sort((a, b) => {
        if (Boolean(a.item.installed) !== Boolean(b.item.installed)) return a.item.installed ? -1 : 1;
        return a.index - b.index;
      })
      .map(({ item }) => item);
  }

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
    return hints.length ? hints.join(' · ') : (item.compatibility?.status || 'not evaluated');
  };

  const moduleURL = (item) => {
    if (item.id === 'admin') return '/manage';
    if (item.id === 'dns') return '/dns';
    if (['system', 'thermal', 'storage', 'network', 'profiling'].includes(item.id)) return `/monitoring?tab=${encodeURIComponent(item.id)}`;
    return '';
  };

  const canAction = (item, action) => Boolean(packageMode) && Boolean(item.actions?.[action]);

  const trustLabel = (item) => {
    const value = String(item.trust?.status || 'unverified').toLowerCase();
    if (value === 'official') return 'OFFICIAL';
    if (value === 'verified') return 'VERIFIED';
    if (value === 'changed') return 'CHANGED';
    if (value === 'blocked') return 'BLOCKED';
    if (value === 'deprecated') return 'DEPRECATED';
    return 'UNVERIFIED';
  };

  const trustClass = (item) => {
    const value = String(item.trust?.status || 'unverified').toLowerCase();
    if (value === 'official') return 'official';
    if (value === 'verified') return 'verified';
    if (value === 'changed') return 'warn';
    if (value === 'blocked') return 'error';
    return 'neutral';
  };

  const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  async function runAction(item, action, confirm = '', skipConfirm = false) {
    if (!canAction(item, action) || busyId) return;

    if (!skipConfirm && action === 'install') {
      const source = item.kind === 'module' ? 'GitHub release RouterForge' : 'официальный lifecycle разработчика';
      if (!window.confirm(`Установить ${item.name}?\n\nИсточник: ${source}.`)) return;
    }
    if (!skipConfirm && action === 'update' && !window.confirm(`Обновить ${item.name} по утверждённому release index?`)) return;

    busyId = item.id;
    busyAction = action;
    actionNotice = { cls: 'info', text: `${actionLabel(action)}: ${item.name}…` };

    try {
      const result = await catalogAction(item.id, action, confirm);
      await refreshCatalog();
      actionNotice = { cls: 'good', text: successText(item, action, result) };
      if (action === 'remove') removeItem = null;
    } catch (error) {
      await delay(2500);
      const refreshed = await refreshCatalog();
      const found = [...(refreshed?.modules || []), ...(refreshed?.integrations || [])].find((candidate) => candidate.id === item.id);
      const expectedInstalled = action !== 'remove';
      const expectedVersion = action === 'update' ? item.release?.version : '';
      const installedMatches = found && Boolean(found.installed) === expectedInstalled;
      const versionMatches = !expectedVersion || found?.version === expectedVersion;
      if (installedMatches && versionMatches) {
        actionNotice = { cls: 'good', text: `${item.name}: ${action === 'remove' ? 'удалён' : 'операция завершена'}; runtime перезапустился во время операции.` };
        if (action === 'remove') removeItem = null;
      } else {
        const detail = error?.payload?.detail || error?.payload?.error || error?.message || 'неизвестная ошибка';
        actionNotice = { cls: 'error', text: `${item.name}: операция не выполнена · ${detail}` };
      }
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  const actionLabel = (action) => action === 'install' ? 'Установка' : action === 'update' ? 'Обновление' : 'Удаление';

  function successText(item, action, result) {
    const source = result?.sources?.length ? ` · ${result.sources.join(' · ')}` : '';
    if (action === 'remove') return `${item.name}: удалён`;
    if (action === 'update') return `${item.name}: обновлён${source}`;
    return `${item.name}: установлен${source}`;
  }

  async function updateAllRouterForge() {
    if (updatingAll || busyId || !routerForgeUpdates.length) return;
    if (!window.confirm(`Обновить все доступные компоненты RouterForge (${routerForgeUpdates.length})?\n\nМодули обновятся независимо; Core — последним.`)) return;

    updatingAll = true;
    const ordered = [...routerForgeUpdates].sort((a, b) =>
      a.id === 'routerforge-core' ? 1 : b.id === 'routerforge-core' ? -1 : 0
    );

    try {
      for (const item of ordered) {
        await runAction(item, 'update', '', true);
      }
    } finally {
      updatingAll = false;
      await refreshCatalog();
    }
  }
</script>

<svelte:head><title>RouterForge — Marketplace</title></svelte:head>

<div class="page catalog-page">
  <div class="page-head">
    <div>
      <h1>Marketplace</h1>
      <p>Единый каталог RouterForge: официальные модули платформы и сторонние проекты с явным уровнем доверия.</p>
    </div>
    <span class="state-chip {data.registry?.online ? 'good' : 'warn'}">{data.registry?.online ? 'REGISTRY ONLINE' : `REGISTRY ${(data.registry?.source || 'BUNDLED').toUpperCase()}`}</span>
  </div>

  <div class="toolbar catalog-toolbar-v2">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск модулей, проектов, издателей…"/></div>
    <select bind:value={category}><option value="all">Все категории</option>{#each categories as c}<option value={c}>{c}</option>{/each}</select>
    <button class="button" onclick={refreshCatalog}>↻ Обновить</button>
    {#if packageMode}
      <button
        class="button primary"
        disabled={updatingAll || Boolean(busyId) || !routerForgeUpdates.length}
        onclick={updateAllRouterForge}
      >
        {updatingAll ? 'Обновление…' : `Обновить всё RouterForge${routerForgeUpdates.length ? ` (${routerForgeUpdates.length})` : ''}`}
      </button>
    {/if}
  </div>

  <div class="subtabs catalog-state-filter">
    <button class:active={statusFilter === 'all'} onclick={() => statusFilter = 'all'}>Все <span class="pill">{all.length}</span></button>
    <button class:active={statusFilter === 'installed'} onclick={() => statusFilter = 'installed'}>Установленные <span class="pill">{installedCount}</span></button>
    <button class:active={statusFilter === 'not-installed'} onclick={() => statusFilter = 'not-installed'}>Неустановленные <span class="pill">{notInstalledCount}</span></button>
  </div>

  <div class="market-safety-line mono" class:test-mode={packageMode}>
    {#if packageMode}
      <span><i class="status-dot good"></i> PACKAGE MANAGEMENT <strong>ACTIVE</strong></span>
      <span>CHANNEL <strong>{(data.release?.channel || '—').toUpperCase()}</strong></span>
      <span>OFFICIAL / VERIFIED <strong>EXECUTABLE</strong></span>
      <span>UNVERIFIED / CHANGED <strong>READ ONLY</strong></span>
    {:else}
      <span><i class="status-dot good"></i> CATALOG READ-ONLY</span>
      <span>PACKAGE MANAGEMENT <strong>DISABLED</strong></span>
    {/if}
    <span>REGISTRY <strong>{data.registry?.revision ? data.registry.revision.slice(0, 12) : '—'}</strong></span>
    <span>SOURCE <strong>{data.registry?.source || '—'}</strong></span>
  </div>

  {#if actionNotice}
    <div class="catalog-install-notice {actionNotice.cls}">
      <span class="status-dot {actionNotice.cls}"></span>
      <span>{actionNotice.text}</span>
      <button class="icon-button" aria-label="Закрыть сообщение" onclick={() => actionNotice = null}>×</button>
    </div>
  {/if}

  {#each sections as section (section.id)}
    <section class="catalog-market-section">
      <div class="catalog-section-head">
        <div><h2>{section.title}</h2><p>{section.subtitle}</p></div>
        <span class="state-chip neutral">{section.items.length}</span>
      </div>

      <div class="catalog-grid catalog-grid-v2">
        {#if !section.items.length}<div class="catalog-empty">В этом разделе ничего не найдено</div>{/if}

        {#each section.items as item (item.id)}
          {@const st = stateInfo(item)}
          {@const ownURL = item.kind === 'module' && item.installed ? moduleURL(item) : ''}
          <article class="catalog-card">
            <div>
              <div class="catalog-card-head">
                <div class="catalog-identity">
                  <div class="catalog-icon mono">{acronym(item)}</div>
                  <div><h3>{item.name}</h3><span class="mono">{item.publisher?.name || item.source || 'community'} / {item.category || item.kind}</span></div>
                </div>
                <div class="catalog-state-stack">
                  <span class="state-chip {trustClass(item)}">{trustLabel(item)}</span>
                  <span class="state-chip {st.cls}">{st.label}</span>
                </div>
              </div>

              <p>{item.description || ''}</p>

              <div class="tech-box mono">
                <div><span>Installed</span><strong>{item.version ? `v${item.version}` : '—'}</strong></div>
                <div><span>Available</span><strong class:good={item.update_available}>{item.release?.version ? `v${item.release.version}` : '—'}</strong></div>
                <div><span>Package</span><strong title={packageText(item)}>{packageText(item)}</strong></div>
                <div><span>Publisher</span><strong>{item.publisher?.name || '—'}</strong></div>
                <div><span>Service</span><strong class:good={item.service_running} class:warn={item.service && !item.service_running}>{item.service ? (item.service_running ? 'RUNNING' : 'NOT RUNNING') : item.installed ? 'BUILT-IN / HOOK' : '—'}</strong></div>
                <div><span>Compat</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div>
              </div>

              {#if item.actions?.reason && !item.actions?.install && !item.actions?.update && !item.actions?.remove}
                <div class="catalog-action-reason">{item.actions.reason}</div>
              {/if}
            </div>

            <div class="catalog-card-foot">
              <div class="catalog-actions">
                {#if ownURL}<a class="button primary" href={ownURL}>Открыть модуль</a>{/if}
                {#if item.installed && item.web_port}<a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>Открыть UI :{item.web_port}</a>{/if}

                {#if !item.installed && canAction(item, 'install')}
                  <button class="button primary test-install-button" disabled={Boolean(busyId)} onclick={() => runAction(item, 'install')}>{busyId === item.id && busyAction === 'install' ? 'Установка…' : 'Установить'}</button>
                {/if}
                {#if item.installed && canAction(item, 'update')}
                  <button class="button" disabled={Boolean(busyId)} onclick={() => runAction(item, 'update')}>{busyId === item.id && busyAction === 'update' ? 'Обновление…' : 'Обновить'}</button>
                {/if}
                {#if item.installed && canAction(item, 'remove')}
                  <button class="button danger-subtle" disabled={Boolean(busyId)} onclick={() => removeItem = item}>Удалить</button>
                {/if}

                {#if item.installed || item.install?.method || item.project_url}<button class="button" onclick={() => plannerItem = item}>Подробнее</button>{/if}
                {#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>Проект</a>{/if}
              </div>
              <span class="mono muted">{item.manifest_sha256 ? `manifest ${item.manifest_sha256.slice(0, 8)}` : item.kind}</span>
            </div>
          </article>
        {/each}
      </div>
    </section>
  {/each}

  <div class="catalog-inline-footer mono">
    <span>Registry: {data.registry?.id || 'routerforge-community'} · {data.registry?.source || '—'}</span>
    <span><strong>{visibleModules.length + visibleIntegrations.length}</strong> visible / <strong>{all.length}</strong> total</span>
  </div>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
{#if removeItem}<RemoveConfirm item={removeItem} busy={busyId === removeItem.id && busyAction === 'remove'} oncancel={() => { if (!busyId) removeItem = null; }} onconfirm={(typed) => runAction(removeItem, 'remove', typed)}/>{/if}
