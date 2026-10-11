(function(){'use strict';
  var root=document.getElementById('odin-manager');if(!root)return;
  var status=document.getElementById('odin-status');
  var endpoint='/api/modules/port-access-manager/odin';
  var fields=['wan','ip','port','k1','k2','k3','window','ttl'];
  var input=function(n){return document.getElementById('odin-'+n);};
  function request(method,body){return fetch(endpoint,{method:method,credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json','Accept':'application/json'},body:body?JSON.stringify(body):undefined}).then(function(resp){return resp.json().then(function(data){if(!resp.ok)throw Error(data.error||('HTTP '+resp.status));return data;});});}
  function refresh(){status.textContent='Читаем состояние…';return request('GET').then(function(data){
    status.textContent=(data.enabled?(data.confirmed?'Включён · подтверждён':'ВКЛЮЧЁН · подтвердить в течение 90 секунд!'):'Выключен')+(data.configured?' · конфигурация сохранена':' · конфигурация отсутствует');
    if(data.configured){var s=data.settings;['wan','ip','port','window','ttl'].forEach(function(n){input(n).value=s[n]});['k1','k2','k3'].forEach(function(n,i){input(n).value=s.knock[i]});}
    root.querySelector('[data-odin="save"]').disabled=!!data.enabled;
    root.querySelector('[data-odin="enable"]').disabled=!!data.enabled||!data.configured;
    root.querySelector('[data-odin="disable"]').disabled=!data.enabled;
    root.querySelector('[data-odin="confirm"]').disabled=!data.enabled||!!data.confirmed;
  }).catch(function(e){status.textContent='Ошибка: '+e.message;});}
  function settings(){return {wan:input('wan').value.trim(),ip:input('ip').value.trim(),port:Number(input('port').value),knock:[Number(input('k1').value),Number(input('k2').value),Number(input('k3').value)],window:Number(input('window').value),ttl:Number(input('ttl').value)};}
  root.querySelectorAll('[data-odin]').forEach(function(button){button.addEventListener('click',function(){var action=button.getAttribute('data-odin');if(action!=='refresh'&&!confirm('Port Knocking odin: '+action+'? Изменения firewall выполняются только при включении или выключении.'))return;if(action==='refresh'){refresh();return;}status.textContent='Выполняется '+action+'…';request('POST',{action:action,confirm:'APPLY',settings:settings()}).then(refresh).catch(function(e){status.textContent='Ошибка: '+e.message;});});});
  refresh();
})();
