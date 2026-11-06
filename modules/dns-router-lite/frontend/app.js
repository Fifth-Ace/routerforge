'use strict';
// DEV-only RouterForge DNS Lite. No old P10 handlers, no HTML injection.
(() => {
  const $ = id => document.getElementById(id);
  const state = {config:{schema_version:1,upstreams:[],rules:[]},revision:'',dirty:false,busy:false,editing:null,editingList:'__legacy__',editingDNSId:null,editorBase:null,checked:null,status:null,logs:[],frozenLogs:[],logPaused:false,health:[],page:'dns',lastLogError:''};
  const used = id => state.config.rules.filter(r=>r.upstream===id);
  const listKey = r => r.list_id || '__legacy__';
  const listRules = (id,key) => used(id).filter(r=>listKey(r)===key);
  const listEntries = id => {
    const entries=(state.config.lists||[]).filter(l=>l.upstream===id);
    return (used(id).some(r=>!r.list_id)||!entries.length?[{id:'__legacy__',name:'Основной список (старый формат)',kind:'paste'}]:[]).concat(entries);
  };
  const listIsEditable = (id,key) => {
    const meta=(state.config.lists||[]).find(l=>l.id===key);
    return (!meta||(meta.kind!=='url'&&meta.match==='suffix'&&!meta.group_id)) && !listRules(id,key).some(r=>!isSimple(r));
  };
  const traceEndpoint=(t,field) => {
    if(t.cache)return 'Кэш · локально';
    const direct=t[field+'_endpoint'];if(direct)return direct;
    const id=t[field];
    if(id==='system-local')return 'Локальный DNS Keenetic';
    if(id==='client-acl'||id==='refused')return 'Отказ ACL';
    const u=state.config.upstreams.find(x=>x.id===id);
    return u ? (u.server_name?u.server_name+' ('+u.address+')':u.address)+' · '+u.transport+' ['+u.id+']' : (id||'—');
  };
  const protocolNames={doh:'DoH · HTTPS',dot:'DoT · TLS',doq:'DoQ · QUIC',udp:'UDP',tcp:'TCP',native:'Старый DNS Keenetic'};
  const isEncrypted = t => t==='doh'||t==='dot'||t==='doq';
  const isSimple = r => r.match==='suffix' && !r.group_id && Object.keys(r).every(k=>['id','domain','match','upstream','priority','list_id'].includes(k));
  const newId = (prefix,items) => {let i=1;const ids=new Set(items.map(x=>x.id));while(ids.has(prefix+'-'+i))i++;return prefix+'-'+i;};
  const displayName = u => u.server_name || (u.transport==='native'?'Наследованный DNS Keenetic':u.address);
  const hostName = u => protocolNames[u.transport]+' · '+displayName(u);
  const editorPending=()=>state.editing!==null && $('editorDomains').value!==state.editorBase;
  const show = (message,tone='') => {const el=$('feedback');el.textContent=message;el.dataset.tone=tone;};
  const busy = on => {state.busy=on;for(const id of ['saveButton','applyButton','checkAllDNS','addDnsToggle','inspectDNS','commitDNS','migrateNative','connectButton','disconnectButton','verifyButton','stopButton','gatePreflight','gateEnable','gateDisable','newListButton','deleteListButton','renameListButton','p21Refresh','p21Save','p21Apply','p21SetDevice','p21SetGroup','p21SetRule','p21Probe']){const el=$(id);if(el)el.disabled=on;}if(!on){const key=$('newProtocol').value+'|'+$('newAddress').value.trim();const c=state.checked;$('commitDNS').disabled=!c||c.key!==key||c.editId!==state.editingDNSId;if(state.status)renderGate(state.status);}};
  async function api(path,options={}) {
    const resp=await fetch('../'+path,{cache:'no-store',credentials:'same-origin',...options});
    const ct=resp.headers && resp.headers.get ? (resp.headers.get('content-type')||'') : 'application/json';
    if(!ct.toLowerCase().includes('json'))throw Error('Ожидался JSON от RouterForge, получен другой ответ: '+path);
    const data=await resp.json();
    if(!resp.ok)throw Error((data&&data.error)||('HTTP '+resp.status+' · '+path));
    return data;
  }
  document.addEventListener('routerforge:p28-p31-change',()=>{state.dirty=true;show('Есть несохранённые изменения. Действующий DNS не менялся.');$('applyButton').textContent='✓ Сохранить и применить';});
  const opts=(action,body,match)=>({method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...(match?{'If-Match':'"'+state.revision+'"'}:{})},body:JSON.stringify(body)});
  function mark(){state.dirty=true;show('Есть несохранённые изменения. Действующий DNS не менялся.');render();}
  function setPage(page){state.page=page;$('pageDns').hidden=page!=='dns';$('pageLogs').hidden=page!=='logs';$('p21Page').hidden=page!=='devices';for(const [id,target] of [['navDns','dns'],['navLogs','logs'],['p21Nav','devices']]){$(id).classList.toggle('selected',page===target);if(page===target)$(id).setAttribute('aria-current','page');else $(id).removeAttribute('aria-current');}if(page==='logs')refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));reportHeight();}
  const option = (value,label) => {const el=document.createElement('option');el.value=value;el.textContent=label;return el;};
  function render(){
    p21Render();
    const list=state.config.upstreams||[];
    const primary=$('defaultDNS');const saved=primary.value||state.config.service?.default_upstream||'';
    primary.replaceChildren(option('','Выбери основной DNS…'));
    for(const u of list)primary.appendChild(option(u.id,hostName(u)));
    primary.value=list.some(u=>u.id===saved)?saved:'';
    $('serverCount').textContent=list.length+' DNS';
    $('nativeWarning').hidden=!list.some(u=>u.transport==='native'&&used(u.id).length);
    const root=$('dnsCards');if(!window.__rfDnsLiteCardsLoaded)root.replaceChildren();
    $('editorEmpty').hidden=Boolean(state.editing);$('listEditor').hidden=!state.editing;
    if(window.__rfDnsLiteCardsLoaded){
      const cards=list.map(u=>{
        const rules=used(u.id);
        const namedLists=(state.config.lists||[]).filter(l=>l.upstream===u.id).length;
        const listCount=namedLists+(rules.some(r=>!r.list_id)||!namedLists?1:0);
        const fallbackOptions=list.filter(t=>t.id!==u.id&&t.transport!=='native'&&!(t.fallbacks||[]).length)
          .map(t=>({id:t.id,label:displayName(t)}));
        return {id:u.id,title:hostName(u),transport:u.transport,native:u.transport==='native',
          address:u.address,path:u.path||'',listCount,domainCount:rules.length,
          domains:rules.map(r=>r.domain),fallback:(u.fallbacks||[])[0]||'',fallbackOptions};
      });
      window.dispatchEvent(new CustomEvent('routerforge:dns-lite-cards',{
        detail:{cards,selected:state.editing||''}
      }));
    }else{
    if(!list.length){const p=document.createElement('p');p.className='muted';p.textContent='Добавь первый DNS-сервер. Здесь появятся его список доменов и состояние сертификата.';root.appendChild(p);}
    for(const u of list){
      const rules=used(u.id),card=document.createElement('article');card.className='dns-card';card.dataset.upstreamId=u.id;card.classList.toggle('is-selected',state.editing===u.id);
      const namedLists=(state.config.lists||[]).filter(l=>l.upstream===u.id).length;
      const listCount=namedLists+(rules.some(r=>!r.list_id)||!namedLists?1:0);
      const header=document.createElement('div');header.className='dns-top';const left=document.createElement('div');left.className='dns-title';
      const name=document.createElement('strong');name.textContent=hostName(u);left.appendChild(name);
      const dest=document.createElement('small');dest.textContent=u.address+(u.path?' · '+u.path:'');left.appendChild(dest);
      const count=document.createElement('span');count.className='dns-count';count.textContent=listCount+' сп. · '+rules.length+' дом.';
      count.title=listCount+' списков, '+rules.length+' доменов';header.append(left,count);card.appendChild(header);
      const healthRow=document.createElement('div');healthRow.className='dns-health';
      const stateLabel=document.createElement('span');stateLabel.className='dns-health-status';stateLabel.textContent='Не проверялся';
      const rtt=document.createElement('span');rtt.className='dns-rtt';rtt.textContent='RTT: —';
      const p95=document.createElement('span');p95.className='dns-p95';p95.textContent='p95: —';
      healthRow.append(stateLabel,rtt,p95);card.appendChild(healthRow);
      const metrics=document.createElement('div');metrics.className='dns-card-metrics';
      const listsMetric=document.createElement('span');listsMetric.textContent='Списки  '+listCount;
      const domainsMetric=document.createElement('span');domainsMetric.textContent='Домены  '+rules.length;
      metrics.append(listsMetric,domainsMetric);card.appendChild(metrics);
      const preview=document.createElement('p');preview.className='dns-preview';preview.textContent=rules.length?'Привязка: '+rules.slice(0,2).map(r=>r.domain).join(' · ')+(rules.length>2?' · +'+(rules.length-2):''):'Без доменов';card.appendChild(preview);
      const actions=document.createElement('div');actions.className='dns-card-actions';
      const edit=document.createElement('button');edit.type='button';edit.className='button dns-edit'+(state.editing===u.id?' primary':'');edit.textContent='Списки';edit.disabled=u.transport==='native';edit.addEventListener('click',()=>editList(u.id));actions.appendChild(edit);
      if(u.transport!=='native'){
        const change=document.createElement('button');change.type='button';change.className='button dns-change';change.textContent='Изменить';change.title='Изменить DNS / обновить TLS';change.addEventListener('click',()=>openDNSForm(u.id));actions.appendChild(change);
      }
      const check=document.createElement('button');check.type='button';check.className='button dns-check';check.textContent='Проверить';check.disabled=u.transport==='native';check.addEventListener('click',()=>task(async()=>{await checkHealth(u.id);}));actions.appendChild(check);
      const advanced=document.createElement('details');advanced.className='dns-advanced';const advLabel=document.createElement('summary');advLabel.textContent='Ещё';advanced.appendChild(advLabel);
      const bottom=document.createElement('div');bottom.className='dns-bottom';
      const reserve=document.createElement('label');reserve.className='field';const l=document.createElement('span');l.textContent='Резервный DNS';reserve.appendChild(l);
      const select=document.createElement('select');select.className='dns-select';select.appendChild(option('','Без резерва'));
      for(const t of list.filter(t=>t.id!==u.id&&t.transport!=='native'&&!(t.fallbacks||[]).length))select.appendChild(option(t.id,displayName(t)));
      select.value=(u.fallbacks||[])[0]||'';if(![...select.options].some(o=>o.value===select.value))select.value='';
      select.disabled=u.transport==='native';select.addEventListener('change',()=>{if(select.value&&state.config.upstreams.some(x=>(x.fallbacks||[]).includes(u.id))){show('Этот DNS уже является резервным. Сначала убери его из чужой резервной цепочки.','bad');render();return;}u.fallbacks=select.value?[select.value]:[];mark();});reserve.appendChild(select);bottom.appendChild(reserve);
      const del=document.createElement('button');del.type='button';del.className='button danger';del.textContent='Удалить DNS';del.addEventListener('click',()=>task(()=>removeDNS(u.id),false));bottom.appendChild(del);
      advanced.appendChild(bottom);actions.appendChild(advanced);card.appendChild(actions);root.appendChild(card);
    }
    }
    const chooser=$('editorDNSChoice');
    const managed=list.filter(u=>u.transport!=='native');
    chooser.replaceChildren(option('','Выбери DNS…'),...managed.map(u=>option(u.id,hostName(u))));
    chooser.value=managed.some(u=>u.id===state.editing)?state.editing:'';
    chooser.disabled=!managed.length;
    if(state.editing)renderListToolbar();
    if(state.config.service){$('serviceListen').value=state.config.service.listen;$('cacheSize').value=state.config.service.cache_bytes/(1024*1024);$('allowedNetworks').value=state.config.service.allowed_networks.join(',');}
    const p27=state.config.service?.protection;
    $('p27RateLimit').value=p27?.rate_limit_rps||0;
    $('p27RefuseAny').checked=Boolean(p27?.refuse_any);
    $('p27BogusIPs').value=(p27?.bogus_nxdomain||[]).join('\n');
    if($('p28Enabled')){
    const p28=state.config.service?.upstream_selection||{};
    $('p28Enabled').checked=Boolean(p28.enabled);$('p28Mode').value=p28.mode||'sequential';$('p28Timeout').value=p28.timeout_ms||3000;
    const p29=state.config.service?.performance||{};
    $('p29Enabled').checked=Boolean(p29.enabled);$('p29MinTTL').value=p29.cache_min_ttl||0;$('p29MaxTTL').value=p29.cache_max_ttl||0;
    $('p29Optimistic').checked=Boolean(p29.optimistic_cache);$('p29Pending').value=p29.pending_requests||'default';$('p29Concurrent').value=p29.max_concurrent||32;
    const p30=state.config.service?.privacy||{};
    $('p30Enabled').checked=Boolean(p30.enabled);$('p30ECSMode').value=p30.ecs_mode||'inherit';$('p30ECSAddress').value=p30.ecs_address||'';
    $('p30DNSSECMode').value=p30.dnssec_mode||'off';
    const p31d=state.config.service?.dns64||{};
    $('p31DNS64Enabled').checked=Boolean(p31d.enabled);$('p31DNS64Prefix').value=p31d.prefix||'64:ff9b::/96';$('p31NAT64Confirmed').checked=Boolean(p31d.nat64_confirmed);
    const p31r=state.config.service?.reverse_dns||{};
    $('p31ReverseEnabled').checked=Boolean(p31r.enabled);
    const reverse=$('p31ReverseUpstream'),selected=p31r.upstream||'';
    reverse.replaceChildren(option('','Выбери прямой DNS…'),...state.config.upstreams.filter(u=>u.transport!=='native').map(u=>option(u.id,hostName(u))));reverse.value=selected;
    }
    $('p241IPEnabled').checked=Boolean(state.config.service?.ip_intelligence?.enabled);
    $('p241IPMode').value=state.config.service?.ip_intelligence?.mode||'original';
    const p242health=state.config.service?.ip_intelligence?.health;
    $('p242HealthEnabled').checked=Boolean(p242health?.enabled);
    $('p242HealthProtocol').value=p242health?.protocol||'tcp';
    $('p242HealthPort').value=p242health?.port||443;
    $('p242HealthInterval').value=p242health?.interval_seconds||60;
    $('p242HealthTimeout').value=p242health?.timeout_ms||1200;
    $('p242HealthFailures').value=p242health?.failure_threshold||2;
    $('p243Recoveries').value=p242health?.recovery_threshold||2;
    $('p243MinDown').value=p242health?(p242health.min_down_seconds??0):30;
    $('p243HTTPPath').value=p242health?.http_path||'/';
    $('p243Scope').value=state.config.service?.ip_intelligence?.scope||'all';
    $('p243Domains').value=(state.config.service?.ip_intelligence?.domains||[]).join('\n');
    const chosenLists=state.config.service?.ip_intelligence?.list_ids||[];
    $('p243ListIDs').replaceChildren(...(state.config.lists||[]).map(x=>{const o=new Option((x.name||x.id)+' · '+x.id,x.id);o.selected=chosenLists.includes(x.id);return o;}));
    document.dispatchEvent(new Event('routerforge:p243-render'));
    document.dispatchEvent(new Event('routerforge:p241-render'));
    $('listenerSummary').textContent=state.config.service?.listen||$('serviceListen').value;
    $('applyButton').textContent=state.dirty?'✓ Сохранить и запустить DNS':'▶ Запустить DNS';
    if(state.status)renderGate(state.status);
    renderHealth();
    reportHeight();
  }
  function healthByID(id){return state.health.find(x=>x.id===id)||null;}
  function healthStateText(h){if(!h)return 'Не проверялся';return {UP:'Доступен',DOWN:'Недоступен',STALE:'Данные устарели',UNKNOWN:'Не проверялся',UNSUPPORTED:'Keenetic'}[h.status]||'Не проверялся';}
  function renderHealth(){
    if(window.__rfDnsLiteCardsLoaded){
      window.dispatchEvent(new CustomEvent('routerforge:dns-lite-card-health',{
        detail:{upstreams:state.health.map(h=>({...h}))}
      }));
    }
    let up=0,checks=0,requests=0,latencies=[];
    for(const u of state.config.upstreams){
      const h=healthByID(u.id);if(h?.status==='UP')up++;if(h&&h.status!=='UNKNOWN'&&h.status!=='UNSUPPORTED')checks++;
      requests+=Number(h?.requests_5m||0);
      if(h?.requests_5m>h?.failures_5m)latencies.push(Number(h?.p95_latency_5m_ms||0));
      if(window.__rfDnsLiteCardsLoaded)continue; // Svelte owns the reactive health cells
      const card=[...$('dnsCards').children].find(c=>c.dataset.upstreamId===u.id);if(!card)continue;
      const status=card.querySelector('.dns-health-status');const rtt=card.querySelector('.dns-rtt');const p95=card.querySelector('.dns-p95');
      status.textContent=healthStateText(h);status.dataset.state=(h?.status||'').toLowerCase();
      status.title=h?.last_error||'';
      rtt.textContent=h?.status==='UP'?'DNS RTT: '+h.probe_latency_ms+' мс':'DNS RTT: —';
      p95.textContent=h?.requests_5m>h?.failures_5m?'p95: '+h.p95_latency_5m_ms+' мс':'p95: —';
    }
    $('healthServers').textContent=checks?up+'/'+state.config.upstreams.length:'— / '+state.config.upstreams.length;
    $('healthRequests').textContent=requests;
    $('healthP95').textContent=latencies.length?Math.max(...latencies)+' мс':'—';
    $('healthUpdated').textContent='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});
  }
  async function refreshHealth(){
    const data=await api('service/upstreams/health');state.health=Array.isArray(data.upstreams)?data.upstreams:[];renderHealth();return data;
  }
  async function checkHealth(id=''){
    const result=await api('service/upstreams/check',opts('service-control',{operation:'check',upstream:id},true));
    state.health=Array.isArray(result.upstreams)?result.upstreams:[];renderHealth();
    const failed=id?state.health.find(h=>h.id===id&&h.status==='DOWN'):null;show(failed?'DNS недоступен: '+(failed.last_error||'нет ответа'):id?'DNS проверен. Результат и RTT обновлены.':'Проверка всех DNS завершена. Результаты обновлены.',failed?'bad':'good');
  }
  async function refreshNativeDNS(){
    const target=$('nativeDNSHealth');
    try{
      const response=await fetch('/api/modules/dns/quality?minutes=5',{credentials:'same-origin',cache:'no-store'});
      if(!response.ok)throw Error('HTTP '+response.status);
      const data=await response.json();if(!Array.isArray(data.upstreams))throw Error('неверный формат');
      target.textContent='Модуль DNS доступен · '+data.upstreams.length+' резолверов Keenetic · отдельная статистика';
    }catch(_){target.textContent='Модуль DNS недоступен — RouterForge DNS Router работает независимо';}
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
  function renderListToolbar(){
    if(!state.editing)return;
    const entries=listEntries(state.editing),choice=$('listChoice');
    if(!entries.some(e=>e.id===state.editingList))state.editingList=entries[0].id;
    choice.replaceChildren(...entries.map(e=>option(e.id,e.name+' · '+listRules(state.editing,e.id).length+' доменов')));
    choice.value=state.editingList;
    const meta=(state.config.lists||[]).find(l=>l.id===state.editingList);
    $('listRenameName').value=meta?meta.name:'';
    $('listRenameName').disabled=!meta;
    $('renameListButton').disabled=!meta;
    $('deleteListButton').disabled=!meta;
    const editable=listIsEditable(state.editing,state.editingList);
    $('editorDomains').readOnly=!editable;
    $('saveDomains').disabled=!editable;
    $('importDomains').disabled=!editable;
    $('editorTitle').textContent=(meta?.name||'Основной список')+' · '+displayName(state.config.upstreams.find(u=>u.id===state.editing));
    if(!editable)$('editorHint').textContent='Этот URL/сложный список управляется отдельным механизмом. Ручное редактирование заблокировано.';
  }
  function switchList(key){
    if(editorPending()&&!confirm('Несохранённый текст текущего списка будет потерян. Переключить?')){$('listChoice').value=state.editingList;return;}
    if(!listEntries(state.editing).some(l=>l.id===key))throw Error('Неизвестный список');
    state.editingList=key;
    const rules=listRules(state.editing,key);
    $('editorDomains').value=rules.map(r=>r.domain).join('\n');state.editorBase=$('editorDomains').value;
    $('editorCount').textContent=rules.length+' доменов';
    $('editorHint').textContent='Редактируется только выбранный список. Остальные списки и DNS не меняются.';
    renderListToolbar();reportHeight();
  }
  function editList(id){
    if(editorPending()){if(!confirm('У выбранного DNS есть несохранённый текст списка. Перейти к другому DNS?'))return false;state.editorBase=$('editorDomains').value;}
    const u=state.config.upstreams.find(x=>x.id===id);
    if(!u||u.transport==='native')throw Error('Legacy DNS нужно сначала преобразовать');
    if(state.editing===null)state.editorBase=$('editorDomains').value;
    const changed=state.editing!==id;state.editing=id;
    if(changed)state.editingList=listEntries(id)[0].id;
    $('selectedDNSBadge').textContent=displayName(u);
    $('editorEmpty').hidden=true;$('listEditor').hidden=false;
    switchList(state.editingList);render();reportHeight();return true;
  }
  function closeEditor(){
    if(editorPending()&&!confirm('Закрыть редактор? Несохранённый текст будет потерян.'))return;
    state.editing=null;state.editingList='__legacy__';state.editorBase=null;
    $('listEditor').hidden=true;$('editorEmpty').hidden=false;
    $('selectedDNSBadge').textContent='DNS не выбран';render();
  }
  function saveList(){
    if(!state.editing)return;
    const id=state.editing,key=state.editingList,old=listRules(id,key);
    if(!listIsEditable(id,key))throw Error('URL или сложный список нельзя менять через простой редактор');
    const names=parseDomains($('editorDomains').value);
    const meta=(state.config.lists||[]).find(l=>l.id===key);
    const other=new Set(state.config.rules.filter(r=>!(r.upstream===id&&listKey(r)===key)).map(r=>r.domain));
    const collision=names.find(domain=>other.has(domain));
    if(collision)throw Error('Домен '+collision+' уже есть в другом списке или DNS. Сначала убери дубликат там.');
    const oldMap=new Map(old.map(r=>[r.domain,r])),ids=new Set(state.config.rules.map(r=>r.id));
    const fresh=names.map(domain=>{
      if(oldMap.has(domain))return oldMap.get(domain);
      let n=1;while(ids.has('domain-'+n))n++;const uid='domain-'+n;ids.add(uid);
      return {id:uid,domain,match:'suffix',upstream:id,priority:meta?meta.priority:100,...(key==='__legacy__'?{}:{list_id:key})};
    });
    const unchanged=state.config.rules.filter(r=>!(r.upstream===id&&listKey(r)===key));
    if(unchanged.length+fresh.length>10000)throw Error('Общий лимит: 10 000 правил');
    state.config.rules=unchanged.concat(fresh);
    $('editorDomains').value=names.join('\n');state.editorBase=$('editorDomains').value;
    $('editorCount').textContent=names.length+' доменов';
    $('editorHint').textContent='Список сохранён в черновик. Применение DNS выполняется отдельно.';
    mark();show('Список обновлён: '+names.length+' доменов. Остальные списки и DNS не менялись.');
  }
  function newList(){
    if(!state.editing||editorPending())throw Error('Сначала сохрани текст открытого списка');
    const name=$('newListName').value.trim(),all=state.config.lists||[];
    if(!name||Array.from(name).length>80)throw Error('Название списка: от 1 до 80 символов');
    if(all.length>=64)throw Error('Лимит: 64 списка');
    if(all.some(l=>l.upstream===state.editing&&l.name.toLowerCase()===name.toLowerCase()))throw Error('Список с таким названием уже существует');
    const id=newId('list',all);
    state.config.lists=all.concat({id,name,kind:'paste',upstream:state.editing,match:'suffix',priority:100});
    state.editingList=id;$('newListName').value='';switchList(id);mark();
    show('Новый список создан в черновике. Добавь домены и сохрани список.','good');
  }
  function renameList(){
    if(!state.editing||editorPending())throw Error('Сначала сохрани текст открытого списка');
    const list=(state.config.lists||[]).find(l=>l.id===state.editingList);
    if(!list)throw Error('Основной список старого формата нельзя переименовать');
    const name=$('listRenameName').value.trim();
    if(!name||Array.from(name).length>80)throw Error('Название списка: от 1 до 80 символов');
    if(state.config.lists.some(l=>l.id!==list.id&&l.upstream===list.upstream&&l.name.toLowerCase()===name.toLowerCase()))throw Error('Такое название уже используется');
    list.name=name;mark();show('Имя списка изменено в черновике.','good');
  }
  function deleteList(){
    if(!state.editing||editorPending())throw Error('Сначала сохрани текст открытого списка');
    const list=(state.config.lists||[]).find(l=>l.id===state.editingList);
    if(!list)throw Error('Основной список старого формата нельзя удалить через это действие');
    const size=listRules(state.editing,list.id).length;
    if(!confirm('Удалить список «'+list.name+'» и '+size+' правил только ИЗ ЧЕРНОВИКА?'))return;
    state.config.lists=state.config.lists.filter(l=>l.id!==list.id);
    state.config.rules=state.config.rules.filter(r=>r.list_id!==list.id);
    state.editingList=listEntries(state.editing)[0].id;switchList(state.editingList);mark();
  }
  function removeDNS(id){
    const u=state.config.upstreams.find(x=>x.id===id);if(!u)return;
    if((state.config.dns_groups||[]).some(g=>g.servers?.includes(id))||state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))throw Error('Этот DNS используется группой или резервной цепочкой. Сначала убери зависимость.');
    if(p21UsesDNS(id))throw Error('Этот DNS используется правилами устройств. Сначала измени их.');
    const rules=used(id);if(rules.some(r=>!isSimple(r)))throw Error('Есть сложные правила, удаление через простой экран заблокировано.');
    if(!confirm('Удалить '+displayName(u)+' и '+rules.length+' его доменов ИЗ ЧЕРНОВИКА?'))return;
    state.config.upstreams=state.config.upstreams.filter(x=>x.id!==id);state.config.rules=state.config.rules.filter(r=>r.upstream!==id);state.config.lists=(state.config.lists||[]).filter(l=>l.upstream!==id);
    if(state.config.service?.default_upstream===id){state.config.service=null;$('defaultDNS').value='';}
    if(state.editing===id)closeEditor();mark();
  }
  function openDNSForm(id){
    if(editorPending())throw Error('Сначала сохрани несохранённый текст списка');
    const u=id?state.config.upstreams.find(x=>x.id===id):null;
    if(id&&(!u||u.transport==='native'))throw Error('Этот DNS нельзя изменить через простой редактор');
    state.editingDNSId=id||null;state.checked=null;
    $('dnsAddPanelTitle').textContent=u?'Изменить DNS / обновить TLS':'Добавить DNS';
    $('commitDNS').textContent=u?'Сохранить DNS (черновик)':'Добавить DNS';
    $('newProtocol').value=u?u.transport:'doh';
    $('newAddress').value=!u?'':u.transport==='doh'?'https://'+u.server_name+(u.path||'/dns-query'):
      isEncrypted(u.transport)?u.server_name:u.address.replace(/:53$/,'');
    resetDraftInspection();
    $('certificateInfo').textContent=u?'Черновик: проверка не выполнялась. Активный DNS не изменён.':'Проверка не выполнялась.';
    $('certificateDetails').hidden=true;$('commitDNS').disabled=true;
    $('dnsAddPanel').hidden=false;reportHeight();
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
    if(transport==='udp'||transport==='tcp'){if(!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(raw)||raw.split('.').some(n=>Number(n)>255))throw Error('Для UDP укажи IPv4-адрес, например 9.9.9.9');host=raw;}
    if(isEncrypted(transport)&&(!/^[a-z0-9-]+(?:\.[a-z0-9-]+)+$/.test(host)))throw Error('Для защищённого DNS требуется имя сервера');
    return {transport,host,path};
  }
  function draftStatus(id,text,state='') {const el=$(id);el.textContent=text;el.dataset.state=state;}
  function resetDraftInspection(){
    for(const id of ['draftAddressState','draftIPState','draftTLSState','draftCertificateState','draftDNSState','draftRTTState'])draftStatus(id,'Не проверено');
    $('draftProbeError').textContent='';$('draftProbeError').hidden=true;
  }
  async function inspect(){
    state.checked=null;$('commitDNS').disabled=true;resetDraftInspection();
    let f;
    try {f=newFormData();}
    catch(e){draftStatus('draftAddressState','Ошибка','bad');$('draftProbeError').hidden=false;$('draftProbeError').textContent=e.message;return;}
    const key=$('newProtocol').value+'|'+$('newAddress').value.trim(),editId=state.editingDNSId;
    draftStatus('draftAddressState',f.host,'ok');draftStatus('draftIPState','Получение…','pending');
    draftStatus('draftTLSState',isEncrypted(f.transport)?'Ожидание…':'Не используется',isEncrypted(f.transport)?'pending':'neutral');
    draftStatus('draftCertificateState',(f.transport==='udp'||f.transport==='tcp')?'Не требуется':'Ожидание…',(f.transport==='udp'||f.transport==='tcp')?'neutral':'pending');
    let r;
    try {r=f.transport==='tcp'?{selected_ip:f.host,address:f.host+':53'}:await api('certificates/lookup',opts('certificate-lookup',{transport:f.transport,server_name:f.host}));}
    catch(e){
      if(key!==$('newProtocol').value+'|'+$('newAddress').value.trim()||editId!==state.editingDNSId)return;
      draftStatus('draftIPState','Не получен','bad');
      if(isEncrypted(f.transport))draftStatus('draftTLSState','Не подтверждён','bad');
      draftStatus('draftCertificateState',isEncrypted(f.transport)?'Не получен':'Не требуется',isEncrypted(f.transport)?'bad':'neutral');
      draftStatus('draftDNSState','Недоступен','bad');
      $('draftProbeError').hidden=false;$('draftProbeError').textContent=e.message;
      $('certificateInfo').textContent='Адрес / TLS: '+e.message;reportHeight();return;
    }
    if(key!==$('newProtocol').value+'|'+$('newAddress').value.trim()||editId!==state.editingDNSId)return;
    if(!r.address||!r.selected_ip){draftStatus('draftIPState','Не получен','bad');$('draftProbeError').hidden=false;$('draftProbeError').textContent='Не найден IP DNS-сервера';return;}
    draftStatus('draftIPState',r.address,'ok');
    const preverified=!!r.certificate?.verified;
    if((f.transport==='doh'||f.transport==='dot')&&!preverified){draftStatus('draftTLSState','Ошибка','bad');draftStatus('draftCertificateState','Не получен','bad');$('draftProbeError').hidden=false;$('draftProbeError').textContent='TLS-сертификат не удалось проверить';return;}
    draftStatus('draftTLSState',preverified?'Проверен':f.transport==='doq'?'QUIC: проверяется запросом':'Не используется',preverified?'ok':f.transport==='doq'?'pending':'neutral');
    draftStatus('draftCertificateState',preverified?'Получен · до '+(r.certificate.not_after||'—'):f.transport==='doq'?'Проверяется при QUIC':'Не требуется',preverified?'ok':'neutral');
    const candidate={id:'p194-draft',transport:f.transport,address:r.address};
    if(isEncrypted(f.transport))candidate.server_name=f.host;
    if(f.transport==='doh')candidate.path=f.path;
    draftStatus('draftDNSState','Проверка…','pending');
    let probe;
    try {probe=await api('service/upstreams/inspect',opts('service-control',{operation:'inspect',upstream:candidate}));}
    catch(e){probe={dns_available:false,status:'DOWN',last_error:e.message,tls_status:'UNCONFIRMED'};}
    if(key!==$('newProtocol').value+'|'+$('newAddress').value.trim()||editId!==state.editingDNSId)return;
    const available=probe.dns_available===true;
    draftStatus('draftDNSState',available?'Доступен':'Недоступен',available?'ok':'bad');
    draftStatus('draftRTTState',available?String(probe.probe_latency_ms)+' мс':'—',available?'ok':'neutral');
    if(f.transport==='doq'){
      draftStatus('draftTLSState',available?'Проверен при QUIC':'Не подтверждён',available?'ok':'bad');
      draftStatus('draftCertificateState',available?'Проверен транспортом':'Не получен',available?'ok':'bad');
    }
    $('draftProbeError').hidden=available;
    $('draftProbeError').textContent=available?'':'DNS недоступен: '+(probe.last_error||'нет корректного ответа');
    state.checked={key,form:f,lookup:r,editId,available};
    $('certificateInfo').textContent=(available?'✓ DNS доступен':'⚠ DNS недоступен')+' · IP '+r.selected_ip+' · '+(preverified?'TLS проверен':f.transport==='doq'?'DoQ TLS при запросе':'Без TLS');
    $('certificateDetails').hidden=!r.certificate;
    $('certificateDetailsText').textContent=r.certificate?['Имя: '+r.certificate.server_name,'Кому: '+r.certificate.subject,'Выдан: '+r.certificate.issuer,'Действителен до: '+r.certificate.not_after,'SHA256: '+r.certificate.fingerprint_sha256].join('\n'):'';
    $('commitDNS').disabled=false;reportHeight();
  }
  function addDNS(){
    if(editorPending())throw Error('Сначала сохрани изменённый список доменов справа.');
    const c=state.checked,key=$('newProtocol').value+'|'+$('newAddress').value.trim();
    if(!c||c.key!==key||c.editId!==state.editingDNSId)throw Error('Сначала проверь именно этот адрес DNS и TLS');
    if(c.available!==true&&!confirm('DNS недоступен. Сохранить его в черновик всё равно? Работающий DNS не изменится до применения.'))return;
    const f=c.form,r=c.lookup;
    const prev=state.editingDNSId?state.config.upstreams.find(u=>u.id===state.editingDNSId):null;
    if(state.editingDNSId&&!prev)throw Error('Редактируемый DNS не найден');
    if(state.config.upstreams.some(u=>u.id!==prev?.id&&u.transport===f.transport&&u.address===r.address&&u.server_name===(isEncrypted(f.transport)?f.host:'')&&(u.path||'')===f.path))throw Error('Такой DNS уже добавлен');
    const u=prev||{id:newId('dns',state.config.upstreams)};
    u.transport=f.transport;u.address=r.address;
    delete u.server_name;delete u.path;delete u.certificate_pem;
    if(isEncrypted(f.transport))u.server_name=f.host;
    if(f.transport==='doh')u.path=f.path;
    if(r.certificate?.verified)u.certificate_pem=r.certificate.pem;
    if(!prev)state.config.upstreams.push(u);
    state.editingDNSId=null;state.checked=null;$('dnsAddPanel').hidden=true;
    resetDraftInspection();$('certificateInfo').textContent='Проверка ещё не выполнялась.';$('newAddress').value='';
    mark();
    if(!prev)editList(u.id);
    else if(state.editing===u.id){$('selectedDNSBadge').textContent=displayName(u);renderListToolbar();}
    show(prev?'DNS/TLS обновлены в черновике. ID, списки, резерв и настройки по умолчанию сохранены.':'DNS добавлен. Его список открыт справа. Выбери DNS по умолчанию перед применением.','good');
  }
  function servicePrefs(){
    const primary=$('defaultDNS').value;
    if(!primary || !state.config.upstreams.some(u=>u.id===primary))throw Error('Выбери DNS по умолчанию');
    const cache=Number($('cacheSize').value);if(!Number.isInteger(cache)||cache<0||cache>8)throw Error('Кэш: от 0 до 8 МиБ');
    const listen=$('serviceListen').value.trim(),networks=$('allowedNetworks').value.split(',').map(s=>s.trim()).filter(Boolean);
    const prefs={listen,default_upstream:primary,cache_bytes:cache*1024*1024,allowed_networks:networks};
    const p27r=Number($('p27RateLimit').value);
    if(!Number.isInteger(p27r)||p27r<0||p27r>2000)throw Error('Защита DNS: лимит 0..2000 запросов/с');
    const p27ips=$('p27BogusIPs').value.split(/[\r\n,]+/).map(x=>x.trim()).filter(Boolean);
    if(p27ips.length>16)throw Error('Защита DNS: максимум 16 IP/CIDR');
    if(p27r>0||$('p27RefuseAny').checked||p27ips.length) {
      prefs.protection={rate_limit_rps:p27r,refuse_any:$('p27RefuseAny').checked,bogus_nxdomain:p27ips};
    }
    if($('p242HealthEnabled').checked && !$('p241IPEnabled').checked)throw Error('Сначала включи IP Intelligence');
    const asInt=(id,min,max)=>{const n=Number($(id).value);if(!Number.isInteger(n)||n<min||n>max)throw Error(id+': допустимо '+min+'..'+max);return n;};
    if($('p28Enabled')?.checked) prefs.upstream_selection={enabled:true,mode:$('p28Mode').value,timeout_ms:asInt('p28Timeout',250,5000)};
    if($('p29Enabled')?.checked) prefs.performance={enabled:true,cache_min_ttl:asInt('p29MinTTL',0,3600),cache_max_ttl:asInt('p29MaxTTL',0,86400),optimistic_cache:$('p29Optimistic').checked,pending_requests:$('p29Pending').value,max_concurrent:asInt('p29Concurrent',8,64)};
    if($('p30Enabled')?.checked) prefs.privacy={enabled:true,ecs_mode:$('p30ECSMode').value,ecs_address:$('p30ECSMode').value==='fixed'?$('p30ECSAddress').value.trim():'',dnssec_mode:$('p30DNSSECMode').value};
    if($('p31DNS64Enabled')?.checked) prefs.dns64={enabled:true,prefix:$('p31DNS64Prefix').value.trim(),nat64_confirmed:$('p31NAT64Confirmed').checked};
    if($('p31ReverseEnabled')?.checked){if(!$('p31ReverseUpstream').value)throw Error('Выбери DNS для приватного rDNS');prefs.reverse_dns={enabled:true,upstream:$('p31ReverseUpstream').value};}
    if($('p241IPEnabled').checked){
      prefs.ip_intelligence={enabled:true,mode:$('p241IPMode').value};
      const selectedScope=$('p243Scope').value;
      if(selectedScope==='domains'){
        const domains=parseDomains($('p243Domains').value);
        if(!domains.length||domains.length>128)throw Error('IP Intelligence: укажи 1..128 доменов');
        prefs.ip_intelligence.scope='domains';prefs.ip_intelligence.domains=domains;
      } else if(selectedScope==='lists'){
        const ids=Array.from($('p243ListIDs').selectedOptions,x=>x.value);
        if(!ids.length)throw Error('IP Intelligence: выбери хотя бы один список');
        prefs.ip_intelligence.scope='lists';prefs.ip_intelligence.list_ids=ids;
      }
      if($('p242HealthEnabled').checked)prefs.ip_intelligence.health={
        enabled:true,protocol:$('p242HealthProtocol').value,
        port:asInt('p242HealthPort',80,65535),
        interval_seconds:asInt('p242HealthInterval',30,3600),
        timeout_ms:asInt('p242HealthTimeout',250,3000),
         failure_threshold:asInt('p242HealthFailures',1,5),
         recovery_threshold:asInt('p243Recoveries',1,5),
         min_down_seconds:asInt('p243MinDown',0,3600),
         http_path:$('p243HTTPPath').value.trim()||'/'
      };
    }
    return prefs;
  }
  function syncPrefs(){const next=servicePrefs();const prev=state.config.service;const merged={...prev,...next};if(!next.ip_intelligence)delete merged.ip_intelligence;if(!next.protection)delete merged.protection;for(const k of ["upstream_selection","performance","privacy","dns64","reverse_dns"])if(!next[k] && $("p28Enabled"))delete merged[k];if(JSON.stringify(prev)!==JSON.stringify(merged)){state.config.service=merged;if(prev?.default_upstream!==next.default_upstream)delete state.config.service.default_group;state.dirty=true;}}
  async function save(){if(editorPending())throw Error('Сначала нажми «Сохранить список» в редакторе доменов.');syncPrefs();if(!state.dirty)return;const data=await api('config',{method:'PUT',headers:{'Content-Type':'application/json','X-RouterForge-Action':'save','If-Match':'"'+state.revision+'"'},body:JSON.stringify(state.config)});state.revision=data.revision;state.dirty=false;show('Настройки сохранены. Действующий DNS не изменился. Для запуска нажми «Сохранить и запустить DNS».','good');render();}
  async function serviceCommand(path,body){return api('service/'+path,opts('service-control',body,true));}
  function hasUsedNative(){return state.config.upstreams.some(u=>u.transport==='native'&&(used(u.id).length||state.config.service?.default_upstream===u.id));}
  async function apply(){
    if(hasUsedNative())throw Error('Есть старые native DNS. Сначала нажми «Преобразовать DNS» и проверь адреса.');
    const prefs=servicePrefs();
    if(!confirm('Сохранить настройки и запустить DNS Router на '+prefs.listen+'? Это ещё не переключит всю LAN. Для этого отдельно включи «Основной DNS для LAN».'))return;
    await save();const s=await api('service/status');
    await serviceCommand('control',{operation:s.active?'apply':'start'});
    show('DNS Router запущен. Проверь готовность, затем включи «Основной DNS для LAN». Отдельное тестовое устройство не требуется.','good');await refreshStatus();
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
    if(s.system_gate?.active)return {tone:'good',text:'Основной DNS для LAN активен. Сервис обработал '+upstream+' запросов. Это не аппаратное доказательство перехвата: проверь запрос устройства к внешнему DNS и соответствующую запись журнала.'};
    if(!s.active)return {tone:'bad',text:'DNS Router остановлен. На штатном DNS Keenetic (порт 53) интернет может продолжать работать, но журнал RouterForge пуст.'};
    if(c.state==='recovery_required')return {tone:'bad',text:'DNS-профиль устройства требует восстановления: '+(c.error||'состояние не совпадает с Keenetic')+'. Не устанавливай и не обновляй пакет до исправления профиля.'};
    if(c.state==='pending')return {tone:'bad',text:'Подключение устройства не завершено (pending). DNS Router слушает '+endpoint+', но профиль клиента Keenetic ещё не подтверждён. Сначала верни прежний DNS устройства или заверши подключение; не пытайся обновлять пакет с pending.'};
    const denied=Number(s.denied_requests||0),acl=Number(s.acl_denied_requests||0),protection=Number(s.protection_denied_requests||0);
    if(denied>0 && denied===upstream){
      if(acl===denied)return {tone:'bad',text:'До DNS Router дошли '+denied+' запросов, но ВСЕ отклонены (REFUSED) входным фильтром. Проверь разрешённые клиентские сети и корректность запросов; менять настройки Keenetic не требуется.'};
      if(protection===denied)return {tone:'warn',text:'До DNS Router дошли '+denied+' запросов, но ВСЕ отклонены защитой запросов (NOTIMP/REFUSED). Проверь настройки защиты и причины отказов в журнале.'};
      return {tone:'warn',text:'До DNS Router дошли '+denied+' запросов, но ВСЕ отклонены. Причины могут отличаться: открой журнал и проверь конкретные записи.'};
    }
    if(upstream===0)return {tone:'warn',text:'DNS Router слушает '+endpoint+', но получил 0 запросов. Планшет может использовать штатный DNS Keenetic (:53), свой Private DNS/DoH или другой DNS-профиль. '+(c.state==='connected'?'Профиль назначен, но прохождение трафика не подтверждено.':'Тестовый клиент не подключён через RouterForge.')+' Прямая кнопка «Проверить DNS» тестирует только сервис, не планшет.'};
    if(c.state!=='connected')return {tone:'warn',text:'DNS Router получил '+upstream+' запросов, но клиентский профиль не подключён. Это может быть ручная проверка сервиса; маршрут планшета через Keenetic не доказан.'};
    return {tone:'',text:'DNS Router получил '+upstream+' запросов на '+endpoint+'. Клиентский профиль Keenetic подключён, но чтобы подтвердить именно планшет, сопоставь время его DNS-запросов с журналом.'};
  }
  function showRoutingHint(s){const hint=routingHint(s);for(const id of ['routeAdvice','logGuide']){const node=$(id);node.textContent=hint.text;node.dataset.tone=hint.tone;}}
  function renderGate(s){
    const gate=s.system_gate||{state:'STOCK',active:false};
    const active=Boolean(gate.active),recovery=['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(gate.state),wanted=Boolean(gate.desired||s.desired?.interception_enabled);
    $('healthGate').textContent=active?'RouterForge':recovery?'Проверка':'Keenetic';
    $('systemGatePanel').dataset.state=recovery?'recovery':active?'active':'stock';
    $('gateBadge').textContent=gate.state==='CONFLICT'?'Конфликт':gate.state==='DEGRADED'?'Восстановление':recovery?'Требуется проверка':active?'Активен':'Выключен';
    $('gateCompactBadge').textContent=$('gateBadge').textContent;
    $('gateCompactBadge').dataset.tone=active?'good':recovery?'bad':'';
    $('gateBadge').dataset.tone=active?'good':recovery?'bad':'';
    $('gateEnable').disabled=state.busy||!s.active||active||recovery||state.dirty||editorPending();
    $('gateDisable').disabled=state.busy||(!active&&!recovery&&!wanted);
    $('gatePreflight').disabled=state.busy||!s.active;
    $('gatePreflight').textContent=wanted&&recovery?'Повторить восстановление':'Проверить готовность';
    $('stopButton').disabled=state.busy||!s.active;
    const hits=Number(s.requests||0),denied=Number(s.denied_requests||0);
    const coverage=gate.coverage||{},ipv6=coverage.ipv6_routed==='active_last_verified';
    const coverageText=active?('Перехват: '+(ipv6?'IPv4 и IPv6 global/ULA':'только IPv4')+'. Клиенты с IPv6 link-local source исключены. '):'';
    $('gateAdvice').textContent=recovery?('Перехват не подтверждён. Желаемое состояние: '+(wanted?'ВКЛ':'ВЫКЛ')+'. '+(gate.last_error||'Проверяем listener, порядок правил и lease')+'. Действие восстановления ограничено; сторонние правила не перезаписываются.'):
      active?(coverageText+'Gate проверен: listener, порядок правил и действующие lease. Запросов сервиса: '+hits+', ACL-отказов: '+denied+'. Реальный путь устройства подтверждается отдельной проверкой, не этим счётчиком.'):
      'Перехват выключен. Последний выбор ON/OFF сохраняется отдельно от черновика конфигурации; при обновлении и перезагрузке он восстанавливается после проверки готовности. Тестовое устройство не требуется.';
  }
  async function refreshStatus(){const s=await api('service/status');if(s.mode!=='independent-dns')throw Error('Загружен не DNS Lite. Установи актуальный DEV-пакет.');state.status=s;renderGate(s);
    const connection=s.connection||{state:'disconnected'},active=Boolean(s.active),same=active&&s.applied_revision===state.revision;
    $('serviceBadge').textContent=active?'DNS работает':'DNS остановлен';$('serviceBadge').dataset.tone=active?'good':'bad';
    $('runStatus').textContent=(active?'DNS работает: '+s.listen+' · Основной '+displayName(state.config.upstreams.find(u=>u.id===s.default_upstream)||{transport:'udp',address:s.default_upstream||'—'}):'Сервис остановлен.')+(active&&!same?' · Есть неприменённая версия настроек.':'')+(s.last_error?' · Ошибка: '+s.last_error:'');
    $('connectionSummary').textContent=s.system_gate?.active?'Перехват LAN включён — проверь запросы реального устройства':connection.state==='connected'?'Точечное устройство: '+connection.mac:connection.state==='recovery_required'?'Точечное подключение требует восстановления':'Для LAN-режима отдельное устройство не требуется';
    $('clientState').textContent=connection.state==='connected'?connection.mac:connection.state==='disconnected'?'Не подключён':connection.state;
    $('connectButton').disabled=!active||!same||connection.state!=='disconnected';$('disconnectButton').disabled=connection.state==='disconnected';$('stopButton').disabled=!active;
    showRoutingHint(s);
    $('metricRequests').textContent=s.requests??0;$('metricCache').textContent=s.cache_hits??0;$('metricErrors').textContent=s.failures??0;$('metricRunning').textContent=active?'Работает':'Выкл.';
    $('liveBadge').textContent=active?(Number(s.requests||0)>0?'LIVE · запросы поступают':'LIVE · нет запросов'):'LIVE · DNS остановлен';$('liveBadge').dataset.tone=active?'good':'bad';
    p21Render();
    return s;
  }
  function formatTime(s){try{return new Date(s).toLocaleTimeString('ru-RU',{hour12:false})}catch(_){return s}}
  const qtypeName=n=>({1:'A',28:'AAAA',15:'MX',16:'TXT',5:'CNAME',12:'PTR',33:'SRV',65:'HTTPS',64:'SVCB',255:'ANY'}[n]||'TYPE '+(n??'—'));
  function renderLogs(){
    if(window.__rfDnsLiteSvelteLogLoaded)return; // P9: native reactive log renderer owns rows and captions
    let traces=(state.logPaused?state.frozenLogs:state.logs).slice().reverse();
    const filter=$('logFilter').value.trim().toLowerCase(),kind=$('logType').value;
    if(filter)traces=traces.filter(t=>[t.name,t.client,t.device_name,t.device_mac,t.assigned,t.used,t.assigned_endpoint,t.used_endpoint,traceEndpoint(t,'assigned'),traceEndpoint(t,'used')].some(s=>String(s||'').toLowerCase().includes(filter)));
    if(kind)traces=traces.filter(t=>String(qtypeName(t.type))===kind);
    if($('onlyErrors').checked)traces=traces.filter(t=>!t.success);
    $('logCounter').textContent=state.logs.length;
    $('logVisible').textContent=traces.length+' / '+(state.logPaused?state.frozenLogs.length:state.logs.length);
    $('logPause').textContent=state.logPaused?'▶ Продолжить':'⏸ Пауза';
    $('liveBadge').textContent=state.logPaused?'ПАУЗА · снимок журнала':'LIVE · запросы';
    const root=$('logRows');root.replaceChildren();
    if(!traces.length){const p=document.createElement('p');p.className='muted';p.textContent='Нет подходящих запросов. Попробуй изменить фильтр.';root.appendChild(p);return;}
    for(const t of traces){
      const row=document.createElement('div');row.className='log-row'+(!t.success?' error':'');
      const assigned=traceEndpoint(t,'assigned'),used=traceEndpoint(t,'used');
      const route=t.cache?'Кэш · локально':assigned+(t.used&&t.used!==t.assigned?' → '+used:'');
      const result=t.rcode===5?'REFUSED':t.rcode===4?'NOTIMP':t.rcode===3?'NXDOMAIN':t.success?(t.rcode===0?'OK':'RCODE '+t.rcode):t.rcode===-1?'TIMEOUT / FAIL':'Ошибка (RCODE '+t.rcode+')';
      const device=t.device_mac?((t.device_name||t.device_mac)+' · '+(t.client||'')):(p21Text('unknown')+' · '+(t.client||'—'));
      const vals=[formatTime(t.time),device,t.name||'—',qtypeName(t.type),route,result,String(t.latency_ms??0)+' мс'];
      for(let i=0;i<vals.length;i++){
        const n=document.createElement('span');n.className=i===2?'col-domain':i===4?'col-route':i===5?'col-result':'';
        n.textContent=String(vals[i]);if(i===1)n.title=[t.device_mac,t.identity_source,t.identity_reason,t.policy_layer,t.route_reason].filter(Boolean).join(' · ');
        if(i===4&&!t.cache&&t.used&&t.used===t.assigned)n.title='Фактический DNS: '+used;
        row.appendChild(n);
      }
      root.appendChild(row);
    }
  }
  async function refreshLogs(){
    if(window.__rfDnsLiteSvelteLogLoaded)return; // P9: native bounded GET polling owns traces
    if(state.logPaused)return;
    const t=await api('service/traces');state.logs=Array.isArray(t.traces)?t.traces:[];
    renderLogs();$('logUpdated').textContent='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});
  }
  async function load(){const d=await api('config');if(!d.config||!d.revision||!Array.isArray(d.config.rules)||!Array.isArray(d.config.upstreams))throw Error('Неверный формат конфигурации DNS');state.config=d.config;state.revision=d.revision;state.dirty=false;state.editing=null;state.editingList='__legacy__';state.editorBase=null;$('selectedDNSBadge').textContent='DNS не выбран';$('listEditor').hidden=true;render();const initial=state.config.upstreams.find(u=>u.transport!=='native');if(initial)editList(initial.id);await refreshStatus();await refreshHealth().catch(()=>{});await Promise.all([refreshNativeDNS(),refreshLogs().catch(()=>{})]);show('Настройки загружены. Можно менять DNS и списки.','good');}
  async function task(fn,refresh=true){if(state.busy)return;busy(true);try{await fn();}catch(e){show(e.message,'bad');}finally{try{if(refresh&&state.status){try{await refreshStatus();}catch(_){}}}finally{busy(false);}}}
  function on(id,fn,refresh=true){$(id).addEventListener('click',()=>task(fn,refresh));}
  $('navDns').addEventListener('click',()=>setPage('dns'));$('navLogs').addEventListener('click',()=>setPage('logs'));
  $('editorDNSChoice').addEventListener('change',()=>{const chosen=$('editorDNSChoice').value,previous=state.editing||'';if(!chosen){if(previous)closeEditor();$('editorDNSChoice').value=state.editing||'';return;}if(!editList(chosen))$('editorDNSChoice').value=previous;});
  on('reloadButton',async()=>{if((state.dirty||editorPending())&&!confirm('Отменить все несохранённые изменения, включая текст доменов?'))return;await load();});
  $('addDnsToggle').addEventListener('click',()=>openDNSForm(null));$('cancelDNS').addEventListener('click',()=>{$('dnsAddPanel').hidden=true;state.editingDNSId=null;state.checked=null;});
  for(const id of ['newAddress','newProtocol'])$(id).addEventListener('input',()=>{state.checked=null;$('commitDNS').disabled=true;$('certificateInfo').textContent='Адрес изменён. Нажми «Проверить». ';resetDraftInspection();$('certificateDetails').hidden=true;});
  $('newProtocol').addEventListener('change',()=>{$('newAddress').placeholder=$('newProtocol').value==='doh'?'https://dns.example.com/dns-query':$('newProtocol').value==='udp'?'9.9.9.9':'dns.example.com';});
  on('inspectDNS',inspect,false);on('commitDNS',addDNS,false);
  $('listChoice').addEventListener('change',()=>switchList($('listChoice').value));on('newListButton',newList,false);on('renameListButton',renameList,false);on('deleteListButton',deleteList,false);$('closeEditor').addEventListener('click',closeEditor);$('editorDomains').addEventListener('input',()=>{$('editorCount').textContent=$('editorDomains').value.split(/\r?\n/).filter(x=>x.trim()).length+' строк (черновик)';if(state.status)renderGate(state.status);});on('saveDomains',saveList,false);
  $('importDomains').addEventListener('click',()=>$('domainFile').click());$('domainFile').addEventListener('change',async()=>{const f=$('domainFile').files[0];if(!f)return;try{if(f.size>256*1024)throw Error('Максимум 256 КБ');const names=parseDomains(await f.text());$('editorDomains').value=names.join('\n');show('Импортировано '+names.length+' доменов. Нажми «Сохранить список».');}catch(e){show(e.message,'bad');}finally{$('domainFile').value='';}});
  $('exportDomains').addEventListener('click',()=>{if(!state.editing)return;const data=$('editorDomains').value;const url=URL.createObjectURL(new Blob([data.endsWith('\n')?data:data+'\n'],{type:'text/plain;charset=utf-8'}));try{const a=document.createElement('a');a.href=url;a.download='routerforge-domains.txt';a.click();}finally{URL.revokeObjectURL(url);}});
  $('defaultDNS').addEventListener('change',()=>{mark();});for(const id of ['serviceListen','cacheSize','allowedNetworks','p27RateLimit','p27RefuseAny','p27BogusIPs'])$(id).addEventListener('change',()=>{state.dirty=true;show('Есть несохранённые изменения. Действующий DNS не менялся.');$('applyButton').textContent='✓ Сохранить и применить';});
   for(const id of ['p241IPEnabled','p241IPMode','p242HealthEnabled','p242HealthProtocol','p242HealthPort','p242HealthInterval','p242HealthTimeout','p242HealthFailures','p243Scope','p243Domains','p243ListIDs','p243Recoveries','p243MinDown','p243HTTPPath'])$(id).addEventListener('change',()=>{try{if(id==='p241IPMode')$('p241IPEnabled').checked=true;if(id==='p241IPEnabled'&&!$('p241IPEnabled').checked)$('p242HealthEnabled').checked=false;syncPrefs();mark();}catch(err){show(err.message,'bad');}});
  on('checkAllDNS',async()=>{await checkHealth();});
  on('saveButton',save);on('applyButton',apply);on('migrateNative',migrate);
  on('gatePreflight',async()=>{
    const retry=Boolean(state.status?.system_gate?.desired)&&['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(state.status?.system_gate?.state);
    await api('service/gate/control',opts('service-control',{operation:retry?'retry':'preflight'},true));
    await refreshStatus();show(retry?'Контроллер повторно проверен. Реальный LAN-путь проверь отдельно.':'Готовность подтверждена. Следующий шаг — «Включить для LAN». Настройки не изменены.','good');
  });
  on('gateEnable',async()=>{
    if(state.dirty || editorPending())throw Error('Сначала сохрани и примени все изменения DNS.');
    if(!confirm('Включить RouterForge для обычных DNS-запросов ВСЕХ устройств LAN (UDP/TCP 53)? Приватные DoH/DoT приложений не перехватываются. Штатные настройки Keenetic останутся без изменений. Подключать отдельное тестовое устройство не нужно.'))return;
    await api('service/gate/control',opts('service-control',{operation:'enable'},true));
    await refreshStatus();show('Основной DNS для LAN включён. Проверь запросы устройств в журнале.','good');
  });
  on('gateDisable',async()=>{
    if(!confirm('Отключить системный DNS Gate и вернуть обычную обработку Keenetic?'))return;
    await api('service/gate/control',opts('service-control',{operation:'disable'},true));
    await refreshStatus();show('Перехват RouterForge отключён. Действуют обычные DNS-правила роутера.','good');
  });

  on('verifyButton',async()=>{const r=await serviceCommand('verify',{operation:'verify'});if(r.state!=='PASS')throw Error('Проверка DNS: '+r.state);show('DNS-сервис ответил на '+r.results.length+' прямых UDP/TCP проверок. Для LAN-перехвата отдельно используй «Проверить готовность».','good');});
  on('stopButton',async()=>{if(!confirm('Остановить DNS и вернуть прежний DNS подключённого устройства?'))return;await serviceCommand('control',{operation:'stop'});show('DNS остановлен, прежний DNS устройства возвращён.','good');});
  on('connectButton',async()=>{const mac=$('clientMAC').value.trim(),persist=$('persistClient').checked;if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(mac))throw Error('Укажи MAC в формате 02:11:22:33:44:55');if(!confirm('Назначить DNS Router устройству '+mac+'?'+(persist?' Keenetic сохранит общую конфигурацию.':'')))return;await serviceCommand('connect',{mac,persist});show('Устройство подключено. Открой журнал, затем попробуй открыть сайт на устройстве.','good');});
  on('disconnectButton',async()=>{if(!confirm('Вернуть прежний DNS выбранного устройства?'))return;await serviceCommand('disconnect',{operation:'disconnect'});show('Прежний DNS восстановлен.','good');});
  on('refreshLogs',async()=>{await refreshStatus();await refreshLogs();});$('logFilter').addEventListener('input',renderLogs);$('logType').addEventListener('change',renderLogs);$('onlyErrors').addEventListener('change',renderLogs);$('logPause').addEventListener('click',()=>{if(!state.logPaused){state.frozenLogs=state.logs.slice();state.logPaused=true;renderLogs();}else{state.logPaused=false;state.frozenLogs=[];refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));}});
  $('copyLogs').addEventListener('click',()=>{const lines=(state.logPaused?state.frozenLogs:state.logs).map(t=>[t.time,t.client,t.name,qtypeName(t.type),traceEndpoint(t,'assigned'),traceEndpoint(t,'used'),t.transport,t.rcode,t.success?'PASS':'FAIL',t.latency_ms].join('\t')).join('\n');if(navigator.clipboard?.writeText)navigator.clipboard.writeText(lines).then(()=>show('Журнал скопирован.','good')).catch(()=>show('Не удалось скопировать журнал.','bad'));else show('Копирование недоступно в этом браузере.','bad');});
  // P21: optional display mode. Policies live in the applied config, never here.
  let p21Inventory={devices:[],issues:[],applied:null,applied_revision:''};
  const p21English=(new URLSearchParams(location.search).get('lang')||document.documentElement.lang||'ru').toLowerCase().startsWith('en');
  const p21Labels={
    mode:['Режим','Mode'],basic:['Обычный','Basic'],advanced:['Расширенный','Advanced'],devices:['Устройства и правила','Devices and rules'],
    draftHelp:['Изменения здесь — черновик. Сначала сохранить, затем явно применить DNS.','Changes are drafts. Save, then explicitly apply DNS.'],refresh:['Обновить обнаружение','Refresh discovery'],save:['Сохранить черновик','Save draft'],apply:['Сохранить и применить','Save and apply'],
    identityHelp:['MAC — не аутентификация. Нужен свежий прямой L2-сосед. DHCP даёт имя и проверку противоречий. Клиенты за NAT, VPN или другим роутером не различаются; неизвестный клиент использует общие правила.','MAC is not authentication. A fresh direct LAN neighbor is required. DHCP only supplies names and conflict checks. Clients behind NAT, VPN or another router cannot be distinguished; unidentified clients use common rules.'],
    mac:['MAC устройства','Device MAC'],name:['Название','Name'],target:['DNS или DNS-группа','DNS or DNS group'],inherit:['Наследовать общие доменные правила перед DNS устройства','Inherit common domain rules before the device default'],setDevice:['Добавить / обновить устройство в черновике','Add / update device draft'],groups:['Группы устройств','Device groups'],editGroup:['Редактируемая группа','Group to edit'],members:['MAC участников — по одному в строке','Member MACs, one per line'],groupInherit:['Наследовать общие доменные правила перед DNS группы','Inherit common domain rules before the group default'],setGroup:['Сохранить группу в черновике','Save group draft'],rules:['Правила домена для устройства','Per-device domain rules'],domain:['Домен','Domain'],match:['Совпадение','Match'],priority:['Приоритет при равной точности: меньше — раньше','Equal specificity: lower priority number wins'],setRule:['Добавить / обновить правило в черновике','Add / update rule draft'],explain:['Приоритеты и проверка маршрута','Precedence and route explanation'],
    precedence:['1. Домен устройства → 2. DNS устройства → 3. Группа устройств → 4. Общие домены → 5. DNS по умолчанию. При наследовании общие домены проверяются перед DNS устройства/группы. Локальные зоны всегда обслуживает штатный локальный DNS.','1. Device domain → 2. Device DNS → 3. Device group → 4. Common domain → 5. Default DNS. Explicit inheritance checks common domains before the device/group default. Protected local zones always use local system DNS.'],
    applied:['Проверять применённые правила (иначе сохранённый черновик)','Evaluate applied rules (otherwise saved draft)'],probe:['Объяснить маршрут','Explain route'],notE2E:['Это проверка конфигурации по выбранному MAC, не доказательство определения клиента или сетевого E2E.','This is configuration evaluation for a manually selected MAC, not verified client identity or network E2E.'],
    active:['Активны правила устройств','Device rules are active'],cacheOff:['Общий кэш отключён: изоляция применённых политик устройств.','Shared cache disabled to isolate applied device policies.'],cacheNormal:['Нет применённых политик устройств. Кэш работает по сохранённой настройке сервиса.','No applied device policies. Cache follows the service setting.'],common:['Общие правила','Common rules'],newGroup:['Новая группа','New group'],edit:['Выбрать','Select'],remove:['Удалить из черновика','Remove from draft'],reset:['Вернуть к общим правилам','Reset to common rules'],unknown:['Не определён','Unidentified'],draft:['Черновик','Draft'],live:['Применено','Applied'],fresh:['Свежий L2-сосед','Fresh LAN neighbor'],offline:['Нет надёжного сопоставления','No reliable mapping'],saved:['Изменён только черновик. Для запуска нажми «Сохранить и применить».','Draft changed only. Use Save and apply to activate.'],saveFirst:['Сначала сохрани черновик для проверки его правил.','Save the draft before evaluating its rules.'],badMAC:['Укажи ненулевой unicast MAC в формате 02:11:22:33:44:55','Enter a nonzero unicast MAC, e.g. 02:11:22:33:44:55'],required:['Нужны название, корректный DNS и MAC','A name, valid DNS and MAC are required'],conflict:['MAC уже состоит в другой группе','MAC already belongs to another group']
  };
  const p21Text=key=>(p21Labels[key]||[key,key])[p21English?1:0];
  const p21Draft=write=>{if(!state.config.advanced&&write)state.config.advanced={devices:[],groups:[],rules:[]};const a=state.config.advanced||{};return {devices:a.devices||[],groups:a.groups||[],rules:a.rules||[]};};
  function p21Mutable(){const a=p21Draft(true);state.config.advanced=a;return a;}
  function p21MAC(raw){const value=raw.trim().toLowerCase();if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/.test(value)||value==='00:00:00:00:00:00'||(parseInt(value.slice(0,2),16)&1))throw Error(p21Text('badMAC'));return value;}
  function p21TargetValue(t){return t?(t.group_id?'group:'+t.group_id:'dns:'+t.upstream):'';}
  function p21Target(value){if(!value)return null;if(value.startsWith('group:')){const g=(state.config.dns_groups||[]).find(g=>g.id===value.slice(6));if(!g?.servers?.length)throw Error('Unknown DNS group');return {upstream:g.servers[0],group_id:g.id};}const id=value.slice(4);if(!state.config.upstreams.some(u=>u.id===id&&u.transport!=='native'))throw Error('Unknown direct DNS');return {upstream:id};}
  function p21TargetName(t){if(!t)return p21Text('common');if(t.group_id){const g=(state.config.dns_groups||[]).find(g=>g.id===t.group_id);return g?.name||t.group_id;}const u=state.config.upstreams.find(u=>u.id===t.upstream);return u?displayName(u):t.upstream;}
  function p21FillTarget(id){const select=$(id),previous=select.value;select.replaceChildren(option('',p21Text('common')));for(const u of state.config.upstreams.filter(u=>u.transport!=='native'))select.appendChild(option('dns:'+u.id,displayName(u)));for(const g of state.config.dns_groups||[])select.appendChild(option('group:'+g.id,'DNS '+g.name));select.value=previous;}
  function p21Policy(a,mac){const d=(a?.devices||[]).find(d=>d.mac===mac);if(d?.target)return p21TargetName(d.target);const g=(a?.groups||[]).find(g=>g.macs.includes(mac));return g?.target?p21TargetName(g.target)+' ['+g.name+']':p21Text('common');}
  function p21Button(text,fn){const b=document.createElement('button');b.type='button';b.className='button';b.textContent=text;b.addEventListener('click',()=>task(fn,false));return b;}
  function p21Row(title,description,buttons){const row=document.createElement('div');row.className='p21-device-row';const copy=document.createElement('div');copy.className='p21-device-copy';const strong=document.createElement('strong');strong.textContent=title;const small=document.createElement('small');small.textContent=description;copy.append(strong,small);row.append(copy);const actions=document.createElement('div');actions.className='actions';for(const b of buttons)actions.appendChild(b);row.append(actions);return row;}
  function p21Render(){
    if(!$('p21Page'))return;
    const a=p21Draft(false);for(const id of ['p21Target','p21GroupTarget','p21RuleTarget'])p21FillTarget(id);
    const groupSelect=$('p21GroupID'),selected=groupSelect.value;groupSelect.replaceChildren(option('',p21Text('newGroup')),...a.groups.map(g=>option(g.id,g.name)));groupSelect.value=a.groups.some(g=>g.id===selected)?selected:'';
    const active=Boolean(state.status?.device_policies_active);$('p21ActiveBadge').hidden=!active;$('p21ActiveBadge').textContent=p21Text('active');
    $('p21Cache').textContent=state.status?.cache_mode==='disabled_for_device_policy_isolation'?p21Text('cacheOff'):p21Text('cacheNormal');
    const records=new Map();
    for(const row of p21Inventory.devices||[]){if(!row.mac)continue;if(!records.has(row.mac))records.set(row.mac,{mac:row.mac,name:row.name,ips:[],trusted:false});const d=records.get(row.mac);d.ips.push(row.ip+' · '+row.state);d.trusted=d.trusted||row.trusted;}
    for(const d of a.devices){if(!records.has(d.mac))records.set(d.mac,{mac:d.mac,ips:[],trusted:false});records.get(d.mac).name=d.name||records.get(d.mac).name;}
    for(const g of a.groups)for(const mac of g.macs)if(!records.has(mac))records.set(mac,{mac,ips:[],trusted:false});
    for(const rule of a.rules)if(!records.has(rule.mac))records.set(rule.mac,{mac:rule.mac,ips:[],trusted:false});
    $('p21Devices').replaceChildren();
    for(const d of [...records.values()].slice(0,512)){
      const description=(d.ips.join('\n')||p21Text('offline'))+'\n'+(d.trusted?p21Text('fresh'):p21Text('offline'))+'\n'+p21Text('draft')+': '+p21Policy(a,d.mac)+' · '+p21Text('live')+': '+p21Policy(p21Inventory.applied,d.mac)+' · '+p21Text('rules')+': '+a.rules.filter(r=>r.mac===d.mac).length+' / '+((p21Inventory.applied?.rules||[]).filter(r=>r.mac===d.mac).length);
      $('p21Devices').appendChild(p21Row((d.name||p21Text('unknown'))+' · '+d.mac,description,[p21Button(p21Text('edit'),()=>{const configured=a.devices.find(x=>x.mac===d.mac);$('p21MAC').value=d.mac;$('p21Name').value=d.name||'';$('p21Target').value=p21TargetValue(configured?.target);$('p21Inherit').checked=Boolean(configured?.inherit_domains);$('p21RuleMAC').value=d.mac;$('p21ProbeMAC').value=d.mac;}),p21Button(p21Text('reset'),()=>{const mutable=p21Mutable();const configured=mutable.devices.find(x=>x.mac===d.mac);if(configured){delete configured.target;delete configured.inherit_domains;}mutable.rules=mutable.rules.filter(r=>r.mac!==d.mac);for(const g of mutable.groups)g.macs=g.macs.filter(m=>m!==d.mac);state.config.advanced=mutable;mark();show(p21Text('saved'));})]));
    }
    $('p21Groups').replaceChildren();for(const g of a.groups)$('p21Groups').appendChild(p21Row(g.name,g.macs.join(', ')+' → '+p21TargetName(g.target),[p21Button(p21Text('remove'),()=>{p21Mutable().groups=a.groups.filter(x=>x.id!==g.id);$('p21GroupID').value='';mark();})]));
    $('p21Rules').replaceChildren();for(const r of a.rules)$('p21Rules').appendChild(p21Row(r.mac+' · '+r.domain,r.match+' → '+p21TargetName(r.target)+' · '+r.priority,[p21Button(p21Text('edit'),()=>{$('p21RuleMAC').value=r.mac;$('p21Domain').value=r.domain;$('p21Match').value=r.match;$('p21RuleTarget').value=p21TargetValue(r.target);$('p21Priority').value=r.priority;}),p21Button(p21Text('remove'),()=>{p21Mutable().rules=a.rules.filter(x=>x.id!==r.id);mark();})]));
    $('p21DiscoveryStatus').textContent=(p21Inventory.issues||[]).join(' · ')+(p21Inventory.applied_revision?' · '+p21Text('live')+': '+p21Inventory.applied_revision:'');
  }
  async function p21Refresh(){p21Inventory=await api('service/devices');p21Render();reportHeight();}
  function p21SetMode(value){const mode=value==='advanced'?'advanced':'basic';$('dnsLiteApp').dataset.dnsMode=mode;$('p21Mode').value=mode;try{localStorage.setItem('routerforge.dnsRouter.mode.v1',mode);}catch(_){}if(mode==='basic'&&state.page==='devices')setPage('dns');if(mode==='advanced')p21Refresh().catch(e=>{$('p21DiscoveryStatus').textContent=e.message;});reportHeight();}
  function p21UsesDNS(id){const a=p21Draft(false);return [...a.devices,...a.groups,...a.rules].some(x=>x.target?.upstream===id);}
  function p21Setup(){
    for(const node of document.querySelectorAll('[data-p21-i18n]'))node.textContent=p21Text(node.dataset.p21I18n);
    $('p21Mode').addEventListener('change',()=>p21SetMode($('p21Mode').value));
    $('p21Nav').addEventListener('click',()=>{setPage('devices');p21Refresh().catch(e=>show(e.message,'bad'));});
    on('p21Refresh',p21Refresh,false);on('p21Save',save);on('p21Apply',async()=>{await apply();await p21Refresh();});
    on('p21SetDevice',()=>{const mac=p21MAC($('p21MAC').value),target=p21Target($('p21Target').value),a=p21Mutable();const d={mac,name:$('p21Name').value.trim(),...(target?{target,inherit_domains:$('p21Inherit').checked}:{})};const i=a.devices.findIndex(x=>x.mac===mac);if(i<0)a.devices.push(d);else a.devices[i]=d;mark();show(p21Text('saved'));},false);
    $('p21GroupID').addEventListener('change',()=>{const g=p21Draft(false).groups.find(g=>g.id===$('p21GroupID').value);$('p21GroupName').value=g?.name||'';$('p21Members').value=(g?.macs||[]).join('\n');$('p21GroupTarget').value=p21TargetValue(g?.target);$('p21GroupInherit').checked=Boolean(g?.inherit_domains);});
    on('p21SetGroup',()=>{const a=p21Mutable(),id=$('p21GroupID').value||newId('devices',a.groups),name=$('p21GroupName').value.trim();if(!name)throw Error(p21Text('required'));const macs=[...new Set($('p21Members').value.split(/[\s,]+/).filter(Boolean).map(p21MAC))];if(a.groups.some(g=>g.id!==id&&g.macs.some(m=>macs.includes(m))))throw Error(p21Text('conflict'));const target=p21Target($('p21GroupTarget').value);const group={id,name,macs,...(target?{target,inherit_domains:$('p21GroupInherit').checked}:{})};const i=a.groups.findIndex(g=>g.id===id);if(i<0)a.groups.push(group);else a.groups[i]=group;mark();$('p21GroupID').value=id;show(p21Text('saved'));},false);
    on('p21SetRule',()=>{const a=p21Mutable(),mac=p21MAC($('p21RuleMAC').value),domains=parseDomains($('p21Domain').value),target=p21Target($('p21RuleTarget').value),priority=Number($('p21Priority').value),match=$('p21Match').value;if(domains.length!==1||!target||!Number.isInteger(priority)||priority<0||priority>1000000)throw Error(p21Text('required'));const previous=a.rules.find(r=>r.mac===mac&&r.domain===domains[0]&&r.match===match);const rule={id:previous?.id||newId('device-domain',a.rules),mac,domain:domains[0],match,priority,target};const i=a.rules.findIndex(r=>r.id===rule.id);if(i<0)a.rules.push(rule);else a.rules[i]=rule;mark();show(p21Text('saved'));},false);
    on('p21Probe',async()=>{const applied=$('p21ProbeApplied').checked;if(!applied&&(state.dirty||editorPending()))throw Error(p21Text('saveFirst'));const body={operation:'evaluate',domain:$('p21ProbeDomain').value.trim(),mac:$('p21ProbeMAC').value.trim()?p21MAC($('p21ProbeMAC').value):'',use_applied:applied};const result=await api('service/devices/evaluate',opts('service-control',body,true));$('p21Explanation').textContent=JSON.stringify(result,null,2);},false);
    let mode='basic';try{mode=localStorage.getItem('routerforge.dnsRouter.mode.v1')||mode;}catch(_){}p21SetMode(mode);
  }

  p21Setup();
  const root=document.documentElement,params=new URLSearchParams(window.location.search);
  function syncTheme(){try{if(window.parent===window)return;const host=window.parent.document.documentElement,computed=window.parent.getComputedStyle(host);for(const token of ['--rf-bg','--rf-surface','--rf-surface-2','--rf-text','--rf-muted','--rf-border','--rf-border-strong','--rf-hover','--rf-accent','--rf-accent-soft','--rf-success','--rf-danger','--rf-radius-panel','--rf-radius-control','--rf-ui-panel-radius','--rf-ui-control-radius','--rf-ui-control-h']){const val=computed.getPropertyValue(token).trim();if(val)root.style.setProperty(token,val);}root.style.colorScheme=computed.colorScheme==='light'?'light':'dark';}catch(_){}}
  function reportHeight(){if(window.parent===window)return;try{window.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height:Math.ceil(Math.max(350,$('dnsLiteApp').scrollHeight,$('dnsLiteApp').getBoundingClientRect().height)+6)},window.location.origin);}catch(_){}}
  syncTheme();try{if(window.parent!==window&&window.MutationObserver)new MutationObserver(()=>{syncTheme();reportHeight();}).observe(window.parent.document.documentElement,{attributes:true,attributeFilter:['style','class','data-theme']});}catch(_){}
  if(window.ResizeObserver)new ResizeObserver(reportHeight).observe($('dnsLiteApp'));
  window.addEventListener('resize',reportHeight);
  setInterval(()=>{if(document.hidden||state.busy)return;refreshHealth().catch(()=>{});refreshNativeDNS().catch(()=>{});},60000);
  setInterval(()=>{if(state.page!=='logs'||document.hidden||state.busy)return;Promise.all([refreshStatus(),refreshLogs()]).then(()=>{state.lastLogError='';}).catch(e=>{if(state.lastLogError!==e.message){state.lastLogError=e.message;show('LIVE: '+e.message,'bad');}});},2000);
  // Keep log badge fresh while the overview is open; no UI-triggered DNS probes.
  setInterval(()=>{if(state.page!=='dns'||document.hidden||state.busy)return;refreshLogs().catch(()=>{});},15000);
  // P10 guarded bridge: Svelte handles UI, controller owns drafts, CAS and confirmation.
  if(window.__rfDnsLiteCardsLoaded){
    window.__rfDnsLiteCardsAPI={
      edit:id=>editList(id),
      change:id=>openDNSForm(id),
      check:id=>task(async()=>{await checkHealth(id);}),
      remove:id=>task(()=>removeDNS(id),false),
      fallback:(id,value)=>{
        const upstream=state.config.upstreams.find(x=>x.id===id);
        if(!upstream||upstream.transport==='native'){
          show('DNS не найден или не поддерживает резервирование.','bad');return;
        }
        const allowed=state.config.upstreams.filter(x=>x.id!==id&&x.transport!=='native'&&!(x.fallbacks||[]).length);
        if(value&&!allowed.some(x=>x.id===value)){
          show('Недопустимый резервный DNS.','bad');render();return;
        }
        if(value&&state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id))){
          show('Этот DNS уже является резервным. Сначала убери его из чужой резервной цепочки.','bad');render();return;
        }
        upstream.fallbacks=value?[value]:[];mark();
      }
    };
  }
  setPage(params.get('view')==='logs'?'logs':'dns');
  load().catch(e=>{show('Не удалось открыть DNS Router: '+e.message,'bad');$('serviceBadge').textContent='Недоступен';$('serviceBadge').dataset.tone='bad';});
})();
