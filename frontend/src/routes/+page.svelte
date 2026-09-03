<script>
  import { catalog, catalogOnline } from '$lib/stores/catalog.js';
  import { snapshot, backendOnline } from '$lib/stores/snapshot.js';
  import { adminSummary, adminOnline } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline } from '$lib/stores/systemModule.js';

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
  $: issues = buildIssues($backendOnline, $catalogOnline, $catalog, $snapshot, dns, monitoringInstalled, admin);
  $: moduleCards = installedModules
    .filter((item) => item.presentation?.dashboard?.enabled !== false)
    .sort((a,b) => Number(a.presentation?.dashboard?.priority || 50) - Number(b.presentation?.dashboard?.priority || 50));

  function buildIssues(coreOnline, catalogApiOnline, catalogData, snapshotData, dnsModule, installedMonitoring, adminModule) {
    const out = [];
    if (!coreOnline) out.push({ cls:'error', text:'RouterForge Core недоступен' });
    if (!catalogApiOnline) out.push({ cls:'warn', text:'Catalog API недоступен' });
    if (catalogData?.registry && !catalogData.registry.online) out.push({ cls:'warn', text:`Marketplace Registry работает из ${(catalogData.registry.source || 'bundled').toUpperCase()} cache` });
    if (dnsModule?.installed && snapshotData?.capture_error) out.push({ cls:'error', text:`DNS capture: ${snapshotData.capture_error}` });
    if (dnsModule?.installed && snapshotData?.discovery_error) out.push({ cls:'warn', text:`DNS discovery: ${snapshotData.discovery_error}` });
    for (const item of installedMonitoring || []) {
      if (!item.service_running) out.push({ cls:'warn', text:`${item.name}: runtime не запущен` });
    }
    if (adminModule?.installed && !adminModule.service_running) out.push({ cls:'warn', text:'RouterForge Control установлен, но runtime не запущен' });
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
</script>

<svelte:head><title>RouterForge — Главная</title></svelte:head>

<div class="page routerforge-home">
  <div class="page-head routerforge-home-head">
    <div>
      <span class="routerforge-eyebrow mono">ROUTERFORGE / BETA</span>
      <h1>{telemetry?.hostname || 'RouterForge'}</h1>
      <p>Локальная платформа управления, мониторинга и расширений для роутера.</p>
    </div>
    <span class="state-chip {$backendOnline ? 'good' : 'error'}">CORE {$backendOnline ? 'ONLINE' : 'OFFLINE'}</span>
  </div>

  <section class="metric-grid four routerforge-platform-metrics">
    <div class="metric-card">
      <span>RouterForge</span>
      <strong>v{$snapshot.version || 'beta'}</strong>
      <small>Core · :2233</small>
    </div>
    <div class="metric-card">
      <span>Возможности</span>
      <strong>{installedModules.length}</strong>
      <small>{monitoringOnline.length}/{monitoringInstalled.length} monitoring online</small>
    </div>
    <div class="metric-card">
      <span>Marketplace</span>
      <strong>{$catalog.registry?.online ? 'ONLINE' : 'CACHE'}</strong>
      <small>{integrations.length + modules.length} entries · {externalInstalled} external</small>
    </div>
    <div class="metric-card">
      <span>Host</span>
      <strong>{telemetry ? `${ramPct.toFixed(0)}% RAM` : '—'}</strong>
      <small>{telemetry ? `load ${Number(telemetry.load_1 || 0).toFixed(2)} · ${telemetry.process_count || 0} processes` : 'установите System Monitor'}</small>
    </div>
  </section>

  <div class="routerforge-home-grid">
    <section class="panel routerforge-attention">
      <div class="panel-head">
        <div><strong>Состояние</strong><span>То, что требует внимания прямо сейчас</span></div>
        <span class="state-chip {issues.length ? 'warn' : 'good'}">{issues.length ? `${issues.length} ATTENTION` : 'ALL GOOD'}</span>
      </div>
      {#if issues.length}
        <div class="routerforge-issue-list">
          {#each issues as issue}
            <div class="routerforge-issue"><span class="status-dot {issue.cls}"></span><span>{issue.text}</span></div>
          {/each}
        </div>
      {:else}
        <div class="routerforge-all-good"><span class="status-dot good"></span><strong>Критических событий нет</strong><span>Core, registry и установленные providers выглядят нормально.</span></div>
      {/if}
    </section>

    <section class="panel routerforge-platform-panel">
      <div class="panel-head">
        <div><strong>Платформа</strong><span>Состав текущей установки</span></div>
      </div>
      <div class="routerforge-platform-grid">
        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">DNS</span>
          <span class="routerforge-platform-copy">
            <strong>DNS</strong>
            <small>Наблюдение и диагностика</small>
          </span>
          <span class="state-chip {dns?.installed ? 'good' : 'neutral'}">{dns?.installed ? 'ENABLED' : 'NOT INSTALLED'}</span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">MON</span>
          <span class="routerforge-platform-copy">
            <strong>Monitoring</strong>
            <small>Системные providers</small>
          </span>
          <span class="state-chip {monitoringInstalled.length && monitoringOnline.length === monitoringInstalled.length ? 'good' : monitoringInstalled.length ? 'warn' : 'neutral'}">
            {monitoringInstalled.length ? `${monitoringOnline.length}/${monitoringInstalled.length} ONLINE` : 'NOT INSTALLED'}
          </span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">CTL</span>
          <span class="routerforge-platform-copy">
            <strong>Control</strong>
            <small>Администрирование роутера</small>
          </span>
          <span class="state-chip {admin?.installed && admin?.service_running ? 'good' : admin?.installed ? 'warn' : 'neutral'}">
            {admin?.installed ? admin?.service_running ? 'ONLINE' : 'INSTALLED' : 'NOT INSTALLED'}
          </span>
        </div>

        <div class="routerforge-platform-item">
          <span class="routerforge-platform-code mono">REG</span>
          <span class="routerforge-platform-copy">
            <strong>Registry</strong>
            <small>Источник Marketplace</small>
          </span>
          <span class="state-chip {$catalog.registry?.online ? 'info' : 'warn'}">{($catalog.registry?.source || '—').toUpperCase()}</span>
        </div>
      </div>
    </section>
  </div>

  <section class="routerforge-capabilities-section">
    <div class="catalog-section-head">
      <div><h2>Установленные возможности</h2><p>Главная показывает только то, что действительно установлено на этом роутере.</p></div>
      <a class="button" href="/catalog">Marketplace</a>
    </div>

    <div class="routerforge-capability-grid">
      {#if !moduleCards.length}
        <a class="routerforge-empty-capability" href="/catalog">
          <strong>Добавить возможности</strong>
          <span>DNS, мониторинг, управление и сторонние проекты доступны в Marketplace.</span>
        </a>
      {/if}
      {#each moduleCards as item (item.id)}
        <a class="routerforge-capability-card" href={hrefFor(item)}>
          <span class="routerforge-capability-icon mono">{short(item)}</span>
          <span class="routerforge-capability-copy">
            <strong>{item.name}</strong>
            <small>{item.description}</small>
          </span>
          <span class="state-chip {item.id === 'dns' || item.service_running ? 'good' : 'warn'}">
            {item.id === 'dns' ? 'ENABLED' : item.service_running ? 'ONLINE' : 'INSTALLED'}
          </span>
        </a>
      {/each}
    </div>
  </section>
</div>
