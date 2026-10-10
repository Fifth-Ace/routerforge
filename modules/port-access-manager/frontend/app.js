(function () {
  'use strict';
  // Core/Antiscan theme tokens are authoritative. Never override --pa-text
  // or --pa-bg with accent-coloured query settings: CSS maps them to --rf-*.
  // Native RouterForge host tokens: accent never replaces text or muted text.
  var hostThemeTokens = ['--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted','--rf-border','--rf-border-strong','--rf-accent','--rf-accent-soft','--rf-radius-panel','--rf-radius-control','--rf-radius-card'];
  function syncHostTheme() {
    if (window.parent === window) return;
    try {
      var host = window.parent.document.documentElement;
      var style = window.parent.getComputedStyle(host);
      var own = document.documentElement;
      hostThemeTokens.forEach(function(token) {
        var value = style.getPropertyValue(token).trim();
        if (value) own.style.setProperty(token, value);
      });
      ['theme','density','radius','uiScale'].forEach(function(key) {
        if (host.dataset[key]) own.dataset[key] = host.dataset[key];
      });
    } catch (_) { /* Standalone module fallback. */ }
  }
  syncHostTheme();
  if (window.parent !== window && typeof MutationObserver !== 'undefined') {
    try {
      var hostRoot = window.parent.document.documentElement;
      new MutationObserver(syncHostTheme).observe(hostRoot, {
        attributes:true, attributeFilter:['style','class','data-theme','data-density','data-radius','data-ui-scale']
      });
    } catch (_) { /* Standalone module fallback. */ }
  }
  var labels = {
    'knockd': 'knockd',
    'fwknopd': 'fwknopd / SPA',
    'iptables-recent': 'iptables recent'
  };
  var root = document.getElementById('engines');
  var checked = document.getElementById('checked');
  var tabs = Array.prototype.slice.call(document.querySelectorAll('[data-tab]'));
  var selected = 'knockd';
  var current = {};
  // Core proxy strips /api/modules/<id> and adds /v1 itself.
  // The standalone module, in contrast, exposes /v1/status directly.
  function statusURL() {
    var path = window.location.pathname;
    var match = path.match(/^(.*\/api\/modules\/port-access-manager)\/(?:v1\/)?ui(?:\/.*)?$/);
    if (match) return match[1] + '/status';
    match = path.match(/^(.*)\/v1\/ui(?:\/.*)?$/);
    if (match) return match[1] + '/v1/status';
    return '/api/modules/port-access-manager/status';
  }
  function el(tag, text, className) {
    var node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
  }
  function renderStatus() {
    root.textContent = '';
    Object.keys(labels).forEach(function (id) {
      var data = current[id];
      var card = el('article', undefined, 'engine-card');
      card.appendChild(el('strong', labels[id]));
      if (!data) {
        card.appendChild(el('p', 'Статус неизвестен', 'muted'));
      } else if (id === 'iptables-recent') {
        card.appendChild(el('p', data.installed ? 'Поддержка ядра обнаружена' : 'Поддержка ядра не обнаружена', data.installed ? 'good' : 'muted'));
        card.appendChild(el('p', 'Наличие recent ≠ активная защита', 'muted'));
      } else {
        card.appendChild(el('p', data.installed ? 'Установлен' : 'Не установлен', data.installed ? 'good' : 'muted'));
        card.appendChild(el('p', data.running ? 'Служба работает' : 'Служба не запущена', 'muted'));
        card.appendChild(el('p', data.configuration_present ? 'Файл конфигурации найден' : 'Файл конфигурации не найден', 'muted'));
      }
      root.appendChild(card);
    });
  }
  function refresh() {
    checked.textContent = 'Получаем состояние…';
    fetch(statusURL(), { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' } })
      .then(function (response) {
        if (!response.ok) throw new Error('HTTP ' + response.status);
        var contentType = response.headers.get('content-type') || '';
        if (contentType.indexOf('application/json') < 0) throw new Error('Ожидался JSON, получен ' + contentType);
        return response.json();
      })
      .then(function (data) {
        if (data.module !== 'port-access-manager' || !Array.isArray(data.engines)) throw new Error('Неверный ответ API');
        current = {};
        data.engines.forEach(function (engine) { current[engine.id] = engine; });
        renderStatus();
        checked.textContent = 'Связь с Core: OK · режим ' + (data.mode || 'неизвестен');
      })
      .catch(function (error) {
        checked.textContent = 'Ошибка диагностики: ' + error.message;
        root.textContent = '';
        root.appendChild(el('p', 'Нет данных от службы. Настройки по-прежнему не применяются.'));
      });
  }
  var safetyChecks = document.getElementById('preflight-checks');
  var safetyVerdict = document.getElementById('preflight-verdict');
  var safetyLabels = {
    recent_match: 'iptables recent',
    filter_table: 'Таблица filter',
    ndm_forward_chain: 'Цепочки NDM / FORWARD',
    management_rescue: 'Резервный SSH-доступ',
    rollback: 'Автоматический откат',
    ndm_chain_evidence: 'Цепочки NDM (наблюдение)',
    ssh_listener_evidence: 'SSH (слушающие порты)',
    wan_route_evidence: 'IPv4-маршрут (наблюдение)',
    wan_scope: 'WAN / NAT / защищаемый порт'
  };
  function refreshPreflight() {
    safetyVerdict.textContent = 'Проверяем…';
    safetyVerdict.className = 'safety-verdict';
    safetyChecks.textContent = '';
    safetyChecks.appendChild(el('p', 'Запрашиваем read-only preflight…', 'muted'));
    var url = statusURL().replace(/\/status$/, '/preflight');
    fetch(url, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' } })
      .then(function (response) {
        if (!response.ok) throw new Error('HTTP ' + response.status);
        if ((response.headers.get('content-type') || '').indexOf('application/json') < 0) throw new Error('Некорректный тип ответа');
        return response.json();
      })
      .then(function (data) {
        if (data.module !== 'port-access-manager' || data.mutation_api !== false || data.ready_for_apply !== false || !Array.isArray(data.checks)) {
          throw new Error('Неподтверждённый read-only контракт');
        }
        safetyVerdict.textContent = 'ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО';
        safetyVerdict.className = 'safety-verdict safety-blocked';
        safetyChecks.textContent = '';
        data.checks.forEach(function (check) {
          var row = el('div', undefined, 'safety-check');
          var heading = el('strong', safetyLabels[check.id] || check.id);
          var state = el('span', check.state === 'present' ? 'Обнаружено' : (check.state === 'observed' ? 'Наблюдается' : 'Требует проверки'), check.state === 'present' ? 'safety-present' : (check.state === 'observed' ? 'safety-present' : 'safety-warning'));
          var detail = el('small', check.detail || 'Нет данных', 'muted');
          row.appendChild(heading);
          row.appendChild(state);
          row.appendChild(detail);
          safetyChecks.appendChild(row);
        });
      })
      .catch(function (error) {
        safetyVerdict.textContent = 'ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО';
        safetyVerdict.className = 'safety-verdict safety-blocked';
        safetyChecks.textContent = '';
        safetyChecks.appendChild(el('p', 'Preflight недоступен: ' + error.message + '. Применение недоступно.', 'invalid'));
      });
  }
  document.getElementById('preflight-refresh').addEventListener('click', refreshPreflight);
  refreshPreflight();

  function port(value) { return /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 65535; }
  function range(value, low, high) { return /^\d+$/.test(value) && Number(value) >= low && Number(value) <= high; }
  function renderPreview() {
    var panel = document.getElementById('panel-' + selected);
    var inputs = panel.querySelectorAll('[data-field]');
    var data = { engine: selected, operation: 'preview-only', applied: false };
    var errors = [];
    Array.prototype.forEach.call(inputs, function (input) {
      var name = input.getAttribute('data-field');
      var value = input.value.trim();
      if (name === 'sequence') {
        var steps = value.split(',').map(function (entry) { return entry.trim(); });
        if (steps.length !== 3 || !steps.every(port) || new Set(steps).size !== steps.length) {
          errors.push('Укажи три разных TCP-порта в диапазоне 1–65535.');
        } else { data.sequence = steps.map(Number); }
      } else if (name === 'target') {
        if (!port(value)) errors.push('Защищаемый порт должен быть от 1 до 65535.');
        else data.target_port = Number(value);
      } else {
        var min = name === 'window' ? 5 : 30;
        var max = name === 'window' ? 300 : 3600;
        if (!range(value, min, max)) errors.push('Некорректное значение ' + name + ': допустимо ' + min + '–' + max + ' секунд.');
        else data[name + '_seconds'] = Number(value);
      }
    });
    if (data.sequence && data.sequence.indexOf(data.target_port) >= 0) errors.push('Открываемый порт не должен совпадать с портами стуков.');
    var validation = document.getElementById('validation');
    validation.textContent = errors.length ? errors.join(' ') : 'Параметры корректны для локального предпросмотра. Проверка сети и правил firewall не выполнялась.';
    validation.className = errors.length ? 'invalid' : 'good';
    document.getElementById('preview').textContent = errors.length ? 'Предпросмотр недоступен: исправь параметры.' : JSON.stringify(data, null, 2);
  }
  function activate(id) {
    selected = id;
    tabs.forEach(function (tab) {
      var active = tab.getAttribute('data-tab') === id;
      tab.setAttribute('aria-selected', String(active));
      tab.tabIndex = active ? 0 : -1;
      document.getElementById('panel-' + tab.getAttribute('data-tab')).hidden = !active;
    });
    renderPreview();
  }
  tabs.forEach(function (tab, index) {
    tab.addEventListener('click', function () { activate(tab.getAttribute('data-tab')); });
    tab.addEventListener('keydown', function (event) {
      if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
      event.preventDefault();
      var delta = event.key === 'ArrowRight' ? 1 : -1;
      var next = tabs[(index + delta + tabs.length) % tabs.length];
      activate(next.getAttribute('data-tab'));
      next.focus();
    });
  });
  Array.prototype.forEach.call(document.querySelectorAll('[data-field]'), function (input) {
    input.addEventListener('input', renderPreview);
  });
  document.getElementById('refresh').addEventListener('click', refresh);
  activate(selected);
  refresh();
}());
