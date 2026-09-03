<script>
  import { page } from '$app/stores';
  import { snapshot, backendOnline, streamMode } from '$lib/stores/snapshot.js';
  import { adminSummary, adminOnline } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline } from '$lib/stores/systemModule.js';
  import { catalog } from '$lib/stores/catalog.js';
  import { authState, logoutAuth } from '$lib/stores/auth.js';

  $: modules = $catalog.modules || [];
  $: telemetryInstalled = modules.some((item) =>
    ['system', 'thermal', 'storage', 'network'].includes(item.id) && item.installed
  );
  $: dynamicModuleItems = modules
    .filter((item) => item.installed && item.presentation?.navigation?.href)
    .map((item) => ({
      href: item.presentation.navigation.href,
      label: item.presentation.navigation.label || item.name,
      order: Number(item.presentation.navigation.order || 50)
    }))
    .sort((a, b) => a.order - b.order);

  $: items = [
    { href: '/', label: 'Главная', order: 10 },
    ...(telemetryInstalled ? [{ href: '/monitoring', label: 'Мониторинг', order: 20 }] : []),
    ...dynamicModuleItems,
    { href: '/catalog', label: 'Marketplace', order: 80 },
    { href: '/settings', label: 'Настройки', order: 90 }
  ].sort((a, b) => a.order - b.order);

  $: s = $snapshot || {};
  $: current = $page.url.pathname;
  $: hostTelemetry = $adminOnline ? $adminSummary : $systemModuleOnline ? $systemModuleSummary : null;
  $: dnsInstalled = modules.some((item) => item.id === 'dns' && item.installed);
  $: primaryActiveDown = Number(s.primary_active_down ?? s.active_down ?? 0);
  $: primaryActiveDegraded = Number(s.primary_active_degraded ?? s.active_degraded ?? 0);
  $: primaryQualityBad = Number(s.primary_active_quality_bad ?? s.active_quality_bad ?? 0);
  $: primaryQualityWarn = Number(s.primary_active_quality_warn ?? s.active_quality_warn ?? 0);
  $: dnsState =
    s.capture_error ? 'ERROR'
      : primaryActiveDown ? 'DOWN'
      : (primaryActiveDegraded || primaryQualityBad || primaryQualityWarn) ? 'DEGRADED'
      : 'OK';
  $: state = dnsInstalled ? dnsState : 'OK';
  $: stateClass = state === 'OK' ? 'good' : state === 'DEGRADED' ? 'warn' : 'error';

  function active(href) {
    if (href === '/') return current === '/';
    return current === href || current.startsWith(`${href}/`);
  }

  async function logout() {
    await logoutAuth();
  }
</script>

<header class="app-header">
  <div class="header-inner">
    <a class="brand" href="/" aria-label="RouterForge">
      <img class="routerforge-header-mark" src="/routerforge-mark.png" alt="" aria-hidden="true"/>
      <span class="brand-copy">
        <strong>RouterForge</strong>
        <span>Router Console</span>
      </span>
      <span class="version-badge">v{s.version || 'beta'}</span>
    </a>

    <nav class="main-nav" data-sveltekit-preload-code="eager" data-sveltekit-preload-data="hover">
      {#each items as item}
        <a class:active={active(item.href)} class="nav-link" href={item.href}>{item.label}</a>
      {/each}
    </nav>

    <div class="header-status mono">
      {#if hostTelemetry}
        <span class="header-meta"><i>HOST:</i><b>{hostTelemetry.hostname || '—'}</b></span>
        <span class="header-separator"></span>
      {/if}
      {#if $authState.required}
        <span class="header-auth"><i>AUTH:</i><b>{$authState.user || 'root'}</b><button type="button" onclick={logout}>Выйти</button></span>
        <span class="header-separator"></span>
      {/if}
      <span class="stream-label">{$streamMode.toUpperCase()}</span>
      <span class="core-state {stateClass}">
        <span class="status-dot {stateClass}"></span>
        CORE: {$backendOnline ? 'ONLINE' : 'OFFLINE'}
      </span>
    </div>
  </div>
</header>
