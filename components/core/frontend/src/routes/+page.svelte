<script>
  import { onMount } from 'svelte';

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
  import HomeWorkspace from '$lib/components/HomeWorkspace.svelte';


  let webWorkspace = null;
  let webProbeBusyId = '';
  let webNotice = '';

  // Dismissal is local to this browser and router origin; no server mutation.
  const noticeStorageKeys = Object.freeze({
    portal: 'routerforge:home:notice:portal:dismissed',
    mips: 'routerforge:home:notice:stable-mips:dismissed'
  });
  let portalNoticeVisible = true;
  let stableMipsNoticeVisible = true;
  onMount(() => {
    try {
      portalNoticeVisible = localStorage.getItem(noticeStorageKeys.portal) !== '1';
      stableMipsNoticeVisible = localStorage.getItem(noticeStorageKeys.mips) !== '1';
    } catch { /* private mode: buttons still close notices for this page */ }
  });
  function dismissNotice(kind) {
    if (kind === 'portal') portalNoticeVisible = false;
    else if (kind === 'mips') stableMipsNoticeVisible = false;
    else return;
    try { localStorage.setItem(noticeStorageKeys[kind], '1'); } catch {}
  }


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

  // Workspace is a projection of the existing stores, not an independent collector.
  $: workspaceMetrics = makeWorkspaceMetrics({locale, telem, memory, opt, updates, modules, snapshot:$snapshot, ready:$backendReady, online:$backendOnline, catalogReady:$catalogReady, catalogOnline:$catalogOnline});
  function makeWorkspaceMetrics({locale,telem,memory,opt,updates,modules,snapshot,ready,online,catalogReady,catalogOnline}) {
    const en=locale==='en', tr=(ru,english)=>en?english:ru;
    const unknown=()=>({value:'—',detail:tr('Нет данных','No data'),state:'unknown'});
    const numeric=v=>typeof v==='number'&&Number.isFinite(v)?v:null;
    const pct=v=>numeric(v)!==null&&v>=0&&v<=100?v:null;
    const make=(value,detail='',percent=null,rows=[],state='normal')=>({value,detail,percent,rows,state});
    const percent=(value,detail='',rows=[])=>pct(value)===null?unknown():make(`${value.toFixed(0)}%`,detail,value,rows);
    const result={};
    const cpuRows=(telem.cpu?.cpus||[]).filter(c=>pct(c.usage_pct)!==null);
    result.cpu=cpuRows.length?percent(cpuRows.reduce((sum,c)=>sum+c.usage_pct,0)/cpuRows.length,`${cpuRows.length} ${tr('ядер','cores')}`,cpuRows.map((c,i)=>`CPU ${i} · ${c.usage_pct.toFixed(0)}%`)):unknown();
    const total=numeric(memory?.total_kb),used=numeric(memory?.used_kb);
    const ram=pct(memory?.used_pct)??(total>0&&used!==null?pct(used/total*100):null);
    result.ram=percent(ram,total>0&&used!==null?`${bytes(used*1024)} / ${bytes(total*1024)}`:'',total>0&&used!==null&&used>=0&&used<=total?[`${tr('Свободно','Available')} · ${bytes((total-used)*1024)}`]:[]);
    const sensors=(telem.thermal?.sensors||telem.thermal?.thermal?.sensors||[]).filter(x=>['soc','cpu'].includes(String(x.role||'').toLowerCase())&&numeric(x.temp_c)!==null);
    const temp=sensors.length?Math.max(...sensors.map(x=>x.temp_c)):null;
    result.thermal=temp===null?unknown():make(`${temp.toFixed(0)}°C`,tr('CPU / SoC','CPU / SoC'),null,sensors.map(x=>`${String(x.name||x.role)} · ${x.temp_c.toFixed(0)}°C`));
    result.thermal.numeric=temp;
    const usedBytes=numeric(opt.used_bytes),totalBytes=numeric(opt.total_bytes);
    const storage=pct(opt.used_pct)??(totalBytes>0&&usedBytes!==null?pct(usedBytes/totalBytes*100):null);
    result.storage=percent(storage,totalBytes>0&&usedBytes!==null?`/opt · ${bytes(usedBytes)} / ${bytes(totalBytes)}`:'/opt',totalBytes>0&&usedBytes!==null&&usedBytes>=0&&usedBytes<=totalBytes?[`${tr('Свободно','Available')} · ${bytes(totalBytes-usedBytes)}`]:[]);
    if(Array.isArray(telem.networkRoutes?.routes)){
      const routes=telem.networkRoutes.routes.filter(x=>x.destination==='0.0.0.0'&&(!x.mask||x.mask==='0.0.0.0'));
      result.route=make(routes.length?tr('Есть','Present'):tr('Нет','Missing'),tr('Маршрут по умолчанию, не проверка интернета','Default route, not an internet probe'),null,[],routes.length?'normal':'warning');
    }else result.route=unknown();
    const upstreams=Array.isArray(snapshot?.upstreams)?snapshot.upstreams:[];
    const observed=upstreams.filter(x=>['UP','OK','HEALTHY','DOWN','DEGRADED'].includes(String(x.health_status||'').toUpperCase()));
    const bad=observed.filter(x=>['DOWN','DEGRADED'].includes(String(x.health_status).toUpperCase())).length;
    result.dns=observed.length?make(`${observed.length-bad}/${observed.length}`,tr('Без DOWN / DEGRADED среди известных','Not DOWN / DEGRADED among known'),null,observed.map(x=>`${String(x.name||x.sni||x.target||'DNS')} · ${x.health_status}`),bad?'warning':'normal'):unknown();
    result.core=ready?make(online?tr('В сети','Online'):tr('Не в сети','Offline'),String(snapshot?.version||''),null,[],online?'normal':'critical'):unknown();
    result.updates=catalogReady&&catalogOnline?make(String(updates.length),tr('Установленные компоненты','Installed components'),null,updates.map(x=>`${String(x.name||x.id)}${x.version?` · ${x.version}`:''}${x.release?.version||x.available_version?` → ${x.release?.version||x.available_version}`:''}`),updates.length?'warning':'normal'):unknown();
    const uptime=numeric(telem.platform?.uptime_seconds);
    result.uptime=uptime!==null&&uptime>=0?make(fmtDuration(uptime,locale),tr('С момента запуска','Since boot')):unknown();
    const known=catalogReady&&catalogOnline?modules.filter(x=>x.installed&&!x.builtin&&typeof x.service_running==='boolean'):[];
    const running=known.filter(x=>x.service_running).length;
    result.services=known.length?make(`${running}/${known.length}`,tr('Службы с известным статусом','Services with known status'),null,known.map(x=>`${String(x.name||x.id)} · ${x.service_running?tr('Работает','Running'):tr('Остановлена','Stopped')}`),running<known.length?'warning':'normal'):unknown();
    const service=id=>{const item=known.find(x=>x.id===id);return item?make(item.service_running?tr('Работает','Running'):tr('Остановлена','Stopped'),tr('Статус службы, не проверка доступности','Service status, not reachability'),null,[],item.service_running?'normal':'warning'):unknown();};
    result['dns-router-status']=service('dns-router');result['nfqws-status']=service('nfqws-manager');
    const net=telem.networkInterfaces;
    const rows=Array.isArray(net?.interfaces)?net.interfaces:[];
    const states=rows.filter(x=>['up','down','lowerlayerdown','dormant'].includes(String(x.oper_state||'').toLowerCase()));
    result.interfaces=states.length?make(`${states.filter(x=>x.oper_state==='up').length}/${states.length}`,tr('Логические интерфейсы с известным состоянием','Logical interfaces with known state'),null,states.map(x=>`${String(x.display_name||x.name)} · ${x.oper_state}`)):unknown();
    result.traffic=unknown();
    const routes=Array.isArray(telem.networkRoutes?.routes)?telem.networkRoutes.routes.filter(x=>x.destination==='0.0.0.0'&&(!x.mask||x.mask==='0.0.0.0')&&typeof x.interface==='string'&&Number.isFinite(x.metric)):[];
    const best=routes.length?Math.min(...routes.map(x=>x.metric)):null;
    const names=[...new Set(routes.filter(x=>x.metric===best).map(x=>x.interface))];
    const sys=Array.isArray(net?.system_interfaces)?net.system_interfaces:[];
    const matches=names.length===1?sys.filter(x=>x.name===names[0]):[];
    const iface=matches.length===1?matches[0]:null;
    const rx=numeric(iface?.rate?.rx_bps),tx=numeric(iface?.rate?.tx_bps);
    // Monitoring rates are bytes per second, not bits. One exact interface only.
    if(rx!==null&&tx!==null&&rx>=0&&tx>=0&&typeof net?.sampled_at==='string'&&Number.isFinite(Date.parse(net.sampled_at))){
      const mbps=v=>`${(v*8/1000000).toFixed(2)} ${tr('Мбит/с','Mbps')}`;
      result.traffic={...make(`↓ ${mbps(rx)}`,`↑ ${mbps(tx)} · ${iface.name}`,null,[tr('Интерфейс маршрута по умолчанию','Default route interface')]),rx:rx*8/1000000,tx:tx*8/1000000,interfaceName:iface.name,sampleKey:`${iface.name}|${net.sampled_at}`};
    }
    return result;
  }


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

  {#snippet attention()}
  <section class="panel routerforge-attention routerforge-attention-v2">
    <div class="panel-head">
      <div><strong>{t(locale, 'home.state')}</strong><span>{t(locale, 'home.stateSubtitle')}</span></div>
      <span class="state-chip {actionableIssues.some((x)=>x.severity==='critical') ? 'error' : actionableIssues.length ? 'warn' : ($backendReady && $backendOnline && $catalogReady && $catalogOnline) ? 'good' : 'neutral'}">
        {actionableIssues.length ? t(locale, 'home.attention', {count: actionableIssues.length}) : ($backendReady && $backendOnline && $catalogReady && $catalogOnline) ? t(locale, 'home.allGood') : (locale==='en'?'Waiting for data':'Ожидание данных')}
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
    {:else if $backendReady && $backendOnline && $catalogReady && $catalogOnline}
      <div class="routerforge-all-good"><span class="status-dot good"></span><strong>{t(locale, 'home.nothingToDo')}</strong><span>{t(locale, 'home.allGoodDeviceHint')}</span></div>
    {:else}
      <div class="catalog-empty">{locale==='en'?'Waiting for data':'Ожидание данных'}</div>
    {/if}
  </section>
  {/snippet}

  {#if portalNoticeVisible || stableMipsNoticeVisible}
    <div class="rf-home-notices">
      {#if portalNoticeVisible}
        <section class="panel rf-home-notice" aria-labelledby="rf-notice-portal-title">
          <div class="rf-home-notice-body">
            <span class="rf-home-notice-kicker">{t(locale, 'home.portalEyebrow')}</span>
            <h2 id="rf-notice-portal-title">{t(locale, 'home.portalTitle')}</h2>
            <p>{t(locale, 'home.portalDescription')}</p>
            <a href="https://fifth-ace.github.io/routerforge-portal/" target="_blank" rel="noopener noreferrer">{t(locale, 'home.portalSite')} ↗</a>
          </div>
          <button type="button" class="rf-home-notice-close" aria-label={t(locale, 'home.noticeDismiss')} title={t(locale, 'home.noticeDismiss')} onclick={() => dismissNotice('portal')}>×</button>
        </section>
      {/if}
      {#if stableMipsNoticeVisible}
        <section class="panel rf-home-notice rf-home-notice-stable" aria-labelledby="rf-notice-mips-title">
          <div class="rf-home-notice-body">
            <span class="rf-home-notice-kicker">{t(locale, 'home.stableMipsEyebrow')}</span>
            <h2 id="rf-notice-mips-title">{t(locale, 'home.stableMipsTitle')}</h2>
            <p>{t(locale, 'home.stableMipsDescription')}</p>
            <a href="https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable" target="_blank" rel="noopener noreferrer">{t(locale, 'home.stableMipsLink')} ↗</a>
          </div>
          <button type="button" class="rf-home-notice-close" aria-label={t(locale, 'home.noticeDismiss')} title={t(locale, 'home.noticeDismiss')} onclick={() => dismissNotice('mips')}>×</button>
        </section>
      {/if}
    </div>
  {/if}

  {#snippet installedPanel()}
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
  {/snippet}

  <HomeWorkspace {locale} metrics={workspaceMetrics} sampleKeys={{cpu:telem.cpu,ram:memory,thermal:telem.thermal,'traffic-rx':workspaceMetrics.traffic?.sampleKey,'traffic-tx':workspaceMetrics.traffic?.sampleKey}} ready={$backendReady} online={$backendOnline} issueCount={actionableIssues.length} criticalCount={actionableIssues.filter(x=>x.severity==='critical').length} alerts={issues} installed={($catalogReady&&$catalogOnline)?[...installedModules.map(item=>({name:item.name||item.id,state:$backendOnline?stateClass(item):'neutral',href:hrefFor(item)})),...installedIntegrations.map(item=>({name:item.name||item.id,state:stateClass(item),href:'/apps?tab=integrations'}))]:[]} catalogKnown={$catalogReady&&$catalogOnline} {attention} modules={installedPanel} />
</div>

{#if webWorkspace}
  <ExternalWebWorkspace
    workspace={webWorkspace}
    {locale}
    onclose={() => webWorkspace = null}
  />
{/if}

<style>
  .rf-home-notices{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,360px),1fr));gap:10px}
  .rf-home-notice{position:relative;min-width:0;min-height:100px;padding:12px 48px 12px 15px;border:1px solid var(--rf-accent-border,#376b8c);background:var(--rf-panel,#111820)}
  .rf-home-notice-stable{border-color:var(--rf-border,#303640)}
  .rf-home-notice-body{display:grid;gap:4px;min-width:0}
  .rf-home-notice-kicker{font-size:9px;font-weight:700;letter-spacing:.09em;color:var(--rf-accent,#38bdf8)}
  .rf-home-notice h2{font-size:15px;line-height:1.2;margin:0;color:var(--rf-text,#d6e2f0)}
  .rf-home-notice p{font-size:11px;line-height:1.4;margin:0;color:var(--rf-muted,#92a4b7)}
  .rf-home-notice a{font-size:11px;line-height:1.3;color:var(--rf-accent,#38bdf8);font-weight:650;width:max-content;max-width:100%}
  .rf-home-notice a:hover{text-decoration:underline}
  .rf-home-notice-close{position:absolute;right:8px;top:8px;width:32px;height:32px;display:grid;place-items:center;cursor:pointer;border-radius:7px;border:1px solid var(--rf-border,#303640);background:transparent;color:var(--rf-muted,#92a4b7);font-size:22px;line-height:1}
  .rf-home-notice-close:hover,.rf-home-notice-close:focus-visible{border-color:var(--rf-accent-border,#376b8c);color:var(--rf-text,#d6e2f0);background:var(--rf-accent-soft,rgba(56,189,248,.1))}
  @media(max-width:480px){.rf-home-notice{min-height:0;padding:11px 46px 12px 12px}}
</style>
