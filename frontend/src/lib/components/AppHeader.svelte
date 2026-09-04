<script>
  import { page } from '$app/stores';
  import { snapshot, backendOnline, streamMode } from '$lib/stores/snapshot.js';
  import { adminSummary, adminOnline } from '$lib/stores/admin.js';
  import { systemModuleSummary, systemModuleOnline } from '$lib/stores/systemModule.js';
  import { catalog } from '$lib/stores/catalog.js';
  import { authState, logoutAuth } from '$lib/stores/auth.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  $: locale = $settings.locale || 'ru';
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
    { href: '/', labelKey: 'nav.home', order: 10 },
    ...(telemetryInstalled ? [{ href: '/monitoring', labelKey: 'nav.monitoring', order: 20 }] : []),
    ...dynamicModuleItems,
    { href: '/catalog', labelKey: 'nav.marketplace', order: 80 },
    { href: '/settings', labelKey: 'nav.settings', order: 90 }
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

  function navLabel(item) {
    if (item.labelKey) return t(locale, item.labelKey);
    const known = {
      '/manage': 'nav.manage',
      '/dns': null
    };
    const key = known[item.href];
    return key ? t(locale, key) : item.label;
  }

  function setLocale(value) {
    settings.update((currentSettings) => ({ ...currentSettings, locale: value }));
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
        <a class:active={active(item.href)} class="nav-link" href={item.href}>{navLabel(item)}</a>
      {/each}
    </nav>

    <div class="header-status mono">
      <div class="rf-language-switch" aria-label={t(locale, 'common.language')}>
        <button type="button" class:active={locale === 'ru'} aria-pressed={locale === 'ru'} onclick={() => setLocale('ru')}>RU</button>
        <span>/</span>
        <button type="button" class:active={locale === 'en'} aria-pressed={locale === 'en'} onclick={() => setLocale('en')}>EN</button>
      </div>
      <span class="header-separator"></span>
      {#if hostTelemetry}
        <span class="header-meta"><i>HOST:</i><b>{hostTelemetry.hostname || '—'}</b></span>
        <span class="header-separator"></span>
      {/if}
      {#if $authState.required}
        <span class="header-auth"><i>AUTH:</i><b>{$authState.user || 'root'}</b><button type="button" onclick={logout}>{t(locale, 'nav.logout')}</button></span>
        <span class="header-separator"></span>
      {/if}
      <span class="stream-label">{$streamMode.toUpperCase()}</span>
      <span class="core-state {stateClass}">
        <span class="status-dot {stateClass}"></span>
        CORE: {$backendOnline ? t(locale, 'common.online') : t(locale, 'common.offline')}
      </span>
    </div>
  </div>
</header>

<style>
  .rf-language-switch {
    display: inline-flex;
    align-items: center;
    gap: .22rem;
    white-space: nowrap;
  }

  .rf-language-switch button {
    appearance: none;
    border: 0;
    background: transparent;
    color: var(--rf-muted, var(--muted));
    font: inherit;
    font-weight: 600;
    padding: .15rem .18rem;
    cursor: pointer;
  }

  .rf-language-switch button:hover,
  .rf-language-switch button.active {
    color: var(--rf-text, var(--text));
  }

  .rf-language-switch button.active {
    text-decoration: underline;
    text-decoration-color: var(--rf-accent, var(--accent));
    text-underline-offset: .25rem;
  }

  .rf-language-switch span {
    color: var(--rf-border-strong, var(--border));
  }
</style>
