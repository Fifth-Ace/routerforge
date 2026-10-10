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
  // K3E: localize known preflight evidence without hiding dynamic facts.
  function localizeSafetyDetail(check) {
    var detail = String(check.detail || 'Нет данных');
    var messages = {
      'Kernel recent match is loaded; this does not prove firewall protection': 'Модуль recent загружен; это не подтверждает действующую защиту firewall.',
      'recent match is not visible; capability has not been proven': 'Модуль recent не обнаружен; поддержка не подтверждена.',
      'Cannot read kernel table registry': 'Не удалось прочитать список таблиц ядра.',
      'Kernel filter table is visible; no rule order has been verified': 'Таблица filter обнаружена, но порядок правил не проверен.',
      'Filter table is not visible in kernel table registry': 'Таблица filter не обнаружена.',
      'Keenetic NDM forwarding chain order and persistence have not been verified': 'Порядок цепочек NDM и сохранение правил после перезапуска не проверены.',
      'SSH 22/2222, active management path and an out-of-band rescue have not been verified': 'Порты SSH, действующий канал управления и резервный доступ не проверены.',
      'No timed rollback transaction has been staged or tested': 'Транзакционный откат по таймеру ещё не подготовлен и не проверен на роутере.',
      'WAN interface, protected target and NAT forwarding have not been verified': 'WAN-интерфейс, защищаемый адрес и проброс NAT не проверены.'
    };
    if (messages[detail]) return messages[detail];
    return detail
      .replace(/^Visible filter chains: /, 'Обнаружены цепочки filter: ')
      .replace(/^TCP LISTEN on port\(s\) /, 'TCP-порты в состоянии LISTEN: ')
      .replace(/^Default IPv4 route interface: /, 'Интерфейс маршрута IPv4 по умолчанию: ')
      .replace(/; insertion order and NDM persistence NOT verified/g, '; порядок правил и сохранение NDM НЕ проверены')
      .replace(/; reachability, authenticated session and rescue NOT verified/g, '; доступность, авторизация и резервный доступ НЕ проверены')
      .replace(/; physical WAN identity, NAT target and forwarding NOT verified/g, '; физический WAN, адрес NAT и проброс НЕ проверены');
  }

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
          var detail = el('small', localizeSafetyDetail(check), 'muted');
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

  document.getElementById('knock-forward-check').addEventListener('click', function () {
    var wan = document.getElementById('knock-wan').value.trim();
    var target = document.querySelector('#panel-iptables-recent [data-field="target"]').value.trim();
    var result = document.getElementById('knock-forward-result');
    if (!/^[a-zA-Z][a-zA-Z0-9_.:-]{0,14}$/.test(wan) || !/^\d+$/.test(target) || +target < 1 || +target > 65535) {
      result.textContent = 'Укажи WAN-интерфейс и корректный защищаемый порт.';
      return;
    }
    result.textContent = 'Читаем filter/FORWARD…';
    fetch(statusURL().replace(/\/status$/, '/forward-order') + '?wan=' + encodeURIComponent(wan) + '&port=' + encodeURIComponent(target), {
      credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
    }).then(function (response) {
      return response.json().then(function (data) { if (!response.ok) throw new Error(data.error || 'HTTP ' + response.status); return data; });
    }).then(function (data) {
      if (data.module !== 'port-access-manager' || data.ready_for_apply !== false || data.mutation_api !== false || data.observed_only !== true) throw new Error('Не подтверждён read-only контракт');
      result.textContent = 'FORWARD: планируемая позиция ' + data.planned_position + '. Никаких правил не применено.';
    }).catch(function (error) { result.textContent = 'Проверка не пройдена: ' + error.message; });
  });

  document.getElementById('knock-nat-check').addEventListener('click', function () {
    var wan = document.getElementById('knock-wan').value.trim();
    var publicPort = document.getElementById('knock-public-port').value.trim();
    var target = document.querySelector('#panel-iptables-recent [data-field="target"]').value.trim();
    var result = document.getElementById('knock-nat-result');
    if (!/^[a-zA-Z][a-zA-Z0-9_.:-]{0,14}$/.test(wan) ||
        !/^\d+$/.test(publicPort) || +publicPort < 1 || +publicPort > 65535 ||
        !/^\d+$/.test(target) || +target < 1 || +target > 65535) {
      result.textContent = 'Проверь WAN и оба TCP-порта.';
      return;
    }
    result.textContent = 'Читаем NAT…';
    var url = statusURL().replace(/\/status$/, '/nat-evidence');
    url += '?wan=' + encodeURIComponent(wan) + '&public=' + encodeURIComponent(publicPort) + '&target=' + encodeURIComponent(target);
    fetch(url, { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' } })
      .then(function (response) {
        return response.json().then(function (data) {
          if (!response.ok) throw new Error(data.error || 'HTTP ' + response.status);
          return data;
        });
      }).then(function (data) {
        if (data.module !== 'port-access-manager' || data.ready_for_apply !== false ||
            data.mutation_api !== false || data.observed_only !== true) throw new Error('Нарушен read-only контракт');
        result.textContent = 'DNAT найден: ' + data.destination_ip + ':' + data.target_port +
          ' · ' + data.path + '. Правила не применены.';
      }).catch(function (error) { result.textContent = 'DNAT не подтверждён: ' + error.message; });
  });

  document.getElementById('knock-check-all').addEventListener('click', function () {
    document.getElementById('knock-forward-check').click();
    document.getElementById('knock-nat-check').click();
  });

  function port(value) { return /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 65535; }
  function range(value, low, high) { return /^\d+$/.test(value) && Number(value) >= low && Number(value) <= high; }
  var previewSerial = 0;
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
    if (!errors.length && selected === 'iptables-recent') {
      var url = statusURL().replace(/\/status$/, '/knock-preview');
      var query = new URLSearchParams({first:data.sequence[0],second:data.sequence[1],third:data.sequence[2],target:data.target_port,window:data.window_seconds,ttl:data.ttl_seconds});
      var request = ++previewSerial;
      fetch(url+'?'+query.toString(), {credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}})
        .then(function(response){if(!response.ok)throw new Error('HTTP '+response.status);return response.json();})
        .then(function(plan){if(request!==previewSerial||selected!=='iptables-recent')return;
          if(plan.applied!==false||plan.hooked!==false||!Array.isArray(plan.commands))throw new Error('Неверный dry-run контракт');
          document.getElementById('preview').textContent = 'Порядок команд (НЕ ПРИМЕНЕНО):\n'+plan.commands.map(function(args,i){return (i+1)+'. '+args.join(' ');}).join('\n')+'\n\nЦепочка не подключена к NDM/FORWARD.';
        }).catch(function(err){if(request===previewSerial&&selected==='iptables-recent')document.getElementById('preview').textContent='Ошибка предпросмотра: '+err.message;});
    }
  }
  function activate(id) {
    previewSerial++;
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
  // Staging changes configuration only: never sends a firewall apply action.
  var stageMessage = document.getElementById('staged-message');
  var stageList = document.getElementById('staged-list');
  function stagedEndpoint() { return statusURL().replace(/\/status$/, '/staged-rules'); }
  function stagedRequest(method, body) {
    return fetch(stagedEndpoint(), {
      method:method,credentials:'same-origin',cache:'no-store',
      headers:{'Content-Type':'application/json','X-RouterForge-Action':'stage-rule',Accept:'application/json'},
      body: body ? JSON.stringify(body) : undefined
    }).then(function(response) {
      return response.json().then(function(data) {
        if (!response.ok) throw new Error(data.error || ('HTTP '+response.status));
        return data;
      });
    });
  }
  function loadStaged() {
    stageMessage.textContent = 'Читаем сохранённые правила…';
    stagedRequest('GET').then(function(data) {
      if (data.firewall_mutation !== false || data.ready_for_apply !== false || !Array.isArray(data.rules)) throw new Error('Небезопасный ответ API');
      stageList.textContent = '';
      data.rules.forEach(function(rule) {
        var row = el('div',undefined,'engine-card');
        row.appendChild(el('strong',rule.id));
        row.appendChild(el('p',rule.engine + ' · WAN '+rule.wan+' · '+rule.sequence.join(',')+' → '+rule.target+' · выключено','muted'));
        var remove = el('button','Удалить черновик');
        remove.type='button';
        remove.addEventListener('click',function() {
          if (!window.confirm('Удалить черновик '+rule.id+'?')) return;
          stagedRequest('DELETE',{id:rule.id}).then(loadStaged).catch(function(err){stageMessage.textContent=err.message;});
        });
        var assess = el('button','Проверить правило');
        assess.type='button';
        assess.addEventListener('click',function() {
          var output=document.getElementById('staged-assessment');
          output.textContent='Проверяем '+rule.id+'…';
          var url=statusURL().replace(/\/status$/, '/staged-assessment')+'?id='+encodeURIComponent(rule.id);
          fetch(url,{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}})
            .then(function(response){return response.json().then(function(data){if(!response.ok)throw new Error(data.error||'HTTP '+response.status);return data;});})
            .then(function(data){
              if(data.rule_id!==rule.id||data.ready_for_apply!==false||data.firewall_mutation!==false||data.deployment_blocked!==true||!Array.isArray(data.deployment_blockers)||!Array.isArray(data.rollback_preview))throw new Error('Небезопасный ответ API');
              output.textContent='Правило: '+rule.id+'\nПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО\nПричины: '+data.deployment_blockers.join('; ')+'\n\nRollback preview:\n'+data.rollback_preview.map(function(parts){return parts.join(' ');}).join('\n');
            }).catch(function(err){output.textContent='Проверка не выполнена: '+err.message;});
        });
        row.appendChild(assess);
        row.appendChild(remove);stageList.appendChild(row);
      });
      stageMessage.textContent = 'Черновиков: '+data.rules.length+' · firewall не изменён';
    }).catch(function(err){stageMessage.textContent='Сохранение пока недоступно: '+err.message;});
  }
  document.getElementById('staged-refresh').addEventListener('click',loadStaged);
  document.getElementById('staged-save').addEventListener('click',function() {
    var panel=document.getElementById('panel-iptables-recent');
    var sequence=panel.querySelector('[data-field="sequence"]').value.split(',').map(function(x){return Number(x.trim());});
    var target=Number(panel.querySelector('[data-field="target"]').value);
    var windowSeconds=Number(panel.querySelector('[data-field="window"]').value);
    var accessSeconds=Number(panel.querySelector('[data-field="ttl"]').value);
    var id=document.getElementById('staged-id').value.trim();
    var wan=document.getElementById('knock-wan').value.trim();
    if (sequence.length!==3 || sequence.some(function(x){return !Number.isInteger(x);})) {stageMessage.textContent='Нужно ровно три порта';return;}
    stagedRequest('POST',{rule:{id:id,engine:'iptables-recent',sequence:sequence,target:target,window_seconds:windowSeconds,access_seconds:accessSeconds,wan:wan,enabled:false}})
      .then(loadStaged).catch(function(err){stageMessage.textContent='Не сохранено: '+err.message;});
  });
  // Live kernel xt_recent list, independent of staged configuration.
  var liveList = document.getElementById('recent-live-list');
  var liveStatus = document.getElementById('recent-live-status');
  function refreshRecentAccess() {
    liveStatus.textContent = 'Читаем текущий список ядра…';
    fetch(statusURL().replace(/\/status$/, '/recent-access'), {credentials:'same-origin',cache:'no-store'})
      .then(function(r){return r.json().then(function(d){if(!r.ok)throw new Error(d.error||('HTTP '+r.status));return d;});})
      .then(function(data){
        liveList.textContent='';
        if (!data.available){liveStatus.textContent=data.reason||'Активный список не найден';return;}
        if(!Array.isArray(data.entries))throw new Error('Некорректный ответ');
        data.entries.forEach(function(entry){
          var row=el('div',undefined,'engine-card');
          row.appendChild(el('strong',entry.ip));
          var button=el('button','Отозвать доступ');button.type='button';
          button.addEventListener('click',function(){
            if(!window.confirm('Отозвать авторизацию для '+entry.ip+'?'))return;
            fetch(statusURL().replace(/\/status$/, '/recent-revoke'),{
              method:'POST',credentials:'same-origin',
              headers:{'Content-Type':'application/json',Accept:'application/json'},
              body:JSON.stringify({ip:entry.ip,confirm:'REVOKE'})
            }).then(function(resp){return resp.json().then(function(data){if(!resp.ok)throw new Error(data.error||'HTTP '+resp.status);return data;});})
              .then(refreshRecentAccess).catch(function(err){liveStatus.textContent='Отзыв доступа: '+err.message;});
          });
          row.appendChild(button);liveList.appendChild(row);
        });
        liveStatus.textContent='Авторизованных IP: '+data.entries.length+' · источник: xt_recent';
      }).catch(function(err){liveStatus.textContent='Не удалось получить список: '+err.message;});
  }
  document.getElementById('recent-live-refresh').addEventListener('click',refreshRecentAccess);
  refreshRecentAccess();

  loadStaged();
}());
