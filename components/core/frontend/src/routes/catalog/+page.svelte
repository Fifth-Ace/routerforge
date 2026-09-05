<script>
  import { catalog, refreshCatalog, forceRefreshCatalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { catalogAction } from '$lib/api.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
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
  let checkingUpdates = false;

  $: locale = $settings.locale || 'ru';
  $: data = $catalog || { modules: [], integrations: [], read_only: true, package_management_enabled: false };
  $: packageMode = Boolean(data.package_management_enabled ?? data['install_test_mode']);
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: all = [...modules, ...integrations];
  $: installedCount = all.filter((item) => item.installed).length;
  $: notInstalledCount = all.length - installedCount;
  const hasVerifiedUpdate = (item) => Boolean(item?.installed && item?.update_available && item?.actions?.update);
  $: availableUpdates = all.filter(hasVerifiedUpdate);
  $: routerForgeUpdates = modules.filter((item) => hasVerifiedUpdate(item) && (item.id === 'routerforge-core' || item.publisher?.id === 'routerforge'));
  $: categories = [...new Set(all.map((x) => x.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, locale));
  $: visibleModules = filterAndSort(modules, statusFilter, category, search);
  $: visibleIntegrations = filterAndSort(integrations, statusFilter, category, search);
  $: sections = [
    { id: 'modules', title: t(locale,'marketplace.sections.modules'), subtitle: t(locale,'marketplace.sections.modulesSubtitle'), items: visibleModules },
    { id: 'integrations', title: t(locale,'marketplace.sections.integrations'), subtitle: t(locale,'marketplace.sections.integrationsSubtitle'), items: visibleIntegrations }
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
      .sort((a, b) => Boolean(a.item.installed) !== Boolean(b.item.installed) ? (a.item.installed ? -1 : 1) : a.index - b.index)
      .map(({ item }) => item);
  }

  const acronym = (item) => acronyms[item.id] || String(item.name || 'EXT').replace(/[^A-Za-z0-9]/g, '').slice(0, 3).toUpperCase() || 'EXT';
  const packageText = (item) => {
    const packages = item.install?.packages || item.detection?.packages || [];
    return packages.length ? packages.join(', ') : '—';
  };
  const compatibilityText = (item, lang) => {
    const hints = item.compatibility?.hints || [];
    if (item.compatibility?.status === 'built-in') return t(lang,'marketplace.compatBuiltIn');
    return hints.length ? hints.join(' · ') : (item.compatibility?.status || t(lang,'marketplace.compatNotEvaluated'));
  };
  const moduleURL = (item) => {
    if (item.id === 'admin') return '/manage';
    if (item.id === 'dns') return '/dns';
    if (['system', 'thermal', 'storage', 'network', 'profiling'].includes(item.id)) return `/monitoring?tab=${encodeURIComponent(item.id)}`;
    return '';
  };
  const canAction = (item, action) => Boolean(packageMode) && Boolean(item.actions?.[action]);

  const trustLabel = (item, lang) => {
    const value = String(item.trust?.status || 'unverified').toLowerCase();
    const key = ['official','verified','changed','blocked','deprecated'].includes(value) ? value : 'unverified';
    return t(lang,`marketplace.trust.${key}`);
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
  const actionLabel = (action) => t(locale, `marketplace.action.${action}`);

  async function runAction(item, action, confirm = '', skipConfirm = false) {
    if (!canAction(item, action) || busyId) return;

    if (!skipConfirm && action === 'install') {
      const source = item.kind === 'module' ? t(locale,'marketplace.action.sourceRouterForge') : t(locale,'marketplace.action.sourceOfficial');
      if (!window.confirm(t(locale,'marketplace.action.confirmInstall',{name:item.name,source}))) return;
    }
    if (!skipConfirm && action === 'update' && !window.confirm(t(locale,'marketplace.action.confirmUpdate',{name:item.name}))) return;

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
        const result = action === 'remove' ? t(locale,'marketplace.action.operationRemoved') : t(locale,'marketplace.action.operationFinished');
        actionNotice = { cls: 'good', text: t(locale,'marketplace.action.runtimeRestarted',{name:item.name,result}) };
        if (action === 'remove') removeItem = null;
      } else {
        const detail = error?.payload?.detail || error?.payload?.error || error?.message || t(locale,'errors.unknown');
        actionNotice = { cls: 'error', text: t(locale,'marketplace.action.failed',{name:item.name,detail}) };
      }
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  function successText(item, action, result) {
    const source = result?.sources?.length ? ` · ${result.sources.join(' · ')}` : '';
    if (action === 'remove') return t(locale,'marketplace.action.removed',{name:item.name});
    if (action === 'update') return t(locale,'marketplace.action.updated',{name:item.name,source});
    return t(locale,'marketplace.action.installed',{name:item.name,source});
  }

  async function checkForUpdates() {
    if (checkingUpdates || busyId) return;
    checkingUpdates = true;
    actionNotice = { cls: 'info', text: t(locale,'marketplace.action.checkingRegistry') };
    try {
      const result = await forceRefreshCatalog();
      const freshItems = [...(result?.catalog?.modules || []), ...(result?.catalog?.integrations || [])];
      const updates = freshItems.filter(hasVerifiedUpdate);
      actionNotice = updates.length
        ? { cls: 'good', text: t(locale,'marketplace.action.updatesAvailable',{count:updates.length}) }
        : { cls: 'info', text: t(locale,'marketplace.action.noUpdates') };
    } catch (error) {
      const detail = error?.payload?.detail || error?.payload?.error || error?.message || t(locale,'errors.unknown');
      actionNotice = { cls: 'error', text: t(locale,'marketplace.action.checkFailed',{detail}) };
    } finally {
      checkingUpdates = false;
    }
  }

  async function updateAllRouterForge() {
    if (updatingAll || busyId || !routerForgeUpdates.length) return;
    if (!window.confirm(t(locale,'marketplace.action.confirmUpdateAll',{count:routerForgeUpdates.length}))) return;

    updatingAll = true;
    const ordered = [...routerForgeUpdates].sort((a, b) => a.id === 'routerforge-core' ? 1 : b.id === 'routerforge-core' ? -1 : 0);
    try {
      for (const item of ordered) await runAction(item, 'update', '', true);
    } finally {
      updatingAll = false;
      await refreshCatalog();
    }
  }
</script>

<svelte:head><title>RouterForge — {t(locale,'marketplace.pageTitle')}</title></svelte:head>

<div class="page catalog-page">
  <div class="page-head">
    <div><h1>Marketplace</h1><p>{t(locale,'marketplace.subtitle')}</p></div>
    <span class="state-chip {data.registry?.online ? 'good' : 'warn'}">{data.registry?.online ? t(locale,'marketplace.registryOnline') : `REGISTRY ${(data.registry?.source || 'BUNDLED').toUpperCase()}`}</span>
  </div>

  <div class="toolbar catalog-toolbar-v2">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder={t(locale,'marketplace.searchPlaceholder')}/></div>
    <select bind:value={category}><option value="all">{t(locale,'marketplace.allCategories')}</option>{#each categories as c}<option value={c}>{c}</option>{/each}</select>
    <button class="button catalog-refresh-button" class:update-available={availableUpdates.length > 0} disabled={checkingUpdates || Boolean(busyId)} onclick={checkForUpdates}>
      {checkingUpdates ? t(locale,'marketplace.checking') : availableUpdates.length ? t(locale,'marketplace.updates',{count:availableUpdates.length}) : t(locale,'marketplace.checkUpdates')}
    </button>
    {#if packageMode}
      <button class="button primary" disabled={updatingAll || Boolean(busyId) || !routerForgeUpdates.length} onclick={updateAllRouterForge}>
        {updatingAll ? t(locale,'common.updating') : t(locale,'marketplace.updateAll',{count:routerForgeUpdates.length ? ` (${routerForgeUpdates.length})` : ''})}
      </button>
    {/if}
  </div>

  <div class="subtabs catalog-state-filter">
    <button class:active={statusFilter === 'all'} onclick={() => statusFilter = 'all'}>{t(locale,'marketplace.filters.all')} <span class="pill">{all.length}</span></button>
    <button class:active={statusFilter === 'installed'} onclick={() => statusFilter = 'installed'}>{t(locale,'marketplace.filters.installed')} <span class="pill">{installedCount}</span></button>
    <button class:active={statusFilter === 'not-installed'} onclick={() => statusFilter = 'not-installed'}>{t(locale,'marketplace.filters.notInstalled')} <span class="pill">{notInstalledCount}</span></button>
  </div>

  <div class="market-safety-line mono" class:test-mode={packageMode}>
    {#if packageMode}
      <span><i class="status-dot good"></i> {t(locale,'marketplace.safety.packageManagement')} <strong>{t(locale,'marketplace.safety.active')}</strong></span>
      <span>{t(locale,'marketplace.safety.channel')} <strong>{(data.release?.channel || '—').toUpperCase()}</strong></span>
      <span>{t(locale,'marketplace.safety.officialVerified')} <strong>{t(locale,'marketplace.safety.executable')}</strong></span>
      <span>{t(locale,'marketplace.safety.unverifiedChanged')} <strong>{t(locale,'marketplace.safety.readOnly')}</strong></span>
    {:else}
      <span><i class="status-dot good"></i> {t(locale,'marketplace.safety.catalogReadOnly')}</span>
      <span>{t(locale,'marketplace.safety.packageManagement')} <strong>{t(locale,'marketplace.safety.disabled')}</strong></span>
    {/if}
    <span>{t(locale,'marketplace.safety.registry')} <strong>{data.registry?.revision ? data.registry.revision.slice(0, 12) : '—'}</strong></span>
    <span>{t(locale,'marketplace.safety.source')} <strong>{data.registry?.source || '—'}</strong></span>
  </div>

  {#if actionNotice}
    <div class="catalog-install-notice {actionNotice.cls}">
      <span class="status-dot {actionNotice.cls}"></span><span>{actionNotice.text}</span>
      <button class="icon-button" aria-label={t(locale,'marketplace.closeNotice')} onclick={() => actionNotice = null}>×</button>
    </div>
  {/if}

  {#each sections as section (section.id)}
    <section class="catalog-market-section">
      <div class="catalog-section-head"><div><h2>{section.title}</h2><p>{section.subtitle}</p></div><span class="state-chip neutral">{section.items.length}</span></div>
      <div class="catalog-grid catalog-grid-v2">
        {#if !section.items.length}<div class="catalog-empty">{t(locale,'marketplace.empty')}</div>{/if}

        {#each section.items as item (item.id)}
          {@const st = stateInfo(item,locale)}
          {@const ownURL = item.kind === 'module' && item.installed ? moduleURL(item) : ''}
          <article class="catalog-card">
            <div>
              <div class="catalog-card-head">
                <div class="catalog-identity">
                  <div class="catalog-icon mono">{acronym(item)}</div>
                  <div><h3>{item.name}</h3><span class="mono">{item.publisher?.name || item.source || 'community'} / {item.category || item.kind}</span></div>
                </div>
                <div class="catalog-state-stack"><span class="state-chip {trustClass(item)}">{trustLabel(item,locale)}</span><span class="state-chip {st.cls}">{st.label}</span></div>
              </div>

              <p>{item.description || ''}</p>

              <div class="tech-box mono">
                <div><span>{t(locale,'marketplace.tech.installed')}</span><strong>{item.version ? `v${item.version}` : '—'}</strong></div>
                <div><span>{t(locale,'marketplace.tech.available')}</span><strong class:good={item.update_available}>{item.release?.version ? `v${item.release.version}` : '—'}</strong></div>
                <div><span>{t(locale,'marketplace.tech.package')}</span><strong title={packageText(item)}>{packageText(item)}</strong></div>
                <div><span>{t(locale,'marketplace.tech.publisher')}</span><strong>{item.publisher?.name || '—'}</strong></div>
                <div><span>{t(locale,'marketplace.tech.service')}</span><strong class:good={item.service_running} class:warn={item.service && !item.service_running}>{item.service ? (item.service_running ? t(locale,'marketplace.tech.running') : t(locale,'marketplace.tech.notRunning')) : item.installed ? t(locale,'marketplace.tech.builtInHook') : '—'}</strong></div>
                <div><span>{t(locale,'marketplace.tech.compat')}</span><strong title={compatibilityText(item,locale)}>{compatibilityText(item,locale)}</strong></div>
              </div>

              {#if item.actions?.reason && !item.actions?.install && !item.actions?.update && !item.actions?.remove}
                <div class="catalog-action-reason">{item.actions.reason}</div>
              {/if}
            </div>

            <div class="catalog-card-foot">
              <div class="catalog-actions">
                {#if ownURL}<a class="button primary" href={ownURL}>{t(locale,'marketplace.openModule')}</a>{/if}
                {#if item.installed && item.web_port}<a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>{t(locale,'marketplace.openUi',{port:item.web_port})}</a>{/if}
                {#if !item.installed && canAction(item, 'install')}<button class="button primary test-install-button" disabled={Boolean(busyId)} onclick={() => runAction(item, 'install')}>{busyId === item.id && busyAction === 'install' ? t(locale,'common.installing') : t(locale,'common.install')}</button>{/if}
                {#if item.installed && canAction(item, 'update')}<button class="button" disabled={Boolean(busyId)} onclick={() => runAction(item, 'update')}>{busyId === item.id && busyAction === 'update' ? t(locale,'common.updating') : t(locale,'common.update')}</button>{/if}
                {#if item.installed && canAction(item, 'remove')}<button class="button danger-subtle" disabled={Boolean(busyId)} onclick={() => removeItem = item}>{t(locale,'common.remove')}</button>{/if}
                {#if item.installed || item.install?.method || item.project_url}<button class="button" onclick={() => plannerItem = item}>{t(locale,'common.details')}</button>{/if}
                {#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>{t(locale,'common.project')}</a>{/if}
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
    <span>{t(locale,'marketplace.visibleTotal',{visible:visibleModules.length + visibleIntegrations.length,total:all.length})}</span>
  </div>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
{#if removeItem}<RemoveConfirm item={removeItem} busy={busyId === removeItem.id && busyAction === 'remove'} oncancel={() => { if (!busyId) removeItem = null; }} onconfirm={(typed) => runAction(removeItem, 'remove', typed)}/>{/if}
