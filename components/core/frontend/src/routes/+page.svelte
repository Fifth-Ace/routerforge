<script>

  import { catalog, catalogOnline, catalogReady } from '$lib/stores/catalog.js';
  import { snapshot, backendOnline, backendReady } from '$lib/stores/snapshot.js';
  import { overview, averageCPU, cpuTemperature } from '$lib/stores/overview.js';
  import { settings } from '$lib/stores/settings.js';
  import { authState } from '$lib/stores/auth.js';
  import { probeCatalogWeb } from '$lib/api.js';
  import { bytes, fmtDuration, catalogWebURL, catalogWebSecurityDecision, catalogWebResolvedURL, catalogWebProbeStatusAllowed } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
  import { catalogItemServiceNeedsAttention, railInstalledIntegrations } from '$lib/integrations.js';
  import ExternalWebWorkspace from '$lib/components/ExternalWebWorkspace.svelte';


  let webWorkspace = null;
  let webProbeBusyId = '';
  let webNotice = '';

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: installedModules = modules.filter((item) => item.installed && item.id !== 'profiling');
  $: installedIntegrations = railInstalledIntegrations(integrations);
  $: telem = $overview || {};
  $: memory = telem.memory || telem.summary?.memory;
  $: ramPct = Number(memory?.used_pct || (memory?.total_kb ? Number(memory.used_kb || 0) / Number(memory.total_kb) * 100 : 0));
  $: cpuPct = averageCPU(telem.cpu);
  $: cpuTemp = cpuTemperature(telem.thermal);
  $: cpuTempWarning = Number($settings.cpuTempWarning || 75);
  $: opt = telem.platform?.opt || {};
  $: updates = [...modules, ...installedIntegrations].filter((item) => item.installed && item.update_available);
  $: issueInputs = {
    locale,
    backendReady: $backendReady,
    backendOnline: $backendOnline,
    catalogReady: $catalogReady,
    catalogOnline: $catalogOnline,
    catalog: $catalog,
    snapshot: $snapshot,
    telem,
    modules,
    integrations,
    updates,
    cpuTemp,
    ramPct,
    cpuPct,
    opt
  };
  $: issues = buildIssues(issueInputs);
  $: actionableIssues = issues.filter((issue) => issue.severity !== 'info');


  function recent(iso, minutes = 5) {
    const ts = new Date(iso || '').getTime();
    return Number.isFinite(ts) && Date.now() - ts <= minutes * 60000;
  }

  function plainDNSRecentGroups(snapshot) {
    const meta = new Map(
      (snapshot?.resolvers || []).map((resolver) => [
        `${resolver.address}:${Number(resolver.port || 53)}`,
        resolver
      ])
    );
    const groups = new Map();

    for (const event of snapshot?.recent || []) {
      if (!recent(event.time, 5)) continue;
      const key = `${event.resolver}:${Number(event.port || 53)}`;
      let row = groups.get(key);
      if (!row) {
        row = {
          key,
          meta: meta.get(key) || null,
          requests: 0,
          responses: 0,
          errors: 0,
          timeouts: 0
        };
        groups.set(key, row);
      }

      row.requests += 1;
      const status = String(event.status || '').toUpperCase();
      if (status === 'TIMEOUT') {
        row.timeouts += 1;
        continue;
      }
      if (status !== 'ANSWER') continue;

      row.responses += 1;
      const rcode = String(event.rcode || '').toUpperCase();
      if (rcode && !['NOERROR','NXDOMAIN'].includes(rcode)) row.errors += 1;
    }

    return [...groups.values()];
  }

  function buildIssues(_inputs) {
    const out = [];
    const push = (severity, title, detail = '', href = '') => out.push({ severity, title, detail, href });

    if ($backendReady && !$backendOnline) push('critical', t(locale, 'home.issueCoreUnavailable'));
    if ($catalogReady && !$catalogOnline) push('warning', t(locale, 'home.issueAppCenterUnavailable'));

    if ($catalog.registry && !$catalog.registry.online) push('warning',
      t(locale, 'home.issueRegistryLocal'),
      String($catalog.registry.source || 'bundled').toUpperCase(), '/apps');

    const release = $catalog.release || {};
    if (release.supported === false) {
      push('critical', t(locale, 'home.issueUnsupportedTarget'),
        release.error || [release.channel, release.target].filter(Boolean).join(' / '), '/apps');
    } else if (release.error) {
      push('warning', t(locale, 'home.issueReleaseChannelError'), release.error, '/apps');
    }

    for (const item of [...modules, ...installedIntegrations]) {
      if (!item.installed || item.builtin) continue;
      if (catalogItemServiceNeedsAttention(item)) {
        push('warning', `${item.name}: ${t(locale, 'home.issueServiceStopped')}`, '', '/apps?tab=installed');
      }
      const trust = String(item.trust?.status || '').toLowerCase();
      if (['changed','blocked','deprecated'].includes(trust)) push(trust === 'blocked' ? 'critical' : 'warning',
        `${item.name}: ${t(locale, 'home.issueManifestAttention')}`, trust.toUpperCase(), '/apps?tab=installed');
    }

    if (cpuTemp >= cpuTempWarning) push(cpuTemp >= 90 ? 'critical' : 'warning',
      t(locale, 'home.issueCpuTemperature'), `${cpuTemp.toFixed(0)}\u00b0C`, '/monitoring?tab=thermal');
    if (ramPct >= 90) push(ramPct >= 96 ? 'critical' : 'warning',
      t(locale, 'home.issueRamUsage'), `${ramPct.toFixed(0)}%`, '/monitoring?tab=system');
    if (Number(opt.used_pct || 0) >= 85) push(Number(opt.used_pct) >= 95 ? 'critical' : 'warning',
      t(locale, 'home.issueOptSpace'),
      `${Number(opt.used_pct).toFixed(0)}% \u00b7 ${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}`, '/monitoring?tab=storage');
    if (telem.cpuSustainedHigh?.active) push('warning',
      t(locale, 'home.issueCpuSustained'),
      `${Number(telem.cpuSustainedHigh.usage_pct || cpuPct).toFixed(0)}% \u00b7 >=30s`, '/monitoring?tab=system');

    if ($snapshot.capture_error) push('warning', t(locale, 'home.issueDnsTrafficError'), $snapshot.capture_error, '/dns?view=diagnostics');
    if ($snapshot.discovery_error) push('warning', t(locale, 'home.issueDnsDiscoveryError'), $snapshot.discovery_error, '/dns?view=diagnostics');
    if ($snapshot.client_capture_error) push('warning', t(locale, 'home.issueDnsClientCaptureError'), $snapshot.client_capture_error, '/dns?view=diagnostics');

    for (const upstream of $snapshot.upstreams || []) {
      const active = Boolean(upstream.active) || recent(upstream.last_request, 5);
      if (!active) continue;

      const health = String(upstream.health_status || '').toUpperCase();
      const window5 = upstream.stats_5m || {};
      const quality = String(window5.quality_status || '').toUpperCase();
      const name = upstream.name || upstream.sni || upstream.target || t(locale, 'home.dnsUpstream');
      const path = [
        upstream.protocol,
        upstream.profile,
        upstream.policy_description,
        upstream.health_cause,
        Number(window5.timeout_pct || 0) > 0 ? t(locale, 'home.dnsTimeoutPct', { value: Number(window5.timeout_pct).toFixed(1) }) : '',
        Number(window5.fallback_pct || 0) > 0 ? t(locale, 'home.dnsFallbackPct', { value: Number(window5.fallback_pct).toFixed(1) }) : '',
        Number(window5.p95_latency_ms || 0) > 0 ? t(locale, 'home.dnsP95Ms', { value: Number(window5.p95_latency_ms).toFixed(0) }) : ''
      ].filter(Boolean).join(' \u00b7 ');

      if (health === 'DOWN' || health === 'DEGRADED') {
        push(health === 'DOWN' ? 'critical' : 'warning', `${name}: ${health === 'DOWN' ? t(locale, 'home.issueDnsDown') : t(locale, 'home.issueDnsDegraded')}`, path, '/dns?view=overview');
      } else if (quality === 'BAD' || quality === 'WARN') {
        push('warning', `${name}: ${quality === 'BAD' ? t(locale, 'home.issueDnsQualityBad') : t(locale, 'home.issueDnsQualityWarn')}`, path, '/dns?view=quality');
      }
    }

    for (const resolver of plainDNSRecentGroups(telem.plainDns)) {
      const requests = Number(resolver.requests || 0);
      const responses = Number(resolver.responses || 0);
      const errors = Number(resolver.errors || 0);
      const timeouts = Number(resolver.timeouts || 0);
      const failures = errors + timeouts;
      const failurePct = requests > 0 ? failures / requests * 100 : 0;
      const responsePct = requests > 0 ? responses / requests * 100 : 100;
      const meta = resolver.meta || {};
      const name = meta.name || meta.address || resolver.key || t(locale, 'home.dnsResolver');
      const context = [
        meta.source,
        ...(meta.profiles || []),
        meta.interface,
        failures ? t(locale, 'home.dnsFailures', { failures, requests, percent: failurePct.toFixed(0) }) : '',
        t(locale, 'home.dnsResponses', { percent: responsePct.toFixed(0) })
      ].filter(Boolean).join(' \u00b7 ');

      if (requests > 0 && responses === 0 && timeouts > 0) {
        push('critical', `${name}: ${t(locale, 'home.issueDnsNoResponse')}`,
          context, '/dns?view=overview');
      } else if ((failures >= 3 && failurePct >= 5) || (requests >= 5 && responsePct < 80)) {
        push('warning', `${name}: ${t(locale, 'home.issueDnsProblems')}`, context, '/dns?view=overview');
      }
    }

    if (telem.networkRoutes) {
      const routes = telem.networkRoutes.routes || [];
      const hasDefault = routes.some((route) =>
        route.destination === '0.0.0.0' && (!route.mask || route.mask === '0.0.0.0')
      );
      if (!hasDefault) push('critical',
        t(locale, 'home.issueDefaultRouteMissing'),
        t(locale, 'home.issueDefaultRouteDetail'),
        '/monitoring?tab=network');
    }

    const recentFailedAction = (telem.appActions?.items || []).find((job) =>
      job.state === 'failed' && recent(job.completed_at || job.started_at, 15)
    );
    if (recentFailedAction) push('warning',
      t(locale, 'home.issueAppActionFailed'),
      `${recentFailedAction.target || ''} \u00b7 ${recentFailedAction.action || ''} \u00b7 ${recentFailedAction.error || t(locale, 'home.actionFailed')}`, '/apps');

    if (updates.length) push('info',
      t(locale, 'home.updatesAvailable', {count: updates.length}),
      updates.slice(0,4).map((item) => `${item.name} ${item.version || ''} \u2192 ${item.release?.version || item.available_version || ''}`.trim()).join(' \u00b7 '),
      '/apps?tab=updates');

    const rank = { critical:0, warning:1, info:2 };
    return out.sort((a,b) => rank[a.severity] - rank[b.severity]);
  }

  function integrationWorkspaceHref(item) {
    if (item?.kind !== 'integration' || !item?.installed) return '';
    if (!catalogWebURL(item, 'http')) return '';
    return `/apps?tab=integrations&open=${encodeURIComponent(item.id)}`;
  }

  function hrefFor(item) {
    const workspace = integrationWorkspaceHref(item);
    if (workspace) return workspace;
    if (item?.kind === 'integration') return '/apps?tab=integrations';
    if (item.id === 'dns') return '/dns';
    if (item.id === 'admin') return '/manage';
    if (['system','thermal','storage','network'].includes(item.id)) return `/monitoring?tab=${item.id}`;
    return '/apps?tab=installed';
  }

  async function openHomeIntegration(event, item) {
    const fallback = integrationWorkspaceHref(item);
    if (!fallback) return;

    event.preventDefault();
    if (webProbeBusyId) return;

    const decision = catalogWebSecurityDecision(item, $authState);
    if (!decision.embedAllowed) {
      webNotice = decision.reason === 'session-cookie-cross-port'
        ? t(locale, 'integrationsUi.embedCookie')
        : decision.reason === 'mixed-content'
          ? t(locale, 'integrationsUi.embedMixedContent')
          : t(locale, 'integrationsUi.embedUnavailable');
      return;
    }

    webProbeBusyId = item.id;
    webNotice = '';

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
        webNotice = t(locale, 'integrationsUi.embedPolicyBlocked');
        return;
      }

      if (!safe) {
        webNotice = t(locale, 'integrationsUi.embedProbeUnsafe');
        return;
      }

      const resolvedURL = catalogWebResolvedURL(item, probe, $authState);
      if (!resolvedURL) {
        webNotice = t(locale, 'integrationsUi.embedNoListener');
        return;
      }

      webWorkspace = { item, url:resolvedURL, probe };
    } catch (error) {
      webNotice = error?.payload?.error || error?.message || t(locale, 'integrationsUi.probeFailed');
    } finally {
      webProbeBusyId = '';
    }
  }

  function stateText(item) {
    if (item.update_available) return `↑ ${item.release?.version || item.available_version || ''}`;
    if (item.id === 'routerforge-core') return t(locale, 'home.badgeCoreOnline');
    if (item.id === 'dns') return t(locale, 'home.installedEnabled');
    if (item.service_running) return t(locale, 'home.installedOnline');
    return t(locale, 'home.installedInstalled');
  }
  function stateClass(item) {
    if (item.update_available) return 'warn';
    if (item.id === 'routerforge-core' || item.id === 'dns' || item.service_running) return 'good';
    return 'neutral';
  }
</script>

<svelte:head><title>RouterForge — {t(locale,'home.pageTitle')}</title></svelte:head>

<div class="page routerforge-home routerforge-home-v2">
  <div class="page-head routerforge-home-head">
    <div>
      <h1>{telem.platform?.hostname || 'RouterForge'}</h1>
      <p>{telem.platform?.model || 'Keenetic'} &middot; Core v{$snapshot.version || '\u2014'} &middot; {t(locale, 'home.uptime')} {telem.platform?.uptime_seconds ? fmtDuration(telem.platform.uptime_seconds, locale) : '\u2014'}</p>
    </div>
    <span class="state-chip {!$backendReady ? 'neutral' : $backendOnline ? 'good' : 'error'}">CORE {!$backendReady ? t(locale, 'home.coreLoading') : $backendOnline ? t(locale, 'home.badgeCoreOnline') : t(locale, 'home.coreOffline')}</span>
  </div>

  <div class="routerforge-quick-telemetry mono">
    <span>CPU <strong>{telem.cpu ? `${cpuPct.toFixed(0)}%` : '—'}</strong></span>
    <span>RAM <strong>{memory ? `${ramPct.toFixed(0)}%` : '—'}</strong></span>
    <span>/opt <strong>{opt.total_bytes ? `${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}` : '—'}</strong></span>
    <span>{t(locale, 'home.cpuTempLabel')} <strong class:warn={cpuTemp > 75} class:bad={cpuTemp >= 90}>{cpuTemp ? `${cpuTemp.toFixed(0)}°C` : '—'}</strong></span>
  </div>

  <section class="panel routerforge-attention routerforge-attention-v2">
    <div class="panel-head">
      <div><strong>{t(locale, 'home.state')}</strong><span>{t(locale, 'home.stateSubtitle')}</span></div>
      <span class="state-chip {actionableIssues.some((x)=>x.severity==='critical') ? 'error' : actionableIssues.length ? 'warn' : 'good'}">
        {actionableIssues.length ? t(locale, 'home.attention', {count: actionableIssues.length}) : t(locale, 'home.allGood')}
      </span>
    </div>
    {#if issues.length}
      <div class="routerforge-attention-list">
        {#each issues as issue}
          <a class="routerforge-attention-row {issue.severity}" href={issue.href || '/'}>
            <span class="status-dot {issue.severity === 'critical' ? 'error' : issue.severity === 'warning' ? 'warn' : 'info'}"></span>
            <span class="routerforge-attention-copy"><strong>{issue.title}</strong>{#if issue.detail}<small>{issue.detail}</small>{/if}</span>
            <span aria-hidden="true">›</span>
          </a>
        {/each}
      </div>
    {:else}
      <div class="routerforge-all-good"><span class="status-dot good"></span><strong>{t(locale, 'home.nothingToDo')}</strong><span>{t(locale, 'home.allGoodDeviceHint')}</span></div>
    {/if}
  </section>

  <section class="routerforge-installed-v2">
    <div class="catalog-section-head">
      <div><h2>{t(locale, 'home.installedOnRouter')}</h2><p>{t(locale, 'home.installedOnRouterHint')}</p></div>
      <a class="button" href="/apps">{t(locale, 'home.appCenter')}</a>
    </div>

    <div class="routerforge-installed-groups">
      <section class="panel">
        <div class="panel-head"><div><strong>RouterForge</strong><span>{installedModules.length}</span></div></div>
        <div class="routerforge-installed-list">
          {#each installedModules as item (item.id)}
            <a href={hrefFor(item)} class="routerforge-installed-row">
              <span><strong>{item.name}</strong><small>{item.version ? `v${item.version}` : '—'}</small></span>
              <span class="state-chip {stateClass(item)}">{stateText(item)}</span>
            </a>
          {/each}
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'home.integrations')}</strong><span>{installedIntegrations.length}</span></div></div>
        {#if webNotice}<div class="catalog-action-reason">{webNotice}</div>{/if}
        <div class="routerforge-installed-list">
          {#if !installedIntegrations.length}<div class="catalog-empty">{t(locale, 'home.noIntegrations')}</div>{/if}
          {#each installedIntegrations as item (item.id)}
            <a
              href={hrefFor(item)}
              class="routerforge-installed-row"
              aria-busy={webProbeBusyId === item.id}
              onclick={(event) => openHomeIntegration(event, item)}
            >
              <span><strong>{item.name}</strong><small>{item.version ? `v${item.version}` : '—'}</small></span>
              <span class="state-chip {stateClass(item)}">{stateText(item)}</span>
            </a>
          {/each}
        </div>
      </section>
    </div>
  </section>
</div>

{#if webWorkspace}
  <ExternalWebWorkspace
    workspace={webWorkspace}
    {locale}
    onclose={() => webWorkspace = null}
  />
{/if}
