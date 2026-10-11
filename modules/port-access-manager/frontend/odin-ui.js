(function () {
  'use strict';
  var root = document.getElementById('odin-manager');
  if (!root) return;
  var status = document.getElementById('odin-status');
  var endpoint = '/api/modules/port-access-manager/odin';
  var input = function (name) { return document.getElementById('odin-' + name); };
  var actions = ['save', 'enable', 'confirm', 'disable', 'refresh'];
  var buttons = {};
  actions.forEach(function (action) { buttons[action] = root.querySelector('[data-odin="' + action + '"]'); });
  var busy = false;
  var lastData = null;
  var localDeadline = 0;
  var lastEnabled = false;
  var lastConfirmed = false;
  var deadlineRefreshRequested = false;
  // Paint seconds without mutating the DOM every second. The containing
  // RouterForge web view observes DOM changes to size its embedded page.
  var countdown = document.createElement('canvas');
  countdown.id = 'odin-watchdog-countdown';
  countdown.className = 'odin-countdown';
  countdown.width = 560;
  countdown.height = 24;
  countdown.style.width = 'min(100%, 560px)';
  countdown.style.height = '24px';
  countdown.style.display = 'block';
  countdown.setAttribute('role', 'img');
  countdown.setAttribute('aria-label', 'Watchdog countdown');
  status.insertAdjacentElement('afterend', countdown);
  function paintCountdown(message) {
    var ctx = countdown.getContext('2d');
    if (!ctx) return;
    ctx.clearRect(0, 0, countdown.width, countdown.height);
    ctx.font = '12px "Segoe UI", Arial, sans-serif';
    ctx.fillStyle = getComputedStyle(countdown).color;
    ctx.fillText(message, 0, 17, countdown.width);
  }

  function request(method, body) {
    return fetch(endpoint, {
      method: method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: body ? JSON.stringify(body) : undefined
    }).then(function (resp) {
      return resp.text().then(function (raw) {
        var data;
        try { data = raw ? JSON.parse(raw) : {}; }
        catch (e) { throw Error('Некорректный ответ сервера (HTTP ' + resp.status + ')'); }
        if (!resp.ok) throw Error(data.error || data.message || ('HTTP ' + resp.status));
        return data;
      });
    });
  }
  function settings() {
    return {
      wan: input('wan').value.trim(),
      ip: input('ip').value.trim(),
      port: Number(input('port').value),
      knock: [Number(input('k1').value), Number(input('k2').value), Number(input('k3').value)],
      window: Number(input('window').value),
      ttl: Number(input('ttl').value)
    };
  }
  function validate(s) {
    var ipParts = s.ip.split('.');
    if (!/^[a-zA-Z0-9_.:-]+$/.test(s.wan)) return 'Укажи корректный WAN-интерфейс';
    if (ipParts.length !== 4 || ipParts.some(function (part) { return !/^\d{1,3}$/.test(part) || Number(part) > 255; })) return 'Некорректный IPv4 сервиса';
    var ports = s.knock.concat([s.port]);
    if (ports.some(function (p) { return !Number.isInteger(p) || p < 1 || p > 65535; })) return 'Порты должны быть от 1 до 65535';
    if (new Set(ports).size !== ports.length) return 'Порты стуков и сервиса должны различаться';
    if (!Number.isInteger(s.window) || s.window < 5 || s.window > 300) return 'Окно стуков: 5–300 секунд';
    if (!Number.isInteger(s.ttl) || s.ttl < 30 || s.ttl > 86400) return 'Доступ: 30–86400 секунд';
    return '';
  }
  function updateCountdown() {
    if (!lastData || !lastData.enabled || lastData.confirmed) { paintCountdown(''); return; }
    if (!localDeadline) { paintCountdown('Срок watchdog неизвестен: обнови состояние. Не подтверждай защиту без проверки.'); return; }
    var seconds = Math.max(0, Math.ceil((localDeadline - Date.now()) / 1000));
    paintCountdown(seconds ? ('До автоматического отката: приблизительно ' + seconds + ' сек.') : 'Срок подтверждения истёк. Проверяем состояние…');
    if (seconds === 0 && !busy && !deadlineRefreshRequested) { deadlineRefreshRequested = true; refresh(); }
  }
  function render(data, fill) {
    var enabled = !!data.enabled;
    var confirmed = !!data.confirmed;
    var changed = !lastData || enabled !== lastEnabled || confirmed !== lastConfirmed;
    var serverDeadline = Number(data.watchdog_deadline_unix) || 0;
    var serverNow = Number(data.server_unix) || 0;
    if (!enabled || confirmed) localDeadline = 0;
    else if (serverDeadline > 0 && serverNow > 0) localDeadline = Date.now() + Math.max(0, serverDeadline - serverNow) * 1000;
    else localDeadline = 0;
    if (localDeadline > Date.now()) deadlineRefreshRequested = false;
    lastEnabled = enabled;
    lastConfirmed = confirmed;
    lastData = data;
    status.textContent = enabled ? (data.confirmed ? 'Защита включена · подтверждена' : 'Защита включена · ожидает подтверждения') : 'Защита выключена';
    status.className = enabled ? (data.confirmed ? 'odin-state odin-state-on' : 'odin-state odin-state-pending') : 'odin-state';
    if (fill && data.configured && data.settings) {
      var s = data.settings;
      ['wan', 'ip', 'port', 'window', 'ttl'].forEach(function (name) { input(name).value = s[name]; });
      ['k1', 'k2', 'k3'].forEach(function (name, i) { input(name).value = s.knock[i]; });
    }
    buttons.save.disabled = busy || enabled;
    buttons.enable.disabled = busy || enabled || !data.configured;
    buttons.disable.disabled = busy || !enabled;
    buttons.confirm.disabled = busy || !enabled || !!data.confirmed;
    buttons.refresh.disabled = busy;
    updateCountdown();
    if (changed) document.dispatchEvent(new Event('routerforge-odin-state-changed'));
  }
  var firstLoad = true;
  function refresh() {
    return request('GET').then(function (data) { render(data, firstLoad); firstLoad = false; return data; }).catch(function (e) {
      status.textContent = 'Ошибка получения состояния: ' + e.message;
      buttons.enable.disabled = true;
      buttons.confirm.disabled = true;
      document.dispatchEvent(new Event('routerforge-odin-state-changed'));
      return null;
    });
  }
  function setBusy(on) {
    busy = on;
    actions.forEach(function (a) { buttons[a].disabled = on; });
    if (!on && lastData) render(lastData, false);
  }
  var labels = {
    save: 'Сохранить настройки',
    enable: 'Включить защиту',
    confirm: 'Подтвердить защиту',
    disable: 'Выключить защиту'
  };
  actions.forEach(function (action) {
    buttons[action].addEventListener('click', function () {
      if (busy) return;
      if (action === 'refresh') { refresh(); return; }
      if (action === 'enable' && !lastData.configured) return;
      var s = settings();
      if (action === 'save') { var reason = validate(s); if (reason) { status.textContent = reason; return; } }
      var message = action === 'enable'
        ? 'Включить защиту odin? Неправильная конфигурация может закрыть доступ. Без подтверждения watchdog откатит изменения через 90 секунд.'
        : action === 'confirm'
          ? 'Подтвердить защиту? Автооткат через 90 секунд будет отменён. Убедись, что доступ по стукам проверен.'
          : action === 'disable'
            ? 'Выключить защиту odin и убрать её правила из firewall?'
            : 'Сохранить настройки odin?';
      if (!window.confirm(message)) return;
      setBusy(true);
      status.textContent = 'Выполняется: ' + labels[action] + '…';
      request('POST', { action: action, confirm: 'APPLY', settings: s }).then(function () {
        if (action === 'enable') localDeadline = 0; // Deadline comes from server, not the browser.
        if (action === 'disable' || action === 'confirm') localDeadline = 0;
        return request('GET');
      }).then(function (data) { setBusy(false); render(data, true); }).catch(function (e) {
        setBusy(false);
        status.textContent = 'Ошибка: ' + e.message + '. Обнови состояние перед повторной операцией.';
        refresh();
      });
    });
  });
  refresh();
  window.setInterval(function () { updateCountdown(); }, 1000);
  window.setInterval(function () { if (!busy && !document.hidden) refresh(); }, 10000);
})();
