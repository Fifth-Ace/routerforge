<script>
  import { catalog, catalogOnline } from '$lib/stores/catalog.js';
  import { snapshot, backendOnline } from '$lib/stores/snapshot.js';
  import { adminSummary, adminOnline } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline } from '$lib/stores/systemModule.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: installedModules = modules.filter((item) => item.installed && !item.builtin && item.id !== 'profiling');
  $: dns = modules.find((item) => item.id === 'dns');
  $: admin = modules.find((item) => item.id === 'admin');
  $: monitoring = modules.filter((item) => ['system','thermal','storage','network'].includes(item.id));
  $: monitoringInstalled = monitoring.filter((item) => item.installed);
  $: monitoringOnline = monitoring.filter((item) => item.installed && item.service_running);
  $: externalInstalled = integrations.filter((item) => item.installed).length;
  $: telemetry = $systemModuleOnline ? $systemModuleSummary : $adminOnline ? $adminSummary : null;
  $: memory = telemetry?.memory;
  $: ramPct = memory?.total_kb ? Number(memory.used_kb || 0) / Number(memory.total_kb) * 100 : 0;
  $: issues = buildIssues(locale, $backendOnline, $catalogOnline, $catalog, $snapshot, dns, monitoringInstalled, admin);
  $: moduleCards = installedModules
    .filter((item) => item.presentation?.dashboard?.enabled !== false)
    .sort((a,b) => Number(a.presentation?.dashboard?.priority || 50) - Number(b.presentation?.dashboard?.priority || 50));

  function buildIssues(lang, coreOnline, catalogApiOnline, catalogData, snapshotData, dnsModule, installedMonitoring, adminModule) {
    const out = [];
    if (!coreOnline) out.push({ cls:'error', text:t(lang, 'errors.coreUnavailable') });
    if (!catalogApiOnline) out.push({ cls:'warn', text:t(lang, 'errors.catalogUnavailable') });
    if (catalogData?.registry && !catalogData.registry.online) out.push({ cls:'warn', text:t(lang, 'home.issueRegistryCache', { source: (catalogData.registry.source || 'bundled').toUpperCase() }) });
    if (dnsModule?.installed && snapshotData?.capture_error) out.push({ cls:'error', text:`${t(lang, 'errors.dnsCapture')}: ${snapshotData.capture_error}` });
    if (dnsModule?.installed && snapshotData?.discovery_error) out.push({ cls:'warn', text:`${t(lang, 'errors.dnsDiscovery')}: ${snapshotData.discovery_error}` });
    for (const item of installedMonitoring || []) {
      if (!item.service_running) out.push({ cls:'warn', text:t(lang, 'home.issueRuntimeStopped', { name: item.name }) });
    }
    if (adminModule?.installed && !adminModule.service_running) out.push({ cls:'warn', text:t(lang, 'home.issueControlRuntimeStopped') });
    return out;
  }

  function hrefFor(item) {
    if (item.presentation?.navigation?.href) return item.presentation.navigation.href;
    if (['system','thermal','storage','network'].includes(item.id)) return `/monitoring?tab=${encodeURIComponent(item.id)}`;
    return '/catalog';
  }

  function short(item) {
    const map = { dns:'DNS', admin:'CTL', system:'SYS', thermal:'TMP', storage:'DSK', network:'NET' };
    return map[item.id] || String(item.name || 'MOD').slice(0,3).toUpperCase();
  }

  function moduleState(item, lang) {
    if (item.id === 'dns') return t(lang, 'common.enabled').toUpperCase();
    return item.service_running ? t(lang, 'common.online').toUpperCase() : t(lang, 'common.installed').toUpperCase();
  }
</script>

<svelte:head><title>RouterForge — {t(locale, 'home.pageTitle')}</title></svelte:head>

<div class="page routerforge-home">
  <div class="page-head routerforge-home-head">
    <div>
      <span class="routerforge-eyebrow mono">ROUTERFORGE / BETA</span>
      <h1>{telemetry?.hostname || 'RouterForge'}</h1>
      <p>{t(locale, 'home.subtitle')}</p>
    </div>
    <span class="state-chip {$backendOnline ? 'good' : 'error'}">CORE {$backendOnline ? t(locale, 'common.online').toUpperCase() : t(locale, 'common.offline').toUpperCase()}</span>
  </div>

  <section class="metric-grid four routerforge-platform-metrics">
    <div class="metric-card">
      <span>RouterForge</span>
      <strong>v{$snapshot.version || 'beta'}</strong>
      <small>Core · :2233</small>
    </div>
    <div class="metric-card">
      <span>{t(locale, 'home.capabilities')}</span>
      <strong>{installedModules.length}</strong>
      <small>{t(locale, 'home.monitoringOnline', { online: monitoringOnline.length, installed: monitoringInstalled.length })}</small>
    </div>
    <div class="metric-card">
      <span>Marketplace</span>
      <strong>{$catalog.registry?.online ? 'ONLINE' : 'CACHE'}</strong>
      <small>{t(locale, 'home.entriesExternal', { entries: integrations.length + modules.length, external: externalInstalled })}</small>
    </div>
    <div class="metric-card">
      <span>{t(locale, 'home.host')}</span>
      <strong>{telemetry ? `${ramPct.toFixed(0)}% RAM` : '—'}</strong>
      <small>{telemetry ? t(locale, 'home.processes', { load: Number(telemetry.load_1 || 0).toFixed(2), count: telemetry.process_count || 0 }) : t(locale, 'home.installSystemMonitor')}</small>
    </div>
  </section>

  <div class="routerforge-home-grid">
    <section class="panel routerforge-attention">
      <div class="panel-head">
        <div><strong>{t(locale, 'home.state')}</strong><span>{t(locale, 'home.stateSubtitle')}</span></div>
        <span class="state-chip {issues.length ? 'warn' : 'good'}">{issues.length ? t(locale, 'home.attention', { count: issues.length }) : t(locale, 'home.allGood')}</span>
      </div>
      {#if issues.length}
        <div class="routerforge-issue-list">
          {#each issues as issue}
            <div class="routerforge-issue"><span class="status-dot {issue.cls}"></span><span>{issue.text}</span></div>
          {/each}
        </div>
      {:else}
        <div class="routerforge-all-good"><span class="status-dot good"></span><strong>{t(locale, 'home.noCritical')}</strong><span>{t(locale, 'home.allGoodHint')}</span></div>
      {/if}
    </section>

    <section class="panel routerforge-platform-panel">
      <div class="panel-head"><div><strong>{t(locale, 'home.platform')}</strong><span>{t(locale, 'home.platformSubtitle')}</span></div></div>
      <div class="routerforge-platform-grid">
        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">DNS</span>
          <span class="routerforge-platform-copy"><strong>DNS</strong><small>{t(locale, 'home.dnsDescription')}</small></span>
          <span class="state-chip {dns?.installed ? 'good' : 'neutral'}">{dns?.installed ? t(locale, 'common.enabled').toUpperCase() : t(locale, 'common.notInstalled').toUpperCase()}</span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">MON</span>
          <span class="routerforge-platform-copy"><strong>{t(locale, 'nav.monitoring')}</strong><small>{t(locale, 'home.monitoringDescription')}</small></span>
          <span class="state-chip {monitoringInstalled.length && monitoringOnline.length === monitoringInstalled.length ? 'good' : monitoringInstalled.length ? 'warn' : 'neutral'}">
            {monitoringInstalled.length ? `${monitoringOnline.length}/${monitoringInstalled.length} ${t(locale, 'common.online').toUpperCase()}` : t(locale, 'common.notInstalled').toUpperCase()}
          </span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">CTL</span>
          <span class="routerforge-platform-copy"><strong>{t(locale, 'nav.manage')}</strong><small>{t(locale, 'home.controlDescription')}</small></span>
          <span class="state-chip {admin?.installed && admin?.service_running ? 'good' : admin?.installed ? 'warn' : 'neutral'}">
            {admin?.installed ? admin?.service_running ? t(locale, 'common.online').toUpperCase() : t(locale, 'common.installed').toUpperCase() : t(locale, 'common.notInstalled').toUpperCase()}
          </span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">REG</span>
          <span class="routerforge-platform-copy"><strong>Registry</strong><small>{t(locale, 'home.registryDescription')}</small></span>
          <span class="state-chip {$catalog.registry?.online ? 'info' : 'warn'}">{($catalog.registry?.source || '—').toUpperCase()}</span>
        </div>
      </div>
    </section>
  </div>

  <section class="routerforge-capabilities-section">
    <div class="catalog-section-head">
      <div><h2>{t(locale, 'home.installedCapabilities')}</h2><p>{t(locale, 'home.installedCapabilitiesSubtitle')}</p></div>
      <a class="button" href="/catalog">Marketplace</a>
    </div>

    <div class="routerforge-capability-grid">
      {#if !moduleCards.length}
        <a class="routerforge-empty-capability" href="/catalog">
          <strong>{t(locale, 'home.addCapabilities')}</strong>
          <span>{t(locale, 'home.addCapabilitiesHint')}</span>
        </a>
      {/if}
      {#each moduleCards as item (item.id)}
        <a class="routerforge-capability-card" href={hrefFor(item)}>
          <span class="routerforge-capability-icon mono">{short(item)}</span>
          <span class="routerforge-capability-copy"><strong>{item.name}</strong><small>{item.description}</small></span>
          <span class="state-chip {item.id === 'dns' || item.service_running ? 'good' : 'warn'}">{moduleState(item, locale)}</span>
        </a>
      {/each}
    </div>
  </section>
</div>
