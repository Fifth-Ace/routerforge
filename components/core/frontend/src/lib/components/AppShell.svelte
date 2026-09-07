<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline, catalogReady, startCatalogPolling } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { overview, refreshOverview, startOverviewPolling, averageCPU, cpuTemperature } from '$lib/stores/overview.js';
  import { t } from '$lib/i18n/index.js';

  let stopCatalog = null;
  let stopCatalogSync = null;
  let stopOverview = null;

  onMount(() => {
    stopCatalog = startCatalogPolling();

    let lastInstalledFingerprint = '';
    stopCatalogSync = catalog.subscribe((value) => {
      const moduleIds = (value?.modules || [])
        .filter((item) => item.installed)
        .map((item) => item.id)
        .sort();
      const integrationIds = (value?.integrations || [])
        .filter((item) => item.installed)
        .map((item) => item.id)
        .sort();
      const hydrated = value?.phase !== 'loading' || moduleIds.length > 0 || integrationIds.length > 0;
      if (!hydrated) return;

      const fingerprint = `${moduleIds.join(',')}|${integrationIds.join(',')}`;
      if (fingerprint === lastInstalledFingerprint) return;
      lastInstalledFingerprint = fingerprint;
      refreshOverview(value);
    });

    stopOverview = startOverviewPolling(() => $catalog, 10000);
    return () => { stopCatalogSync?.(); stopCatalog?.(); stopOverview?.(); };
  });

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: installedModules = modules.filter((item) => item.installed && !item.builtin && item.id !== 'profiling');
  $: installedIntegrations = integrations.filter((item) => item.installed);
  $: telem = $overview || {};
  $: memory = telem.memory || telem.summary?.memory;
  $: ramPct = Number(memory?.used_pct || (memory?.total_kb ? Number(memory.used_kb || 0) / Number(memory.total_kb) * 100 : 0));
  $: cpuPct = averageCPU(telem.cpu);
  $: cpuTemp = cpuTemperature(telem.thermal);
  $: model = telem.platform?.model || 'Keenetic';
  $: processes = Number(telem.summary?.process_count || 0);

  function stateLabel(item) {
    if (item.update_available) return 'UPDATE';
    if (item.id === 'dns') return t(locale,'common.enabled').toUpperCase();
    if (item.service_running) return t(locale,'common.online').toUpperCase();
    return t(locale,'common.installed').toUpperCase();
  }
  function stateClass(item) {
    if (item.update_available) return 'warn';
    if (item.id === 'dns' || item.service_running) return 'good';
    return 'neutral';
  }
</script>

<div class="global-shell">
  <aside class="global-rail global-rail-left">
    <div class="rail-main">
      <section class="rail-block">
        <div class="rail-section-label">RouterForge</div>
        <div class="global-module-tree mono">
          <div class="global-tree-root">
            <span class="status-dot good"></span><strong>Core</strong>
            <span class="global-tree-state good">[{t(locale,'common.online').toUpperCase()}]</span>
          </div>
          {#each installedModules as item, index (item.id)}
            <div class="global-tree-row">
              <span class="tree-branch">{index === installedModules.length - 1 ? '└─' : '├─'}</span>
              <span class="status-dot {stateClass(item)}"></span>
              <span class="global-tree-name">{item.name}</span>
              <span class="global-tree-state {stateClass(item)}">[{stateLabel(item)}]</span>
            </div>
          {/each}
        </div>
      </section>

      {#if installedIntegrations.length}
        <section class="rail-block">
          <div class="rail-section-label">{locale === 'ru' ? 'Интеграции' : 'Integrations'}</div>
          <div class="global-module-tree mono">
            {#each installedIntegrations as item, index (item.id)}
              <div class="global-tree-row">
                <span class="tree-branch">{index === installedIntegrations.length - 1 ? '└─' : '├─'}</span>
                <span class="status-dot {stateClass(item)}"></span>
                <span class="global-tree-name">{item.name}</span>
                <span class="global-tree-state {stateClass(item)}">[{stateLabel(item)}]</span>
              </div>
            {/each}
          </div>
        </section>
      {/if}
    </div>

    <div class="rail-bottom-stack">
      <section class="rail-status-card mono">
        <div class="rail-section-label">{locale === 'ru' ? 'Телеметрия устройства' : 'Device telemetry'}</div>
        <div><span>{locale === 'ru' ? 'Модель' : 'Model'}</span><strong>{model}</strong></div>
        <div><span>CPU Temp</span><strong class:warn={cpuTemp > 75} class:bad={cpuTemp >= 90}>{cpuTemp ? `${cpuTemp.toFixed(0)}°C` : '—'}</strong></div>
        <div><span>CPU Usage</span><strong>{telem.cpu ? `${cpuPct.toFixed(0)}%` : '—'}</strong></div>
        <div><span>RAM Usage</span><strong>{memory ? `${ramPct.toFixed(0)}%` : '—'}</strong></div>
        <div><span>{locale === 'ru' ? 'Процессы' : 'Processes'}</span><strong>{processes || '—'}</strong></div>
      </section>

      <section class="rail-status-card mono">
        <div class="rail-section-label">{t(locale,'marketplace.pageTitle')}</div>
        <div><span>API</span><strong class={!$catalogReady ? '' : $catalogOnline ? 'good' : 'bad'}>{!$catalogReady ? '...' : $catalogOnline ? t(locale,'common.online').toUpperCase() : t(locale,'common.offline').toUpperCase()}</strong></div>
        <div><span>Registry</span><strong class={!$catalogReady ? '' : $catalog.registry?.online ? 'good' : 'warn'}>{!$catalogReady ? '...' : ($catalog.registry?.source || 'BUNDLED').toUpperCase()}</strong></div>
        <div><span>{locale === 'ru' ? 'Обновления' : 'Updates'}</span><strong class={[...modules,...integrations].some((x)=>x.update_available) ? 'warn' : 'good'}>{[...modules,...integrations].filter((x)=>x.update_available).length}</strong></div>
      </section>

      <a class="rail-repository-link" href="https://github.com/Fifth-Ace/routerforge" target="_blank" rel="noreferrer" aria-label={t(locale,'shell.repositoryAria')}>
        <img src="/routerforge-mark.png" alt="" />
        <span class="rail-repository-copy"><small>{t(locale,'shell.sourceCode')}</small><strong>{t(locale,'shell.github')}</strong></span>
        <span class="rail-repository-arrow" aria-hidden="true">↗</span>
      </a>
    </div>
  </aside>
  <div class="global-content"><slot /></div>
</div>
