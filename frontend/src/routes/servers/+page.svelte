<script>
  import { snapshot } from '$lib/stores/snapshot.js';
  import { fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, quality, qualityClass } from '$lib/utils.js';

  let selectedPort=0;

  $: upstreams=$snapshot.upstreams||[];
  $: if(upstreams.length&&!upstreams.some((u)=>Number(u.port)===Number(selectedPort)))selectedPort=Number(upstreams[0].port);
  $: selected=upstreams.find((u)=>Number(u.port)===Number(selectedPort))||upstreams[0]||null;
  $: win=selected?.stats_5m||{};
  $: selectedStatus=statusFor(selected||{});
  $: selectedQuality=selected?quality(selected):100;
  $: recentFlow=selected?($snapshot.flow||[]).filter((f)=>Number(f.port)===Number(selected.port)).slice(-18).reverse():[];

  const qualityWindows=[['5 минут','stats_5m'],['1 час','stats_1h'],['24 часа','stats_24h']];

  function assessment(d={}) {
    if(d.assessment==='POLICY_ROUTE_FAIL')return 'Policy route не проходит, default route доступен';
    if(d.assessment==='INTERFACE_ROUTE_FAIL')return 'Маршрут через заданный интерфейс не проходит, default route доступен';
    if(d.assessment==='UPSTREAM_OK_LOCAL_PROXY_FAIL')return 'Policy/interface route до upstream работает; ломается локальный DNS proxy Keenetic';
    if(d.assessment==='UPSTREAM_OK_LOCAL_PATH_FAIL')return 'Upstream доступен; вероятна проблема локального DNS proxy / Policy Keenetic';
    if(d.assessment==='UPSTREAM_OK')return 'Upstream доступен напрямую';
    if(String(d.assessment||'').startsWith('UPSTREAM_'))return `Ошибка upstream на этапе ${d.stage||'—'}`;
    return '';
  }
</script>

<svelte:head><title>DNS Monitor — Серверы</title></svelte:head>

<div class="page">
  <div class="page-head"><div><h1>Серверы</h1><p>Состояние каждого локального DNS proxy и его реального upstream.</p></div><span class="page-kicker mono">{upstreams.length} RESOLVERS</span></div>

  {#if !selected}
    <section class="panel"><div class="empty">DNS серверы не обнаружены</div></section>
  {:else}
    <div class="server-layout">
      <aside class="resolver-list panel">
        <div class="panel-head"><div><strong>DNS серверы</strong><span>{upstreams.length} обнаружено</span></div></div>
        <div class="resolver-items">
          {#each upstreams as u (u.port)}
            {@const st=statusFor(u)}
            <button class="resolver-item" class:active={Number(u.port)===Number(selectedPort)} onclick={()=>selectedPort=Number(u.port)}>
              <div class="resolver-main"><span class="status-dot {st.cls}"></span><strong>{u.name}</strong><span class="pill accent">{u.protocol}</span></div>
              <span>{u.profile} · :{u.port}</span>
            </button>
          {/each}
        </div>
      </aside>

      <div class="stack">
        <section class="hero-card">
          <div class="hero-head"><div><h2>{selected.name} <span class="pill accent">{selected.protocol}</span></h2><p>{selected.profile} · {selected.target}</p></div><span class="state-chip {selectedStatus.cls}">{selectedStatus.label}</span></div>
          <div class="hero-metrics">
            <div><strong>{fmtInt(win.requests||0)}</strong><span>Запросы 5m</span></div>
            <div><strong>{win.p95_latency_ms?fmtMs(win.p95_latency_ms):'—'}</strong><span>P95 latency 5m</span></div>
            <div><strong>{fmtPct(win.fallback_pct||0)}</strong><span>Fallback 5m</span></div>
            <div><strong class={qualityClass(win)}>{selectedQuality.toFixed(2)}%</strong><span>Quality 5m</span></div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head"><div><strong>Качество resolver</strong><span>Окна статистики</span></div></div>
          {#each qualityWindows as [label,key]}
            {@const w=selected[key]||{}}
            <div class="info-row"><div><strong>{label}</strong><span>{fmtInt(w.requests||0)} запросов · {fmtInt(w.errors||0)} DNS errors · {fmtInt(w.timeouts||0)} timeout</span></div><div class="info-value"><span class={qualityClass(w)}>{Number(w.quality_pct??100).toFixed(2)}%</span> · p95 {w.p95_latency_ms?fmtMs(w.p95_latency_ms):'—'} · fallback {fmtPct(w.fallback_pct||0)}</div></div>
          {/each}
        </section>

        <section class="panel">
          <div class="panel-head"><div><strong>Диагностика DoT/DoH</strong><span>автоматически при DOWN</span></div>{#if selected.diagnostic?.ran}<span class="state-chip {selected.diagnostic.status==='FAIL'?'error':'good'}">{selected.diagnostic.status||'—'}</span>{/if}</div>
          {#if !selected.diagnostic?.ran}
            <div class="empty">Прямая диагностика ещё не запускалась. Она стартует автоматически, если health-check дважды подряд переведёт resolver в DOWN.</div>
          {:else}
            {@const d=selected.diagnostic}
            <div class="info-row"><div><strong>Этап</strong><span>Последний достигнутый этап</span></div><div class="info-value">{d.stage||'—'}</div></div>
            <div class="info-row"><div><strong>Target IP</strong><span>IP прямой проверки</span></div><div class="info-value mono">{d.target_ip||'—'}</div></div>
            <div class="info-row"><div><strong>Resolve / TCP / TLS</strong><span>Этапы соединения</span></div><div class="info-value">{fmtMs(d.resolve_ms)} · {fmtMs(d.tcp_ms)} · {fmtMs(d.tls_ms)}</div></div>
            <div class="info-row"><div><strong>{selected.protocol==='DoH'?'HTTP / DNS':'DNS'}</strong><span>Протокольная проверка</span></div><div class="info-value">{fmtMs(d.protocol_ms)}{selected.protocol==='DoH'&&d.http_status?` · HTTP ${d.http_status}`:''}</div></div>
            <div class="info-row"><div><strong>DNS RCODE</strong><span>Ответ прямого DNS probe</span></div><div class="info-value">{d.dns_rcode||'—'}</div></div>
            {#if assessment(d)}<div class="info-row"><div><strong>Вывод</strong><span>{d.route_scope||''}</span></div><div class="info-value warn-text">{assessment(d)}</div></div>{/if}
            <div class="info-row"><div><strong>Ошибка</strong><span>Причина остановки</span></div><div class="info-value">{d.error||'нет'}</div></div>
          {/if}
        </section>

        <section class="panel">
          <div class="panel-head"><div><strong>Сведения о DNS</strong><span>Discovery metadata</span></div></div>
          <div class="info-row"><div><strong>Target</strong><span>Адрес upstream resolver</span></div><div class="info-value mono">{selected.target}</div></div>
          <div class="info-row"><div><strong>Профиль</strong><span>Профиль DNS proxy Keenetic</span></div><div class="info-value">{selected.profile}</div></div>
          {#if selected.policy_mark}<div class="info-row"><div><strong>Policy route</strong><span>Динамическая policy routing Keenetic</span></div><div class="info-value mono">mark 0x{Number(selected.policy_mark).toString(16)} · table {selected.policy_table||'—'}</div></div>{/if}
          <div class="info-row"><div><strong>SNI</strong><span>TLS Server Name</span></div><div class="info-value mono">{selected.sni||'—'}</div></div>
          <div class="info-row"><div><strong>Domain filter</strong><span>Доменная привязка resolver</span></div><div class="info-value">{selected.domain||'все домены'}</div></div>
          <div class="info-row"><div><strong>Interface</strong><span>Исходящий интерфейс Keenetic</span></div><div class="info-value">{selected.interface||'не задан'}</div></div>
          {#if selected.linux_interface}<div class="info-row"><div><strong>Linux interface</strong><span>Сопоставленный netdev Linux</span></div><div class="info-value mono">{selected.linux_interface}</div></div>{/if}
          <div class="info-row"><div><strong>Local port</strong><span>Внутренний DNS proxy порт</span></div><div class="info-value mono">{selected.port}</div></div>
          <div class="info-row"><div><strong>Timeout / Proceed</strong><span>Keenetic fallback thresholds</span></div><div class="info-value mono">{selected.timeout_ms?`${selected.timeout_ms} ms`:'—'} / {selected.proceed_ms?`${selected.proceed_ms} ms`:'—'}</div></div>
        </section>

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
</div>
