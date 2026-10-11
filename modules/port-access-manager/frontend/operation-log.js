(function () {
  'use strict';
  var root = document.querySelector('main.pa');
  if (!root) return;
  var panel = document.createElement('section');
  panel.className = 'panel rf-pa-operation-log';
  panel.setAttribute('aria-label', 'Журнал Port Access Manager');
  panel.innerHTML = '<h2>Журнал операций · Live</h2>' +
    '<p class="help">Операции RouterForge: HTTP PASS/FAIL и попытки применения. SPA-ключи и конфигурации не записываются. Журнал хранится в памяти до перезапуска модуля, это не захват сетевых пакетов.</p>' +
    '<div class="refresh-bar"><button type="button" id="pa-log-pause">Пауза</button><button type="button" id="pa-log-copy">Копировать</button><button type="button" id="pa-log-clear">Очистить экран</button><span id="pa-log-state" role="status">Подключаемся…</span></div>' +
    '<pre id="pa-log-lines" tabindex="0" aria-label="Журнал операций" style="height:250px;max-height:45vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.5 monospace;border:1px solid var(--rf-border,#6665);border-radius:8px;padding:12px;">Ожидаем события…</pre>';
  root.appendChild(panel);
  var pre = panel.querySelector('#pa-log-lines');
  var state = panel.querySelector('#pa-log-state');
  var pause = panel.querySelector('#pa-log-pause');
  var halted = false, busy = false, seen = 0, lines = [];
  function render() {
    pre.textContent = lines.length ? lines.join('\n') : 'Событий пока нет.';
    pre.scrollTop = pre.scrollHeight;
  }
  pause.addEventListener('click', function () {
    halted = !halted;
    pause.textContent = halted ? 'Продолжить' : 'Пауза';
    state.textContent = halted ? 'Пауза' : 'Live · 2 сек.';
  });
  panel.querySelector('#pa-log-clear').addEventListener('click', function () { lines = []; render(); });
  panel.querySelector('#pa-log-copy').addEventListener('click', function () {
    if (!navigator.clipboard) { state.textContent = 'Копирование недоступно в этом контексте'; return; }
    navigator.clipboard.writeText(lines.join('\n')).then(function () { state.textContent = 'Скопировано'; }, function () { state.textContent = 'Копирование отклонено браузером'; });
  });
  function refresh() {
    if (halted || busy || document.hidden) return;
    busy = true;
    fetch('/api/modules/port-access-manager/operation-logs', {credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}})
    .then(function (r) { if (!r.ok) throw Error('HTTP ' + r.status); return r.json(); })
    .then(function (data) {
      if (!Array.isArray(data.entries) || data.scope !== 'operations') throw Error('Неверный формат API');
      data.entries.forEach(function (entry) {
        if (!Number.isSafeInteger(entry.id) || entry.id <= seen) return;
        seen = entry.id;
        lines.push([entry.at, entry.engine, entry.action, entry.status].join('  '));
      });
      if (lines.length > 500) lines = lines.slice(-500);
      render();
      state.textContent = 'Live · 2 сек. · записей: ' + data.entries.length;
    }).catch(function (e) { state.textContent = 'Ошибка связи: ' + e.message; })
    .then(function () { busy = false; });
  }
  refresh();
  setInterval(refresh, 2000);
})();
