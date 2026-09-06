<script>
  import { catalog, refreshCatalog, forceRefreshCatalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import {
    catalogAction, setCatalogChannel, getEntwarePackages,
    refreshEntwarePackages, entwarePackageAction
  } from '$lib/api.js';
  import { stateInfo, localWebURL } from '$lib/utils.js';
  import { appText as a } from '$lib/app-center-i18n.js';
  import { t } from '$lib/i18n/index.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';
  import RemoveConfirm from '$lib/components/RemoveConfirm.svelte';

  const acronyms = {
    'awg-manager':'AWG', nfqws2:'NQ2', nfqws:'NQ1', 'nfqws-web':'NQW', 'hydraroute-neo':'HRN',
    'routerforge-core':'RFC', dns:'DNS', system:'SYS', thermal:'TMP', storage:'DSK', network:'NET',
    admin:'ADM', profiling:'PRF', xkeen:'XKN', 'xkeen-ui':'XUI', 'keen-pbr':'PBR', kvas:'KVS',
    'bypass-keenetic':'BYP', 'traffic-via-vpn':'VPN', 'adguardhome-keenetic':'AGH', skeen:'SKN',
    'chur-keenetic':'CHR', 'keenetic-sing-box-ui':'SBU', 'keenetic-entware-extras':'KEE'
  };

  let tab = 'routerforge';
  let search = '';
  let plannerItem = null;
  let removeItem = null;
  let busyId = '';
  let busyAction = '';
  let actionNotice = null;
  let channelBusy = false;
  let checkingUpdates = false;
  let updatingAll = false;
  let entwareLoading = false;
  let entwareRefreshing = false;
  let entwareState = 'all';
  let entwareData = { items:[], total:0, offset:0, limit:100, installed_count:0, upgradable_count:0, online:false };

  $: locale = $settings.locale || 'ru';
  $: data = $catalog || { modules:[], integrations:[], read_only:true, package_management_enabled:false };
  $: packageMode = Boolean(data.package_management_enabled ?? data.install_test_mode);
  $: releaseChannel = data.release?.channel || 'beta';
  $: releaseTarget = data.release?.target || '—';
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: routerForgeUpdates = modules.filter(hasVerifiedUpdate);
  $: integrationUpdates = integrations.filter(hasVerifiedUpdate);
  $: installedCatalog = [...modules, ...integrations].filter((item) => item.installed);
  $: updateCatalog = [...routerForgeUpdates, ...integrationUpdates];

  $: catalogItems = tab === 'routerforge' ? filterCatalog(modules, search)
    : tab === 'integrations' ? filterCatalog(integrations, search)
    : tab === 'installed' ? filterCatalog(installedCatalog, search)
    : tab === 'updates' ? filterCatalog(updateCatalog, search)
    : [];

  $: sectionTitle = tab === 'routerforge' ? a(locale,'tabs.routerforge')
    : tab === 'integrations' ? a(locale,'tabs.integrations')
    : tab === 'installed' ? a(locale,'tabs.installed')
    : tab === 'updates' ? a(locale,'tabs.updates')
    : a(locale,'tabs.entware');

  $: sectionSubtitle = tab === 'routerforge' ? a(locale,'routerforgeSubtitle')
    : tab === 'integrations' ? a(locale,'integrationsSubtitle')
    : tab === 'installed' ? a(locale,'installedSubtitle')
    : tab === 'updates' ? a(locale,'updatesSubtitle')
    : a(locale,'entwareSubtitle');

  $: tabs = [
    ['routerforge', a(locale,'tabs.routerforge'), modules.length],
    ['integrations', a(locale,'tabs.integrations'), integrations.length],
    ['entware', a(locale,'tabs.entware'), entwareData.total || ''],
    ['installed', a(locale,'tabs.installed'), ''],
    ['updates', a(locale,'tabs.updates'), updateCatalog.length + Number(entwareData.upgradable_count || 0)]
  ];

  function hasVerifiedUpdate(item) {
    return Boolean(item?.installed && item?.update_available && item?.actions?.update);
  }

  function filterCatalog(items, query) {
    const q = String(query || '').trim().toLowerCase();
    if (!q) return items;
    return items.filter((item) =>
      `${item.name} ${item.category || ''} ${item.description || ''} ${item.version || ''} ${(item.detection?.packages || []).join(' ')} ${item.publisher?.name || ''}`
        .toLowerCase().includes(q)
    );
  }

  function acronym(item) {
    return acronyms[item.id] || String(item.name || 'APP').replace(/[^A-Za-z0-9]/g,'').slice(0,3).toUpperCase() || 'APP';
  }

  function packageText(item) {
    const packages = item.install?.packages || item.detection?.packages || [];
    return packages.length ? packages.join(', ') : item.id === 'routerforge-core' ? 'routerforge-core' : '—';
  }

  function compatibilityText(item) {
    const hints = item.compatibility?.hints || [];
    if (item.compatibility?.status === 'built-in') return locale === 'ru' ? 'встроено' : 'built-in';
    return hints.length ? hints.join(' · ') : (item.compatibility?.status || '—');
  }

  function moduleURL(item) {
    if (item.id === 'admin') return '/manage';
    if (item.id === 'dns') return '/dns';
    if (['system','thermal','storage','network','profiling'].includes(item.id)) return `/monitoring?tab=${encodeURIComponent(item.id)}`;
    return '';
  }

  function canAction(item, action) {
    return packageMode && Boolean(item.actions?.[action]);
  }

  function trustLabel(item) {
    const value = String(item.trust?.status || 'unverified').toLowerCase();
    const labels = locale === 'ru'
      ? { official:'ОФИЦИАЛЬНЫЙ', verified:'ПРОВЕРЕН', changed:'ИЗМЕНЁН', blocked:'ЗАБЛОКИРОВАН', deprecated:'УСТАРЕЛ', unverified:'НЕ ПРОВЕРЕН' }
      : { official:'OFFICIAL', verified:'VERIFIED', changed:'CHANGED', blocked:'BLOCKED', deprecated:'DEPRECATED', unverified:'UNVERIFIED' };
    return labels[value] || labels.unverified;
  }

  function trustClass(item) {
    const value = String(item.trust?.status || '').toLowerCase();
    if (value === 'official') return 'official';
    if (value === 'verified') return 'verified';
    if (value === 'changed') return 'warn';
    if (value === 'blocked') return 'error';
    return 'neutral';
  }

  function entwareMode() {
    if (tab === 'installed') return 'installed';
    if (tab === 'updates') return 'upgradable';
    return entwareState === 'all' ? '' : entwareState;
  }

  async function runCatalogAction(item, action, confirm = '', skipConfirm = false) {
    if (!canAction(item, action) || busyId) return;
    if (!skipConfirm && !window.confirm(`${action === 'install' ? a(locale,'install') : a(locale,'update')} ${item.name}?`)) return;

    busyId = item.id;
    busyAction = action;
    try {
      await catalogAction(item.id, action, confirm);
      await refreshCatalog();
      actionNotice = { cls:'good', text:a(locale,'actionDone',{name:item.name}) };
      if (action === 'remove') removeItem = null;
    } catch (error) {
      actionNotice = { cls:'error', text:a(locale,'actionFailed',{name:item.name,error:error?.payload?.detail || error?.payload?.error || error?.message || 'error'}) };
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  async function changeReleaseChannel(event) {
    const select = event.currentTarget;
    const next = String(select.value || '').toLowerCase();
    const current = String(releaseChannel || 'beta').toLowerCase();
    if (next === current || channelBusy || busyId) return;

    if (!window.confirm(next === 'beta' ? a(locale,'channelBeta') : a(locale,'channelStable'))) {
      select.value = current;
      return;
    }

    channelBusy = true;
    try {
      const result = await setCatalogChannel(next);
      await refreshCatalog();
      const release = result?.release || {};
      actionNotice = {
        cls: release.online === false ? 'warn' : 'good',
        text: a(locale,'channelDone',{channel:String(release.channel || next).toUpperCase(),target:release.target || '—'})
      };
    } catch (error) {
      select.value = current;
      actionNotice = { cls:'error', text:error?.payload?.error || error?.message || 'error' };
    } finally {
      channelBusy = false;
    }
  }

  async function checkForUpdates() {
    if (checkingUpdates || busyId) return;
    checkingUpdates = true;
    try {
      await forceRefreshCatalog();
      if (['entware','installed','updates'].includes(tab)) await loadEntware(0);
      actionNotice = { cls:'good', text:a(locale,'checkUpdates') };
    } catch (error) {
      actionNotice = { cls:'error', text:error?.payload?.error || error?.message || 'error' };
    } finally {
      checkingUpdates = false;
    }
  }

  async function updateAllRouterForge() {
    if (updatingAll || busyId || !routerForgeUpdates.length) return;
    if (!window.confirm(`${a(locale,'routerforgeUpdateAll')} (${routerForgeUpdates.length})?`)) return;
    updatingAll = true;
    const ordered = [...routerForgeUpdates].sort((x,y) => x.id === 'routerforge-core' ? 1 : y.id === 'routerforge-core' ? -1 : 0);
    try {
      for (const item of ordered) await runCatalogAction(item, 'update', '', true);
    } finally {
      updatingAll = false;
      await refreshCatalog();
    }
  }

  async function loadEntware(offset = 0) {
    if (entwareLoading) return;
    entwareLoading = true;
    try {
      entwareData = await getEntwarePackages({
        query: search,
        state: entwareMode(),
        offset,
        limit: 100
      });
    } catch (error) {
      entwareData = { ...entwareData, online:false, error:error?.payload?.error || error?.message || 'error', items:[] };
    } finally {
      entwareLoading = false;
    }
  }

  async function refreshEntware() {
    if (entwareRefreshing || busyId) return;
    entwareRefreshing = true;
    try {
      await refreshEntwarePackages();
      await loadEntware(0);
      actionNotice = { cls:'good', text:a(locale,'entwareListsDone') };
    } catch (error) {
      actionNotice = { cls:'error', text:error?.payload?.detail || error?.payload?.error || error?.message || 'error' };
    } finally {
      entwareRefreshing = false;
    }
  }

  async function runEntwareAction(pkg, action) {
    if (!packageMode || busyId) return;
    const question = action === 'install' ? a(locale,'confirmInstall',{name:pkg.name})
      : action === 'update' ? a(locale,'confirmUpdate',{name:pkg.name})
      : a(locale,'confirmRemove',{name:pkg.name});
    if (!window.confirm(question)) return;

    busyId = `entware:${pkg.name}`;
    busyAction = action;
    try {
      await entwarePackageAction(pkg.name, action, action === 'remove' ? pkg.name : '');
      await loadEntware(entwareData.offset || 0);
      actionNotice = { cls:'good', text:a(locale,'actionDone',{name:pkg.name}) };
    } catch (error) {
      actionNotice = { cls:'error', text:a(locale,'actionFailed',{name:pkg.name,error:error?.payload?.detail || error?.payload?.error || error?.message || 'error'}) };
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  function setTab(next) {
    tab = next;
    search = '';
    entwareState = 'all';
    if (['entware','installed','updates'].includes(next)) void loadEntware(0);
  }
</script>

<svelte:head><title>RouterForge — {a(locale,'pageTitle')}</title></svelte:head>

<div class="page catalog-page app-center-page">
  <div class="page-head">
    <div>
      <span class="routerforge-eyebrow mono">ROUTERFORGE / APP CENTER</span>
      <h1>{a(locale,'pageTitle')}</h1>
      <p>{a(locale,'subtitle')}</p>
    </div>
    <span class="state-chip {data.registry?.online ? 'good' : 'warn'}">
      {a(locale,'registry')} {(data.registry?.source || 'BUNDLED').toUpperCase()}
    </span>
  </div>

  <div class="subtabs app-center-tabs">
    {#each tabs as entry (entry[0])}
      <button class:active={tab === entry[0]} onclick={() => setTab(entry[0])}>
        {entry[1]} {#if entry[2] !== ''}<span class="pill">{entry[2]}</span>{/if}
      </button>
    {/each}
  </div>

  <div class="toolbar catalog-toolbar-v2">
    <div class="search-control flex">
      <span>⌕</span>
      <input bind:value={search} onkeydown={(event) => { if (event.key === 'Enter' && ['entware','installed','updates'].includes(tab)) void loadEntware(0); }} placeholder={a(locale,'search')}/>
    </div>

    {#if ['entware','installed','updates'].includes(tab)}
      <button class="button" disabled={entwareLoading} onclick={() => loadEntware(0)}>{a(locale,'searchButton')}</button>
    {/if}

    {#if tab === 'entware'}
      <select bind:value={entwareState} onchange={() => loadEntware(0)}>
        <option value="all">{a(locale,'all')}</option>
        <option value="installed">{a(locale,'installedOnly')}</option>
        <option value="available">{a(locale,'availableOnly')}</option>
        <option value="upgradable">{a(locale,'tabs.updates')}</option>
      </select>
    {/if}

    <select aria-label="RouterForge channel" value={releaseChannel} disabled={channelBusy || Boolean(busyId)} onchange={changeReleaseChannel}>
      <option value="stable">RouterForge Stable</option>
      <option value="beta">RouterForge Beta</option>
    </select>

    <button class="button" disabled={checkingUpdates || Boolean(busyId)} onclick={checkForUpdates}>
      {checkingUpdates ? a(locale,'checking') : a(locale,'checkUpdates')}
    </button>

    {#if ['entware','installed','updates'].includes(tab) && packageMode}
      <button class="button" disabled={entwareRefreshing || Boolean(busyId)} onclick={refreshEntware}>
        {entwareRefreshing ? a(locale,'updatingLists') : a(locale,'updateLists')}
      </button>
    {/if}

    {#if tab === 'routerforge' && packageMode && routerForgeUpdates.length}
      <button class="button primary" disabled={updatingAll || Boolean(busyId)} onclick={updateAllRouterForge}>
        {a(locale,'routerforgeUpdateAll')} ({routerForgeUpdates.length})
      </button>
    {/if}
  </div>

  <div class="market-safety-line mono" class:test-mode={packageMode}>
    <span><i class="status-dot {packageMode ? 'good' : 'neutral'}"></i> {a(locale,'packageManagement')} <strong>{packageMode ? a(locale,'active').toUpperCase() : a(locale,'readOnly').toUpperCase()}</strong></span>
    <span>{a(locale,'channel')} <strong>{String(releaseChannel).toUpperCase()}</strong></span>
    <span>{a(locale,'target')} <strong>{releaseTarget}</strong>{#if data.release?.experimental} · EXPERIMENTAL{/if}</span>
    <span>{a(locale,'registry')} <strong>{data.registry?.revision ? data.registry.revision.slice(0,12) : '—'}</strong></span>
  </div>

  {#if actionNotice}
    <div class="catalog-install-notice {actionNotice.cls}">
      <span class="status-dot {actionNotice.cls}"></span><span>{actionNotice.text}</span>
      <button class="icon-button" aria-label={t(locale,'common.close')} onclick={() => actionNotice = null}>×</button>
    </div>
  {/if}

  <section class="catalog-market-section">
    <div class="catalog-section-head">
      <div><h2>{sectionTitle}</h2><p>{sectionSubtitle}</p></div>
      {#if tab === 'routerforge'}<span class="state-chip info">{a(locale,'coreCapability')}</span>
      {:else}<span class="state-chip neutral">{tab === 'entware' ? entwareData.total || 0 : catalogItems.length + (['installed','updates'].includes(tab) ? Number(entwareData.total || 0) : 0)}</span>{/if}
    </div>

    {#if tab !== 'entware'}
      <div class="catalog-grid catalog-grid-v2">
        {#if !catalogItems.length && !['installed','updates'].includes(tab)}<div class="catalog-empty">{a(locale,'noItems')}</div>{/if}
        {#each catalogItems as item (item.id)}
          {@const st = stateInfo(item,locale)}
          {@const ownURL = item.kind === 'module' && item.installed ? moduleURL(item) : ''}
          <article class="catalog-card">
            <div>
              <div class="catalog-card-head">
                <div class="catalog-identity">
                  <div class="catalog-icon mono">{acronym(item)}</div>
                  <div><h3>{item.name}</h3><span class="mono">{item.publisher?.name || item.source || 'community'} / {item.category || item.kind}</span></div>
                </div>
                <div class="catalog-state-stack"><span class="state-chip {trustClass(item)}">{trustLabel(item)}</span><span class="state-chip {st.cls}">{st.label}</span></div>
              </div>
              <p>{item.description || ''}</p>
              <div class="tech-box mono">
                <div><span>{a(locale,'installed')}</span><strong>{item.version ? `v${item.version}` : '—'}</strong></div>
                <div><span>{a(locale,'available')}</span><strong class:good={item.update_available}>{item.release?.version ? `v${item.release.version}` : '—'}</strong></div>
                <div><span>{a(locale,'package')}</span><strong title={packageText(item)}>{packageText(item)}</strong></div>
                <div><span>{a(locale,'publisher')}</span><strong>{item.publisher?.name || '—'}</strong></div>
                <div><span>{a(locale,'service')}</span><strong class:good={item.service_running}>{item.service ? (item.service_running ? 'RUNNING' : 'STOPPED') : item.id === 'routerforge-core' ? 'CORE' : '—'}</strong></div>
                <div><span>{a(locale,'compatibility')}</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div>
              </div>
              {#if item.actions?.reason && !item.actions?.install && !item.actions?.update && !item.actions?.remove}
                <div class="catalog-action-reason">{item.actions.reason}</div>
              {/if}
            </div>
            <div class="catalog-card-foot">
              <div class="catalog-actions">
                {#if ownURL}<a class="button primary" href={ownURL}>{a(locale,'open')}</a>{/if}
                {#if item.installed && item.web_port}<a class="button" target="_blank" rel="noopener noreferrer" href={localWebURL(item.web_port)}>{a(locale,'open')} :{item.web_port}</a>{/if}
                {#if !item.installed && canAction(item,'install')}<button class="button primary" disabled={Boolean(busyId)} onclick={() => runCatalogAction(item,'install')}>{busyId === item.id ? a(locale,'installing') : a(locale,'install')}</button>{/if}
                {#if item.installed && canAction(item,'update')}<button class="button" disabled={Boolean(busyId)} onclick={() => runCatalogAction(item,'update')}>{busyId === item.id ? a(locale,'updating') : a(locale,'update')}</button>{/if}
                {#if item.installed && canAction(item,'remove')}<button class="button danger-subtle" disabled={Boolean(busyId)} onclick={() => removeItem = item}>{a(locale,'remove')}</button>{/if}
                {#if item.installed || item.install?.method || item.project_url}<button class="button" onclick={() => plannerItem = item}>{a(locale,'details')}</button>{/if}
                {#if item.project_url}<a class="button compact" target="_blank" rel="noopener noreferrer" href={item.project_url}>{a(locale,'project')}</a>{/if}
              </div>
              <span class="mono muted">{item.manifest_sha256 ? `manifest ${item.manifest_sha256.slice(0,8)}` : item.kind}</span>
            </div>
          </article>
        {/each}
      </div>
    {/if}

    {#if ['entware','installed','updates'].includes(tab)}
      <div class="entware-summary mono">
        <span>{a(locale,'total')}: <strong>{entwareData.total || 0}</strong></span>
        <span>{a(locale,'installedCount')}: <strong>{entwareData.installed_count || 0}</strong></span>
        <span>{a(locale,'updatesCount')}: <strong>{entwareData.upgradable_count || 0}</strong></span>
      </div>
      {#if tab === 'entware'}<div class="catalog-action-reason">{a(locale,'entwareWarning')}</div>{/if}

      {#if entwareLoading}
        <div class="catalog-empty">{t(locale,'common.loading')}</div>
      {:else if entwareData.error}
        <div class="catalog-install-notice error">{entwareData.error}</div>
      {:else}
        <div class="entware-package-list">
          {#if !(entwareData.items || []).length && !catalogItems.length}<div class="catalog-empty">{a(locale,'noItems')}</div>{/if}
          {#each entwareData.items || [] as pkg (pkg.name)}
            <article class="entware-package-row">
              <div class="entware-package-main">
                <div><strong>{pkg.name}</strong>{#if pkg.upgradable}<span class="state-chip warn">UPDATE</span>{/if}{#if pkg.installed}<span class="state-chip good">INSTALLED</span>{/if}</div>
                <p>{pkg.description || '—'}</p>
                <span class="mono muted">{pkg.installed_version ? `installed ${pkg.installed_version}` : ''}{pkg.installed_version && pkg.available_version ? ' · ' : ''}{pkg.available_version ? `available ${pkg.available_version}` : ''}</span>
              </div>
              <div class="catalog-actions">
                {#if !pkg.installed && packageMode}<button class="button primary" disabled={Boolean(busyId)} onclick={() => runEntwareAction(pkg,'install')}>{busyId === `entware:${pkg.name}` ? a(locale,'installing') : a(locale,'install')}</button>{/if}
                {#if pkg.installed && pkg.upgradable && packageMode}<button class="button" disabled={Boolean(busyId)} onclick={() => runEntwareAction(pkg,'update')}>{busyId === `entware:${pkg.name}` ? a(locale,'updating') : a(locale,'update')}</button>{/if}
                {#if pkg.installed && packageMode && !pkg.name.startsWith('routerforge-') && pkg.name !== 'opkg'}<button class="button danger-subtle" disabled={Boolean(busyId)} onclick={() => runEntwareAction(pkg,'remove')}>{busyId === `entware:${pkg.name}` ? a(locale,'removing') : a(locale,'remove')}</button>{/if}
              </div>
            </article>
          {/each}
        </div>

        {#if Number(entwareData.total || 0) > Number(entwareData.limit || 100)}
          <div class="entware-pagination">
            <button class="button" disabled={Number(entwareData.offset || 0) <= 0} onclick={() => loadEntware(Math.max(0, Number(entwareData.offset || 0) - Number(entwareData.limit || 100)))}>{a(locale,'prev')}</button>
            <span class="mono">{Number(entwareData.offset || 0) + 1}–{Math.min(Number(entwareData.total || 0), Number(entwareData.offset || 0) + Number(entwareData.limit || 100))} / {entwareData.total}</span>
            <button class="button" disabled={Number(entwareData.offset || 0) + Number(entwareData.limit || 100) >= Number(entwareData.total || 0)} onclick={() => loadEntware(Number(entwareData.offset || 0) + Number(entwareData.limit || 100))}>{a(locale,'next')}</button>
          </div>
        {/if}
      {/if}
    {/if}
  </section>
</div>

{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
{#if removeItem}<RemoveConfirm item={removeItem} busy={busyId === removeItem.id && busyAction === 'remove'} oncancel={() => { if (!busyId) removeItem = null; }} onconfirm={(typed) => runCatalogAction(removeItem,'remove',typed)}/>{/if}

<style>
  .app-center-tabs { margin-bottom: 1rem; }
  .entware-summary { display:flex; gap:1.2rem; flex-wrap:wrap; margin:1rem 0; color:var(--rf-muted,var(--muted)); }
  .entware-package-list { display:grid; gap:.55rem; margin-top:1rem; }
  .entware-package-row { display:flex; justify-content:space-between; gap:1rem; align-items:center; border:1px solid var(--rf-border,var(--border)); border-radius:.7rem; padding:.8rem 1rem; background:var(--rf-panel,var(--panel)); }
  .entware-package-main { min-width:0; }
  .entware-package-main > div { display:flex; align-items:center; gap:.45rem; flex-wrap:wrap; }
  .entware-package-main p { margin:.25rem 0; }
  .entware-pagination { display:flex; justify-content:center; align-items:center; gap:.75rem; margin:1rem 0; }
  @media (max-width: 760px) {
    .entware-package-row { align-items:flex-start; flex-direction:column; }
  }
</style>
