(function () {
  'use strict';
  var main=document.querySelector('main.pa');
  var header=main&&main.querySelector('header');
  if(!main||!header||!document.getElementById('odin-manager'))return;
  function el(tag,klass,text){var n=document.createElement(tag);if(klass)n.className=klass;if(text!==undefined)n.textContent=text;return n;}
  function move(id,to){var n=document.getElementById(id);if(n&&to)to.appendChild(n);return n;}
  var intro=header.querySelector('p');
  if(intro)intro.textContent='Выберите способ защиты, настройте доступ и управляйте им в одном месте.';
  var shell=el('div','rf-pa-workspace');
  header.insertAdjacentElement('afterend',shell);
  var summary=el('section','rf-pa-summary');shell.appendChild(summary);
  var heading=el('div','rf-pa-summary-heading');summary.appendChild(heading);
  heading.appendChild(el('strong','', 'Состояние защиты'));
  var refresh=el('button','','Обновить состояние');refresh.type='button';heading.appendChild(refresh);
  var metrics=el('div','rf-pa-metrics');summary.appendChild(metrics);
  function metric(label){var wrap=el('div','rf-pa-metric'),title=el('span','',label),val=el('b','', 'Проверяем…');wrap.appendChild(title);wrap.appendChild(val);metrics.appendChild(wrap);return val;}
  var engineMetric=metric('Выбранный движок'),stateMetric=metric('Защита'),configMetric=metric('Конфигурация'),ndmMetric=metric('NDM / подтверждение');
  var chooser=el('section','rf-pa-selector');shell.appendChild(chooser);
  var activeView='protection';
  var viewTabs=el('nav','rf-pa-view-tabs');
  viewTabs.setAttribute('aria-label','Port Access Manager');
  var protectTab=el('button','rf-pa-view-tab','\u0417\u0430\u0449\u0438\u0442\u0430');
  var toolsTab=el('button','rf-pa-view-tab','\u0414\u043e\u043f. \u0438\u043d\u0441\u0442\u0440\u0443\u043c\u0435\u043d\u0442\u044b');
  protectTab.type='button';toolsTab.type='button';
  viewTabs.appendChild(protectTab);viewTabs.appendChild(toolsTab);
  shell.insertBefore(viewTabs,chooser);
  chooser.appendChild(el('h2','','Способ защиты'));
  var choices=el('div','rf-pa-choices');chooser.appendChild(choices);
  var options=[{id:'odin',name:'odin · Keenetic',desc:'Без дополнительного демона'},{id:'knockd',name:'knockd',desc:'Классический Port Knocking'},{id:'fwknopd',name:'fwknopd / SPA',desc:'Один зашифрованный пакет'}];
  var buttons={};
  options.forEach(function(o){var b=el('button','rf-pa-choice');b.type='button';b.setAttribute('aria-pressed','false');b.appendChild(el('strong','',o.name));b.appendChild(el('small','',o.desc));choices.appendChild(b);buttons[o.id]=b;b.addEventListener('click',function(){select(o.id);});});
  var working=el('section','rf-pa-working');shell.appendChild(working);
  var odin=move('odin-manager',working);
  var daemons=el('div','rf-pa-daemons');working.appendChild(daemons);
  var service=move('existing-engine-control',daemons);
  var config=move('engine-config-panel',daemons);
  var wizard=move('engine-wizard-panel',daemons);
  var access=move('recent-live-panel',shell);
  var advanced=el('section','rf-pa-advanced');shell.appendChild(advanced);advanced.hidden=true;
  var summaryNode=el('h2','','Инженерное · диагностика, предпросмотр и черновики');advanced.appendChild(summaryNode);
  var advancedBody=el('div','rf-pa-advanced-body');advanced.appendChild(advancedBody);
  // Move original content instead of rebuilding it: all existing handlers retain their nodes.
  Array.prototype.slice.call(main.children).forEach(function(node){
    if(node===header||node===shell||node.tagName==='SCRIPT')return;
    advancedBody.appendChild(node);
  });
  // odin owns KKD; legacy recent UI belongs to the engineering section.
  var odinAccess=el('section','panel rf-pa-odin-access');
  odinAccess.id='odin-live-access';
  var odinAccessTitle=el('h2','','Авторизованные IP · odin');odinAccess.appendChild(odinAccessTitle);
  odinAccess.appendChild(el('p','help','Текущие разрешения из KKD. Истечение доступа проверяется ядром; точное оставшееся время недоступно.'));
  var accessTools=el('div','refresh-bar');odinAccess.appendChild(accessTools);
  var accessRefresh=el('button','','Обновить IP');accessRefresh.type='button';accessTools.appendChild(accessRefresh);
  var accessState=el('span','muted','Ожидаем ответ…');accessState.setAttribute('role','status');accessTools.appendChild(accessState);
  var accessRows=el('div','rf-pa-access-list');odinAccess.appendChild(accessRows);
  shell.insertBefore(odinAccess,advanced);
  if(access)advancedBody.appendChild(access);
  var lastAccessSignature=null;
  function refreshOdinAccess(){
    accessState.textContent='Читаем KKD…';
    fetch('/api/modules/port-access-manager/odin-access',{credentials:'same-origin',cache:'no-store'})
    .then(function(r){return r.json().then(function(d){if(!r.ok)throw Error(d.error||('HTTP '+r.status));return d;});})
    .then(function(d){
      if(d.available&&(d.list!=='KKD'||!Array.isArray(d.entries)))throw Error('Неверный ответ API');
      var ips=d.available?d.entries.filter(function(e){return typeof e.ip==='string';}).map(function(e){return e.ip;}):[];
      var signature=JSON.stringify([!!d.available,ips,d.reason||'']);
      if(signature!==lastAccessSignature){
        lastAccessSignature=signature;
        accessRows.textContent='';
        if(d.available){
          if(!ips.length)accessRows.appendChild(el('p','muted','Авторизованных IP пока нет.'));
          ips.forEach(function(ip){accessRows.appendChild(el('div','rf-pa-access-ip',ip));});
        }
      }
      accessState.textContent=d.available?('Активных записей: '+ips.length):(d.reason||'Авторизации недоступны');
    }).catch(function(err){accessRows.textContent='';accessState.textContent='Ошибка чтения: '+err.message;});
  }
  accessRefresh.addEventListener('click',refreshOdinAccess);
  document.addEventListener('routerforge-odin-state-changed',function(){if(current==='odin')refreshOdinAccess();});
  var legacyNotice=document.getElementById('preflight-title');
  if(legacyNotice)legacyNotice.textContent='Проверка экспериментальных сценариев (не относится к проверенному odin)';
  var legacySafety=document.querySelector('#safety-preflight .help');
  if(legacySafety)legacySafety.textContent='Read-only диагностика экспериментального iptables recent. Статус этого preflight не блокирует работающий odin.';
  var oldFooter=advancedBody.querySelector('footer');
  if(oldFooter)oldFooter.textContent='Экспериментальные черновики и preflight не применяют firewall. Управление odin находится в основном разделе.';
  // P35D: keep legacy tools intact, but place dense read-only diagnostics in an optional nested panel.
  var diagPanel=document.getElementById('safety-preflight');
  if(diagPanel&&diagPanel.parentNode===advancedBody){
    var diagFold=el('details','rf-pa-diag-fold');
    diagFold.appendChild(el('summary','','Дополнительные проверки NDM и экспериментальных правил'));
    advancedBody.insertBefore(diagFold,diagPanel);
    diagFold.appendChild(diagPanel);
  }
  var preview=document.querySelector('.rf-pa-advanced-body .preview');
  if(preview&&preview.parentNode===advancedBody){
    var previewFold=el('details','rf-pa-diag-fold');
    previewFold.appendChild(el('summary','','Технический предпросмотр конфигурации'));
    advancedBody.insertBefore(previewFold,preview);
    previewFold.appendChild(preview);
  }
  var existingCards=service?service.querySelectorAll('.engine-card'):[];
  var selector=document.getElementById('engine-config-engine');
  var current='odin';
  // P35D.1: engine-specific presentation; existing service handlers stay attached.
  var engineScope=el('div','rf-pa-engine-scope');
  daemons.insertBefore(engineScope,daemons.firstChild);
  var daemonTitle=el('strong','','');engineScope.appendChild(daemonTitle);
  var daemonStatus=el('span','rf-pa-engine-status','Проверяем службу…');engineScope.appendChild(daemonStatus);
  var daemonDetails=el('p','muted','');engineScope.appendChild(daemonDetails);
  var daemonMessage=el('p','rf-pa-daemon-hint','Для этих движков требуется установленный и настроенный upstream-сервис Entware. RouterForge не устанавливает их автоматически.');
  daemons.insertBefore(daemonMessage,service);
  var odinAlert=el('div','rf-pa-odin-alert');
  odinAlert.setAttribute('role','status');
  odinAlert.hidden=true;
  if(odin)odin.insertBefore(odinAlert,odin.firstChild);
  function refreshOdinAlert(){
    fetch('/api/modules/port-access-manager/odin',{credentials:'same-origin',cache:'no-store'})
    .then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json();})
    .then(function(d){
      if(!d.enabled||d.confirmed){odinAlert.hidden=true;odinAlert.textContent='';return;}
      odinAlert.textContent='Внимание: защита ожидает подтверждения. Сначала проверь доступ после стуков, затем нажми «Подтвердить защиту». Без подтверждения независимый watchdog откатит правила.';
      odinAlert.hidden=false;
    }).catch(function(){odinAlert.hidden=true;});
  }
  // Legacy scratch tools do not belong to any production engine.
  var legacyDev=el('details','rf-pa-legacy-dev');
  legacyDev.appendChild(el('summary','','Архивные эксперименты: iptables recent и черновики (не применяется)'));
  var legacyBody=el('div','rf-pa-legacy-body');legacyDev.appendChild(legacyBody);
  var legacyIds=['staging-panel','recent-live-panel','safety-preflight'];
  var legacyTabRoot=advancedBody.querySelector('.tabs');
  if(legacyTabRoot)legacyBody.appendChild(legacyTabRoot);
  ['panel-knockd','panel-fwknopd','panel-iptables-recent'].forEach(function(id){var node=document.getElementById(id);if(node)legacyBody.appendChild(node);});
  legacyIds.forEach(function(id){var node=document.getElementById(id);if(node&&node.parentNode===advancedBody)legacyBody.appendChild(node);});
  // Keep other engineering controls accessible, but out of ordinary engine settings.
  advancedBody.appendChild(legacyDev);
  summaryNode.textContent='Дополнительные инструменты · только для разработчиков';
  function refreshDaemonIdentity(id){
    var name=id==='knockd'?'knockd':'fwknopd / SPA';
    daemonTitle.textContent=name;
    daemonDetails.textContent=id==='knockd'?'TCP-стуки обслуживает внешняя служба knockd. Настройки odin здесь не используются.':'Зашифрованную SPA-авторизацию обслуживает внешняя служба fwknopd. TCP-стуки odin здесь не используются.';
    daemonStatus.textContent='Проверяем установленную службу…';
    fetch('/api/modules/port-access-manager/engine-service?engine='+encodeURIComponent(id),{credentials:'same-origin',cache:'no-store'})
    .then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json();})
    .then(function(d){if(current!==id)return;daemonStatus.textContent=!d.installed?'Не установлен':(d.running?'Запущен':'Установлен · остановлен');})
    .catch(function(){if(current===id)daemonStatus.textContent='Состояние службы неизвестно';});
  }
  function select(id){
    current=id;
    options.forEach(function(o){var active=o.id===id;buttons[o.id].setAttribute('aria-pressed',String(active));buttons[o.id].classList.toggle('active',active);});
    if(odin)odin.hidden=id!=='odin';
    if(daemons)daemons.hidden=id==='odin';
    // The original RF_KNOCK_AUTH list is a legacy experiment, never odin access.
    if(access)access.hidden=true;
    if(advanced)advanced.hidden=activeView!=='tools';
    if(legacyDev)legacyDev.open=false;
    if(id!=='odin'){
      refreshDaemonIdentity(id);
      var oldTab=document.querySelector('[data-tab="'+id+'"]');
      if(oldTab)oldTab.click();
    }
    odinAccess.hidden=id!=='odin';
    Array.prototype.forEach.call(existingCards,function(card,i){card.hidden=(id==='odin'||(i===0)!==(id==='knockd'));});
    if(config)config.hidden=id==='odin';
    if(selector&&id!=='odin'){
      selector.value=id;
      selector.dispatchEvent(new Event('change',{bubbles:true}));
    }
    engineMetric.textContent=options.filter(function(o){return o.id===id;})[0].name;
    update();
  }
  function update(){
    if(current==='odin'){
      fetch('/api/modules/port-access-manager/odin',{credentials:'same-origin',cache:'no-store'})
      .then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json();})
      .then(function(data){
        stateMetric.textContent=data.enabled?(data.confirmed?'Включена':'Ожидает подтверждения'):'Выключена';
        configMetric.textContent=data.configured?'Сохранена':'Не настроена';
        ndmMetric.textContent=data.enabled?(data.confirmed?'Подтверждено':(data.watchdog_deadline_unix&&data.server_unix?'Watchdog · '+Math.max(0,data.watchdog_deadline_unix-data.server_unix)+' сек.':'Watchdog · срок неизвестен')):'Не активен';
      }).catch(function(){stateMetric.textContent='Нет связи с API';configMetric.textContent='—';ndmMetric.textContent='—';});
    }else{
      fetch('/api/modules/port-access-manager/engine-service?engine='+encodeURIComponent(current),{credentials:'same-origin',cache:'no-store'})
      .then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json();})
      .then(function(data){stateMetric.textContent=!data.installed?'Не установлен':(data.running?'Запущен':'Остановлен');configMetric.textContent=data.installed?'Сервис установлен':'Сервис не установлен';ndmMetric.textContent=data.installed?'Внешняя служба Entware':'Нет установленной службы';})
      .catch(function(){stateMetric.textContent='Нет связи с API';configMetric.textContent='—';ndmMetric.textContent='—';});
    }
  }
  refresh.addEventListener('click',function(){update();var existing=document.querySelector('[data-odin="refresh"]');if(current==='odin'&&existing)existing.click();else{var control=document.getElementById('existing-engine-refresh');if(control)control.click();}});
  document.addEventListener('routerforge-odin-state-changed',function(){if(current==='odin'){update();refreshOdinAlert();}});
  function switchView(view){
    activeView=view;
    var showTools=view==='tools';
    chooser.hidden=showTools;
    working.hidden=showTools;
    odinAccess.hidden=showTools||current!=='odin';
    advanced.hidden=!showTools;
    protectTab.classList.toggle('active',!showTools);
    toolsTab.classList.toggle('active',showTools);
    protectTab.setAttribute('aria-current',showTools?'false':'page');
    toolsTab.setAttribute('aria-current',showTools?'page':'false');
  }
  protectTab.addEventListener('click',function(){switchView('protection');});
  toolsTab.addEventListener('click',function(){switchView('tools');});
  select('odin');
  switchView('protection');
  refreshOdinAlert();
  refreshOdinAccess();
  window.setInterval(function(){if(current==='odin'&&!document.hidden)refreshOdinAccess();},15000);
  document.addEventListener('visibilitychange',function(){if(!document.hidden)update();});
})();
