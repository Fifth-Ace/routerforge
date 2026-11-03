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
    ['ip', 'IP Intelligence', t('Планируется', 'Planned')],
    ['upstream', t('Выбор DNS', 'Upstream selection'), t('Планируется', 'Planned')],
    ['security', t('Защита запросов', 'Query protection'), t('Планируется', 'Planned')],
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
  planned('ip', t(
    'P24.0 содержит девять алгоритмов выбора IP. Обработка DNS-ответов, мониторинг адресов и сохранение настроек будут добавлены отдельно.',
    'P24.0 contains nine IP ordering strategies. Response integration, health checks and settings persistence will follow.'),
    ['Original', 'Round-Robin', 'Random', 'Fastest IP', 'Health-Aware', 'Adaptive', 'Sticky IP', 'Weighted', 'Healthy Only']);
  planned('upstream', t(
    'Режимы DNSProxy будут согласованы с уже работающими правилами RouterForge и резервными цепочками.',
    'DNSProxy upstream modes must coexist with RouterForge routing policies and failover chains.'),
    ['Load balance', 'Parallel', 'Fastest address', t('Тайм-ауты и резервные серверы', 'Timeouts and fallbacks')]);
  planned('security', t(
    'Защита от избыточных и нежелательных DNS-запросов без вмешательства в сетевой фильтр Keenetic.',
    'DNS request protection without changing the Keenetic packet filter.'),
    [t('Ограничение частоты запросов', 'Rate limiting'), 'Refuse ANY', 'Bogus NXDOMAIN']);
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
