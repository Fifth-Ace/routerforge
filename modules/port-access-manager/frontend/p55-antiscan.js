(function () {
  'use strict';
  // P55: presentation-only Antiscan-inspired layout. Original control nodes/listeners survive.
  var root=document.querySelector('.p54-root');
  var main=document.querySelector('main.pa');
  if(!root||!main)return;
  var nav=root.querySelector('.p54-navigation');
  var pages={};
  ['protection','access','engines','diagnostics'].forEach(function(id){pages[id]=document.getElementById('p54-'+id);});
  if(!nav||!pages.protection)return;
  var header=main.querySelector('header');
  if(header){
    header.classList.add('p55-hero');
    var title=header.querySelector('h1');if(title)title.textContent='Контроль доступа';
    var desc=header.querySelector('p');if(desc)desc.textContent='Защита проброшенных сервисов: odin, классический Port Knocking и SPA.';
  }
  main.classList.add('p55-access');
  // Keep the original diagnostic refresh button (with its listeners) instead of making a fake refresh action.
  var summary=pages.protection.querySelector('.rf-pa-summary');
  var refresh=document.getElementById('refresh');
  var headActions=document.createElement('div');headActions.className='p55-head-actions';
  var state=document.createElement('span');state.className='p55-state';state.textContent='ПРОВЕРКА…';headActions.appendChild(state);
  if(refresh){refresh.textContent='↻ Обновить';headActions.appendChild(refresh);}
  if(header)header.appendChild(headActions);
  var metrics=summary&&summary.querySelector('.rf-pa-metrics');
  if(metrics){
    var metricNodes=metrics.querySelectorAll('.rf-pa-metric');
    ['ДВИЖОК','ЗАЩИТА','КОНФИГУРАЦИЯ','WATCHDOG / NDM'].forEach(function(label,i){var span=metricNodes[i]&&metricNodes[i].querySelector('span');if(span)span.textContent=label;});
    root.insertBefore(summary,nav);
  }
  var protectionLabel=nav.querySelector('.p54-tab');
  var names=['Обзор','Доступ','Движки','Диагностика'];
  Array.prototype.forEach.call(nav.querySelectorAll('.p54-tab'),function(button,i){
    button.textContent=names[i]||button.textContent;
    button.classList.add('p55-tab');
  });
  // Existing status text comes from the original, already bound manager.
  function refreshState(){
    var b=metrics&&metrics.querySelectorAll('.rf-pa-metric b')[1];
    var s=b&&b.textContent||'';
    state.textContent=/включена|подтверждено/i.test(s)?'РАБОТАЕТ':/ожидает|watchdog/i.test(s)?'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ':/выключена|не установлен|остановлен/i.test(s)?'НЕ АКТИВНА':'ПРОВЕРКА…';
    state.className='p55-state '+(/РАБОТАЕТ/.test(state.textContent)?'good':/ОЖИДАЕТ/.test(state.textContent)?'warn':'');
  }
  if(metrics&&typeof MutationObserver!=='undefined'){new MutationObserver(refreshState).observe(metrics,{subtree:true,childList:true,characterData:true});}
  refreshState();
  // Reuse original nodes, do not clone or recreate any control (and its event listeners).
  var chooser=pages.protection.querySelector('.rf-pa-selector');
  if(chooser){var h=chooser.querySelector('h2');if(h)h.textContent='Способ защиты';}
  var working=pages.protection.querySelector('.rf-pa-working');
  var deploy=document.getElementById('rfpa-deploy-panel');
  if(deploy){
    deploy.classList.add('p55-deploy');
    var actionTitle=deploy.querySelector('.rf-pa-action-heading');if(actionTitle)actionTitle.remove();
    var h=deploy.querySelector('h3');if(h)h.textContent='Применение и откат';
    var bar=deploy.querySelector('.rf-pa-deploy-actions');if(bar)bar.classList.add('p55-actions');
    var checkbox=document.getElementById('rfpa-replace-existing');
    if(checkbox){var label=checkbox.closest('label');if(label)label.classList.add('p55-confirm-row');}
    var status=document.getElementById('rfpa-deploy-status');if(status)status.classList.add('p55-compact-status');
  }
  // Make sections compact and use the same Antiscan card vocabulary.
  Array.prototype.forEach.call(root.querySelectorAll('.panel,.rf-pa-summary,.rf-pa-selector,.rf-pa-engine-scope,.rf-pa-advanced'),function(p){p.classList.add('p55-panel');});
  Array.prototype.forEach.call(root.querySelectorAll('.form-grid'),function(grid){grid.classList.add('p55-grid');});
  // P54's navigation still owns page switching; no second navigation state machine.
  // In the new layout, remove duplicate section header and nested presentation chrome.
  Array.prototype.forEach.call(root.querySelectorAll('.p54-heading'),function(h){h.classList.add('p55-section-heading');});
  // Move the original runtime log to the diagnostics page if already placed there by P54.
  var log=document.querySelector('.rf-pa-operation-log');if(log&&pages.diagnostics&&!pages.diagnostics.contains(log))pages.diagnostics.appendChild(log);
})();
