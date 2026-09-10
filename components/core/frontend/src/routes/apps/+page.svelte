<script>
  import { onMount, onDestroy } from 'svelte';
  import { catalog, refreshCatalog, forceRefreshCatalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { authState } from '$lib/stores/auth.js';
  import {
    catalogAction, setCatalogChannel, getEntwarePackages,
    refreshEntwarePackages,
    getEntwarePackageDetail, preflightAppAction, startAppAction,
    getAppActions, getAppAction, cancelAppAction, appActionEventsURL, probeCatalogWeb
  } from '$lib/api.js';
  import { stateInfo, catalogWebURL, catalogWebPort, catalogWebSecurityDecision, catalogWebResolvedURL, catalogWebProbeStatusAllowed, bytes } from '$lib/utils.js';
  import { appText as a } from '$lib/app-center-i18n.js';
  import { t } from '$lib/i18n/index.js';
  import InstallPlanner from '$lib/components/InstallPlanner.svelte';
  import RemoveConfirm from '$lib/components/RemoveConfirm.svelte';
  import ExternalWebWorkspace from '$lib/components/ExternalWebWorkspace.svelte';

  const acronyms = {
    'awg-manager':'AWG', nfqws2:'NQ2', nfqws:'NQ1', 'nfqws-web':'NQW', 'hydraroute-neo':'HRN',
    'routerforge-core':'RFC', dns:'DNS', admin:'ADM', monitoring:'MON', system:'SYS', thermal:'TMP', storage:'DSK', network:'NET',
    admin:'ADM', profiling:'PRF', xkeen:'XKN', 'xkeen-ui':'XUI', 'keen-pbr':'PBR', kvas:'KVS',
    'bypass-keenetic':'BYP', 'traffic-via-vpn':'VPN', 'adguardhome-keenetic':'AGH', skeen:'SKN',
    'chur-keenetic':'CHR', 'keenetic-sing-box-ui':'SBU', 'keenetic-entware-extras':'KEE'
  };

  let tab = 'routerforge';
  let search = '';
  let plannerItem = null;
  let removeItem = null;
  let webWorkspace = null;
  let webProbeBusyId = '';
  let busyId = '';
  let busyAction = '';
  let actionNotice = null;
  let channelBusy = false;
  let checkingUpdates = false;
  let bulkUpdating = false;
  let entwareLoading = false;
  let entwareRefreshing = false;
  let entwareState = 'all';
  let entwareData = { items:[], total:0, offset:0, limit:100, installed_count:0, upgradable_count:0, online:false };
  let activeJob = null;
  let actionLog = [];
  let actionHistory = [];
  let actionHistoryExpanded = false;
  let actionEvents = null;
  let entwareDetail = null;

  onMount(() => {
    void refreshActionHistory();
    void openRequestedCatalogWeb();
  });

  onDestroy(() => {
    actionEvents?.close();
  });

  $: locale = $settings.locale || 'ru';
  $: data = $catalog || { modules:[], integrations:[], read_only:true, package_management_enabled:false };
  $: packageMode = Boolean(data.package_management_enabled ?? data.install_test_mode);
  $: releaseChannel = data.release?.channel || 'beta';
  $: releaseTarget = data.release?.target || '—';
  $: modules = data.modules || [];
  $: integrations = data.integrations || [];
  $: routerForgeUpdates = modules.filter(hasCatalogUpdate);
  $: officialRouterForgeUpdates = routerForgeUpdates.filter(isOfficialRouterForgeUpdate);
  $: integrationUpdates = integrations.filter(hasCatalogUpdate);
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

  async function openRequestedCatalogWeb() {
    const params = new URLSearchParams(window.location.search);
    const requestedTab = String(params.get('tab') || '').trim();

    if (['routerforge','integrations','entware','installed','updates'].includes(requestedTab)) {
      setTab(requestedTab);
    }

    const requestedID = String(params.get('open') || '').trim();
    if (!requestedID) return;

    params.delete('open');
    const query = params.toString();
    window.history.replaceState(
      window.history.state,
      '',
      `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`
    );

    let current = $catalog || { modules:[], integrations:[] };
    let item = [...(current.modules || []), ...(current.integrations || [])]
      .find((candidate) => candidate.id === requestedID);

    if (!item) {
      current = await refreshCatalog() || { modules:[], integrations:[] };
      item = [...(current.modules || []), ...(current.integrations || [])]
        .find((candidate) => candidate.id === requestedID);
    }

    if (!item || !item.installed || !catalogWebURL(item)) {
      actionNotice = {
        cls:'warn',
        text: locale === 'ru'
          ? '\u0418\u043d\u0442\u0435\u0433\u0440\u0430\u0446\u0438\u044f \u043d\u0435 \u0433\u043e\u0442\u043e\u0432\u0430 \u043a \u043e\u0442\u043a\u0440\u044b\u0442\u0438\u044e \u0432 Web workspace.'
          : 'The integration is not ready to open in the Web workspace.'
      };
      return;
    }

    await openEmbeddedWeb(item);
  }

  function hasCatalogUpdate(item) {
    return Boolean(item?.installed && item?.update_available && item?.actions?.update);
  }
  function isOfficialRouterForgeUpdate(item) {
    return Boolean(
      hasCatalogUpdate(item)
      && (item?.id === 'routerforge-core' || item?.publisher?.id === 'routerforge' || item?.source === 'routerforge-official')
    );
  }

  function filterCatalog(items, query) {
    const q = String(query || '').trim().toLowerCase();
    if (!q) return items;
    return items.filter((item) =>
      `${item.name} ${item.category || ''} ${item.description || ''} ${item.version || ''} ${item.available_version || ''} ${(item.detection?.packages || []).join(' ')} ${item.publisher?.name || ''}`
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

  function descriptionText(item) {
    if (item?.kind === 'module') {
      const key = `moduleDescriptions.${item.id}`;
      const translated = a(locale, key);
      if (translated !== key) return translated;
    }
    return item?.description || '';
  }

  function hasServiceContract(item) {
    if (item?.id === 'routerforge-core') return true;
    return Boolean(item?.service) || Boolean(item?.detection?.services?.length);
  }

  function serviceStatusText(item) {
    if (item?.id === 'routerforge-core') return a(locale,'coreService');
    return item?.service_running ? a(locale,'running') : a(locale,'stopped');
  }

  function coreRestartTransportError(error) {
    if (error?.status || error?.payload) return false;
    const message = String(error?.message || '').toLowerCase();
    return message.includes('failed to fetch')
      || message.includes('networkerror')
      || message.includes('network request failed')
      || message.includes('load failed');
  }

  async function waitForCoreRecovery(targetVersion, timeoutMs = 45000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const refreshed = await refreshCatalog();
      const core = (refreshed?.modules || []).find((candidate) => candidate.id === 'routerforge-core');
      if (core?.installed && (!targetVersion || String(core.version || '') === String(targetVersion))) {
        return core;
      }
      await new Promise((resolve) => setTimeout(resolve, 750));
    }
    return null;
  }

  function jobStateLabel(state) {
    const value = String(state || 'queued').toLowerCase();
    const keys = {
      queued:'jobQueued',
      running:'jobRunning',
      succeeded:'jobSucceeded',
      failed:'jobFailed',
      cancelled:'jobCancelled'
    };
    return a(locale, keys[value] || 'jobQueued');
  }

  function moduleURL(item) {
    if (item.id === 'admin') return '/manage';
    if (item.id === 'dns') return '/dns';
    if (item.id === 'monitoring') return '/monitoring';
    if (['system','thermal','storage','network'].includes(item.id)) return `/monitoring?tab=${encodeURIComponent(item.id)}`;
    return '';
  }

  function canAction(item, action) {
    return packageMode && Boolean(item.actions?.[action]);
  }

  function webSecurityDecision(item) {
    return catalogWebSecurityDecision(item, $authState);
  }

  function showWebSecurityNotice(item) {
    const decision = webSecurityDecision(item);
    const text = decision.reason === 'session-cookie-cross-port'
      ? (locale === 'ru'
        ? '\u0412\u043D\u0435\u0448\u043D\u0438\u0439 Web UI \u0437\u0430\u0431\u043B\u043E\u043A\u0438\u0440\u043E\u0432\u0430\u043D: cookie \u0441\u0435\u0441\u0441\u0438\u0438 RouterForge \u043D\u0435 \u0438\u0437\u043E\u043B\u0438\u0440\u0443\u044E\u0442\u0441\u044F \u043F\u043E TCP-\u043F\u043E\u0440\u0442\u0443.'
        : 'External Web UI is blocked because RouterForge session cookies are not isolated by TCP port.')
      : decision.reason === 'mixed-content'
        ? (locale === 'ru'
          ? '\u0412\u0441\u0442\u0440\u043E\u0435\u043D\u043D\u044B\u0439 \u0440\u0435\u0436\u0438\u043C \u0437\u0430\u0431\u043B\u043E\u043A\u0438\u0440\u043E\u0432\u0430\u043D: HTTPS RouterForge \u043D\u0435 \u043C\u043E\u0436\u0435\u0442 \u0432\u0441\u0442\u0440\u0430\u0438\u0432\u0430\u0442\u044C HTTP Web UI.'
          : 'Embedded mode is blocked because HTTPS RouterForge cannot embed an HTTP Web UI.')
        : (locale === 'ru' ? '\u0412\u043D\u0435\u0448\u043D\u0438\u0439 Web UI \u0441\u0435\u0439\u0447\u0430\u0441 \u043D\u0435\u0434\u043E\u0441\u0442\u0443\u043F\u0435\u043D.' : 'External Web UI is not available right now.');

    actionNotice = { cls:'warn', text };
  }

  async function openEmbeddedWeb(item) {
    const decision = webSecurityDecision(item);
    if (!decision.embedAllowed) {
      showWebSecurityNotice(item);
      return;
    }

    webProbeBusyId = item.id;
    try {
      const probe = await probeCatalogWeb(item.id);
      const safe = probe?.reachable === true
        && catalogWebProbeStatusAllowed(item, probe)
        && probe?.redirect === false
        && ['embedded-supported', 'probe-required'].includes(probe?.mode)
        && probe?.embed === true
        && probe?.frame_header_policy === 'no-blocking-header-detected';

      const frameBlocked = probe?.reachable === true
        && probe?.frame_header_policy === 'blocked';

      if (frameBlocked) {
        actionNotice = {
          cls:'warn',
          text: locale === 'ru'
            ? 'Web UI \u043e\u0431\u043d\u0430\u0440\u0443\u0436\u0435\u043d, \u043d\u043e \u0441\u0430\u043c\u043e \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0435 \u0437\u0430\u043f\u0440\u0435\u0449\u0430\u0435\u0442 \u0432\u0441\u0442\u0440\u0430\u0438\u0432\u0430\u043d\u0438\u0435 \u0447\u0435\u0440\u0435\u0437 X-Frame-Options/CSP.'
            : 'Web UI was detected, but the application itself blocks embedding through X-Frame-Options/CSP.'
        };
        return;
      }

      if (!safe) {
        actionNotice = {
          cls:'warn',
          text: locale === 'ru'
            ? '\u0412\u0441\u0442\u0440\u0430\u0438\u0432\u0430\u043D\u0438\u0435 \u043E\u0442\u043C\u0435\u043D\u0435\u043D\u043E: runtime web-probe \u043D\u0435 \u043F\u043E\u0434\u0442\u0432\u0435\u0440\u0434\u0438\u043B \u0431\u0435\u0437\u043E\u043F\u0430\u0441\u043D\u044B\u0439 iframe.'
            : 'Embed cancelled because the runtime web probe did not confirm a safe iframe.'
        };
        return;
      }

      const resolvedURL = catalogWebResolvedURL(item, probe, $authState);
      if (!resolvedURL) {
        actionNotice = {
          cls:'warn',
          text: locale === 'ru'
            ? '\u0412\u0441\u0442\u0440\u0430\u0438\u0432\u0430\u043d\u0438\u0435 \u043e\u0442\u043c\u0435\u043d\u0435\u043d\u043e: \u043d\u0435 \u043d\u0430\u0439\u0434\u0435\u043d \u0431\u0435\u0437\u043e\u043f\u0430\u0441\u043d\u044b\u0439 Web UI listener, \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0439 \u0431\u0440\u0430\u0443\u0437\u0435\u0440\u0443.'
            : 'Embed cancelled because no safe browser-reachable Web UI listener was resolved.'
        };
        return;
      }

      webWorkspace = { item, url:resolvedURL, probe };
    } catch (error) {
      actionNotice = {
        cls:'warn',
        text:error?.payload?.error || error?.message || 'web probe failed'
      };
    } finally {
      webProbeBusyId = '';
    }
  }

  function trustLabel(item) {
    if (item?.kind === 'integration' && !item?.manifest_id) {
      return locale === 'ru' ? '\u0411\u0415\u0417 \u041c\u0410\u041d\u0418\u0424\u0415\u0421\u0422\u0410' : 'NO MANIFEST';
    }
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

  function catalogWarning(item) {
    const status = String(item?.trust?.status || 'unverified').toLowerCase();
    if (item?.kind === 'integration' && (!item?.manifest_id || status === 'unverified')) {
      return locale === 'ru'
        ? '\u041c\u0430\u043d\u0438\u0444\u0435\u0441\u0442 \u043e\u0442\u0441\u0443\u0442\u0441\u0442\u0432\u0443\u0435\u0442 \u0438\u043b\u0438 \u043d\u0435 \u043f\u0440\u043e\u0432\u0435\u0440\u0435\u043d. \u0414\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0435 \u0434\u0435\u0439\u0441\u0442\u0432\u0438\u044f \u0438\u0441\u043f\u043e\u043b\u044c\u0437\u0443\u044e\u0442 \u0442\u043e\u043b\u044c\u043a\u043e \u0443\u0436\u0435 \u043d\u0430\u0441\u0442\u0440\u043e\u0435\u043d\u043d\u044b\u0439 opkg feed; upstream-\u0441\u043a\u0440\u0438\u043f\u0442\u044b \u0430\u0432\u0442\u043e\u043c\u0430\u0442\u0438\u0447\u0435\u0441\u043a\u0438 \u043d\u0435 \u0437\u0430\u043f\u0443\u0441\u043a\u0430\u044e\u0442\u0441\u044f.'
        : 'Manifest is missing or unverified. Available actions use only an already configured opkg feed; upstream scripts are never executed automatically.';
    }
    if (item?.builtin) return '';
    const hasAction = Boolean(item?.actions?.install || item?.actions?.update || item?.actions?.remove);
    return hasAction ? '' : (item?.actions?.reason || '');
  }

  function entwareMode() {
    if (tab === 'installed') return 'installed';
    if (tab === 'updates') return 'upgradable';
    return entwareState === 'all' ? '' : entwareState;
  }

  async function refreshActionHistory() {
    try {
      const result = await getAppActions();
      actionHistory = result?.items || [];
    } catch {
      actionHistory = [];
    }
  }

  function jobStateClass(state) {
    if (state === 'succeeded') return 'good';
    if (state === 'failed') return 'error';
    if (state === 'cancelled') return 'warn';
    return 'info';
  }

  async function refreshAfterPackageAction() {
    try {
      const result = await forceRefreshCatalog();
      const nextCatalog = result?.catalog || null;
      const remote = Boolean(nextCatalog?.registry?.online && String(nextCatalog?.registry?.source || '').toLowerCase() === 'remote');
      return { catalog:nextCatalog, remote };
    } catch {
      const fallback = await refreshCatalog();
      return { catalog:fallback, remote:false };
    }
  }

  function registryFallbackText() {
    return locale === 'ru'
      ? 'Пакет обновлён, но registry пока читается из кеша. Повторная синхронизация будет выполнена при следующей проверке.'
      : 'Package updated, but the registry is still served from cache. Remote sync will be retried on the next check.';
  }

  async function finishWatchedAction(state, error = '') {
    if (!activeJob) return;
    const finished = activeJob;
    activeJob = { ...activeJob, state, error };
    busyId = '';
    busyAction = '';
    actionEvents?.close();
    actionEvents = null;
    const refreshResult = state === 'succeeded'
      ? await refreshAfterPackageAction()
      : { catalog:await refreshCatalog(), remote:true };
    if (['entware','installed','updates'].includes(tab)) await loadEntware(entwareData.offset || 0);
    await refreshActionHistory();
    actionNotice = state === 'succeeded'
      ? refreshResult.remote
        ? { cls:'good', text: locale === 'ru' ? `\u0414\u0435\u0439\u0441\u0442\u0432\u0438\u0435 \u0437\u0430\u0432\u0435\u0440\u0448\u0435\u043d\u043e: ${finished.target}` : `Action completed: ${finished.target}` }
        : { cls:'warn', text:registryFallbackText() }
      : { cls: state === 'cancelled' ? 'warn' : 'error', text: error || (locale === 'ru' ? `\u0414\u0435\u0439\u0441\u0442\u0432\u0438\u0435 ${state}: ${finished.target}` : `Action ${state}: ${finished.target}`) };
  }

  async function pollAction(id) {
    for (let i = 0; i < 190; i += 1) {
      try {
        const job = await getAppAction(id);
        activeJob = job;
        actionLog = job.lines || actionLog;
        if (['succeeded','failed','cancelled'].includes(job.state)) {
          await finishWatchedAction(job.state, job.error || '');
          return;
        }
      } catch (error) {
        actionNotice = { cls:'warn', text:error?.payload?.error || error?.message || 'action polling failed' };
      }
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
  }

  function watchActionJob(job, busyTarget) {
    activeJob = job;
    actionLog = job.lines || [];
    busyId = busyTarget;
    busyAction = job.action || '';
    actionEvents?.close();

    const source = new EventSource(appActionEventsURL(job.id));
    actionEvents = source;
    source.onmessage = async (message) => {
      let event;
      try { event = JSON.parse(message.data); } catch { return; }
      if (event.type === 'line') {
        actionLog = [...actionLog, event.line].slice(-320);
        return;
      }
      if (event.type === 'state') {
        activeJob = { ...activeJob, state:event.state, error:event.error || '' };
        if (['succeeded','failed','cancelled'].includes(event.state)) {
          await finishWatchedAction(event.state, event.error || '');
        }
      }
    };
    source.onerror = () => {
      source.close();
      if (actionEvents === source) actionEvents = null;
      if (activeJob && !['succeeded','failed','cancelled'].includes(activeJob.state)) {
        void pollAction(job.id);
      }
    };
  }

  function preflightMessage(preflight, title) {
    const lines = [title];
    if (preflight?.method) lines.push(`${a(locale,'method')}: ${preflight.method}`);
    if (preflight?.packages?.length) lines.push(`${a(locale,'packages')}: ${preflight.packages.join(', ')}`);
    const entware = preflight?.entware;
    if (entware?.architecture) lines.push(`${a(locale,'architecture')}: ${entware.architecture}`);
    if (entware?.download_size_bytes) lines.push(`${a(locale,'download')}: ${bytes(entware.download_size_bytes)}`);
    if (entware?.installed_size_bytes) lines.push(`${a(locale,'installedSize')}: ${bytes(entware.installed_size_bytes)}`);
    if (entware?.dependencies?.length) lines.push(`${a(locale,'depends')}: ${entware.dependencies.join(', ')}`);
    if (entware?.reverse_dependencies?.length) lines.push(`${a(locale,'reverseDepends')}: ${entware.reverse_dependencies.join(', ')}`);
    if (preflight?.warnings?.length) lines.push('', ...preflight.warnings.map((value) => `! ${value}`));
    return lines.join('\n');
  }

  async function runAsyncCatalogAction(item, action, confirm = '', skipConfirm = false) {
    if (busyId) return;
    try {
      const request = { kind:'catalog', target:item.id, action, confirm };
      const preflight = await preflightAppAction(request);
      if (!preflight.allowed) {
        actionNotice = { cls:'error', text:preflight.reason || 'preflight rejected action' };
        return;
      }
      const title = `${action === 'install' ? a(locale,'install') : action === 'remove' ? a(locale,'remove') : a(locale,'update')} ${item.name}?`;
      if (!skipConfirm && !window.confirm(preflightMessage(preflight, title))) return;
      const job = await startAppAction(request);
      if (action === 'remove') removeItem = null;
      watchActionJob(job, item.id);
    } catch (error) {
      busyId = '';
      busyAction = '';
      actionNotice = { cls:'error', text:error?.payload?.detail || error?.payload?.error || error?.message || 'error' };
    }
  }

  async function showEntwareDetail(pkg) {
    try {
      entwareDetail = await getEntwarePackageDetail(pkg.name);
    } catch (error) {
      actionNotice = { cls:'error', text:error?.payload?.error || error?.message || 'error' };
    }
  }

  async function cancelCurrentAction() {
    if (!activeJob?.id || ['succeeded','failed','cancelled'].includes(activeJob.state)) return;
    try {
      await cancelAppAction(activeJob.id);
    } catch (error) {
      actionNotice = { cls:'error', text:error?.payload?.error || error?.message || 'cancel failed' };
    }
  }
  async function runCatalogAction(item, action, confirm = '', skipConfirm = false) {
    if (!canAction(item, action) || busyId) return;
    if (item.id !== 'routerforge-core') {
      await runAsyncCatalogAction(item, action, confirm, skipConfirm);
      return;
    }
    if (!skipConfirm && !window.confirm(`${action === 'install' ? a(locale,'install') : a(locale,'update')} ${item.name}?`)) return;

    const targetVersion = String(item.release?.version || item.available_version || '').trim();
    busyId = item.id;
    busyAction = action;
    actionNotice = { cls:'warn', text:a(locale,'coreRestarting') };

    try {
      try {
        await catalogAction(item.id, action, confirm);
      } catch (error) {
        if (!coreRestartTransportError(error)) throw error;
      }

      const recovered = await waitForCoreRecovery(targetVersion);
      if (!recovered) {
        throw new Error(a(locale,'coreRecoveryTimeout',{version:targetVersion || '?'}));
      }

      const registryRefresh = await refreshAfterPackageAction();
      actionNotice = registryRefresh.remote
        ? { cls:'good', text:a(locale,'actionDone',{name:item.name}) }
        : { cls:'warn', text:registryFallbackText() };
      if (action === 'remove') removeItem = null;
      setTimeout(() => window.location.reload(), 300);
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

    const confirmKey = next === 'dev'
      ? 'channelDev'
      : next === 'beta'
        ? 'channelBeta'
        : 'channelStable';

    if (!window.confirm(a(locale, confirmKey))) {
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

  async function waitForCatalogItemVersion(id, targetVersion, timeoutMs = 60000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const refreshed = await refreshCatalog();
      const item = (refreshed?.modules || []).find((candidate) => candidate.id === id);
      if (item?.installed && (!targetVersion || String(item.version || '') === String(targetVersion))) return item;
      await new Promise((resolve) => setTimeout(resolve, 750));
    }
    return null;
  }

  async function recheckBulkPreflight(item, request, preflight) {
    if (preflight?.allowed) return { item, allowed:true };

    const refreshed = await refreshAfterPackageAction();
    const current = (refreshed.catalog?.modules || []).find((candidate) => candidate.id === item.id);
    if (!isOfficialRouterForgeUpdate(current)) {
      return { item:current, allowed:false, alreadyCurrent:true };
    }

    const retryPreflight = await preflightAppAction(request);
    if (retryPreflight?.allowed) return { item:current, allowed:true };
    throw new Error(`${item.name || item.id}: ${retryPreflight?.reason || preflight?.reason || 'preflight rejected update'}`);
  }

  async function runBulkRestartingUpdate(item) {
    const targetVersion = String(item.release?.version || item.available_version || '').trim();
    busyId = item.id;
    busyAction = 'update';
    try {
      if (item.id !== 'routerforge-core') {
        const request = { kind:'catalog', target:item.id, action:'update', confirm:'' };
        const preflight = await preflightAppAction(request);
        const checked = await recheckBulkPreflight(item, request, preflight);
        if (checked.alreadyCurrent) return;
        item = checked.item || item;
      }

      try {
        await catalogAction(item.id, 'update', '');
      } catch (error) {
        if (!coreRestartTransportError(error)) {
          const refreshed = await refreshAfterPackageAction();
          const current = (refreshed.catalog?.modules || []).find((candidate) => candidate.id === item.id);
          if (!isOfficialRouterForgeUpdate(current)) return;
          throw new Error(`${item.name || item.id}: ${error?.payload?.detail || error?.payload?.error || error?.message || 'update failed'}`);
        }
      }

      const recovered = item.id === 'routerforge-core'
        ? await waitForCoreRecovery(targetVersion, 60000)
        : await waitForCatalogItemVersion(item.id, targetVersion, 60000);
      if (!recovered) throw new Error(`RouterForge ${item.name || item.id} recovery timeout (${targetVersion || '?'})`);

      if (item.id === 'routerforge-core') {
        await refreshAfterPackageAction();
      }
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  async function runBulkModuleUpdate(item) {
    const request = { kind:'catalog', target:item.id, action:'update', confirm:'' };
    const preflight = await preflightAppAction(request);
    const checked = await recheckBulkPreflight(item, request, preflight);
    if (checked.alreadyCurrent) return;
    item = checked.item || item;

    const job = await startAppAction(request);
    activeJob = job;
    actionLog = job.lines || [];
    busyId = item.id;
    busyAction = 'update';

    try {
      for (let i = 0; i < 190; i += 1) {
        const current = await getAppAction(job.id);
        activeJob = current;
        actionLog = current.lines || actionLog;
        if (current.state === 'succeeded') {
          await refreshCatalog();
          return;
        }
        if (current.state === 'failed' || current.state === 'cancelled') {
          throw new Error(current.error || `${item.name || item.id}: ${current.state}`);
        }
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
      throw new Error(`${item.name || item.id}: update timeout`);
    } finally {
      busyId = '';
      busyAction = '';
    }
  }

  async function updateAllRouterForge() {
    if (bulkUpdating || busyId) return;
    const initial = [...officialRouterForgeUpdates];
    if (!initial.length) return;

    const names = initial.map((item) => item.name || item.id).join('\n• ');
    const question = locale === 'ru'
      ? `Обновить все официальные компоненты RouterForge (${initial.length})?\n\n• ${names}\n\nСторонние интеграции и Entware-пакеты затронуты не будут.`
      : `Update all official RouterForge components (${initial.length})?\n\n• ${names}\n\nThird-party integrations and Entware packages will not be touched.`;
    if (!window.confirm(question)) return;

    const priority = (item) => item.id === 'routerforge-core' ? 0 : item.id === 'profiling' ? 20 : 10;
    const queue = [...initial].sort((a, b) => priority(a) - priority(b) || String(a.id).localeCompare(String(b.id)));

    bulkUpdating = true;
    try {
      for (let index = 0; index < queue.length; index += 1) {
        const refreshed = await refreshCatalog();
        const current = (refreshed?.modules || []).find((candidate) => candidate.id === queue[index].id);
        if (!isOfficialRouterForgeUpdate(current)) continue;

        actionNotice = {
          cls:'warn',
          text: locale === 'ru'
            ? `RouterForge: обновление ${index + 1}/${queue.length} — ${current.name || current.id}`
            : `RouterForge: updating ${index + 1}/${queue.length} — ${current.name || current.id}`
        };

        if (current.id === 'routerforge-core' || current.id === 'profiling') {
          await runBulkRestartingUpdate(current);
        } else {
          await runBulkModuleUpdate(current);
        }
      }

      const registryRefresh = await refreshAfterPackageAction();
      await refreshActionHistory();
      actionNotice = registryRefresh.remote
        ? {
            cls:'good',
            text: locale === 'ru' ? 'Все доступные обновления RouterForge установлены.' : 'All available RouterForge updates are installed.'
          }
        : { cls:'warn', text:registryFallbackText() };
      setTimeout(() => window.location.reload(), 450);
    } catch (error) {
      actionNotice = { cls:'error', text:error?.payload?.detail || error?.payload?.error || error?.message || 'bulk update failed' };
      await refreshCatalog();
      await refreshActionHistory();
    } finally {
      bulkUpdating = false;
      busyId = '';
      busyAction = '';
    }
  }
  function reviewRouterForgeUpdates() {
    setTab('updates');
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
      await refreshCatalog();
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
    const confirm = action === 'remove' ? pkg.name : '';
    const request = { kind:'entware', target:pkg.name, action, confirm };
    try {
      const preflight = await preflightAppAction(request);
      if (!preflight.allowed) {
        actionNotice = { cls:'error', text:preflight.reason || 'preflight rejected action' };
        return;
      }
      const question = action === 'install' ? a(locale,'confirmInstall',{name:pkg.name})
        : action === 'update' ? a(locale,'confirmUpdate',{name:pkg.name})
        : a(locale,'confirmRemove',{name:pkg.name});
      if (!window.confirm(preflightMessage(preflight, question))) return;
      const job = await startAppAction(request);
      watchActionJob(job, `entware:${pkg.name}`);
    } catch (error) {
      busyId = '';
      busyAction = '';
      actionNotice = { cls:'error', text:a(locale,'actionFailed',{name:pkg.name,error:error?.payload?.detail || error?.payload?.error || error?.message || 'error'}) };
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

  </div>

  <div class="subtabs app-center-tabs">
    {#each tabs as entry (entry[0])}
      <button class:active={tab === entry[0]} onclick={() => setTab(entry[0])}>
        {entry[1]} {#if entry[2] !== ''}<span class="pill">{entry[2]}</span>{/if}
      </button>
    {/each}
  </div>

  <div class="toolbar catalog-toolbar-v3">
    <div class="catalog-search-group">
      <div class="search-control flex">
        <span>⌕</span>
        <input bind:value={search} onkeydown={(event) => { if (event.key === 'Enter' && ['entware','installed','updates'].includes(tab)) void loadEntware(0); }} placeholder={a(locale,'search')}/>
      </div>
      {#if ['entware','installed','updates'].includes(tab)}
        <button class="button search-submit" disabled={entwareLoading} onclick={() => loadEntware(0)}>{a(locale,'searchButton')}</button>
      {/if}
    </div>

    <div class="catalog-control-group">
      {#if tab === 'entware'}
        <select class="entware-state-select" bind:value={entwareState} onchange={() => loadEntware(0)}>
          <option value="all">{a(locale,'all')}</option>
          <option value="installed">{a(locale,'installedOnly')}</option>
          <option value="available">{a(locale,'availableOnly')}</option>
          <option value="upgradable">{a(locale,'tabs.updates')}</option>
        </select>
      {/if}

      <select class="channel-select" aria-label="RouterForge channel" value={releaseChannel} disabled={channelBusy || Boolean(busyId)} onchange={changeReleaseChannel}>
        <option value="stable">RouterForge Stable</option>
        <option value="beta">RouterForge Beta</option>
        <option value="dev">RouterForge Dev</option>
      </select>

      <span
        class="registry-chip state-chip {data.registry?.online && String(data.registry?.source || '').toLowerCase() === 'remote' ? 'good' : data.registry?.source === 'cache' ? 'warn' : 'neutral'}"
        title={data.registry?.last_sync || data.registry?.error || ''}
      >
        {a(locale,'registry')} {(data.registry?.source || 'BUNDLED').toUpperCase()}
      </span>

      <button class="button check-updates-button" disabled={checkingUpdates || Boolean(busyId)} onclick={checkForUpdates}>
        {checkingUpdates ? a(locale,'checking') : a(locale,'checkUpdates')}
      </button>

      {#if tab === 'entware' && packageMode}
        <button class="button" disabled={entwareRefreshing || Boolean(busyId)} onclick={refreshEntware}>
          {entwareRefreshing ? a(locale,'updatingLists') : a(locale,'updateLists')}
        </button>
      {/if}

      {#if tab === 'routerforge' && packageMode && officialRouterForgeUpdates.length}
        <button class="button primary" disabled={bulkUpdating || Boolean(busyId)} onclick={updateAllRouterForge}>
          {bulkUpdating ? (locale === 'ru' ? 'Обновляем RouterForge…' : 'Updating RouterForge…') : a(locale,'routerforgeUpdateAll')} ({officialRouterForgeUpdates.length})
        </button>
      {/if}

      {#if tab === 'routerforge' && packageMode && routerForgeUpdates.length}
        <button class="button" disabled={bulkUpdating || Boolean(busyId)} onclick={reviewRouterForgeUpdates}>
          {locale === 'ru' ? 'Просмотреть обновления' : 'Review updates'} ({routerForgeUpdates.length})
        </button>
      {/if}
    </div>
  </div>

  <div class="market-safety-line mono" class:test-mode={packageMode}>
    <span><i class="status-dot {packageMode ? 'good' : 'neutral'}"></i> {a(locale,'packageManagement')} <strong>{packageMode ? a(locale,'active').toUpperCase() : a(locale,'readOnly').toUpperCase()}</strong></span>
    <span>{a(locale,'channel')} <strong>{String(releaseChannel).toUpperCase()}</strong></span>
    <span>{a(locale,'target')} <strong>{releaseTarget}</strong>{#if data.release?.experimental} · {a(locale,'experimental')}{/if}</span>
    <span>{a(locale,'registry')} <strong>{data.registry?.revision ? data.registry.revision.slice(0,12) : '—'}</strong></span>
  </div>

  {#if actionNotice}
    <div class="catalog-install-notice {actionNotice.cls}">
      <span class="status-dot {actionNotice.cls}"></span><span>{actionNotice.text}</span>
      <button class="icon-button" aria-label={t(locale,'common.close')} onclick={() => actionNotice = null}>×</button>
    </div>
  {/if}

  {#if activeJob}
    <section class="app-action-console">
      <div class="app-action-console-head">
        <div>
          <strong>{locale === 'ru' ? '\u0414\u0435\u0439\u0441\u0442\u0432\u0438\u0435 \u0426\u0435\u043d\u0442\u0440\u0430 \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0439' : 'App Center action'}</strong>
          <span class="mono">{activeJob.kind} / {activeJob.target} / {activeJob.action}</span>
        </div>
        <div class="catalog-actions">
          <span class="state-chip {jobStateClass(activeJob.state)}">{jobStateLabel(activeJob.state)}</span>
          {#if !['succeeded','failed','cancelled'].includes(activeJob.state)}
            <button class="button danger-subtle" onclick={cancelCurrentAction}>{locale === 'ru' ? '\u041e\u0442\u043c\u0435\u043d\u0438\u0442\u044c' : 'Cancel'}</button>
          {/if}
        </div>
      </div>
      {#if activeJob.error}<div class="catalog-install-notice error">{activeJob.error}</div>{/if}
      <pre class="app-action-log mono">{actionLog.length ? actionLog.join('\n') : (locale === 'ru' ? '\u041e\u0436\u0438\u0434\u0430\u043d\u0438\u0435 \u0432\u044b\u0432\u043e\u0434\u0430...' : 'Waiting for output...')}</pre>
    </section>
  {/if}

  {#if actionHistory.length}
    <section class="app-action-history">
      <button
        class="app-action-history-toggle"
        type="button"
        aria-expanded={actionHistoryExpanded}
        aria-controls="app-action-history-list"
        onclick={() => actionHistoryExpanded = !actionHistoryExpanded}
      >
        <span class="app-action-history-title">
          <strong>{locale === 'ru' ? '\u0418\u0441\u0442\u043e\u0440\u0438\u044f \u0434\u0435\u0439\u0441\u0442\u0432\u0438\u0439' : 'Action history'}</strong>
          <small>{locale === 'ru' ? '\u041f\u043e\u0441\u043b\u0435\u0434\u043d\u0438\u0435 \u043e\u043f\u0435\u0440\u0430\u0446\u0438\u0438 \u043f\u0430\u043a\u0435\u0442\u043d\u043e\u0433\u043e \u043c\u0435\u043d\u0435\u0434\u0436\u0435\u0440\u0430.' : 'Recent package-manager operations.'}</small>
        </span>
        <span class="app-action-history-meta">
          <span class="state-chip neutral">{actionHistory.length}</span>
          <span class:expanded={actionHistoryExpanded} class="app-action-history-chevron" aria-hidden="true">v</span>
        </span>
      </button>
      {#if actionHistoryExpanded}
        <div class="app-action-history-list" id="app-action-history-list">
          {#each actionHistory.slice(0,5) as job (job.id)}
            <div class="app-action-history-row">
              <span class="app-action-history-copy">
                <strong>{job.target}</strong>
                <small class="mono">{job.kind} / {job.action}</small>
              </span>
              <span class="state-chip {jobStateClass(job.state)}">{jobStateLabel(job.state)}</span>
            </div>
          {/each}
        </div>
      {/if}
    </section>
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
          {@const warning = catalogWarning(item)}
          <article class="catalog-card">
            <div>
              <div class="catalog-card-head">
                <div class="catalog-identity">
                  <div class="catalog-icon mono">{acronym(item)}</div>
                  <div><h3>{item.name}</h3><span class="mono">{item.publisher?.name || item.source || 'community'} / {item.category || item.kind}</span></div>
                </div>
                <div class="catalog-state-stack"><span class="state-chip {trustClass(item)}">{trustLabel(item)}</span><span class="state-chip {st.cls}">{st.label}</span></div>
              </div>
              <p>{descriptionText(item)}</p>
              <div class="tech-box mono">
                <div><span>{a(locale,'installed')}</span><strong>{item.version ? `v${item.version}` : a(locale,'notInstalled')}</strong></div>
                <div><span>{a(locale,'available')}</span><strong class:good={item.update_available}>{item.release?.version ? `v${item.release.version}` : item.available_version ? `v${item.available_version}` : a(locale,'notAvailable')}</strong></div>
                <div><span>{a(locale,'package')}</span><strong title={packageText(item)}>{packageText(item)}</strong></div>
                {#if item.package_meta?.architecture}
                  <div><span>{a(locale,'architecture')}</span><strong>{item.package_meta.architecture}</strong></div>
                {/if}
                {#if Number(item.package_meta?.download_size_bytes || 0) > 0}
                  <div><span>{a(locale,'download')}</span><strong>{bytes(item.package_meta.download_size_bytes)}</strong></div>
                {/if}
                {#if Number(item.package_meta?.installed_size_bytes || 0) > 0}
                  <div><span>{a(locale,'installedSize')}</span><strong>{bytes(item.package_meta.installed_size_bytes)}</strong></div>
                {/if}
                {#if item.package_meta?.depends?.length}
                  <div><span>{a(locale,'depends')}</span><strong title={item.package_meta.depends.join(', ')}>{item.package_meta.depends.join(', ')}</strong></div>
                {/if}
                {#if item.package_meta?.conflicts?.length}
                  <div><span>{a(locale,'conflicts')}</span><strong title={item.package_meta.conflicts.join(', ')}>{item.package_meta.conflicts.join(', ')}</strong></div>
                {/if}
                {#if item.release?.min_core_version}<div><span>{a(locale,'minCore')}</span><strong>v{item.release.min_core_version}</strong></div>{/if}
                <div><span>{a(locale,'publisher')}</span><strong>{item.publisher?.name || item.source || a(locale,'unknown')}</strong></div>
                {#if hasServiceContract(item)}
                  <div><span>{a(locale,'service')}</span><strong class:good={item.id === 'routerforge-core' || item.service_running}>{serviceStatusText(item)}</strong></div>
                {/if}
                <div><span>{a(locale,'compatibility')}</span><strong title={compatibilityText(item)}>{compatibilityText(item)}</strong></div>
              </div>
              {#if warning}
                <div class="catalog-action-reason">{warning}</div>
              {/if}
            </div>
            <div class="catalog-card-foot">
              <div class="catalog-actions">
                {#if ownURL}<a class="button primary" href={ownURL}>{a(locale,'open')}</a>{/if}
                {#if item.installed && catalogWebURL(item)}
                  {#if webSecurityDecision(item).externalAllowed}
                    <a class="button" target="_blank" rel="noopener noreferrer" href={catalogWebURL(item)}>{a(locale,'open')} :{catalogWebPort(item)}</a>
                  {:else}
                    <button class="button" type="button" onclick={() => showWebSecurityNotice(item)}>{a(locale,'open')} :{catalogWebPort(item)}</button>
                  {/if}
                  {#if webSecurityDecision(item).embedAllowed}
                    <button class="button primary" type="button" disabled={webProbeBusyId === item.id} onclick={() => openEmbeddedWeb(item)}>
                      {webProbeBusyId === item.id ? (locale === 'ru' ? '\u041F\u0440\u043E\u0432\u0435\u0440\u043A\u0430\u2026' : 'Checking\u2026') : (locale === 'ru' ? '\u0412\u043D\u0443\u0442\u0440\u0438' : 'Embedded')}
                    </button>
                  {/if}
                {/if}
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
                <div><strong>{pkg.name}</strong>{#if pkg.upgradable}<span class="state-chip warn">{a(locale,'updateBadge')}</span>{/if}{#if pkg.installed}<span class="state-chip good">{a(locale,'installedBadge')}</span>{/if}</div>
                <p>{pkg.description || a(locale,'unknown')}</p>
                <span class="mono muted">{pkg.installed_version ? a(locale,'installedVersion',{version:pkg.installed_version}) : ''}{pkg.installed_version && pkg.available_version ? ' · ' : ''}{pkg.available_version ? a(locale,'availableVersion',{version:pkg.available_version}) : ''}</span>
              </div>
              <div class="catalog-actions">
                <button class="button" disabled={Boolean(busyId)} onclick={() => showEntwareDetail(pkg)}>{a(locale,'details')}</button>
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

{#if entwareDetail}
  <div class="app-detail-backdrop" role="presentation" onclick={() => entwareDetail = null}>
    <section class="app-detail-modal" role="dialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <div class="catalog-section-head">
        <div><h2>{entwareDetail.name}</h2><p>{entwareDetail.description || ''}</p></div>
        <button class="icon-button" aria-label={t(locale,'common.close')} onclick={() => entwareDetail = null}>x</button>
      </div>
      <div class="tech-box mono">
        <div><span>{a(locale,'version')}</span><strong>{entwareDetail.version || entwareDetail.available_version || a(locale,'unknown')}</strong></div>
        <div><span>{a(locale,'installed')}</span><strong>{entwareDetail.installed_version || a(locale,'notInstalled')}</strong></div>
        {#if entwareDetail.architecture}<div><span>{a(locale,'architecture')}</span><strong>{entwareDetail.architecture}</strong></div>{/if}
        {#if entwareDetail.section}<div><span>{a(locale,'section')}</span><strong>{entwareDetail.section}</strong></div>{/if}
        {#if Number(entwareDetail.download_size_bytes || 0) > 0}<div><span>{a(locale,'downloadSize')}</span><strong>{bytes(entwareDetail.download_size_bytes)}</strong></div>{/if}
        {#if Number(entwareDetail.installed_size_bytes || 0) > 0}<div><span>{a(locale,'installedSize')}</span><strong>{bytes(entwareDetail.installed_size_bytes)}</strong></div>{/if}
        {#if entwareDetail.depends?.length}<div><span>{a(locale,'depends')}</span><strong>{entwareDetail.depends.join(', ')}</strong></div>{/if}
        {#if entwareDetail.maintainer}<div><span>{a(locale,'maintainer')}</span><strong>{entwareDetail.maintainer}</strong></div>{/if}
      </div>
    </section>
  </div>
{/if}

{#if webWorkspace}<ExternalWebWorkspace workspace={webWorkspace} {locale} onclose={() => webWorkspace = null}/>{/if}
{#if plannerItem}<InstallPlanner item={plannerItem} onclose={() => plannerItem = null}/>{/if}
{#if removeItem}<RemoveConfirm item={removeItem} busy={busyId === removeItem.id && busyAction === 'remove'} oncancel={() => { if (!busyId) removeItem = null; }} onconfirm={(typed) => runCatalogAction(removeItem,'remove',typed)}/>{/if}

<style>
  .app-center-tabs { margin-bottom: 1rem; }
  .app-center-page .catalog-toolbar-v3 {
    display:grid;
    grid-template-columns:minmax(18rem,1fr) auto;
    gap:.65rem;
    align-items:center;
    padding:.55rem;
  }
  .app-center-page .catalog-search-group,
  .app-center-page .catalog-control-group {
    min-width:0;
    display:flex;
    align-items:center;
    gap:.48rem;
  }
  .app-center-page .catalog-search-group .search-control {
    min-width:12rem;
    flex:1 1 24rem;
  }
  .app-center-page .catalog-search-group .search-control input {
    width:100%;
    min-width:0;
  }
  .app-center-page .catalog-control-group {
    justify-content:flex-end;
    flex-wrap:wrap;
  }
  .app-center-page .channel-select {
    min-width:10.5rem;
  }
  .app-center-page .registry-chip {
    flex:0 0 auto;
    white-space:nowrap;
  }
  .app-center-page .check-updates-button {
    white-space:nowrap;
  }
  .entware-summary { display:flex; gap:1.2rem; flex-wrap:wrap; margin:1rem 0; color:var(--rf-muted,var(--muted)); }
  .entware-package-list { display:grid; gap:.55rem; margin-top:1rem; }
  .entware-package-row { display:flex; justify-content:space-between; gap:1rem; align-items:center; border:1px solid var(--rf-border,var(--border)); border-radius:.7rem; padding:.8rem 1rem; background:var(--rf-panel,var(--panel)); }
  .entware-package-main { min-width:0; }
  .entware-package-main > div { display:flex; align-items:center; gap:.45rem; flex-wrap:wrap; }
  .entware-package-main p { margin:.25rem 0; }
  .entware-pagination { display:flex; justify-content:center; align-items:center; gap:.75rem; margin:1rem 0; }
  .app-action-console, .app-action-history { margin:1rem 0; border:1px solid var(--rf-border,var(--border)); border-radius:.75rem; background:var(--rf-panel,var(--panel)); overflow:hidden; }
  .app-action-console-head { display:flex; justify-content:space-between; gap:1rem; align-items:center; padding:.8rem 1rem; border-bottom:1px solid var(--rf-border,var(--border)); }
  .app-action-console-head > div:first-child { display:grid; gap:.2rem; }
  .app-action-log { min-height:90px; max-height:310px; overflow:auto; margin:0; padding:1rem; white-space:pre-wrap; word-break:break-word; font-size:.78rem; }
  .app-action-history-list { display:grid; }
  .app-action-history-row { display:flex; justify-content:space-between; gap:1rem; align-items:center; padding:.65rem 1rem; border-top:1px solid var(--rf-border,var(--border)); }
  .app-action-history-row > span:first-child { display:grid; gap:.15rem; }
  .app-action-history-row small { color:var(--rf-muted,var(--muted)); }
  .app-detail-backdrop { position:fixed; inset:0; z-index:1000; display:grid; place-items:center; padding:1rem; background:rgba(0,0,0,.55); }
  .app-detail-modal { width:min(720px,100%); max-height:85vh; overflow:auto; padding:1rem; border:1px solid var(--rf-border,var(--border)); border-radius:.85rem; background:var(--rf-panel,var(--panel)); box-shadow:0 24px 80px rgba(0,0,0,.35); }
  @media (max-width: 760px) {
    .entware-package-row { align-items:flex-start; flex-direction:column; }
  }

  /* 0.6.0 release-candidate UI fixes: action history and catalog card wrapping. */
  .app-center-page .app-action-history-toggle {
    width:100%;
    min-height:64px;
    display:flex;
    justify-content:space-between;
    align-items:center;
    gap:1rem;
    padding:.85rem 1rem;
    border:0;
    background:transparent;
    color:inherit;
    text-align:left;
    cursor:pointer;
  }
  .app-center-page .app-action-history-toggle:hover {
    background:var(--rf-hover,var(--hover));
  }
  .app-center-page .app-action-history-title {
    min-width:0;
    display:grid;
    gap:.28rem;
  }
  .app-center-page .app-action-history-title > strong {
    color:var(--rf-text,var(--text));
    font-size:1rem;
    line-height:1.2;
  }
  .app-center-page .app-action-history-title > small {
    color:var(--rf-muted,var(--muted));
    font-size:.78rem;
    line-height:1.4;
  }
  .app-center-page .app-action-history-meta {
    flex:0 0 auto;
    display:flex;
    align-items:center;
    gap:.55rem;
  }
  .app-center-page .app-action-history-chevron {
    display:inline-grid;
    place-items:center;
    width:1.5rem;
    height:1.5rem;
    color:var(--rf-muted,var(--muted));
    font-size:.85rem;
    line-height:1;
    transition:transform .15s ease;
  }
  .app-center-page .app-action-history-chevron.expanded {
    transform:rotate(180deg);
  }
  .app-center-page .app-action-history-list {
    display:grid;
    border-top:1px solid var(--rf-border,var(--border));
  }
  .app-center-page .app-action-history-row {
    min-width:0;
    display:flex;
    justify-content:space-between;
    align-items:center;
    gap:1rem;
    padding:.78rem 1rem;
    border-top:0;
  }
  .app-center-page .app-action-history-row + .app-action-history-row {
    border-top:1px solid var(--rf-border,var(--border));
  }
  .app-center-page .app-action-history-copy {
    min-width:0;
    display:grid;
    gap:.22rem;
  }
  .app-center-page .app-action-history-copy strong,
  .app-center-page .app-action-history-copy small {
    min-width:0;
    overflow-wrap:anywhere;
  }

  .app-center-page .catalog-grid-v2 .catalog-card {
    min-height:100%;
  }
  .app-center-page .catalog-grid-v2 .catalog-card > div:first-child,
  .app-center-page .catalog-grid-v2 .catalog-card-head,
  .app-center-page .catalog-grid-v2 .catalog-identity,
  .app-center-page .catalog-grid-v2 .catalog-identity > div:last-child {
    min-width:0;
  }
  .app-center-page .catalog-grid-v2 .catalog-card > div > p,
  .app-center-page .catalog-grid-v2 .catalog-identity h3,
  .app-center-page .catalog-grid-v2 .catalog-identity span {
    overflow-wrap:anywhere;
  }
  .app-center-page .catalog-grid-v2 .catalog-state-stack {
    flex:0 0 auto;
    display:flex;
    flex-direction:column;
    align-items:flex-end;
    gap:.35rem;
  }
  .app-center-page .catalog-grid-v2 .tech-box > div {
    min-height:24px;
    grid-template-columns:minmax(104px,34%) minmax(0,1fr);
    gap:12px;
    align-items:start;
    padding:3px 0;
  }
  .app-center-page .catalog-grid-v2 .tech-box span,
  .app-center-page .catalog-grid-v2 .tech-box strong {
    min-width:0;
    line-height:1.35;
    white-space:normal;
    overflow-wrap:anywhere;
    word-break:break-word;
  }
  .app-center-page .catalog-grid-v2 .tech-box strong {
    overflow:visible;
    text-overflow:clip;
  }
  .app-center-page .catalog-grid-v2 .catalog-card-foot {
    margin-top:auto;
    align-items:flex-end;
  }
  .app-center-page .catalog-grid-v2 .catalog-card-foot > span {
    min-width:0;
    margin-left:auto;
    overflow-wrap:anywhere;
    text-align:right;
  }

  @media (max-width:980px) {
    .app-center-page .catalog-toolbar-v3 {
      grid-template-columns:1fr;
    }
    .app-center-page .catalog-control-group {
      justify-content:flex-start;
    }
  }

  @media (max-width:760px) {
    .app-center-page .catalog-search-group {
      align-items:stretch;
    }
    .app-center-page .catalog-control-group {
      display:grid;
      grid-template-columns:minmax(0,1fr) auto;
      align-items:stretch;
    }
    .app-center-page .channel-select {
      min-width:0;
      width:100%;
    }
    .app-center-page .registry-chip {
      align-self:center;
    }
    .app-center-page .check-updates-button {
      grid-column:1 / -1;
    }
    .app-center-page .app-action-history-toggle,
    .app-center-page .app-action-history-row {
      align-items:flex-start;
    }
    .app-center-page .catalog-grid-v2 .catalog-card-head {
      flex-direction:column;
    }
    .app-center-page .catalog-grid-v2 .catalog-state-stack {
      flex-direction:row;
      align-items:center;
      flex-wrap:wrap;
    }
    .app-center-page .catalog-grid-v2 .tech-box > div {
      grid-template-columns:minmax(104px,40%) minmax(0,1fr);
    }
    .app-center-page .catalog-grid-v2 .catalog-card-foot {
      align-items:flex-start;
      flex-direction:column;
    }
    .app-center-page .catalog-grid-v2 .catalog-card-foot > span {
      margin-left:0;
      text-align:left;
    }
  }

</style>
