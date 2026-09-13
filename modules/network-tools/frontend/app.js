(function () {
  'use strict';
  var U = window.RFNet;
  var state = {
    health: null,
    summary: null,
    doctor: null,
    traceroute: null,
    route: null,
    flows: [],
    flowSource: '',
    activeProbe: null,
    histories: {
      availability: [],
      rtt: [],
      loss: [],
      sessions: []
    }
  };
  var flowLimit = 80;

  function q(id) { return document.getElementById(id); }
  function pushHistory(name, value) {
    if (!Number.isFinite(Number(value))) return;
    var list = state.histories[name];
    list.push(Number(value));
    if (list.length > 24) list.splice(0, list.length - 24);
  }
  function spark(values, bad) {
    if (!values || values.length === 0) return '<svg class="spark'+(bad?' bad':'')+'" viewBox="0 0 100 32" aria-hidden="true"></svg>';
    var vals = values.slice();
    if (vals.length === 1) vals = [vals[0], vals[0]];
    var min = Math.min.apply(null, vals), max = Math.max.apply(null, vals);
    if (max === min) { max += 1; min -= 1; }
    var pts = vals.map(function (v, i) {
      var x = i / (vals.length - 1) * 100;
      var y = 28 - ((v - min) / (max - min) * 22);
      return x.toFixed(1)+','+y.toFixed(1);
    });
    var area = '0,32 '+pts.join(' ')+' 100,32';
    return '<svg class="spark'+(bad?' bad':'')+'" viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true"><polygon class="area" points="'+area+'"></polygon><polyline class="line" points="'+pts.join(' ')+'"></polyline></svg>';
  }
  function updateKpis() {
    q('spark-availability').innerHTML = spark(state.histories.availability, false);
    q('spark-rtt').innerHTML = spark(state.histories.rtt, false);
    q('spark-loss').innerHTML = spark(state.histories.loss, true);
    q('spark-sessions').innerHTML = spark(state.histories.sessions, false);
  }
  function selectedChecks() {
    var out = [];
    if (q('check-ping').checked) out.push('ping');
    if (q('check-dns').checked) out.push('dns');
    if (q('check-http').checked) out.push('http');
    if (q('check-tcp').checked) out.push('tcp');
    return out;
  }
  function applyProfile() {
    var p = q('doctor-profile').value;
    q('check-ping').checked = true;
    q('check-dns').checked = true;
    q('check-http').checked = p === 'web';
    q('check-tcp').checked = true;
    if (p === 'dns') q('check-http').checked = false;
  }
  function resultLabel(kind) {
    return {ping:'Ping',dns:'DNS',http:'HTTP',tcp:'TCP'}[kind] || kind;
  }
  function resultIcon(kind) {
    return {ping:'⌁',dns:'◎',http:'▣',tcp:'◉'}[kind] || '•';
  }
  function resultDetail(kind, r) {
    if (!r) return 'нет данных';
    if (kind === 'ping') return r.rtt_avg_ms ? ('RTT '+Number(r.rtt_avg_ms).toFixed(1)+' ms') : ('Время '+r.duration_ms+' ms');
    if (kind === 'dns') return (r.addresses || []).join(', ') || ('Время '+r.duration_ms+' ms');
    if (kind === 'http') return r.status_code ? ('HTTP '+r.status_code+' · '+r.duration_ms+' ms') : ('Время '+r.duration_ms+' ms');
    if (kind === 'tcp') return 'Время '+r.duration_ms+' ms';
    return '';
  }
  function renderDoctor(d) {
    var checks = d.checks || [];
    var probes = d.probes || {};
    if (!checks.length) return '<div class="nt-empty">Нет результатов.</div>';
    return checks.map(function (kind) {
      var r = probes[kind] || {};
      var tone = r.ok ? '' : 'bad';
      var status = r.ok ? 'Успешно' : 'Ошибка';
      var loss = kind === 'ping' && Number.isFinite(Number(r.loss_pct)) ? ('Потери '+Number(r.loss_pct).toFixed(1)+'%') : '';
      var extra = r.error || loss || '';
      return '<div class="nt-result-row">'+
        '<span class="nt-result-icon">'+resultIcon(kind)+'</span>'+
        '<strong>'+U.esc(resultLabel(kind))+'</strong>'+
        '<span class="mono">'+U.esc(d.target || '')+'</span>'+
        U.badge(status,tone)+
        '<span class="nt-result-meta">'+U.esc(resultDetail(kind,r))+'</span>'+
        '<span class="nt-result-meta" title="'+U.esc(extra)+'">›</span>'+
      '</div>';
    }).join('');
  }
  function updateDoctorKpis(d) {
    var checks = d.checks || [];
    var probes = d.probes || {};
    var ok = 0, rtts = [], loss = null;
    checks.forEach(function (kind) {
      var r = probes[kind] || {};
      if (r.ok) ok++;
      if (Number(r.rtt_avg_ms) > 0) rtts.push(Number(r.rtt_avg_ms));
      else if (r.ok && Number(r.duration_ms) > 0 && (kind === 'tcp' || kind === 'http')) rtts.push(Number(r.duration_ms));
      if (kind === 'ping' && Number.isFinite(Number(r.loss_pct))) loss = Number(r.loss_pct);
    });
    var availability = checks.length ? ok / checks.length * 100 : 0;
    var rtt = rtts.length ? rtts.reduce(function(a,b){return a+b;},0)/rtts.length : 0;
    q('kpi-availability').textContent = availability.toFixed(1)+'%';
    q('kpi-rtt').textContent = rtt > 0 ? rtt.toFixed(0)+' ms' : '—';
    q('kpi-loss').textContent = loss == null ? '—' : loss.toFixed(1)+'%';
    pushHistory('availability', availability);
    if (rtt > 0) pushHistory('rtt', rtt);
    if (loss != null) pushHistory('loss', loss);
    updateKpis();
  }
  async function runDoctor() {
    var target = q('doctor-target').value.trim();
    var checks = selectedChecks();
    if (!checks.length) {
      U.error(q('doctor-results'), new Error('Выберите хотя бы одну проверку'));
      return;
    }
    q('doctor-results').innerHTML = '<div class="nt-empty">Выполняются bounded probes…</div>';
    try {
      var d = await U.request('/doctor', {target: target, checks: checks.join(',')});
      state.doctor = d;
      q('doctor-results').innerHTML = renderDoctor(d);
      updateDoctorKpis(d);
      q('route-target').value = target;
      q('trace-target').value = target;
      q('probe-target').value = target;
      await inspectRoute(target);
      U.notifyHeight();
    } catch (e) { U.error(q('doctor-results'), e); }
  }
  function avg(values) {
    var nums = (values || []).map(Number).filter(Number.isFinite);
    if (!nums.length) return '—';
    return (nums.reduce(function(a,b){return a+b;},0)/nums.length).toFixed(1)+' ms';
  }
  function renderTrace(data) {
    var hops = data.hops || [];
    if (!hops.length) {
      return '<div class="nt-empty">'+U.esc(data.error || 'Хопы не получены')+'</div>';
    }
    var rows = hops.map(function (h) {
      var rtts = h.rtt_ms || [];
      return '<tr><td><span class="hop-number">'+U.esc(h.hop)+'</span></td>'+
        '<td class="mono">'+U.esc(h.address || '*')+'</td>'+
        '<td>'+U.esc(rtts[0] != null ? Number(rtts[0]).toFixed(1)+' ms' : '—')+'</td>'+
        '<td>'+U.esc(rtts[1] != null ? Number(rtts[1]).toFixed(1)+' ms' : '—')+'</td>'+
        '<td>'+U.esc(rtts[2] != null ? Number(rtts[2]).toFixed(1)+' ms' : '—')+'</td>'+
        '<td>'+U.esc(Number(h.loss_pct || 0).toFixed(0)+'%')+'</td>'+
        '<td>'+U.esc(h.label || (h.hop === hops.length ? 'destination' : ''))+'</td></tr>';
    }).join('');
    return '<table><thead><tr><th>#</th><th>Хоп</th><th>RTT 1</th><th>RTT 2</th><th>RTT 3</th><th>Потери</th><th>Узел</th></tr></thead><tbody>'+rows+'</tbody></table>';
  }
  async function runTrace() {
    q('trace-results').innerHTML = '<div class="nt-empty">Трассировка выполняется…</div>';
    try {
      var d = await U.request('/traceroute', {target:q('trace-target').value.trim()});
      state.traceroute = d;
      q('trace-results').innerHTML = renderTrace(d);
      U.notifyHeight();
    } catch (e) { U.error(q('trace-results'), e); }
  }
  function routeFacts(selected, target) {
    if (!selected) return '<div class="nt-empty">Подходящий IPv4-маршрут не найден.</div>';
    var destination = selected.destination+'/'+selected.prefix;
    return '<div class="route-title">Выбранный маршрут</div>'+
      '<div class="fact-row"><span>Назначение</span><strong>'+U.esc(destination)+'</strong></div>'+
      '<div class="fact-row"><span>Следующий хоп</span><strong>'+U.esc(selected.gateway || 'direct')+'</strong></div>'+
      '<div class="fact-row"><span>Интерфейс</span><strong>'+U.esc(selected.interface)+'</strong></div>'+
      '<div class="fact-row"><span>Источник таблицы</span><strong>'+U.esc(selected.table || 'main')+'</strong></div>'+
      '<div class="fact-row"><span>Метрика</span><strong>'+U.esc(selected.metric)+'</strong></div>'+
      '<div class="fact-row"><span>Тип маршрута</span><strong>'+U.esc(selected.type || 'unicast')+'</strong></div>'+
      '<div class="fact-row"><span>Политика</span><strong>—</strong></div>'+
      '<div class="fact-row"><span>Статус</span><strong>'+U.badge('active','')+'</strong></div>';
  }
  function routeDecision(data, target) {
    var resolution = data.resolution || {};
    var selected = data.selected;
    var addresses = resolution.addresses || [];
    var steps = [
      ['Поиск назначения', addresses.length ? ('Resolved: '+addresses.join(', ')) : ('Target: '+target)],
      ['Поиск в таблице: main', selected ? ('Найден маршрут '+selected.destination+'/'+selected.prefix+' (metric '+selected.metric+')') : 'Совпадение не найдено'],
      ['Проверка политики (PBR)', 'Отдельная policy-таблица этим read-only источником не подтверждена'],
      ['Выбор следующего хопа', selected ? ('Через '+(selected.gateway || 'direct')+' ('+selected.interface+')') : '—'],
      ['Установка маршрута', selected ? 'Маршрут активен в kernel route table' : '—']
    ];
    return '<div class="route-title">Путь принятия решения</div>'+steps.map(function(s,i){
      return '<div class="decision-row"><span class="decision-number">'+(i+1)+'</span><div class="decision-copy"><strong>'+U.esc(s[0])+'</strong><small>'+U.esc(s[1])+'</small></div></div>';
    }).join('');
  }
  async function inspectRoute(target) {
    target = target || q('route-target').value.trim();
    try {
      var d = await U.request('/routes', {target:target});
      state.route = d;
      q('route-facts').innerHTML = routeFacts(d.selected, target);
      q('route-decision').innerHTML = routeDecision(d, target);
      U.notifyHeight();
    } catch (e) {
      U.error(q('route-facts'), e);
      U.error(q('route-decision'), e);
    }
  }
  function flowRow(f) {
    var source = (f.source || '')+(f.source_port ? ':'+f.source_port : '');
    var dest = (f.destination || '')+(f.destination_port ? ':'+f.destination_port : '');
    var tone = f.state === 'ESTABLISHED' || f.state === 'ACTIVE' ? '' : 'neutral';
    return '<tr><td>'+U.esc(f.seen_at || '—')+'</td><td class="mono">'+U.esc(source)+'</td><td class="mono">'+U.esc(dest)+'</td>'+
      '<td>'+U.esc(f.protocol || '—')+'</td><td>'+U.esc((f.source_port || '—')+' → '+(f.destination_port || '—'))+'</td>'+
      '<td>'+U.esc(U.fmtBytes(f.bytes || 0))+'</td><td>'+U.badge(f.state || 'ACTIVE',tone)+'</td></tr>';
  }
  function renderFlows() {
    var search = q('flow-search').value.trim().toLowerCase();
    var list = state.flows.filter(function (f) {
      if (!search) return true;
      return JSON.stringify(f).toLowerCase().indexOf(search) >= 0;
    });
    var visible = list.slice(0, flowLimit);
    q('flow-count').textContent = 'Всего: '+list.length+' сессий';
    if (!visible.length) {
      q('flow-results').innerHTML = '<div class="nt-empty">Нет соединений по текущему фильтру.</div>';
      return;
    }
    q('flow-results').innerHTML = '<table><thead><tr><th>Время</th><th>Источник</th><th>Назначение</th><th>Протокол</th><th>Порты</th><th>Объём</th><th>Состояние</th></tr></thead><tbody>'+visible.map(flowRow).join('')+'</tbody></table>';
  }
  async function refreshFlows() {
    try {
      var d = await U.request('/flows', {limit:512});
      state.flows = d.flows || [];
      state.flowSource = d.source || '';
      renderFlows();
      q('kpi-sessions').textContent = state.flows.length.toLocaleString('ru-RU');
      pushHistory('sessions', state.flows.length);
      updateKpis();
      U.notifyHeight();
    } catch (e) { U.error(q('flow-results'), e); }
  }
  async function runActiveProbe() {
    var kind = q('probe-kind').value;
    var target = q('probe-target').value.trim();
    var port = Number(q('probe-port').value || 0);
    q('probe-output').textContent = 'Выполняется '+kind+'…';
    try {
      var d = await U.request('/probe', {kind:kind,target:target,port:port});
      state.activeProbe = d;
      q('probe-output').textContent = JSON.stringify(d,null,2);
    } catch (e) { q('probe-output').textContent = 'FAIL: '+e.message; }
    U.notifyHeight();
  }
  function saveReport() {
    var report = {
      generated_at: new Date().toISOString(),
      module: 'network-tools',
      health: state.health,
      summary: state.summary,
      doctor: state.doctor,
      traceroute: state.traceroute,
      route: state.route,
      flow_source: state.flowSource,
      flows: state.flows,
      active_probe: state.activeProbe
    };
    var blob = new Blob([JSON.stringify(report,null,2)+'\n'], {type:'application/json'});
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'routerforge-network-report-'+new Date().toISOString().replace(/[:.]/g,'-')+'.json';
    document.body.appendChild(a);
    a.click();
    a.remove();
    window.setTimeout(function(){URL.revokeObjectURL(url);},1000);
  }
  function copyText(value) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(value).catch(function(){});
    }
  }
  function bindTabs() {
    Array.prototype.forEach.call(document.querySelectorAll('.nt-tab'), function (button) {
      button.onclick = function () {
        Array.prototype.forEach.call(document.querySelectorAll('.nt-tab'), function (b) { b.classList.remove('active'); });
        button.classList.add('active');
        var target = document.getElementById(button.getAttribute('data-target'));
        if (target) target.scrollIntoView({behavior:'smooth',block:'start'});
      };
    });
  }
  async function load() {
    bindTabs();
    try {
      state.health = await U.request('/health');
      state.summary = await U.request('/summary');
      q('kpi-sessions').textContent = Number(state.summary.active_sessions || 0).toLocaleString('ru-RU');
      pushHistory('sessions', Number(state.summary.active_sessions || 0));
      updateKpis();
      await Promise.all([refreshFlows(), inspectRoute('8.8.8.8')]);
    } catch (e) {
      U.error('.nt-page', e);
    }
    U.notifyHeight();
  }

  q('doctor-profile').onchange = applyProfile;
  q('doctor-run').onclick = runDoctor;
  q('doctor-copy').onclick = function(){ copyText(JSON.stringify(state.doctor || {},null,2)); };
  q('trace-run').onclick = runTrace;
  q('trace-copy').onclick = function(){ copyText(state.traceroute ? (state.traceroute.raw || JSON.stringify(state.traceroute,null,2)) : ''); };
  q('route-run').onclick = function(){ inspectRoute(); };
  q('flow-refresh').onclick = refreshFlows;
  q('flow-search').oninput = renderFlows;
  q('flow-more').onclick = function(){ flowLimit += 80; renderFlows(); U.notifyHeight(); };
  q('probe-kind').onchange = function(){
    var kind = q('probe-kind').value;
    q('probe-port').value = kind === 'http' ? '80' : kind === 'tcp' ? '53' : '0';
  };
  q('probe-run').onclick = runActiveProbe;
  q('save-report').onclick = saveReport;
  load();
}());
