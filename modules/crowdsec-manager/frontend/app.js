(function(){'use strict';var $=function(id){return document.getElementById(id)};var busy=false;
function txt(id,s){$(id).textContent=s}
function badge(id,v){txt(id,v?'Работает':'Нет');$(id).className=v?'ok':'fail'}
function refresh(){if(busy)return;busy=true;$('refresh').disabled=true;
fetch('/api/modules/crowdsec-manager/status',{credentials:'same-origin',cache:'no-store'}).then(function(r){if(!r.ok)throw Error('HTTP '+r.status);return r.json()}).then(function(d){
$('verdict').textContent=d.protected?'Защита активна: процесс, ipset и правила FORWARD подтверждены':'Защита не подтверждена: проверьте диагностику ниже';$('verdict').className='banner '+(d.protected?'good':'bad');
badge('running',d.running);txt('installed',d.installed?'Бинарник установлен':'Бинарник не найден');badge('firewall',d.firewall.ok);txt('firewall-hint',d.firewall.detail);badge('sets',d.sets.ok);txt('sets-hint',d.sets.detail);badge('hook',d.ndm_hook.ok);txt('hook-hint',d.ndm_hook.detail);txt('chain-hint',(d.chains||[]).join(' · ')||'Цепочка не подтверждена');txt('updated',d.updated||'—');
var host=$('set-list');host.textContent='';(d.set_names||[]).forEach(function(n){var a=document.createElement('span');a.textContent=n;host.appendChild(a)});if(!(d.set_names||[]).length)host.textContent='Списков CrowdSec нет';var notes=$('notices');notes.textContent='';(d.notes||[]).forEach(function(s){var p=document.createElement('p');p.textContent=s;notes.appendChild(p)});
}).catch(function(e){$('verdict').textContent='Нет связи с runtime CrowdSec Manager: '+e.message;$('verdict').className='banner bad'}).then(function(){busy=false;$('refresh').disabled=false})}
$('refresh').addEventListener('click',refresh);refresh();setInterval(function(){if(!document.hidden)refresh()},15000)
})();
