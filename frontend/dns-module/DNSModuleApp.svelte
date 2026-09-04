<script>
  import { onMount } from 'svelte';

  const API = '/api/modules/dns';
  const query = new URLSearchParams(window.location.search);
  const topTabs = ['overview','resolvers','rules','traffic','diagnostics'];
  const trafficTabs = ['traffic','flow','devices','interfaces','domains'];
  const diagnosticTabs = ['journal','connections','dns','system'];

  let locale = query.get('locale') === 'en' ? 'en' : 'ru';
  let tab = topTabs.includes(query.get('view')) ? query.get('view') : 'overview';
  let loading = true;
  let error = '';
  let success = '';
  let info = null;
  let resolverState = { resolvers: [], active_count: 0, disabled_count: 0, dynamic_count: 0, physical_entries: 0, dot_physical_entries: 0, dot_physical_limit: 0, doh_physical_entries: 0, secure_physical_entries: 0, secure_physical_limit: 0, plain_dns_domain_limit: 16 };
  let snapshot = {};
  let plain = { resolvers: [], recent: [], pending: 0, mode: '', note: '' };
  let history = { minutes: 5, points: [] };
  let fallbacks = { minutes: 60, edges: [] };
  let clients = [];
  let interfaces = [];
  let systemInfo = {};
  let errorBursts = { minutes: 60, bursts: [] };

  let editorOpen = false;
  let editing = null;
  let saving = false;
  let form = blankForm();
  let advanced = false;
  let refreshTimer = null;
  let refreshBusy = false;

  let overviewSearch = '';
  let overviewActiveOnly = false;

  let resolverSearch = '';
  let resolverProtocol = 'all';
  let resolverStatus = 'all';
  let resolverView = (() => {
    try { return localStorage.getItem('routerforge:dns:resolver-view') === 'cards' ? 'cards' : 'detail'; }
    catch { return 'detail'; }
  })();
  let selectedResolverId = '';

  let rulesProfile = 'all';
  let rulesMinutes = 60;

  let trafficTab = 'traffic';
  let historyMinutes = 5;
  let flowSearch = '';
  let flowProfile = 'all';
  let fallbackOnly = false;
  let flowPaused = false;
  let frozenFlow = [];
  let clientSearch = '';
  let selectedIP = '';
  let clientDetail = null;
  let clientPaused = false;
  let frozenClientEvents = [];
  let clientFlowSearch = '';
  let clientOutcome = 'all';

  let diagnosticsTab = 'journal';
  let diagKind = 'all';
  let diagSearch = '';
  let diagPaused = false;
  let frozenErrors = [];
  let frozenBursts = [];
  let diagMinutes = 60;

  const strings = {
    ru: {
      title:'DNS', subtitle:'Резолверы Keenetic, маршрутизация, трафик и диагностика.',
      overview:'Обзор', resolvers:'Резолверы', rules:'Правила', traffic:'Трафик', diagnostics:'Диагностика',
      refresh:'Обновить', add:'Добавить DNS', edit:'Настроить', disable:'Отключить', enable:'Включить', remove:'Удалить',
      active:'АКТИВЕН', disabled:'ОТКЛЮЧЕН', dynamic:'ДИНАМИЧЕСКИЙ', global:'Все домены', scope:'Область запросов',
      requests:'Запросы', responses:'Ответы', sent:'Передано', avg:'Среднее', median:'Медиана', errors:'Ошибки', cache:'Ответы из кэша',
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
      secureSlots:'DoT/DoH-слоты', dnsDomainLimit:'Домены DNS', afterSave:'После сохранения', limitExceeded:'Превышен лимит Keenetic',
      mainDns:'Основной DNS', protectedDns:'Защищённые DNS', problems:'Проблемы', quality:'Качество', latency:'Latency', fallback:'Fallback',
      searchDns:'Поиск DNS…', activeOnly:'Активные', currentUse:'используется сейчас', detected:'Доступен', unavailable:'Недоступен', degraded:'Есть проблемы',
      trafficShare:'трафика', timeouts:'Таймауты', policyContexts:'Маршрутные DNS-контексты', route:'Маршрут', resolverCount:'Резолверы', markTable:'MARK / TABLE',
      bindings:'Доменные привязки', bindingsHint:'Нативные domain bindings активных резолверов Keenetic.', fallbackRoutes:'Fallback-маршруты', fallbackHint:'Фактические переходы между DNS upstream за выбранный период.', keeneticProfiles:'Профили Keenetic', discoveryData:'Текущие discovery-данные', triggers:'Срабатывания', from:'Откуда', to:'Куда', allProfiles:'Все профили',
      trafficGraph:'DNS traffic', buckets:'интервалов', flow:'DNS Flow', devices:'Устройства', interfaces:'Интерфейсы', domainsTab:'Домены',
      domainOrDevice:'Домен или устройство…', fallbackOnly:'Только fallback', pause:'Пауза', continue:'Продолжить', live:'LIVE', frozen:'ЗАМОРОЖЕНО', noMatching:'Ничего не найдено.',
      device:'Устройство', type:'Тип', profile:'Профиль', allDevices:'Все устройства', activeDevice:'Активен', offline:'Не в сети', clientState:'DNS состояние клиента', policyRoute:'Маршрут Policy', systemDefault:'Системный маршрут по умолчанию', noPolicyRoute:'Специальный маршрут Policy не обнаружен.',
      forwarded:'Forwarded', cacheLocal:'Cache local', clientAvg:'Client avg', clientP95:'Client p95', outcome:'Результат', allOutcomes:'Все результаты',
      interfaceName:'Интерфейс', network:'Сеть', topClients:'Топ клиентов', qtypes:'Типы запросов',
      journal:'Журнал', connections:'Соединения', dnsInfo:'Сведения о DNS', systemTab:'Система', errorSummary:'Сводка ошибок', examples:'Примеры доменов', localProxies:'Локальные DNS proxy', target:'Назначение',
      discovery:'Discovery', counters:'Счётчики', lastDiscovery:'Последний discovery', discoveryError:'Ошибка discovery', captureError:'Ошибка capture', clientCaptureError:'Ошибка client capture', none:'нет', runtime:'Runtime', process:'Процесс',
      health:'Health', issues:'Проблемы', lastUse:'Последнее использование', noFallbacks:'Fallback-переходов за период нет.', mainRecent:'Последние запросы основного DNS',
      filterProtocol:'Протокол', filterStatus:'Статус', configured:'Настроенные', routeOk:'Есть default route', routeMissing:'Нет default route',
      diagnosticsRuntime:'Runtime diagnostics', stage:'Stage', assessment:'Assessment', healthError:'Health error'
    },
    en: {
      title:'DNS', subtitle:'Keenetic resolvers, routing, traffic and diagnostics.',
      overview:'Overview', resolvers:'Resolvers', rules:'Rules', traffic:'Traffic', diagnostics:'Diagnostics',
      refresh:'Refresh', add:'Add DNS', edit:'Configure', disable:'Disable', enable:'Enable', remove:'Delete',
      active:'ACTIVE', disabled:'DISABLED', dynamic:'DYNAMIC', global:'All domains', scope:'Query scope',
      requests:'Requests', responses:'Responses', sent:'Sent', avg:'Average', median:'Median', errors:'Errors', cache:'Cache answers',
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
      secureSlots:'DoT/DoH slots', dnsDomainLimit:'DNS domains', afterSave:'After save', limitExceeded:'Keenetic limit exceeded',
      mainDns:'Main DNS', protectedDns:'Protected DNS', problems:'Problems', quality:'Quality', latency:'Latency', fallback:'Fallback',
      searchDns:'Search DNS…', activeOnly:'Active', currentUse:'in use now', detected:'Detected', unavailable:'Unavailable', degraded:'Degraded',
      trafficShare:'traffic', timeouts:'Timeouts', policyContexts:'Policy DNS contexts', route:'Route', resolverCount:'Resolvers', markTable:'MARK / TABLE',
      bindings:'Domain bindings', bindingsHint:'Native domain bindings of active Keenetic resolvers.', fallbackRoutes:'Fallback routes', fallbackHint:'Observed transitions between DNS upstreams in the selected period.', keeneticProfiles:'Keenetic profiles', discoveryData:'Current discovery data', triggers:'Triggers', from:'From', to:'To', allProfiles:'All profiles',
      trafficGraph:'DNS traffic', buckets:'buckets', flow:'DNS Flow', devices:'Devices', interfaces:'Interfaces', domainsTab:'Domains',
      domainOrDevice:'Domain or device…', fallbackOnly:'Fallback only', pause:'Pause', continue:'Continue', live:'LIVE', frozen:'FROZEN', noMatching:'No matching events.',
      device:'Device', type:'Type', profile:'Profile', allDevices:'All devices', activeDevice:'Active', offline:'Offline', clientState:'Client DNS state', policyRoute:'Policy route', systemDefault:'System default route', noPolicyRoute:'No dedicated policy route discovered.',
      forwarded:'Forwarded', cacheLocal:'Cache local', clientAvg:'Client avg', clientP95:'Client p95', outcome:'Outcome', allOutcomes:'All outcomes',
      interfaceName:'Interface', network:'Network', topClients:'Top clients', qtypes:'Query types',
      journal:'Journal', connections:'Connections', dnsInfo:'DNS info', systemTab:'System', errorSummary:'Error summary', examples:'Example domains', localProxies:'Local DNS proxies', target:'Target',
      discovery:'Discovery', counters:'Counters', lastDiscovery:'Last discovery', discoveryError:'Discovery error', captureError:'Capture error', clientCaptureError:'Client capture error', none:'none', runtime:'Runtime', process:'Process',
      health:'Health', issues:'Issues', lastUse:'Last use', noFallbacks:'No fallback transitions for this period.', mainRecent:'Recent main DNS queries',
      filterProtocol:'Protocol', filterStatus:'Status', configured:'Configured', routeOk:'Default route available', routeMissing:'No default route',
      diagnosticsRuntime:'Runtime diagnostics', stage:'Stage', assessment:'Assessment', healthError:'Health error'
    }
  };

  $: L = strings[locale];
  $: resolvers = resolverState?.resolvers || [];
  $: activeResolvers = resolvers.filter((r) => !r.disabled && !r.dynamic);
  $: disabledResolvers = resolvers.filter((r) => r.disabled);
  $: dynamicResolvers = resolvers.filter((r) => r.dynamic);
  $: allUpstreams = snapshot?.upstreams || [];
  $: systemUpstreams = allUpstreams.filter((u) => u.profile === 'System');
  $: protectedUpstreams = systemUpstreams.length ? systemUpstreams : allUpstreams;
  $: policyUpstreams = allUpstreams.filter((u) => u.profile !== 'System');
  $: profiles = [...new Set(allUpstreams.map((u) => u.profile).filter(Boolean))].sort((a,b) => a.localeCompare(b, locale, { numeric:true }));
  $: plainResolvers = [...(plain?.resolvers || [])].sort((a,b) => String(a.address || '').localeCompare(String(b.address || ''), undefined, { numeric:true }));
  $: totalRequests = Number(snapshot?.total_requests || 0) + plainResolvers.reduce((sum,r) => sum + Number(r.requests || 0), 0);
  $: totalResponses = Number(snapshot?.total_responses || 0) + plainResolvers.reduce((sum,r) => sum + Number(r.responses || 0), 0);
  $: totalFallbacks = Number(snapshot?.total_fallbacks || 0);
  $: totalTimeouts = Number(snapshot?.total_timeouts || 0) + plainResolvers.reduce((sum,r) => sum + Number(r.timeouts || 0), 0);
  $: totalErrors = allUpstreams.reduce((sum,u) => sum + Number(u.servfail || 0) + Number(u.refused || 0) + Number(u.other_errors || 0) + Number(u.timeouts || 0), 0) + plainResolvers.reduce((sum,r) => sum + Number(r.errors || 0) + Number(r.timeouts || 0), 0);
  $: topDomains = snapshot?.top_domains || [];
  $: liveFlow = snapshot?.flow || [];
  $: dotSlotsUsed = Number(resolverState?.dot_physical_entries || 0);
  $: dohSlotsUsed = Number(resolverState?.doh_physical_entries || 0);
  $: secureSlotsUsed = Number(resolverState?.secure_physical_entries ?? (dotSlotsUsed + dohSlotsUsed));
  $: secureSlotLimit = Number(resolverState?.secure_physical_limit || resolverState?.dot_physical_limit || 0);
  $: plainDnsDomainLimit = Number(resolverState?.plain_dns_domain_limit || 16);
  $: formDomainCount = form.domains.split(/\r?\n|,/).map((v) => v.trim()).filter(Boolean).length;
  $: formPhysicalCount = formDomainCount || 1;
  $: editingActiveSecureSlots = editing && !editing.disabled && !editing.dynamic && ['DoT','DoH'].includes(editing.protocol) ? Number(editing.physical_count || 1) : 0;
  $: editorAffectsActive = !editing || (!editing.disabled && !editing.dynamic);
  $: formActiveSecureSlots = editorAffectsActive && ['DoT','DoH'].includes(form.protocol) ? formPhysicalCount : 0;
  $: projectedSecureSlots = Math.max(0, secureSlotsUsed - editingActiveSecureSlots + formActiveSecureSlots);
  $: secureCapacityExceeded = secureSlotLimit > 0 && projectedSecureSlots > secureSlotLimit;
  $: plainDomainExceeded = form.protocol === 'DNS' && formDomainCount > plainDnsDomainLimit;
  $: editorLimitExceeded = secureCapacityExceeded || plainDomainExceeded;

  $: filteredPlain = plainResolvers.filter((r) => {
    if (overviewActiveOnly && !plainRecentlyActive(r)) return false;
    const q = overviewSearch.trim().toLowerCase();
    return !q || `${r.name || ''} ${r.address || ''} ${r.source || ''} ${r.interface || ''} ${(r.domains || []).join(' ')}`.toLowerCase().includes(q);
  });
  $: filteredProtected = protectedUpstreams.filter((u) => {
    if (overviewActiveOnly && !u.active) return false;
    const q = overviewSearch.trim().toLowerCase();
    return !q || `${u.name || ''} ${u.target || ''} ${u.sni || ''} ${u.domain || ''}`.toLowerCase().includes(q);
  });
  $: serverCount = protectedUpstreams.length + plainResolvers.length;
  $: healthyCount = protectedUpstreams.filter((u) => u.health_status !== 'DOWN').length + plainResolvers.filter((r) => plainStatus(r).cls !== 'error').length;
  $: activeCount = protectedUpstreams.filter((u) => u.active).length + plainResolvers.filter(plainRecentlyActive).length;
  $: downCount = protectedUpstreams.filter((u) => u.health_status === 'DOWN').length + plainResolvers.filter((r) => plainStatus(r).cls === 'error').length;
  $: degradedCount = protectedUpstreams.filter((u) => u.health_status === 'DEGRADED').length + plainResolvers.filter((r) => plainStatus(r).cls === 'warn').length;

  $: filteredResolvers = resolvers.filter((r) => {
    const q = resolverSearch.trim().toLowerCase();
    const status = r.dynamic ? 'dynamic' : r.disabled ? 'disabled' : 'active';
    if (resolverProtocol !== 'all' && r.protocol !== resolverProtocol) return false;
    if (resolverStatus !== 'all' && status !== resolverStatus) return false;
    return !q || `${r.name || ''} ${r.protocol || ''} ${r.address || ''} ${r.uri || ''} ${r.sni || ''} ${(r.domains || []).join(' ')}`.toLowerCase().includes(q);
  });

  $: if (filteredResolvers.length && !filteredResolvers.some((r) => r.id === selectedResolverId)) selectedResolverId = filteredResolvers[0].id;
  $: if (!filteredResolvers.length && selectedResolverId) selectedResolverId = '';
  $: selectedResolver = filteredResolvers.find((r) => r.id === selectedResolverId) || null;
  $: selectedRuntime = selectedResolver ? buildResolverRuntime(selectedResolver, allUpstreams, plainResolvers) : null;
  $: selectedRecent = selectedResolver ? resolverRecentEvents(selectedResolver, selectedRuntime, liveFlow, plain?.recent || []) : [];

  $: ruleEdges = (fallbacks?.edges || []).filter((e) => rulesProfile === 'all' || e.from_profile === rulesProfile);
  $: ruleEdgeCount = ruleEdges.reduce((sum,e) => sum + Number(e.count || 0), 0);
  $: profileStats = profiles.map((name) => {
    const rows = allUpstreams.filter((u) => u.profile === name);
    return { name, count:rows.length, active:rows.filter((u) => u.active).length, requests:rows.reduce((sum,u) => sum + Number(u.requests || 0), 0), first:rows[0] || {} };
  });

  $: flowSource = flowPaused ? frozenFlow : liveFlow;
  $: flowRows = flowSource.slice().reverse().filter((x) => {
    const q = flowSearch.trim().toLowerCase();
    if (q && !`${x.domain || ''} ${x.client_name || ''} ${x.client_hostname || ''} ${x.client_ip || ''} ${x.upstream || ''}`.toLowerCase().includes(q)) return false;
    if (flowProfile !== 'all' && x.profile !== flowProfile) return false;
    if (fallbackOnly && !x.fallback) return false;
    return true;
  }).slice(0,500);
  $: filteredClients = clients.filter((c) => {
    const q = clientSearch.trim().toLowerCase();
    return !q || `${c.name || ''} ${c.hostname || ''} ${c.ip || ''} ${c.mac || ''} ${c.policy || ''} ${c.access || ''} ${c.ssid || ''}`.toLowerCase().includes(q);
  }).sort((a,b) => {
    if (Boolean(a.active) !== Boolean(b.active)) return a.active ? -1 : 1;
    return String(a.name || a.hostname || a.ip || '').localeCompare(String(b.name || b.hostname || b.ip || ''), locale, { numeric:true, sensitivity:'base' });
  });
  $: clientEventSource = clientPaused ? frozenClientEvents : (clientDetail?.events || []);
  $: clientEvents = clientEventSource.filter((e) => {
    const q = clientFlowSearch.trim().toLowerCase();
    if (q && !`${e.domain || ''} ${e.resolver || ''} ${e.rcode || ''}`.toLowerCase().includes(q)) return false;
    if (clientOutcome !== 'all' && e.outcome !== clientOutcome) return false;
    return true;
  });
  $: qtypeGroups = Object.entries(liveFlow.reduce((acc,row) => { const key = row.qtype || '—'; acc[key] = (acc[key] || 0) + 1; return acc; }, {})).sort((a,b) => b[1] - a[1]);

  $: diagErrorsSource = diagPaused ? frozenErrors : (snapshot?.errors || []);
  $: diagRows = diagErrorsSource.slice().reverse().filter((x) => {
    if (diagKind !== 'all' && x.kind !== diagKind) return false;
    const q = diagSearch.trim().toLowerCase();
    return !q || `${x.kind || ''} ${x.profile || ''} ${x.upstream || ''} ${x.domain || ''} ${x.message || ''}`.toLowerCase().includes(q);
  });
  $: diagBursts = (diagPaused ? frozenBursts : (errorBursts?.bursts || snapshot?.error_bursts || [])).slice(0,20);

  $: historyPoints = history?.points || [];
  $: historyMax = Math.max(1, ...historyPoints.map((p) => Number(p.requests || 0)));
  $: chartCoords = historyPoints.map((p,i) => ({
    x: historyPoints.length > 1 ? i * 1000 / (historyPoints.length - 1) : 0,
    y: 190 - Number(p.requests || 0) / historyMax * 160,
    requests:Number(p.requests || 0), fallbacks:Number(p.fallbacks || 0), errors:Number(p.errors || 0), time:p.time
  }));
  $: chartLine = chartCoords.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');
  $: chartArea = chartCoords.length ? `0,190 ${chartLine} 1000,190` : '';

  function blankForm() {
    return { protocol:'DoT', address:'', uri:'', port:853, sni:'', interface:'', domains:'', spki:'', format:'' };
  }

  function num(value) { return Number(value || 0); }
  function fmtInt(value) { return new Intl.NumberFormat(locale === 'en' ? 'en-GB' : 'ru-RU').format(num(value)); }
  function fmtPct(value, digits = 1) { return `${num(value).toFixed(digits)}%`; }
  function fmtMs(value) { const n = num(value); return n ? `${n < 10 ? n.toFixed(1) : n.toFixed(0)} ms` : '—'; }
  function timeOnly(value) { const d = new Date(value); return Number.isNaN(d.getTime()) ? '—' : d.toLocaleTimeString(locale === 'en' ? 'en-GB' : 'ru-RU'); }
  function fmtAgo(value) {
    if (!value || String(value).startsWith('0001-')) return '—';
    const ms = Date.now() - new Date(value).getTime();
    if (!Number.isFinite(ms)) return '—';
    if (ms < 60_000) return locale === 'en' ? 'now' : 'сейчас';
    const m = Math.floor(ms / 60_000);
    if (m < 60) return locale === 'en' ? `${m}m ago` : `${m} мин назад`;
    const h = Math.floor(m / 60);
    if (h < 24) return locale === 'en' ? `${h}h ago` : `${h} ч назад`;
    const d = Math.floor(h / 24);
    return locale === 'en' ? `${d}d ago` : `${d} дн назад`;
  }
  function plainRecentlyActive(r = {}) {
    const iso = String(r.last_request || '');
    if (!iso || iso.startsWith('0001-')) return false;
    const ts = new Date(iso).getTime();
    return Number.isFinite(ts) && Date.now() - ts < 5 * 60 * 1000;
  }
  function plainStatus(r = {}) {
    const req = num(r.requests), res = num(r.responses), err = num(r.errors), timeout = num(r.timeouts);
    if (req > 0 && res === 0 && timeout > 0) return { cls:'error', label:L.unavailable };
    if (req > 0 && (err > 0 || timeout > 0)) return { cls:'warn', label:L.degraded };
    if (plainRecentlyActive(r)) return { cls:'good', label:L.active };
    return { cls:'neutral', label:L.detected };
  }
  function upstreamStatus(u = {}) {
    if (u.health_status === 'DOWN') return { cls:'error', label:'DOWN' };
    if (u.health_status === 'DEGRADED') return { cls:'warn', label:'DEGRADED' };
    if (u.active) return { cls:'good', label:L.active };
    if (u.health_status === 'UP') return { cls:'neutral', label:'UP' };
    return { cls:'neutral', label:u.health_status || L.detected };
  }
  function qualityClass(w = {}) {
    const q = num(w.quality_pct ?? 100);
    if (w.quality_status === 'BAD' || q < 90) return 'bad-text';
    if (w.quality_status === 'WARN' || q < 98) return 'warn-text';
    return 'good-text';
  }
  function latencyClass(value) { const n = num(value); return n >= 500 ? 'bad' : n >= 200 ? 'warn' : 'good'; }
  function share(value) { return totalRequests ? num(value) / totalRequests * 100 : 0; }
  function scopes(resolver) { return resolver?.domains?.length ? resolver.domains : []; }
  function displayDomain(domain) { if (domain === 'xn--p1ai') return '.рф'; return domain ? `.${domain}` : L.global; }
  function endpoint(resolver) {
    if (resolver.protocol === 'DoH') return resolver.uri || '—';
    const port = resolver.port || (resolver.protocol === 'DoT' ? 853 : 53);
    return `${resolver.address || '—'}:${port}`;
  }
  function resolverStatusClass(resolver) { if (resolver.disabled) return 'warn'; if (resolver.dynamic) return 'neutral'; return 'good'; }
  function resolverStatusText(resolver) { if (resolver.disabled) return L.disabled; if (resolver.dynamic) return L.dynamic; return L.active; }
  function resolverScopeSummary(resolver) {
    const count = scopes(resolver).length;
    if (!count) return L.global;
    return locale === 'en' ? `${count} domain${count === 1 ? '' : 's'}` : `${count} ${count === 1 ? 'домен' : count > 1 && count < 5 ? 'домена' : 'доменов'}`;
  }
  function setResolverView(value) {
    if (!['detail','cards'].includes(value)) return;
    resolverView = value;
    try { localStorage.setItem('routerforge:dns:resolver-view', value); } catch {}
    setTimeout(syncFrameHeight, 0);
  }
  function selectResolver(id) { selectedResolverId = id; }
  function normHost(value) { return String(value || '').trim().toLowerCase().replace(/^\[|\]$/g, '').replace(/\.$/, ''); }
  function normURL(value) {
    const raw = String(value || '').trim();
    if (!raw) return '';
    try {
      const u = new URL(raw);
      const pathname = u.pathname === '/' ? '' : u.pathname.replace(/\/+$/, '');
      return `${u.protocol.toLowerCase()}//${u.host.toLowerCase()}${pathname}${u.search}`;
    } catch { return raw.toLowerCase().replace(/\/+$/, ''); }
  }
  function resolverUpstreamRows(resolver, rows = []) {
    if (!resolver || resolver.protocol === 'DNS') return [];
    const protocol = String(resolver.protocol || '').toLowerCase();
    if (protocol === 'doh') {
      const uri = normURL(resolver.uri);
      return rows.filter((u) => String(u.protocol || '').toLowerCase() === protocol && normURL(u.target) === uri);
    }
    const address = normHost(resolver.address);
    const sni = normHost(resolver.sni);
    return rows.filter((u) => {
      if (String(u.protocol || '').toLowerCase() !== protocol || normHost(u.target) !== address) return false;
      const upstreamSNI = normHost(u.sni);
      return sni ? upstreamSNI === sni : !upstreamSNI;
    });
  }
  function resolverPlainRows(resolver, rows = []) {
    if (!resolver || resolver.protocol !== 'DNS') return [];
    const address = normHost(resolver.address);
    const port = num(resolver.port || 53);
    return rows.filter((r) => normHost(r.address) === address && num(r.port || 53) === port);
  }
  function latestISO(rows = [], key = 'last_request') {
    let latest = '';
    let latestMS = 0;
    for (const row of rows) {
      const value = String(row?.[key] || '');
      const ms = new Date(value).getTime();
      if (Number.isFinite(ms) && ms > latestMS) { latestMS = ms; latest = value; }
    }
    return latest;
  }
  function aggregateWindow(rows = [], key = 'stats_5m') {
    const out = { requests:0, responses:0, late_responses:0, pending:0, success:0, errors:0, timeouts:0, fallbacks:0, avg_latency_ms:0, p95_latency_ms:0, max_latency_ms:0, quality_pct:100, fallback_pct:0, quality_status:'OK' };
    let latencyWeighted = 0;
    let latencyWeight = 0;
    for (const row of rows) {
      const w = row?.[key] || {};
      out.requests += num(w.requests);
      out.responses += num(w.responses);
      out.late_responses += num(w.late_responses);
      out.pending += num(w.pending);
      out.success += num(w.success);
      out.errors += num(w.errors);
      out.timeouts += num(w.timeouts);
      out.fallbacks += num(w.fallbacks);
      out.p95_latency_ms = Math.max(out.p95_latency_ms, num(w.p95_latency_ms));
      out.max_latency_ms = Math.max(out.max_latency_ms, num(w.max_latency_ms));
      const weight = num(w.responses) || num(w.requests);
      if (weight > 0 && num(w.avg_latency_ms) > 0) { latencyWeighted += num(w.avg_latency_ms) * weight; latencyWeight += weight; }
    }
    if (latencyWeight) out.avg_latency_ms = latencyWeighted / latencyWeight;
    const attempts = out.success + out.errors + out.timeouts;
    if (attempts > 0) out.quality_pct = out.success / attempts * 100;
    if (out.requests > 0) out.fallback_pct = out.fallbacks / out.requests * 100;
    out.quality_status = out.quality_pct < 90 ? 'BAD' : out.quality_pct < 98 ? 'WARN' : 'OK';
    return out;
  }
  function aggregatePlain(rows = []) {
    const out = { requests:0, responses:0, errors:0, timeouts:0, nxdomain:0, fallbacks:0, avg_latency_ms:0, p95_latency_ms:0, quality_pct:100, fallback_pct:0 };
    let latencyWeighted = 0;
    let latencyWeight = 0;
    for (const row of rows) {
      out.requests += num(row.requests);
      out.responses += num(row.responses);
      out.errors += num(row.errors);
      out.timeouts += num(row.timeouts);
      out.nxdomain += num(row.nxdomain);
      out.p95_latency_ms = Math.max(out.p95_latency_ms, num(row.p95_latency_ms));
      const weight = num(row.responses) || num(row.requests);
      if (weight > 0 && num(row.avg_latency_ms) > 0) { latencyWeighted += num(row.avg_latency_ms) * weight; latencyWeight += weight; }
    }
    if (latencyWeight) out.avg_latency_ms = latencyWeighted / latencyWeight;
    if (out.requests > 0) out.quality_pct = Math.min(100, out.responses / out.requests * 100);
    return out;
  }
  function runtimeStateForRows(rows = []) {
    if (!rows.length) return { cls:'neutral', label:locale === 'en' ? 'NO RUNTIME' : 'НЕТ RUNTIME' };
    if (rows.some((u) => u.health_status === 'DOWN')) return { cls:'error', label:'DOWN' };
    if (rows.some((u) => u.health_status === 'DEGRADED')) return { cls:'warn', label:'DEGRADED' };
    if (rows.some((u) => u.active)) return { cls:'good', label:L.active };
    if (rows.some((u) => u.health_status === 'UP')) return { cls:'neutral', label:'UP' };
    return { cls:'neutral', label:L.detected };
  }
  function pickDiagnostic(rows = []) {
    const withDiagnostics = rows.filter((u) => u?.diagnostic?.ran);
    if (!withDiagnostics.length) return null;
    return withDiagnostics.sort((a,b) => {
      const rank = (u) => u.diagnostic?.status === 'FAIL' ? 3 : u.health_status === 'DOWN' ? 2 : u.health_status === 'DEGRADED' ? 1 : 0;
      return rank(b) - rank(a);
    })[0]?.diagnostic || null;
  }
  function buildResolverRuntime(resolver, upstreamRows = [], plainRows = []) {
    if (!resolver) return null;
    if (resolver.protocol === 'DNS') {
      const rows = resolverPlainRows(resolver, plainRows);
      const summary = aggregatePlain(rows);
      const active = rows.some(plainRecentlyActive);
      let state = { cls:'neutral', label:rows.length ? L.detected : (locale === 'en' ? 'NO RUNTIME' : 'НЕТ RUNTIME') };
      if (rows.length && summary.requests > 0 && summary.responses === 0 && summary.timeouts > 0) state = { cls:'error', label:L.unavailable };
      else if (rows.length && (summary.errors > 0 || summary.timeouts > 0)) state = { cls:'warn', label:L.degraded };
      else if (active) state = { cls:'good', label:L.active };
      return { kind:'plain', rows, summary, state, windows:null, diagnostic:null, ports:[num(resolver.port || 53)], last_request:latestISO(rows) };
    }
    const rows = resolverUpstreamRows(resolver, upstreamRows);
    const windows = {
      stats_5m:aggregateWindow(rows, 'stats_5m'),
      stats_1h:aggregateWindow(rows, 'stats_1h'),
      stats_24h:aggregateWindow(rows, 'stats_24h')
    };
    return {
      kind:'secure', rows, summary:windows.stats_5m, state:runtimeStateForRows(rows), windows,
      diagnostic:pickDiagnostic(rows),
      ports:[...new Set(rows.map((u) => num(u.port)).filter(Boolean))].sort((a,b) => a - b),
      profilePorts:[...new Set(rows.map((u) => num(u.profile_dns_port)).filter(Boolean))].sort((a,b) => a - b),
      last_request:latestISO(rows)
    };
  }
  function resolverRecentEvents(resolver, runtime, flowRows = [], plainRecent = []) {
    if (!resolver || !runtime) return [];
    if (resolver.protocol === 'DNS') {
      const address = normHost(resolver.address);
      const port = num(resolver.port || 53);
      return plainRecent.filter((e) => normHost(e.resolver) === address && num(e.port || 53) === port).slice(0, 40).map((e) => ({
        time:e.time, domain:e.domain, qtype:e.qtype, fallback:false, status:e.status || e.rcode || '—', rcode:e.rcode || ''
      }));
    }
    const ports = new Set(runtime.ports || []);
    return flowRows.slice().reverse().filter((e) => ports.has(num(e.port))).slice(0, 40).map((e) => ({
      time:e.time, domain:e.domain, qtype:e.qtype, fallback:Boolean(e.fallback), status:e.fallback ? 'FALLBACK' : 'OK', rcode:''
    }));
  }
  function runtimeWindowLabel(key) {
    if (key === 'stats_5m') return locale === 'en' ? '5 minutes' : '5 минут';
    if (key === 'stats_1h') return locale === 'en' ? '1 hour' : '1 час';
    return locale === 'en' ? '24 hours' : '24 часа';
  }
  function proxyName(proxy) { return proxy.display_name || (proxy.name === 'System' ? L.system : proxy.name); }
  function outcomeClass(value) { return value === 'FORWARDED' ? 'good' : value === 'ERROR' || value === 'CLIENT_TIMEOUT' ? 'error' : value === 'CACHE_LOCAL' ? 'info' : 'neutral'; }

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
    if (!quiet) error = '';
    try {
      const [nextInfo, nextResolvers, nextSnapshot] = await Promise.all([request('/info'), request('/resolvers'), request('/snapshot')]);
      info = nextInfo;
      resolverState = nextResolvers;
      snapshot = nextSnapshot;
      try { plain = await request('/plain-dns?limit=160'); } catch {}
    } catch (e) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  async function loadHistory() { try { history = await request(`/history?minutes=${historyMinutes}`); } catch (e) { error = e?.message || String(e); } }
  async function loadFallbacks() { try { fallbacks = await request(`/fallbacks?minutes=${rulesMinutes}`); } catch (e) { error = e?.message || String(e); } }
  async function loadClients() { try { clients = (await request('/clients')).clients || []; } catch (e) { error = e?.message || String(e); } }
  async function loadInterfaces() { try { interfaces = (await request('/interfaces')).interfaces || []; } catch (e) { error = e?.message || String(e); } }
  async function loadSelectedClient() {
    if (!selectedIP || clientPaused) return;
    try { clientDetail = await request(`/client?ip=${encodeURIComponent(selectedIP)}&limit=800`); } catch (e) { error = e?.message || String(e); }
  }
  async function loadSystem() { try { systemInfo = await request('/system'); } catch (e) { error = e?.message || String(e); } }
  async function loadBursts() { try { errorBursts = await request(`/error-bursts?minutes=${diagMinutes}`); } catch (e) { error = e?.message || String(e); } }

  async function ensureViewData(nextTab = tab) {
    if (nextTab === 'rules') await loadFallbacks();
    if (nextTab === 'traffic') await ensureTrafficData();
    if (nextTab === 'diagnostics') await ensureDiagnosticsData();
  }
  async function ensureTrafficData() {
    if (trafficTab === 'traffic') await loadHistory();
    else if (trafficTab === 'devices') selectedIP ? await loadSelectedClient() : await loadClients();
    else if (trafficTab === 'interfaces') await loadInterfaces();
  }
  async function ensureDiagnosticsData() {
    if (diagnosticsTab === 'journal') await loadBursts();
    else if (diagnosticsTab === 'system') await loadSystem();
  }

  async function refreshCurrent() {
    if (refreshBusy || document.hidden) return;
    refreshBusy = true;
    try {
      await loadAll(true);
      await ensureViewData(tab);
    } finally { refreshBusy = false; }
  }

  function setTab(value) {
    tab = value;
    const next = new URL(window.location.href);
    next.searchParams.set('view', value);
    window.history.replaceState(window.history.state, '', next);
    const parentRoutes = { overview:'/dns', resolvers:'/dns/servers', rules:'/dns/routing', traffic:'/dns/traffic', diagnostics:'/dns/tools' };
    try {
      if (window.parent !== window && window.parent.location.pathname.startsWith('/dns')) {
        const target = parentRoutes[value] || '/dns';
        if (window.parent.location.pathname !== target) window.parent.history.replaceState(window.parent.history.state, '', target);
      }
    } catch {}
    ensureViewData(value);
  }
  function setTrafficTab(value) { if (!trafficTabs.includes(value)) return; trafficTab = value; ensureTrafficData(); }
  function setDiagnosticsTab(value) { if (!diagnosticTabs.includes(value)) return; diagnosticsTab = value; ensureDiagnosticsData(); }
  function selectHistory(value) { historyMinutes = value; loadHistory(); }
  function selectRulesMinutes(value) { rulesMinutes = value; loadFallbacks(); }
  function selectDiagMinutes(value) { diagMinutes = value; loadBursts(); }

  function toggleFlowPause() { if (!flowPaused) { frozenFlow = liveFlow.slice(); flowPaused = true; } else { flowPaused = false; frozenFlow = []; } }
  function toggleClientPause() { if (!clientPaused) { frozenClientEvents = (clientDetail?.events || []).slice(); clientPaused = true; } else { clientPaused = false; frozenClientEvents = []; loadSelectedClient(); } }
  function toggleDiagPause() { if (!diagPaused) { frozenErrors = (snapshot?.errors || []).slice(); frozenBursts = (errorBursts?.bursts || snapshot?.error_bursts || []).slice(); diagPaused = true; } else { diagPaused = false; frozenErrors = []; frozenBursts = []; loadBursts(); } }
  function openClient(ip) { selectedIP = ip; clientPaused = false; frozenClientEvents = []; clientDetail = null; loadSelectedClient(); }
  function closeClient() { selectedIP = ''; clientPaused = false; frozenClientEvents = []; clientDetail = null; loadClients(); }

  function openAdd() { editing = null; form = blankForm(); editorOpen = true; }
  function openEdit(resolver) {
    editing = resolver;
    form = {
      protocol:resolver.protocol || 'DoT', address:resolver.address || '', uri:resolver.uri || '',
      port:resolver.port || (resolver.protocol === 'DoH' ? 443 : resolver.protocol === 'DNS' ? 53 : 853),
      sni:resolver.sni || '', interface:resolver.interface || '', domains:(resolver.domains || []).join('\n'), spki:resolver.spki || '', format:resolver.format || ''
    };
    editorOpen = true;
  }
  function payloadFromForm() {
    return {
      protocol:form.protocol,
      address:form.protocol === 'DoH' ? '' : form.address.trim(),
      uri:form.protocol === 'DoH' ? form.uri.trim() : '',
      port:Number(form.port || 0),
      sni:form.protocol === 'DoT' ? form.sni.trim() : '',
      interface:form.interface.trim(),
      domains:form.domains.split(/\r?\n|,/).map((v) => v.trim()).filter(Boolean),
      spki:['DoT','DoH'].includes(form.protocol) ? form.spki.trim() : '',
      format:form.protocol === 'DoH' ? form.format.trim() : ''
    };
  }
  async function saveEditor() {
    saving = true; error = ''; success = '';
    try {
      const payload = payloadFromForm();
      const result = editing
        ? await request(`/resolvers/${encodeURIComponent(editing.id)}`, { method:'PATCH', body:JSON.stringify(payload) })
        : await request('/resolvers', { method:'POST', body:JSON.stringify(payload) });
      if (result?.resolver?.id) selectedResolverId = result.resolver.id;
      editorOpen = false;
      success = L.saved;
      await loadAll(true);
    } catch (e) { error = e?.message || String(e); }
    finally { saving = false; }
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

  function reportText() {
    const lines = ['RouterForge DNS', new Date().toISOString(), `module=${snapshot?.version || '—'}`, ''];
    lines.push(`requests=${totalRequests} responses=${totalResponses} fallbacks=${totalFallbacks} timeouts=${totalTimeouts} errors=${totalErrors}`);
    lines.push(`resolvers=${resolvers.length} secure_slots=${secureSlotsUsed}/${secureSlotLimit || 'unknown'}`, '');
    for (const u of allUpstreams) lines.push(`[${u.profile}] ${u.name} ${u.protocol} ${u.target || ''} port=${u.port} health=${u.health_status || ''} requests=${u.requests || 0} fallbacks=${u.fallbacks || 0}`);
    lines.push('', `resolver_control=${JSON.stringify(resolverState, null, 2)}`);
    return lines.join('\n');
  }
  async function copyReport() {
    const text = reportText();
    try { await navigator.clipboard.writeText(text); }
    catch { const area = document.createElement('textarea'); area.value = text; document.body.appendChild(area); area.select(); document.execCommand('copy'); area.remove(); }
    success = L.copied;
  }
  function saveReport() {
    const blob = new Blob([reportText()], { type:'text/plain;charset=utf-8' });
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = href; a.download = `routerforge-dns-${new Date().toISOString().replace(/[:.]/g,'-')}.txt`; a.click();
    setTimeout(() => URL.revokeObjectURL(href), 1000);
  }

  function syncCoreVisualTokens() {
    try {
      if (window.parent === window) return;
      const parentRoot = window.parent.document.documentElement;
      const source = window.parent.getComputedStyle(parentRoot);
      const target = document.documentElement.style;
      const tokens = ['--page-pad','--ui-body','--ui-small','--ui-xs','--ui-micro','--ui-title','--ui-panel-title','--ui-control-h','--concept-bg','--concept-surface','--concept-panel','--concept-panel-2','--concept-hover','--concept-border','--concept-border-soft','--accent','--good','--warn','--bad','--font-sans','--font-mono'];
      for (const token of tokens) { const value = source.getPropertyValue(token).trim(); if (value) target.setProperty(token, value); }
    } catch {}
  }

  function syncFrameHeight() {
    try {
      if (window.parent === window || !window.frameElement) return;
      const frame = window.frameElement;
      const root = document.querySelector('.dns-module');
      if (!root) return;

      // ModuleFrame historically owns a viewport-sized iframe. Once the DNS
      // page became feature-rich again that produced a second, inner scrollbar
      // even when the Core page itself still had plenty of room. Because the
      // module is same-origin, size the iframe to its real content and let the
      // RouterForge shell/browser own vertical scrolling.
      const contentHeight = Math.ceil(root.getBoundingClientRect().height) + 4;
      const frameTop = frame.getBoundingClientRect().top;
      const viewportFloor = Math.max(720, Math.floor(window.parent.innerHeight - frameTop - 2));
      const nextHeight = Math.max(contentHeight, viewportFloor);

      frame.style.minHeight = '0px';
      if (Math.abs(frame.getBoundingClientRect().height - nextHeight) > 1) {
        frame.style.height = `${nextHeight}px`;
      }
    } catch {}
  }

  onMount(() => {
    const syncShell = () => {
      syncCoreVisualTokens();
      syncFrameHeight();
    };

    syncShell();

    const root = document.querySelector('.dns-module');
    const resizeObserver = typeof ResizeObserver !== 'undefined' && root
      ? new ResizeObserver(() => syncFrameHeight())
      : null;
    resizeObserver?.observe(root);

    window.addEventListener('resize', syncShell);
    try { window.parent.addEventListener('resize', syncFrameHeight); } catch {}

    loadAll().then(() => {
      ensureViewData(tab);
      syncFrameHeight();
    });
    refreshTimer = setInterval(refreshCurrent, 5000);

    return () => {
      clearInterval(refreshTimer);
      resizeObserver?.disconnect();
      window.removeEventListener('resize', syncShell);
      try { window.parent.removeEventListener('resize', syncFrameHeight); } catch {}
    };
  });
</script>

<div class="dns-module page">
  <div class="page-head">
    <div><h1>{L.title}</h1><p>{L.subtitle}</p></div>
    <div class="toolbar-actions">
      <span class="page-kicker mono">DNS {snapshot?.version ? `· ${snapshot.version}` : ''}</span>
      <button class="action" type="button" onclick={() => { loadAll(); ensureViewData(tab); }}>{L.refresh}</button>
      {#if tab === 'overview' || tab === 'resolvers'}<button class="action primary" type="button" onclick={openAdd}>+ {L.add}</button>{/if}
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
    <div class="toolbar parity-toolbar">
      <div class="search-control"><span>⌕</span><input bind:value={overviewSearch} placeholder={L.searchDns}/></div>
      <div class="segmented"><button class:active={overviewActiveOnly} type="button" onclick={() => overviewActiveOnly = true}>{L.activeOnly}</button><button class:active={!overviewActiveOnly} type="button" onclick={() => overviewActiveOnly = false}>{L.all}</button></div>
    </div>

    <section class="metric-grid four">
      <div class="metric-card"><span>DNS-СЕРВЕРЫ</span><strong>{healthyCount}/{serverCount}</strong><small>{activeCount} {locale === 'en' ? 'active' : 'активны'} · DOWN {downCount}</small></div>
      <div class="metric-card"><span>{L.requests}</span><strong>{fmtInt(totalRequests)}</strong><small>{fmtInt(totalResponses)} {locale === 'en' ? 'responses' : 'ответов'}</small></div>
      <div class="metric-card"><span>{L.fallback}</span><strong>{fmtInt(totalFallbacks)}</strong><small>{fmtPct(totalRequests ? totalFallbacks / totalRequests * 100 : 0, 2)}</small></div>
      <div class="metric-card"><span>{L.problems}</span><strong>{fmtInt(totalErrors)}</strong><small>{fmtInt(totalTimeouts)} timeout · degraded {degradedCount}</small></div>
    </section>

    {#if filteredPlain.length}
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{L.mainDns}</strong><span>show ip name-server · passive request/response correlation</span></div><span class="state-pill info">{filteredPlain.length} DNS</span></div>
        <div class="table-wrap"><table><thead><tr><th>DNS</th><th>{L.status}</th><th>{L.requests}</th><th>{L.latency}</th><th>{L.problems}</th><th>{L.quality}</th><th>{L.iface}</th></tr></thead><tbody>
          {#each filteredPlain as r (`${r.address}:${r.port || 53}`)}
            <tr>
              <td><div class="cell-title">{r.name || r.address}</div><div class="cell-sub mono">{r.address}:{r.port || 53}{r.source ? ` · ${r.source}` : ''}</div></td>
              <td><span class="state-chip {plainStatus(r).cls}">{plainStatus(r).label}</span><div class="cell-sub">{plainRecentlyActive(r) ? L.currentUse : fmtAgo(r.last_request)}</div></td>
              <td><strong>{fmtInt(r.requests)}</strong><div class="cell-sub">{fmtPct(share(r.requests), 2)} {L.trafficShare}</div></td>
              <td>{#if num(r.p95_latency_ms)}<span class="latency {latencyClass(r.p95_latency_ms)}">p95 {fmtMs(r.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(r.avg_latency_ms)}</div>{:else}—{/if}</td>
              <td><strong class={num(r.errors) + num(r.timeouts) > 0 ? 'warn-text' : ''}>{fmtInt(num(r.errors) + num(r.timeouts))}</strong><div class="cell-sub">{fmtInt(r.timeouts)} timeout · {fmtInt(r.nxdomain)} NX</div></td>
              <td><strong class={num(r.requests) && num(r.responses) / num(r.requests) < .95 ? 'warn-text' : 'good-text'}>{fmtPct(num(r.requests) ? num(r.responses) / num(r.requests) * 100 : 100)}</strong></td>
              <td>{r.interface || '—'}</td>
            </tr>
          {/each}
        </tbody></table></div>
      </section>
    {/if}

    {#if filteredProtected.length}
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{L.protectedDns}</strong><span>System DoT/DoH runtime · без policy-дублей</span></div><span class="state-pill info">{filteredProtected.length}</span></div>
        <div class="table-wrap"><table><thead><tr><th>DNS</th><th>{L.type}</th><th>{L.status}</th><th>{L.requests}</th><th>{L.latency}</th><th>{L.problems}</th><th>{L.fallback}</th><th>{L.quality}</th><th>{L.port}</th></tr></thead><tbody>
          {#each filteredProtected as u (u.port)}
            <tr>
              <td><div class="cell-title">{u.name}</div><div class="cell-sub">{u.target || u.sni || '—'}{u.domain ? ` · ${u.domain}` : ''}</div></td>
              <td><span class="pill accent">{u.protocol}</span></td>
              <td><span class="state-chip {upstreamStatus(u).cls}">{upstreamStatus(u).label}</span><div class="cell-sub">{u.active ? L.currentUse : fmtAgo(u.last_request)}</div></td>
              <td><strong>{fmtInt(u.requests)}</strong><div class="cell-sub">{fmtPct(share(u.requests), 2)} {L.trafficShare}</div></td>
              <td>{#if num(u.stats_5m?.p95_latency_ms)}<span class="latency {latencyClass(u.stats_5m?.p95_latency_ms)}">p95 {fmtMs(u.stats_5m?.p95_latency_ms)}</span><div class="cell-sub">avg {fmtMs(u.stats_5m?.avg_latency_ms)}</div>{:else}—{/if}</td>
              <td><strong class={num(u.stats_5m?.errors) + num(u.stats_5m?.timeouts) > 0 ? 'warn-text' : ''}>{fmtInt(num(u.stats_5m?.errors) + num(u.stats_5m?.timeouts))}</strong><div class="cell-sub">{fmtInt(u.stats_5m?.timeouts)} timeout</div></td>
              <td><strong class={num(u.stats_5m?.fallbacks) > 0 ? 'warn-text' : ''}>{fmtInt(u.stats_5m?.fallbacks)}</strong><div class="cell-sub">{fmtPct(u.stats_5m?.fallback_pct || 0)}</div></td>
              <td><strong class={qualityClass(u.stats_5m)}>{fmtPct(u.stats_5m?.quality_pct ?? 100, 1)}</strong></td>
              <td class="mono">{u.port}</td>
            </tr>
          {/each}
        </tbody></table></div>
      </section>
    {/if}

    {#if !filteredPlain.length && !filteredProtected.length}<section class="panel"><div class="empty-box">{L.noMatching}</div></section>{/if}

    {#if profileStats.filter((p) => p.name !== 'System').length}
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{L.policyContexts}</strong><span>{locale === 'en' ? 'Service Keenetic policy contexts are separate from main DNS health.' : 'Служебные Policy Keenetic — отдельно от основного DNS.'}</span></div><span class="state-pill info">{L.advanced}</span></div>
        <div class="table-wrap"><table><thead><tr><th>POLICY</th><th>{L.route}</th><th>DNS PROXY</th><th>{L.resolverCount}</th><th>{L.requests}</th><th>{L.markTable}</th></tr></thead><tbody>
          {#each profileStats.filter((p) => p.name !== 'System') as p}
            <tr><td><div class="cell-title">{p.first.policy_description || p.name}</div><div class="cell-sub mono">{p.name}</div></td><td>{#if p.first.policy_has_default}<span class="state-chip good">{L.routeOk}</span>{:else}<span class="state-chip neutral">{L.routeMissing}</span>{/if}</td><td class="mono">:{p.first.profile_dns_port || '—'}</td><td>{p.count} · {p.active} active</td><td>{fmtInt(p.requests)}</td><td class="mono">0x{num(p.first.policy_mark).toString(16)} · {p.first.policy_table || '—'}</td></tr>
          {/each}
        </tbody></table></div>
      </section>
    {/if}

  {:else if tab === 'resolvers'}
    <div class="toolbar parity-toolbar resolver-toolbar">
      <div class="search-control"><span>⌕</span><input bind:value={resolverSearch} placeholder={L.searchDns}/></div>
      <select bind:value={resolverProtocol}><option value="all">{L.filterProtocol}: {L.all}</option><option value="DNS">DNS</option><option value="DoT">DoT</option><option value="DoH">DoH</option></select>
      <select bind:value={resolverStatus}><option value="all">{L.filterStatus}: {L.all}</option><option value="active">{L.active}</option><option value="disabled">{L.disabled}</option><option value="dynamic">{L.dynamic}</option></select>
      <div class="toolbar-spacer"></div>
      <div class="segmented resolver-view-switch" aria-label={locale === 'en' ? 'Resolver view' : 'Вид резолверов'}>
        <button class:active={resolverView === 'detail'} type="button" aria-pressed={resolverView === 'detail'} onclick={() => setResolverView('detail')}>{locale === 'en' ? 'List' : 'Список'}</button>
        <button class:active={resolverView === 'cards'} type="button" aria-pressed={resolverView === 'cards'} onclick={() => setResolverView('cards')}>{locale === 'en' ? 'Cards' : 'Карточки'}</button>
      </div>
      <span class="state-pill info">{L.secureSlots}: {secureSlotsUsed}/{secureSlotLimit || '—'}</span>
    </div>

    <div class="resolver-summary-strip">
      <div><span>{L.configured}</span><strong>{resolvers.length}</strong></div>
      <div><span>{L.activeResolvers}</span><strong class="good-text">{activeResolvers.length}</strong></div>
      <div><span>{L.disabledResolvers}</span><strong class={disabledResolvers.length ? 'warn-text' : ''}>{disabledResolvers.length}</strong></div>
      <div><span>{L.dynamicResolvers}</span><strong>{dynamicResolvers.length}</strong></div>
      <div><span>{L.secureSlots}</span><strong class={secureSlotLimit && secureSlotsUsed >= secureSlotLimit ? 'warn-text' : ''}>{secureSlotsUsed}/{secureSlotLimit || '—'}</strong></div>
    </div>

    {#if !filteredResolvers.length}
      <section class="panel"><div class="empty-box">{L.noResolvers}</div></section>
    {:else if resolverView === 'cards'}
      <section class="panel resolver-panel resolver-cards-panel">
        <div class="panel-head"><div><strong>{L.configured}</strong><span>{L.nativeHint}</span></div><span class="panel-meta">{filteredResolvers.length}/{resolvers.length}</span></div>
        <div class="resolver-grid resolver-grid-body resolver-grid-uniform">
          {#each filteredResolvers as resolver (resolver.id)}
            <article class="resolver-card resolver-card-uniform">
              <div class="resolver-head"><div><h3>{resolver.name}</h3><div class="resolver-meta mono resolver-card-endpoint" title={endpoint(resolver)}>{resolver.protocol} · {endpoint(resolver)}</div></div><span class="state-pill {resolverStatusClass(resolver)}">{resolverStatusText(resolver)}</span></div>
              <div class="detail-grid compact">
                {#if resolver.sni}<div class="detail-item"><span>SNI</span><strong class="mono">{resolver.sni}</strong></div>{/if}
                {#if resolver.interface}<div class="detail-item"><span>{L.iface}</span><strong>{resolver.interface}</strong></div>{/if}
                <div class="detail-item"><span>{L.physicalCount}</span><strong>{resolver.physical_count || 1}</strong></div>
              </div>
              <div class="resolver-meta">{L.scope}</div>
              <div class="scope-list">{#if scopes(resolver).length}{#each scopes(resolver) as d}<span class="scope-chip">{displayDomain(d)}</span>{/each}{:else}<span class="scope-chip">{L.global}</span>{/if}</div>
              <div class="resolver-actions">
                <button class="action primary" type="button" onclick={() => { selectResolver(resolver.id); setResolverView('detail'); }}>{locale === 'en' ? 'Details' : 'Сведения'}</button>
                <button class="action" type="button" disabled={resolver.dynamic} onclick={() => openEdit(resolver)}>{L.edit}</button>
                {#if resolver.disabled}<button class="action primary" type="button" onclick={() => resolverAction(resolver,'enable')}>{L.enable}</button>{:else if !resolver.dynamic}<button class="action" type="button" onclick={() => resolverAction(resolver,'disable')}>{L.disable}</button>{/if}
                <button class="action danger" type="button" disabled={resolver.dynamic} onclick={() => resolverAction(resolver,'delete')}>{L.remove}</button>
              </div>
              {#if resolver.dynamic}<div class="resolver-meta resolver-readonly-note">{L.readOnly}{resolver.service ? ` · ${resolver.service}` : ''}</div>{/if}
            </article>
          {/each}
        </div>
      </section>
    {:else}
      <div class="resolver-master-detail">
        <aside class="panel resolver-master-panel">
          <div class="panel-head"><div><strong>{L.configured}</strong><span>{filteredResolvers.length}/{resolvers.length} · {L.nativeHint}</span></div></div>
          <div class="resolver-master-list">
            {#each filteredResolvers as resolver (resolver.id)}
              <button class="resolver-master-item" class:active={resolver.id === selectedResolverId} type="button" onclick={() => selectResolver(resolver.id)}>
                <span class="resolver-master-dot {resolverStatusClass(resolver)}"></span>
                <span class="resolver-master-copy">
                  <span class="resolver-master-title"><strong>{resolver.name}</strong><span class="pill accent">{resolver.protocol}</span></span>
                  <span class="resolver-master-endpoint mono" title={endpoint(resolver)}>{endpoint(resolver)}</span>
                  <span class="resolver-master-meta">{resolverScopeSummary(resolver)} · {resolver.physical_count || 1} {locale === 'en' ? 'native' : 'нативн.'}{resolver.dynamic ? ` · ${resolver.service || 'DHCP'}` : ''}</span>
                </span>
                <span class="state-pill {resolverStatusClass(resolver)} resolver-master-state">{resolverStatusText(resolver)}</span>
              </button>
            {/each}
          </div>
        </aside>

        {#if selectedResolver}
          <div class="resolver-detail-stack">
            <section class="panel resolver-detail-hero">
              <div class="resolver-detail-head">
                <div class="resolver-detail-title">
                  <div class="resolver-title-line"><h2>{selectedResolver.name}</h2><span class="pill accent">{selectedResolver.protocol}</span></div>
                  <p class="mono" title={endpoint(selectedResolver)}>{endpoint(selectedResolver)}{selectedResolver.source ? ` · ${selectedResolver.source}` : ''}</p>
                </div>
                <div class="resolver-detail-actions">
                  <span class="state-pill {resolverStatusClass(selectedResolver)}">{resolverStatusText(selectedResolver)}</span>
                  <button class="action" type="button" disabled={selectedResolver.dynamic} onclick={() => openEdit(selectedResolver)}>{L.edit}</button>
                  {#if selectedResolver.disabled}<button class="action primary" type="button" onclick={() => resolverAction(selectedResolver,'enable')}>{L.enable}</button>{:else if !selectedResolver.dynamic}<button class="action" type="button" onclick={() => resolverAction(selectedResolver,'disable')}>{L.disable}</button>{/if}
                  <button class="action danger" type="button" disabled={selectedResolver.dynamic} onclick={() => resolverAction(selectedResolver,'delete')}>{L.remove}</button>
                </div>
              </div>
              {#if selectedResolver.dynamic}<div class="resolver-dynamic-banner"><span class="state-pill neutral">READ ONLY</span><span>{L.readOnly}{selectedResolver.service ? ` · ${selectedResolver.service}` : ''}</span></div>{/if}
              <div class="resolver-detail-metrics">
                <div><strong>{fmtInt(selectedRuntime?.summary?.requests || 0)}</strong><span>{L.requests}{selectedRuntime?.kind === 'secure' ? ' · 5m' : ''}</span></div>
                <div><strong>{num(selectedRuntime?.summary?.p95_latency_ms) ? fmtMs(selectedRuntime.summary.p95_latency_ms) : '—'}</strong><span>P95 latency{selectedRuntime?.kind === 'secure' ? ' · 5m' : ''}</span></div>
                <div><strong>{fmtPct(selectedRuntime?.summary?.fallback_pct || 0, 2)}</strong><span>{L.fallback}{selectedRuntime?.kind === 'secure' ? ' · 5m' : ''}</span></div>
                <div><strong class={selectedRuntime?.rows?.length ? qualityClass(selectedRuntime?.summary || {}) : ''}>{selectedRuntime?.rows?.length ? fmtPct(selectedRuntime?.summary?.quality_pct ?? 100, 1) : '—'}</strong><span>{L.quality}{selectedRuntime?.kind === 'secure' ? ' · 5m' : ''}</span></div>
              </div>
              <div class="resolver-runtime-line"><span>{locale === 'en' ? 'Runtime state' : 'Состояние runtime'}</span><span class="state-chip {selectedRuntime?.state?.cls || 'neutral'}">{selectedRuntime?.state?.label || '—'}</span><span>{selectedRuntime?.last_request ? fmtAgo(selectedRuntime.last_request) : (locale === 'en' ? 'no observed queries' : 'запросы не наблюдались')}</span></div>
            </section>

            {#if selectedRuntime?.kind === 'secure' && selectedRuntime?.rows?.length}
              <section class="panel resolver-quality-panel">
                <div class="panel-head"><div><strong>{locale === 'en' ? 'Resolver quality' : 'Качество resolver'}</strong><span>{locale === 'en' ? 'Aggregated windows across native entries' : 'Агрегированные окна по нативным записям'}</span></div><span class="state-pill info">{selectedRuntime.rows.length} native</span></div>
                {#each ['stats_5m','stats_1h','stats_24h'] as key}
                  {@const w = selectedRuntime.windows[key]}
                  <div class="info-row resolver-window-row"><div><strong>{runtimeWindowLabel(key)}</strong><span>{fmtInt(w.requests)} {locale === 'en' ? 'requests' : 'запросов'} · {fmtInt(w.errors)} DNS errors · {fmtInt(w.timeouts)} timeout</span></div><div class="info-value"><strong class={qualityClass(w)}>{fmtPct(w.quality_pct ?? 100, 2)}</strong> · p95 {num(w.p95_latency_ms) ? fmtMs(w.p95_latency_ms) : '—'} · fallback {fmtPct(w.fallback_pct || 0, 2)}</div></div>
                {/each}
              </section>
            {:else if selectedRuntime?.kind === 'plain'}
              <section class="panel resolver-quality-panel">
                <div class="panel-head"><div><strong>{locale === 'en' ? 'Plain DNS statistics' : 'Статистика обычного DNS'}</strong><span>Passive request → response correlation</span></div></div>
                <div class="info-row"><div><strong>{L.responses}</strong><span>{locale === 'en' ? 'Matched responses' : 'Сопоставленные ответы'}</span></div><div class="info-value">{fmtInt(selectedRuntime.summary.responses)}</div></div>
                <div class="info-row"><div><strong>{L.timeouts}</strong><span>{locale === 'en' ? 'No response before timeout' : 'Ответ не получен до таймаута'}</span></div><div class="info-value {num(selectedRuntime.summary.timeouts) ? 'warn-text' : ''}">{fmtInt(selectedRuntime.summary.timeouts)}</div></div>
                <div class="info-row"><div><strong>NXDOMAIN</strong><span>DNS RCODE</span></div><div class="info-value">{fmtInt(selectedRuntime.summary.nxdomain)}</div></div>
              </section>
            {/if}

            {#if selectedResolver.protocol !== 'DNS'}
              <section class="panel resolver-diagnostic-panel">
                <div class="panel-head"><div><strong>{locale === 'en' ? 'DoT/DoH diagnostics' : 'Диагностика DoT/DoH'}</strong><span>{locale === 'en' ? 'Automatic runtime probe / health state' : 'Автоматический runtime probe / health'}</span></div>{#if selectedRuntime?.diagnostic?.ran}<span class="state-chip {selectedRuntime.diagnostic.status === 'FAIL' ? 'error' : 'good'}">{selectedRuntime.diagnostic.status || '—'}</span>{/if}</div>
                {#if selectedRuntime?.diagnostic?.ran}
                  {@const d = selectedRuntime.diagnostic}
                  <div class="info-row"><div><strong>{L.stage}</strong><span>{locale === 'en' ? 'Last reached stage' : 'Последний достигнутый этап'}</span></div><div class="info-value">{d.stage || '—'}</div></div>
                  <div class="info-row"><div><strong>Target IP</strong><span>{locale === 'en' ? 'Direct probe target' : 'IP прямой проверки'}</span></div><div class="info-value mono">{d.target_ip || '—'}</div></div>
                  <div class="info-row"><div><strong>Resolve / TCP / TLS</strong><span>{locale === 'en' ? 'Connection stages' : 'Этапы соединения'}</span></div><div class="info-value mono">{fmtMs(d.resolve_ms)} · {fmtMs(d.tcp_ms)} · {fmtMs(d.tls_ms)}</div></div>
                  <div class="info-row"><div><strong>{selectedResolver.protocol === 'DoH' ? 'HTTP / DNS' : 'DNS'}</strong><span>{locale === 'en' ? 'Protocol check' : 'Протокольная проверка'}</span></div><div class="info-value">{num(d.protocol_ms) ? fmtMs(d.protocol_ms) : '—'}{selectedResolver.protocol === 'DoH' && d.http_status ? ` · HTTP ${d.http_status}` : ''}</div></div>
                  <div class="info-row"><div><strong>DNS RCODE</strong><span>{locale === 'en' ? 'Direct DNS probe result' : 'Ответ прямого DNS probe'}</span></div><div class="info-value">{d.dns_rcode || '—'}</div></div>
                  <div class="info-row"><div><strong>{L.assessment}</strong><span>{d.route_scope || 'default-route'}</span></div><div class="info-value {d.status === 'FAIL' ? 'warn-text' : 'good-text'}">{d.assessment || (d.status === 'FAIL' ? (locale === 'en' ? 'Probe failed' : 'Проверка не пройдена') : 'OK')}</div></div>
                  <div class="info-row"><div><strong>{locale === 'en' ? 'Error' : 'Ошибка'}</strong><span>{locale === 'en' ? 'Stop reason' : 'Причина остановки'}</span></div><div class="info-value mono">{d.error || selectedRuntime.rows.find((u) => u.last_health_error)?.last_health_error || '—'}</div></div>
                {:else}<div class="empty-box small">{locale === 'en' ? 'Runtime diagnostics have not run for this resolver yet.' : 'Runtime-диагностика для этого резолвера ещё не запускалась.'}</div>{/if}
              </section>
            {/if}

            <section class="panel resolver-info-panel">
              <div class="panel-head"><div><strong>{locale === 'en' ? 'DNS information' : 'Сведения о DNS'}</strong><span>{locale === 'en' ? 'Configuration + native/runtime metadata' : 'Конфигурация + native/runtime metadata'}</span></div><span class="state-pill info">{selectedResolver.physical_count || 1} native</span></div>
              <div class="info-row"><div><strong>{L.target}</strong><span>{locale === 'en' ? 'Configured resolver endpoint' : 'Настроенный endpoint резолвера'}</span></div><div class="info-value mono">{endpoint(selectedResolver)}</div></div>
              {#if selectedResolver.sni}<div class="info-row"><div><strong>SNI / FQDN</strong><span>TLS Server Name</span></div><div class="info-value mono">{selectedResolver.sni}</div></div>{/if}
              {#if selectedResolver.spki}<div class="info-row"><div><strong>SPKI</strong><span>{locale === 'en' ? 'Certificate pin' : 'Пин сертификата'}</span></div><div class="info-value mono">{selectedResolver.spki}</div></div>{/if}
              {#if selectedResolver.format}<div class="info-row"><div><strong>Format</strong><span>DoH wire format</span></div><div class="info-value mono">{selectedResolver.format}</div></div>{/if}
              <div class="info-row"><div><strong>Domain filter</strong><span>{L.scope}</span></div><div class="info-value resolver-scope-value">{scopes(selectedResolver).length ? scopes(selectedResolver).map(displayDomain).join(' · ') : L.global}</div></div>
              <div class="info-row"><div><strong>{L.iface}</strong><span>{locale === 'en' ? 'Keenetic outgoing interface' : 'Исходящий интерфейс Keenetic'}</span></div><div class="info-value">{selectedResolver.interface || selectedRuntime?.rows?.find((u) => u.interface)?.interface || '—'}</div></div>
              <div class="info-row"><div><strong>{L.physicalCount}</strong><span>{L.nativeHint}</span></div><div class="info-value">{selectedResolver.physical_count || 1}</div></div>
              {#if selectedRuntime?.ports?.length}<div class="info-row"><div><strong>{locale === 'en' ? 'Local port' : 'Локальный порт'}</strong><span>{locale === 'en' ? 'Internal resolver proxy' : 'Внутренний resolver proxy'}</span></div><div class="info-value mono">{selectedRuntime.ports.map((p) => `:${p}`).join(' · ')}</div></div>{/if}
              {#if selectedRuntime?.profilePorts?.length}<div class="info-row"><div><strong>System DNS proxy</strong><span>{locale === 'en' ? 'Profile DNS listener' : 'DNS listener профиля'}</span></div><div class="info-value mono">{selectedRuntime.profilePorts.map((p) => `:${p}`).join(' · ')}</div></div>{/if}
              {#if selectedRuntime?.rows?.[0]?.timeout_ms || selectedRuntime?.rows?.[0]?.proceed_ms}<div class="info-row"><div><strong>Timeout / Proceed</strong><span>Keenetic fallback thresholds</span></div><div class="info-value mono">{selectedRuntime.rows[0].timeout_ms ? `${selectedRuntime.rows[0].timeout_ms} ms` : '—'} / {selectedRuntime.rows[0].proceed_ms ? `${selectedRuntime.rows[0].proceed_ms} ms` : '—'}</div></div>{/if}
              <div class="info-row"><div><strong>{locale === 'en' ? 'Source' : 'Источник'}</strong><span>{selectedResolver.dynamic ? L.readOnly : (locale === 'en' ? 'RouterForge/Keenetic configuration' : 'Конфигурация RouterForge/Keenetic')}</span></div><div class="info-value">{selectedResolver.service || selectedResolver.source || (selectedResolver.dynamic ? 'Keenetic service/DHCP' : 'static')}</div></div>
            </section>

            <section class="panel table-panel resolver-recent-panel">
              <div class="panel-head"><div><strong>{locale === 'en' ? 'Recent queries' : 'Последние запросы'}</strong><span>{selectedRuntime?.last_request ? fmtAgo(selectedRuntime.last_request) : L.noData}</span></div><span class="panel-meta">{selectedRecent.length}</span></div>
              <div class="table-wrap"><table><thead><tr><th>{locale === 'en' ? 'Time' : 'Время'}</th><th>{L.domains}</th><th>{L.type}</th><th>RCODE</th><th>{L.fallback}</th></tr></thead><tbody>
                {#if selectedRecent.length}{#each selectedRecent as e, i (`${e.time}-${e.domain}-${e.qtype}-${i}`)}<tr><td class="mono">{timeOnly(e.time)}</td><td>{e.domain}</td><td><span class="pill">{e.qtype}</span></td><td>{e.rcode || '—'}</td><td>{#if e.fallback}<span class="pill warn">YES</span>{:else}<span class="cell-sub">{e.status || '—'}</span>{/if}</td></tr>{/each}{:else}<tr><td colspan="5" class="empty-row">{locale === 'en' ? 'No live queries for the selected resolver.' : 'Для выбранного резолвера live-запросов пока нет.'}</td></tr>{/if}
              </tbody></table></div>
            </section>
          </div>
        {/if}
      </div>
    {/if}

    <div class="resolver-safety-note">
      <span class="state-pill good">READBACK / ROLLBACK</span>
      <span>{L.rollbackNote}</span>
    </div>

  {:else if tab === 'rules'}
    <div class="toolbar parity-toolbar">
      <select bind:value={rulesProfile}><option value="all">{L.allProfiles}</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
      <div class="segmented"><button class:active={rulesMinutes === 5} type="button" onclick={() => selectRulesMinutes(5)}>5 мин</button><button class:active={rulesMinutes === 60} type="button" onclick={() => selectRulesMinutes(60)}>1 ч</button><button class:active={rulesMinutes === 1440} type="button" onclick={() => selectRulesMinutes(1440)}>24 ч</button></div>
      <div class="toolbar-spacer"></div><span class="panel-meta">{fmtInt(ruleEdgeCount)} fallback</span>
    </div>

    <div class="two-col parity-two-col routing-grid">
      <section class="panel table-panel">
        <div class="panel-head"><div><strong>{L.fallbackRoutes}</strong><span>{L.fallbackHint}</span></div></div>
        <div class="table-wrap"><table><thead><tr><th>{L.from}</th><th></th><th>{L.to}</th><th>{L.profile}</th><th>{L.triggers}</th></tr></thead><tbody>
          {#if ruleEdges.length}{#each ruleEdges as e (`${e.from_port}-${e.to_port}-${e.from_profile}`)}<tr><td><div class="cell-title">{e.from_upstream}</div><div class="cell-sub mono">:{e.from_port}</div></td><td class="accent-text">→</td><td><div class="cell-title">{e.to_upstream}</div><div class="cell-sub mono">:{e.to_port}</div></td><td>{e.from_profile}</td><td><strong class="good-text">{fmtInt(e.count)}</strong></td></tr>{/each}{:else}<tr><td colspan="5" class="empty-row">{L.noFallbacks}</td></tr>{/if}
        </tbody></table></div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>{L.keeneticProfiles}</strong><span>{L.discoveryData}</span></div></div>
        {#each profileStats as p}<div class="info-row"><div><strong>{p.name}</strong><span>{p.count} DNS · {p.active} active{p.first.policy_description ? ` · ${p.first.policy_description}` : ''}</span></div><div class="info-value accent-text">{fmtInt(p.requests)}</div></div>{/each}
      </section>
    </div>

    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{L.bindings}</strong><span>{L.bindingsHint}</span></div></div>
      <div class="table-wrap"><table><thead><tr><th>{L.resolvers}</th><th>{L.protocol}</th><th>{L.scope}</th><th>{L.physicalCount}</th></tr></thead><tbody>
        {#each activeResolvers as resolver}<tr><td><div class="cell-title">{resolver.name}</div><div class="cell-sub mono">{endpoint(resolver)}</div></td><td><span class="pill accent">{resolver.protocol}</span></td><td>{scopes(resolver).length ? scopes(resolver).map(displayDomain).join(', ') : L.global}</td><td>{resolver.physical_count || 1}</td></tr>{/each}
      </tbody></table></div>
    </section>

    <div class="two-col parity-two-col">
      <section class="panel">
        <div class="panel-head"><div><strong>{L.localRecords}</strong><span>{info?.static_records?.length || 0}</span></div></div>
        {#if info?.static_records?.length}<div class="table-wrap"><table><thead><tr><th>Host</th><th>Type</th><th>Value</th></tr></thead><tbody>{#each info.static_records as r}<tr><td class="mono">{r.host}</td><td>{r.type}</td><td class="mono">{r.value}</td></tr>{/each}</tbody></table></div>{:else}<div class="empty-box">{L.noData}</div>{/if}
      </section>
      <section class="panel">
        <div class="panel-head"><div><strong>{L.rebind}</strong><span>{info?.rebind?.enabled ? L.rebindOn : L.rebindOff}</span></div></div>
        <div class="detail-grid padded"><div class="detail-item"><span>{L.protectedNets}</span><strong>{(info?.rebind?.nets || []).join(', ') || '—'}</strong></div><div class="detail-item"><span>{L.exclusions}</span><strong>{(info?.rebind?.excludes || []).join(', ') || '—'}</strong></div></div>
      </section>
    </div>

  {:else if tab === 'traffic'}
    <div class="subtabs parity-subtabs">
      <button class:active={trafficTab === 'traffic'} type="button" onclick={() => setTrafficTab('traffic')}>{L.traffic}</button>
      <button class:active={trafficTab === 'flow'} type="button" onclick={() => setTrafficTab('flow')}>{L.flow}</button>
      <button class:active={trafficTab === 'devices'} type="button" onclick={() => setTrafficTab('devices')}>{L.devices}</button>
      <button class:active={trafficTab === 'interfaces'} type="button" onclick={() => setTrafficTab('interfaces')}>{L.interfaces}</button>
      <button class:active={trafficTab === 'domains'} type="button" onclick={() => setTrafficTab('domains')}>{L.domainsTab}</button>
    </div>

    {#if trafficTab === 'traffic'}
      <div class="toolbar parity-toolbar"><div class="toolbar-spacer"></div><div class="segmented"><button class:active={historyMinutes === 5} type="button" onclick={() => selectHistory(5)}>5 мин</button><button class:active={historyMinutes === 60} type="button" onclick={() => selectHistory(60)}>1 ч</button><button class:active={historyMinutes === 180} type="button" onclick={() => selectHistory(180)}>3 ч</button><button class:active={historyMinutes === 1440} type="button" onclick={() => selectHistory(1440)}>24 ч</button></div></div>
      <section class="panel">
        <div class="panel-head"><div><strong>{L.trafficGraph}</strong><span>{historyPoints.length} {L.buckets}</span></div><div class="legend"><span><i class="legend-dot requests-dot"></i>{L.requests}</span><span><i class="legend-dot fallback-dot"></i>{L.fallback}</span><span><i class="legend-dot error-dot"></i>{L.errors}</span></div></div>
        {#if historyPoints.length}
          <div class="chart-shell">
            <svg viewBox="0 0 1000 220" preserveAspectRatio="none" role="img" aria-label="DNS traffic history">
              <line x1="0" y1="190" x2="1000" y2="190" class="chart-axis"/><line x1="0" y1="110" x2="1000" y2="110" class="chart-grid"/><line x1="0" y1="30" x2="1000" y2="30" class="chart-grid"/>
              <polygon points={chartArea} class="chart-area"/><polyline points={chartLine} class="chart-line"/>
              {#each chartCoords as p}{#if p.fallbacks > 0}<line x1={p.x} y1="196" x2={p.x} y2="184" class="chart-fallback"/>{/if}{#if p.errors > 0}<line x1={p.x + 2} y1="196" x2={p.x + 2} y2="181" class="chart-error"/>{/if}{/each}
            </svg>
            <div class="chart-labels"><span>{historyPoints[0]?.time ? timeOnly(historyPoints[0].time) : '—'}</span><span>max {fmtInt(historyMax)}</span><span>{historyPoints.at(-1)?.time ? timeOnly(historyPoints.at(-1).time) : '—'}</span></div>
          </div>
        {:else}<div class="empty-box">{L.noData}</div>{/if}
      </section>
      <section class="metric-grid four compact-metrics"><div class="metric-card"><span>{L.requests}</span><strong>{fmtInt(snapshot?.total_requests)}</strong><small>{fmtInt(snapshot?.total_responses)} {L.responses}</small></div><div class="metric-card"><span>{L.fallback}</span><strong>{fmtInt(snapshot?.total_fallbacks)}</strong><small>{fmtPct(snapshot?.total_requests ? num(snapshot.total_fallbacks) / num(snapshot.total_requests) * 100 : 0,2)}</small></div><div class="metric-card"><span>{L.errors}</span><strong>{fmtInt(snapshot?.active_quality_bad)}</strong><small>quality bad active</small></div><div class="metric-card"><span>{L.timeouts}</span><strong>{fmtInt(snapshot?.total_timeouts)}</strong><small>runtime total</small></div></section>

    {:else if trafficTab === 'flow'}
      <div class="toolbar parity-toolbar">
        <div class="search-control"><span>⌕</span><input bind:value={flowSearch} placeholder={L.domainOrDevice}/></div>
        <select bind:value={flowProfile}><option value="all">{L.allProfiles}</option>{#each profiles as p}<option value={p}>{p}</option>{/each}</select>
        <button class="action" class:active={fallbackOnly} type="button" onclick={() => fallbackOnly = !fallbackOnly}>{L.fallbackOnly}</button>
        <div class="toolbar-spacer"></div><span class="live-chip {flowPaused ? 'paused' : 'running'}">{flowPaused ? `${L.frozen} · ${frozenFlow.length}` : L.live}</span><button class="action" type="button" onclick={toggleFlowPause}>{flowPaused ? L.continue : L.pause}</button>
      </div>
      <section class="panel table-panel live-table"><div class="table-wrap"><table><thead><tr><th>{locale === 'en' ? 'Time' : 'Время'}</th><th>{L.device}</th><th>{L.domains}</th><th>{L.profile}</th><th>DNS</th><th>{L.type}</th><th>{L.fallback}</th></tr></thead><tbody>
        {#if flowRows.length}{#each flowRows as x (`${x.time}|${x.client_ip || ''}|${x.domain}|${x.qtype}|${x.port || ''}`)}<tr><td class="mono">{timeOnly(x.time)}</td><td>{#if x.client_ip}<button class="table-link" type="button" onclick={() => { setTrafficTab('devices'); openClient(x.client_ip); }}><strong>{x.client_name || x.client_hostname || x.client_ip}</strong><span>{x.client_ip}{x.client_access ? ` · ${x.client_access}` : ''}</span></button>{:else}—{/if}</td><td><strong>{x.domain}</strong></td><td>{x.profile}</td><td>{x.upstream}</td><td><span class="pill">{x.qtype}</span></td><td>{#if x.fallback}<span class="pill warn">YES</span>{:else}—{/if}</td></tr>{/each}{:else}<tr><td colspan="7" class="empty-row">{L.noMatching}</td></tr>{/if}
      </tbody></table></div></section>
      {#if plain?.recent?.length}<section class="panel table-panel"><div class="panel-head"><div><strong>{L.mainRecent}</strong><span>{plain.recent.length}</span></div></div><div class="table-wrap"><table><thead><tr><th>{locale === 'en' ? 'Time' : 'Время'}</th><th>DNS</th><th>{L.domains}</th><th>{L.type}</th><th>RCODE</th><th>{L.latency}</th><th>{L.status}</th></tr></thead><tbody>{#each plain.recent.slice(0,80) as e, i (`${e.time}-${e.resolver}-${e.domain}-${i}`)}<tr><td class="mono">{timeOnly(e.time)}</td><td class="mono">{e.resolver}:{e.port || 53}</td><td>{e.domain}</td><td><span class="pill">{e.qtype}</span></td><td>{e.rcode || '—'}</td><td>{fmtMs(e.latency_ms)}</td><td><span class="state-chip {e.status === 'TIMEOUT' ? 'error' : 'good'}">{e.status}</span></td></tr>{/each}</tbody></table></div></section>{/if}

    {:else if trafficTab === 'devices'}
      {#if selectedIP}
        <div class="toolbar parity-toolbar"><button class="action" type="button" onclick={closeClient}>← {L.allDevices}</button><div class="toolbar-spacer"></div><span class="live-chip {clientPaused ? 'paused' : 'running'}">{clientPaused ? L.frozen : L.live}</span><button class="action" type="button" onclick={toggleClientPause}>{clientPaused ? L.continue : L.pause}</button></div>
        {#if clientDetail?.client}
          <section class="hero-card parity-hero">
            <div class="hero-head"><div><h2>{clientDetail.client.name || clientDetail.client.hostname || clientDetail.client.ip}</h2><p>{clientDetail.client.ip} · {clientDetail.client.mac || '—'} · {clientDetail.client.access || clientDetail.client.network || '—'}</p></div><span class="state-chip {clientDetail.client.active ? 'good' : 'neutral'}">{clientDetail.client.active ? L.activeDevice : L.offline}</span></div>
            <div class="hero-metrics six"><div><strong>{fmtInt(clientDetail.client.requests)}</strong><span>{L.requests}</span></div><div><strong>{fmtInt(clientDetail.client.forwarded)}</strong><span>{L.forwarded}</span></div><div><strong>{fmtInt(clientDetail.client.cache_local)}</strong><span>{L.cacheLocal}</span></div><div><strong>{fmtInt(clientDetail.client.fallbacks)}</strong><span>{L.fallback}</span></div><div><strong>{fmtMs(clientDetail.client.avg_client_latency_ms)}</strong><span>{L.clientAvg}</span></div><div><strong>{fmtMs(clientDetail.client.p95_client_latency_ms)}</strong><span>{L.clientP95}</span></div></div>
          </section>
          <div class="two-col parity-two-col">
            <section class="panel"><div class="panel-head"><div><strong>{L.clientState}</strong><span>{clientDetail.client.policy || 'System'}</span></div></div><div class="info-row"><div><strong>{L.errors}</strong><span>upstream</span></div><div class={`info-value ${num(clientDetail.client.errors) > 0 ? 'bad-text' : ''}`}>{fmtInt(clientDetail.client.errors)}</div></div><div class="info-row"><div><strong>{L.timeouts}</strong><span>upstream</span></div><div class={`info-value ${num(clientDetail.client.timeouts) > 0 ? 'bad-text' : ''}`}>{fmtInt(clientDetail.client.timeouts)}</div></div><div class="info-row"><div><strong>Client errors</strong></div><div class="info-value">{fmtInt(clientDetail.client.client_errors)}</div></div><div class="info-row"><div><strong>Client timeouts</strong></div><div class="info-value">{fmtInt(clientDetail.client.client_timeouts)}</div></div></section>
            <section class="panel"><div class="panel-head"><div><strong>{L.policyRoute}</strong><span>{clientDetail.client.route?.mode || L.systemDefault}</span></div></div>{#if clientDetail.client.route?.paths?.length}{#each clientDetail.client.route.paths as path}<div class="route-path"><div><strong>{path.description || path.keenetic_interface || path.linux_interface}</strong><span>{[path.keenetic_interface,path.linux_interface,path.type].filter(Boolean).join(' · ')}</span></div>{#if path.weight}<code>weight {path.weight}</code>{/if}</div>{/each}{:else}<div class="empty-box small">{L.noPolicyRoute}</div>{/if}</section>
          </div>
          <div class="toolbar parity-toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientFlowSearch} placeholder={L.domainOrDevice}/></div><select bind:value={clientOutcome}><option value="all">{L.allOutcomes}</option><option value="FORWARDED">FORWARDED</option><option value="CACHE_LOCAL">CACHE_LOCAL</option><option value="ERROR">ERROR</option><option value="CLIENT_TIMEOUT">CLIENT_TIMEOUT</option></select><div class="toolbar-spacer"></div><span class="panel-meta">{clientEvents.length}</span></div>
          <section class="panel table-panel"><div class="table-wrap"><table><thead><tr><th>{locale === 'en' ? 'Time' : 'Время'}</th><th>{L.domains}</th><th>{L.outcome}</th><th>RCODE</th><th>{L.latency}</th><th>DNS</th><th>{L.fallback}</th></tr></thead><tbody>{#if clientEvents.length}{#each clientEvents as e (`${e.time}|${e.domain}|${e.outcome}|${e.resolver_port || ''}`)}<tr><td class="mono">{timeOnly(e.time)}</td><td>{e.domain}</td><td><span class="state-chip {outcomeClass(e.outcome)}">{e.outcome}</span></td><td>{e.rcode || '—'}</td><td>{fmtMs(e.latency_ms)}</td><td>{e.resolver || '—'}{e.resolver_port ? ` :${e.resolver_port}` : ''}</td><td>{e.fallback ? 'YES' : '—'}</td></tr>{/each}{:else}<tr><td colspan="7" class="empty-row">{L.noData}</td></tr>{/if}</tbody></table></div></section>
        {:else}<div class="empty-box">…</div>{/if}
      {:else}
        <div class="toolbar parity-toolbar"><div class="search-control"><span>⌕</span><input bind:value={clientSearch} placeholder={L.domainOrDevice}/></div><div class="toolbar-spacer"></div><span class="panel-meta">{filteredClients.length}</span></div>
        <section class="panel table-panel"><div class="table-wrap"><table><thead><tr><th>{L.device}</th><th>IP / MAC</th><th>Policy</th><th>{L.network}</th><th>{L.status}</th><th>{L.requests}</th><th>{L.problems}</th><th>{L.fallback}</th></tr></thead><tbody>{#if filteredClients.length}{#each filteredClients as c (c.ip)}<tr><td><button class="table-link" type="button" onclick={() => openClient(c.ip)}><strong>{c.name || c.hostname || c.ip}</strong><span>{c.access || c.ssid || c.port || '—'}</span></button></td><td><div class="cell-title mono">{c.ip}</div><div class="cell-sub mono">{c.mac || '—'}</div></td><td>{c.policy || 'System'}</td><td>{c.network || c.network_id || '—'}</td><td><span class="state-chip {c.active ? 'good' : 'neutral'}">{c.active ? L.activeDevice : L.offline}</span></td><td>{fmtInt(c.requests)}</td><td>{fmtInt(num(c.errors) + num(c.timeouts))}</td><td>{fmtInt(c.fallbacks)}</td></tr>{/each}{:else}<tr><td colspan="8" class="empty-row">{L.noData}</td></tr>{/if}</tbody></table></div></section>
      {/if}

    {:else if trafficTab === 'interfaces'}
      <section class="panel table-panel"><div class="panel-head"><div><strong>{L.interfaces}</strong><span>{interfaces.length}</span></div></div><div class="table-wrap"><table><thead><tr><th>{L.interfaceName}</th><th>{L.network}</th><th>{L.devices}</th><th>{L.requests}</th><th>{L.errors}</th><th>{L.timeouts}</th><th>{L.fallback}</th><th>{L.topClients}</th></tr></thead><tbody>{#if interfaces.length}{#each interfaces as row (row.key)}<tr><td><div class="cell-title">{row.name}</div><div class="cell-sub">{row.ssid || row.ap || row.key}</div></td><td>{row.network || '—'}</td><td>{row.devices}</td><td>{fmtInt(row.requests)}</td><td>{fmtInt(row.errors)}</td><td>{fmtInt(row.timeouts)}</td><td>{fmtInt(row.fallbacks)}</td><td>{(row.top_clients || []).slice(0,3).map((x) => `${x.name} (${fmtInt(x.count)})`).join(' · ') || '—'}</td></tr>{/each}{:else}<tr><td colspan="8" class="empty-row">{L.noData}</td></tr>{/if}</tbody></table></div></section>

    {:else if trafficTab === 'domains'}
      <div class="two-col parity-two-col domains-grid">
        <section class="panel table-panel"><div class="panel-head"><div><strong>{L.topDomains}</strong><span>{topDomains.length}</span></div></div><div class="table-wrap"><table><thead><tr><th>#</th><th>{L.domains}</th><th>{L.requests}</th><th></th></tr></thead><tbody>{#each topDomains.slice(0,100) as row, i}<tr><td class="mono">{i + 1}</td><td class="mono">{row.domain || row.name || '—'}</td><td>{fmtInt(row.count ?? row.requests)}</td><td><div class="bar"><i style={`width:${topDomains[0]?.count ? Math.min(100, num(row.count) / num(topDomains[0].count) * 100) : 0}%`}></i></div></td></tr>{/each}</tbody></table></div></section>
        <section class="panel"><div class="panel-head"><div><strong>{L.qtypes}</strong><span>{liveFlow.length} events</span></div></div>{#if qtypeGroups.length}{#each qtypeGroups as [name,count]}<div class="info-row"><div><strong>{name}</strong></div><div class="info-value">{fmtInt(count)}</div></div>{/each}{:else}<div class="empty-box">{L.noData}</div>{/if}</section>
      </div>
    {/if}

  {:else if tab === 'diagnostics'}
    <div class="subtabs parity-subtabs">
      <button class:active={diagnosticsTab === 'journal'} type="button" onclick={() => setDiagnosticsTab('journal')}>{L.journal}</button>
      <button class:active={diagnosticsTab === 'connections'} type="button" onclick={() => setDiagnosticsTab('connections')}>{L.connections}</button>
      <button class:active={diagnosticsTab === 'dns'} type="button" onclick={() => setDiagnosticsTab('dns')}>{L.dnsInfo}</button>
      <button class:active={diagnosticsTab === 'system'} type="button" onclick={() => setDiagnosticsTab('system')}>{L.systemTab}</button>
      <div class="subtab-spacer"></div><button class="subtab-action" type="button" onclick={() => advanced = !advanced}>{L.advanced}: {advanced ? 'ON' : 'OFF'}</button><button class="subtab-action" type="button" onclick={copyReport}>{L.copy}</button><button class="subtab-action" type="button" onclick={saveReport}>{L.saveReport}</button>
    </div>

    {#if diagnosticsTab === 'journal'}
      <div class="toolbar parity-toolbar"><span class="live-chip {diagPaused ? 'paused' : 'running'}">{diagPaused ? L.frozen : L.live}</span><button class="action" type="button" onclick={toggleDiagPause}>{diagPaused ? L.continue : L.pause}</button>{#each ['all','SERVFAIL','TIMEOUT','DOWN','RECOVERED'] as value}<button class="action" class:active={diagKind === value} type="button" onclick={() => diagKind = value}>{value === 'all' ? L.all.toUpperCase() : value}</button>{/each}<div class="search-control flex"><span>⌕</span><input bind:value={diagSearch} placeholder={L.searchDns}/></div><div class="segmented"><button class:active={diagMinutes === 5} type="button" onclick={() => selectDiagMinutes(5)}>5м</button><button class:active={diagMinutes === 60} type="button" onclick={() => selectDiagMinutes(60)}>1ч</button><button class:active={diagMinutes === 1440} type="button" onclick={() => selectDiagMinutes(1440)}>24ч</button></div></div>
      {#if diagBursts.length}<section class="panel table-panel"><div class="panel-head"><div><strong>{L.errorSummary}</strong><span>{diagBursts.length} burst buckets</span></div></div><div class="table-wrap"><table><thead><tr><th>{locale === 'en' ? 'Time' : 'Время'}</th><th>DNS</th><th>{L.type}</th><th>{locale === 'en' ? 'Count' : 'Кол-во'}</th><th>{L.examples}</th></tr></thead><tbody>{#each diagBursts as b (`${b.minute}-${b.upstream}-${b.kind}`)}<tr><td class="mono">{timeOnly(b.minute)}</td><td>{b.profile} · <strong>{b.upstream}</strong></td><td><span class="pill {b.kind === 'TIMEOUT' ? 'warn' : 'error'}">{b.kind}</span></td><td class="bad-text">{fmtInt(b.count)}</td><td class="cell-sub">{(b.domains || []).join(', ') || '—'}</td></tr>{/each}</tbody></table></div></section>{/if}
      <section class="panel"><div class="log-shell">{#if diagRows.length}{#each diagRows as row (`${row.time}|${row.kind}|${row.upstream}|${row.domain}`)}<div class="log-row"><span class="mono muted">{timeOnly(row.time)}</span><span class="log-kind {row.kind}">{row.kind}</span><span>{row.profile || ''} · {row.upstream || ''} · {row.domain || ''} — {row.message || ''}</span></div>{/each}{:else}<div class="empty-box small">{L.noData}</div>{/if}</div></section>

    {:else if diagnosticsTab === 'connections'}
      <section class="panel table-panel"><div class="panel-head"><div><strong>{L.localProxies}</strong><span>{allUpstreams.length} endpoints</span></div></div><div class="table-wrap"><table><thead><tr><th>{L.profile}</th><th>DNS</th><th>{L.protocol}</th><th>Local</th><th>{L.target}</th><th>{L.iface}</th><th>{L.status}</th></tr></thead><tbody>{#each allUpstreams as u (u.port)}<tr><td>{u.profile}</td><td><strong>{u.name}</strong></td><td><span class="pill accent">{u.protocol}</span></td><td class="mono">127.0.0.1:{u.port}</td><td class="mono">{u.target || '—'}</td><td>{u.interface || u.linux_interface || '—'}</td><td><span class="state-chip {upstreamStatus(u).cls}">{upstreamStatus(u).label}</span></td></tr>{/each}</tbody></table></div></section>

    {:else if diagnosticsTab === 'dns'}
      <div class="two-col parity-two-col">
        <section class="panel"><div class="panel-head"><div><strong>{L.discovery}</strong><span>{L.status}</span></div></div><div class="info-row"><div><strong>{L.lastDiscovery}</strong></div><div class="info-value">{snapshot?.last_discovery ? fmtAgo(snapshot.last_discovery) : '—'}</div></div><div class="info-row"><div><strong>{L.discoveryError}</strong></div><div class="info-value">{snapshot?.discovery_error || L.none}</div></div><div class="info-row"><div><strong>{L.captureError}</strong></div><div class="info-value">{snapshot?.capture_error || L.none}</div></div><div class="info-row"><div><strong>{L.clientCaptureError}</strong></div><div class="info-value">{snapshot?.client_capture_error || L.none}</div></div></section>
        <section class="panel"><div class="panel-head"><div><strong>{L.counters}</strong><span>runtime totals</span></div></div><div class="info-row"><div><strong>{L.requests}</strong></div><div class="info-value">{fmtInt(snapshot?.total_requests)}</div></div><div class="info-row"><div><strong>{L.responses}</strong></div><div class="info-value">{fmtInt(snapshot?.total_responses)}</div></div><div class="info-row"><div><strong>{L.fallback}</strong></div><div class="info-value">{fmtInt(snapshot?.total_fallbacks)}</div></div><div class="info-row"><div><strong>{L.timeouts}</strong></div><div class="info-value">{fmtInt(snapshot?.total_timeouts)}</div></div></section>
      </div>
      <section class="panel table-panel"><div class="panel-head"><div><strong>{L.diagnosticsRuntime}</strong><span>{allUpstreams.length} upstreams</span></div></div><div class="table-wrap"><table><thead><tr><th>DNS</th><th>{L.health}</th><th>{L.lastUse}</th><th>{L.stage}</th><th>{L.assessment}</th><th>{L.healthError}</th>{#if advanced}<th>MARK/TABLE</th><th>Linux IF</th>{/if}</tr></thead><tbody>{#each allUpstreams as u (u.port)}<tr><td><div class="cell-title">{u.profile} · {u.name}</div><div class="cell-sub mono">:{u.port} · {u.target || '—'}</div></td><td><span class="state-chip {upstreamStatus(u).cls}">{upstreamStatus(u).label}</span></td><td>{fmtAgo(u.last_request)}</td><td>{u.diagnostic?.stage || '—'}</td><td>{u.diagnostic?.assessment || '—'}</td><td>{u.last_health_error || u.diagnostic?.error || '—'}</td>{#if advanced}<td class="mono">0x{num(u.policy_mark).toString(16)} / {u.policy_table || '—'}</td><td class="mono">{u.linux_interface || '—'}</td>{/if}</tr>{/each}</tbody></table></div></section>
      {#if advanced}
        {#each info?.proxies || [] as proxy}<section class="panel"><div class="panel-head"><div><strong>{proxyName(proxy)}</strong><span class="mono">{proxy.name} · TCP {proxy.tcp_port} / UDP {proxy.udp_port}</span></div></div><div class="detail-grid padded"><div class="detail-item"><span>{L.requests}</span><strong>{fmtInt(proxy.stat?.total_requests)}</strong></div><div class="detail-item"><span>{L.sent}</span><strong>{fmtInt(proxy.stat?.proxy_requests_sent)}</strong></div><div class="detail-item"><span>{L.cache}</span><strong>{fmtPct(num(proxy.stat?.cache_hit_ratio) * 100)}</strong></div><div class="detail-item"><span>{L.memory}</span><strong>{proxy.stat?.memory || '—'}</strong></div></div></section>{/each}
      {/if}

    {:else if diagnosticsTab === 'system'}
      <div class="two-col parity-two-col">
        <section class="panel"><div class="panel-head"><div><strong>RouterForge DNS</strong><span>{L.runtime}</span></div></div><div class="info-row"><div><strong>{L.moduleVersion}</strong></div><div class="info-value">{snapshot?.version || '—'}</div></div><div class="info-row"><div><strong>Uptime</strong></div><div class="info-value">{fmtInt(snapshot?.uptime_seconds)} s</div></div><div class="info-row"><div><strong>Go</strong></div><div class="info-value mono">{systemInfo?.go_version || '—'}</div></div><div class="info-row"><div><strong>Architecture</strong></div><div class="info-value mono">{systemInfo?.goarch || '—'}</div></div></section>
        <section class="panel"><div class="panel-head"><div><strong>{L.process}</strong><span>routerforge-dns</span></div></div><div class="info-row"><div><strong>RSS</strong></div><div class="info-value">{fmtInt(systemInfo?.rss_kb)} KiB</div></div><div class="info-row"><div><strong>VmSize</strong></div><div class="info-value">{fmtInt(systemInfo?.vmsize_kb)} KiB</div></div><div class="info-row"><div><strong>Goroutines</strong></div><div class="info-value">{fmtInt(systemInfo?.goroutines)}</div></div><div class="info-row"><div><strong>PID</strong></div><div class="info-value mono">{fmtInt(systemInfo?.pid)}</div></div></section>
      </div>
    {/if}
  {/if}

  {#if editorOpen}
    <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) editorOpen = false; }}>
      <div class="modal" role="dialog" aria-modal="true">
        <h2>{editing ? L.editTitle : L.addTitle}</h2>
        <div class="form-grid">
          <label>{L.protocol}<select class="protocol-select" bind:value={form.protocol} onchange={() => { if (form.protocol === 'DoT') form.port = 853; if (form.protocol === 'DNS') form.port = 53; if (form.protocol === 'DoH') form.port = 443; }}><option>DNS</option><option>DoT</option><option>DoH</option></select></label>
          {#if form.protocol !== 'DoH'}<label>{L.port}<input type="number" min="1" max="65535" bind:value={form.port}/></label>{/if}
          {#if form.protocol === 'DoH'}<label class="span-2">{L.uri}<input class="mono" placeholder="https://dns.example/dns-query" bind:value={form.uri}/></label>{:else}<label class="span-2">{L.address}<input class="mono" placeholder={form.protocol === 'DNS' ? '1.1.1.1' : '1.1.1.1 / dns.example'} bind:value={form.address}/></label>{/if}
          {#if form.protocol === 'DoT'}<label>{L.sni}<input class="mono" placeholder="cloudflare-dns.com" bind:value={form.sni}/></label><label>{L.spki}<input class="mono" bind:value={form.spki}/></label>{/if}
          {#if form.protocol === 'DoH'}<label>{L.format}<input class="mono" placeholder="dnsm / json" bind:value={form.format}/></label><label>{L.spki}<input class="mono" bind:value={form.spki}/></label>{/if}
          <label>{L.iface}<input class="mono" placeholder="ISP" bind:value={form.interface}/></label>
          <label class="span-2">{L.domains}<textarea class="mono" placeholder={'ru\nsu\nxn--p1ai'} bind:value={form.domains}></textarea><span class:slot-warning={editorLimitExceeded}>{L.domainsHint}{#if ['DoT','DoH'].includes(form.protocol) && secureSlotLimit} · {L.secureSlots}: {secureSlotsUsed}/{secureSlotLimit} → {L.afterSave}: {projectedSecureSlots}/{secureSlotLimit}{#if secureCapacityExceeded} · {L.limitExceeded}{/if}{:else if form.protocol === 'DNS'} · {L.dnsDomainLimit}: {formDomainCount}/{plainDnsDomainLimit}{#if plainDomainExceeded} · {L.limitExceeded}{/if}{/if}</span></label>
        </div>
        <div class="modal-actions"><button class="action" type="button" onclick={() => editorOpen = false}>{L.cancel}</button><button class="action primary" type="button" disabled={saving || editorLimitExceeded} onclick={saveEditor}>{saving ? '…' : L.save}</button></div>
      </div>
    </div>
  {/if}
</div>
