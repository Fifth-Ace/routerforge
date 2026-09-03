<script>
  import { catalog, catalogOnline, refreshCatalog } from '$lib/stores/catalog.js';
  import { installCatalogItem } from '$lib/api.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';

  const acronyms = {
    'awg-manager': 'AWG', nfqws2: 'NQ2', nfqws: 'NQ1', 'nfqws-web': 'NQW', 'hydraroute-neo': 'HRN',
    'dns-core': 'DNS', marketplace: 'MKT', system: 'SYS', thermal: 'TMP',
    storage: 'DSK', network: 'NET', admin: 'ADM', profiling: 'PRF',
    xkeen: 'XKN', 'xkeen-ui': 'XUI', 'keen-pbr': 'PBR', kvas: 'KVS', 'bypass-keenetic': 'BYP',
    'traffic-via-vpn': 'VPN', 'adguardhome-keenetic': 'AGH', skeen: 'SKN', 'chur-keenetic': 'CHR', 'keenetic-sing-box-ui': 'SBU',
    'keenetic-entware-extras': 'KEE'
  };

  let search = '';
  let statusFilter = 'all';
  let category = 'all';
  let plannerItem = null;
  let installingId = '';
  let installNotice = null;

  $: data = $catalog || { modules: [], integrations: [], read_only: true, install_test_mode: false };
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: all = [...modules, ...integrations];
  $: installedCount = all.filter((item) => item.installed).length;
  $: notInstalledCount = all.length - installedCount;
  $: categories = [...new Set(all.map((x) => x.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'));
  $: visibleModules = filterAndSort(modules, statusFilter, category, search);
  $: visibleIntegrations = filterAndSort(integrations, statusFilter, category, search);
  $: sections = [
    {
      id: 'modules',
      title: 'Модули DNS Monitor',
      subtitle: 'Core, Marketplace и optional-модули проекта.',
      items: visibleModules
    },
    {
      id: 'integrations',
      title: 'Сторонние проекты',
      subtitle: 'Обнаружение и интеграции Keenetic / Netcraze / Entware. Автоустановка стороннего кода отключена.',
      items: visibleIntegrations
    }
  ];

  function filterAndSort(items, selectedStatus, selectedCategory, query) {
    const q = String(query || '').trim().toLowerCase();

    return items
      .filter((item) => {
        if (selectedStatus === 'installed' && !item.installed) return false;
        if (selectedStatus === 'not-installed' && item.installed) return false;
        if (selectedCategory !== 'all' && item.category !== selectedCategory) return false;

        return !q || `${item.name} ${item.category} ${item.description} ${item.version || ''} ${(item.detection?.packages || []).join(' ')}`
          .toLowerCase()
          .includes(q);
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
    if (item.id === 'admin') return '/admin';
    if (['system', 'thermal', 'storage', 'network', 'profiling'].includes(item.id)) {
      return `/modules?tab=${encodeURIComponent(item.id)}`;
    }
    return '';
  };

  const canTestInstall = (item) =>
    Boolean(data.install_test_mode)
    && item.kind === 'module'
    && item.managed
    && !item.installed
    && item.install?.method === 'opkg-feed'
    && item.install?.repository === 'dns-monitor';

  const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  async function installItem(item) {
    if (!canTestInstall(item) || installingId) return;
    const packages = item.install?.packages || [];
    const confirmed = window.confirm(
      `TEST MODE\n\nУстановить ${item.name}${packages.length ? ` (${packages.join(', ')})` : ''}?\n\nCore выполнит только allowlisted opkg install для модуля DNS Monitor.`
    );
    if (!confirmed) return;

    installingId = item.id;
    installNotice = { cls: 'info', text: `Устанавливаем ${item.name}…` };

    try {
      const result = await installCatalogItem(item.id);
      await refreshCatalog();
      installNotice = {
        cls: 'good',
        text: `${item.name}: установлено${result.sources?.length ? ` · ${result.sources.join(' · ')}` : ''}`
      };
    } catch (error) {
      // Profiling package restarts Core in postinst, so the HTTP request may be
      // interrupted even when installation itself succeeded. Re-check catalog.
      await delay(2500);
      const refreshed = await refreshCatalog();
      const found = [...(refreshed?.modules || [])].find((candidate) => candidate.id === item.id);
      if (found?.installed) {
        installNotice = { cls: 'good', text: `${item.name}: установлено; Core перезапустился во время операции.` };
      } else {
        const detail = error?.payload?.detail || error?.payload?.error || error?.message || 'неизвестная ошибка';
        installNotice = { cls: 'error', text: `${item.name}: установка не выполнена · ${detail}` };
      }
    } finally {
      installingId = '';
    }
  }
</script>

<svelte:head><title>DNS Monitor — Каталог</title></svelte:head>

<div class="page catalog-page">
  <div class="page-head">
    <div>
      <h1>Marketplace</h1>
      <p>Модули DNS Monitor отдельно от сторонних проектов. Установленные элементы всегда показываются первыми.</p>
    </div>
    <span class="state-chip {$catalogOnline ? 'good' : 'warn'}">{$catalogOnline ? 'REGISTRY ONLINE' : 'REGISTRY OFFLINE'}</span>
  </div>

  <div class="toolbar catalog-toolbar-v2">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск модулей, проектов, пакетов…"/></div>
    <select bind:value={category}><option value="all">Все категории</option>{#each categories as c}<option value={c}>{c}</option>{/each}</select>
    <button class="button" onclick={refreshCatalog}>↻ Обновить</button>
  </div>

  <div class="subtabs catalog-state-filter">
    <button class:active={statusFilter === 'all'} onclick={() => statusFilter = 'all'}>Все <span class="pill">{all.length}</span></button>
    <button class:active={statusFilter === 'installed'} onclick={() => statusFilter = 'installed'}>Установленные <span class="pill">{installedCount}</span></button>
    <button class:active={statusFilter === 'not-installed'} onclick={() => statusFilter = 'not-installed'}>Неустановленные <span class="pill">{notInstalledCount}</span></button>
  </div>

  <div class="market-safety-line mono" class:test-mode={data.install_test_mode}>
    {#if data.install_test_mode}
      <span><i class="status-dot warn"></i> INSTALL TEST MODE <strong>ACTIVE</strong></span>
      <span>DNS MONITOR MODULES <strong>ALLOWLISTED</strong></span>
      <span>THIRD-PARTY EXECUTION <strong>DISABLED</strong></span>
    {:else}
      <span><i class="status-dot good"></i> CATALOG READ-ONLY</span>
      <span>DNS MONITOR INSTALL <strong>DISABLED</strong></span>
      <span>THIRD-PARTY EXECUTION <strong>DISABLED</strong></span>
    {/if}
    <span>PHASE <strong>{data.phase || '—'}</strong></span>
  </div>

  {#if installNotice}
    <div class="catalog-install-notice {installNotice.cls}">
      <span class="status-dot {installNotice.cls}"></span>
      <span>{installNotice.text}</span>
      <button class="icon-button" aria-label="Закрыть сообщение" onclick={() => installNotice = null}>×</button>
    </div>
  {/if}

  {#each sections as section (section.id)}
    <section class="catalog-market-section">
      <div class="catalog-section-head">
        <div>
          <h2>{section.title}</h2>
          <p>{section.subtitle}</p>
        </div>
        <span class="state-chip neutral">{section.items.length}</span>
      </div>

      <div class="catalog-grid catalog-grid-v2">
        {#if !section.items.length}
          <div class="catalog-empty">В этом разделе ничего не найдено</div>
        {/if}

        {#each section.items as item (item.id)}
          {@const st = stateInfo(item)}
          {@const ownURL = item.kind === 'module' && item.installed ? moduleURL(item) : ''}
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
                <div><span>Service</span><strong class:good={item.service_running} class:warn={item.service && !item.service_running}>{item.service ? (item.service_running ? 'RUNNING' : 'NOT RUNNING') : item.installed ? 'BUILT-IN / HOOK' : '—'}</strong></div>
                <div><span>Compat</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div>
              </div>
            </div>

            <div class="catalog-card-foot">
              <div class="catalog-actions">
                {#if ownURL}<a class="button primary" href={ownURL}>Открыть модуль</a>{/if}
                {#if item.installed && item.web_port}
                  <a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>Открыть UI :{item.web_port}</a>
                {/if}

                {#if canTestInstall(item)}
                  <button class="button primary test-install-button" disabled={Boolean(installingId)} onclick={() => installItem(item)}>
                    {installingId === item.id ? 'Установка…' : 'Установить (TEST)'}
                  </button>
                {:else if item.installed || item.install?.method || item.project_url}
                  <button class="button" onclick={() => plannerItem = item}>{item.installed ? 'Подробнее' : 'План установки'}</button>
                {/if}

                {#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>Проект</a>{/if}
              </div>
              <span class="mono muted">{item.kind === 'module' ? 'dns-monitor' : 'third-party'}</span>
            </div>
          </article>
        {/each}
      </div>
    </section>
  {/each}

  <div class="catalog-inline-footer mono">
    <span>Generated: {data.generated_at ? new Date(data.generated_at).toLocaleTimeString('ru-RU') : '—'}</span>
    <span><strong>{visibleModules.length + visibleIntegrations.length}</strong> visible / <strong>{all.length}</strong> total</span>
  </div>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
