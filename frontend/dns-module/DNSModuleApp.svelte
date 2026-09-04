<script>
  import { onMount } from 'svelte';

  const API = '/api/modules/dns';
  const query = new URLSearchParams(window.location.search);
  let locale = query.get('locale') === 'en' ? 'en' : 'ru';
  let tab = ['overview','resolvers','rules','traffic','diagnostics'].includes(query.get('view')) ? query.get('view') : 'overview';
  let loading = true;
  let error = '';
  let success = '';
  let info = null;
  let resolverState = { resolvers: [], active_count: 0, disabled_count: 0, dynamic_count: 0, physical_entries: 0, dot_physical_entries: 0, dot_physical_limit: 0 };
  let snapshot = {};
  let advanced = false;
  let editorOpen = false;
  let editing = null;
  let saving = false;
  let form = blankForm();
  let refreshTimer = null;

  const strings = {
    ru: {
      title:'DNS', subtitle:'Управление резолверами Keenetic, правилами и работой DNS.',
      overview:'Обзор', resolvers:'Резолверы', rules:'Правила', traffic:'Трафик', diagnostics:'Диагностика',
      refresh:'Обновить', add:'Добавить DNS', edit:'Настроить', disable:'Отключить', enable:'Включить', remove:'Удалить',
      active:'АКТИВЕН', disabled:'ОТКЛЮЧЕН', dynamic:'ДИНАМИЧЕСКИЙ', global:'Все домены', scope:'Область запросов',
      requests:'Запросы', sent:'Передано', answers:'Ответы', avg:'Среднее', median:'Медиана', errors:'Ошибки', cache:'Ответы из кэша',
      activeResolvers:'Активные', disabledResolvers:'Отключённые', dynamicResolvers:'Динамические', physical:'Физические записи',
      native:'Native mode', nativeHint:'Один логический резолвер может разворачиваться в несколько нативных записей Keenetic.',
      noResolvers:'Резолверы не найдены.', noData:'Данных пока нет.', localRecords:'Локальные записи', rebind:'Защита DNS',
      policy:'DNS-контекст', upstreams:'Работа резолверов', advanced:'Advanced', copy:'Копировать отчёт', saveReport:'Сохранить отчёт',
      protocol:'Протокол', address:'Адрес', uri:'DoH URL', port:'Порт', sni:'SNI / FQDN', iface:'Интерфейс', domains:'Домены',
      domainsHint:'По одному домену в строке. Пусто — для всех доменов.', spki:'SPKI', format:'Format', cancel:'Отмена', save:'Сохранить',
      confirmDelete:'Удалить этот резолвер из конфигурации Keenetic?', confirmDisable:'Временно отключить этот резолвер?', confirmEnable:'Вернуть этот резолвер в активную конфигурацию?',
      readOnly:'Управляется Keenetic автоматически', physicalCount:'Нативных записей', moduleError:'Не удалось загрузить данные DNS',
      system:'Системный', copied:'Отчёт скопирован.', saved:'Изменение применено и подтверждено readback-проверкой.',
      rollbackNote:'Все изменения проходят snapshot → mutation → save → readback; при несовпадении выполняется rollback.',
      topDomains:'Популярные домены', recentFlow:'Последние DNS-события', status:'Состояние', memory:'Память', rank:'Rank',
      rebindOn:'Включена', rebindOff:'Выключена', protectedNets:'Защищённые сети', exclusions:'Исключения',
      editTitle:'Настройка резолвера', addTitle:'Новый резолвер', all:'Все', moduleVersion:'Версия модуля',
      dotSlots:'DoT-слоты', afterSave:'После сохранения', limitExceeded:'Превышен лимит Keenetic'
    },
    en: {
      title:'DNS', subtitle:'Keenetic resolver management, rules and DNS runtime.',
      overview:'Overview', resolvers:'Resolvers', rules:'Rules', traffic:'Traffic', diagnostics:'Diagnostics',
      refresh:'Refresh', add:'Add DNS', edit:'Configure', disable:'Disable', enable:'Enable', remove:'Delete',
      active:'ACTIVE', disabled:'DISABLED', dynamic:'DYNAMIC', global:'All domains', scope:'Query scope',
      requests:'Requests', sent:'Sent', answers:'Answers', avg:'Average', median:'Median', errors:'Errors', cache:'Cache answers',
      activeResolvers:'Active', disabledResolvers:'Disabled', dynamicResolvers:'Dynamic', physical:'Physical entries',
      native:'Native mode', nativeHint:'One logical resolver can expand into multiple native Keenetic entries.',
      noResolvers:'No resolvers found.', noData:'No data yet.', localRecords:'Local records', rebind:'DNS protection',
      policy:'DNS context', upstreams:'Resolver runtime', advanced:'Advanced', copy:'Copy report', saveReport:'Save report',
      protocol:'Protocol', address:'Address', uri:'DoH URL', port:'Port', sni:'SNI / FQDN', iface:'Interface', domains:'Domains',
      domainsHint:'One domain per line. Empty means all domains.', spki:'SPKI', format:'Format', cancel:'Cancel', save:'Save',
      confirmDelete:'Delete this resolver from Keenetic configuration?', confirmDisable:'Temporarily disable this resolver?', confirmEnable:'Restore this resolver to active configuration?',
      readOnly:'Managed automatically by Keenetic', physicalCount:'Native entries', moduleError:'Could not load DNS data',
      system:'System', copied:'Report copied.', saved:'Change applied and confirmed by readback verification.',
      rollbackNote:'Every change uses snapshot → mutation → save → readback; mismatch triggers rollback.',
      topDomains:'Top domains', recentFlow:'Recent DNS events', status:'Status', memory:'Memory', rank:'Rank',
      rebindOn:'Enabled', rebindOff:'Disabled', protectedNets:'Protected networks', exclusions:'Exclusions',
      editTitle:'Configure resolver', addTitle:'New resolver', all:'All', moduleVersion:'Module version',
      dotSlots:'DoT slots', afterSave:'After save', limitExceeded:'Keenetic limit exceeded'
    }
  };
  $: L = strings[locale];
  $: resolvers = resolverState?.resolvers || [];
  $: activeResolvers = resolvers.filter((r) => !r.disabled && !r.dynamic);
  $: disabledResolvers = resolvers.filter((r) => r.disabled);
  $: dynamicResolvers = resolvers.filter((r) => r.dynamic);
  $: totalRequests = (info?.proxies || []).reduce((sum,p) => sum + Number(p.stat?.total_requests || 0), 0);
  $: totalSent = (info?.proxies || []).reduce((sum,p) => sum + Number(p.stat?.proxy_requests_sent || 0), 0);
  $: cacheHits = (info?.proxies || []).reduce((sum,p) => sum + Number(p.stat?.cache_hits || 0), 0);
  $: topDomains = snapshot?.top_domains || [];
  $: flow = snapshot?.flow || [];
  $: dotSlotsUsed = Number(resolverState?.dot_physical_entries || 0);
  $: dotSlotLimit = Number(resolverState?.dot_physical_limit || 0);
  $: formPhysicalCount = form.domains.split(/\r?\n|,/).map((v) => v.trim()).filter(Boolean).length || 1;
  $: editingActiveDoTSlots = editing && !editing.disabled && !editing.dynamic && editing.protocol === 'DoT' ? Number(editing.physical_count || 1) : 0;
  $: editorAffectsActive = !editing || (!editing.disabled && !editing.dynamic);
  $: formActiveDoTSlots = editorAffectsActive && form.protocol === 'DoT' ? formPhysicalCount : 0;
  $: projectedDoTSlots = Math.max(0, dotSlotsUsed - editingActiveDoTSlots + formActiveDoTSlots);
  $: dotCapacityExceeded = dotSlotLimit > 0 && projectedDoTSlots > dotSlotLimit;

  function blankForm() {
    return { protocol:'DoT', address:'', uri:'', port:853, sni:'', interface:'', domains:'', spki:'', format:'' };
  }

  async function request(path, options = {}) {
    const headers = { Accept:'application/json', ...(options.headers || {}) };
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      headers['X-RouterForge-Action'] = 'dns-control';
    }
    const response = await fetch(`${API}${path}`, { ...options, headers, cache:'no-store' });
    const text = await response.text();
    let data = null;
    try { data = text ? JSON.parse(text) : {}; } catch { data = { error:text || `HTTP ${response.status}` }; }
    if (!response.ok) throw new Error(data?.error || `HTTP ${response.status}`);
    return data;
  }

  async function loadAll(quiet = false) {
    if (!quiet) loading = true;
    error = '';
    try {
      const [nextInfo, nextResolvers, nextSnapshot] = await Promise.all([
        request('/info'), request('/resolvers'), request('/snapshot')
      ]);
      info = nextInfo;
      resolverState = nextResolvers;
      snapshot = nextSnapshot;
    } catch (e) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  function setTab(value) {
    tab = value;

    // Keep tab switching entirely inside the already-mounted DNS module.
    // The old parent.location.href assignment reloaded the Core route and the
    // iframe on every click, which caused a visible full-page jump.
    const next = new URL(window.location.href);
    next.searchParams.set('view', value);
    history.replaceState(history.state, '', next);

    // Keep the friendly parent URL in sync without asking SvelteKit/Core to
    // navigate. A refresh still lands on the matching compatibility wrapper.
    const parentRoutes = {
      overview: '/dns',
      resolvers: '/dns/servers',
      rules: '/dns/routing',
      traffic: '/dns/traffic',
      diagnostics: '/dns/tools'
    };
    try {
      if (window.parent !== window && window.parent.location.pathname.startsWith('/dns')) {
        const target = parentRoutes[value] || '/dns';
        if (window.parent.location.pathname !== target) {
          window.parent.history.replaceState(window.parent.history.state, '', target);
        }
      }
    } catch {}
  }

  function scopes(resolver) {
    return resolver?.domains?.length ? resolver.domains : [];
  }

  function displayDomain(domain) {
    if (domain === 'xn--p1ai') return '.рф';
    return domain ? `.${domain}` : L.global;
  }

  function endpoint(resolver) {
    if (resolver.protocol === 'DoH') return resolver.uri || '—';
    const port = resolver.port || (resolver.protocol === 'DoT' ? 853 : 53);
    return `${resolver.address || '—'}:${port}`;
  }

  function statusClass(resolver) {
    if (resolver.disabled) return 'warn';
    if (resolver.dynamic) return 'neutral';
    return 'good';
  }

  function statusText(resolver) {
    if (resolver.disabled) return L.disabled;
    if (resolver.dynamic) return L.dynamic;
    return L.active;
  }

  function openAdd() {
    editing = null;
    form = blankForm();
    editorOpen = true;
  }

  function openEdit(resolver) {
    editing = resolver;
    form = {
      protocol: resolver.protocol || 'DoT', address: resolver.address || '', uri: resolver.uri || '',
      port: resolver.port || (resolver.protocol === 'DoH' ? 443 : resolver.protocol === 'DNS' ? 53 : 853),
      sni: resolver.sni || '', interface: resolver.interface || '',
      domains: (resolver.domains || []).join('\n'), spki: resolver.spki || '', format: resolver.format || ''
    };
    editorOpen = true;
  }

  function payloadFromForm() {
    return {
      protocol: form.protocol,
      address: form.protocol === 'DoH' ? '' : form.address.trim(),
      uri: form.protocol === 'DoH' ? form.uri.trim() : '',
      port: Number(form.port || 0),
      sni: form.protocol === 'DoT' ? form.sni.trim() : '',
      interface: form.interface.trim(),
      domains: form.domains.split(/\r?\n|,/).map((v) => v.trim()).filter(Boolean),
      spki: form.protocol === 'DoT' ? form.spki.trim() : '',
      format: form.protocol === 'DoH' ? form.format.trim() : ''
    };
  }

  async function saveEditor() {
    saving = true; error = ''; success = '';
    try {
      const payload = payloadFromForm();
      if (editing) {
        await request(`/resolvers/${encodeURIComponent(editing.id)}`, { method:'PATCH', body:JSON.stringify(payload) });
      } else {
        await request('/resolvers', { method:'POST', body:JSON.stringify(payload) });
      }
      editorOpen = false;
      success = L.saved;
      await loadAll(true);
    } catch (e) {
      error = e?.message || String(e);
    } finally { saving = false; }
  }

  async function resolverAction(resolver, action) {
    const prompts = { delete:L.confirmDelete, disable:L.confirmDisable, enable:L.confirmEnable };
    if (!confirm(prompts[action])) return;
    error = ''; success = '';
    try {
      if (action === 'delete') await request(`/resolvers/${encodeURIComponent(resolver.id)}`, { method:'DELETE', body:null });
      else await request(`/resolvers/${encodeURIComponent(resolver.id)}/${action}`, { method:'POST', body:'{}' });
      success = L.saved;
      await loadAll(true);
    } catch (e) { error = e?.message || String(e); }
  }

  function proxyName(proxy) {
    return proxy.display_name || (proxy.name === 'System' ? L.system : proxy.name);
  }

  function reportText() {
    const lines = [`RouterForge DNS`, new Date().toISOString(), ''];
    for (const proxy of info?.proxies || []) {
      lines.push(`[${proxyName(proxy)}] requests=${proxy.stat?.total_requests || 0} sent=${proxy.stat?.proxy_requests_sent || 0} cache=${proxy.stat?.cache_hit_ratio || 0}`);
      for (const u of proxy.upstreams || []) {
        lines.push(`  ${u.protocol} ${u.address}:${u.port} sni=${u.sni || '-'} domain=${u.domain || '*'} sent=${u.r_sent || 0} answers=${u.a_rcvd || 0} avg=${u.avg_resp || '-'}`);
      }
    }
    lines.push('', `Resolvers: ${JSON.stringify(resolverState, null, 2)}`);
    return lines.join('\n');
  }

  async function copyReport() {
    const text = reportText();
    try { await navigator.clipboard.writeText(text); }
    catch {
      const area = document.createElement('textarea'); area.value = text; document.body.appendChild(area); area.select(); document.execCommand('copy'); area.remove();
    }
    success = L.copied;
  }

  function saveReport() {
    const blob = new Blob([reportText()], { type:'text/plain;charset=utf-8' });
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = href; a.download = `routerforge-dns-${new Date().toISOString().replace(/[:.]/g,'-')}.txt`; a.click();
    setTimeout(() => URL.revokeObjectURL(href), 1000);
  }

  // The DNS UI is rendered in a same-origin iframe. Core's responsive scale is
  // calculated on the full browser viewport, while the iframe viewport is
  // narrower because the persistent rail has already been subtracted. Copy the
  // computed shell tokens from the parent so both surfaces stay pixel-aligned.
  function syncCoreVisualTokens() {
    try {
      if (window.parent === window) return;
      const parentRoot = window.parent.document.documentElement;
      const source = window.parent.getComputedStyle(parentRoot);
      const target = document.documentElement.style;
      const tokens = [
        '--page-pad','--ui-body','--ui-small','--ui-xs','--ui-micro','--ui-title',
        '--ui-panel-title','--ui-control-h','--concept-bg','--concept-surface',
        '--concept-panel','--concept-panel-2','--concept-hover','--concept-border',
        '--concept-border-soft','--accent','--good','--warn','--bad','--font-sans','--font-mono'
      ];
      for (const token of tokens) {
        const value = source.getPropertyValue(token).trim();
        if (value) target.setProperty(token, value);
      }
    } catch {}
  }
  onMount(() => {
    syncCoreVisualTokens();
    window.addEventListener('resize', syncCoreVisualTokens);
    loadAll();
    refreshTimer = setInterval(() => loadAll(true), 5000);
    return () => {
      clearInterval(refreshTimer);
      window.removeEventListener('resize', syncCoreVisualTokens);
    };
  });
</script>

<div class="dns-module page">
  <div class="page-head">
    <div>

      <h1>{L.title}</h1>
      <p>{L.subtitle}</p>
    </div>
    <div class="toolbar-actions">
      <button class="action" type="button" onclick={() => loadAll()}>{L.refresh}</button>
      <button class="action primary" type="button" onclick={openAdd}>+ {L.add}</button>
    </div>
  </div>

  <div class="module-tabs">
    {#each [['overview',L.overview],['resolvers',L.resolvers],['rules',L.rules],['traffic',L.traffic],['diagnostics',L.diagnostics]] as item}
      <button class:active={tab === item[0]} type="button" onclick={() => setTab(item[0])}>{item[1]}</button>
    {/each}
  </div>

  {#if error}<div class="error-box"><strong>{L.moduleError}:</strong> {error}</div>{/if}
  {#if success}<div class="success-box">{success}</div>{/if}

  {#if loading}
    <div class="empty-box">…</div>
  {:else if tab === 'overview'}
    <section class="metric-grid four">
      <div class="metric-card"><span>{L.activeResolvers}</span><strong>{resolverState.active_count || 0}</strong><small>{L.native}</small></div>
      <div class="metric-card"><span>{L.disabledResolvers}</span><strong>{resolverState.disabled_count || 0}</strong><small>RouterForge metadata</small></div>
      <div class="metric-card"><span>{L.requests}</span><strong>{totalRequests}</strong><small>{L.sent}: {totalSent}</small></div>
      <div class="metric-card"><span>{L.cache}</span><strong>{cacheHits}</strong><small>{L.physical}: {resolverState.physical_entries || 0} · {L.dotSlots}: {dotSlotsUsed}/{dotSlotLimit || '—'}</small></div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><strong>{L.native}</strong><span>{L.nativeHint}</span></div></div>
      <p class="resolver-meta">{L.rollbackNote}</p>
    </section>

    <section class="panel">
      <div class="panel-head"><div><strong>{L.resolvers}</strong><span>{activeResolvers.length} + {disabledResolvers.length} + {dynamicResolvers.length}</span></div></div>
      <div class="resolver-grid">
        {#each resolvers.slice(0,6) as resolver (resolver.id)}
          <div class="resolver-card">
            <div class="resolver-head"><div><h3>{resolver.name}</h3><div class="resolver-meta mono">{resolver.protocol} · {endpoint(resolver)}</div></div><span class="state-pill {statusClass(resolver)}">{statusText(resolver)}</span></div>
            <div class="scope-list">{#if scopes(resolver).length}{#each scopes(resolver) as d}<span class="scope-chip">{displayDomain(d)}</span>{/each}{:else}<span class="scope-chip">{L.global}</span>{/if}</div>
          </div>
        {/each}
      </div>
    </section>

  {:else if tab === 'resolvers'}
    <section class="panel resolver-panel">
      <div class="panel-head">
        <div><strong>{L.resolvers}</strong><span>{L.nativeHint}</span></div>
        <span class="state-pill info">{L.dotSlots}: {dotSlotsUsed}/{dotSlotLimit || '—'}</span>
      </div>
      {#if !resolvers.length}<div class="empty-box">{L.noResolvers}</div>{/if}
      <div class="resolver-grid resolver-grid-body">
      {#each resolvers as resolver (resolver.id)}
        <article class="resolver-card">
          <div class="resolver-head">
            <div><h3>{resolver.name}</h3><div class="resolver-meta mono">{resolver.protocol} · {endpoint(resolver)}</div></div>
            <span class="state-pill {statusClass(resolver)}">{statusText(resolver)}</span>
          </div>
          {#if resolver.sni}<div class="detail-grid"><div class="detail-item"><span>SNI</span><strong class="mono">{resolver.sni}</strong></div>{#if resolver.interface}<div class="detail-item"><span>{L.iface}</span><strong>{resolver.interface}</strong></div>{/if}</div>{/if}
          <div class="resolver-meta">{L.scope}</div>
          <div class="scope-list">{#if scopes(resolver).length}{#each scopes(resolver) as d}<span class="scope-chip">{displayDomain(d)}</span>{/each}{:else}<span class="scope-chip">{L.global}</span>{/if}</div>
          <div class="resolver-meta">{L.physicalCount}: {resolver.physical_count || 1}{#if resolver.dynamic && resolver.service} · {resolver.service}{/if}</div>
          <div class="resolver-actions">
            <button class="action" type="button" disabled={resolver.dynamic} onclick={() => openEdit(resolver)}>{L.edit}</button>
            {#if resolver.disabled}
              <button class="action primary" type="button" onclick={() => resolverAction(resolver,'enable')}>{L.enable}</button>
            {:else if !resolver.dynamic}
              <button class="action" type="button" onclick={() => resolverAction(resolver,'disable')}>{L.disable}</button>
            {/if}
            <button class="action danger" type="button" disabled={resolver.dynamic} onclick={() => resolverAction(resolver,'delete')}>{L.remove}</button>
          </div>
          {#if resolver.dynamic}<div class="resolver-meta">{L.readOnly}</div>{/if}
        </article>
      {/each}
      </div>
    </section>

  {:else if tab === 'rules'}
    <section class="panel">
      <div class="panel-head"><div><strong>{L.scope}</strong><span>{L.nativeHint}</span></div></div>
      <div class="table-wrap"><table><thead><tr><th>{L.resolvers}</th><th>{L.protocol}</th><th>{L.scope}</th><th>{L.physicalCount}</th></tr></thead><tbody>
        {#each activeResolvers as resolver}
          <tr><td>{resolver.name}</td><td class="mono">{resolver.protocol}</td><td>{scopes(resolver).length ? scopes(resolver).map(displayDomain).join(', ') : L.global}</td><td>{resolver.physical_count || 1}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><strong>{L.localRecords}</strong><span>{info?.static_records?.length || 0}</span></div></div>
      {#if info?.static_records?.length}<div class="table-wrap"><table><thead><tr><th>Host</th><th>Type</th><th>Value</th>{#if advanced}<th>Flag</th>{/if}</tr></thead><tbody>{#each info.static_records as r}<tr><td class="mono">{r.host}</td><td>{r.type}</td><td class="mono">{r.value}</td>{#if advanced}<td>{r.flag}</td>{/if}</tr>{/each}</tbody></table></div>{:else}<div class="empty-box">{L.noData}</div>{/if}
    </section>
    <section class="panel">
      <div class="panel-head"><div><strong>{L.rebind}</strong><span>{info?.rebind?.enabled ? L.rebindOn : L.rebindOff}</span></div></div>
      <div class="detail-grid"><div class="detail-item"><span>{L.protectedNets}</span><strong>{(info?.rebind?.nets || []).join(', ') || '—'}</strong></div><div class="detail-item"><span>{L.exclusions}</span><strong>{(info?.rebind?.excludes || []).join(', ') || '—'}</strong></div></div>
    </section>

  {:else if tab === 'traffic'}
    <section class="metric-grid four">
      <div class="metric-card"><span>{L.requests}</span><strong>{totalRequests}</strong><small>{L.sent}: {totalSent}</small></div>
      <div class="metric-card"><span>{L.cache}</span><strong>{cacheHits}</strong><small>{(totalRequests ? cacheHits/totalRequests*100 : 0).toFixed(1)}%</small></div>
      <div class="metric-card"><span>{L.topDomains}</span><strong>{topDomains.length}</strong><small>snapshot</small></div>
      <div class="metric-card"><span>{L.recentFlow}</span><strong>{flow.length}</strong><small>snapshot</small></div>
    </section>
    <section class="panel"><div class="panel-head"><div><strong>{L.topDomains}</strong></div></div>{#if topDomains.length}<div class="table-wrap"><table><tbody>{#each topDomains.slice(0,50) as row}<tr><td class="mono">{row.domain || row.name || '—'}</td><td>{row.count ?? row.requests ?? '—'}</td></tr>{/each}</tbody></table></div>{:else}<div class="empty-box">{L.noData}</div>{/if}</section>
    <section class="panel"><div class="panel-head"><div><strong>{L.recentFlow}</strong></div></div>{#if flow.length}<div class="table-wrap"><table><thead><tr><th>Time</th><th>Domain</th><th>{L.status}</th><th>{L.resolvers}</th></tr></thead><tbody>{#each flow.slice(0,100) as row}<tr><td class="mono">{row.time || row.ts || '—'}</td><td class="mono">{row.domain || '—'}</td><td>{row.outcome || row.status || '—'}</td><td>{row.upstream || row.resolver || '—'}</td></tr>{/each}</tbody></table></div>{:else}<div class="empty-box">{L.noData}</div>{/if}</section>

  {:else if tab === 'diagnostics'}
    <div class="toolbar"><div><strong>{L.diagnostics}</strong><div class="resolver-meta">ndnproxy / Keenetic</div></div><div class="toolbar-actions"><button class="action" type="button" onclick={() => advanced = !advanced}>{L.advanced}: {advanced ? 'ON':'OFF'}</button><button class="action" type="button" onclick={copyReport}>{L.copy}</button><button class="action" type="button" onclick={saveReport}>{L.saveReport}</button></div></div>
    {#each info?.proxies || [] as proxy}
      <section class="panel">
        <div class="panel-head"><div><strong>{proxyName(proxy)}</strong><span class="mono">{proxy.name} · TCP {proxy.tcp_port} / UDP {proxy.udp_port}</span></div></div>
        <div class="detail-grid"><div class="detail-item"><span>{L.requests}</span><strong>{proxy.stat?.total_requests || 0}</strong></div><div class="detail-item"><span>{L.sent}</span><strong>{proxy.stat?.proxy_requests_sent || 0}</strong></div><div class="detail-item"><span>{L.cache}</span><strong>{((proxy.stat?.cache_hit_ratio || 0)*100).toFixed(1)}%</strong></div><div class="detail-item"><span>{L.memory}</span><strong>{proxy.stat?.memory || '—'}</strong></div></div>
        <div class="table-wrap"><table><thead><tr><th>{L.resolvers}</th><th>{L.scope}</th><th>{L.sent}</th><th>{L.answers}</th><th>NX</th><th>{L.median}</th><th>{L.avg}</th>{#if advanced}<th>{L.rank}</th><th>Local</th>{/if}</tr></thead><tbody>
          {#each proxy.upstreams || [] as u}<tr><td><strong>{u.name}</strong><div class="resolver-meta mono">{u.protocol} · {u.address}:{u.port}{u.sni ? ` · ${u.sni}` : ''}</div></td><td>{u.domain ? displayDomain(u.domain) : L.global}</td><td>{u.r_sent}</td><td>{u.a_rcvd}</td><td>{u.nx_rcvd}</td><td>{u.med_resp || '—'}</td><td>{u.avg_resp || '—'}</td>{#if advanced}<td>{u.rank}</td><td>{u.local_port}</td>{/if}</tr>{/each}
        </tbody></table></div>
      </section>
    {/each}
  {/if}

  {#if editorOpen}
    <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) editorOpen = false; }}>
      <div class="modal" role="dialog" aria-modal="true">
        <h2>{editing ? L.editTitle : L.addTitle}</h2>
        <div class="form-grid">
          <label>{L.protocol}<select class="protocol-select" bind:value={form.protocol} onchange={() => { if (form.protocol === 'DoT' && !form.port) form.port=853; if (form.protocol === 'DNS') form.port=53; if (form.protocol === 'DoH') form.port=443; }}><option>DNS</option><option>DoT</option><option>DoH</option></select></label>
          <label>{L.port}<input type="number" min="1" max="65535" bind:value={form.port}/></label>
          {#if form.protocol === 'DoH'}<label class="span-2">{L.uri}<input class="mono" placeholder="https://dns.example/dns-query" bind:value={form.uri}/></label>{:else}<label class="span-2">{L.address}<input class="mono" placeholder={form.protocol === 'DNS' ? '1.1.1.1' : '1.1.1.1 / dns.example'} bind:value={form.address}/></label>{/if}
          {#if form.protocol === 'DoT'}<label>{L.sni}<input class="mono" placeholder="cloudflare-dns.com" bind:value={form.sni}/></label><label>{L.spki}<input class="mono" bind:value={form.spki}/></label>{/if}
          {#if form.protocol === 'DoH'}<label>{L.format}<input class="mono" bind:value={form.format}/></label>{/if}
          <label>{L.iface}<input class="mono" placeholder="ISP" bind:value={form.interface}/></label>
          <label class="span-2">{L.domains}<textarea class="mono" placeholder={'ru\nsu\nxn--p1ai'} bind:value={form.domains}></textarea><span class:slot-warning={dotCapacityExceeded}>{L.domainsHint}{#if form.protocol === 'DoT' && dotSlotLimit} · {L.dotSlots}: {dotSlotsUsed}/{dotSlotLimit} → {L.afterSave}: {projectedDoTSlots}/{dotSlotLimit}{#if dotCapacityExceeded} · {L.limitExceeded}{/if}{/if}</span></label>
        </div>
        <div class="modal-actions"><button class="action" type="button" onclick={() => editorOpen = false}>{L.cancel}</button><button class="action primary" type="button" disabled={saving || dotCapacityExceeded} onclick={saveEditor}>{saving ? '…' : L.save}</button></div>
      </div>
    </div>
  {/if}
</div>
