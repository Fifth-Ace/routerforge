<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { getPlainDNS } from '$lib/api.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, quality, qualityClass, groupBy } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let mode = 'plain';
  let selectedPort = 0;
  let plain = { resolvers: [], recent: [], pending: 0 };
  let plainKey = '';
  let plainError = '';
  let plainTimer = null;

  $: locale = $settings.locale || 'ru';
  $: allUpstreams = $snapshot.upstreams || [];
  $: systemUpstreams = allUpstreams.filter((u) => u.profile === 'System');
  $: upstreams = systemUpstreams.length ? systemUpstreams : allUpstreams;
  $: policyGroups = Object.entries(groupBy(allUpstreams.filter((u)=>u.profile!=='System'),'profile')).sort(([a],[b])=>a.localeCompare(b,locale,{numeric:true}));
  $: if (upstreams.length && !upstreams.some((u) => Number(u.port) === Number(selectedPort))) selectedPort = Number(upstreams[0].port);
  $: selected = upstreams.find((u) => Number(u.port) === Number(selectedPort)) || upstreams[0] || null;
  $: win = selected?.stats_5m || {};
  $: selectedStatus = statusFor(selected || {}, locale);
  $: selectedQuality = selected ? quality(selected) : 100;
  $: recentFlow = selected ? ($snapshot.flow || []).filter((f) => Number(f.port) === Number(selected.port)).slice(-18).reverse() : [];

  $: plainResolvers = [...(plain.resolvers || [])].sort((a,b)=>{
    const ad=String(a.source||'').toUpperCase()==='DHCP'?0:1;
    const bd=String(b.source||'').toUpperCase()==='DHCP'?0:1;
    return ad-bd||String(a.address||'').localeCompare(String(b.address||''),undefined,{numeric:true});
  });
  $: if (plainResolvers.length && !plainResolvers.some((r) => resolverKey(r) === plainKey)) plainKey = resolverKey(plainResolvers[0]);
  $: selectedPlain = plainResolvers.find((r) => resolverKey(r) === plainKey) || plainResolvers[0] || null;
  $: plainRecent = selectedPlain
    ? (plain.recent || []).filter((event) => event.resolver === selectedPlain.address && Number(event.port) === Number(selectedPlain.port)).slice(0, 30)
    : [];

  $: qualityWindows = [[t(locale,'dns.servers.window5m'),'stats_5m'],[t(locale,'dns.servers.window1h'),'stats_1h'],[t(locale,'dns.servers.window24h'),'stats_24h']];

  function resolverKey(r = {}) { return `${r.address || ''}|${r.port || 53}`; }

  function plainState(r = {}, lang=locale) {
    const requests = Number(r.requests || 0);
    const responses = Number(r.responses || 0);
    const timeouts = Number(r.timeouts || 0);
    if (!requests) return { cls: 'neutral', label: t(lang,'dns.servers.detected') };
    if (timeouts > 0 && responses === 0) return { cls: 'error', label: 'TIMEOUT' };
    if (timeouts > 0) return { cls: 'warn', label: t(lang,'dns.servers.hasTimeout') };
    return { cls: 'good', label: t(lang,'dns.servers.active') };
  }

  function plainSuccess(r = {}) {
    const requests = Number(r.requests || 0);
    return requests ? Number(r.responses || 0) / requests * 100 : 100;
  }

  function assessment(d = {}, lang=locale) {
    if (d.assessment === 'POLICY_ROUTE_FAIL') return t(lang,'dns.servers.assessmentPolicyRouteFail');
    if (d.assessment === 'INTERFACE_ROUTE_FAIL') return t(lang,'dns.servers.assessmentInterfaceRouteFail');
    if (d.assessment === 'UPSTREAM_OK_LOCAL_PROXY_FAIL') return t(lang,'dns.servers.assessmentProxyFail');
    if (d.assessment === 'UPSTREAM_OK_LOCAL_PATH_FAIL') return t(lang,'dns.servers.assessmentLocalPathFail');
    if (d.assessment === 'UPSTREAM_OK') return t(lang,'dns.servers.assessmentUpstreamOk');
    if (String(d.assessment || '').startsWith('UPSTREAM_')) return t(lang,'dns.servers.assessmentUpstreamStage',{stage:d.stage||'—'});
    return '';
  }

  async function refreshPlain() {
    try { plain = await getPlainDNS(160); plainError = ''; }
    catch (error) { plainError = error?.message || t(locale,'errors.plainDnsUnavailable'); }
  }

  onMount(() => {
    refreshPlain();
    plainTimer = setInterval(() => { if (!document.hidden) refreshPlain(); }, 2500);
    return () => clearInterval(plainTimer);
  });
</script>

<svelte:head><title>RouterForge — {t(locale,'dns.servers.pageTitle')}</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head">
    <div><h1>{t(locale,'dns.servers.pageTitle')}</h1><p>{t(locale,'dns.servers.subtitle')}</p></div>
    <span class="page-kicker mono">{plainResolvers.length} MAIN · {upstreams.length} PROTECTED</span>
  </div>

  <div class="subtabs server-mode-tabs">
    <button class:active={mode === 'plain'} onclick={() => mode = 'plain'}>{t(locale,'dns.servers.mainDns')} <span class="pill">{plainResolvers.length}</span></button>
    <button class:active={mode === 'encrypted'} onclick={() => mode = 'encrypted'}>{t(locale,'dns.servers.protectedDns')} <span class="pill">{upstreams.length}</span></button>
  </div>

  {#if mode === 'plain'}
    {#if plainError}
      <section class="panel"><div class="empty">{plainError}</div></section>
    {:else if !selectedPlain}
      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale,'dns.servers.mainDns')}</strong><span>show ip name-server + show dns-proxy</span></div><span class="state-chip neutral">0 DNS</span></div>
        <div class="empty">{t(locale,'dns.servers.notFoundPlain')}</div>
      </section>
    {:else}
      {@const pst = plainState(selectedPlain,locale)}
      <div class="server-layout">
        <aside class="resolver-list panel">
          <div class="panel-head"><div><strong>{t(locale,'dns.servers.mainDns')}</strong><span>{t(locale,'dns.servers.found',{count:plainResolvers.length})}</span></div></div>
          <div class="resolver-items">
            {#each plainResolvers as r (resolverKey(r))}
              {@const st = plainState(r,locale)}
              <button class="resolver-item" class:active={resolverKey(r) === plainKey} onclick={() => plainKey = resolverKey(r)}>
                <div class="resolver-main"><span class="status-dot {st.cls}"></span><strong>{r.name || r.address}</strong><span class="pill accent">DNS</span></div>
                <span class="mono">{r.address}:{r.port || 53}{r.source?` · ${r.source}`:''}</span>
              </button>
            {/each}
          </div>
        </aside>

        <div class="stack">
          <section class="hero-card">
            <div class="hero-head">
              <div><h2>{selectedPlain.name || selectedPlain.address} <span class="pill accent">DNS :53</span></h2><p class="mono">{selectedPlain.address}:{selectedPlain.port || 53}{selectedPlain.source?` · ${selectedPlain.source}`:''}{selectedPlain.interface?` · ${selectedPlain.interface}`:''}</p></div>
              <span class="state-chip {pst.cls}">{pst.label}</span>
            </div>
            <div class="hero-metrics">
              <div><strong>{fmtInt(selectedPlain.requests || 0,locale)}</strong><span>{t(locale,'dns.servers.requests')}</span></div>
              <div><strong>{selectedPlain.p95_latency_ms ? fmtMs(selectedPlain.p95_latency_ms) : '—'}</strong><span>P95 latency</span></div>
              <div><strong>{selectedPlain.avg_latency_ms ? fmtMs(selectedPlain.avg_latency_ms) : '—'}</strong><span>Avg latency</span></div>
              <div><strong class={plainSuccess(selectedPlain) < 95 ? 'warn-text' : 'good'}>{plainSuccess(selectedPlain).toFixed(1)}%</strong><span>{t(locale,'dns.servers.responses')}</span></div>
            </div>
          </section>

          {#if $settings.uiLevel==='advanced'}
            <section class="panel">
              <div class="panel-head"><div><strong>{t(locale,'dns.servers.advancedStats')}</strong><span>Passive request → response correlation</span></div><span class="state-chip info">{t(locale,'common.advanced').toUpperCase()}</span></div>
              <div class="info-row"><div><strong>{t(locale,'common.responses')}</strong><span>{t(locale,'dns.servers.matchedResponses')}</span></div><div class="info-value">{fmtInt(selectedPlain.responses || 0,locale)}</div></div>
              <div class="info-row"><div><strong>{t(locale,'common.timeouts')}</strong><span>{t(locale,'dns.servers.noResponse8s')}</span></div><div class="info-value {Number(selectedPlain.timeouts || 0) ? 'warn-text' : ''}">{fmtInt(selectedPlain.timeouts || 0,locale)}</div></div>
              <div class="info-row"><div><strong>{t(locale,'common.dnsErrors')}</strong><span>{t(locale,'dns.servers.rcodeHint')}</span></div><div class="info-value">{fmtInt(selectedPlain.errors || 0,locale)}</div></div>
              <div class="info-row"><div><strong>NXDOMAIN</strong><span>{t(locale,'dns.servers.nxdomainHint')}</span></div><div class="info-value">{fmtInt(selectedPlain.nxdomain || 0,locale)}</div></div>
              <div class="info-row"><div><strong>{t(locale,'common.source')}</strong><span>{t(locale,'dns.servers.sourceHint')}</span></div><div class="info-value">{selectedPlain.source || '—'}</div></div>
              <div class="info-row"><div><strong>{t(locale,'common.interface')}</strong><span>{t(locale,'dns.servers.interfaceHint')}</span></div><div class="info-value">{selectedPlain.interface || '—'}</div></div>
              <div class="info-row"><div><strong>{t(locale,'common.profiles')}</strong><span>{t(locale,'dns.servers.profilesHint')}</span></div><div class="info-value">{(selectedPlain.profiles || []).join(' · ') || '—'}</div></div>
              <div class="info-row"><div><strong>{t(locale,'dns.servers.domainFilters')}</strong><span>{t(locale,'dns.servers.boundDomains')}</span></div><div class="info-value">{(selectedPlain.domains || []).join(' · ') || t(locale,'dns.servers.allDomains')}</div></div>
              {#if plain.note}<div class="plain-dns-warning">{plain.note}</div>{/if}
            </section>
          {/if}

          <section class="panel table-panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.servers.recentQueries')}</strong><span>{t(locale,'common.eventCount',{count:plainRecent.length})}</span></div></div>
            <div class="table-scroll"><table><thead><tr><th>{t(locale,'common.time')}</th><th>{t(locale,'common.domain')}</th><th>{t(locale,'common.type')}</th><th>RCODE</th><th>Latency</th><th>{t(locale,'common.status')}</th></tr></thead><tbody>
              {#if plainRecent.length}
                {#each plainRecent as event, index (`${event.time}-${event.resolver}-${event.port}-${event.protocol}-${event.domain}-${event.qtype}-${event.status}-${index}`)}
                  <tr><td class="mono">{new Date(event.time).toLocaleTimeString(locale==='en'?'en-GB':'ru-RU')}</td><td>{event.domain}</td><td><span class="pill">{event.qtype}</span></td><td>{event.rcode || '—'}</td><td class="mono">{event.latency_ms ? fmtMs(event.latency_ms) : '—'}</td><td><span class="state-chip {event.status === 'TIMEOUT' ? 'error' : 'good'}">{event.status}</span></td></tr>
                {/each}
              {:else}<tr><td colspan="6" class="empty">{t(locale,'dns.servers.noObservedPlain')}</td></tr>{/if}
            </tbody></table></div>
          </section>
        </div>
      </div>
    {/if}

  {:else if !selected}
    <section class="panel"><div class="empty">{t(locale,'dns.servers.noProtected')}</div></section>
  {:else}
    <div class="server-layout">
      <aside class="resolver-list panel">
        <div class="panel-head"><div><strong>{t(locale,'dns.servers.protectedDns')}</strong><span>{t(locale,'dns.servers.systemResolvers',{count:upstreams.length})}</span></div></div>
        <div class="resolver-items">
          {#each upstreams as u (u.port)}
            {@const st = statusFor(u,locale)}
            <button class="resolver-item" class:active={Number(u.port) === Number(selectedPort)} onclick={() => selectedPort = Number(u.port)}>
              <div class="resolver-main"><span class="status-dot {st.cls}"></span><strong>{u.name}</strong><span class="pill accent">{u.protocol}</span></div>
              <span>{u.target}{u.domain?` · ${u.domain}`:''}</span>
            </button>
          {/each}
        </div>
      </aside>

      <div class="stack">
        <section class="hero-card">
          <div class="hero-head"><div><h2>{selected.name} <span class="pill accent">{selected.protocol}</span></h2><p>{selected.target}{selected.domain?` · ${selected.domain}`:''}</p></div><span class="state-chip {selectedStatus.cls}">{selectedStatus.label}</span></div>
          <div class="hero-metrics">
            <div><strong>{fmtInt(win.requests || 0,locale)}</strong><span>{t(locale,'common.requests')} 5m</span></div>
            <div><strong>{win.p95_latency_ms ? fmtMs(win.p95_latency_ms) : '—'}</strong><span>P95 latency 5m</span></div>
            <div><strong>{fmtPct(win.fallback_pct || 0)}</strong><span>Fallback 5m</span></div>
            <div><strong class={qualityClass(win)}>{selectedQuality.toFixed(2)}%</strong><span>{t(locale,'common.quality')} 5m</span></div>
          </div>
        </section>

        {#if $settings.uiLevel==='advanced'}
          <section class="panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.servers.qualityResolver')}</strong><span>{t(locale,'dns.servers.statWindows')}</span></div><span class="state-chip info">{t(locale,'common.advanced').toUpperCase()}</span></div>
            {#each qualityWindows as [label,key]}
              {@const w = selected[key] || {}}
              <div class="info-row"><div><strong>{label}</strong><span>{t(locale,'dns.servers.qualityWindowSummary',{requests:fmtInt(w.requests||0,locale),errors:fmtInt(w.errors||0,locale),timeouts:fmtInt(w.timeouts||0,locale)})}</span></div><div class="info-value"><span class={qualityClass(w)}>{Number(w.quality_pct ?? 100).toFixed(2)}%</span> · p95 {w.p95_latency_ms ? fmtMs(w.p95_latency_ms) : '—'} · fallback {fmtPct(w.fallback_pct || 0)}</div></div>
            {/each}
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.servers.diagnostics')}</strong><span>{t(locale,'dns.servers.autoOnDown')}</span></div>{#if selected.diagnostic?.ran}<span class="state-chip {selected.diagnostic.status === 'FAIL' ? 'error' : 'good'}">{selected.diagnostic.status || '—'}</span>{/if}</div>
            {#if !selected.diagnostic?.ran}
              <div class="empty">{t(locale,'dns.servers.notRun')}</div>
            {:else}
              {@const d = selected.diagnostic}
              <div class="info-row"><div><strong>{t(locale,'dns.servers.stage')}</strong><span>{t(locale,'dns.servers.stageHint')}</span></div><div class="info-value">{d.stage || '—'}</div></div>
              <div class="info-row"><div><strong>{t(locale,'dns.servers.targetIp')}</strong><span>{t(locale,'dns.servers.targetIpHint')}</span></div><div class="info-value mono">{d.target_ip || '—'}</div></div>
              <div class="info-row"><div><strong>Resolve / TCP / TLS</strong><span>{t(locale,'dns.servers.connectionStages')}</span></div><div class="info-value">{fmtMs(d.resolve_ms)} · {fmtMs(d.tcp_ms)} · {fmtMs(d.tls_ms)}</div></div>
              <div class="info-row"><div><strong>{selected.protocol === 'DoH' ? 'HTTP / DNS' : 'DNS'}</strong><span>{t(locale,'dns.servers.protocolCheck')}</span></div><div class="info-value">{fmtMs(d.protocol_ms)}{selected.protocol === 'DoH' && d.http_status ? ` · HTTP ${d.http_status}` : ''}</div></div>
              <div class="info-row"><div><strong>DNS RCODE</strong><span>{t(locale,'dns.servers.dnsRcodeHint')}</span></div><div class="info-value">{d.dns_rcode || '—'}</div></div>
              {#if assessment(d,locale)}<div class="info-row"><div><strong>{t(locale,'dns.servers.assessment')}</strong><span>{d.route_scope || ''}</span></div><div class="info-value warn-text">{assessment(d,locale)}</div></div>{/if}
              <div class="info-row"><div><strong>{t(locale,'dns.servers.error')}</strong><span>{t(locale,'dns.servers.errorHint')}</span></div><div class="info-value">{d.error || t(locale,'common.none')}</div></div>
            {/if}
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>{t(locale,'dns.servers.dnsInfo')}</strong><span>{t(locale,'dns.servers.discoveryMetadata')}</span></div></div>
            <div class="info-row"><div><strong>{t(locale,'common.target')}</strong><span>{t(locale,'dns.servers.targetHint')}</span></div><div class="info-value mono">{selected.target}</div></div>
            <div class="info-row"><div><strong>SNI</strong><span>TLS Server Name</span></div><div class="info-value mono">{selected.sni || '—'}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.servers.domainFilter')}</strong><span>{t(locale,'dns.servers.domainFilterHint')}</span></div><div class="info-value">{selected.domain || t(locale,'dns.servers.allDomains')}</div></div>
            <div class="info-row"><div><strong>{t(locale,'common.interface')}</strong><span>{t(locale,'dns.servers.outInterfaceHint')}</span></div><div class="info-value">{selected.interface || t(locale,'dns.servers.notSet')}</div></div>
            {#if selected.linux_interface}<div class="info-row"><div><strong>Linux interface</strong><span>{t(locale,'dns.servers.linuxInterfaceHint')}</span></div><div class="info-value mono">{selected.linux_interface}</div></div>{/if}
            <div class="info-row"><div><strong>{t(locale,'dns.servers.localPort')}</strong><span>{t(locale,'dns.servers.localPortHint')}</span></div><div class="info-value mono">{selected.port}</div></div>
            <div class="info-row"><div><strong>{t(locale,'dns.servers.systemDnsProxy')}</strong><span>{t(locale,'dns.servers.systemDnsProxyHint')}</span></div><div class="info-value mono">:{selected.profile_dns_port || 53}</div></div>
            <div class="info-row"><div><strong>Timeout / Proceed</strong><span>{t(locale,'dns.servers.fallbackThresholds')}</span></div><div class="info-value mono">{selected.timeout_ms ? `${selected.timeout_ms} ms` : '—'} / {selected.proceed_ms ? `${selected.proceed_ms} ms` : '—'}</div></div>
          </section>
        {/if}

        <section class="panel table-panel">
          <div class="panel-head"><div><strong>{t(locale,'dns.servers.recentQueries')}</strong><span>{fmtAgo(selected.last_request,locale)}</span></div></div>
          <div class="table-scroll"><table><thead><tr><th>{t(locale,'common.time')}</th><th>{t(locale,'common.domain')}</th><th>{t(locale,'common.type')}</th><th>{t(locale,'common.fallback')}</th></tr></thead><tbody>
            {#if recentFlow.length}
              {#each recentFlow as f (`${f.time}-${f.domain}-${f.qtype}`)}<tr><td class="mono">{new Date(f.time).toLocaleTimeString(locale==='en'?'en-GB':'ru-RU')}</td><td>{f.domain}</td><td><span class="pill">{f.qtype}</span></td><td>{#if f.fallback}<span class="pill warn">{t(locale,'common.yes')}</span>{:else}—{/if}</td></tr>{/each}
            {:else}<tr><td colspan="4" class="empty">{t(locale,'dns.servers.noLiveQueries')}</td></tr>{/if}
          </tbody></table></div>
        </section>
      </div>
    </div>
  {/if}

  {#if $settings.uiLevel==='advanced'&&policyGroups.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale,'dns.servers.policyContexts')}</strong><span>{t(locale,'dns.servers.policyContextsHint')}</span></div><span class="state-chip info">{t(locale,'common.advanced').toUpperCase()}</span></div>
      <div class="table-scroll"><table>
        <thead><tr><th>Policy</th><th>{t(locale,'common.route')}</th><th>DNS proxy</th><th>{t(locale,'dns.overview.resolvers')}</th><th>Mark / table</th></tr></thead>
        <tbody>
          {#each policyGroups as [name,items]}
            {@const first=items[0]||{}}
            <tr>
              <td><div class="cell-title">{first.policy_description||name}</div><div class="cell-sub mono">{name}</div></td>
              <td>{#if first.policy_has_default}<span class="state-chip good">{t(locale,'dns.servers.hasDefaultRoute')}</span>{:else}<span class="state-chip neutral">{t(locale,'dns.servers.noDefaultRoute')}</span>{/if}</td>
              <td class="mono">:{first.profile_dns_port||'—'}</td><td>{items.length}</td>
              <td class="mono">0x{Number(first.policy_mark||0).toString(16)} · {first.policy_table||'—'}</td>
            </tr>
          {/each}
        </tbody>
      </table></div>
    </section>
  {/if}
</div>
