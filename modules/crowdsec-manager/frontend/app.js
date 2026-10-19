(function () {
  'use strict';
  // P50 theme contract: read the actual RouterForge Shell tokens, not URL
  // preferences. settings.text may be an accent-like user setting while the
  // rendered Shell --text remains neutral. This iframe is same-origin.
  (function () {
    var root = document.documentElement;
    var pairs = [
      ['--bg', '--cs-bg'],
      ['--surface', '--cs-panel'],
      ['--surface-2', '--cs-panel-soft'],
      ['--border', '--cs-border'],
      ['--text', '--cs-text'],
      ['--muted', '--cs-muted'],
      ['--accent', '--cs-accent'],
      ['--good', '--cs-good'],
      ['--bad', '--cs-bad'],
      ['--warn', '--cs-warning']
    ];
    function synchronizeTheme() {
      try {
        if (!window.parent || window.parent === window) return;
        var parentRoot = window.parent.document.documentElement;
        var computed = window.parent.getComputedStyle(parentRoot);
        pairs.forEach(function (pair) {
          var value = computed.getPropertyValue(pair[0]).trim();
          if (value) root.style.setProperty(pair[1], value);
        });
        // Text always reflects what the user actually sees in the Shell.
        var realText = window.parent.getComputedStyle(window.parent.document.body).color;
        if (!computed.getPropertyValue('--text').trim() && realText) {
          root.style.setProperty('--cs-text', realText);
        }
      } catch (_) {
        // Standalone/direct opening preserves neutral CSS fallback tokens.
      }
    }
    synchronizeTheme();
    // Style changes in the Shell are communicated by the iframe lifecycle.
    window.addEventListener('pageshow', synchronizeTheme);
  })();

  var $ = function (id) { return document.getElementById(id); };
  var endpoint = '/api/modules/crowdsec-manager/';
  var busy = false;
  function txt(id, value) { $(id).textContent = String(value == null ? '—' : value); }
  function colored(id, good, yes, no) { txt(id, good ? yes : no); $(id).className = good ? 'ok' : 'fail'; }
  function msg(value, error) { txt('toast', value); $('toast').className = error ? 'fail' : 'ok'; }
  function api(path, method, data) {
    return fetch(endpoint + path, { method: method || 'GET', credentials: 'same-origin', cache: 'no-store',
      headers: data ? { 'Content-Type': 'application/json' } : {}, body: data ? JSON.stringify(data) : undefined
    }).then(function (r) { return r.json().then(function (d) { if (!r.ok || d.error) throw Error(d.error || 'HTTP ' + r.status); return d; }); });
  }
  function fill(id, items, create) {
    var host = $(id); host.textContent = '';
    if (!items.length) { host.textContent = 'Нет данных'; return; }
    items.forEach(function (item) { host.appendChild(create(item)); });
  }
  function dom(tag, value, cls) { var node = document.createElement(tag); node.textContent = value; if (cls) node.className = cls; return node; }
  function render(d) {
    var s = d.snapshot || {}, fw = s.firewall || {}, sets = s.sets || {}, hook = s.ndm_hook || {}, cfg = d.lapi || {};
    var good = d.state === 'healthy' && !!s.protected;
    $('bouncer-install').hidden = false;
    var missing = [];
    if (!s.installed) missing.push('не найден bouncer');
    if (!s.running) missing.push('bouncer не запущен');
    if (!sets.ok) missing.push('не найдены CrowdSec ipset');
    if (!fw.ok) missing.push('не подтверждён путь iptables → ipset DROP');
    if (!d.hook_executable) missing.push('нет исполняемого NDM hook');
    if (!d.init_installed) missing.push('нет init-скрипта bouncer');
    if (!cfg.exists) missing.push('нет конфигурации LAPI');
    if (!cfg.api_key_set) missing.push('нет ключа LAPI');
    if (!cfg.api_url) missing.push('не задан адрес LAPI');
    if (cfg.tls_verify === false && !d.tls_waived) missing.push('отключена проверка TLS');
    if (cfg.api_url && cfg.api_url.indexOf('http://') === 0 && !d.tls_waived) missing.push('LAPI работает без TLS');
    if (!d.lapi_probe || d.lapi_probe.indexOf('LAPI health endpoint reachable') !== 0) missing.push('LAPI health не подтверждён');
    if (cfg.error) missing.push('ошибка конфигурации LAPI');
    if (!good && !missing.length) missing.push('не подтверждён общий статус; смотри детали');
    txt('verdict', good ? (d.tls_waived ? 'Фильтрация подтверждена. ВНИМАНИЕ: TLS игнорируется по вашему согласию, соединение с LAPI не защищено или сертификат не проверяется.' : 'Фильтрация подтверждена по активным цепочкам и ipset DROP; NDM rebuild отдельно не проверялся') : 'NOT VERIFIED — ' + missing.join('; ') + '.');
    $('verdict').className = 'banner ' + (good ? 'good' : 'bad');
    colored('protection', good, d.tls_waived ? 'ACTIVE · TLS RISK' : 'ACTIVE', 'NOT VERIFIED');
    $('tls-waiver-panel').hidden = !d.tls_unsecured;
    $('tls-waiver-accept').hidden = !!d.tls_waived;
    $('tls-waiver-revoke').hidden = !d.tls_waived;
    $('tls-waiver-confirm-wrap').hidden = !!d.tls_waived;
    txt('tls-waiver-status', d.tls_waived ? 'Вы согласились игнорировать предупреждение TLS для этого адреса LAPI. Соединение остаётся небезопасным.' : 'Без TLS данные могут передаваться без шифрования или без проверки подлинности сервера.');
    txt('protection-hint', 'Installed: ' + !!s.installed + ' · running: ' + !!s.running + ' · ipset: ' + !!sets.ok);
    colored('running', s.running, 'Работает', 'Остановлен');
    txt('installed', s.installed ? 'Бинарник обнаружен' : 'Бинарник не установлен');
    colored('firewall', fw.ok, 'DROP подключён', 'Не подтверждён');
    txt('firewall-hint', fw.detail || 'Нет сведений');
    colored('hook', d.hook_executable, 'Исполняемый', 'Не готов');
    txt('hook-hint', hook.detail || 'Нет сведений');
    txt('hook-hint-long', d.hook_managed ? 'Установлен управляемый RouterForge hook' : (d.hook_executable ? 'Найден внешний hook. RouterForge его не изменяет.' : 'Hook не установлен или не исполняется: после NDM rebuild защита может пропасть.'));
    txt('chain-hint', (s.chains || []).join(' · ') || 'Нет подтверждённой связки.');
    txt('init-status', d.init_installed ? 'Скрипт /opt/etc/init.d/S99csfirewall готов' : 'Отсутствует исполняемый S99csfirewall');
    txt('updated', s.updated || '—');
    txt('key-present', cfg.api_key_set ? 'Ключ сохранён' : 'Ключ отсутствует');
    txt('tls-state', cfg.api_url && cfg.api_url.indexOf('http://') === 0 ? 'ВНИМАНИЕ: HTTP без шифрования' : (cfg.tls_verify ? 'Проверка TLS сертификата включена' : 'ВНИМАНИЕ: в bouncer конфиге отключена проверка TLS'));
    txt('lapi-probe', d.lapi_probe || 'Нет данных о LAPI');
    // Do not overwrite an in-progress user edit on every refresh.
    if (!$('lapi-url').matches(':focus') && !$('lapi-key').value && !busy) $('lapi-url').value = cfg.api_url || '';
    txt('counters', d.drop_counters || 'iptables counters недоступны');
    fill('sets', d.set_details || [], function (item) {
      var el = dom('div', '', 'set');
      el.appendChild(dom('strong', item.name));
      el.appendChild(dom('p', item.count_known ? item.members + ' IP / CIDR' : 'Число неизвестно', 'ok'));
      el.appendChild(dom('small', (item.samples || []).join(', ') || 'Нет примеров')); return el;
    });
    var keys = Object.keys(d.metrics || {}).sort();
    fill('metrics', keys, function (key) { var el = dom('div', '', 'metric'); el.appendChild(dom('code', key)); el.appendChild(dom('strong', d.metrics[key])); return el; });
    txt('metrics-state', d.metrics_error ? 'Metrics: ' + d.metrics_error : 'Локальный Prometheus endpoint опрошен');
    fill('notices', s.notes || [], function (note) { return dom('p', note); });
    $('hook-install').disabled = !!d.hook_executable && !d.hook_managed;
    $('hook-remove').disabled = !d.hook_managed;
  }
  function refresh() {
    if (busy) return;
    $('refresh').disabled = true;
    api('overview').then(render).catch(function (e) { txt('verdict', 'Нет связи с runtime: ' + e.message); $('verdict').className = 'banner bad'; })
      .then(function () { $('refresh').disabled = false; });
  }
  function action(path, data, question) {
    if (busy || !window.confirm(question)) return;
    busy = true; msg('Выполняем…');
    api(path, 'POST', data).then(function () { if (path === 'lapi') $('lapi-key').value = ''; msg('Выполнено. Для подтверждения защиты проверь состояние ниже.'); })
      .catch(function (e) { msg(e.message, true); })
      .then(function () { busy = false; refresh(); });
  }
  $('refresh').addEventListener('click', refresh);
  $('tls-waiver-accept').addEventListener('click', function () {
    if (!$('tls-waiver-confirm').checked) { msg('Сначала подтвердите, что принимаете риск соединения без проверки TLS.', true); return; }
    action('tls-waiver', { action: 'enable', confirm: 'crowdsec-tls-risk-accepted' }, 'Подтверждаю: понимаю риск незашифрованного или непроверенного соединения с LAPI и хочу не учитывать только TLS при оценке состояния. Продолжить?');
  });
  $('tls-waiver-revoke').addEventListener('click', function () {
    action('tls-waiver', { action: 'disable', confirm: 'crowdsec-tls-risk-accepted' }, 'Отменить согласие и снова учитывать TLS при оценке состояния?');
  });
  Array.prototype.forEach.call(document.querySelectorAll('[data-service]'), function (button) {
    button.addEventListener('click', function () { var a = button.getAttribute('data-service');
      action('service', { action: a, confirm: 'crowdsec-service' }, 'Выполнить ' + a + ' для /opt/etc/init.d/S99csfirewall? Это может изменить активную фильтрацию.');
    });
  });
  $('hook-install').addEventListener('click', function () {
    action('hook', { action: 'install', confirm: 'crowdsec-hook' }, 'Установить NDM hook? Он будет восстанавливать DROP-цепочку при rebuild netfilter Keenetic.');
  });
  $('hook-remove').addEventListener('click', function () {
    action('hook', { action: 'remove', confirm: 'crowdsec-hook' }, 'Удалить только управляемый RouterForge hook? После NDM rebuild защита может отключиться.');
  });
  $('lapi-form').addEventListener('submit', function (event) {
    event.preventDefault();
    var url = $('lapi-url').value.trim(), key = $('lapi-key').value;
    action('lapi', { api_url: url, api_key: key, confirm: 'crowdsec-lapi' }, 'Сохранить URL/API key в существующий config с резервной копией? После сохранения перезапусти bouncer.');
  });
  function pollInstaller() {
    api('install/status').then(function (state) {
      var release = state.release || 'unknown';
      txt('install-platform', 'Entware target: ' + (state.platform || 'unknown') + (state.supported ? ' · verified build available' : ' · нет проверенной сборки'));
      txt('install-release', 'Bouncer ' + release);
      var button = $('install-start');
      button.disabled = state.busy || state.installed || !state.supported;
      $('install-form').hidden = state.installed;
      $('install-remove').hidden = !state.managed;
      $('install-remove').disabled = state.busy;
      $('install-preserve-wrap').hidden = !state.managed;
      $('install-url').disabled = state.busy;
      $('install-key').disabled = state.busy;
      if (state.error) {
        txt('install-progress', 'FAIL · ' + state.error);
        $('install-progress').className = 'fail';
      } else {
        txt('install-progress', (state.busy ? 'LIVE · ' : state.completed ? 'PASS · ' : '') + (state.detail || (state.supported ? 'Готов к установке' : 'Нет проверенной сборки для этого ABI')));
        $('install-progress').className = state.completed ? 'ok' : 'small';
      }
      if (state.completed && !state.error) refresh();
    }).catch(function (e) { txt('install-progress', 'Статус установки: ' + e.message); });
  }
  $('install-form').addEventListener('submit', function (event) {
    event.preventDefault();
    var url = $('install-url').value.trim();
    var key = $('install-key').value;
    if (busy || !window.confirm('Установить официальный проверенный Firewall Bouncer, зависимости ipset/iptables и создать службу Entware на этом роутере?')) return;
    $('install-key').value = '';
    $('install-start').disabled = true;
    txt('install-progress', 'Отправляем задание установки…');
    api('install/start', 'POST', { api_url:url, api_key:key, confirm:'crowdsec-install' })
      .then(function () { pollInstaller(); })
      .catch(function (e) { txt('install-progress', 'FAIL · ' + e.message); $('install-start').disabled = false; });
  });
  $('install-remove').addEventListener('click', function () {
    if (!window.confirm('Удалить установленный RouterForge Firewall Bouncer? Служба будет остановлена.')) return;
    $('install-remove').disabled = true;
    txt('install-progress', 'Останавливаем и удаляем bouncer…');
    api('install/remove', 'POST', { confirm: 'crowdsec-remove', keep_config: $('install-keep-config').checked ? 'true' : 'false' })
      .then(function () { txt('install-progress', 'Bouncer удалён'); pollInstaller(); refresh(); })
      .catch(function (e) { txt('install-progress', 'FAIL · ' + e.message); pollInstaller(); });
  });
  pollInstaller();
  window.setInterval(function () { if (!document.hidden) pollInstaller(); }, 2500);
  refresh(); window.setInterval(function () { if (!document.hidden) refresh(); }, 15000);
})();
