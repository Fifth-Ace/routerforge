'use strict';
/* P25: Advanced-only settings hub. Existing inputs retain their original IDs,
 * event listeners, draft and apply semantics. Future capabilities are READ-ONLY.
 * The basic-mode DOM is restored exactly when switching back.
 */
(() => {
  const $ = id => document.getElementById(id);
  const app = $('dnsLiteApp');
  const workspace = $('p21Page');
  const navLogs = $('navLogs');
  const additional = $('additionalSettings');
  const cacheInput = $('cacheSize');
  if (!app || !workspace || !navLogs || !additional || !cacheInput || $('p25SettingsNav')) return;

  const english = document.documentElement.lang === 'en' ||
    new URLSearchParams(location.search).get('lang') === 'en';
  const t = (ru, en) => english ? en : ru;
  const element = (tag, cls, label) => {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (label !== undefined) node.textContent = label;
    return node;
  };

  const tab = element('button', 'tab p22-advanced-tab', t('Настройки', 'Settings'));
  tab.id = 'p25SettingsNav';
  tab.type = 'button';
  tab.setAttribute('aria-controls', 'p25SettingsCard');
  navLogs.before(tab);

  const card = element('section', 'panel rfds-card p25-settings-card');
  card.id = 'p25SettingsCard';
  card.hidden = true;
  card.setAttribute('aria-label', t('Настройки DNS Router', 'DNS Router settings'));
  const heading = element('div', 'p25-settings-heading');
  heading.append(
    element('h2', '', t('Настройки DNS Router', 'DNS Router settings')),
    element('p', 'muted', t('Настройки сервиса, кэша и будущих возможностей DNSProxy в одном месте.',
      'Service, cache and upcoming DNSProxy capabilities in one place.'))
  );
  card.append(heading);
  const layout = element('div', 'p25-settings-layout');
  const side = element('nav', 'p25-settings-sidebar');
  side.setAttribute('aria-label', t('Категории настроек', 'Settings categories'));
  const body = element('div', 'p25-settings-content');
  layout.append(side, body);
  card.append(layout);
  workspace.append(card);

  const entries = [
    ['service', t('DNS-сервис', 'DNS service'), t('Доступно', 'Available')],
    ['cache', t('Кэш', 'Cache'), t('Доступно', 'Available')],
    ['ip', 'IP Intelligence', t('Экспериментально', 'Experimental')],
    ['upstream', t('Выбор DNS', 'Upstream selection'), t('Планируется', 'Planned')],
    ['security', t('Защита запросов', 'Query protection'), t('Доступно', 'Available')],
    ['privacy', 'EDNS / ECS / DNSSEC', t('Планируется', 'Planned')],
    ['dns64', 'DNS64 / rDNS', t('Планируется', 'Planned')],
    ['performance', t('Производительность', 'Performance'), t('Планируется', 'Planned')]
  ];
  const buttons = new Map();
  const panels = new Map();
  for (const [key, title, state] of entries) {
    const button = element('button', 'p25-settings-link');
    button.type = 'button';
    button.dataset.settingsSection = key;
    button.append(element('span', 'p25-settings-link-label', title),
      element('small', 'p25-settings-link-state', state));
    side.append(button);
    buttons.set(key, button);

    const panel = element('section', 'p25-settings-section');
    panel.id = 'p25Settings_' + key;
    panel.hidden = true;
    panel.setAttribute('aria-label', title);
    panel.append(element('h3', '', title));
    body.append(panel);
    panels.set(key, panel);
    button.addEventListener('click', () => selectSection(key));
  }
  function selectSection(key) {
    for (const [name, panel] of panels) {
      const selected = key === name;
      panel.hidden = !selected;
      buttons.get(name).classList.toggle('selected', selected);
      buttons.get(name).setAttribute('aria-pressed', String(selected));
    }
    if (key === 'ip') refreshIPHealth();
    if (key === 'performance') refreshP29Metrics();
  }

  const service = panels.get('service');
  service.append(element('p', 'p25-settings-info', t(
    'Здесь находятся действующие параметры DNS-сервиса. Сохранение и применение — через общую панель сверху.',
    'Existing service parameters. Save and apply using the common toolbar above.')));
  const cache = panels.get('cache');
  cache.append(element('p', 'p25-settings-info', t(
    'Размер существующего DNS-кэша. Изменения вступают в силу только после сохранения и применения.',
    'Current DNS cache size. Changes take effect only after saving and applying.')));

  function planned(key, message, features) {
    const panel = panels.get(key);
    panel.append(element('p', 'p25-settings-planned-note', t(
      'Пока не подключено к работающему DNS. Нет скрытого включения или изменения конфигурации.',
      'Not connected to the live DNS service yet. No hidden activation or configuration change.')));
    panel.append(element('p', 'p25-settings-info', message));
    const list = element('ul', 'p25-settings-features');
    features.forEach(x => list.append(element('li', '', x)));
    panel.append(list);
  }
  // P24.1: real, explicitly opt-in DNS response ordering. No background
  // health checks yet; unavailable measurements do not invent IP health.
  const ipPanel=panels.get('ip');
  ipPanel.append(element('p','p25-settings-info',t(
    'P24.2: после явного включения фоновые TCP/TLS-проверки публичных IP влияют на сортировку. Без измерений адрес остаётся UNKNOWN.',
    'P24.0 contains nine IP ordering strategies. P24.2 adds opt-in public-IP TCP/TLS checks; unknown never means down.')));
  const ipControls=$('p241IPControls');
  ipControls.hidden=false;
  ipPanel.append(ipControls);
  function showScope(){const scope=$('p243Scope').value;const a=ipPanel.querySelector('.p243-domains'),b=ipPanel.querySelector('.p243-lists');a.hidden=scope!=='domains';b.hidden=scope!=='lists';}
  $('p243Scope').addEventListener('change',showScope);
  document.addEventListener('routerforge:p243-render',showScope);
  showScope();
  // P24.2 monitor is read-only. Active only on the visible IP settings page.
  const monitorRows=$('p242MonitorRows'),monitorStatus=$('p242MonitorStatus');
  async function refreshIPHealth(){
    if(app.dataset.dnsMode!=='advanced'||card.hidden||panels.get('ip').hidden)return;
    try{
      const response=await fetch('../service/ip-health',{method:'GET',cache:'no-store'});
      if(!response.ok)throw Error('HTTP '+response.status);
      const data=await response.json();
      const list=Array.isArray(data.entries)?data.entries:[];
      monitorStatus.textContent=data.enabled?t('Активен · записей: ','Active · entries: ')+list.length:t('Монитор выключен','Monitor disabled');
      monitorRows.replaceChildren();
      if(list.length===0){monitorRows.append(element('span','muted',t('Нет измерений. Дождись DNS-запросов.','No measurements yet.')));return;}
      for(const record of list){
        const line=element('div','p242-monitor-row');
        line.title=[record.last_checked?t('Проверен: ','Checked: ')+record.last_checked:'',record.last_error?t('Ошибка: ','Error: ')+record.last_error:'',record.last_down?t('DOWN с: ','DOWN since: ')+record.last_down:''].filter(Boolean).join(' · ');
        line.append(element('span','p242-monitor-address',String(record.address||'—')),
          element('span','p242-monitor-domain',String(record.domain||'—')),
          element('span','p242-monitor-state',String(record.status||'UNKNOWN')),
          element('span','p242-monitor-rtt',(record.rtt_ms?String(record.rtt_ms)+' ms':'—')+' · '+String(record.failures||0)+' '+t('ош.','err')+' / '+String(record.recoveries||0)+' '+t('усп.','ok')),
          element('span','p242-monitor-last',record.last_success?String(record.last_success).replace('T',' ').replace(/Z$/,' UTC'):'—'));
        monitorRows.append(line);
      }
    }catch(err){monitorStatus.textContent=t('Ошибка мониторинга: ','Monitor error: ')+err.message;}
  }
  setInterval(refreshIPHealth,10000);
  const modeNotes={
    original:t('Без изменения IP в DNS-ответе.','No address changes.'),
    round_robin:t('Циклически меняет первый IP при каждом новом DNS-запросе, включая ответы из кэша.','Rotates the first IP on each DNS request, including cached responses.'),
    random:t('Случайно переставляет все IP; исходный DNS-ответ сохранён.','Shuffles all addresses; original DNS response is preserved.'),
    fastest:t('С проверками TCP/TLS сортирует по задержке; без них сохраняет исходный порядок.','Waiting for latency measurements: until P24.2, original order is preserved.'),
    health_aware:t('С проверками ставит доступные IP первыми; неизвестные адреса не считаются упавшими.','Waiting for health checks: until P24.2, original order is preserved.'),
    adaptive:t('С измерениями учитывает доступность и задержку; без измерений использует ротацию.','Before P24.2, rotates unmeasured addresses; health scoring is unavailable.'),
    sticky:t('Закрепляет порядок за проверенным MAC; для неизвестных устройств использует ротацию.','Stable ordering for trusted MACs; falls back to rotation for unknown devices.'),
    weighted:t('С измерениями веса учитывают задержку; без них используются равные веса.','Weights are equal until P24.2, behaves as simple rotation.'),
    healthy_only:t('Без проверок IP ничего не исключается. DNSSEC-защищённые наборы не сокращаются.','Without checks no IPs are removed. DNSSEC-protected RRsets are never filtered.')
  };
  function updateModeNote(){
    const enabled=$('p241IPEnabled').checked;
    $('p241IPModeNote').textContent=enabled?(modeNotes[$('p241IPMode').value]||modeNotes.original):t(
      'Функция выключена. DNS-ответы не изменяются.','Feature disabled. DNS responses remain unchanged.');
  }
  $('p241IPEnabled').addEventListener('change',updateModeNote);
  $('p241IPMode').addEventListener('change',updateModeNote);
  document.addEventListener('routerforge:p241-render',updateModeNote);
  updateModeNote();
  planned('upstream', t(
    'Режимы DNSProxy будут согласованы с уже работающими правилами RouterForge и резервными цепочками.',
    'DNSProxy upstream modes must coexist with RouterForge routing policies and failover chains.'),
    ['Load balance', 'Parallel', 'Fastest address', t('Тайм-ауты и резервные серверы', 'Timeouts and fallbacks')]);
  const securityPanel = panels.get('security');
  securityPanel.append(element('p','p25-settings-info',t(
    'Реальная защита DNS: ограничение частоты на IP клиента, отказ на ANY и замена ответов с указанными IP на NXDOMAIN. Сначала сохрани, затем явно примени настройки.',
    'Live query protection: per-client rate limiting, ANY refusal and sinkhole-address NXDOMAIN replacement. Save, then explicitly apply.')));
  const protectionControls=$('p27SecurityControls');
  protectionControls.hidden=false;
  securityPanel.append(protectionControls);
  planned('privacy', t(
    'Настройки EDNS Client Subnet и DNSSEC. Установка бита DO — не полноценная криптографическая проверка подписей.',
    'EDNS Client Subnet and DNSSEC options. Setting the DO bit does not validate DNSSEC signatures.'),
    ['EDNS Client Subnet', t('Префикс ECS', 'ECS prefix'), 'DNSSEC DO']);
  planned('dns64', t(
    'DNS64 нужен только при соответствующей NAT64-инфраструктуре; локальные имена Keenetic продолжат обрабатываться отдельно.',
    'DNS64 requires a corresponding NAT64 network; local Keenetic names remain separately resolved.'),
    ['DNS64 / NAT64', t('Префиксы DNS64', 'DNS64 prefixes'), 'Private rDNS']);
  planned('performance', t(
    'Настройки обработки повторных запросов и поведения кэша будут добавлены без нарушения текущей изоляции устройств.',
    'Duplicate request processing and cache behavior will be added without breaking device policy isolation.'),
    ['Optimistic cache', 'Cache min/max TTL', 'Pending requests', t('Метрики и диагностика', 'Metrics and diagnostics')]);

  // P28-P31: genuine settings instead of decorative planned sections.
  function newInput(section,id,title,kind,values){
    const panel=panels.get(section),wrap=element('label','field');
    const caption=element('span','',title),el=element(kind==='select'?'select':'input');
    el.id=id;
    if(kind==='select') for(const pair of values){const op=element('option','',pair[1]);op.value=pair[0];el.append(op);}
    else if(kind==='check'){el.type='checkbox';wrap.className='check';}
    else {el.type=kind||'text';}
    wrap.append(caption,el);panel.append(wrap);el.addEventListener('change',()=>document.dispatchEvent(new Event('routerforge:p28-p31-change')));return el;
  }
  function available(key,note){
    const panel=panels.get(key);
    panel.querySelectorAll('.p25-settings-planned-note,.p25-settings-features,.p25-settings-info').forEach(x=>x.remove());
    const el=buttons.get(key).querySelector('.p25-settings-link-state');
    if(el)el.textContent=t('Доступно','Available');
    panel.append(element('p','p25-settings-info',note));
  }
  available('upstream',t('Применяется только к существующей явной DNS-группе или резервной цепочке. Без второго DNS выбранный режим ничего не меняет.','Requires an explicit DNS group or fallback chain; a single server is unchanged.'));
  newInput('upstream','p28Enabled',t('Включить выбор DNS','Enable upstream selection'),'check');
  newInput('upstream','p28Mode',t('Алгоритм','Mode'),'select', [['sequential','Failover / Sequential'],['load_balance','Load Balance'],['parallel','Parallel'],['fastest','Fastest DNS']]);
  const p28timeout=newInput('upstream','p28Timeout',t('Тайм-аут на DNS, мс','Timeout per DNS, ms'),'number');p28timeout.min=250;p28timeout.max=5000;p28timeout.value=3000;
  available('performance',t('TTL применяется к кэшу DNSProxy; общий кэш выключается при применённых индивидуальных правилах устройств.','DNSProxy cache TTL; device-specific policies retain cache isolation.'));
  newInput('performance','p29Enabled',t('Включить расширенные настройки','Enable performance settings'),'check');
  for(const [id,title,max,value] of [['p29MinTTL',t('Минимум TTL, с','Min cache TTL, s'),3600,0],['p29MaxTTL',t('Максимум TTL, с (0 — без ограничения)','Max cache TTL, s (0 = no cap)'),86400,0],['p29Concurrent',t('Параллельные обработчики','Concurrent handlers'),64,32]]){const x=newInput('performance',id,title,'number');x.min=id==='p29Concurrent'?8:0;x.max=max;x.value=value;}
  newInput('performance','p29Optimistic',t('Optimistic cache','Optimistic cache'),'check');
  newInput('performance','p29Pending',t('Объединять одинаковые запросы','Deduplicate pending queries'),'select',[['default',t('По умолчанию (вкл.)','Default (on)')],['on',t('Включено','On')],['off',t('Выключено','Off')]]);
  const p29Metrics=element('p','p25-settings-info',t('Метрики: нет данных','Metrics: no data'));p29Metrics.id='p29Metrics';panels.get('performance').append(p29Metrics);
  async function refreshP29Metrics(){
    if(app.dataset.dnsMode!=='advanced'||card.hidden||panels.get('performance').hidden)return;
    try{const r=await fetch('../service/status',{method:'GET',cache:'no-store'});if(!r.ok)throw Error('HTTP '+r.status);const d=await r.json();
      p29Metrics.textContent=t('Запросы: ','Queries: ')+String(d.requests||0)+t(' · Кэш: ',' · Cache: ')+String(d.cache_hits||0)+t(' · Ошибки: ',' · Errors: ')+String(d.failures||0)+t(' · DNS-измерений: ',' · Measured upstreams: ')+Object.keys(d.upstream_selection_metrics?.upstreams||{}).length;
    }catch(e){p29Metrics.textContent=t('Метрики недоступны: ','Metrics unavailable: ')+e.message;}
  }
  setInterval(refreshP29Metrics,10000);
  available('privacy',t('DNSSEC: Request DO запрашивает подписи. Require AD доверяет подтверждению ТОЛЬКО шифрованного DNS; это не локальная криптографическая проверка, неподписанные ответы могут отклоняться.','DNSSEC: Request DO requests records; Require AD trusts authenticated encrypted resolvers. It is not local cryptographic validation and can reject unsigned names.'));
  newInput('privacy','p30Enabled',t('Включить настройки EDNS/ECS/DNSSEC','Enable EDNS/ECS/DNSSEC controls'),'check');
  newInput('privacy','p30ECSMode',t('EDNS Client Subnet','EDNS Client Subnet'),'select',[['inherit',t('Сохранять запрос клиента','Preserve client option')],['strip',t('Удалять ECS','Strip ECS')],['fixed',t('Фиксированный публичный адрес','Fixed public IP')]]);
  newInput('privacy','p30ECSAddress',t('Публичный IP ECS','Public ECS address'),'text');
  newInput('privacy','p30DNSSECMode','DNSSEC','select',[['off','Off'],['request_do','Request DO'],['require_ad','Require AD (encrypted DNS only)']]);
  available('dns64',t('DNS64 требует существующего NAT64: мы не устанавливаем NAT64 и не меняем маршруты. rDNS применяется только к приватным PTR.','DNS64 requires existing NAT64; RouterForge does not configure routes. Custom rDNS handles private PTR only.'));
  newInput('dns64','p31DNS64Enabled',t('Включить DNS64','Enable DNS64'),'check');
  newInput('dns64','p31DNS64Prefix',t('Префикс NAT64 /96','NAT64 /96 prefix'),'text').value='64:ff9b::/96';
  newInput('dns64','p31NAT64Confirmed',t('Подтверждаю, что NAT64 уже настроен','I confirm NAT64 already works'),'check');
  newInput('dns64','p31ReverseEnabled',t('Отдельный DNS для приватных PTR','Custom private PTR resolver'),'check');
  newInput('dns64','p31ReverseUpstream',t('DNS для rDNS','rDNS upstream'),'select',[]);
  // Visibility and service writes are controlled by app.js save/apply only.
  const originalParent = additional.parentNode;
  const originalNext = additional.nextSibling;
  const originalOpen = additional.open;
  const cacheField = cacheInput.closest('.field');
  const cacheParent = cacheField.parentNode;
  const cacheNext = cacheField.nextSibling;
  function returnTo(node, parent, next) {
    if (next && next.parentNode === parent) parent.insertBefore(node, next);
    else parent.appendChild(node);
  }
  function placement() {
    if (app.dataset.dnsMode === 'advanced') {
      service.append(additional);
      additional.open = true;
      cache.append(cacheField);
    } else {
      returnTo(cacheField, cacheParent, cacheNext);
      returnTo(additional, originalParent, originalNext);
      additional.open = originalOpen;
      card.hidden = true;
      tab.classList.remove('selected');
      tab.removeAttribute('aria-current');
      app.removeAttribute('data-p25-settings-active');
    }
  }
  tab.addEventListener('click', () => {
    if (app.dataset.dnsMode !== 'advanced') return;
    // P21 remains authoritative for native page navigation and apply semantics.
    $('p21Nav').click();
    for (const id of ['p22DevicesCard', 'p22GroupsCard', 'p22RulesCard']) $(id).hidden = true;
    card.hidden = false;
    for (const id of ['navDns', 'navLogs', 'p21Nav', 'p22NavGroups', 'p22NavRules']) {
      $(id).classList.remove('selected');
      $(id).removeAttribute('aria-current');
    }
    tab.classList.add('selected');
    tab.setAttribute('aria-current', 'page');
    app.dataset.p25SettingsActive = 'true';
    refreshIPHealth();
    const label = $('p23DraftToolbar')?.querySelector('.p22-draft-toolbar-label');
    if (label) label.textContent = t('Настройки DNS · черновик', 'DNS settings · draft');
  });
  for (const id of ['navDns', 'navLogs', 'p21Nav', 'p22NavGroups', 'p22NavRules']) {
    $(id).addEventListener('click', () => {
      card.hidden = true;
      tab.classList.remove('selected');
      tab.removeAttribute('aria-current');
      app.removeAttribute('data-p25-settings-active');
    });
  }
  $('p21Mode').addEventListener('change', placement);
  const modeObserver = new MutationObserver(placement);
  modeObserver.observe(app, {attributes:true,attributeFilter:['data-dns-mode']});
  selectSection('service');
  placement();
})();
