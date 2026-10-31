'use strict';
// DEV-only RouterForge DNS Lite. No old P10 handlers, no HTML injection.
(() => {
  const $ = id => document.getElementById(id);
  const state = {config:{schema_version:1,upstreams:[],rules:[]},revision:'',dirty:false,busy:false,editing:null,editorBase:null,checked:null,status:null,logs:[],page:'dns',lastLogError:''};
  const used = id => state.config.rules.filter(r=>r.upstream===id);
  const protocolNames={doh:'DoH · HTTPS',dot:'DoT · TLS',doq:'DoQ · QUIC',udp:'UDP',tcp:'TCP',native:'Старый DNS Keenetic'};
  const isEncrypted = t => t==='doh'||t==='dot'||t==='doq';
  const isSimple = r => r.match==='suffix' && !r.group_id && Object.keys(r).every(k=>['id','domain','match','upstream','priority'].includes(k));
  const newId = (prefix,items) => {let i=1;const ids=new Set(items.map(x=>x.id));while(ids.has(prefix+'-'+i))i++;return prefix+'-'+i;};
  const displayName = u => u.server_name || (u.transport==='native'?'Наследованный DNS Keenetic':u.address);
  const hostName = u => protocolNames[u.transport]+' · '+displayName(u);
  const editorPending=()=>state.editing!==null && $('editorDomains').value!==state.editorBase;
  const show = (message,tone='') => {const el=$('feedback');el.textContent=message;el.dataset.tone=tone;};
  const busy = on => {state.busy=on;for(const id of ['saveButton','applyButton','addDnsToggle','inspectDNS','commitDNS','migrateNative','connectButton','disconnectButton','verifyButton','stopButton','gatePreflight','gateEnable','gateDisable']){const el=$(id);if(el)el.disabled=on;}};
  async function api(path,options={}) {
    const resp=await fetch('../'+path,{cache:'no-store',credentials:'same-origin',...options});
    const ct=resp.headers && resp.headers.get ? (resp.headers.get('content-type')||'') : 'application/json';
    if(!ct.toLowerCase().includes('json'))throw Error('Ожидался JSON от RouterForge, получен другой ответ: '+path);
    const data=await resp.json();
    if(!resp.ok)throw Error((data&&data.error)||('HTTP '+resp.status+' · '+path));
    return data;
  }
  const opts=(action,body,match)=>({method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...(match?{'If-Match':'"'+state.revision+'"'}:{})},body:JSON.stringify(body)});
  function mark(){state.dirty=true;show('Есть несохранённые изменения. Действующий DNS не менялся.');render();}
  function setPage(page){state.page=page;$('pageDns').hidden=page!=='dns';$('pageLogs').hidden=page!=='logs';for(const [id,target] of [['navDns','dns'],['navLogs','logs']]){$(id).classList.toggle('selected',page===target);if(page===target)$(id).setAttribute('aria-current','page');else $(id).removeAttribute('aria-current');}if(page==='logs')refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));reportHeight();}
  const option = (value,label) => {const el=document.createElement('option');el.value=value;el.textContent=label;return el;};
  function render(){
    const list=state.config.upstreams||[];
    const primary=$('defaultDNS');const saved=primary.value||state.config.service?.default_upstream||'';
    primary.replaceChildren(option('','Выбери основной DNS…'));
    for(const u of list)primary.appendChild(option(u.id,hostName(u)));
    primary.value=list.some(u=>u.id===saved)?saved:'';
    $('serverCount').textContent=list.length+' DNS';
    $('nativeWarning').hidden=!list.some(u=>u.transport==='native'&&used(u.id).length);
    const root=$('dnsCards');root.replaceChildren();
    $('editorEmpty').hidden=Boolean(state.editing);$('listEditor').hidden=!state.editing;
    if(!list.length){const p=document.createElement('p');p.className='muted';p.textContent='Добавь первый DNS-сервер. Здесь появятся его список доменов и состояние сертификата.';root.appendChild(p);}
    for(const u of list){
      const rules=used(u.id),card=document.createElement('article');card.className='dns-card';card.dataset.upstreamId=u.id;card.classList.toggle('is-selected',state.editing===u.id);
      const header=document.createElement('div');header.className='dns-top';const left=document.createElement('div');left.className='dns-title';
      const name=document.createElement('strong');name.textContent=hostName(u);left.appendChild(name);
      const dest=document.createElement('small');dest.textContent=u.transport==='native'?u.address:'IP: '+u.address+(u.path?' · '+u.path:'');left.appendChild(dest);
      const cert=document.createElement('small');cert.textContent=(u.transport==='dot'||u.transport==='doh')?(u.certificate_pem?'✓ Сертификат проверен и сохранён (TLS через доверенные CA)':'TLS через доверенные CA · сертификат не сохранён'):u.transport==='doq'?'TLS через доверенные CA · проверка при подключении':'Без шифрования';left.appendChild(cert);
      const count=document.createElement('span');count.className='dns-count';count.textContent=rules.length+' доменов';header.append(left,count);card.appendChild(header);
      const preview=document.createElement('p');preview.className='dns-preview';preview.textContent=rules.length?rules.slice(0,3).map(r=>r.domain).join(' · ')+(rules.length>3?' · ещё '+(rules.length-3):''):'Список доменов пуст';card.appendChild(preview);
      const bottom=document.createElement('div');bottom.className='dns-bottom';
      const edit=document.createElement('button');edit.type='button';edit.className='button dns-edit'+(state.editing===u.id?' primary':'');edit.textContent=state.editing===u.id?'✓ Список открыт':'✎ Открыть список →';edit.disabled=u.transport==='native'||rules.some(r=>!isSimple(r));edit.addEventListener('click',()=>editList(u.id));
      const reserve=document.createElement('label');reserve.className='field';const l=document.createElement('span');l.textContent='Резервный DNS';reserve.appendChild(l);const select=document.createElement('select');select.className='dns-select';select.appendChild(option('','Без резерва'));
      for(const t of list.filter(t=>t.id!==u.id && t.transport!=='native' && !(t.fallbacks||[]).length))select.appendChild(option(t.id,displayName(t)));
      select.value=(u.fallbacks||[])[0]||'';
      if(![...select.options].some(o=>o.value===select.value))select.value='';
      select.disabled=u.transport==='native';select.addEventListener('change',()=>{if(select.value && state.config.upstreams.some(x=>(x.fallbacks||[]).includes(u.id))){show('Этот DNS уже является резервным. Сначала убери его из чужой резервной цепочки.','bad');render();return;}u.fallbacks=select.value?[select.value]:[];mark();});reserve.appendChild(select);bottom.appendChild(reserve);
      const del=document.createElement('button');del.type='button';del.className='button danger';del.textContent='Удалить DNS';del.addEventListener('click',()=>removeDNS(u.id));bottom.appendChild(del);
      const advanced=document.createElement('details');advanced.className='dns-advanced';const advLabel=document.createElement('summary');advLabel.textContent='Резервный DNS и удаление';advanced.appendChild(advLabel);advanced.appendChild(bottom);card.appendChild(edit);card.appendChild(advanced);root.appendChild(card);
    }
    if(state.config.service){$('serviceListen').value=state.config.service.listen;$('cacheSize').value=state.config.service.cache_bytes/(1024*1024);$('allowedNetworks').value=state.config.service.allowed_networks.join(',');}
    $('listenerSummary').textContent=state.config.service?.listen||$('serviceListen').value;
    $('applyButton').textContent=state.dirty?'✓ Сохранить и применить':'▶ Применить DNS';
    reportHeight();
  }
  function parseDomains(text){
    if(text.length>256*1024)throw Error('Список слишком большой (максимум 256 КБ)');
    const names=[],seen=new Set();
    for(const line of text.split(/\r?\n/)){
      const clean=line.split('#')[0].trim();if(!clean)continue;
      for(let raw of clean.split(/[\s,;]+/)){
        if(!raw)continue;raw=raw.toLowerCase().replace(/\.$/,'');
        if(raw.length>253||!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/.test(raw))throw Error('Некорректный домен: '+raw);
        if(!seen.has(raw)){seen.add(raw);names.push(raw);}
        if(names.length>10000)throw Error('Список слишком большой: больше 10 000 доменов');
      }
    }
    return names;
  }
  function editList(id){if(editorPending() && !confirm('У выбранного DNS есть несохранённые изменения списка. Перейти к другому DNS и потерять введённый текст?'))return;const u=state.config.upstreams.find(x=>x.id===id);const rules=used(id);if(!u||u.transport==='native'||rules.some(r=>!isSimple(r)))throw Error('Для этого DNS сначала преобразуй native или сложные правила');state.editing=id;$('editorTitle').textContent='Список доменов · '+displayName(u);$('selectedDNSBadge').textContent=displayName(u);$('editorDomains').value=rules.map(x=>x.domain).join('\n');state.editorBase=$('editorDomains').value;$('editorCount').textContent=rules.length+' доменов';$('editorEmpty').hidden=true;$('listEditor').hidden=false;$('editorHint').textContent=rules.length+' доменов сейчас. Изменения не затронут другие DNS.';render();reportHeight();}
  function closeEditor(){if(editorPending() && !confirm('Закрыть редактор? Несохранённый текст списка будет потерян.'))return;state.editing=null;state.editorBase=null;$('listEditor').hidden=true;$('editorEmpty').hidden=false;$('selectedDNSBadge').textContent='DNS не выбран';render();}
  function saveList(){
    if(!state.editing)return;
    const id=state.editing,old=used(id);if(old.some(r=>!isSimple(r)))throw Error('Найдено сложное правило. Список не изменён');
    const names=parseDomains($('editorDomains').value),want=new Set(names);
    const other=new Set(state.config.rules.filter(r=>r.upstream!==id).map(r=>r.domain));
    const collision=names.find(x=>other.has(x));if(collision)throw Error('Домен '+collision+' уже привязан к другому DNS. Сначала убери его там.');
    const oldMap=new Map(old.map(r=>[r.domain,r]));const ids=new Set(state.config.rules.map(r=>r.id));
    const fresh=names.map(domain=>{
      if(oldMap.has(domain))return oldMap.get(domain);
      let n=1;while(ids.has('domain-'+n))n++;const uid='domain-'+n;ids.add(uid);return {id:uid,domain,match:'suffix',upstream:id,priority:100};
    });
    state.config.rules=state.config.rules.filter(r=>r.upstream!==id).concat(fresh);
    $('editorDomains').value=names.join('\n');state.editorBase=$('editorDomains').value;$('editorCount').textContent=names.length+' доменов';$('editorHint').textContent='Список сохранён в черновик. Нажми «Сохранить и применить» ниже.';mark();show('Список обновлён: '+names.length+' доменов. Остальные DNS не менялись.');
  }
  function removeDNS(id){
    const u=state.config.upstreams.find(x=>x.id===id);if(!u)return;
    if((state.config.dns_groups||[]).some(g=>g.servers?.includes(id))||state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))throw Error('Этот DNS используется группой или резервной цепочкой. Сначала убери зависимость.');
    const rules=used(id);if(rules.some(r=>!isSimple(r)))throw Error('Есть сложные правила, удаление через простой экран заблокировано.');
    if(!confirm('Удалить '+displayName(u)+' и '+rules.length+' его доменов ИЗ ЧЕРНОВИКА?'))return;
    state.config.upstreams=state.config.upstreams.filter(x=>x.id!==id);state.config.rules=state.config.rules.filter(r=>r.upstream!==id);
    if(state.config.service?.default_upstream===id){state.config.service=null;$('defaultDNS').value='';}
    if(state.editing===id)closeEditor();mark();
  }
  function newFormData(){
    let transport=$('newProtocol').value;let raw=$('newAddress').value.trim();
    if(!raw)throw Error('Укажи адрес DNS');
    let host='',path='';
    if(transport==='doh'){
      let url;try{url=new URL(/^https:\/\//i.test(raw)?raw:'https://'+raw)}catch(_){throw Error('Неверный DoH URL');}
      if(url.protocol!=='https:'||url.username||url.password||url.search||url.hash||url.port||url.pathname.includes('..')||url.pathname.startsWith('//'))throw Error('DoH: нужен HTTPS-адрес без нестандартного порта и параметров');
      host=url.hostname.toLowerCase();path=url.pathname==='/'?'/dns-query':url.pathname;
    }else{if(/[\/:?@\s]/.test(raw)&&transport!=='udp')throw Error('Нужен домен DNS-сервера без протокола и порта');host=raw.toLowerCase();}
    if(transport==='udp'){if(!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(raw)||raw.split('.').some(n=>Number(n)>255))throw Error('Для UDP укажи IPv4-адрес, например 9.9.9.9');host=raw;}
    if(isEncrypted(transport)&&(!/^[a-z0-9-]+(?:\.[a-z0-9-]+)+$/.test(host)))throw Error('Для защищённого DNS требуется имя сервера');
    return {transport,host,path};
  }
  async function inspect(){
    state.checked=null;$('commitDNS').disabled=true;
    const f=newFormData(),key=$('newProtocol').value+'|'+$('newAddress').value.trim();
    const r=await api('certificates/lookup',opts('certificate-lookup',{transport:f.transport,server_name:f.host}));
    if(key!==$('newProtocol').value+'|'+$('newAddress').value.trim())throw Error('Адрес изменился во время проверки');
    if(!r.address||!r.selected_ip)throw Error('Не найден IP DNS-сервера');
    if((f.transport==='doh'||f.transport==='dot')&&(!r.certificate?.verified||!r.certificate.pem))throw Error('TLS-сертификат не удалось проверить');
    state.checked={key,form:f,lookup:r};
    let cert=f.transport==='doh'||f.transport==='dot'?'✓ TLS проверен по доверенным CA. Сертификат сохранится для просмотра.':f.transport==='doq'?'DoQ: сертификат будет проверен при подключении, предварительный просмотр недоступен.':'UDP не использует TLS-сертификаты.';
    $('certificateInfo').textContent='IP '+r.selected_ip+' · '+cert;
    $('certificateDetails').hidden=!r.certificate;
    $('certificateDetailsText').textContent=r.certificate?['Имя: '+r.certificate.server_name,'Кому: '+r.certificate.subject,'Выдан: '+r.certificate.issuer,'Действителен до: '+r.certificate.not_after,'SHA256: '+r.certificate.fingerprint_sha256].join('\n'):'';
    $('commitDNS').disabled=false;reportHeight();
  }
  function addDNS(){
    if(editorPending())throw Error('Сначала сохрани изменённый список доменов справа.');
    const c=state.checked;if(!c||c.key!==$('newProtocol').value+'|'+$('newAddress').value.trim())throw Error('Сначала проверь этот DNS');
    const f=c.form,r=c.lookup;
    if(state.config.upstreams.some(u=>u.transport===f.transport&&u.address===r.address&&u.server_name===(isEncrypted(f.transport)?f.host:'')&&(u.path||'')===f.path))throw Error('Такой DNS уже добавлен');
    const u={id:newId('dns',state.config.upstreams),transport:f.transport,address:r.address};
    if(isEncrypted(f.transport))u.server_name=f.host;
    if(f.transport==='doh')u.path=f.path;
    if(r.certificate?.verified)u.certificate_pem=r.certificate.pem;
    state.config.upstreams.push(u);state.checked=null;$('dnsAddPanel').hidden=true;$('certificateInfo').textContent='Проверка ещё не выполнялась.';$('newAddress').value='';mark();editList(u.id);
    show('DNS добавлен. Его список открыт справа. Выбери DNS по умолчанию перед применением.');
  }
  function servicePrefs(){
    const primary=$('defaultDNS').value;
    if(!primary || !state.config.upstreams.some(u=>u.id===primary))throw Error('Выбери DNS по умолчанию');
    const cache=Number($('cacheSize').value);if(!Number.isInteger(cache)||cache<0||cache>8)throw Error('Кэш: от 0 до 8 МиБ');
    const listen=$('serviceListen').value.trim(),networks=$('allowedNetworks').value.split(',').map(s=>s.trim()).filter(Boolean);
    return {listen,default_upstream:primary,cache_bytes:cache*1024*1024,allowed_networks:networks};
  }
  function syncPrefs(){const next=servicePrefs();const prev=state.config.service;if(JSON.stringify(prev)!==JSON.stringify(next)){state.config.service={...prev,...next};if(prev?.default_upstream!==next.default_upstream)delete state.config.service.default_group;state.dirty=true;}}
  async function save(){if(editorPending())throw Error('Сначала нажми «Сохранить список» в редакторе доменов.');syncPrefs();if(!state.dirty)return;const data=await api('config',{method:'PUT',headers:{'Content-Type':'application/json','X-RouterForge-Action':'save','If-Match':'"'+state.revision+'"'},body:JSON.stringify(state.config)});state.revision=data.revision;state.dirty=false;show('Настройки сохранены. Действующий DNS изменится только после применения.','good');render();}
  async function serviceCommand(path,body){return api('service/'+path,opts('service-control',body,true));}
  function hasUsedNative(){return state.config.upstreams.some(u=>u.transport==='native'&&(used(u.id).length||state.config.service?.default_upstream===u.id));}
  async function apply(){
    if(hasUsedNative())throw Error('Есть старые native DNS. Сначала нажми «Преобразовать DNS» и проверь адреса.');
    const prefs=servicePrefs();
    if(!confirm('Сохранить настройки и запустить DNS Router на '+prefs.listen+'? DNS устройств Keenetic не меняется.'))return;
    await save();const s=await api('service/status');
    await serviceCommand('control',{operation:s.active?'apply':'start'});
    show('DNS запущен. Проверь его, затем подключи одно устройство для реального теста.','good');await refreshStatus();
  }
  async function migrate(){
    if(state.dirty)throw Error('Сначала сохрани изменения. Миграция выполняется только из сохранённого состояния.');
    const s=await api('service/status');if(s.active||s.connection?.state!=='disconnected')throw Error('Перед миграцией останови сервис и отключи тестовое устройство.');
    if(!confirm('Преобразовать используемые native DNS Keenetic в прямые DoH-соединения? При ошибке конвертации ничего не изменится.'))return;
    await serviceCommand('migrate-native',{operation:'migrate'});await load();show('DNS преобразованы. Проверь адреса и сертификаты перед применением.','good');
  }
  function routingHint(s){
    const c=s.connection||{state:'disconnected'};
    const endpoint=s.listen||state.config.service?.listen||$('serviceListen').value;
    const upstream=Number(s.requests||0);
    if(!s.active)return {tone:'bad',text:'DNS Router остановлен. На штатном DNS Keenetic (порт 53) интернет может продолжать работать, но журнал RouterForge пуст.'};
    if(c.state==='recovery_required')return {tone:'bad',text:'DNS-профиль устройства требует восстановления: '+(c.error||'состояние не совпадает с Keenetic')+'. Не устанавливай и не обновляй пакет до исправления профиля.'};
    if(c.state==='pending')return {tone:'bad',text:'Подключение устройства не завершено (pending). DNS Router слушает '+endpoint+', но профиль клиента Keenetic ещё не подтверждён. Сначала верни прежний DNS устройства или заверши подключение; не пытайся обновлять пакет с pending.'};
    const denied=Number(s.denied_requests||0);
    if(denied>0 && denied===upstream)return {tone:'bad',text:'До DNS Router дошли '+denied+' запросов, но ВСЕ отклонены (REFUSED) списком разрешённых клиентских сетей. Проверь «Дополнительные настройки → Разрешённые сети» и IP клиента; менять настройки Keenetic не требуется.'};
    if(upstream===0)return {tone:'warn',text:'DNS Router слушает '+endpoint+', но получил 0 запросов. Планшет может использовать штатный DNS Keenetic (:53), свой Private DNS/DoH или другой DNS-профиль. '+(c.state==='connected'?'Профиль назначен, но прохождение трафика не подтверждено.':'Тестовый клиент не подключён через RouterForge.')+' Прямая кнопка «Проверить DNS» тестирует только сервис, не планшет.'};
    if(c.state!=='connected')return {tone:'warn',text:'DNS Router получил '+upstream+' запросов, но клиентский профиль не подключён. Это может быть ручная проверка сервиса; маршрут планшета через Keenetic не доказан.'};
    return {tone:'',text:'DNS Router получил '+upstream+' запросов на '+endpoint+'. Клиентский профиль Keenetic подключён, но чтобы подтвердить именно планшет, сопоставь время его DNS-запросов с журналом.'};
  }
  function showRoutingHint(s){const hint=routingHint(s);for(const id of ['routeAdvice','logGuide']){const node=$(id);node.textContent=hint.text;node.dataset.tone=hint.tone;}}
  function renderGate(s){
    const gate=s.system_gate||{state:'STOCK',active:false};
    $('gateBadge').textContent=gate.state;
    $('gateBadge').dataset.tone=gate.active?'good':gate.state==='RECOVERY_REQUIRED'?'bad':'';
    $('gateEnable').disabled=state.busy||!s.active||gate.active||gate.state==='RECOVERY_REQUIRED';
    $('gateDisable').disabled=state.busy||(!gate.active&&gate.state!=='RECOVERY_REQUIRED');
    const hits=Number(s.requests||0),denied=Number(s.denied_requests||0);
    $('gateAdvice').textContent=gate.state==='RECOVERY_REQUIRED'?('Восстановление: '+(gate.last_error||'Нужна проверка правил firewall')):
      gate.active?('DNS Gate активен на br0 (UDP/TCP 53). Обработано запросов: '+hits+', ACL-отказов: '+denied+'. Проверяй журнал и устройство.'):
      'Системный DNS Keenetic работает штатно. Для включения сначала проверь готовность, затем подтверди перехват LAN.';
  }
  async function refreshStatus(){const s=await api('service/status');if(s.mode!=='independent-dns')throw Error('Загружен не DNS Lite. Установи актуальный DEV-пакет.');state.status=s;renderGate(s);
    const connection=s.connection||{state:'disconnected'},active=Boolean(s.active),same=active&&s.applied_revision===state.revision;
    $('serviceBadge').textContent=active?'DNS работает':'DNS остановлен';$('serviceBadge').dataset.tone=active?'good':'bad';
    $('runStatus').textContent=(active?'DNS работает: '+s.listen+' · Основной '+displayName(state.config.upstreams.find(u=>u.id===s.default_upstream)||{transport:'udp',address:s.default_upstream||'—'}):'Сервис остановлен.')+(active&&!same?' · Есть неприменённая версия настроек.':'')+(s.last_error?' · Ошибка: '+s.last_error:'');
    $('connectionSummary').textContent=connection.state==='connected'?'Устройство подключено: '+connection.mac:connection.state==='recovery_required'?'Подключение требует восстановления':'Устройства не подключены';
    $('clientState').textContent=connection.state==='connected'?connection.mac:connection.state==='disconnected'?'Не подключён':connection.state;
    $('connectButton').disabled=!active||!same||connection.state!=='disconnected';$('disconnectButton').disabled=connection.state==='disconnected';$('stopButton').disabled=!active;
    showRoutingHint(s);
    $('metricRequests').textContent=s.requests??0;$('metricCache').textContent=s.cache_hits??0;$('metricErrors').textContent=s.failures??0;$('metricRunning').textContent=active?'Работает':'Выкл.';
    $('liveBadge').textContent=active?(Number(s.requests||0)>0?'LIVE · запросы поступают':'LIVE · нет запросов'):'LIVE · DNS остановлен';$('liveBadge').dataset.tone=active?'good':'bad';
    return s;
  }
  function formatTime(s){try{return new Date(s).toLocaleTimeString('ru-RU',{hour12:false})}catch(_){return s}}
  function renderLogs(){
    let traces=state.logs.slice().reverse(),filter=$('logFilter').value.trim().toLowerCase();
    if(filter)traces=traces.filter(t=>[t.name,t.client,t.assigned,t.used,t.transport].some(s=>String(s||'').toLowerCase().includes(filter)));
    if($('onlyErrors').checked)traces=traces.filter(t=>!t.success);
    $('logCounter').textContent=state.logs.length;
    const root=$('logRows');root.replaceChildren();
    if(!traces.length){const p=document.createElement('p');p.className='muted';p.textContent='Пока нет подходящих запросов. Открой сайт с подключённого устройства или проверь DNS.';root.appendChild(p);return;}
    for(const t of traces){
      const row=document.createElement('div');row.className='log-row'+(!t.success?' error':'');
      const vals=[formatTime(t.time),(t.name||'—')+(t.client?' · '+t.client:''),t.cache?'Кэш':(t.assigned||'—')+(t.used&&t.assigned!==t.used?' → '+t.used:''),t.rcode===5?'REFUSED · ACL':t.success?(t.rcode===3?'NXDOMAIN':t.rcode===0?'OK':'RCODE '+t.rcode):'Ошибка',t.latency_ms+' мс'];
      for(let i=0;i<vals.length;i++){const n=document.createElement('span');n.className=i===1?'col-domain':'';n.textContent=String(vals[i]);row.appendChild(n);}root.appendChild(row);
    }
  }
  async function refreshLogs(){const t=await api('service/traces');state.logs=Array.isArray(t.traces)?t.traces:[];renderLogs();$('logUpdated').textContent='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});}
  async function load(){const d=await api('config');if(!d.config||!d.revision||!Array.isArray(d.config.rules)||!Array.isArray(d.config.upstreams))throw Error('Неверный формат конфигурации DNS');state.config=d.config;state.revision=d.revision;state.dirty=false;state.editing=null;state.editorBase=null;$('selectedDNSBadge').textContent='DNS не выбран';$('listEditor').hidden=true;render();await refreshStatus();show('Настройки загружены. Можно менять DNS и списки.','good');}
  async function task(fn){if(state.busy)return;busy(true);try{await fn();}catch(e){show(e.message,'bad');}finally{busy(false);if(state.status){try{await refreshStatus();}catch(_){}}}}
  function on(id,fn){$(id).addEventListener('click',()=>task(fn));}
  $('navDns').addEventListener('click',()=>setPage('dns'));$('navLogs').addEventListener('click',()=>setPage('logs'));
  on('reloadButton',async()=>{if((state.dirty||editorPending())&&!confirm('Отменить все несохранённые изменения, включая текст доменов?'))return;await load();});
  $('addDnsToggle').addEventListener('click',()=>{$('dnsAddPanel').hidden=false;reportHeight();});$('cancelDNS').addEventListener('click',()=>{$('dnsAddPanel').hidden=true;});
  for(const id of ['newAddress','newProtocol'])$(id).addEventListener('input',()=>{state.checked=null;$('commitDNS').disabled=true;$('certificateInfo').textContent='Адрес изменён. Нажми «Проверить». ';$('certificateDetails').hidden=true;});
  $('newProtocol').addEventListener('change',()=>{$('newAddress').placeholder=$('newProtocol').value==='doh'?'https://dns.example.com/dns-query':$('newProtocol').value==='udp'?'9.9.9.9':'dns.example.com';});
  on('inspectDNS',inspect);on('commitDNS',addDNS);
  $('closeEditor').addEventListener('click',closeEditor);$('editorDomains').addEventListener('input',()=>{$('editorCount').textContent=$('editorDomains').value.split(/\r?\n/).filter(x=>x.trim()).length+' строк (черновик)';});on('saveDomains',saveList);
  $('importDomains').addEventListener('click',()=>$('domainFile').click());$('domainFile').addEventListener('change',async()=>{const f=$('domainFile').files[0];if(!f)return;try{if(f.size>256*1024)throw Error('Максимум 256 КБ');const names=parseDomains(await f.text());$('editorDomains').value=names.join('\n');show('Импортировано '+names.length+' доменов. Нажми «Сохранить список».');}catch(e){show(e.message,'bad');}finally{$('domainFile').value='';}});
  $('exportDomains').addEventListener('click',()=>{if(!state.editing)return;const data=$('editorDomains').value;const url=URL.createObjectURL(new Blob([data.endsWith('\n')?data:data+'\n'],{type:'text/plain;charset=utf-8'}));try{const a=document.createElement('a');a.href=url;a.download='routerforge-domains.txt';a.click();}finally{URL.revokeObjectURL(url);}});
  $('defaultDNS').addEventListener('change',()=>{mark();});for(const id of ['serviceListen','cacheSize','allowedNetworks'])$(id).addEventListener('change',()=>{state.dirty=true;show('Есть несохранённые изменения. Действующий DNS не менялся.');$('applyButton').textContent='✓ Сохранить и применить';});
  on('saveButton',save);on('applyButton',apply);on('migrateNative',migrate);
  on('gatePreflight',async()=>{
    await api('service/gate/control',opts('service-control',{operation:'preflight'},true));
    show('PRECHECK: готовность br0 и DNS listener подтверждена. IPv4/IPv6 kernel gate проверится при активации. Настройки не изменены.','good');
  });
  on('gateEnable',async()=>{
    if(state.dirty || editorPending())throw Error('Сначала сохрани и примени все изменения DNS.');
    if(!confirm('Включить RouterForge для ВСЕХ обычных DNS-запросов UDP/TCP 53 сети br0? Приватные DoH/DoT приложений не перехватываются. Штатные настройки Keenetic останутся без изменений.'))return;
    await api('service/gate/control',opts('service-control',{operation:'enable'},true));
    await refreshStatus();show('DNS Gate включён для LAN. Проверь реальные запросы планшета в журнале.','good');
  });
  on('gateDisable',async()=>{
    if(!confirm('Отключить системный DNS Gate и вернуть обычную обработку Keenetic?'))return;
    await api('service/gate/control',opts('service-control',{operation:'disable'},true));
    await refreshStatus();show('Системный DNS Gate отключён. Клиенты снова используют Keenetic.','good');
  });

  on('verifyButton',async()=>{const r=await serviceCommand('verify',{operation:'verify'});if(r.state!=='PASS')throw Error('Проверка DNS: '+r.state);show('Проверено '+r.results.length+' прямых UDP/TCP запросов. Подключение устройства проверяется отдельно.','good');});
  on('stopButton',async()=>{if(!confirm('Остановить DNS и вернуть прежний DNS подключённого устройства?'))return;await serviceCommand('control',{operation:'stop'});show('DNS остановлен, прежний DNS устройства возвращён.','good');});
  on('connectButton',async()=>{const mac=$('clientMAC').value.trim(),persist=$('persistClient').checked;if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(mac))throw Error('Укажи MAC в формате 02:11:22:33:44:55');if(!confirm('Назначить DNS Router устройству '+mac+'?'+(persist?' Keenetic сохранит общую конфигурацию.':'')))return;await serviceCommand('connect',{mac,persist});show('Устройство подключено. Открой журнал, затем попробуй открыть сайт на устройстве.','good');});
  on('disconnectButton',async()=>{if(!confirm('Вернуть прежний DNS выбранного устройства?'))return;await serviceCommand('disconnect',{operation:'disconnect'});show('Прежний DNS восстановлен.','good');});
  on('refreshLogs',async()=>{await refreshStatus();await refreshLogs();});$('logFilter').addEventListener('input',renderLogs);$('onlyErrors').addEventListener('change',renderLogs);
  $('copyLogs').addEventListener('click',()=>{const lines=state.logs.map(t=>[t.time,t.name,t.assigned,t.used,t.transport,t.rcode,t.success?'PASS':'FAIL',t.latency_ms].join('\t')).join('\n');if(navigator.clipboard?.writeText)navigator.clipboard.writeText(lines).then(()=>show('Журнал скопирован.','good')).catch(()=>show('Не удалось скопировать журнал.','bad'));else show('Копирование недоступно в этом браузере.','bad');});
  const root=document.documentElement,params=new URLSearchParams(window.location.search);
  function syncTheme(){try{if(window.parent===window)return;const host=window.parent.document.documentElement,computed=window.parent.getComputedStyle(host);for(const token of ['--rf-bg','--rf-surface','--rf-surface-2','--rf-text','--rf-muted','--rf-border','--rf-accent','--rf-accent-soft']){const val=computed.getPropertyValue(token).trim();if(val)root.style.setProperty(token,val);}root.style.colorScheme=computed.colorScheme==='light'?'light':'dark';}catch(_){}}
  function reportHeight(){if(window.parent===window)return;try{window.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height:Math.ceil(Math.max(350,$('dnsLiteApp').scrollHeight,$('dnsLiteApp').getBoundingClientRect().height)+6)},window.location.origin);}catch(_){}}
  syncTheme();try{if(window.parent!==window&&window.MutationObserver)new MutationObserver(()=>{syncTheme();reportHeight();}).observe(window.parent.document.documentElement,{attributes:true,attributeFilter:['style','class','data-theme']});}catch(_){}
  if(window.ResizeObserver)new ResizeObserver(reportHeight).observe($('dnsLiteApp'));
  window.addEventListener('resize',reportHeight);
  setInterval(()=>{if(state.page!=='logs'||document.hidden||state.busy)return;Promise.all([refreshStatus(),refreshLogs()]).then(()=>{state.lastLogError='';}).catch(e=>{if(state.lastLogError!==e.message){state.lastLogError=e.message;show('LIVE: '+e.message,'bad');}});},2000);
  setPage(params.get('view')==='logs'?'logs':'dns');
  load().catch(e=>{show('Не удалось открыть DNS Router: '+e.message,'bad');$('serviceBadge').textContent='Недоступен';$('serviceBadge').dataset.tone='bad';});
})();
