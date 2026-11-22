<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import ReleaseNotesWorkspace from '$lib/components/ReleaseNotesWorkspace.svelte';
  import { catalog, catalogOnline, catalogReady, startCatalogPolling } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { overview, refreshOverview, startOverviewPolling } from '$lib/stores/overview.js';
  import { t } from '$lib/i18n/index.js';
  import { railInstalledIntegrations } from '$lib/integrations.js';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { preferredReleaseNotesChannel, noteVersionForCore, latestPublishedNoteVersion, unreadPublishedNoteVersion } from '$lib/release-notes-feed.js';
  import { releaseNoteReads, loadReleaseNoteReads } from '$lib/release-notes-read-state.js';
  import { shouldShowServerAnnouncement, dismissServerAnnouncement } from '$lib/routerforge-server-preview.js';

  let stopCatalog = null;
  let stopCatalogSync = null;
  let stopOverview = null;
  let stopPageSync = null;
  let narrowRailOpen = true;
  let releaseNotesOpen = false;
  let publishedNoteVersions = { beta: '', stable: '' };
  let serverAnnouncementVisible = false;

  function closeServerAnnouncement() {
    serverAnnouncementVisible = false;
    try { dismissServerAnnouncement(window.localStorage); }
    catch { try { dismissServerAnnouncement(window.sessionStorage); } catch {} }
  }

  function toggleNarrowRail() {
    narrowRailOpen = !narrowRailOpen;
    try {
      localStorage.setItem('routerforge:narrow-rail-open', narrowRailOpen ? '1' : '0');
    } catch {}
  }

  onMount(() => {
    try {
      narrowRailOpen = localStorage.getItem('routerforge:narrow-rail-open') !== '0';
    } catch { /* Browsers may disable local storage; use session-only state. */ }
    loadReleaseNoteReads();
    try { serverAnnouncementVisible = shouldShowServerAnnouncement(window.localStorage); }
    catch { try { serverAnnouncementVisible = shouldShowServerAnnouncement(window.sessionStorage); }
      catch { serverAnnouncementVisible = true; } }

    // Core caches the published index for 15 minutes. A five-minute check and
    // visibility refresh pick up future releases without reinstalling RouterForge.
    let notesMounted = true;
    let notesChecking = false;
    async function refreshPublishedNotes() {
      if (!notesMounted || notesChecking) return;
      notesChecking = true;
      try {
        const response = await fetch('/api/release-notes/catalog', {
          credentials: 'same-origin', cache: 'no-store'
        });
        if (!response.ok) return;
        const result = await response.json();
        if (!notesMounted || !Array.isArray(result?.releases)) return;
        publishedNoteVersions = {
          beta: latestPublishedNoteVersion(result.releases, 'beta'),
          stable: latestPublishedNoteVersion(result.releases, 'stable')
        };
      } catch { /* Preserve the previous valid state while offline. */ }
      finally { notesChecking = false; }
    }
    function onNotesVisibility() {
      if (!document.hidden) void refreshPublishedNotes();
    }
    document.addEventListener('visibilitychange', onNotesVisibility);
    const notesTimer = window.setInterval(() => {
      if (!document.hidden) void refreshPublishedNotes();
    }, 5 * 60 * 1000);
    void refreshPublishedNotes();

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
      refreshOverview(value, $page.url.pathname === '/');
    });

    let lastOverviewPath = $page.url.pathname;
    stopPageSync = page.subscribe((value) => {
      const nextPath = value.url.pathname;
      if (nextPath === lastOverviewPath) return;
      const detailChanged = (lastOverviewPath === '/') !== (nextPath === '/');
      lastOverviewPath = nextPath;
      if (detailChanged && !document.hidden) {
        refreshOverview($catalog, nextPath === '/');
      }
    });

    stopOverview = startOverviewPolling(
      () => $catalog,
      15000,
      () => $page.url.pathname === '/'
    );
    return () => {
      notesMounted = false;
      window.clearInterval(notesTimer);
      document.removeEventListener('visibilitychange', onNotesVisibility);
      stopPageSync?.(); stopCatalogSync?.(); stopCatalog?.(); stopOverview?.();
    };
  });

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: coreModule = modules.find((item) => item.id === 'routerforge-core');
  $: installedCoreVersion = String(coreModule?.version || $snapshot.version || '').trim();
  $: releaseNotesChannel = preferredReleaseNotesChannel($catalog.release?.channel, installedCoreVersion);
  $: unreadReleaseVersion = unreadPublishedNoteVersion(
    publishedNoteVersions[releaseNotesChannel], $releaseNoteReads[releaseNotesChannel],
    noteVersionForCore(installedCoreVersion)
  );
  $: integrations = $catalog.integrations || [];
  $: installedModules = modules.filter((item) => item.installed && !item.builtin && item.id !== 'profiling');
  $: installedIntegrations = railInstalledIntegrations(integrations);
  $: telem = $overview || {};
  $: coreTelemetry = telem.platform?.telemetry || {};
  $: ramPct = Number(coreTelemetry.ram_usage_pct || 0);
  $: cpuPct = Number(coreTelemetry.cpu_usage_pct || 0);
  $: cpuTemp = Number(coreTelemetry.cpu_temp_c || 0);
  $: cpuTempWarning = Number($settings.cpuTempWarning || 75);
  $: model = telem.platform?.model || 'Keenetic';
  $: processes = Number(coreTelemetry.process_count || 0);

  function compactModuleName(item) {
    const original = String(item?.name || item?.id || 'Module').trim();
    const compact = original.replace(/^RouterForge(?:\s+|[-_:]+)*/i, '').trim();
    return compact || original;
  }

  function moduleIsOff(item) {
    if (!item || item.id === 'routerforge-core' || item.id === 'dns') return false;
    const hasServiceContract = Boolean(item.service) || Boolean(item.detection?.services?.length);
    return hasServiceContract && item.service_running === false;
  }

  function stateLabel(item) {
    if (item?.update_available) return t(locale, 'shell.statusUpdate');
    return moduleIsOff(item) ? t(locale, 'shell.statusOff') : t(locale, 'shell.statusOn');
  }
  function stateClass(item) {
    if (item?.update_available) return 'warn';
    return moduleIsOff(item) ? 'bad' : 'good';
  }
</script>

<div class="global-shell">
  <button
    class="rail-mobile-toggle"
    class:closed={!narrowRailOpen}
    type="button"
    aria-expanded={narrowRailOpen}
    aria-controls="routerforge-global-rail"
    onclick={toggleNarrowRail}
  >
    <span>{t(locale, 'shell.panel')}</span>
    <span class="rail-toggle-chevron" aria-hidden="true">{narrowRailOpen ? '▲' : '▼'}</span>
  </button>

  <aside
    id="routerforge-global-rail"
    class="global-rail global-rail-left"
    class:collapsed={!narrowRailOpen}
  >
    <div class="rail-main">
      <section class="rail-block">
        <div class="rail-section-label">RouterForge</div>
        <div class="global-module-tree mono">
          <div class="global-tree-root">
            <span class="status-dot {stateClass(coreModule)}"></span><strong>Core</strong>
            <span class="global-tree-state {stateClass(coreModule)}">[{stateLabel(coreModule)}]</span>
          </div>
          {#each installedModules as item, index (item.id)}
            <div class="global-tree-row">
              <span class="tree-branch">{index === installedModules.length - 1 ? '└─' : '├─'}</span>
              <span class="status-dot {stateClass(item)}"></span>
              <span class="global-tree-name" title={item.name}>{compactModuleName(item)}</span>
              <span class="global-tree-state {stateClass(item)}">[{stateLabel(item)}]</span>
            </div>
          {/each}
        </div>
      </section>

      {#if installedIntegrations.length}
        <section class="rail-block">
          <div class="rail-section-label">{t(locale, 'integrationsUi.title')}</div>
          <div class="global-module-tree mono">
            {#each installedIntegrations as item, index (item.id)}
              <div class="global-tree-row">
                <span class="tree-branch">{index === installedIntegrations.length - 1 ? '└─' : '├─'}</span>
                <span class="status-dot {stateClass(item)}"></span>
                <span class="global-tree-name" title={item.name}>{compactModuleName(item)}</span>
                <span class="global-tree-state {stateClass(item)}">[{stateLabel(item)}]</span>
              </div>
            {/each}
          </div>
        </section>
      {/if}
    </div>

    <div class="rail-bottom-stack">
      <section class="rail-status-card mono">
        <div class="rail-section-label">{t(locale, 'shell.hostTelemetry')}</div>
        <div><span>{t(locale, 'shell.model')}</span><strong>{model}</strong></div>
        <div><span>{t(locale, 'shell.cpuTemp')}</span><strong class:warn={cpuTemp >= cpuTempWarning && cpuTemp < 90} class:bad={cpuTemp >= 90}>{coreTelemetry.cpu_temp_available ? `${cpuTemp.toFixed(0)}°C` : '—'}</strong></div>
        <div><span>{t(locale, 'shell.cpuUsage')}</span><strong>{coreTelemetry.cpu_usage_available ? `${cpuPct.toFixed(0)}%` : '—'}</strong></div>
        <div><span>{t(locale, 'shell.ramUsage')}</span><strong>{coreTelemetry.ram_usage_available ? `${ramPct.toFixed(0)}%` : '—'}</strong></div>
        <div><span>{t(locale, 'monitoring.modules.processes')}</span><strong>{coreTelemetry.process_count_available ? processes : '—'}</strong></div>
      </section>

      <section class="rail-status-card mono">
        <div class="rail-section-label">{t(locale,'marketplace.pageTitle')}</div>
        <div><span>API</span><strong class={!$catalogReady ? '' : $catalogOnline ? 'good' : 'bad'}>{!$catalogReady ? '...' : $catalogOnline ? t(locale,'common.online').toUpperCase() : t(locale,'common.offline').toUpperCase()}</strong></div>
        <div><span>{t(locale, 'shell.registryLabel')}</span><strong class={!$catalogReady ? '' : $catalog.registry?.online ? 'good' : 'warn'}>{!$catalogReady ? '...' : ($catalog.registry?.source || 'BUNDLED').toUpperCase()}</strong></div>
        <div><span>{t(locale, 'shell.updates')}</span><strong class={[...modules,...integrations].some((x)=>x.update_available) ? 'warn' : 'good'}>{[...modules,...integrations].filter((x)=>x.update_available).length}</strong></div>
      </section>

      <div class="rail-community-links">
        <a class="rail-repository-link" href="https://github.com/Fifth-Ace/routerforge" target="_blank" rel="noreferrer" aria-label={t(locale,'shell.repositoryAria')}>
          <img src="/routerforge-mark.png" alt="" />
          <span class="rail-repository-copy"><small>{t(locale,'shell.sourceCode')}</small><strong>GitHub</strong></span>
          <span class="rail-repository-arrow" aria-hidden="true">↗</span>
        </a>

        <a class="rail-repository-link rail-portal-link" href="https://fifth-ace.github.io/routerforge-portal/" target="_blank" rel="noopener noreferrer" aria-label={t(locale, 'shell.portalSidebarAria')}>
          <span class="rail-link-icon rail-portal-icon" aria-hidden="true">▣</span>
          <span class="rail-repository-copy"><small>{t(locale, 'shell.portalSidebarEyebrow')}</small><strong>RouterForge Portal</strong></span>
          <span class="rail-repository-arrow" aria-hidden="true">↗</span>
        </a>

        <a class="rail-repository-link rail-telegram-link" href="https://t.me/RouterForge" target="_blank" rel="noreferrer" aria-label={t(locale, 'shell.telegramAria')}>
          <span class="rail-link-icon" aria-hidden="true">TG</span>
          <span class="rail-repository-copy"><small>{t(locale, 'shell.community')}</small><strong>Telegram</strong></span>
          <span class="rail-repository-arrow" aria-hidden="true">↗</span>
        </a>

        <button
          class="rail-repository-link rail-release-notes-link"
          class:unread={!!unreadReleaseVersion}
          type="button"
          aria-haspopup="dialog"
          aria-label={unreadReleaseVersion ? (locale === 'ru' ? `Что нового: непрочитанный выпуск ${unreadReleaseVersion}` : `What's new: unread release ${unreadReleaseVersion}`) : t(locale, 'shell.whatsNew')}
          onclick={() => releaseNotesOpen = true}
        >
          <span class="rail-link-icon rail-release-notes-icon" aria-hidden="true">✦</span>
          <span class="rail-repository-copy"><small>RouterForge</small><strong>{t(locale, 'shell.whatsNew')}{#if unreadReleaseVersion}<span class="rf-notes-new">NEW</span>{/if}</strong></span>
          <span class="rail-repository-arrow" aria-hidden="true">→</span>
        </button>
      </div>
      <a class="rail-legal-badge" href="/legal">{t(locale, 'shell.legalSummary')}</a>
    </div>
  </aside>

  <div class="global-content">
    {#if serverAnnouncementVisible}
      <aside class="rf-server-announcement" aria-label={locale === 'ru' ? 'Новинка RouterForge Server' : 'Introducing RouterForge Server'}>
        <span class="rf-server-announcement-icon" aria-hidden="true">▣</span>
        <div class="rf-server-announcement-copy">
          <strong>{locale === 'ru' ? 'Новинка: RouterForge Server' : 'New: RouterForge Server'}</strong>
          <span>{locale === 'ru'
            ? 'Новый консольный менеджер Linux/VPS уже доступен отдельно. Интеграция с RouterForge — в разработке.'
            : 'Our standalone Linux/VPS console manager is available. RouterForge integration is in development.'}</span>
        </div>
        <a class="button rf-server-announcement-detail" href="/server-hub" onclick={closeServerAnnouncement}>{locale === 'ru' ? 'Подробнее' : 'Learn more'}</a>
        <button type="button" class="rf-server-announcement-close" aria-label={locale === 'ru' ? 'Больше не показывать' : 'Dismiss announcement'} onclick={closeServerAnnouncement}>×</button>
      </aside>
    {/if}
    <slot />
  </div>
</div>

{#if releaseNotesOpen}
  <ReleaseNotesWorkspace {locale} onclose={() => releaseNotesOpen = false} />
{/if}

<style>
  .rail-release-notes-link {
    appearance: none;
    width: 100%;
    text-align: left;
    color: inherit;
    cursor: pointer;
    font: inherit;
  }

  .rail-release-notes-icon {
    color: var(--rf-accent, var(--accent));
  }
  /* Unread published releases: gentle highlight, never a disruptive popup. */
  .rail-release-notes-link.unread {
    border-color: var(--rf-accent, #38bdf8);
    animation: rf-notes-soft-glow 3.2s ease-in-out infinite;
  }
  @keyframes rf-notes-soft-glow {
    0%, 100% { background: transparent; box-shadow: 0 0 0 0 rgba(56,189,248,0); }
    50% { background: var(--rf-accent-soft, rgba(56,189,248,.12)); box-shadow: 0 0 17px rgba(56,189,248,.20); }
  }
  .rf-notes-new {
    display: inline-flex;
    margin-left: 7px;
    padding: 2px 5px;
    border-radius: 4px;
    background: var(--rf-accent, #38bdf8);
    color: var(--rf-bg, #07111b);
    font-size: 9px;
    font-weight: 800;
    line-height: 1;
    vertical-align: middle;
    letter-spacing: .04em;
  }
  @media (prefers-reduced-motion: reduce) {
    .rail-release-notes-link.unread { animation: none; background: var(--rf-accent-soft, rgba(56,189,248,.12)); }
  }

  /* A dismissible one-time announcement, not an auto-opening modal. */
  .rf-server-announcement {display:flex;align-items:center;gap:11px;padding:10px 12px;margin:10px 12px 0;border:1px solid var(--rf-border, #334155);border-radius:10px;background:var(--rf-card,var(--panel));color:var(--rf-text,var(--text));}
  .rf-server-announcement-icon {font-size:24px;color:var(--rf-accent,#38bdf8);}
  .rf-server-announcement-copy {flex:1;min-width:0;display:grid;gap:3px;font-size:12px;line-height:1.35;}
  .rf-server-announcement-copy strong {font-size:13px;}
  .rf-server-announcement-copy span {color:var(--rf-muted,var(--muted));}
  .rf-server-announcement-detail {white-space:nowrap;}
  .rf-server-announcement-close {border:0;background:transparent;color:inherit;font:inherit;font-size:22px;cursor:pointer;padding:3px 8px;border-radius:6px;}
  .rf-server-announcement-close:hover {background:var(--rf-accent-soft,rgba(56,189,248,.12));}
  @media (max-width:720px) {.rf-server-announcement {flex-wrap:wrap;margin:8px 8px 0;}.rf-server-announcement-detail {margin-left:auto;}}

  /* Four compact links occupy about the space of the previous three large tiles. */
  .rail-community-links{gap:5px}
  .rail-community-links .rail-repository-link{min-height:43px;padding:5px 9px;grid-template-columns:28px minmax(0,1fr) 12px;gap:8px;border-radius:7px}
  .rail-community-links .rail-repository-link img,.rail-community-links .rail-link-icon{width:28px;height:28px}
  .rail-community-links .rail-link-icon{font-size:10px}
  .rail-community-links .rail-portal-icon{font-size:17px}
  .rail-community-links .rail-repository-copy{gap:1px}
  .rail-community-links .rail-repository-arrow{font-size:13px}
</style>
