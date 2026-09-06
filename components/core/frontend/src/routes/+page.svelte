<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline } from '$lib/stores/catalog.js';
  import { snapshot, backendOnline } from '$lib/stores/snapshot.js';
  import { overview, startOverviewPolling, averageCPU, cpuTemperature } from '$lib/stores/overview.js';
  import { settings } from '$lib/stores/settings.js';
  import { bytes } from '$lib/utils.js';
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

  const text = (ru, en) => locale === 'ru' ? ru : en;

  function recent(iso, minutes = 5) {
    const ts = new Date(iso || '').getTime();
    return Number.isFinite(ts) && Date.now() - ts <= minutes * 60000;
  }

  function buildIssues() {
    const out = [];
    const push = (severity, title, detail = '', href = '') => out.push({ severity, title, detail, href });

    if (!$backendOnline) push('critical', text('RouterForge Core недоступен','RouterForge Core is unavailable'));
    if (!$catalogOnline) push('warning', text('Центр приложений недоступен','App Center is unavailable'));
    if ($catalog.registry && !$catalog.registry.online) push('warning',
      text('Registry работает из локального источника','Registry is using a local source'),
      String($catalog.registry.source || 'bundled').toUpperCase(), '/apps');

    for (const item of [...modules, ...integrations]) {
      if (!item.installed || item.builtin || item.id === 'dns') continue;
      if ((item.managed || item.service) && !item.service_running) {
        push('warning', `${item.name}: ${text('служба не работает','service is not running')}`, '', '/apps?tab=installed');
      }
      const trust = String(item.trust?.status || '').toLowerCase();
      if (['changed','blocked','deprecated'].includes(trust)) push(trust === 'blocked' ? 'critical' : 'warning',
        `${item.name}: ${text('manifest требует внимания','manifest requires attention')}`, trust.toUpperCase(), '/apps?tab=installed');
    }

    if (cpuTemp > 75) push(cpuTemp >= 85 ? 'critical' : 'warning',
      text('Повышенная температура CPU','High CPU temperature'), `${cpuTemp.toFixed(0)}°C`, '/monitoring?tab=thermal');
    if (ramPct >= 90) push(ramPct >= 97 ? 'critical' : 'warning',
      text('Высокое использование RAM','High RAM usage'), `${ramPct.toFixed(0)}%`, '/monitoring?tab=system');
    if (Number(opt.used_pct || 0) >= 90) push(Number(opt.used_pct) >= 97 ? 'critical' : 'warning',
      text('Заканчивается место в /opt','/opt is running low on space'),
      `${Number(opt.used_pct).toFixed(0)}% · ${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}`, '/monitoring?tab=storage');

    if ($snapshot.capture_error) push('warning', text('Ошибка наблюдения DNS-трафика','DNS traffic observation error'), $snapshot.capture_error, '/dns?view=diagnostics');
    if ($snapshot.discovery_error) push('warning', text('Ошибка обнаружения DNS','DNS discovery error'), $snapshot.discovery_error, '/dns?view=diagnostics');
    if ($snapshot.client_capture_error) push('warning', text('Ошибка DNS capture клиентов','DNS client capture error'), $snapshot.client_capture_error, '/dns?view=diagnostics');

    for (const upstream of $snapshot.upstreams || []) {
      const health = String(upstream.health_status || '').toUpperCase();
      if (!['DOWN','DEGRADED'].includes(health)) continue;
      const name = upstream.name || upstream.sni || upstream.target || text('DNS upstream','DNS upstream');
      push(health === 'DOWN' ? 'critical' : 'warning', `${name}: ${health}`,
        [upstream.profile, upstream.target, upstream.sni].filter(Boolean).join(' · '), '/dns?view=overview');
    }

    for (const resolver of telem.plainDns?.resolvers || []) {
      if (!recent(resolver.last_request, 5)) continue;
      const requests = Number(resolver.requests || 0);
      const responses = Number(resolver.responses || 0);
      const errors = Number(resolver.errors || 0);
      const timeouts = Number(resolver.timeouts || 0);
      const name = resolver.name || resolver.address || text('DNS resolver','DNS resolver');
      if (requests > 0 && responses === 0 && timeouts > 0) push('critical',
        `${name}: ${text('DNS не отвечает','DNS is not responding')}`, `${timeouts} timeout`, '/dns?view=overview');
      else if (errors > 0 || timeouts > 0) push('warning',
        `${name}: ${text('проблемы DNS','DNS problems')}`, `${errors} errors · ${timeouts} timeout`, '/dns?view=overview');
    }

    if (updates.length) push('info',
      text(`Доступно обновлений: ${updates.length}`, `Updates available: ${updates.length}`),
      updates.slice(0,4).map((item) => `${item.name} ${item.version || ''} → ${item.release?.version || item.available_version || ''}`.trim()).join(' · '),
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
      <p>{telem.platform?.model || 'Keenetic'} · Core v{$snapshot.version || '—'}</p>
    </div>
    <span class="state-chip {$backendOnline ? 'good' : 'error'}">CORE {$backendOnline ? 'ONLINE' : 'OFFLINE'}</span>
  </div>

  <div class="routerforge-quick-telemetry mono">
    <span>CPU <strong>{telem.cpu ? `${cpuPct.toFixed(0)}%` : '—'}</strong></span>
    <span>RAM <strong>{memory ? `${ramPct.toFixed(0)}%` : '—'}</strong></span>
    <span>/opt <strong>{opt.total_bytes ? `${bytes(opt.used_bytes)} / ${bytes(opt.total_bytes)}` : '—'}</strong></span>
    <span>CPU Temp <strong class:warn={cpuTemp > 75} class:bad={cpuTemp >= 85}>{cpuTemp ? `${cpuTemp.toFixed(0)}°C` : '—'}</strong></span>
  </div>

  <section class="panel routerforge-attention routerforge-attention-v2">
    <div class="panel-head">
      <div><strong>{text('Состояние','Status')}</strong><span>{text('Всё, что требует внимания прямо сейчас','Everything that needs attention right now')}</span></div>
      <span class="state-chip {issues.some((x)=>x.severity==='critical') ? 'error' : issues.length ? 'warn' : 'good'}">
        {issues.length ? text(`${issues.length} ТРЕБУЕТ ВНИМАНИЯ`, `${issues.length} ATTENTION`) : text('ВСЁ ХОРОШО','ALL GOOD')}
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
