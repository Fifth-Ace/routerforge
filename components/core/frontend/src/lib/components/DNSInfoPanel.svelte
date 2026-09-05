<script>
  import { onMount } from 'svelte';
  import { getDNSInfo } from '$lib/api.js';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { fmtInt } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  let info = { generated_at: '', proxies: [], static_records: [], rebind: { enabled: false, nets: [], excludes: [] } };
  let loading = false;
  let error = '';
  let actionMessage = '';
  let actionKind = 'info';

  $: locale = $settings.locale || 'ru';
  $: proxies = info.proxies || [];
  $: logical = logicalUpstreams(proxies);
  $: staticRecords = info.static_records || [];
  $: rebind = info.rebind || { enabled: false, nets: [], excludes: [] };
  $: totalRequests = proxies.reduce((sum, proxy) => sum + Number(proxy.stat?.total_requests || 0), 0);
  $: totalProxyRequests = proxies.reduce((sum, proxy) => sum + Number(proxy.stat?.proxy_requests_sent || 0), 0);
  $: totalCacheHits = proxies.reduce((sum, proxy) => sum + Number(proxy.stat?.cache_hits || 0), 0);
  $: cachePct = totalRequests ? Math.round(totalCacheHits / totalRequests * 100) : 0;

  function profileLabel(name = '') {
    if (name === 'System') return locale === 'en' ? 'System' : 'Системный';
    const match = ($snapshot.upstreams || []).find((u) => u.profile === name && u.policy_description);
    return match?.policy_description || name || '—';
  }

  function logicalUpstreams(items = []) {
    const grouped = new Map();
    for (const proxy of items) {
      for (const upstream of proxy.upstreams || []) {
        const key = [proxy.name, upstream.protocol, upstream.address, upstream.port, upstream.sni, upstream.interface].join('|');
        let row = grouped.get(key);
        if (!row) {
          row = {
            ...upstream,
            profile: proxy.name,
            domains: [],
            instances: 0,
            r_sent: 0,
            a_rcvd: 0,
            nx_rcvd: 0
          };
          grouped.set(key, row);
        }
        const domain = String(upstream.domain || '').trim();
        if (domain && !row.domains.includes(domain)) row.domains.push(domain);
        row.instances += 1;
        row.r_sent += Number(upstream.r_sent || 0);
        row.a_rcvd += Number(upstream.a_rcvd || 0);
        row.nx_rcvd += Number(upstream.nx_rcvd || 0);
      }
    }
    return [...grouped.values()].sort((a, b) => {
      const profileOrder = String(a.profile).localeCompare(String(b.profile), undefined, { numeric: true });
      if (profileOrder) return profileOrder;
      return String(a.name || a.address).localeCompare(String(b.name || b.address), undefined, { numeric: true });
    });
  }

  function endpoint(upstream = {}) {
    if (upstream.protocol === 'DoH' && upstream.target) return upstream.target;
    const address = upstream.address || upstream.target || '—';
    return upstream.port ? `${address}:${upstream.port}` : address;
  }

  function cachePercent(proxy = {}) {
    return Math.round(Number(proxy.stat?.cache_hit_ratio || 0) * 100);
  }

  async function load() {
    loading = true;
    error = '';
    actionMessage = '';
    try {
      info = await getDNSInfo();
    } catch (e) {
      error = e?.payload?.error || e?.message || t(locale, 'dns.tools.dnsInfoUnavailable');
    } finally {
      loading = false;
    }
  }

  function reportText() {
    const lines = [
      'RouterForge DNS diagnostics',
      `${t(locale, 'dns.tools.generated')}: ${info.generated_at ? new Date(info.generated_at).toLocaleString(locale === 'en' ? 'en-GB' : 'ru-RU') : '—'}`,
      `${t(locale, 'dns.tools.proxies')}: ${proxies.length}`,
      `${t(locale, 'dns.tools.upstreams')}: ${logical.length}`,
      ''
    ];

    for (const proxy of proxies) {
      lines.push(`${profileLabel(proxy.name)} (${proxy.name})`);
      lines.push(`TCP ${proxy.tcp_port || '—'} / UDP ${proxy.udp_port || '—'}`);
      lines.push(`${t(locale, 'common.requests')}: ${proxy.stat?.total_requests || 0}`);
      lines.push(`${t(locale, 'dns.tools.proxyRequests')}: ${proxy.stat?.proxy_requests_sent || 0}`);
      lines.push(`${t(locale, 'dns.tools.cacheHit')}: ${cachePercent(proxy)}% (${proxy.stat?.cache_hits || 0})`);
      if (proxy.stat?.memory) lines.push(`${t(locale, 'dns.tools.memory')}: ${proxy.stat.memory}`);
      for (const upstream of proxy.upstreams || []) {
        lines.push(`- [${upstream.protocol}] ${endpoint(upstream)} · SNI ${upstream.sni || '—'} · ${t(locale, 'dns.tools.domains')}: ${upstream.domain || t(locale, 'dns.tools.allDomains')} · sent ${upstream.r_sent || 0} · received ${upstream.a_rcvd || 0} · NX ${upstream.nx_rcvd || 0} · median ${upstream.med_resp || '—'} · avg ${upstream.avg_resp || '—'} · rank ${upstream.rank || 0}`);
      }
      lines.push('');
    }

    lines.push(`${t(locale, 'dns.tools.staticRecords')}: ${staticRecords.length}`);
    for (const record of staticRecords) lines.push(`- ${record.host} ${record.type} ${record.value} (flag ${record.flag || 0})`);
    lines.push('');
    lines.push(`${t(locale, 'dns.tools.rebindProtection')}: ${rebind.enabled ? t(locale, 'common.enabled') : t(locale, 'common.disabled')}`);
    lines.push(`${t(locale, 'dns.tools.protectedNetworks')}: ${(rebind.nets || []).join(', ') || '—'}`);
    lines.push(`${t(locale, 'dns.tools.exclusions')}: ${(rebind.excludes || []).join(', ') || '—'}`);
    return `${lines.join('\n')}\n`;
  }

  async function copyReport() {
    const text = reportText();
    let ok = false;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        ok = true;
      }
    } catch {}
    if (!ok) {
      try {
        const area = document.createElement('textarea');
        area.value = text;
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.focus();
        area.select();
        ok = document.execCommand('copy');
        area.remove();
      } catch {}
    }
    actionKind = ok ? 'good' : 'error';
    actionMessage = ok ? t(locale, 'dns.tools.copied') : t(locale, 'dns.tools.copyFailed');
  }

  function saveReport() {
    const blob = new Blob([reportText()], { type: 'text/plain;charset=utf-8' });
    const href = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    const stamp = new Date().toISOString().replace(/[:.]/g, '-');
    anchor.href = href;
    anchor.download = `routerforge-dns-info-${stamp}.txt`;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(href);
  }

  onMount(load);
</script>

<div class="dns-info-shell">
  <div class="toolbar dns-info-toolbar">
    <button class="button" onclick={load} disabled={loading}>{loading ? t(locale, 'common.loading') : t(locale, 'common.refresh')}</button>
    <button class="button" onclick={copyReport} disabled={!proxies.length || loading}>{t(locale, 'dns.tools.copyDnsInfo')}</button>
    <button class="button" onclick={saveReport} disabled={!proxies.length || loading}>{t(locale, 'dns.tools.saveDnsInfo')}</button>
    <div class="toolbar-spacer"></div>
    {#if actionMessage}<span class="state-chip {actionKind}">{actionMessage}</span>{/if}
    <span class="state-chip neutral">{t(locale, 'dns.tools.dnsInfoReadOnly')}</span>
  </div>

  {#if loading && !proxies.length}
    <section class="panel"><div class="empty">{t(locale, 'dns.tools.dnsInfoLoading')}</div></section>
  {:else if error}
    <section class="panel"><div class="empty bad">{t(locale, 'dns.tools.dnsInfoUnavailable')} · {error}</div></section>
  {:else if !proxies.length}
    <section class="panel"><div class="empty">{t(locale, 'dns.tools.noDnsProxyData')}</div></section>
  {:else}
    <section class="metric-grid four dns-info-metrics">
      <div class="metric-card"><span>{t(locale, 'dns.tools.proxies')}</span><strong>{proxies.length}</strong><small>{fmtInt(totalRequests, locale)} {t(locale, 'common.requests').toLowerCase()}</small></div>
      <div class="metric-card"><span>{t(locale, 'dns.tools.upstreams')}</span><strong>{logical.length}</strong><small>{fmtInt(totalProxyRequests, locale)} {t(locale, 'dns.tools.proxyRequests').toLowerCase()}</small></div>
      <div class="metric-card"><span>{t(locale, 'dns.tools.cache')}</span><strong>{cachePct}%</strong><small>{fmtInt(totalCacheHits, locale)} cache hits</small></div>
      <div class="metric-card"><span>{t(locale, 'dns.tools.staticRecords')}</span><strong>{staticRecords.length}</strong><small>{t(locale, 'dns.tools.rebindProtection')}: {rebind.enabled ? t(locale, 'common.enabled') : t(locale, 'common.disabled')}</small></div>
    </section>

    <section class="panel table-panel">
      <div class="panel-head">
        <div><strong>{t(locale, 'dns.tools.upstreams')}</strong><span>{t(locale, 'dns.tools.upstreamsHint')}</span></div>
        <span class="panel-meta">{logical.length}</span>
      </div>
      <div class="table-scroll"><table class="dns-upstreams-table">
        <thead><tr><th>{t(locale, 'common.profile')}</th><th>DNS</th><th>{t(locale, 'common.type')}</th><th>{t(locale, 'dns.tools.sni')}</th><th>{t(locale, 'dns.tools.domains')}</th><th>{t(locale, 'dns.tools.sent')}</th><th>{t(locale, 'dns.tools.received')}</th><th>NX</th><th>{t(locale, 'dns.tools.rank')}</th></tr></thead>
        <tbody>
          {#each logical as upstream (`${upstream.profile}|${upstream.protocol}|${upstream.address}|${upstream.port}|${upstream.sni}|${upstream.interface}`)}
            <tr>
              <td><div class="cell-title">{profileLabel(upstream.profile)}</div><div class="cell-sub mono">{upstream.profile}</div></td>
              <td><div class="cell-title">{upstream.name || upstream.address}</div><div class="cell-sub mono wrap-anywhere">{endpoint(upstream)}</div></td>
              <td><span class="pill accent">{upstream.protocol}</span>{#if upstream.instances > 1}<div class="cell-sub">×{upstream.instances}</div>{/if}</td>
              <td class="mono wrap-anywhere">{upstream.sni || '—'}</td>
              <td><div class="domain-list-inline">{#if upstream.domains.length}{#each upstream.domains as domain}<span class="domain-chip mono">.{String(domain).replace(/^\./, '')}</span>{/each}{:else}<span class="muted">{t(locale, 'dns.tools.allDomains')}</span>{/if}</div></td>
              <td class="mono">{fmtInt(upstream.r_sent || 0, locale)}</td>
              <td class="mono">{fmtInt(upstream.a_rcvd || 0, locale)}</td>
              <td class="mono">{fmtInt(upstream.nx_rcvd || 0, locale)}</td>
              <td><span class="pill">{upstream.rank || '—'}</span></td>
            </tr>
          {/each}
        </tbody>
      </table></div>
    </section>

    <section class="panel policy-panel">
      <div class="panel-head"><div><strong>{t(locale, 'dns.tools.policyStats')}</strong><span>{t(locale, 'dns.tools.policyStatsHint')}</span></div></div>
      <div class="policy-list">
        {#each proxies as proxy, index (proxy.name)}
          <details class="policy-card" open={index === 0}>
            <summary>
              <div class="policy-summary-main"><strong>{profileLabel(proxy.name)}</strong><span class="mono">{proxy.name} · TCP :{proxy.tcp_port || '—'} · UDP :{proxy.udp_port || '—'}</span></div>
              <div class="policy-summary-metrics">
                <span><strong>{fmtInt(proxy.stat?.total_requests || 0, locale)}</strong><small>{t(locale, 'common.requests')}</small></span>
                <span><strong>{fmtInt(proxy.stat?.proxy_requests_sent || 0, locale)}</strong><small>{t(locale, 'dns.tools.proxyRequests')}</small></span>
                <span><strong>{cachePercent(proxy)}%</strong><small>{t(locale, 'dns.tools.cacheHit')}</small></span>
                <span><strong>{proxy.stat?.memory || '—'}</strong><small>{t(locale, 'dns.tools.memory')}</small></span>
              </div>
            </summary>
            <div class="policy-body table-scroll"><table>
              <thead><tr><th>DNS</th><th>{t(locale, 'dns.tools.domains')}</th><th>{t(locale, 'dns.tools.sent')}</th><th>{t(locale, 'dns.tools.received')}</th><th>NX</th><th>{t(locale, 'dns.tools.median')}</th><th>{t(locale, 'dns.tools.average')}</th><th>{t(locale, 'dns.tools.rank')}</th></tr></thead>
              <tbody>
                {#each proxy.upstreams || [] as upstream, upstreamIndex (`${proxy.name}|${upstream.local_port}|${upstream.domain}|${upstreamIndex}`)}
                  <tr>
                    <td><div class="cell-title">{upstream.name || upstream.address}</div><div class="cell-sub mono wrap-anywhere">{endpoint(upstream)}{upstream.interface ? ` · ${upstream.interface}` : ''}</div></td>
                    <td>{upstream.domain ? `.${String(upstream.domain).replace(/^\./, '')}` : t(locale, 'dns.tools.allDomains')}</td>
                    <td class="mono">{fmtInt(upstream.r_sent || 0, locale)}</td>
                    <td class="mono">{fmtInt(upstream.a_rcvd || 0, locale)}</td>
                    <td class="mono">{fmtInt(upstream.nx_rcvd || 0, locale)}</td>
                    <td class="mono">{upstream.med_resp || '—'}</td>
                    <td class="mono">{upstream.avg_resp || '—'}</td>
                    <td><span class="pill accent">{upstream.rank || '—'}</span></td>
                  </tr>
                {/each}
              </tbody>
            </table></div>
          </details>
        {/each}
      </div>
    </section>

    <div class="two-col dns-shared-grid">
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{t(locale, 'dns.tools.staticRecords')}</strong><span>{t(locale, 'common.entryCount', { count: staticRecords.length })}</span></div></div>
        {#if staticRecords.length}
          <div class="table-scroll"><table class="static-table"><thead><tr><th>{t(locale, 'dns.tools.host')}</th><th>{t(locale, 'common.type')}</th><th>{t(locale, 'dns.tools.value')}</th><th>Flag</th></tr></thead><tbody>
            {#each staticRecords as record (`${record.host}|${record.type}|${record.value}|${record.flag}`)}<tr><td class="mono wrap-anywhere">{record.host}</td><td><span class="pill {record.type === 'AAAA' ? '' : 'good'}">{record.type}</span></td><td class="mono wrap-anywhere">{record.value}</td><td class="mono">{record.flag}</td></tr>{/each}
          </tbody></table></div>
        {:else}<div class="empty">{t(locale, 'dns.tools.noStaticRecords')}</div>{/if}
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'dns.tools.rebindProtection')}</strong><span>{t(locale, 'dns.tools.rebindHint')}</span></div><span class="state-chip {rebind.enabled ? 'good' : 'neutral'}">{rebind.enabled ? t(locale, 'common.enabled') : t(locale, 'common.disabled')}</span></div>
        <div class="info-row"><div><strong>{t(locale, 'dns.tools.protectedNetworks')}</strong><span>norebind_ip4net</span></div><div class="info-value tag-value">{#if (rebind.nets || []).length}{#each rebind.nets as net}<span class="domain-chip mono">{net}</span>{/each}{:else}—{/if}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'dns.tools.exclusions')}</strong><span>norebind_exclude</span></div><div class="info-value tag-value">{#if (rebind.excludes || []).length}{#each rebind.excludes as item}<span class="domain-chip mono">{item}</span>{/each}{:else}—{/if}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'dns.tools.generated')}</strong><span>show dns-proxy</span></div><div class="info-value mono">{info.generated_at ? new Date(info.generated_at).toLocaleString(locale === 'en' ? 'en-GB' : 'ru-RU') : '—'}</div></div>
      </section>
    </div>
  {/if}
</div>

<style>
  .dns-info-shell { margin-top: 14px; }
  .dns-info-toolbar { margin-bottom: 14px; }
  .dns-info-toolbar button:disabled { opacity: .55; cursor: not-allowed; }
  .dns-info-metrics { margin-top: 0; }
  .dns-upstreams-table { min-width: 1120px; }
  .wrap-anywhere { overflow-wrap: anywhere; word-break: break-word; }
  .domain-list-inline, .tag-value { display: flex; flex-wrap: wrap; gap: 5px; align-items: center; }
  .tag-value { justify-content: flex-end; }
  .domain-chip { display: inline-flex; min-height: 20px; align-items: center; padding: 0 6px; border: 1px solid rgba(56,189,248,.22); border-radius: 999px; background: rgba(56,189,248,.06); color: var(--accent); font-size: 9px; }
  .policy-list { padding: 8px; display: grid; gap: 8px; }
  .policy-card { border: 1px solid var(--border); border-radius: 7px; overflow: hidden; background: var(--bg); }
  .policy-card summary { min-height: 58px; padding: 10px 12px; display: flex; align-items: center; gap: 18px; cursor: pointer; list-style: none; }
  .policy-card summary::-webkit-details-marker { display: none; }
  .policy-card summary::before { content: '›'; color: var(--muted); font-size: 17px; transition: transform .12s ease; }
  .policy-card[open] summary::before { transform: rotate(90deg); }
  .policy-summary-main { min-width: 160px; }
  .policy-summary-main strong { display: block; color: #fff; font-size: 11px; }
  .policy-summary-main span { display: block; margin-top: 4px; color: var(--muted); font-size: 9px; }
  .policy-summary-metrics { margin-left: auto; display: grid; grid-template-columns: repeat(4, minmax(82px, 1fr)); gap: 8px; }
  .policy-summary-metrics>span { min-width: 0; text-align: right; }
  .policy-summary-metrics strong { display: block; color: #fff; font: 600 12px/1.2 var(--font-mono); }
  .policy-summary-metrics small { display: block; margin-top: 3px; color: var(--muted); font-size: 8px; text-transform: uppercase; }
  .policy-body { border-top: 1px solid var(--border); }
  .policy-body table { min-width: 920px; }
  .dns-shared-grid { align-items: start; }
  .static-table { min-width: 620px; }

  @media (max-width: 900px) {
    .policy-card summary { align-items: flex-start; flex-wrap: wrap; }
    .policy-summary-metrics { width: calc(100% - 30px); margin-left: 30px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .policy-summary-metrics>span { text-align: left; }
    .dns-shared-grid { grid-template-columns: minmax(0, 1fr); }
  }

  @media (max-width: 640px) {
    .dns-info-toolbar { align-items: stretch; flex-wrap: wrap; }
    .dns-info-toolbar .button { flex: 1 1 calc(50% - 4px); }
    .dns-info-toolbar .toolbar-spacer { display: none; }
    .dns-info-toolbar .state-chip { flex: 1 1 100%; justify-content: center; }
  }
</style>
