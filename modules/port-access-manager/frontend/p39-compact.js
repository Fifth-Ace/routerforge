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
  advanced.appendChild(service);
  advanced.appendChild(config);
  // Keep sections visible when moved into details; engine-switching handler
  // controls config.hidden and individual service cards independently.
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
})();
