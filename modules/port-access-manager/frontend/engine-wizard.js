(function () {
  'use strict';
  var preview = document.getElementById('engine-wizard-preview');
  var state = document.getElementById('engine-wizard-result');
  var plan = document.getElementById('engine-wizard-plan');
  var engine = document.getElementById('engine-config-engine');
  if (!preview || !state || !plan || !engine) return;
  function value(id) { return document.getElementById(id).value.trim(); }
  function collect() {
    var services;
    try { services = value('engine-wizard-services').split(/\r?\n/).filter(function (line) { return line.trim(); }).map(function (line) {
      var parts = line.split(',').map(function (part) { return part.trim(); });
      if (parts.length !== 3) throw Error('Ожидается имя,IPv4,порт в каждой строке');
      return { name: parts[0], address: parts[1], port: Number(parts[2]) };
    }); } catch (error) { state.textContent = 'Ошибка: ' + error.message; return; }
    var request = {
      engine: engine.value,
      interface: value('engine-wizard-interface'),
      knock: engine.value === 'knockd' ? value('engine-wizard-knock').split(',').map(Number) : [],
      sequence_seconds: engine.value === 'knockd' ? Number(value('engine-wizard-window')) : 0,
      access_seconds: Number(value('engine-wizard-ttl')),
      services: services
    };
    return request;
  }
  var lastDraftHashes = {};
  var lastValidatedRequest = null;
  preview.addEventListener('click', function () {
    var request;
    try { request = collect(); } catch (error) { state.textContent = 'Ошибка: ' + error.message; return; }
    lastValidatedRequest = null;
    state.textContent = 'Проверяем…';
    plan.textContent = '';
    preview.disabled = true;
    fetch('/api/modules/port-access-manager/engine-wizard-preview', {
      method: 'POST', credentials: 'same-origin', cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: JSON.stringify(request)
    }).then(function (response) {
      return response.json().then(function (data) {
        if (!response.ok) throw Error(data.error || 'HTTP ' + response.status);
        return data;
      });
    }).then(function (data) {
      if (data.applied !== false || data.preview_only !== true || data.engine !== request.engine) throw Error('Неверный контракт предпросмотра');
      lastValidatedRequest = request;
      state.textContent = 'Проверено · НЕ применено';
      plan.textContent = data.plan;
    }).catch(function (error) {
      state.textContent = 'Ошибка: ' + error.message;
    }).then(function () { preview.disabled = false; });
  });
  document.getElementById('engine-wizard-draft-save').addEventListener('click', function () {
    var req;
    try { req = collect(); } catch (error) { state.textContent = 'Ошибка: ' + error.message; return; }
    if (!lastValidatedRequest || JSON.stringify(req) !== JSON.stringify(lastValidatedRequest)) {
      state.textContent = 'Сначала проверь текущий план'; return;
    }
    if (!window.confirm('Сохранить отдельный черновик ' + req.engine + '? Рабочие конфигурации и firewall не изменятся.')) return;
    state.textContent = 'Сохраняем черновик…';
    fetch('/api/modules/port-access-manager/engine-wizard-draft', {
      method: 'POST', credentials: 'same-origin', cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: JSON.stringify({draft:req, base_sha256:lastDraftHashes[req.engine] || '', confirm:'SAVE_DRAFT'})
    }).then(function (resp) {return resp.json().then(function (data) {if (!resp.ok) throw Error(data.error || 'HTTP ' + resp.status); return data;});})
    .then(function (data) {if (data.applied !== false || data.service_started !== false) throw Error('Неверный ответ API'); lastDraftHashes[req.engine] = data.sha256; state.textContent = 'Черновик сохранён · защита НЕ применена';})
    .catch(function (error) {state.textContent = 'Ошибка сохранения: ' + error.message;});
  });
  document.getElementById('engine-wizard-draft-load').addEventListener('click', function () {
    var selected = engine.value;
    state.textContent = 'Загружаем черновик…';
    fetch('/api/modules/port-access-manager/engine-wizard-draft?engine=' + encodeURIComponent(selected), {credentials:'same-origin', cache:'no-store'})
      .then(function (resp) {return resp.json().then(function (data) {if (!resp.ok) throw Error(data.error || 'HTTP ' + resp.status); return data;});})
      .then(function (data) {
        if (!data.exists) {lastDraftHashes[selected]='';state.textContent = 'Черновика ещё нет';return;}
        if (data.applied !== false || data.engine !== selected || data.draft.engine !== selected) throw Error('Неверный ответ API');
        var d = data.draft;
        lastDraftHashes[selected]=data.sha256;
        document.getElementById('engine-wizard-interface').value = d.interface || '';
        document.getElementById('engine-wizard-knock').value = (d.knock || []).join(',');
        document.getElementById('engine-wizard-window').value = d.sequence_seconds || 15;
        document.getElementById('engine-wizard-ttl').value = d.access_seconds;
        document.getElementById('engine-wizard-services').value = d.services.map(function(s) {return [s.name,s.address,s.port].join(',');}).join('\n');
        lastValidatedRequest=null; plan.textContent=''; state.textContent='Черновик загружен · проверь план перед сохранением';
      }).catch(function (error) {state.textContent='Ошибка загрузки: ' + error.message;});
  });

  // End-to-end deployment is separate from preview and draft operations.
  var deployState=document.getElementById('rfpa-deploy-status');
  var credentialPanel=document.getElementById('rfpa-credentials');
  var credentialValue=document.getElementById('rfpa-credential-value');
  var deployButtons=['prepare','enable','confirm','disable','refresh'];
  var deployBusy=false;
  function deployURL(id){return '/api/modules/port-access-manager/engine-deploy'+(id?'?engine='+encodeURIComponent(id):'');}
  function deployStatus(){
    var selected=engine.value;
    fetch(deployURL(selected),{credentials:'same-origin',cache:'no-store'})
      .then(function(r){return r.json().then(function(d){if(!r.ok)throw Error(d.error||('HTTP '+r.status));return d;});})
      .then(function(d){if(engine.value!==selected)return;
        deployState.textContent = d.enabled ? (!d.running?'Защитный шлюз включён, но служба остановлена':(d.confirmed?'Защита включена и подтверждена':'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ · watchdog 90 секунд')) : (d.prepared?'Конфигурация готова · защита выключена':'Конфигурация не подготовлена');
        document.getElementById('rfpa-prepare').disabled=deployBusy||d.prepared||d.enabled;
        document.getElementById('rfpa-enable').disabled=deployBusy||!d.prepared||d.enabled;
        document.getElementById('rfpa-confirm').disabled=deployBusy||!d.enabled||d.confirmed;
        document.getElementById('rfpa-disable').disabled=deployBusy||!d.enabled;
      }).catch(function(e){deployState.textContent='Ошибка состояния: '+e.message;});
  }
  function deploy(action){
    if(deployBusy)return;
    var req=collect();if(!req)return;
    var selected=req.engine;
    if(action==='prepare' && (!lastValidatedRequest||JSON.stringify(req)!==JSON.stringify(lastValidatedRequest))){deployState.textContent='Сначала проверь текущий план';return;}
    var messages={prepare:'Создать рабочую конфигурацию '+selected+'? Существующие файлы могут быть заменены только с отдельным подтверждением.',enable:'ВКЛЮЧИТЬ защиту '+selected+'? Подтверди её после проверки с внешнего клиента, иначе watchdog откатит правила через 90 секунд.',confirm:'Подтвердить защиту и отменить автоматический откат? Сначала докажи, что доступ без авторизации закрыт, а после авторизации открыт.',disable:'Удалить только управляемые правила защиты RouterForge? Служба Entware останется в текущем состоянии.'};
    if(!window.confirm(messages[action]))return;
    deployBusy=true;deployState.textContent='Выполняется: '+action+'…';
    deployButtons.forEach(function(a){document.getElementById('rfpa-'+a).disabled=true;});
    fetch(deployURL(),{method:'POST',credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json',Accept:'application/json'},
      body:JSON.stringify({engine:selected,action:action,confirm:'APPLY',wizard:action==='prepare'?req:{},replace_existing:document.getElementById('rfpa-replace-existing').checked})})
    .then(function(r){return r.json().then(function(d){if(!r.ok)throw Error(d.error||('HTTP '+r.status));return d;});})
    .then(function(d){
      if(d.credential_once){credentialValue.textContent=d.credential_once;credentialPanel.open=true;}
      deployState.textContent='Выполнено: '+action+'. Проверяем состояние…';
    }).catch(function(e){deployState.textContent='Операция отклонена: '+e.message;})
    .then(function(){deployBusy=false;deployStatus();});
  }
  ['prepare','enable','confirm','disable'].forEach(function(action){document.getElementById('rfpa-'+action).addEventListener('click',function(){deploy(action);});});
  document.getElementById('rfpa-refresh').addEventListener('click',deployStatus);
  engine.addEventListener('change',function(){credentialValue.textContent='';credentialPanel.open=false;lastValidatedRequest=null;deployStatus();});
  deployStatus();
  window.setInterval(function(){if(!document.hidden&&!deployBusy)deployStatus();},10000);
})();
