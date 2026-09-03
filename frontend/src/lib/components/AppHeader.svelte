<script>
  import { page } from '$app/stores';
  import { snapshot, backendOnline, streamMode } from '$lib/stores/snapshot.js';
  import { adminSummary, adminOnline } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline } from '$lib/stores/systemModule.js';
  import { authState, logoutAuth } from '$lib/stores/auth.js';

  const items = [
    { href: '/', label: 'Обзор' },
    { href: '/servers', label: 'Серверы' },
    { href: '/routing', label: 'Маршрутизация' },
    { href: '/monitoring', label: 'Мониторинг' },
    { href: '/tools', label: 'Инструменты' },
    { href: '/modules', label: 'Модули' },
    { href: '/admin', label: 'Админ' },
    { href: '/catalog', label: 'Каталог' },
    { href: '/settings', label: 'Настройки' }
  ];

  $: s = $snapshot || {};
  $: current = $page.url.pathname;
  $: hostTelemetry = $adminOnline ? $adminSummary : $systemModuleOnline ? $systemModuleSummary : null;
  $: primaryActiveDown = Number(s.primary_active_down ?? s.active_down ?? 0);
  $: primaryActiveDegraded = Number(s.primary_active_degraded ?? s.active_degraded ?? 0);
  $: primaryQualityBad = Number(s.primary_active_quality_bad ?? s.active_quality_bad ?? 0);
  $: primaryQualityWarn = Number(s.primary_active_quality_warn ?? s.active_quality_warn ?? 0);
  $: state =
    s.capture_error ? 'ERROR'
      : primaryActiveDown ? 'DOWN'
      : (primaryActiveDegraded || primaryQualityBad || primaryQualityWarn) ? 'DEGRADED'
      : 'OK';
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
    <a class="brand" href="/" aria-label="DNS Monitor Core Console">
      <span class="brand-mark concept-shield" aria-hidden="true">
        <svg viewBox="0 0 24 24"><path d="M12 3 20 6v5c0 5.2-3.2 8.2-8 10-4.8-1.8-8-4.8-8-10V6l8-3Z"/><path d="m8.7 12 2 2 4.8-5"/></svg>
      </span>
      <span class="brand-copy">
        <strong>DNS Monitor</strong>
        <span>Core Console</span>
      </span>
      <span class="version-badge">v{s.version || 'dev'}</span>
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
        <span class="header-meta"><i>KERNEL:</i><b>{hostTelemetry.kernel || '—'}</b></span>
      {/if}
      {#if $authState.required}
        <span class="header-separator"></span>
        <span class="header-auth"><i>AUTH:</i><b>{$authState.user || 'root'}</b><button type="button" onclick={logout}>Выйти</button></span>
      {/if}
      <span class="header-separator"></span>
      <span class="stream-label">{$streamMode.toUpperCase()}</span>
      <span class="core-state {stateClass}">
        <span class="status-dot {stateClass}"></span>
        CORE: {$backendOnline ? state : 'OFFLINE'}
      </span>
    </div>
  </div>
</header>
