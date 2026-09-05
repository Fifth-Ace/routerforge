<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline, startCatalogPolling } from '$lib/stores/catalog.js';
  import { adminSummary, adminOnline, startAdminPolling } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline, startSystemModulePolling } from '$lib/stores/systemModule.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  let stopCatalog = null;
  let stopAdmin = null;
  let stopSystem = null;
  let unsubscribeCatalog = null;

  onMount(() => {
    stopCatalog = startCatalogPolling();
    unsubscribeCatalog = catalog.subscribe((value) => {
      const modules = value?.modules || [];
      const adminInstalled = modules.some((item) => item.id === 'admin' && item.installed);
      const systemInstalled = modules.some((item) => item.id === 'system' && item.installed);

      if (adminInstalled && !stopAdmin) stopAdmin = startAdminPolling();
      if (!adminInstalled && stopAdmin) { stopAdmin(); stopAdmin = null; }
      if (systemInstalled && !stopSystem) stopSystem = startSystemModulePolling();
      if (!systemInstalled && stopSystem) { stopSystem(); stopSystem = null; }
    });

    return () => {
      unsubscribeCatalog?.();
      stopCatalog?.();
      stopAdmin?.();
      stopSystem?.();
    };
  });

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: installedCapabilities = modules.filter((item) => item.installed && !item.builtin && item.id !== 'profiling');
  $: integrations = $catalog.integrations || [];
  $: externalInstalled = integrations.filter((x) => x.state === 'installed_external').length;
  $: servicesRunning = integrations.filter((x) => x.service_running).length
    + modules.filter((x) => x.managed && x.service_running).length;
  $: available = [...integrations, ...modules].filter((x) => x.state === 'available').length;
  $: telemetry = $systemModuleOnline ? $systemModuleSummary : $adminOnline ? $adminSummary : null;
  $: mem = telemetry?.memory;
  $: ramPct = mem?.total_kb ? Number(mem.used_kb || 0) / Number(mem.total_kb) * 100 : 0;

  function stateLabel(item, lang) {
    if (item.id === 'dns' && item.installed) return t(lang, 'common.enabled').toUpperCase();
    if (item.service_running) return t(lang, 'common.online').toUpperCase();
    if (item.installed) return t(lang, 'common.installed').toUpperCase();
    return t(lang, 'common.available').toUpperCase();
  }

  function stateClass(item) {
    if (item.id === 'dns' && item.installed) return 'good';
    if (item.service_running) return 'good';
    if (item.installed) return 'warn';
    return 'muted';
  }
</script>

<div class="global-shell">
  <aside class="global-rail global-rail-left">
    <div class="rail-main">
      <section class="rail-block">
        <div class="rail-section-label">RouterForge</div>
        <div class="global-module-tree mono">
          <div class="global-tree-root">
            <span class="status-dot good"></span>
            <strong>Core</strong>
            <span class="global-tree-state good">[{t(locale, 'common.online').toUpperCase()}]</span>
          </div>

          {#each installedCapabilities as module, index (module.id)}
            <div class="global-tree-row">
              <span class="tree-branch">{index === installedCapabilities.length - 1 ? '└─' : '├─'}</span>
              <span class="status-dot {stateClass(module)}"></span>
              <span class="global-tree-name">{module.name}</span>
              <span class="global-tree-state {stateClass(module)}">[{stateLabel(module, locale)}]</span>
            </div>
          {/each}
        </div>
      </section>

      <section class="rail-block">
        <div class="rail-section-label">{t(locale, 'shell.platformSummary')}</div>
        <div class="rail-summary-row"><span>{t(locale, 'shell.capabilities')}</span><strong>{installedCapabilities.length}</strong></div>
        <div class="rail-summary-row"><span>{t(locale, 'shell.externalInstalled')}</span><strong>{externalInstalled}</strong></div>
        <div class="rail-summary-row"><span>{t(locale, 'shell.servicesRunning')}</span><strong>{servicesRunning}</strong></div>
        <div class="rail-summary-row"><span>{t(locale, 'shell.marketplaceAvailable')}</span><strong>{available}</strong></div>
      </section>
    </div>

    <div class="rail-bottom-stack">
      {#if telemetry}
        <section class="rail-status-card mono">
          <div class="rail-section-label">{t(locale, 'shell.hostTelemetry')}</div>
          <div><span>host</span><strong>{telemetry.hostname || '—'}</strong></div>
          <div><span>load.1m</span><strong>{Number(telemetry.load_1 || 0).toFixed(2)}</strong></div>
          <div><span>ram.used</span><strong>{ramPct.toFixed(0)}%</strong></div>
          <div><span>processes</span><strong>{telemetry.process_count || 0}</strong></div>
        </section>
      {/if}

      <section class="rail-status-card mono">
        <div class="rail-section-label">Marketplace</div>
        <div><span>{t(locale, 'shell.catalogApi')}</span><strong class={$catalogOnline ? 'good' : 'bad'}>{$catalogOnline ? t(locale, 'common.online').toUpperCase() : t(locale, 'common.offline').toUpperCase()}</strong></div>
        <div><span>{t(locale, 'shell.registry')}</span><strong class={$catalog.registry?.online ? 'good' : 'warn'}>{$catalog.registry?.online ? t(locale, 'shell.remote') : ($catalog.registry?.source || t(locale, 'shell.bundled')).toUpperCase()}</strong></div>
        <div><span>{t(locale, 'shell.packages')}</span><strong class={$catalog.install_test_mode ? 'good' : 'muted'}>{$catalog.install_test_mode ? t(locale, 'shell.betaEnabled') : t(locale, 'common.disabled').toUpperCase()}</strong></div>
      </section>

      <a class="rail-repository-link" href="https://github.com/Fifth-Ace/routerforge" target="_blank" rel="noreferrer" aria-label={t(locale, 'shell.repositoryAria')}>
        <img src="/routerforge-mark.png" alt="" />
        <span class="rail-repository-copy"><small>{t(locale, 'shell.sourceCode')}</small><strong>{t(locale, 'shell.github')}</strong></span>
        <span class="rail-repository-arrow" aria-hidden="true">↗</span>
      </a>
    </div>
  </aside>

  <div class="global-content"><slot /></div>
</div>
