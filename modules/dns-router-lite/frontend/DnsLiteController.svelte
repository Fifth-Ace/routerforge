<script>
  import {onMount, setContext} from 'svelte';
  import DnsLiteCards from './DnsLiteCards.svelte';
  import DnsLiteDomainEditor from './DnsLiteDomainEditor.svelte';
  import DnsLiteLog from './DnsLiteLog.svelte';
  import DnsLiteStatus from './DnsLiteStatus.svelte';
  import {dnsLiveBadge} from './DnsLiteStatusModel.js';
  import DnsLiteControls from './DnsLiteControls.svelte';
  import DnsModeStatus from './DnsModeStatus.svelte';
  import DnsP21Workspace from './DnsP21Workspace.svelte';
  import DnsP22Navigation from './DnsP22Navigation.svelte';
  import DnsP25Settings from './DnsP25Settings.svelte';
  import {cleanP21MAC, draftDevice, draftGroup, draftRule, dropDeviceOverrides, dropGroup, dropRule, aggregateP21Devices} from './DnsP21Draft.js';
  import {createDnsLiteApi} from './DnsLiteApi.js';
  // Svelte owns domain editor presentation; the controller retains only a
  // private, in-memory draft until the user explicitly saves and applies.
  const state = $state({config:{schema_version:1,upstreams:[],rules:[]},revision:'',dirty:false,busy:false,editing:null,editingList:'__legacy__',editingDNSId:null,editorBase:null,checked:null,status:null,logs:[],frozenLogs:[],logPaused:false,health:[],page:'dns',lastLogError:''});
  let cardsView = $state([]);
  let defaultSelection = $state('');
  let cardHealth = $state({});
  let editorView = $state({opened:false,generation:0});
  let modeView = $state('basic');
  let statusView = $state(null);
  let logView = $state({rows:[],upstreams:[],paused:false,updated:'',error:'',filter:'',type:'',errorsOnly:false});
  // Reactive, read-only presentation. API/CAS/Gate operations stay in the
  // controller; status fields no longer depend on imperative DOM writes.
  let presentation = $state({
    serviceBadge:'Проверяем…',serviceTone:'',healthUpdated:'Нет измерений',
    healthGate:'—',healthServers:'—',healthRequests:'—',healthP95:'—',
    nativeDNSHealth:'Проверяем модуль DNS…',gateBadge:'Выключен',gateTone:'',gatePanelState:'stock',
    listenerSummary:'—',connectionSummary:'Для LAN-режима отдельное тестовое устройство не требуется.',
    gateAdvice:'Системный DNS выключен. Сейчас запросы обрабатывает Keenetic.',
    runStatus:'Состояние DNS-сервиса загружается…',routeAdvice:'Проверяем, доходят ли запросы до DNS Router…',
    routeTone:'',clientState:'Не подключён',liveBadge:'LIVE · ожидание',liveTone:'',
    logUpdated:'Ждём запросов',metricRequests:0,metricCache:0,metricErrors:0,metricRunning:'—',
    logCounter:0,logVisible:'0 / 0'
  });
  let initialized = $state(false);
  let p21Rows = $state({devices:[],groups:[],rules:[]});
  let p21Section = $state('devices');
  let p21Model = $state({rows:{devices:[],groups:[],rules:[]},targets:[],groups:[],inventory:[],cache:'',discoveryStatus:''});
  let p21Selection = $state(null);
  let p21SelectionSeq = 0;
  let p21Route = $state(null);
  let p21ProbeMAC = $state('');
  const settingsHooks = {change:() => {}, render:() => {}};
  const discoveryHooks = {render:()=>{}, refresh:()=>Promise.resolve(), snapshot:()=>null};
  const navigationHooks = {navigate:()=>{}};
  setContext('routerforge:dns-lite-settings',settingsHooks);
  setContext('routerforge:dns-lite-discovery',discoveryHooks);
  setContext('routerforge:dns-lite-navigation',navigationHooks);
  let commands = {};
  function portal(node, id) {
    const target = document.getElementById(id);
    if (!target) throw Error('DNS_LITE_PORTAL_TARGET_MISSING: '+id);
    target.replaceChildren(node);
    return {destroy(){if(node.parentNode===target)node.remove();}};
  }
  function cardAction(name,id,value) {commands.card?.(name,id,value);}
  function editorAction(name,value) {return commands.editor?.(name,value);}
  function modeAction(mode) {commands.mode?.(mode);}
  function statusAction() {commands.refresh?.();}
  function p21RowAction(kind, action, key) {commands.p21Row?.(kind,action,key);}
  function p21FormAction(kind, payload) {commands.p21Form?.(kind,payload);}
  function p21Navigate(which) {commands.p21Navigate?.(which);}
  function controlAction(which, payload) {commands.control?.(which, payload);}
  onMount(() => {
    let cleanup = () => {};
    /* RF_ACTUAL_CONTROLLER_START */
    (() => {
      const byId = id => document.getElementById(id);
      const apiClient = createDnsLiteApi({fetchImpl:(...args)=>fetch(...args)});
      let draftVersion=0, saving=false, disposed=false;
      const intervalHandles=[];
      const eventCleanups=[];
      // No duplicate HTML editor. The draft is independent of the DOM and
      // survives Svelte updates, navigation and rejected list switches.
      const editorDraft={
        domains:'',newName:'',renameName:'',title:'Список доменов',
        choices:[],selected:'',editable:false,named:false,
        hint:'Изменения попадут в черновик и начнут действовать после применения.'
      };

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
      const editorPending=()=>state.editing!==null && editorDraft.domains!==state.editorBase;
      const show = (message,tone='') => {if(disposed)return;const el=byId('feedback');if(!el)return;el.textContent=message;el.dataset.tone=tone;};
      const busy = on => {if(disposed)return;state.busy=on;for(const id of ['saveButton','applyButton','checkAllDNS','addDnsToggle','inspectDNS','commitDNS','migrateNative','connectButton','disconnectButton','verifyButton','stopButton','gatePreflight','gateEnable','gateDisable','p21Refresh','p21Save','p21Apply','p21SetDevice','p21SetGroup','p21SetRule','p21Probe']){const el=byId(id);if(el)el.disabled=on;}if(!on){const key=byId('newProtocol').value+'|'+byId('newAddress').value.trim();const c=state.checked;byId('commitDNS').disabled=!c||c.key!==key||c.editId!==state.editingDNSId;if(state.status)renderGate(state.status);}};
      async function api(path, options={}) {
        return apiClient.request(path,options);
      }
      // Settings inputs have one event owner (DnsP25Settings). The controller
      // receives an explicit command, never a second DOM change listener.
      const p25IPFields=new Set(['p241IPEnabled','p241IPMode','p242HealthEnabled','p242HealthProtocol','p242HealthPort','p242HealthInterval','p242HealthTimeout','p242HealthFailures','p243Scope','p243Domains','p243ListIDs','p243Recoveries','p243MinDown','p243HTTPPath']);
      settingsHooks.change=(id='')=>{
        if(disposed)return;
        if(p25IPFields.has(id)){
          try{
            if(id==='p241IPMode')byId('p241IPEnabled').checked=true;
            if(id==='p241IPEnabled'&&!byId('p241IPEnabled').checked)byId('p242HealthEnabled').checked=false;
            // Editing settings is a local draft action, even when no default
            // DNS has been chosen yet. Full validation remains mandatory on
            // Save/Apply; opening or editing never sends a network mutation.
            syncPrefs(false);
            draftVersion++;
            state.dirty=true;
            show('Есть несохранённые изменения. Действующий DNS не менялся.');
            byId('applyButton').textContent='✓ Сохранить и применить';
          }catch(err){show(err.message,'bad');}
        }else{
          draftVersion++;state.dirty=true;
          show('Есть несохранённые изменения. Действующий DNS не менялся.');
          byId('applyButton').textContent='✓ Сохранить и применить';
        }
      };
      const opts=(action,body,match)=>({method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...(match?{'If-Match':'"'+state.revision+'"'}:{})},body:JSON.stringify(body)});
      function mark(){draftVersion++;state.dirty=true;p21Route=null;show('Есть несохранённые изменения. Действующий DNS не менялся.');render();}
      function setPage(page){state.page=page;byId('pageDns').hidden=page!=='dns';byId('pageLogs').hidden=page!=='logs';byId('p21Page').hidden=page!=='devices';for(const [id,target] of [['navDns','dns'],['navLogs','logs']]){const el=byId(id);if(!el)continue;el.classList.toggle('selected',page===target);if(page===target)el.setAttribute('aria-current','page');else el.removeAttribute('aria-current');}if(page==='logs')refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));reportHeight();}
      const option = (value,label) => {const el=document.createElement('option');el.value=value;el.textContent=label;return el;};
      function render(){
        p21Render();
        const list=state.config.upstreams||[];
        const saved=byId('defaultDNS').value||state.config.service?.default_upstream||'';
        defaultSelection=list.some(u=>u.id===saved)?saved:'';
        byId('serverCount').textContent=list.length+' DNS';
        byId('nativeWarning').hidden=!list.some(u=>u.transport==='native'&&used(u.id).length);
        byId('editorEmpty').hidden=Boolean(state.editing);
        cardsView = list.map(u => {
          const rules=used(u.id);
          const namedLists=(state.config.lists||[]).filter(l=>l.upstream===u.id).length;
          const listCount=namedLists+(rules.some(r=>!r.list_id)||!namedLists?1:0);
          const fallbackOptions=list.filter(t=>t.id!==u.id&&t.transport!=='native'&&!(t.fallbacks||[]).length)
            .map(t=>({id:t.id,label:displayName(t)}));
          return {id:u.id,title:hostName(u),transport:u.transport,native:u.transport==='native',
            address:u.address,path:u.path||'',listCount,domainCount:rules.length,
            domains:rules.map(r=>r.domain),fallback:(u.fallbacks||[])[0]||'',fallbackOptions};
        });
        const chooser=byId('editorDNSChoice');
        const managed=list.filter(u=>u.transport!=='native');
        chooser.replaceChildren(option('','Выбери DNS…'),...managed.map(u=>option(u.id,hostName(u))));
        chooser.value=managed.some(u=>u.id===state.editing)?state.editing:'';
        chooser.disabled=!managed.length;
        if(state.editing)renderListToolbar();
        if(state.config.service){byId('serviceListen').value=state.config.service.listen;byId('cacheSize').value=state.config.service.cache_bytes/(1024*1024);byId('allowedNetworks').value=state.config.service.allowed_networks.join(',');}
        const p27=state.config.service?.protection;
        byId('p27RateLimit').value=p27?.rate_limit_rps||0;
        byId('p27RefuseAny').checked=Boolean(p27?.refuse_any);
        byId('p27BogusIPs').value=(p27?.bogus_nxdomain||[]).join('\n');
        if(byId('p28Enabled')){
        const p28=state.config.service?.upstream_selection||{};
        byId('p28Enabled').checked=Boolean(p28.enabled);byId('p28Mode').value=p28.mode||'sequential';byId('p28Timeout').value=p28.timeout_ms||3000;
        const p29=state.config.service?.performance||{};
        byId('p29Enabled').checked=Boolean(p29.enabled);byId('p29MinTTL').value=p29.cache_min_ttl||0;byId('p29MaxTTL').value=p29.cache_max_ttl||0;
        byId('p29Optimistic').checked=Boolean(p29.optimistic_cache);byId('p29Pending').value=p29.pending_requests||'default';byId('p29Concurrent').value=p29.max_concurrent||32;
        const p30=state.config.service?.privacy||{};
        byId('p30Enabled').checked=Boolean(p30.enabled);byId('p30ECSMode').value=p30.ecs_mode||'inherit';byId('p30ECSAddress').value=p30.ecs_address||'';
        byId('p30DNSSECMode').value=p30.dnssec_mode||'off';
        const p31d=state.config.service?.dns64||{};
        byId('p31DNS64Enabled').checked=Boolean(p31d.enabled);byId('p31DNS64Prefix').value=p31d.prefix||'64:ff9b::/96';byId('p31NAT64Confirmed').checked=Boolean(p31d.nat64_confirmed);
        const p31r=state.config.service?.reverse_dns||{};
        byId('p31ReverseEnabled').checked=Boolean(p31r.enabled);
        const reverse=byId('p31ReverseUpstream'),selected=p31r.upstream||'';
        reverse.replaceChildren(option('','Выбери прямой DNS…'),...state.config.upstreams.filter(u=>u.transport!=='native').map(u=>option(u.id,hostName(u))));reverse.value=selected;
        }
        byId('p241IPEnabled').checked=Boolean(state.config.service?.ip_intelligence?.enabled);
        byId('p241IPMode').value=state.config.service?.ip_intelligence?.mode||'original';
        const p242health=state.config.service?.ip_intelligence?.health;
        byId('p242HealthEnabled').checked=Boolean(p242health?.enabled);
        byId('p242HealthProtocol').value=p242health?.protocol||'tcp';
        byId('p242HealthPort').value=p242health?.port||443;
        byId('p242HealthInterval').value=p242health?.interval_seconds||60;
        byId('p242HealthTimeout').value=p242health?.timeout_ms||1200;
        byId('p242HealthFailures').value=p242health?.failure_threshold||2;
        byId('p243Recoveries').value=p242health?.recovery_threshold||2;
        byId('p243MinDown').value=p242health?(p242health.min_down_seconds??0):30;
        byId('p243HTTPPath').value=p242health?.http_path||'/';
        byId('p243Scope').value=state.config.service?.ip_intelligence?.scope||'all';
        byId('p243Domains').value=(state.config.service?.ip_intelligence?.domains||[]).join('\n');
        const chosenLists=state.config.service?.ip_intelligence?.list_ids||[];
        byId('p243ListIDs').replaceChildren(...(state.config.lists||[]).map(x=>{const o=new Option((x.name||x.id)+' · '+x.id,x.id);o.selected=chosenLists.includes(x.id);return o;}));
        settingsHooks.render();
        presentation.listenerSummary=state.config.service?.listen||byId('serviceListen').value;
        byId('applyButton').textContent=state.dirty?'✓ Сохранить и запустить DNS':'▶ Запустить DNS';
        if(state.status)renderGate(state.status);
        renderHealth();
        reportHeight();
      }
      function healthByID(id){return state.health.find(x=>x.id===id)||null;}
      function healthStateText(h){if(!h)return 'Не проверялся';return {UP:'Доступен',DOWN:'Недоступен',STALE:'Данные устарели',UNKNOWN:'Не проверялся',UNSUPPORTED:'Keenetic'}[h.status]||'Не проверялся';}
      function renderHealth(){
        const next={};
        for(const h of state.health) if(h && typeof h.id==='string') next[h.id]={...h};
        cardHealth=next;
        let up=0,checks=0,requests=0,latencies=[];
        for(const u of state.config.upstreams){
          const h=healthByID(u.id);
          if(h?.status==='UP')up++;
          if(h&&h.status!=='UNKNOWN'&&h.status!=='UNSUPPORTED')checks++;
          requests+=Number(h?.requests_5m||0);
          if(h?.requests_5m>h?.failures_5m)latencies.push(Number(h?.p95_latency_5m_ms||0));
        }
        presentation.healthServers=checks?up+'/'+state.config.upstreams.length:'— / '+state.config.upstreams.length;
        presentation.healthRequests=requests;
        presentation.healthP95=latencies.length?Math.max(...latencies)+' мс':'—';
        presentation.healthUpdated='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});
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
        try{
          const response=await fetch('/api/modules/dns/quality?minutes=5',{credentials:'same-origin',cache:'no-store'});
          if(!response.ok)throw Error('HTTP '+response.status);
          const data=await response.json();if(disposed)return;if(!Array.isArray(data.upstreams))throw Error('неверный формат');
          presentation.nativeDNSHealth='Модуль DNS доступен · '+data.upstreams.length+' резолверов Keenetic · отдельная статистика';
        }catch(_){if(!disposed)presentation.nativeDNSHealth='Модуль DNS недоступен — RouterForge DNS Router работает независимо';}
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
      let editorGeneration=0;
      function syncEditorSnapshot(){
        editorView={
          ...editorDraft,
          opened:Boolean(state.editing),
          generation:++editorGeneration,
          choices:editorDraft.choices.map(c=>({...c}))
        };
      }
      function renderListToolbar(){
        if(!state.editing)return;
        const entries=listEntries(state.editing);
        if(!entries.some(e=>e.id===state.editingList))state.editingList=entries[0].id;
        editorDraft.choices=entries.map(e=>({id:e.id,label:e.name+' · '+listRules(state.editing,e.id).length+' доменов'}));
        editorDraft.selected=state.editingList;
        const meta=(state.config.lists||[]).find(l=>l.id===state.editingList);
        editorDraft.renameName=meta?meta.name:'';
        editorDraft.named=Boolean(meta);
        editorDraft.editable=listIsEditable(state.editing,state.editingList);
        editorDraft.title=(meta?.name||'Основной список')+' · '+displayName(state.config.upstreams.find(u=>u.id===state.editing));
        if(!editorDraft.editable)editorDraft.hint='Этот URL/сложный список управляется отдельным механизмом. Ручное редактирование заблокировано.';
        syncEditorSnapshot();
      }
      function switchList(key){
        if(editorPending()&&!confirm('Несохранённый текст текущего списка будет потерян. Переключить?')){
          // Restore the visible select without altering the draft or the
          // previously selected list. This is a refused user operation.
          syncEditorSnapshot();
          return false;
        }
        if(!listEntries(state.editing).some(l=>l.id===key))throw Error('Неизвестный список');
        state.editingList=key;
        const rules=listRules(state.editing,key);
        editorDraft.domains=rules.map(r=>r.domain).join('\n');
        state.editorBase=editorDraft.domains;
        editorDraft.hint='Редактируется только выбранный список. Остальные списки и DNS не меняются.';
        renderListToolbar();reportHeight();
        return true;
      }
      function editList(id){
        if(editorPending()){
          if(!confirm('У выбранного DNS есть несохранённый текст списка. Перейти к другому DNS?'))return false;
          state.editorBase=editorDraft.domains;
        }
        const u=state.config.upstreams.find(x=>x.id===id);
        if(!u||u.transport==='native')throw Error('Legacy DNS нужно сначала преобразовать');
        // The first editor opening has no previous draft to discard.
        // Seed the comparison baseline before switching from null to DNS id.
        if(state.editing===null)state.editorBase=editorDraft.domains;
        const changed=state.editing!==id;
        state.editing=id;
        if(changed)state.editingList=listEntries(id)[0].id;
        byId('selectedDNSBadge').textContent=displayName(u);
        byId('editorEmpty').hidden=true;
        switchList(state.editingList);render();reportHeight();return true;
      }
      function closeEditor(){
        if(editorPending()&&!confirm('Закрыть редактор? Несохранённый текст будет потерян.'))return false;
        state.editing=null;state.editingList='__legacy__';state.editorBase=null;
        editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';
        byId('editorEmpty').hidden=false;
        byId('selectedDNSBadge').textContent='DNS не выбран';render();
        syncEditorSnapshot();return true;
      }
      function saveList(){
        if(!state.editing)return;
        const id=state.editing,key=state.editingList,old=listRules(id,key);
        if(!listIsEditable(id,key))throw Error('URL или сложный список нельзя менять через простой редактор');
        const names=parseDomains(editorDraft.domains);
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
        editorDraft.domains=names.join('\n');state.editorBase=editorDraft.domains;
        editorDraft.hint='Список сохранён в черновик. Применение DNS выполняется отдельно.';
        mark();show('Список обновлён: '+names.length+' доменов. Остальные списки и DNS не менялись.');
      }
      function newList(){
        if(!state.editing||editorPending())throw Error('Сначала сохрани текст открытого списка');
        const name=editorDraft.newName.trim(),all=state.config.lists||[];
        if(!name||Array.from(name).length>80)throw Error('Название списка: от 1 до 80 символов');
        if(all.length>=64)throw Error('Лимит: 64 списка');
        if(all.some(l=>l.upstream===state.editing&&l.name.toLowerCase()===name.toLowerCase()))throw Error('Список с таким названием уже существует');
        const id=newId('list',all);
        state.config.lists=all.concat({id,name,kind:'paste',upstream:state.editing,match:'suffix',priority:100});
        state.editingList=id;editorDraft.newName='';switchList(id);mark();
        show('Новый список создан в черновике. Добавь домены и сохрани список.','good');
      }
      function renameList(){
        if(!state.editing||editorPending())throw Error('Сначала сохрани текст открытого списка');
        const list=(state.config.lists||[]).find(l=>l.id===state.editingList);
        if(!list)throw Error('Основной список старого формата нельзя переименовать');
        const name=editorDraft.renameName.trim();
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
        if(state.config.service?.default_upstream===id){state.config.service=null;byId('defaultDNS').value='';}
        if(state.editing===id)closeEditor();mark();
      }
      function openDNSForm(id){
        if(editorPending())throw Error('Сначала сохрани несохранённый текст списка');
        const u=id?state.config.upstreams.find(x=>x.id===id):null;
        if(id&&(!u||u.transport==='native'))throw Error('Этот DNS нельзя изменить через простой редактор');
        state.editingDNSId=id||null;state.checked=null;
        byId('dnsAddPanelTitle').textContent=u?'Изменить DNS / обновить TLS':'Добавить DNS';
        byId('commitDNS').textContent=u?'Сохранить DNS (черновик)':'Добавить DNS';
        byId('newProtocol').value=u?u.transport:'doh';
        byId('newAddress').value=!u?'':u.transport==='doh'?'https://'+u.server_name+(u.path||'/dns-query'):
          isEncrypted(u.transport)?u.server_name:u.address.replace(/:53$/,'');
        resetDraftInspection();
        byId('certificateInfo').textContent=u?'Черновик: проверка не выполнялась. Активный DNS не изменён.':'Проверка не выполнялась.';
        byId('certificateDetails').hidden=true;byId('commitDNS').disabled=true;
        byId('dnsAddPanel').hidden=false;reportHeight();
      }
      function newFormData(){
        let transport=byId('newProtocol').value;let raw=byId('newAddress').value.trim();
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
      function draftStatus(id,text,state='') {const el=byId(id);el.textContent=text;el.dataset.state=state;}
      function resetDraftInspection(){
        for(const id of ['draftAddressState','draftIPState','draftTLSState','draftCertificateState','draftDNSState','draftRTTState'])draftStatus(id,'Не проверено');
        byId('draftProbeError').textContent='';byId('draftProbeError').hidden=true;
      }
      async function inspect(){
        state.checked=null;byId('commitDNS').disabled=true;resetDraftInspection();
        let f;
        try {f=newFormData();}
        catch(e){draftStatus('draftAddressState','Ошибка','bad');byId('draftProbeError').hidden=false;byId('draftProbeError').textContent=e.message;return;}
        const key=byId('newProtocol').value+'|'+byId('newAddress').value.trim(),editId=state.editingDNSId;
        draftStatus('draftAddressState',f.host,'ok');draftStatus('draftIPState','Получение…','pending');
        draftStatus('draftTLSState',isEncrypted(f.transport)?'Ожидание…':'Не используется',isEncrypted(f.transport)?'pending':'neutral');
        draftStatus('draftCertificateState',(f.transport==='udp'||f.transport==='tcp')?'Не требуется':'Ожидание…',(f.transport==='udp'||f.transport==='tcp')?'neutral':'pending');
        let r;
        try {r=f.transport==='tcp'?{selected_ip:f.host,address:f.host+':53'}:await api('certificates/lookup',opts('certificate-lookup',{transport:f.transport,server_name:f.host}));}
        catch(e){
          if(key!==byId('newProtocol').value+'|'+byId('newAddress').value.trim()||editId!==state.editingDNSId)return;
          draftStatus('draftIPState','Не получен','bad');
          if(isEncrypted(f.transport))draftStatus('draftTLSState','Не подтверждён','bad');
          draftStatus('draftCertificateState',isEncrypted(f.transport)?'Не получен':'Не требуется',isEncrypted(f.transport)?'bad':'neutral');
          draftStatus('draftDNSState','Недоступен','bad');
          byId('draftProbeError').hidden=false;byId('draftProbeError').textContent=e.message;
          byId('certificateInfo').textContent='Адрес / TLS: '+e.message;reportHeight();return;
        }
        if(key!==byId('newProtocol').value+'|'+byId('newAddress').value.trim()||editId!==state.editingDNSId)return;
        if(!r.address||!r.selected_ip){draftStatus('draftIPState','Не получен','bad');byId('draftProbeError').hidden=false;byId('draftProbeError').textContent='Не найден IP DNS-сервера';return;}
        draftStatus('draftIPState',r.address,'ok');
        const preverified=!!r.certificate?.verified;
        if((f.transport==='doh'||f.transport==='dot')&&!preverified){draftStatus('draftTLSState','Ошибка','bad');draftStatus('draftCertificateState','Не получен','bad');byId('draftProbeError').hidden=false;byId('draftProbeError').textContent='TLS-сертификат не удалось проверить';return;}
        draftStatus('draftTLSState',preverified?'Проверен':f.transport==='doq'?'QUIC: проверяется запросом':'Не используется',preverified?'ok':f.transport==='doq'?'pending':'neutral');
        draftStatus('draftCertificateState',preverified?'Получен · до '+(r.certificate.not_after||'—'):f.transport==='doq'?'Проверяется при QUIC':'Не требуется',preverified?'ok':'neutral');
        const candidate={id:'p194-draft',transport:f.transport,address:r.address};
        if(isEncrypted(f.transport))candidate.server_name=f.host;
        if(f.transport==='doh')candidate.path=f.path;
        draftStatus('draftDNSState','Проверка…','pending');
        let probe;
        try {probe=await api('service/upstreams/inspect',opts('service-control',{operation:'inspect',upstream:candidate}));}
        catch(e){probe={dns_available:false,status:'DOWN',last_error:e.message,tls_status:'UNCONFIRMED'};}
        if(key!==byId('newProtocol').value+'|'+byId('newAddress').value.trim()||editId!==state.editingDNSId)return;
        const available=probe.dns_available===true;
        draftStatus('draftDNSState',available?'Доступен':'Недоступен',available?'ok':'bad');
        draftStatus('draftRTTState',available?String(probe.probe_latency_ms)+' мс':'—',available?'ok':'neutral');
        if(f.transport==='doq'){
          draftStatus('draftTLSState',available?'Проверен при QUIC':'Не подтверждён',available?'ok':'bad');
          draftStatus('draftCertificateState',available?'Проверен транспортом':'Не получен',available?'ok':'bad');
        }
        byId('draftProbeError').hidden=available;
        byId('draftProbeError').textContent=available?'':'DNS недоступен: '+(probe.last_error||'нет корректного ответа');
        state.checked={key,form:f,lookup:r,editId,available};
        byId('certificateInfo').textContent=(available?'✓ DNS доступен':'⚠ DNS недоступен')+' · IP '+r.selected_ip+' · '+(preverified?'TLS проверен':f.transport==='doq'?'DoQ TLS при запросе':'Без TLS');
        byId('certificateDetails').hidden=!r.certificate;
        byId('certificateDetailsText').textContent=r.certificate?['Имя: '+r.certificate.server_name,'Кому: '+r.certificate.subject,'Выдан: '+r.certificate.issuer,'Действителен до: '+r.certificate.not_after,'SHA256: '+r.certificate.fingerprint_sha256].join('\n'):'';
        byId('commitDNS').disabled=false;reportHeight();
      }
      function addDNS(){
        if(editorPending())throw Error('Сначала сохрани изменённый список доменов справа.');
        const c=state.checked,key=byId('newProtocol').value+'|'+byId('newAddress').value.trim();
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
        state.editingDNSId=null;state.checked=null;byId('dnsAddPanel').hidden=true;
        resetDraftInspection();byId('certificateInfo').textContent='Проверка ещё не выполнялась.';byId('newAddress').value='';
        mark();
        if(!prev)editList(u.id);
        else if(state.editing===u.id){byId('selectedDNSBadge').textContent=displayName(u);renderListToolbar();}
        show(prev?'DNS/TLS обновлены в черновике. ID, списки, резерв и настройки по умолчанию сохранены.':'DNS добавлен. Его список открыт справа. Выбери DNS по умолчанию перед применением.','good');
      }
      function servicePrefs(requireDefault=true){
        const primary=byId('defaultDNS').value;
        // Draft editing may precede choosing the default DNS; only an explicit
        // Save/Apply requires a valid upstream. Never relax the persistence gate.
        if(requireDefault && (!primary || !state.config.upstreams.some(u=>u.id===primary)))throw Error('Выбери DNS по умолчанию');
        if(primary && !state.config.upstreams.some(u=>u.id===primary))throw Error('Выбранный DNS недоступен');
        const cache=Number(byId('cacheSize').value);if(!Number.isInteger(cache)||cache<0||cache>8)throw Error('Кэш: от 0 до 8 МиБ');
        const listen=byId('serviceListen').value.trim(),networks=byId('allowedNetworks').value.split(',').map(s=>s.trim()).filter(Boolean);
        const prefs={listen,default_upstream:primary,cache_bytes:cache*1024*1024,allowed_networks:networks};
        const p27r=Number(byId('p27RateLimit').value);
        if(!Number.isInteger(p27r)||p27r<0||p27r>2000)throw Error('Защита DNS: лимит 0..2000 запросов/с');
        const p27ips=byId('p27BogusIPs').value.split(/[\r\n,]+/).map(x=>x.trim()).filter(Boolean);
        if(p27ips.length>16)throw Error('Защита DNS: максимум 16 IP/CIDR');
        if(p27r>0||byId('p27RefuseAny').checked||p27ips.length) {
          prefs.protection={rate_limit_rps:p27r,refuse_any:byId('p27RefuseAny').checked,bogus_nxdomain:p27ips};
        }
        if(byId('p242HealthEnabled').checked && !byId('p241IPEnabled').checked)throw Error('Сначала включи IP Intelligence');
        const asInt=(id,min,max)=>{const n=Number(byId(id).value);if(!Number.isInteger(n)||n<min||n>max)throw Error(id+': допустимо '+min+'..'+max);return n;};
        if(byId('p28Enabled')?.checked) prefs.upstream_selection={enabled:true,mode:byId('p28Mode').value,timeout_ms:asInt('p28Timeout',250,5000)};
        if(byId('p29Enabled')?.checked) prefs.performance={enabled:true,cache_min_ttl:asInt('p29MinTTL',0,3600),cache_max_ttl:asInt('p29MaxTTL',0,86400),optimistic_cache:byId('p29Optimistic').checked,pending_requests:byId('p29Pending').value,max_concurrent:asInt('p29Concurrent',8,64)};
        if(byId('p30Enabled')?.checked) prefs.privacy={enabled:true,ecs_mode:byId('p30ECSMode').value,ecs_address:byId('p30ECSMode').value==='fixed'?byId('p30ECSAddress').value.trim():'',dnssec_mode:byId('p30DNSSECMode').value};
        if(byId('p31DNS64Enabled')?.checked) prefs.dns64={enabled:true,prefix:byId('p31DNS64Prefix').value.trim(),nat64_confirmed:byId('p31NAT64Confirmed').checked};
        if(byId('p31ReverseEnabled')?.checked){if(!byId('p31ReverseUpstream').value)throw Error('Выбери DNS для приватного rDNS');prefs.reverse_dns={enabled:true,upstream:byId('p31ReverseUpstream').value};}
        if(byId('p241IPEnabled').checked){
          prefs.ip_intelligence={enabled:true,mode:byId('p241IPMode').value};
          const selectedScope=byId('p243Scope').value;
          if(selectedScope==='domains'){
            const domains=parseDomains(byId('p243Domains').value);
            if(!domains.length||domains.length>128)throw Error('IP Intelligence: укажи 1..128 доменов');
            prefs.ip_intelligence.scope='domains';prefs.ip_intelligence.domains=domains;
          } else if(selectedScope==='lists'){
            const ids=Array.from(byId('p243ListIDs').selectedOptions,x=>x.value);
            if(!ids.length)throw Error('IP Intelligence: выбери хотя бы один список');
            prefs.ip_intelligence.scope='lists';prefs.ip_intelligence.list_ids=ids;
          }
          if(byId('p242HealthEnabled').checked)prefs.ip_intelligence.health={
            enabled:true,protocol:byId('p242HealthProtocol').value,
            port:asInt('p242HealthPort',80,65535),
            interval_seconds:asInt('p242HealthInterval',30,3600),
            timeout_ms:asInt('p242HealthTimeout',250,3000),
             failure_threshold:asInt('p242HealthFailures',1,5),
             recovery_threshold:asInt('p243Recoveries',1,5),
             min_down_seconds:asInt('p243MinDown',0,3600),
             http_path:byId('p243HTTPPath').value.trim()||'/'
          };
        }
        return prefs;
      }
      function syncPrefs(requireDefault=true){const next=servicePrefs(requireDefault);const prev=state.config.service;const merged={...prev,...next};if(!next.ip_intelligence)delete merged.ip_intelligence;if(!next.protection)delete merged.protection;for(const k of ["upstream_selection","performance","privacy","dns64","reverse_dns"])if(!next[k] && byId("p28Enabled"))delete merged[k];if(JSON.stringify(prev)!==JSON.stringify(merged)){state.config.service=merged;if(prev?.default_upstream!==next.default_upstream)delete state.config.service.default_group;draftVersion++;state.dirty=true;}}
      async function save(){
        if(saving)throw Error('Сохранение DNS уже выполняется');
        if(editorPending())throw Error('Сначала нажми «Сохранить список» в редакторе доменов.');
        syncPrefs();
        if(!state.dirty)return;
        const version=draftVersion, revision=state.revision;
        const payload=JSON.stringify(state.config);
        saving=true;
        try {
          const data=await api('config',{method:'PUT',headers:{'Content-Type':'application/json',
            'X-RouterForge-Action':'save','If-Match':'"'+revision+'"'},body:payload});
          if(!data || typeof data.revision!=='string' || !data.revision)
            throw Error('Сервер не вернул ревизию после сохранения DNS');
          state.revision=data.revision;
          state.dirty=draftVersion!==version || JSON.stringify(state.config)!==payload;
          show(state.dirty?'Черновик изменился во время сохранения. Повтори сохранение последней версии.':'Настройки сохранены. Действующий DNS не изменился. Для запуска нажми «Сохранить и запустить DNS».',state.dirty?'':'good');
          render();
        } finally { saving=false; }
      }
      async function serviceCommand(path,body){return api('service/'+path,opts('service-control',body,true));}
      function hasUsedNative(){return state.config.upstreams.some(u=>u.transport==='native'&&(used(u.id).length||state.config.service?.default_upstream===u.id));}
      async function apply(){
        if(hasUsedNative())throw Error('Есть старые native DNS. Сначала нажми «Преобразовать DNS» и проверь адреса.');
        const prefs=servicePrefs();
        if(!confirm('Сохранить настройки и запустить DNS Router на '+prefs.listen+'? Это ещё не переключит всю LAN. Для этого отдельно включи «Основной DNS для LAN».'))return;
        await save();if(state.dirty)throw Error('Черновик изменился во время сохранения. Сначала сохрани новые изменения.');const s=await api('service/status');
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
        const endpoint=s.listen||state.config.service?.listen||byId('serviceListen').value;
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
      function showRoutingHint(s){const hint=routingHint(s);presentation.routeAdvice=hint.text;presentation.routeTone=hint.tone;}
      function renderGate(s){
        const gate=s.system_gate||{state:'STOCK',active:false};
        const active=Boolean(gate.active),recovery=['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(gate.state),wanted=Boolean(gate.desired||s.desired?.interception_enabled);
        presentation.healthGate=active?'RouterForge':recovery?'Проверка':'Keenetic';
        presentation.gatePanelState=recovery?'recovery':active?'active':'stock';
        presentation.gateBadge=gate.state==='CONFLICT'?'Конфликт':gate.state==='DEGRADED'?'Восстановление':recovery?'Требуется проверка':active?'Активен':'Выключен';
        presentation.gateTone=active?'good':recovery?'bad':'';
        byId('gateEnable').disabled=state.busy||!s.active||active||recovery||state.dirty||editorPending();
        byId('gateDisable').disabled=state.busy||(!active&&!recovery&&!wanted);
        byId('gatePreflight').disabled=state.busy||!s.active;
        byId('gatePreflight').textContent=wanted&&recovery?'Повторить восстановление':'Проверить готовность';
        byId('stopButton').disabled=state.busy||!s.active;
        const hits=Number(s.requests||0),denied=Number(s.denied_requests||0);
        const coverage=gate.coverage||{},ipv6=coverage.ipv6_routed==='active_last_verified';
        const coverageText=active?('Перехват: '+(ipv6?'IPv4 и IPv6 global/ULA':'только IPv4')+'. Клиенты с IPv6 link-local source исключены. '):'';
        presentation.gateAdvice=recovery?('Перехват не подтверждён. Желаемое состояние: '+(wanted?'ВКЛ':'ВЫКЛ')+'. '+(gate.last_error||'Проверяем listener, порядок правил и lease')+'. Действие восстановления ограничено; сторонние правила не перезаписываются.'):
          active?(coverageText+'Gate проверен: listener, порядок правил и действующие lease. Запросов сервиса: '+hits+', ACL-отказов: '+denied+'. Реальный путь устройства подтверждается отдельной проверкой, не этим счётчиком.'):
          'Перехват выключен. Последний выбор ON/OFF сохраняется отдельно от черновика конфигурации; при обновлении и перезагрузке он восстанавливается после проверки готовности. Тестовое устройство не требуется.';
      }
      async function refreshStatus(){const s=await api('service/status');if(s.mode!=='independent-dns')throw Error('Загружен не DNS Lite. Установи актуальный DEV-пакет.');state.status=s;statusView=s;renderGate(s);
        const connection=s.connection||{state:'disconnected'},active=Boolean(s.active),same=active&&s.applied_revision===state.revision;
        presentation.serviceBadge=active?'DNS работает':'DNS остановлен';presentation.serviceTone=active?'good':'bad';
        presentation.runStatus=(active?'DNS работает: '+s.listen+' · Основной '+displayName(state.config.upstreams.find(u=>u.id===s.default_upstream)||{transport:'udp',address:s.default_upstream||'—'}):'Сервис остановлен.')+(active&&!same?' · Есть неприменённая версия настроек.':'')+(s.last_error?' · Ошибка: '+s.last_error:'');
        presentation.connectionSummary=s.system_gate?.active?'Перехват LAN включён — проверь запросы реального устройства':connection.state==='connected'?'Точечное устройство: '+connection.mac:connection.state==='recovery_required'?'Точечное подключение требует восстановления':'Для LAN-режима отдельное устройство не требуется';
        presentation.clientState=connection.state==='connected'?connection.mac:connection.state==='disconnected'?'Не подключён':connection.state;
        byId('connectButton').disabled=!active||!same||connection.state!=='disconnected';byId('disconnectButton').disabled=connection.state==='disconnected';byId('stopButton').disabled=!active;
        showRoutingHint(s);
        presentation.metricRequests=s.requests??0;presentation.metricCache=s.cache_hits??0;presentation.metricErrors=s.failures??0;presentation.metricRunning=active?'Работает':'Выкл.';
        syncLiveBadge();
        p21Render();
        return s;
      }
      function syncLiveBadge(){
        // An API poll is not proof of traffic or a verified client path.
        const label=dnsLiveBadge(state.status,state.logPaused);
        presentation.liveBadge=label.text;
        presentation.liveTone=label.tone;
      }
      function formatTime(s){try{return new Date(s).toLocaleTimeString('ru-RU',{hour12:false})}catch(_){return s}}
      const qtypeName=n=>({1:'A',28:'AAAA',15:'MX',16:'TXT',5:'CNAME',12:'PTR',33:'SRV',65:'HTTPS',64:'SVCB',255:'ANY'}[n]||'TYPE '+(n??'—'));
      function renderLogs(){
        const rows=(state.logPaused?state.frozenLogs:state.logs);
        logView={rows:rows.map(t=>({...t})),upstreams:state.config.upstreams.map(u=>({...u})),
          paused:state.logPaused,updated:presentation.logUpdated,error:state.lastLogError,
          filter:byId('logFilter').value,type:byId('logType').value,errorsOnly:byId('onlyErrors').checked};
        presentation.logCounter=state.logs.length;
        presentation.logVisible=logView.rows.filter(t=>(!logView.type||qtypeName(t.type)===logView.type)&&(!logView.errorsOnly||!t.success)&&(!logView.filter||[t.name,t.client,t.device_name,t.device_mac,t.assigned,t.used].some(x=>String(x||'').toLowerCase().includes(logView.filter.trim().toLowerCase())))).length+' / '+state.logs.length;
        byId('logPause').textContent=state.logPaused?'▶ Продолжить':'⏸ Пауза';
        syncLiveBadge();
      }
      async function refreshLogs(){
        if(state.logPaused)return;
        const t=await api('service/traces');state.logs=Array.isArray(t.traces)?t.traces.slice(-128):[];
        presentation.logUpdated='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});
        renderLogs();
      }
      async function load(){const version=draftVersion;const d=await api('config');if(draftVersion!==version)throw Error('Черновик изменился во время загрузки. Полученный ответ не применён.');if(!d.config||!d.revision||!Array.isArray(d.config.rules)||!Array.isArray(d.config.upstreams))throw Error('Неверный формат конфигурации DNS');state.config=d.config;defaultSelection=d.config.service?.default_upstream||'';state.revision=d.revision;state.dirty=false;state.editing=null;state.editingList='__legacy__';state.editorBase=null;byId('selectedDNSBadge').textContent='DNS не выбран';editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';syncEditorSnapshot();render();const initial=state.config.upstreams.find(u=>u.transport!=='native');if(initial)editList(initial.id);await refreshStatus();await refreshHealth().catch(()=>{});await Promise.all([refreshNativeDNS(),refreshLogs().catch(()=>{})]);show('Настройки загружены. Можно менять DNS и списки.','good');}
      async function task(fn,refresh=true){if(state.busy)return;busy(true);try{await fn();}catch(e){show(e.message,'bad');}finally{try{if(refresh&&state.status){try{await refreshStatus();}catch(_){}}}finally{busy(false);}}}
      // Buttons, selects and log filters are Svelte-owned. This controller
      // exposes commands only; it never installs a second DOM click listener.
      const controlOperations={
        'reload':()=>task(async()=>{if((state.dirty||editorPending())&&!confirm('Отменить все несохранённые изменения, включая текст доменов?'))return;await load();}),
        'nav-dns':()=>setPage('dns'),
        'nav-logs':()=>setPage('logs'),
        'editor-choice':()=>{const chosen=byId('editorDNSChoice').value,previous=state.editing||'';if(!chosen){if(previous)closeEditor();byId('editorDNSChoice').value=state.editing||'';return;}if(!editList(chosen))byId('editorDNSChoice').value=previous;},
        'dns-add':()=>openDNSForm(null),
        'dns-cancel':()=>{byId('dnsAddPanel').hidden=true;state.editingDNSId=null;state.checked=null;},
        'dns-draft-change':()=>{state.checked=null;byId('commitDNS').disabled=true;byId('certificateInfo').textContent='Адрес изменён. Нажми «Проверить». ';resetDraftInspection();byId('certificateDetails').hidden=true;},
        'dns-protocol-change':()=>{byId('newAddress').placeholder=byId('newProtocol').value==='doh'?'https://dns.example.com/dns-query':byId('newProtocol').value==='udp'?'9.9.9.9':'dns.example.com';},
        'dns-inspect':()=>task(inspect,false),
        'dns-commit':()=>task(addDNS,false),
        'default-change':()=>{defaultSelection=byId('defaultDNS').value;mark();},
        'dns-check-all':()=>task(async()=>{await checkHealth();}),
        'save':()=>task(save),
        'apply':()=>task(apply),
        'dns-migrate':()=>task(migrate),
        'gate-preflight':()=>task(async()=>{
          const retry=Boolean(state.status?.system_gate?.desired)&&['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(state.status?.system_gate?.state);
          await api('service/gate/control',opts('service-control',{operation:retry?'retry':'preflight'},true));
          await refreshStatus();show(retry?'Контроллер повторно проверен. Реальный LAN-путь проверь отдельно.':'Готовность подтверждена. Следующий шаг — «Включить для LAN». Настройки не изменены.','good');
        }),
        'gate-enable':()=>task(async()=>{
          if(state.dirty || editorPending())throw Error('Сначала сохрани и примени все изменения DNS.');
          if(!confirm('Включить RouterForge для обычных DNS-запросов ВСЕХ устройств LAN (UDP/TCP 53)? Приватные DoH/DoT приложений не перехватываются. Штатные настройки Keenetic останутся без изменений. Подключать отдельное тестовое устройство не нужно.'))return;
          await api('service/gate/control',opts('service-control',{operation:'enable'},true));
          await refreshStatus();show('Основной DNS для LAN включён. Проверь запросы устройств в журнале.','good');
        }),
        'gate-disable':()=>task(async()=>{
          if(!confirm('Отключить системный DNS Gate и вернуть обычную обработку Keenetic?'))return;
          await api('service/gate/control',opts('service-control',{operation:'disable'},true));
          await refreshStatus();show('Перехват RouterForge отключён. Действуют обычные DNS-правила роутера.','good');
        }),
        'verify':()=>task(async()=>{const r=await serviceCommand('verify',{operation:'verify'});if(r.state!=='PASS')throw Error('Проверка DNS: '+r.state);show('DNS-сервис ответил на '+r.results.length+' прямых UDP/TCP проверок. Для LAN-перехвата отдельно используй «Проверить готовность».','good');}),
        'stop':()=>task(async()=>{if(!confirm('Остановить DNS и вернуть прежний DNS подключённого устройства?'))return;await serviceCommand('control',{operation:'stop'});show('DNS остановлен, прежний DNS устройства возвращён.','good');}),
        'connect':()=>task(async()=>{const mac=byId('clientMAC').value.trim(),persist=byId('persistClient').checked;if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(mac))throw Error('Укажи MAC в формате 02:11:22:33:44:55');if(!confirm('Назначить DNS Router устройству '+mac+'?'+(persist?' Keenetic сохранит общую конфигурацию.':'')))return;await serviceCommand('connect',{mac,persist});show('Устройство подключено. Открой журнал, затем попробуй открыть сайт на устройстве.','good');}),
        'disconnect':()=>task(async()=>{if(!confirm('Вернуть прежний DNS выбранного устройства?'))return;await serviceCommand('disconnect',{operation:'disconnect'});show('Прежний DNS восстановлен.','good');}),
        'log-refresh':()=>task(async()=>{await refreshStatus();await refreshLogs();}),
        'log-filter':()=>renderLogs(),
        'log-pause':()=>{if(!state.logPaused){state.frozenLogs=state.logs.slice();state.logPaused=true;renderLogs();}else{state.logPaused=false;state.frozenLogs=[];refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));}},
        'log-copy':()=>{const lines=(state.logPaused?state.frozenLogs:state.logs).map(t=>[t.time,t.client,t.name,qtypeName(t.type),traceEndpoint(t,'assigned'),traceEndpoint(t,'used'),t.transport,t.rcode,t.success?'PASS':'FAIL',t.latency_ms].join('\t')).join('\n');if(navigator.clipboard?.writeText)navigator.clipboard.writeText(lines).then(()=>show('Журнал скопирован.','good')).catch(()=>show('Не удалось скопировать журнал.','bad'));else show('Копирование недоступно в этом браузере.','bad');}
      };
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
      const p21Draft=()=>{const a=state.config.advanced||{};return {devices:a.devices||[],groups:a.groups||[],rules:a.rules||[]};};
      const p21MAC=raw=>cleanP21MAC(raw,p21Text('badMAC'));
      function p21TargetValue(t){return t?(t.group_id?'group:'+t.group_id:'dns:'+t.upstream):'';}
      function p21Target(value){if(!value)return null;if(value.startsWith('group:')){const g=(state.config.dns_groups||[]).find(g=>g.id===value.slice(6));if(!g?.servers?.length)throw Error('Unknown DNS group');return {upstream:g.servers[0],group_id:g.id};}const id=value.slice(4);if(!state.config.upstreams.some(u=>u.id===id&&u.transport!=='native'))throw Error('Unknown direct DNS');return {upstream:id};}
      function p21TargetName(t){if(!t)return p21Text('common');if(t.group_id){const g=(state.config.dns_groups||[]).find(g=>g.id===t.group_id);return g?.name||t.group_id;}const u=state.config.upstreams.find(u=>u.id===t.upstream);return u?displayName(u):t.upstream;}
      function p21Policy(a,mac){const d=(a?.devices||[]).find(d=>d.mac===mac);if(d?.target)return p21TargetName(d.target);const g=(a?.groups||[]).find(g=>g.macs.includes(mac));return g?.target?p21TargetName(g.target)+' ['+g.name+']':p21Text('common');}
      // P21 list presentation has one owner: native Svelte rows. No DOM-created
      // buttons and no duplicated event handlers in P22.
      function p21Render(){
        if(disposed)return;
        const a=p21Draft();
        const active=Boolean(state.status?.device_policies_active);
        const badge=byId('p21ActiveBadge');
        if(badge){badge.hidden=!active;badge.textContent=p21Text('active');}
        const cache=state.status?.cache_mode==='disabled_for_device_policy_isolation'?p21Text('cacheOff'):p21Text('cacheNormal');
        const records=aggregateP21Devices(p21Inventory.devices||[],a);
        const devices=records.slice(0,512).map(d=>({
          key:d.mac,title:(d.name||p21Text('unknown'))+' · '+d.mac,
          description:(d.ips.join('\n')||p21Text('offline'))+'\n'+(d.trusted?p21Text('fresh'):p21Text('offline'))+'\n'+p21Text('draft')+': '+p21Policy(a,d.mac)+' · '+p21Text('live')+': '+p21Policy(p21Inventory.applied,d.mac)+' · '+p21Text('rules')+': '+a.rules.filter(r=>r.mac===d.mac).length+' / '+((p21Inventory.applied?.rules||[]).filter(r=>r.mac===d.mac).length),
          actions:[{name:'edit',label:p21Text('edit')},{name:'reset',label:p21Text('reset')}]
        }));
        const groups=a.groups.map(g=>({key:g.id,title:g.name,description:g.macs.join(', ')+' → '+p21TargetName(g.target),actions:[{name:'remove',label:p21Text('remove')}]}));
        const rules=a.rules.map(r=>({key:r.id,title:r.mac+' · '+r.domain,description:r.match+' → '+p21TargetName(r.target)+' · '+r.priority,actions:[{name:'edit',label:p21Text('edit')},{name:'remove',label:p21Text('remove')}]}));
        p21Rows={devices,groups,rules};
        const targets=[...state.config.upstreams.filter(u=>u.transport!=='native').map(u=>({value:'dns:'+u.id,label:displayName(u)})),...(state.config.dns_groups||[]).map(g=>({value:'group:'+g.id,label:'DNS '+g.name}))];
        p21Model={rows:p21Rows,targets,groups:a.groups.map(g=>({...g,targetValue:p21TargetValue(g.target)})),inventory:p21Inventory.devices||[],cache,discoveryStatus:(p21Inventory.issues||[]).join(' · ')+(p21Inventory.applied_revision?' · '+p21Text('live')+': '+p21Inventory.applied_revision:'')};
      }
      function p21RowsAction(kind,action,key){
        const a=p21Draft();
        if(kind==='devices'){
          if(action==='edit'){
            const d=aggregateP21Devices(p21Inventory.devices||[],a).find(x=>x.mac===key);
            if(!d)return;
            const configured=a.devices.find(x=>x.mac===key);
            p21Selection={kind:'device',generation:++p21SelectionSeq,mac:key,name:d.name||'',target:p21TargetValue(configured?.target),inherit:Boolean(configured?.inherit_domains)};
            return;
          }
          if(action==='reset'){state.config.advanced=dropDeviceOverrides(a,key);mark();show(p21Text('saved'));return;}
        }
        if(kind==='groups'&&action==='remove'){state.config.advanced=dropGroup(a,key);mark();return;}
        if(kind==='rules'){
          const r=a.rules.find(x=>x.id===key);if(!r)return;
          if(action==='edit'){
            p21Selection={kind:'rule',generation:++p21SelectionSeq,mac:r.mac,domain:r.domain,match:r.match,target:p21TargetValue(r.target),priority:r.priority};
            return;
          }
          if(action==='remove'){state.config.advanced=dropRule(a,key);mark();return;}
        }
        throw Error('UNKNOWN_P21_ACTION');
      }
      // Single read-only inventory owner for P21/P22. One in-flight GET is
      // shared by both views; late responses are ignored after unmount.
      let p21InventoryFlight=null;
      async function p21Refresh(){
        if(disposed)return;
        if(p21InventoryFlight)return p21InventoryFlight;
        const flight=(async()=>{
          const inventory=await api('service/devices');
          if(disposed)return;
          p21Inventory=inventory;
          p21Render();
          discoveryHooks.render(inventory);
          reportHeight();
        })();
        p21InventoryFlight=flight;
        try{return await flight;}
        finally{if(p21InventoryFlight===flight)p21InventoryFlight=null;}
      }
      discoveryHooks.refresh=p21Refresh;
      discoveryHooks.snapshot=()=>p21Inventory;

      function p21SetMode(value){
        const mode=value==='advanced'?'advanced':'basic';
        byId('dnsLiteApp').dataset.dnsMode=mode;
        const legacyMode=byId('p21Mode');if(legacyMode)legacyMode.value=mode;
        modeView=mode;
        try{localStorage.setItem('routerforge.dnsRouter.mode.v1',mode);}catch(_){}
        if(mode==='basic'&&state.page==='devices')setPage('dns');
        if(mode==='advanced')p21Refresh().catch(e=>{if(!disposed)show(e.message,'bad');});
        reportHeight();
      }
      function p21UsesDNS(id){const a=p21Draft();return [...a.devices,...a.groups,...a.rules].some(x=>x.target?.upstream===id);}
      function p21Setup(){
        // The hidden compatibility mode selector is read by existing integrations.
        // P21 forms and P22 navigation are owned by Svelte; no DOM click handlers.
        let mode='basic';try{mode=localStorage.getItem('routerforge.dnsRouter.mode.v1')||mode;}catch(_){}
        p21SetMode(mode);
      }
      commands={
        control:(name,payload)=>{if(disposed)return;const operation=controlOperations[name];if(!operation)throw Error('UNKNOWN_DNS_LITE_CONTROL_COMMAND: '+name);return operation(payload);},
        card:(name,id,value)=>{
          if(name==='edit')return editList(id);
          if(name==='change')return openDNSForm(id);
          if(name==='check')return task(()=>checkHealth(id));
          if(name==='remove')return task(()=>removeDNS(id),false);
          if(name==='fallback'){
            const u=state.config.upstreams.find(x=>x.id===id);
            if(!u||u.transport==='native')throw Error('DNS не найден или не поддерживает резервирование');
            const allowed=state.config.upstreams.filter(x=>x.id!==id&&x.transport!=='native'&&!(x.fallbacks||[]).length);
            if(value&&!allowed.some(x=>x.id===value))throw Error('Недопустимый резервный DNS');
            if(value&&state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))throw Error('Этот DNS уже является резервным');
            u.fallbacks=value?[value]:[];mark();
          }
        },
        editor:(name,value)=>{
          if(name==='input'){
            editorDraft.domains=String(value);
            if(state.status)renderGate(state.status);
            return;
          }
          if(name==='new-name'){editorDraft.newName=String(value);return;}
          if(name==='rename-name'){editorDraft.renameName=String(value);return;}
          if(name==='select'){
            if(state.busy)return false;
            try{return switchList(value);}catch(err){show(err.message,'bad');return false;}
          }
          if(name==='close')return closeEditor();
          if(name==='create'){editorDraft.newName=String(value);return task(newList,false);}
          if(name==='rename'){editorDraft.renameName=String(value);return task(renameList,false);}
          if(name==='delete')return task(deleteList,false);
          if(name==='save'){editorDraft.domains=String(value);return task(saveList,false);}
          if(name==='import-data')return task(async()=>{
            if(!state.editing||!listIsEditable(state.editing,state.editingList))throw Error('Невозможно импортировать в этот список');
            const names=parseDomains(String(value));
            editorDraft.domains=names.join('\n');
            syncEditorSnapshot();
            show('Импортировано '+names.length+' доменов. Нажми «Сохранить список».');
          },false);
          if(name==='import-error')return show('Импорт: '+String(value),'bad');
          if(name==='export-error')return show('Экспорт недоступен в этом браузере.','bad');
        },
        mode:(mode)=>{p21SetMode(mode);if(mode==='advanced'){p21Section='devices';setPage('devices');}},
        refresh:()=>task(refreshStatus,false),
        p21Row:(kind,action,key)=>task(()=>p21RowsAction(kind,action,key),false),
        p21Navigate:(section)=>{
          if(!['devices','groups','rules','settings'].includes(section))return;
          p21Section=section;setPage('devices');
          p21Refresh().catch(e=>{if(!disposed)show(e.message,'bad');});
        },
        p21Form:(kind,payload={})=>task(async()=>{
          if(kind==='refresh'){await p21Refresh();return;}
          if(kind==='save'){await save();return;}
          if(kind==='apply'){await apply();await p21Refresh();return;}
          if(kind==='device'){
            state.config.advanced=draftDevice(p21Draft(),{mac:payload.mac,name:payload.name,target:p21Target(payload.target),inherit_domains:payload.inherit_domains},p21Text('badMAC'));
            mark();show(p21Text('saved'));return;
          }
          if(kind==='group'){
            const previous=p21Draft(),id=payload.id||newId('devices',previous.groups);
            state.config.advanced=draftGroup(previous,{id,name:payload.name,members:payload.members,target:p21Target(payload.target),inherit_domains:payload.inherit_domains},{invalidMAC:p21Text('badMAC'),required:p21Text('required'),conflict:p21Text('conflict')});
            mark();show(p21Text('saved'));return;
          }
          if(kind==='rule'){
            state.config.advanced=draftRule(p21Draft(),{mac:payload.mac,domains:parseDomains(payload.domain),target:p21Target(payload.target),priority:payload.priority,match:payload.match},{invalidMAC:p21Text('badMAC'),required:p21Text('required')});
            mark();show(p21Text('saved'));return;
          }
          if(kind==='probe'){
            p21Route=null;
            if(!payload.applied&&(state.dirty||editorPending()))throw Error(p21Text('saveFirst'));
            const body={operation:'evaluate',domain:String(payload.domain||'').trim(),mac:String(payload.mac||'').trim()?p21MAC(payload.mac):'',use_applied:!!payload.applied};
            p21Route=null;
            const result=await api('service/devices/evaluate',opts('service-control',body,true));
            if(disposed)return;
            p21ProbeMAC=body.mac;
            p21Route=result;
            return;
          }
          throw Error('UNKNOWN_P21_COMMAND');
        },false)
      };
      navigationHooks.navigate=section=>commands.p21Navigate(section);
      p21Setup();
      const root=document.documentElement,params=new URLSearchParams(window.location.search);
      function syncTheme(){try{if(window.parent===window)return;const host=window.parent.document.documentElement,computed=window.parent.getComputedStyle(host);for(const token of ['--rf-bg','--rf-surface','--rf-surface-2','--rf-text','--rf-muted','--rf-border','--rf-border-strong','--rf-hover','--rf-accent','--rf-accent-soft','--rf-success','--rf-danger','--rf-radius-panel','--rf-radius-control','--rf-ui-panel-radius','--rf-ui-control-radius','--rf-ui-control-h']){const val=computed.getPropertyValue(token).trim();if(val)root.style.setProperty(token,val);}root.style.colorScheme=computed.colorScheme==='light'?'light':'dark';}catch(_){}}
      function reportHeight(){if(window.parent===window)return;try{window.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height:Math.ceil(Math.max(350,byId('dnsLiteApp').scrollHeight,byId('dnsLiteApp').getBoundingClientRect().height)+6)},window.location.origin);}catch(_){}}
      syncTheme();try{if(window.parent!==window&&window.MutationObserver)new MutationObserver(()=>{syncTheme();reportHeight();}).observe(window.parent.document.documentElement,{attributes:true,attributeFilter:['style','class','data-theme']});}catch(_){}
      const heightObserver=window.ResizeObserver?new ResizeObserver(reportHeight):null;if(heightObserver)heightObserver.observe(byId('dnsLiteApp'));
      window.addEventListener('resize',reportHeight);
      intervalHandles.push(setInterval(()=>{if(document.hidden||state.busy)return;refreshHealth().catch(()=>{});refreshNativeDNS().catch(()=>{});},60000));
      intervalHandles.push(setInterval(()=>{if(state.page!=='logs'||document.hidden||state.busy)return;Promise.all([refreshStatus(),refreshLogs()]).then(()=>{state.lastLogError='';}).catch(e=>{if(state.lastLogError!==e.message){state.lastLogError=e.message;show('LIVE: '+e.message,'bad');}});},2000));
      // Keep log badge fresh while the overview is open; no UI-triggered DNS probes.
      intervalHandles.push(setInterval(()=>{if(state.page!=='dns'||document.hidden||state.busy)return;refreshLogs().catch(()=>{});},15000));
      intervalHandles.push(setInterval(()=>{if(!document.hidden&&!state.busy)refreshStatus().catch(()=>{});},15000));
      initialized=true;
      setPage(params.get('view')==='logs'?'logs':'dns');
      load().catch(e=>{if(disposed)return;show('Не удалось открыть DNS Router: '+e.message,'bad');presentation.serviceBadge='Недоступен';presentation.serviceTone='bad';});
      cleanup=()=>{
        disposed=true;apiClient.dispose();
        for(const timer of intervalHandles)clearInterval(timer);
        for(const dispose of eventCleanups)dispose();
        if(heightObserver)heightObserver.disconnect();
        window.removeEventListener('resize',reportHeight);
        commands={};navigationHooks.navigate=()=>{};settingsHooks.change=()=>{};settingsHooks.render=()=>{};discoveryHooks.render=()=>{};discoveryHooks.refresh=()=>Promise.resolve();discoveryHooks.snapshot=()=>null;
      };
    })();
    /* RF_ACTUAL_CONTROLLER_END */
    return () => cleanup();
  });
</script>

<DnsLiteControls onAction={controlAction} page={state.page} cards={cardsView} defaultValue={defaultSelection}/>
<DnsLiteStatus view={presentation}/>
<div use:portal={'rf-dns-lite-svelte-mode'} style="display:contents">
  <DnsModeStatus mode={modeView} status={statusView} busy={state.busy} onMode={modeAction} onRefresh={statusAction}/>
</div>
<div use:portal={'dnsCards'} style="display:contents">
  <DnsLiteCards cards={cardsView} selected={state.editing||''} health={cardHealth} onAction={cardAction}/>
</div>
<div use:portal={'rf-dns-lite-svelte-editor'} style="display:contents">
  <DnsLiteDomainEditor snapshot={editorView} onAction={editorAction}/>
</div>
<div use:portal={'rf-dns-lite-svelte-log'} style="display:contents">
  <DnsLiteLog view={logView}/>
</div>
{#if initialized}
  <DnsP22Navigation mode={modeView} page={state.page} section={p21Section} busy={state.busy} onNavigate={p21Navigate}/>
  <DnsP21Workspace model={p21Model} section={p21Section} busy={state.busy} selection={p21Selection} route={p21Route} probeMac={p21ProbeMAC} onAction={p21FormAction} onRowAction={p21RowAction}/>
  <DnsP25Settings/>
{/if}
