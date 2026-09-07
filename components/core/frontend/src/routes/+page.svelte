<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline } from '$lib/stores/catalog.js';
  import { snapshot, backendOnline } from '$lib/stores/snapshot.js';
  import { overview, startOverviewPolling, averageCPU, cpuTemperature } from '$lib/stores/overview.js';
  import { settings } from '$lib/stores/settings.js';
  import { bytes, fmtDuration } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let stopOverview = null;
  onMount(() => {
    stopOverview = startOverviewPolling(() => $catalog, 10000);
    return () => stopOverview?.();
  });

  $: locale = $settings.locale || 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: installedModules = modules.filter((item) => item.installed && item.id !== 'profiling');
  $: installedIntegrations = integrations.filter((item) => item.installed);
  $: telem = $overview || {};
  $: memory = telem.memory || telem.summary?.memory;
  $: ramPct = Number(memory?.used_pct || (memory?.total_kb ? Number(memory.used_kb || 0) / Number(memory.total_kb) * 100 : 0));
  $: cpuPct = averageCPU(telem.cpu);
  $: cpuTemp = cpuTemperature(telem.thermal);
  $: opt = telem.platform?.opt || {};
  $: updates = [...modules, ...integrations].filter((item) => item.installed && item.update_available);
  $: issues = buildIssues();
  $: actionableIssues = issues.filter((issue) => issue.severity !== 'info');

  const text = (ru, en) => locale === 'ru' ? ru : en;

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

  function buildIssues() {
    const out = [];
    const push = (severity, title, detail = '', href = '') => out.push({ severity, title, detail, href });

    if (!$backendOnline) push('critical', text('\u042f\u0434\u0440\u043e RouterForge \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u043e','RouterForge Core is unavailable'));
    if (!$catalogOnline) push('warning', text('\u0426\u0435\u043d\u0442\u0440 \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0439 \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u0435\u043d','App Center is unavailable'));

    if ($catalog.registry && !$catalog.registry.online) push('warning',
      text('Registry \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442 \u0438\u0437 \u043b\u043e\u043a\u0430\u043b\u044c\u043d\u043e\u0433\u043e \u0438\u0441\u0442\u043e\u0447\u043d\u0438\u043a\u0430','Registry is using a local source'),
      String($catalog.registry.source || 'bundled').toUpperCase(), '/apps');

    const release = $catalog.release || {};
    if (release.supported === false) {
      push('critical', text('\u041a\u0430\u043d\u0430\u043b RouterForge \u043d\u0435 \u043f\u043e\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442 \u044d\u0442\u043e\u0442 target','RouterForge channel does not support this target'),
        release.error || [release.channel, release.target].filter(Boolean).join(' / '), '/apps');
    } else if (release.error) {
      push('warning', text('\u041e\u0448\u0438\u0431\u043a\u0430 release channel','Release channel error'), release.error, '/apps');
    }

    for (const item of [...modules, ...integrations]) {
      if (!item.installed || item.builtin) continue;
      const declaredServices = Array.isArray(item.detection?.services) ? item.detection.services : [];
      const hasServiceContract = Boolean(item.service) || declaredServices.length > 0;
      if (hasServiceContract && !item.service_running) {
        push('warning', `${item.name}: ${text('\u0441\u043b\u0443\u0436\u0431\u0430 \u043d\u0435 \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442','service is not running')}`, '', '/apps?tab=installed');
      }
      const trust = String(item.trust?.status || '').toLowerCase();
      if (['changed','blocked','deprecated'].includes(trust)) push(trust === 'blocked' ? 'critical' : 'warning',
        `${item.name}: ${text('manifest \u0442\u0440\u0435\u0431\u0443\u0435\u0442 \u0432\u043d\u0438\u043c\u0430\u043d\u0438\u044f','manifest requires attention')}`, trust.toUpperCase(), '/apps?tab=installed');
    }

    if (cpuTemp > 75) push(cpuTemp >= 90 ? 'critical' : 'warning',
      text('\u041f\u043e\u0432\u044b\u0448\u0435\u043d\u043d\u0430\u044f \u0442\u0435\u043c\u043f\u0435\u0440\u0430\u0442\u0443\u0440\u0430 CPU','High CPU temperature'), `${cpuTemp.toFixed(0)}\u00b0C`, '/monitoring?tab=thermal');
    if (ramPct >= 90) push(ramPct >= 96 ? 'critical' : 'warning',
      text('\u0412\u044b\u0441\u043e\u043a\u043e\u0435 \u0438\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u043d\u0438\u0435 RAM','High RAM usage'), `${ramPct.toFixed(0)}%`, '/monitoring?tab=system');
    if (Number(opt.used_pct || 0) >= 85) push(Number(opt.used_pct) >= 95 ? 'critical' : 'warning',
      text('\u0417\u0430\u043a\u0430\u043d\u0447\u0438\u0432\u0430\u0435\u0442\u0441\u044f \u043c\u0435\u0441\u0442\u043e \u0432 /opt','/opt is running low on space'),
      `${Number(opt.used_pct).toFixed(0)}% \u00b7 ${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}`, '/monitoring?tab=storage');
    if (telem.cpuSustainedHigh?.active) push('warning',
      text('CPU \u0434\u0435\u0440\u0436\u0438\u0442\u0441\u044f \u0432\u044b\u0448\u0435 90%','CPU has stayed above 90%'),
      `${Number(telem.cpuSustainedHigh.usage_pct || cpuPct).toFixed(0)}% \u00b7 >=30s`, '/monitoring?tab=system');

    if ($snapshot.capture_error) push('warning', text('\u041e\u0448\u0438\u0431\u043a\u0430 \u043d\u0430\u0431\u043b\u044e\u0434\u0435\u043d\u0438\u044f DNS-\u0442\u0440\u0430\u0444\u0438\u043a\u0430','DNS traffic observation error'), $snapshot.capture_error, '/dns?view=diagnostics');
    if ($snapshot.discovery_error) push('warning', text('\u041e\u0448\u0438\u0431\u043a\u0430 \u043e\u0431\u043d\u0430\u0440\u0443\u0436\u0435\u043d\u0438\u044f DNS','DNS discovery error'), $snapshot.discovery_error, '/dns?view=diagnostics');
    if ($snapshot.client_capture_error) push('warning', text('\u041e\u0448\u0438\u0431\u043a\u0430 DNS capture \u043a\u043b\u0438\u0435\u043d\u0442\u043e\u0432','DNS client capture error'), $snapshot.client_capture_error, '/dns?view=diagnostics');

    for (const upstream of $snapshot.upstreams || []) {
      const active = Boolean(upstream.active) || recent(upstream.last_request, 5);
      if (!active) continue;

      const health = String(upstream.health_status || '').toUpperCase();
      const window5 = upstream.stats_5m || {};
      const quality = String(window5.quality_status || '').toUpperCase();
      const name = upstream.name || upstream.sni || upstream.target || text('DNS upstream','DNS upstream');
      const path = [
        upstream.protocol,
        upstream.profile,
        upstream.policy_description,
        upstream.health_cause,
        Number(window5.timeout_pct || 0) > 0 ? `timeout ${Number(window5.timeout_pct).toFixed(1)}%` : '',
        Number(window5.fallback_pct || 0) > 0 ? `fallback ${Number(window5.fallback_pct).toFixed(1)}%` : '',
        Number(window5.p95_latency_ms || 0) > 0 ? `p95 ${Number(window5.p95_latency_ms).toFixed(0)} ms` : ''
      ].filter(Boolean).join(' \u00b7 ');

      if (health === 'DOWN' || health === 'DEGRADED') {
        push(health === 'DOWN' ? 'critical' : 'warning', `${name}: ${health}`, path, '/dns?view=overview');
      } else if (quality === 'BAD' || quality === 'WARN') {
        push('warning', `${name}: DNS ${quality}`, path, '/dns?view=quality');
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
      const name = meta.name || meta.address || resolver.key || text('DNS resolver','DNS resolver');
      const context = [
        meta.source,
        ...(meta.profiles || []),
        meta.interface,
        failures ? `${failures}/${requests} failures (${failurePct.toFixed(0)}%)` : '',
        `${responsePct.toFixed(0)}% responses`
      ].filter(Boolean).join(' \u00b7 ');

      if (requests > 0 && responses === 0 && timeouts > 0) {
        push('critical', `${name}: ${text('DNS \u043d\u0435 \u043e\u0442\u0432\u0435\u0447\u0430\u0435\u0442','DNS is not responding')}`,
          context, '/dns?view=overview');
      } else if ((failures >= 3 && failurePct >= 5) || (requests >= 5 && responsePct < 80)) {
        push('warning', `${name}: ${text('\u043f\u0440\u043e\u0431\u043b\u0435\u043c\u044b DNS','DNS problems')}`, context, '/dns?view=overview');
      }
    }

    if (telem.networkRoutes) {
      const routes = telem.networkRoutes.routes || [];
      const hasDefault = routes.some((route) =>
        route.destination === '0.0.0.0' && (!route.mask || route.mask === '0.0.0.0')
      );
      if (!hasDefault) push('critical',
        text('\u041e\u0442\u0441\u0443\u0442\u0441\u0442\u0432\u0443\u0435\u0442 default route','Default route is missing'),
        text('Network provider \u043d\u0435 \u0432\u0438\u0434\u0438\u0442 IPv4 \u043c\u0430\u0440\u0448\u0440\u0443\u0442 0.0.0.0/0','Network provider does not see an IPv4 0.0.0.0/0 route'),
        '/monitoring?tab=network');
    }

    const recentFailedAction = (telem.appActions?.items || []).find((job) =>
      job.state === 'failed' && recent(job.completed_at || job.started_at, 15)
    );
    if (recentFailedAction) push('warning',
      text('\u041d\u0435\u0434\u0430\u0432\u043d\u0435\u0435 \u0434\u0435\u0439\u0441\u0442\u0432\u0438\u0435 \u0426\u0435\u043d\u0442\u0440\u0430 \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0439 \u0437\u0430\u0432\u0435\u0440\u0448\u0438\u043b\u043e\u0441\u044c \u043e\u0448\u0438\u0431\u043a\u043e\u0439','A recent App Center action failed'),
      `${recentFailedAction.target || ''} \u00b7 ${recentFailedAction.action || ''} \u00b7 ${recentFailedAction.error || 'failed'}`, '/apps');

    if (updates.length) push('info',
      text(`\u0414\u043e\u0441\u0442\u0443\u043f\u043d\u043e \u043e\u0431\u043d\u043e\u0432\u043b\u0435\u043d\u0438\u0439: ${updates.length}`, `Updates available: ${updates.length}`),
      updates.slice(0,4).map((item) => `${item.name} ${item.version || ''} \u2192 ${item.release?.version || item.available_version || ''}`.trim()).join(' \u00b7 '),
      '/apps?tab=updates');

    const rank = { critical:0, warning:1, info:2 };
    return out.sort((a,b) => rank[a.severity] - rank[b.severity]);
  }

  function hrefFor(item) {
    if (item.kind === 'integration' && item.web_port) return `http://${location.hostname}:${item.web_port}`;
    if (item.id === 'dns') return '/dns';
    if (item.id === 'admin') return '/manage';
    if (['system','thermal','storage','network'].includes(item.id)) return `/monitoring?tab=${item.id}`;
    return '/apps?tab=installed';
  }

  function stateText(item) {
    if (item.update_available) return `↑ ${item.release?.version || item.available_version || ''}`;
    if (item.id === 'routerforge-core') return 'ONLINE';
    if (item.id === 'dns') return text('ВКЛЮЧЕН','ENABLED');
    if (item.service_running) return text('В СЕТИ','ONLINE');
    return text('УСТАНОВЛЕН','INSTALLED');
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
      <span class="routerforge-eyebrow mono">ROUTERFORGE / {String($catalog.release?.channel || 'beta').toUpperCase()}</span>
      <h1>{telem.platform?.hostname || 'RouterForge'}</h1>
      <p>{telem.platform?.model || 'Keenetic'} &middot; Core v{$snapshot.version || '\u2014'} &middot; {text('\u0410\u043f\u0442\u0430\u0439\u043c','Uptime')} {telem.platform?.uptime_seconds ? fmtDuration(telem.platform.uptime_seconds, locale) : '\u2014'}</p>
    </div>
    <span class="state-chip {$backendOnline ? 'good' : 'error'}">CORE {$backendOnline ? 'ONLINE' : 'OFFLINE'}</span>
  </div>

  <div class="routerforge-quick-telemetry mono">
    <span>CPU <strong>{telem.cpu ? `${cpuPct.toFixed(0)}%` : '—'}</strong></span>
    <span>RAM <strong>{memory ? `${ramPct.toFixed(0)}%` : '—'}</strong></span>
    <span>/opt <strong>{opt.total_bytes ? `${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}` : '—'}</strong></span>
    <span>CPU Temp <strong class:warn={cpuTemp > 75} class:bad={cpuTemp >= 90}>{cpuTemp ? `${cpuTemp.toFixed(0)}°C` : '—'}</strong></span>
  </div>

  <section class="panel routerforge-attention routerforge-attention-v2">
    <div class="panel-head">
      <div><strong>{text('Состояние','Status')}</strong><span>{text('Всё, что требует внимания прямо сейчас','Everything that needs attention right now')}</span></div>
      <span class="state-chip {actionableIssues.some((x)=>x.severity==='critical') ? 'error' : actionableIssues.length ? 'warn' : 'good'}">
        {actionableIssues.length ? text(`${actionableIssues.length} ТРЕБУЕТ ВНИМАНИЯ`, `${actionableIssues.length} ATTENTION`) : text('ВСЁ ХОРОШО','ALL GOOD')}
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
      <div class="routerforge-all-good"><span class="status-dot good"></span><strong>{text('Требующих внимания событий нет','Nothing needs attention')}</strong><span>{text('Устройство и доступные providers работают штатно.','Device and available providers are operating normally.')}</span></div>
    {/if}
  </section>

  <section class="routerforge-installed-v2">
    <div class="catalog-section-head">
      <div><h2>{text('Установлено на роутере','Installed on router')}</h2><p>{text('RouterForge и обнаруженные интеграции. Entware-библиотеки остаются в Центре приложений.','RouterForge and detected integrations. Entware libraries stay in App Center.')}</p></div>
      <a class="button" href="/apps">{text('Центр приложений','App Center')}</a>
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
        <div class="panel-head"><div><strong>{text('Интеграции','Integrations')}</strong><span>{installedIntegrations.length}</span></div></div>
        <div class="routerforge-installed-list">
          {#if !installedIntegrations.length}<div class="catalog-empty">{text('Установленные интеграции не обнаружены.','No installed integrations detected.')}</div>{/if}
          {#each installedIntegrations as item (item.id)}
            <a href={hrefFor(item)} target={item.web_port ? '_blank' : undefined} rel={item.web_port ? 'noopener noreferrer' : undefined} class="routerforge-installed-row">
              <span><strong>{item.name}</strong><small>{item.version ? `v${item.version}` : '—'}</small></span>
              <span class="state-chip {stateClass(item)}">{stateText(item)}</span>
            </a>
          {/each}
        </div>
      </section>
    </div>
  </section>
</div>
