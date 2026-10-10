(function(){
  'use strict';
  var status=document.getElementById('engine-config-status');
  var editor=document.getElementById('engine-config-editor');
  var engineSelect=document.getElementById('engine-config-engine');
  var digest='';
  function endpoint(){
    var p=window.location.pathname;
    var match=p.match(/^(.*\/api\/modules\/port-access-manager)\/(?:v1\/)?ui(?:\/.*)?$/);
    if(match)return match[1]+'/engine-config?engine='+encodeURIComponent(engineSelect.value);
    match=p.match(/^(.*)\/v1\/ui(?:\/.*)?$/);
    if(match)return match[1]+'/v1/engine-config?engine='+encodeURIComponent(engineSelect.value);
    return '/api/modules/port-access-manager/engine-config?engine='+encodeURIComponent(engineSelect.value);
  }
  function clear(){ editor.value='';digest='';editor.disabled=true; }
  function request(action,content){
    return fetch(endpoint(),{method:'POST',credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({action:action,base_sha256:digest,confirm:action==='read'?'SHOW_CONFIG':action==='save'?'SAVE_CONFIG':'RESTORE_CONFIG',content:content||''})})
    .then(function(r){return r.json().then(function(data){if(!r.ok)throw new Error(data.error||('HTTP '+r.status));return data;});});
  }
  function info(){
    clear();status.textContent='Проверяем установленную конфигурацию…';
    fetch(endpoint(),{credentials:'same-origin',cache:'no-store'}).then(function(r){return r.json().then(function(data){if(!r.ok)throw new Error(data.error||('HTTP '+r.status));return data;});})
      .then(function(data){digest=data.sha256;status.textContent='Конфигурация найдена ('+data.size+' байт). Резервная копия: '+(data.backup_available?'есть':'пока нет')+'. Содержимое не загружено.';})
      .catch(function(e){status.textContent=e.message;});
  }
  document.getElementById('engine-config-load').addEventListener('click',function(){
    if(!digest){status.textContent='Сначала проверь конфигурацию';return;}
    if(!window.confirm('Показать содержимое файла '+engineSelect.value+'? В fwknopd могут находиться секретные ключи.'))return;
    request('read').then(function(data){editor.value=data.content;editor.disabled=false;status.textContent='Загружено. Изменения не применены к работающей службе.';}).catch(function(e){status.textContent=e.message;});
  });
  document.getElementById('engine-config-save').addEventListener('click',function(){
    if(editor.disabled||!digest){status.textContent='Сначала загрузи файл';return;}
    if(!window.confirm('Сохранить конфигурацию '+engineSelect.value+'? Будет создана резервная копия. Для применения нужен перезапуск службы.'))return;
    request('save',editor.value).then(function(data){digest=data.sha256;status.textContent='Сохранено. Резервная копия создана. Перезапусти службу через управление движками.';}).catch(function(e){status.textContent=e.message;});
  });
  document.getElementById('engine-config-restore').addEventListener('click',function(){
    if(!digest){status.textContent='Сначала проверь конфигурацию';return;}
    if(!window.confirm('Восстановить предыдущий файл '+engineSelect.value+' из резервной копии?'))return;
    request('restore').then(function(){info();status.textContent='Предыдущая версия восстановлена. Для применения перезапусти службу.';}).catch(function(e){status.textContent=e.message;});
  });
  engineSelect.addEventListener('change',info);
  document.getElementById('engine-config-refresh').addEventListener('click',info);
  info();
}());
