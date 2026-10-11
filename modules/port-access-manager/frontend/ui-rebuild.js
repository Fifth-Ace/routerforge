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
  var access=move('recent-live-panel',shell);
  var advanced=el('details','rf-pa-advanced');shell.appendChild(advanced);
  var summaryNode=el('summary','','Инженерное · диагностика, предпросмотр и черновики');advanced.appendChild(summaryNode);
  var advancedBody=el('div','rf-pa-advanced-body');advanced.appendChild(advancedBody);
  // Move original content instead of rebuilding it: all existing handlers retain their nodes.
  Array.prototype.slice.call(main.children).forEach(function(node){
    if(node===header||node===shell||node.tagName==='SCRIPT')return;
    advancedBody.appendChild(node);
  });
  // P35C: odin owns RFOD_D; legacy recent UI belongs to the engineering section.
  var odinAccess=el('section','panel rf-pa-odin-access');
  odinAccess.id='odin-live-access';
  var odinAccessTitle=el('h2','','Авторизованные IP · odin');odinAccess.appendChild(odinAccessTitle);
  odinAccess.appendChild(el('p','help','Текущие разрешения из RFOD_D. Истечение доступа проверяется ядром; точное оставшееся время недоступно.'));
  var accessTools=el('div','refresh-bar');odinAccess.appendChild(accessTools);
  var accessRefresh=el('button','','Обновить IP');accessRefresh.type='button';accessTools.appendChild(accessRefresh);
  var accessState=el('span','muted','Ожидаем ответ…');accessState.setAttribute('role','status');accessTools.appendChild(accessState);
  var accessRows=el('div','rf-pa-access-list');odinAccess.appendChild(accessRows);
  shell.insertBefore(odinAccess,advanced);
  if(access)advancedBody.appendChild(access);
  function refreshOdinAccess(){
    accessState.textContent='Читаем RFOD_D…';
    fetch('/api/modules/port-access-manager/odin-access',{credentials:'same-origin',cache:'no-store'})
    .then(function(r){return r.json().then(function(d){if(!r.ok)throw Error(d.error||('HTTP '+r.status));return d;});})
    .then(function(d){
      accessRows.textContent='';
      if(!d.available){accessState.textContent=d.reason||'Авторизации недоступны';return;}
      if(d.list!=='RFOD_D'||!Array.isArray(d.entries))throw Error('Неверный ответ API');
      accessState.textContent='Активных записей: '+d.entries.length;
      if(!d.entries.length){accessRows.appendChild(el('p','muted','Авторизованных IP пока нет.'));return;}
      d.entries.forEach(function(entry){if(typeof entry.ip==='string')accessRows.appendChild(el('div','rf-pa-access-ip',entry.ip));});
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
  var existingCards=service?service.querySelectorAll('.engine-card'):[];
  var selector=document.getElementById('engine-config-engine');
  var current='odin';
  function select(id){
    current=id;
    options.forEach(function(o){var active=o.id===id;buttons[o.id].setAttribute('aria-pressed',String(active));buttons[o.id].classList.toggle('active',active);});
    if(odin)odin.hidden=id!=='odin';
    if(daemons)daemons.hidden=id==='odin';
    if(access)access.hidden=false; // Legacy recent UI stays in collapsed engineering details.
    odinAccess.hidden=id!=='odin';
    Array.prototype.forEach.call(existingCards,function(card,i){card.hidden=(id==='odin'||(i===0)!==(id==='knockd'));});
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
        ndmMetric.textContent=data.enabled?(data.confirmed?'Подтверждено':'Watchdog · 90 секунд'):'Не активен';
      }).catch(function(){stateMetric.textContent='Нет связи с API';configMetric.textContent='—';ndmMetric.textContent='—';});
    }else{
      fetch('/api/modules/port-access-manager/engine-service?engine='+encodeURIComponent(current),{credentials:'same-origin',cache:'no-store'})
      .then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json();})
      .then(function(data){stateMetric.textContent=data.running?'Запущен':'Остановлен';configMetric.textContent=data.installed?'Сервис установлен':'Сервис не установлен';ndmMetric.textContent='Управляет upstream';})
      .catch(function(){stateMetric.textContent='Нет связи с API';configMetric.textContent='—';ndmMetric.textContent='—';});
    }
  }
  refresh.addEventListener('click',function(){update();var existing=document.querySelector('[data-odin="refresh"]');if(current==='odin'&&existing)existing.click();else{var control=document.getElementById('existing-engine-refresh');if(control)control.click();}});
  document.addEventListener('routerforge-odin-state-changed',function(){if(current==='odin')update();});
  select('odin');
  refreshOdinAccess();
  document.addEventListener('visibilitychange',function(){if(!document.hidden)update();});
})();
