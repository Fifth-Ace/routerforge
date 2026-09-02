<script>
  import { page } from '$app/stores';
  import { snapshot, backendOnline, streamMode } from '$lib/stores/snapshot.js';
  import { timeOnly } from '$lib/utils.js';

  const items = [
    { href: '/', label: 'Обзор' },
    { href: '/servers', label: 'Серверы' },
    { href: '/routing', label: 'Маршрутизация' },
    { href: '/monitoring', label: 'Мониторинг' },
    { href: '/tools', label: 'Инструменты' },
    { href: '/admin', label: 'Админ' },
    { href: '/catalog', label: 'Каталог' },
    { href: '/settings', label: 'Настройки' }
  ];

  $: s = $snapshot || {};
  $: current = $page.url.pathname;
  $: state =
    s.capture_error ? 'ERROR'
      : s.active_down ? 'DOWN'
      : (s.active_degraded || s.active_quality_bad || s.active_quality_warn) ? 'DEGRADED'
      : 'OK';
  $: stateClass = state === 'OK' ? 'good' : state === 'DEGRADED' ? 'warn' : 'error';

  function active(href) {
    if (href === '/') return current === '/';
    return current === href || current.startsWith(`${href}/`);
  }
</script>

<header class="app-header">
  <div class="header-inner">
    <a class="brand" href="/" aria-label="DNS Monitor">
      <span class="brand-mark">D</span>
      <span class="brand-name">DNS Monitor</span>
      <span class="version-badge">v{s.version || '0.2.0-dev'}</span>
    </a>

    <nav class="main-nav" data-sveltekit-preload-code="eager" data-sveltekit-preload-data="hover">
      {#each items as item}
        <a class:active={active(item.href)} class="nav-link" href={item.href}>{item.label}</a>
      {/each}
    </nav>

    <div class="header-status mono">
      <span class="stream-label">{$streamMode.toUpperCase()}</span>
      <span class="status-dot" class:good={$backendOnline && stateClass === 'good'} class:warn={stateClass === 'warn'} class:error={!$backendOnline || stateClass === 'error'}></span>
      <span class={stateClass}>{$backendOnline ? state : 'OFFLINE'}</span>
      <span>{timeOnly(s.server_time || new Date())}</span>
    </div>
  </div>
</header>
