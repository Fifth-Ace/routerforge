<script>
  import DNSNav from '$lib/components/DNSNav.svelte';
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { getPlainDNS } from '$lib/api.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, quality, qualityClass, groupBy } from '$lib/utils.js';

  let mode = 'plain';
  let selectedPort = 0;
  let plain = { resolvers: [], recent: [], pending: 0 };
  let plainKey = '';
  let plainError = '';
  let plainTimer = null;

  $: allUpstreams = $snapshot.upstreams || [];
  $: systemUpstreams = allUpstreams.filter((u) => u.profile === 'System');
  $: upstreams = systemUpstreams.length ? systemUpstreams : allUpstreams;
  $: policyGroups = Object.entries(groupBy(allUpstreams.filter((u)=>u.profile!=='System'),'profile')).sort(([a],[b])=>a.localeCompare(b,undefined,{numeric:true}));
  $: if (upstreams.length && !upstreams.some((u) => Number(u.port) === Number(selectedPort))) selectedPort = Number(upstreams[0].port);
  $: selected = upstreams.find((u) => Number(u.port) === Number(selectedPort)) || upstreams[0] || null;
  $: win = selected?.stats_5m || {};
  $: selectedStatus = statusFor(selected || {});
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

  const qualityWindows = [['5 минут','stats_5m'],['1 час','stats_1h'],['24 часа','stats_24h']];

  function resolverKey(r = {}) {
    return `${r.address || ''}|${r.port || 53}`;
  }

  function plainState(r = {}) {
    const requests = Number(r.requests || 0);
    const responses = Number(r.responses || 0);
    const timeouts = Number(r.timeouts || 0);
    if (!requests) return { cls: 'neutral', label: 'ОБНАРУЖЕН' };
    if (timeouts > 0 && responses === 0) return { cls: 'error', label: 'TIMEOUT' };
    if (timeouts > 0) return { cls: 'warn', label: 'ЕСТЬ TIMEOUT' };
    return { cls: 'good', label: 'АКТИВЕН' };
  }

  function plainSuccess(r = {}) {
    const requests = Number(r.requests || 0);
    return requests ? Number(r.responses || 0) / requests * 100 : 100;
  }

  function assessment(d = {}) {
    if (d.assessment === 'POLICY_ROUTE_FAIL') return 'Policy route не проходит, default route доступен';
    if (d.assessment === 'INTERFACE_ROUTE_FAIL') return 'Маршрут через заданный интерфейс не проходит, default route доступен';
    if (d.assessment === 'UPSTREAM_OK_LOCAL_PROXY_FAIL') return 'Policy/interface route до upstream работает; ломается локальный DNS proxy Keenetic';
    if (d.assessment === 'UPSTREAM_OK_LOCAL_PATH_FAIL') return 'Upstream доступен; вероятна проблема локального DNS proxy / Policy Keenetic';
    if (d.assessment === 'UPSTREAM_OK') return 'Upstream доступен напрямую';
    if (String(d.assessment || '').startsWith('UPSTREAM_')) return `Ошибка upstream на этапе ${d.stage || '—'}`;
    return '';
  }

  async function refreshPlain() {
    try {
      plain = await getPlainDNS(160);
      plainError = '';
    } catch (error) {
      plainError = error?.message || 'Plain DNS API недоступен';
    }
  }

  onMount(() => {
    refreshPlain();
    plainTimer = setInterval(() => {
      if (!document.hidden) refreshPlain();
    }, 2500);
    return () => clearInterval(plainTimer);
  });
</script>

<svelte:head><title>RouterForge — Серверы</title></svelte:head>

<div class="page">
  <DNSNav />
  <div class="page-head">
    <div><h1>Серверы</h1><p>Основной DNS и защищённые DoT/DoH без служебных policy-дублей.</p></div>
    <span class="page-kicker mono">{plainResolvers.length} MAIN · {upstreams.length} PROTECTED</span>
  </div>

  <div class="subtabs server-mode-tabs">
    <button class:active={mode === 'plain'} onclick={() => mode = 'plain'}>Основной DNS <span class="pill">{plainResolvers.length}</span></button>
    <button class:active={mode === 'encrypted'} onclick={() => mode = 'encrypted'}>Защищённые DNS <span class="pill">{upstreams.length}</span></button>
  </div>

  {#if mode === 'plain'}
    {#if plainError}
      <section class="panel"><div class="empty">{plainError}</div></section>
    {:else if !selectedPlain}
      <section class="panel">
        <div class="panel-head"><div><strong>Основной DNS</strong><span>show ip name-server + show dns-proxy</span></div><span class="state-chip neutral">0 DNS</span></div>
        <div class="empty">Обычные DNS UDP/TCP :53 не обнаружены.</div>
      </section>
    {:else}
      {@const pst = plainState(selectedPlain)}
      <div class="server-layout">
        <aside class="resolver-list panel">
          <div class="panel-head"><div><strong>Основной DNS</strong><span>{plainResolvers.length} обнаружено</span></div></div>
          <div class="resolver-items">
            {#each plainResolvers as r (resolverKey(r))}
              {@const st = plainState(r)}
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
              <div><strong>{fmtInt(selectedPlain.requests || 0)}</strong><span>Запросы</span></div>
              <div><strong>{selectedPlain.p95_latency_ms ? fmtMs(selectedPlain.p95_latency_ms) : '—'}</strong><span>P95 latency</span></div>
              <div><strong>{selectedPlain.avg_latency_ms ? fmtMs(selectedPlain.avg_latency_ms) : '—'}</strong><span>Avg latency</span></div>
              <div><strong class={plainSuccess(selectedPlain) < 95 ? 'warn-text' : 'good'}>{plainSuccess(selectedPlain).toFixed(1)}%</strong><span>Ответы</span></div>
            </div>
          </section>

          {#if $settings.uiLevel==='advanced'}
            <section class="panel">
              <div class="panel-head"><div><strong>Расширенная статистика</strong><span>Passive request → response correlation</span></div><span class="state-chip info">РАСШИРЕННЫЙ</span></div>
              <div class="info-row"><div><strong>Responses</strong><span>Сопоставленные ответы</span></div><div class="info-value">{fmtInt(selectedPlain.responses || 0)}</div></div>
              <div class="info-row"><div><strong>Timeouts</strong><span>Нет ответа за 8 секунд</span></div><div class="info-value {Number(selectedPlain.timeouts || 0) ? 'warn-text' : ''}">{fmtInt(selectedPlain.timeouts || 0)}</div></div>
              <div class="info-row"><div><strong>DNS errors</strong><span>RCODE кроме NOERROR/NXDOMAIN</span></div><div class="info-value">{fmtInt(selectedPlain.errors || 0)}</div></div>
              <div class="info-row"><div><strong>NXDOMAIN</strong><span>Отдельно от transport/errors</span></div><div class="info-value">{fmtInt(selectedPlain.nxdomain || 0)}</div></div>
              <div class="info-row"><div><strong>Источник</strong><span>Откуда Keenetic получил resolver</span></div><div class="info-value">{selectedPlain.source || '—'}</div></div>
              <div class="info-row"><div><strong>Интерфейс</strong><span>Keenetic interface</span></div><div class="info-value">{selectedPlain.interface || '—'}</div></div>
              <div class="info-row"><div><strong>Profiles</strong><span>Дополнительные DNS proxy context</span></div><div class="info-value">{(selectedPlain.profiles || []).join(' · ') || '—'}</div></div>
              <div class="info-row"><div><strong>Domain filters</strong><span>Привязанные домены</span></div><div class="info-value">{(selectedPlain.domains || []).join(' · ') || 'все домены'}</div></div>
              <div class="plain-dns-warning">{plain.note}</div>
            </section>
          {/if}

          <section class="panel table-panel">
            <div class="panel-head"><div><strong>Последние запросы</strong><span>{plainRecent.length} событий</span></div></div>
            <div class="table-scroll"><table><thead><tr><th>Время</th><th>Домен</th><th>Тип</th><th>RCODE</th><th>Latency</th><th>Status</th></tr></thead><tbody>
              {#if plainRecent.length}
                {#each plainRecent as event, index (`${event.time}-${event.resolver}-${event.port}-${event.protocol}-${event.domain}-${event.qtype}-${event.status}-${index}`)}
                  <tr><td class="mono">{new Date(event.time).toLocaleTimeString('ru-RU')}</td><td>{event.domain}</td><td><span class="pill">{event.qtype}</span></td><td>{event.rcode || '—'}</td><td class="mono">{event.latency_ms ? fmtMs(event.latency_ms) : '—'}</td><td><span class="state-chip {event.status === 'TIMEOUT' ? 'error' : 'good'}">{event.status}</span></td></tr>
                {/each}
              {:else}
                <tr><td colspan="6" class="empty">Пока нет observed plain-DNS traffic</td></tr>
              {/if}
            </tbody></table></div>
          </section>
        </div>
      </div>
    {/if}

  {:else if !selected}
    <section class="panel"><div class="empty">System DoT/DoH DNS серверы не обнаружены</div></section>
  {:else}
    <div class="server-layout">
      <aside class="resolver-list panel">
        <div class="panel-head"><div><strong>Защищённые DNS</strong><span>{upstreams.length} System resolver'ов</span></div></div>
        <div class="resolver-items">
          {#each upstreams as u (u.port)}
            {@const st = statusFor(u)}
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
            <div><strong>{fmtInt(win.requests || 0)}</strong><span>Запросы 5m</span></div>
            <div><strong>{win.p95_latency_ms ? fmtMs(win.p95_latency_ms) : '—'}</strong><span>P95 latency 5m</span></div>
            <div><strong>{fmtPct(win.fallback_pct || 0)}</strong><span>Fallback 5m</span></div>
            <div><strong class={qualityClass(win)}>{selectedQuality.toFixed(2)}%</strong><span>Quality 5m</span></div>
          </div>
        </section>

        {#if $settings.uiLevel==='advanced'}
          <section class="panel">
            <div class="panel-head"><div><strong>Качество resolver</strong><span>Окна статистики</span></div><span class="state-chip info">РАСШИРЕННЫЙ</span></div>
            {#each qualityWindows as [label,key]}
              {@const w = selected[key] || {}}
              <div class="info-row"><div><strong>{label}</strong><span>{fmtInt(w.requests || 0)} запросов · {fmtInt(w.errors || 0)} DNS errors · {fmtInt(w.timeouts || 0)} timeout</span></div><div class="info-value"><span class={qualityClass(w)}>{Number(w.quality_pct ?? 100).toFixed(2)}%</span> · p95 {w.p95_latency_ms ? fmtMs(w.p95_latency_ms) : '—'} · fallback {fmtPct(w.fallback_pct || 0)}</div></div>
            {/each}
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>Диагностика DoT/DoH</strong><span>автоматически при DOWN</span></div>{#if selected.diagnostic?.ran}<span class="state-chip {selected.diagnostic.status === 'FAIL' ? 'error' : 'good'}">{selected.diagnostic.status || '—'}</span>{/if}</div>
            {#if !selected.diagnostic?.ran}
              <div class="empty">Прямая диагностика ещё не запускалась.</div>
            {:else}
              {@const d = selected.diagnostic}
              <div class="info-row"><div><strong>Этап</strong><span>Последний достигнутый этап</span></div><div class="info-value">{d.stage || '—'}</div></div>
              <div class="info-row"><div><strong>Target IP</strong><span>IP прямой проверки</span></div><div class="info-value mono">{d.target_ip || '—'}</div></div>
              <div class="info-row"><div><strong>Resolve / TCP / TLS</strong><span>Этапы соединения</span></div><div class="info-value">{fmtMs(d.resolve_ms)} · {fmtMs(d.tcp_ms)} · {fmtMs(d.tls_ms)}</div></div>
              <div class="info-row"><div><strong>{selected.protocol === 'DoH' ? 'HTTP / DNS' : 'DNS'}</strong><span>Протокольная проверка</span></div><div class="info-value">{fmtMs(d.protocol_ms)}{selected.protocol === 'DoH' && d.http_status ? ` · HTTP ${d.http_status}` : ''}</div></div>
              <div class="info-row"><div><strong>DNS RCODE</strong><span>Ответ прямого DNS probe</span></div><div class="info-value">{d.dns_rcode || '—'}</div></div>
              {#if assessment(d)}<div class="info-row"><div><strong>Вывод</strong><span>{d.route_scope || ''}</span></div><div class="info-value warn-text">{assessment(d)}</div></div>{/if}
              <div class="info-row"><div><strong>Ошибка</strong><span>Причина остановки</span></div><div class="info-value">{d.error || 'нет'}</div></div>
            {/if}
          </section>

          <section class="panel">
            <div class="panel-head"><div><strong>Сведения о DNS</strong><span>Discovery metadata</span></div></div>
            <div class="info-row"><div><strong>Target</strong><span>Адрес upstream resolver</span></div><div class="info-value mono">{selected.target}</div></div>
            <div class="info-row"><div><strong>SNI</strong><span>TLS Server Name</span></div><div class="info-value mono">{selected.sni || '—'}</div></div>
            <div class="info-row"><div><strong>Domain filter</strong><span>Доменная привязка resolver</span></div><div class="info-value">{selected.domain || 'все домены'}</div></div>
            <div class="info-row"><div><strong>Interface</strong><span>Исходящий интерфейс Keenetic</span></div><div class="info-value">{selected.interface || 'не задан'}</div></div>
            {#if selected.linux_interface}<div class="info-row"><div><strong>Linux interface</strong><span>Сопоставленный netdev Linux</span></div><div class="info-value mono">{selected.linux_interface}</div></div>{/if}
            <div class="info-row"><div><strong>Local port</strong><span>Внутренний resolver proxy</span></div><div class="info-value mono">{selected.port}</div></div>
            <div class="info-row"><div><strong>System DNS proxy</strong><span>Порт профиля System</span></div><div class="info-value mono">:{selected.profile_dns_port || 53}</div></div>
            <div class="info-row"><div><strong>Timeout / Proceed</strong><span>Keenetic fallback thresholds</span></div><div class="info-value mono">{selected.timeout_ms ? `${selected.timeout_ms} ms` : '—'} / {selected.proceed_ms ? `${selected.proceed_ms} ms` : '—'}</div></div>
          </section>
        {/if}

        <section class="panel table-panel">
          <div class="panel-head"><div><strong>Последние запросы</strong><span>{fmtAgo(selected.last_request)}</span></div></div>
          <div class="table-scroll"><table><thead><tr><th>Время</th><th>Домен</th><th>Тип</th><th>Fallback</th></tr></thead><tbody>
            {#if recentFlow.length}
              {#each recentFlow as f (`${f.time}-${f.domain}-${f.qtype}`)}<tr><td class="mono">{new Date(f.time).toLocaleTimeString('ru-RU')}</td><td>{f.domain}</td><td><span class="pill">{f.qtype}</span></td><td>{#if f.fallback}<span class="pill warn">да</span>{:else}—{/if}</td></tr>{/each}
            {:else}<tr><td colspan="4" class="empty">Нет live-запросов</td></tr>{/if}
          </tbody></table></div>
        </section>
      </div>
    </div>
  {/if}

  {#if $settings.uiLevel==='advanced'&&policyGroups.length}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Маршрутные DNS-контексты</strong><span>Служебные Policy Keenetic · отдельно от основных DNS</span></div><span class="state-chip info">РАСШИРЕННЫЙ</span></div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>Policy</th><th>Маршрут</th><th>DNS proxy</th><th>Resolver'ы</th><th>Mark / table</th></tr></thead>
          <tbody>
            {#each policyGroups as [name,items]}
              {@const first=items[0]||{}}
              <tr>
                <td><div class="cell-title">{first.policy_description||name}</div><div class="cell-sub mono">{name}</div></td>
                <td>{#if first.policy_has_default}<span class="state-chip good">Есть default route</span>{:else}<span class="state-chip neutral">Без default route</span>{/if}</td>
                <td class="mono">:{first.profile_dns_port||'—'}</td>
                <td>{items.length}</td>
                <td class="mono">0x{Number(first.policy_mark||0).toString(16)} · {first.policy_table||'—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</div>
