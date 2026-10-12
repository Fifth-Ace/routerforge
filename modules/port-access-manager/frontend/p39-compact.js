(function () {
  'use strict';
  var shell = document.querySelector('.rf-pa-daemons');
  var wizard = document.getElementById('engine-wizard-panel');
  var service = document.getElementById('existing-engine-control');
  var config = document.getElementById('engine-config-panel');
  var selector = document.getElementById('engine-config-engine');
  if (!shell || !wizard || !service || !config || !selector) return;
  // Existing handler-bearing nodes are moved, not cloned or replaced.
  var intro = document.createElement('div');
  intro.className = 'rf-pa-compact-heading';
  intro.innerHTML = '<strong>Настройка защиты</strong><small>Настройка → проверка → подготовка → включение с автооткатом → подтверждение</small>';
  var originalScope = shell.querySelector('.rf-pa-engine-scope');
  if (originalScope) originalScope.insertAdjacentElement('afterend', intro);
  else shell.insertBefore(intro, shell.firstChild);
  var wizardHeading = wizard.querySelector('h2');
  if (wizardHeading) wizardHeading.textContent = 'Параметры защиты';
  var fields = wizard.querySelector('.form-grid');
  var knockFields = [document.getElementById('engine-wizard-knock'), document.getElementById('engine-wizard-window')];
  function setEngineView() {
    var spa = selector.value === 'fwknopd';
    knockFields.forEach(function (field) {
      if (!field) return;
      var label = field.closest('label');
      if (label) label.hidden = spa;
      field.disabled = spa;
    });
    if (fields) fields.classList.toggle('rf-pa-spa-fields', spa);
    if (wizardHeading) wizardHeading.textContent = spa ? 'Настройка SPA-защиты' : 'Настройка Port Knocking';
  }
  // Native select/change is driven by existing ui-rebuild.js engine switching.
  selector.addEventListener('change', setEngineView);
  var chooser = shell.parentElement && shell.parentElement.parentElement;
  if (chooser) chooser.addEventListener('click', function () { window.setTimeout(setEngineView, 0); });
  setEngineView();
  // The original app.js owns every service button and refreshes its status.
  // Only improve hierarchy, without a competing request or timer.
  var serviceRefresh = document.getElementById('existing-engine-refresh');
  if (serviceRefresh) serviceRefresh.textContent = 'Обновить состояние службы';
  var deploy = document.getElementById('rfpa-deploy-panel');
  if (deploy) {
    var h = deploy.querySelector('h3');
    if (h) h.textContent = 'Применение и откат';
    var hint = deploy.querySelector('.help');
    if (hint) hint.textContent = 'Применение изменяет firewall. После включения проверь доступ и подтверди защиту в течение 90 секунд. Иначе независимый watchdog выполнит откат.';
    var buttons = deploy.querySelector('.refresh-bar');
    if (buttons) buttons.classList.add('rf-pa-deploy-actions');
    var secret = document.getElementById('rfpa-credentials');
    if (secret) secret.classList.add('rf-pa-secrets');
  }
  var advanced = document.createElement('details');
  advanced.className = 'rf-pa-compact-advanced';
  advanced.innerHTML = '<summary>Расширенное управление службой и файлами конфигурации</summary>';
  wizard.insertAdjacentElement('afterend', advanced);
  // P47: service installation and lifecycle belong to the main workspace.
  // Move the ORIGINAL handler-bearing service panel, never clone buttons.
  shell.insertBefore(service, wizard);
  service.classList.add('rf-pa-service-workspace');
  var serviceTitle = service.querySelector('h2');
  if (serviceTitle) serviceTitle.textContent = 'Служба выбранного движка';
  var serviceHint = service.querySelector('.help');
  if (serviceHint) serviceHint.textContent = 'Установи движок через Entware, затем настрой защиту. Управление службой и включение firewall — разные операции.';
  advanced.appendChild(config);
  // Keep sections visible when moved into details; engine-switching handler
  // controls config.hidden and individual service cards independently.
  // Keep install and start/stop near configuration, advanced config editor remains folded.
  var info = document.createElement('p');
  info.className = 'help rf-pa-compact-warning';
  info.textContent = 'Запуск службы Entware и включение защиты — разные действия. Используй расширенное управление службой только при необходимости.';
  advanced.insertBefore(info, advanced.children[1] || null);
  var plan = document.getElementById('engine-wizard-plan');
  if (plan) {
    var planDetails = document.createElement('details');
    planDetails.className = 'rf-pa-plan-details';
    planDetails.innerHTML = '<summary>Технический план (без применения)</summary>';
    plan.parentNode.insertBefore(planDetails, plan);
    planDetails.appendChild(plan);
  }
  // Logs stay visible without hunting at the bottom of the whole page.
  var log = document.querySelector('.rf-pa-operation-log');
  if (log) {
    var logWrap = document.createElement('details');
    logWrap.className = 'rf-pa-log-details';
    logWrap.open = true;
    logWrap.innerHTML = '<summary>Журнал операций · Live</summary>';
    log.parentNode.insertBefore(logWrap, log);
    logWrap.appendChild(log);
    var logHeading = log.querySelector('h2');
    if (logHeading) logHeading.hidden = true;
  }
  // P46: Reuse the existing live controls and handlers; only enhance presentation.
  (function enhanceEngineWorkspace() {
    var panel = document.getElementById('engine-wizard-panel');
    var selector = document.getElementById('engine-config-engine');
    var grid = panel && panel.querySelector('.form-grid');
    if (!panel || !selector || !grid || panel.querySelector('.rf-pa-journey')) return;
    panel.classList.add('rf-pa-ux');
    var make = function(tag, cls, text) {
      var el = document.createElement(tag);
      if (cls) el.className = cls;
      if (text !== undefined) el.textContent = text;
      return el;
    };
    var journey = make('div', 'rf-pa-journey');
    [ ['01', 'Параметры', 'Настройте способ допуска'],
      ['02', 'Проверка', 'Проверьте и подготовьте'],
      ['03', 'Защита', 'Включите и подтвердите'] ].forEach(function(step) {
      var item = make('div', 'rf-pa-journey-step');
      item.appendChild(make('span', 'rf-pa-journey-num', step[0]));
      var copy = make('div');
      copy.appendChild(make('strong', '', step[1]));
      copy.appendChild(make('small', '', step[2]));
      item.appendChild(copy);
      journey.appendChild(item);
    });
    grid.parentNode.insertBefore(journey, grid);
    var top = make('div', 'rf-pa-mode-intro');
    var symbol = make('span', 'rf-pa-mode-symbol', '⇢');
    var copy = make('div', 'rf-pa-mode-copy');
    var modeName = make('strong');
    var modeHint = make('p');
    copy.appendChild(modeName);
    copy.appendChild(modeHint);
    top.appendChild(symbol);
    top.appendChild(copy);
    journey.parentNode.insertBefore(top, journey.nextSibling);
    var knock = document.getElementById('engine-wizard-knock');
    var windowInput = document.getElementById('engine-wizard-window');
    var ttl = document.getElementById('engine-wizard-ttl');
    var services = document.getElementById('engine-wizard-services');
    var interfaceInput = document.getElementById('engine-wizard-interface');
    var insight = make('div', 'rf-pa-live-insight');
    insight.setAttribute('aria-live', 'polite');
    grid.insertAdjacentElement('afterend', insight);
    var sequence = make('div', 'rf-pa-port-sequence');
    var summary = make('p', 'rf-pa-insight-summary');
    var serviceItems = make('div', 'rf-pa-service-pills');
    insight.appendChild(sequence);
    insight.appendChild(summary);
    insight.appendChild(serviceItems);
    var note = make('p', 'rf-pa-mode-security');
    insight.insertAdjacentElement('afterend', note);
    var actions = panel.querySelector('#rfpa-deploy-panel');
    if (actions) {
      actions.classList.add('rf-pa-action-surface');
      var actionHeading = make('div', 'rf-pa-action-heading');
      actionHeading.appendChild(make('span', 'rf-pa-action-dot'));
      actionHeading.appendChild(make('span', '', 'Применение и контроль доступа'));
      actions.insertBefore(actionHeading, actions.firstChild);
      var prepare = document.getElementById('rfpa-prepare');
      var enable = document.getElementById('rfpa-enable');
      var confirm = document.getElementById('rfpa-confirm');
      var disable = document.getElementById('rfpa-disable');
      [prepare, enable, confirm, disable].forEach(function(button) {
        if (button) button.classList.add('rf-pa-action-button');
      });
      if (enable) enable.classList.add('rf-pa-action-primary');
      if (confirm) confirm.classList.add('rf-pa-action-confirm');
      if (disable) disable.classList.add('rf-pa-action-danger');
      var status = document.getElementById('rfpa-deploy-status');
      if (status) status.classList.add('rf-pa-live-status');
    }
    function updateInsight() {
      var spa = selector.value === 'fwknopd';
      modeName.textContent = spa ? 'Single Packet Authorization' : 'Трёхэтапный Port Knocking';
      modeHint.textContent = spa ? 'Один криптографически подтверждённый пакет вместо открытой последовательности стуков.' : 'Доступ открывается только после правильной последовательности TCP-стуков.';
      symbol.textContent = spa ? '◇' : '⇢';
      sequence.textContent = '';
      if (spa) {
        var capsule = make('div', 'rf-pa-spa-capsule');
        capsule.appendChild(make('span', 'rf-pa-spa-icon', '◆'));
        capsule.appendChild(make('strong', '', 'Зашифрованный SPA-пакет'));
        capsule.appendChild(make('small', '', 'Ключи выдаются один раз при подготовке'));
        sequence.appendChild(capsule);
      } else {
        var ports = (knock && knock.value || '').split(',').map(function(p) { return p.trim(); }).filter(Boolean);
        ports.forEach(function(p, index) {
          if (index) sequence.appendChild(make('span', 'rf-pa-port-arrow', '→'));
          var token = make('span', 'rf-pa-port-token', p);
          if (!/^\d+$/.test(p) || Number(p) < 1 || Number(p) > 65535 || ports.indexOf(p) !== index) token.classList.add('rf-pa-port-invalid');
          sequence.appendChild(token);
        });
        if (!ports.length) sequence.appendChild(make('span', 'rf-pa-input-warning', 'Укажите TCP-порты стуков'));
      }
      var lines = (services && services.value || '').split(/\r?\n/).map(function(s) { return s.trim(); }).filter(Boolean);
      summary.textContent = (interfaceInput && interfaceInput.value.trim() || 'WAN не указан') + '  ·  ' + (spa ? 'SPA' : 'окно ' + (windowInput && windowInput.value || '—') + ' сек') + '  ·  доступ ' + (ttl && ttl.value || '—') + ' сек  ·  сервисов: ' + lines.length;
      serviceItems.textContent = '';
      lines.slice(0, 8).forEach(function(line) {
        var parts = line.split(',').map(function(p) { return p.trim(); });
        serviceItems.appendChild(make('span', 'rf-pa-service-pill' + (parts.length !== 3 ? ' rf-pa-service-invalid' : ''), parts.length === 3 ? parts[0] + ' · ' + parts[1] + ':' + parts[2] : 'Проверьте запись: ' + line));
      });
      if (lines.length > 8) serviceItems.appendChild(make('span', 'rf-pa-service-pill', 'ещё ' + (lines.length - 8)));
      note.textContent = spa ? 'SPA: сохраните полученные ключи сразу. Секреты не восстанавливаются через API и не записываются в журнал.' : 'Knockd: порты стуков не требуют DNAT. Указывайте сервисы, доступ к которым будет разрешён после последовательности.';
      panel.dataset.engineMode = spa ? 'spa' : 'knock';
    }
    [knock, windowInput, ttl, services, interfaceInput].forEach(function(input) {
      if (input) input.addEventListener('input', updateInsight);
    });
    selector.addEventListener('change', updateInsight);
    var load = document.getElementById('engine-wizard-draft-load');
    if (load) load.addEventListener('click', function() { window.setTimeout(updateInsight, 300); });
    updateInsight();
  })();

  // P48: one next action, reusing original guarded buttons. No extra mutation APIs.
  (function liveProgression() {
    var panel = document.getElementById('engine-wizard-panel');
    var select = document.getElementById('engine-config-engine');
    var before = document.querySelector('.rf-pa-journey');
    if (!panel || !select || !before || panel.querySelector('.rf-pa-next-step')) return;
    var bar = document.createElement('div');
    bar.className = 'rf-pa-next-step';
    var copy = document.createElement('div');
    var title = document.createElement('strong');
    var detail = document.createElement('p');
    var action = document.createElement('button');
    action.type = 'button';
    action.className = 'rf-pa-next-button';
    copy.appendChild(title); copy.appendChild(detail);
    bar.appendChild(copy); bar.appendChild(action);
    before.insertAdjacentElement('afterend', bar);
    var service = {};
    var deployment = {};
    var previewed = {};
    function active() { return select.value; }
    function btn(id) { return document.getElementById(id); }
    function refresh() {
      var id = active();
      var svc = service[id];
      var dep = deployment[id];
      var next = null;
      var label, hint;
      if (!svc) {
        label = 'Проверяем службу'; hint = 'Ожидаем ответ Entware';
      } else if (svc.installation && svc.installation.state === 'running') {
        label = 'Устанавливаем движок'; hint = 'Ожидаем завершения установки Entware';
      } else if (!svc.installed) {
        label = 'Установить ' + (id === 'knockd' ? 'Knockd' : 'fwknopd');
        hint = svc.installation && svc.installation.state === 'failed' ?
          ('Не удалось установить: ' + (svc.installation.error || 'ошибка Entware')) :
          'Сначала установи службу, затем подготовь конфигурацию';
        next = document.querySelector('[data-engine-action="'+id+'"][data-action="install"]');
      } else if (!dep) {
        label = 'Проверяем защиту'; hint = 'Служба найдена, ожидаем состояние шлюза';
      } else if (dep.enabled && !dep.running) {
        label = 'Служба остановлена'; hint = 'Шлюз включён, но служба не работает. Не подтверждай защиту до проверки доступа';
      } else if (dep.enabled && dep.confirmed) {
        label = 'Защита активна'; hint = 'Подтверждена. Управление отключением доступно ниже';
      } else if (dep.enabled) {
        label = 'Подтверди после внешней проверки';
        hint = 'Проверь доступ снаружи. Без подтверждения watchdog автоматически откатит правила';
        next = btn('rfpa-confirm');
      } else if (dep.prepared) {
        label = 'Включить защиту'; hint = 'Конфигурация готова. Включение запускает защитный шлюз и окно автоотката';
        next = btn('rfpa-enable');
      } else if (previewed[id]) {
        label = 'Подготовить конфигурацию'; hint = 'План проверен. Подготовка не включает firewall';
        next = btn('rfpa-prepare');
      } else {
        label = 'Проверить план'; hint = 'Проверь WAN, порты и сервисы перед подготовкой';
        next = btn('engine-wizard-preview');
      }
      title.textContent = label;
      detail.textContent = hint;
      action.hidden = !next;
      action.disabled = !next || next.disabled;
      action.textContent = label;
      action.onclick = next ? function() { if (!next.disabled) next.click(); } : null;
      bar.classList.toggle('rf-pa-next-ready', !!(dep && dep.enabled && dep.confirmed));
      var steps = before.querySelectorAll('.rf-pa-journey-step');
      var stage = dep && dep.enabled ? 2 : dep && dep.prepared ? 2 : previewed[id] ? 1 : 0;
      Array.prototype.forEach.call(steps, function(item, i) {
        item.classList.toggle('rf-pa-journey-current', i === stage);
        item.classList.toggle('rf-pa-journey-done', i < stage || !!(dep && dep.enabled && dep.confirmed));
      });
    }
    document.addEventListener('rfpa:service-state', function(ev) {
      service[ev.detail.engine] = ev.detail; refresh();
    });
    document.addEventListener('rfpa:deploy-state', function(ev) {
      deployment[ev.detail.engine] = ev.detail; refresh();
    });
    document.addEventListener('rfpa:preview-ok', function(ev) {
      previewed[ev.detail.engine] = true; refresh();
    });
    ['engine-wizard-interface','engine-wizard-knock','engine-wizard-window',
     'engine-wizard-ttl','engine-wizard-services'].forEach(function(id) {
      var input = btn(id);
      if (input) input.addEventListener('input', function() {
        previewed[active()] = false; refresh();
      });
    });
    var load = btn('engine-wizard-draft-load');
    if (load) load.addEventListener('click', function() { previewed[active()] = false; refresh(); });
    select.addEventListener('change', refresh);
    ['rfpa-prepare','rfpa-enable','rfpa-confirm','rfpa-disable'].forEach(function(id) {
      var button = btn(id);
      if (!button) return;
      new MutationObserver(refresh).observe(button, {attributes:true,attributeFilter:['disabled']});
    });
    refresh();
  })();

})();
