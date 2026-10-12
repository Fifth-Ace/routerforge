(function () {
  'use strict';
  // P53: presentation and navigation only. Never clone or replace an action node.
  var workspace = document.querySelector('.rf-pa-workspace');
  var header = document.querySelector('main.pa > header');
  var selector = document.querySelector('.rf-pa-selector');
  var working = document.querySelector('.rf-pa-working');
  var summary = document.querySelector('.rf-pa-summary');
  var viewTabs = document.querySelector('.rf-pa-view-tabs');
  if (!workspace || !header || !selector || !working || !summary || !viewTabs) return;

  function el(tag, cls, value) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (value !== undefined) n.textContent = value;
    return n;
  }
  var lead = header.querySelector('p');
  if (lead) lead.textContent = 'Открывайте доступ к нужным сервисам и управляйте защитой без лишних технических шагов.';
  var notice = document.querySelector('.rf-pa-advanced-body .notice');
  if (notice) notice.classList.add('rf-pa-p53-legacy-notice');

  var hero = el('section', 'rf-pa-p53-hero');
  hero.appendChild(el('div', 'rf-pa-p53-kicker', 'ЦЕНТР ДОСТУПА'));
  hero.appendChild(el('h2', '', 'Что хотите сделать?'));
  hero.appendChild(el('p', 'rf-pa-p53-subtitle', 'Выберите действие. Все существующие режимы и технические инструменты остаются доступны.'));
  var shortcuts = el('div', 'rf-pa-p53-shortcuts');
  hero.appendChild(shortcuts);
  workspace.insertBefore(hero, summary);

  function getMode() {
    var selected = document.querySelector('.rf-pa-choice[aria-pressed="true"]');
    if (!selected) return 'odin';
    var all = Array.prototype.slice.call(document.querySelectorAll('.rf-pa-choice'));
    return all.indexOf(selected) === 1 ? 'knockd' : all.indexOf(selected) === 2 ? 'fwknopd' : 'odin';
  }
  function showProtection() {
    var tab = viewTabs.querySelector('.rf-pa-view-tab:first-child');
    if (tab && !tab.classList.contains('active')) tab.click();
  }
  function showTools() {
    var tab = viewTabs.querySelector('.rf-pa-view-tab:nth-child(2)');
    if (tab && !tab.classList.contains('active')) tab.click();
  }
  function scrollToElement(target) {
    if (!target) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
  function focusFirst(container) {
    if (!container) return;
    var control = container.querySelector('input:not([disabled]), select:not([disabled]), textarea:not([disabled])');
    if (control && !container.hidden) window.setTimeout(function () { control.focus({ preventScroll: true }); }, 200);
  }
  function shortcut(name, detail, cls, action) {
    var b = el('button', 'rf-pa-p53-shortcut ' + cls);
    b.type = 'button';
    b.appendChild(el('strong', '', name));
    b.appendChild(el('span', '', detail));
    b.addEventListener('click', action);
    shortcuts.appendChild(b);
    return b;
  }
  shortcut('Настроить доступ', 'Выбрать движок и задать параметры', 'primary', function () {
    showProtection(); scrollToElement(selector);
  });
  shortcut('Управлять защитой', 'Запуск, подтверждение, остановка', '', function () {
    showProtection();
    var target = getMode() === 'odin' ? document.getElementById('odin-manager') : document.getElementById('rfpa-deploy-panel');
    scrollToElement(target || working);
  });
  shortcut('Посмотреть доступы', 'Кто уже авторизован', '', function () {
    showProtection();
    var target = getMode() === 'odin' ? document.getElementById('odin-live-access') : document.getElementById('rfpa-deploy-panel');
    scrollToElement(target || working);
  });
  shortcut('Диагностика', 'Проверки, журнал и черновики', '', function () {
    showTools(); scrollToElement(viewTabs);
  });

  var workflow = el('div', 'rf-pa-p53-workflow');
  selector.insertAdjacentElement('afterend', workflow);
  workflow.appendChild(el('strong', '', 'Как это работает'));
  workflow.appendChild(el('p', '', '1. Выберите способ защиты   →   2. Настройте сервис   →   3. Проверьте параметры   →   4. Включите и подтвердите защиту'));

  var descriptions = {
    odin: ['odin · для проброшенного порта', 'Защитите уже настроенный в KeeneticOS проброс с помощью трёх TCP-стуков и автоматического отката.'],
    knockd: ['knockd · последовательность TCP-стуков', 'Управляйте существующей службой Entware, подготовьте конфигурацию и примените её с контролем отката.'],
    fwknopd: ['fwknopd · зашифрованный SPA', 'Один авторизованный пакет вместо серии стуков. Ключи показываются только при подготовке.']
  };
  var context = el('div', 'rf-pa-p53-context');
  var contextTitle = el('strong');
  var contextBody = el('p');
  context.appendChild(contextTitle);
  context.appendChild(contextBody);
  selector.appendChild(context);
  function updateContext() {
    var mode = getMode();
    var copy = descriptions[mode];
    contextTitle.textContent = copy[0];
    contextBody.textContent = copy[1];
    workflow.hidden = document.querySelector('.rf-pa-view-tab:nth-child(2).active') !== null;
  }
  Array.prototype.forEach.call(document.querySelectorAll('.rf-pa-choice,.rf-pa-view-tab'), function (b) {
    b.addEventListener('click', function () { window.setTimeout(updateContext, 0); });
  });
  updateContext();

  // Make operational statuses prominent, leave every action and handler in place.
  var odin = document.getElementById('odin-manager');
  if (odin) {
    odin.classList.add('rf-pa-p53-main-panel');
    var h = odin.querySelector('h2');
    if (h) h.textContent = 'Настройка и управление odin';
    var bar = odin.querySelector('.refresh-bar');
    if (bar) bar.classList.add('rf-pa-p53-action-bar');
    Array.prototype.forEach.call(odin.querySelectorAll('[data-odin]'), function (b) {
      if (b.getAttribute('data-odin') === 'enable') b.classList.add('rf-pa-p53-primary-action');
      if (b.getAttribute('data-odin') === 'confirm') b.classList.add('rf-pa-p53-confirm-action');
      if (b.getAttribute('data-odin') === 'disable') b.classList.add('rf-pa-p53-danger-action');
    });
  }
  var advancedTab = viewTabs.querySelector('.rf-pa-view-tab:nth-child(2)');
  if (advancedTab) advancedTab.textContent = 'Диагностика и расширенные инструменты';
  var protectionTab = viewTabs.querySelector('.rf-pa-view-tab:first-child');
  if (protectionTab) protectionTab.textContent = 'Управление доступом';
  var chooserTitle = selector.querySelector('h2');
  if (chooserTitle) chooserTitle.textContent = 'Выберите способ защиты';

  // Quick access to existing editor without bypassing any of its checks.
  var edit = shortcut('Редактор конфигурации', 'Существующие настройки knockd / fwknopd', 'secondary', function () {
    showProtection();
    var chosen = getMode();
    if (chosen === 'odin') {
      var selectKnock = document.querySelectorAll('.rf-pa-choice')[1];
      if (selectKnock) selectKnock.click();
    }
    var panel = document.getElementById('engine-config-panel');
    var details = panel && panel.closest('details');
    if (details) details.open = true;
    scrollToElement(panel);
    focusFirst(panel);
  });
  edit.classList.add('rf-pa-p53-secondary-shortcut');
})();
