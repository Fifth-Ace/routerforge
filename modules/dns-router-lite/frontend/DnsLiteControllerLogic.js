// DNS Lite business lifecycle extracted from the Svelte component.
// Svelte owns every reactive view and the only root; no global bridge.
// Guarded REST/CAS/Gate effects intentionally remain here pending full native UI ownership.
import {cleanP21MAC, draftDevice, draftGroup, draftRule, dropDeviceOverrides, dropGroup, dropRule, aggregateP21Devices} from './DnsP21Draft.js';
import {createDnsLiteApi} from './DnsLiteApi.js';
import {dnsLiveBadge} from './DnsLiteStatusModel.js';
import {hydrateLiteSettings,refreshLiteSettingsOptions,buildLiteServicePrefs,mergeLiteServicePrefs,parseLiteDomains} from './DnsLiteSettingsModel.js';
import {visibleLiteLogs} from './DnsLiteLogModel.js';

export function startDnsLiteController({state,ui,presentation,settingsHooks,discoveryHooks,navigationHooks,controller,reportHeight=()=>{}}) {
    let cleanup = () => {};

      const apiClient = createDnsLiteApi({fetchImpl:(...args)=>fetch(...args)});
      let draftVersion=0, saving=false, disposed=false;
      let nativeDNSRead=null;
      const settingsFields=settingsHooks.fields;
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
      const show = (message,tone='') => {if(disposed)return;presentation.feedbackText=String(message);presentation.feedbackTone=tone;};
      const busy = on => {if(disposed)return;state.busy=on;if(!on&&state.status)renderGate(state.status);};
      async function api(path, options={}) {
        return apiClient.request(path,options);
      }
      // All P25/P27-P31/P24x fields now belong exclusively to Svelte state.
      // The controller sees values only through the model, never via DOM.
      settingsHooks.change=(id='')=>{
        if(disposed)return;
        try {
          if(id==='p241IPMode')settingsFields.p241IPEnabled=true;
          if(id==='p241IPEnabled'&&!settingsFields.p241IPEnabled)settingsFields.p242HealthEnabled=false;
          syncPrefs(false);
          draftVersion++;state.dirty=true;
          show('Есть несохранённые изменения. Действующий DNS не менялся.');
        }catch(err){show(err.message,'bad');}
      };
      const opts=(action,body,match)=>({method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...(match?{'If-Match':'"'+state.revision+'"'}:{})},body:JSON.stringify(body)});
      function mark(){draftVersion++;state.dirty=true;ui.p21Route=null;show('Есть несохранённые изменения. Действующий DNS не менялся.');render();}
      function setPage(page){state.page=page;// Page visibility and navigation are reactive Svelte-owned controls.
if(page==='logs')refreshLogs().catch(e=>show('Журнал: '+e.message,'bad'));reportHeight();}
      function render(){
        p21Render();
        const list=state.config.upstreams||[];
        const saved=ui.defaultSelection;
        ui.defaultSelection=list.some(u=>u.id===saved)?saved:'';
        settingsFields.defaultDNS=ui.defaultSelection;
        ui.cardsView = list.map(u => {
          const rules=used(u.id);
          const namedLists=(state.config.lists||[]).filter(l=>l.upstream===u.id).length;
          const listCount=namedLists+(rules.some(r=>!r.list_id)||!namedLists?1:0);
          const fallbackOptions=list.filter(t=>t.id!==u.id&&t.transport!=='native'&&!(t.fallbacks||[]).length)
            .map(t=>({id:t.id,label:displayName(t)}));
          return {id:u.id,title:hostName(u),transport:u.transport,native:u.transport==='native',
            address:u.address,path:u.path||'',listCount,domainCount:rules.length,
            domains:rules.map(r=>r.domain),fallback:(u.fallbacks||[])[0]||'',fallbackOptions};
        });
        if(state.editing)renderListToolbar();
        refreshLiteSettingsOptions(settingsFields,state.config);
        settingsHooks.render();
        presentation.listenerSummary=state.config.service?.listen||settingsFields.serviceListen;
        
        if(state.status)renderGate(state.status);
        renderHealth();
        reportHeight();
      }
      function healthByID(id){return state.health.find(x=>x.id===id)||null;}
      function healthStateText(h){if(!h)return 'Не проверялся';return {UP:'Доступен',DOWN:'Недоступен',STALE:'Данные устарели',UNKNOWN:'Не проверялся',UNSUPPORTED:'Keenetic'}[h.status]||'Не проверялся';}
      function renderHealth(){
        const next={};
        for(const h of state.health) if(h && typeof h.id==='string') next[h.id]={...h};
        ui.cardHealth=next;
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
        nativeDNSRead?.abort();
        const read=new AbortController();
        nativeDNSRead=read;
        try{
          const response=await fetch('/api/modules/dns/quality?minutes=5',{credentials:'same-origin',cache:'no-store',signal:read.signal});
          if(!response.ok)throw Error('HTTP '+response.status);
          const data=await response.json();
          if(disposed||read.signal.aborted)return;
          if(!Array.isArray(data.upstreams))throw Error('неверный формат');
          presentation.nativeDNSHealth='Модуль DNS доступен · '+data.upstreams.length+' резолверов Keenetic · отдельная статистика';
        }catch(_){if(!disposed&&!read.signal.aborted)presentation.nativeDNSHealth='Модуль DNS недоступен — RouterForge DNS Router работает независимо';}
        finally{if(nativeDNSRead===read)nativeDNSRead=null;}
      }
      const parseDomains=parseLiteDomains;
      let editorGeneration=0;
      function syncEditorSnapshot(){
        ui.editorView={
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
        switchList(state.editingList);render();reportHeight();return true;
      }
      function closeEditor(){
        if(editorPending()&&!confirm('Закрыть редактор? Несохранённый текст будет потерян.'))return false;
        state.editing=null;state.editingList='__legacy__';state.editorBase=null;
        editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';
        render();
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
        if(state.config.service?.default_upstream===id){state.config.service=null;ui.defaultSelection='';settingsFields.defaultDNS='';}
        if(state.editing===id)closeEditor();mark();
      }
      function openDNSForm(id){
        if(editorPending())throw Error('Сначала сохрани несохранённый текст списка');
        const u=id?state.config.upstreams.find(x=>x.id===id):null;
        if(id&&(!u||u.transport==='native'))throw Error('Этот DNS нельзя изменить через простой редактор');
        state.editingDNSId=id||null;state.checked=null;
        ui.dnsForm.title=u?'Изменить DNS / обновить TLS':'Добавить DNS';
        ui.dnsForm.button=u?'Сохранить DNS (черновик)':'Добавить DNS';
        ui.dnsForm.protocol=u?u.transport:'doh';
        ui.dnsForm.address=!u?'':u.transport==='doh'?'https://'+u.server_name+(u.path||'/dns-query'):
          isEncrypted(u.transport)?u.server_name:u.address.replace(/:53$/,'');
        resetDraftInspection();
        ui.dnsForm.certificateInfo=u?'Черновик: проверка не выполнялась. Активный DNS не изменён.':'Проверка не выполнялась.';
        ui.dnsForm.certificateVisible=false;ui.dnsForm.ready=false;
        ui.dnsForm.opened=true;reportHeight();
      }
      function newFormData(){
        let transport=ui.dnsForm.protocol;let raw=ui.dnsForm.address.trim();
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
      function draftStatus(id,text,state='') {ui.dnsForm.checks[id]={text,state};}
      function resetDraftInspection(){
        for(const id of ['draftAddressState','draftIPState','draftTLSState','draftCertificateState','draftDNSState','draftRTTState'])draftStatus(id,'Не проверено');
        ui.dnsForm.error='';
      }
      async function inspect(){
        state.checked=null;ui.dnsForm.ready=false;resetDraftInspection();
        let f;
        try {f=newFormData();}
        catch(e){draftStatus('draftAddressState','Ошибка','bad');ui.dnsForm.error=e.message;return;}
        const key=ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim(),editId=state.editingDNSId;
        draftStatus('draftAddressState',f.host,'ok');draftStatus('draftIPState','Получение…','pending');
        draftStatus('draftTLSState',isEncrypted(f.transport)?'Ожидание…':'Не используется',isEncrypted(f.transport)?'pending':'neutral');
        draftStatus('draftCertificateState',(f.transport==='udp'||f.transport==='tcp')?'Не требуется':'Ожидание…',(f.transport==='udp'||f.transport==='tcp')?'neutral':'pending');
        let r;
        try {r=f.transport==='tcp'?{selected_ip:f.host,address:f.host+':53'}:await api('certificates/lookup',opts('certificate-lookup',{transport:f.transport,server_name:f.host}));}
        catch(e){
          if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
          draftStatus('draftIPState','Не получен','bad');
          if(isEncrypted(f.transport))draftStatus('draftTLSState','Не подтверждён','bad');
          draftStatus('draftCertificateState',isEncrypted(f.transport)?'Не получен':'Не требуется',isEncrypted(f.transport)?'bad':'neutral');
          draftStatus('draftDNSState','Недоступен','bad');
          ui.dnsForm.error=e.message;
          ui.dnsForm.certificateInfo='Адрес / TLS: '+e.message;reportHeight();return;
        }
        if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
        if(!r.address||!r.selected_ip){draftStatus('draftIPState','Не получен','bad');ui.dnsForm.error='Не найден IP DNS-сервера';return;}
        draftStatus('draftIPState',r.address,'ok');
        const preverified=!!r.certificate?.verified;
        if((f.transport==='doh'||f.transport==='dot')&&!preverified){draftStatus('draftTLSState','Ошибка','bad');draftStatus('draftCertificateState','Не получен','bad');ui.dnsForm.error='TLS-сертификат не удалось проверить';return;}
        draftStatus('draftTLSState',preverified?'Проверен':f.transport==='doq'?'QUIC: проверяется запросом':'Не используется',preverified?'ok':f.transport==='doq'?'pending':'neutral');
        draftStatus('draftCertificateState',preverified?'Получен · до '+(r.certificate.not_after||'—'):f.transport==='doq'?'Проверяется при QUIC':'Не требуется',preverified?'ok':'neutral');
        const candidate={id:'p194-draft',transport:f.transport,address:r.address};
        if(isEncrypted(f.transport))candidate.server_name=f.host;
        if(f.transport==='doh')candidate.path=f.path;
        draftStatus('draftDNSState','Проверка…','pending');
        let probe;
        try {probe=await api('service/upstreams/inspect',opts('service-control',{operation:'inspect',upstream:candidate}));}
        catch(e){probe={dns_available:false,status:'DOWN',last_error:e.message,tls_status:'UNCONFIRMED'};}
        if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
        const available=probe.dns_available===true;
        draftStatus('draftDNSState',available?'Доступен':'Недоступен',available?'ok':'bad');
        draftStatus('draftRTTState',available?String(probe.probe_latency_ms)+' мс':'—',available?'ok':'neutral');
        if(f.transport==='doq'){
          draftStatus('draftTLSState',available?'Проверен при QUIC':'Не подтверждён',available?'ok':'bad');
          draftStatus('draftCertificateState',available?'Проверен транспортом':'Не получен',available?'ok':'bad');
        }
        ui.dnsForm.error=available?'':'DNS недоступен: '+(probe.last_error||'нет корректного ответа');
        state.checked={key,form:f,lookup:r,editId,available};
        ui.dnsForm.certificateInfo=(available?'✓ DNS доступен':'⚠ DNS недоступен')+' · IP '+r.selected_ip+' · '+(preverified?'TLS проверен':f.transport==='doq'?'DoQ TLS при запросе':'Без TLS');
        ui.dnsForm.certificateVisible=Boolean(r.certificate);
        ui.dnsForm.certificateText=r.certificate?['Имя: '+r.certificate.server_name,'Кому: '+r.certificate.subject,'Выдан: '+r.certificate.issuer,'Действителен до: '+r.certificate.not_after,'SHA256: '+r.certificate.fingerprint_sha256].join('\n'):'';
        ui.dnsForm.ready=true;reportHeight();
      }
      function addDNS(){
        if(editorPending())throw Error('Сначала сохрани изменённый список доменов справа.');
        const c=state.checked,key=ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim();
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
        state.editingDNSId=null;state.checked=null;ui.dnsForm.opened=false;ui.dnsForm.ready=false;
        resetDraftInspection();ui.dnsForm.certificateInfo='Проверка ещё не выполнялась.';ui.dnsForm.address='';
        mark();
        if(!prev)editList(u.id);
        else if(state.editing===u.id){renderListToolbar();}
        show(prev?'DNS/TLS обновлены в черновике. ID, списки, резерв и настройки по умолчанию сохранены.':'DNS добавлен. Его список открыт справа. Выбери DNS по умолчанию перед применением.','good');
      }
      function servicePrefs(requireDefault=true){return buildLiteServicePrefs(settingsFields,state.config,requireDefault);}
      function syncPrefs(requireDefault=true){
        const next=servicePrefs(requireDefault),prev=state.config.service;
        const merged=mergeLiteServicePrefs(prev,next);
        if(JSON.stringify(prev)!==JSON.stringify(merged)){
          state.config.service=merged;draftVersion++;state.dirty=true;
        }
      }
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
        const endpoint=s.listen||state.config.service?.listen||settingsFields.serviceListen;
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
        ui.controlView.gateEnableDisabled=!s.active||active||recovery||state.dirty||editorPending();
        ui.controlView.gateDisableDisabled=(!active&&!recovery&&!wanted);
        ui.controlView.preflightDisabled=!s.active;
        ui.controlView.preflightLabel=wanted&&recovery?'Повторить восстановление':'Проверить готовность';
        ui.controlView.stopDisabled=!s.active;
        const hits=Number(s.requests||0),denied=Number(s.denied_requests||0);
        const coverage=gate.coverage||{},ipv6=coverage.ipv6_routed==='active_last_verified';
        const coverageText=active?('Перехват: '+(ipv6?'IPv4 и IPv6 global/ULA':'только IPv4')+'. Клиенты с IPv6 link-local source исключены. '):'';
        presentation.gateAdvice=recovery?('Перехват не подтверждён. Желаемое состояние: '+(wanted?'ВКЛ':'ВЫКЛ')+'. '+(gate.last_error||'Проверяем listener, порядок правил и lease')+'. Действие восстановления ограничено; сторонние правила не перезаписываются.'):
          active?(coverageText+'Gate проверен: listener, порядок правил и действующие lease. Запросов сервиса: '+hits+', ACL-отказов: '+denied+'. Реальный путь устройства подтверждается отдельной проверкой, не этим счётчиком.'):
          'Перехват выключен. Последний выбор ON/OFF сохраняется отдельно от черновика конфигурации; при обновлении и перезагрузке он восстанавливается после проверки готовности. Тестовое устройство не требуется.';
      }
      async function refreshStatus(){const s=await api('service/status');if(s.mode!=='independent-dns')throw Error('Загружен не DNS Lite. Установи актуальный DEV-пакет.');state.status=s;ui.statusView=s;renderGate(s);
        const connection=s.connection||{state:'disconnected'},active=Boolean(s.active),same=active&&s.applied_revision===state.revision;
        presentation.serviceBadge=active?'DNS работает':'DNS остановлен';presentation.serviceTone=active?'good':'bad';
        presentation.runStatus=(active?'DNS работает: '+s.listen+' · Основной '+displayName(state.config.upstreams.find(u=>u.id===s.default_upstream)||{transport:'udp',address:s.default_upstream||'—'}):'Сервис остановлен.')+(active&&!same?' · Есть неприменённая версия настроек.':'')+(s.last_error?' · Ошибка: '+s.last_error:'');
        presentation.connectionSummary=s.system_gate?.active?'Перехват LAN включён — проверь запросы реального устройства':connection.state==='connected'?'Точечное устройство: '+connection.mac:connection.state==='recovery_required'?'Точечное подключение требует восстановления':'Для LAN-режима отдельное устройство не требуется';
        presentation.clientState=connection.state==='connected'?connection.mac:connection.state==='disconnected'?'Не подключён':connection.state;
        ui.controlView.connectDisabled=!active||!same||connection.state!=='disconnected';ui.controlView.disconnectDisabled=connection.state==='disconnected';ui.controlView.stopDisabled=!active;
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
        const criteria=ui.logCriteria;
        ui.logView={rows:rows.map(t=>({...t})),upstreams:state.config.upstreams.map(u=>({...u})),
          paused:state.logPaused,updated:presentation.logUpdated,error:state.lastLogError,
          filter:criteria.filter,type:criteria.type,errorsOnly:criteria.errorsOnly};
        presentation.logCounter=state.logs.length;
        presentation.logVisible=visibleLiteLogs(rows,criteria,state.config.upstreams).length+' / '+state.logs.length;
        syncLiveBadge();
      }
      async function refreshLogs(){
        if(state.logPaused)return;
        const t=await api('service/traces');state.logs=Array.isArray(t.traces)?t.traces.slice(-128):[];
        presentation.logUpdated='Обновлено '+new Date().toLocaleTimeString('ru-RU',{hour12:false});
        renderLogs();
      }
      async function load(){const version=draftVersion;const d=await api('config');if(draftVersion!==version)throw Error('Черновик изменился во время загрузки. Полученный ответ не применён.');if(!d.config||!d.revision||!Array.isArray(d.config.rules)||!Array.isArray(d.config.upstreams))throw Error('Неверный формат конфигурации DNS');state.config=d.config;hydrateLiteSettings(settingsFields,d.config);ui.defaultSelection=settingsFields.defaultDNS;state.revision=d.revision;state.dirty=false;state.editing=null;state.editingList='__legacy__';state.editorBase=null;editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';syncEditorSnapshot();render();const initial=state.config.upstreams.find(u=>u.transport!=='native');if(initial)editList(initial.id);await refreshStatus();await refreshHealth().catch(()=>{});await Promise.all([refreshNativeDNS(),refreshLogs().catch(()=>{})]);show('Настройки загружены. Можно менять DNS и списки.','good');}
      async function task(fn,refresh=true){if(state.busy)return;busy(true);try{await fn();}catch(e){show(e.message,'bad');}finally{try{if(refresh&&state.status){try{await refreshStatus();}catch(_){}}}finally{busy(false);}}}
      // Buttons, selects and log filters are Svelte-owned. This controller
      // exposes controller.commands only; it never installs a second DOM click listener.
      const controlOperations={
        'reload':()=>task(async()=>{if((state.dirty||editorPending())&&!confirm('Отменить все несохранённые изменения, включая текст доменов?'))return;await load();}),
        'nav-dns':()=>setPage('dns'),
        'nav-logs':()=>setPage('logs'),
        'editor-choice':(value)=>{const chosen=String(value||'');if(!chosen){if(state.editing)closeEditor();return;}editList(chosen);},
        'dns-add':()=>openDNSForm(null),
        'dns-cancel':()=>{ui.dnsForm.opened=false;state.editingDNSId=null;state.checked=null;ui.dnsForm.ready=false;},
        'dns-draft-change':(payload)=>{
          if(payload?.protocol!==undefined)ui.dnsForm.protocol=String(payload.protocol);
          if(payload?.address!==undefined)ui.dnsForm.address=String(payload.address);
          state.checked=null;ui.dnsForm.ready=false;ui.dnsForm.certificateInfo='Адрес изменён. Нажми «Проверить». ';
          resetDraftInspection();ui.dnsForm.certificateVisible=false;ui.dnsForm.certificateText='';
        },
        'dns-protocol-change':()=>{},
        'dns-inspect':()=>task(inspect,false),
        'dns-commit':()=>task(addDNS,false),
        'default-change':(selection)=>{
          ui.defaultSelection=String(selection||'');settingsFields.defaultDNS=ui.defaultSelection;
          if(ui.defaultSelection&&!state.config.upstreams.some(u=>u.id===ui.defaultSelection))throw Error('Выбранный DNS недоступен');
          syncPrefs(false);mark();
        },
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
        'connect':(data)=>task(async()=>{const mac=String(data?.mac||'').trim(),persist=Boolean(data?.persist);if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(mac))throw Error('Укажи MAC в формате 02:11:22:33:44:55');if(!confirm('Назначить DNS Router устройству '+mac+'?'+(persist?' Keenetic сохранит общую конфигурацию.':'')))return;await serviceCommand('connect',{mac,persist});show('Устройство подключено. Открой журнал, затем попробуй открыть сайт на устройстве.','good');}),
        'disconnect':()=>task(async()=>{if(!confirm('Вернуть прежний DNS выбранного устройства?'))return;await serviceCommand('disconnect',{operation:'disconnect'});show('Прежний DNS восстановлен.','good');}),
        'log-refresh':()=>task(async()=>{await refreshStatus();await refreshLogs();}),
        'log-filter':(criteria)=>{ui.logCriteria={filter:String(criteria?.filter||''),type:String(criteria?.type||''),errorsOnly:Boolean(criteria?.errorsOnly)};renderLogs();},
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
        const cache=state.status?.cache_mode==='disabled_for_device_policy_isolation'?p21Text('cacheOff'):p21Text('cacheNormal');
        const records=aggregateP21Devices(p21Inventory.devices||[],a);
        const devices=records.slice(0,512).map(d=>({
          key:d.mac,title:(d.name||p21Text('unknown'))+' · '+d.mac,
          description:(d.ips.join('\n')||p21Text('offline'))+'\n'+(d.trusted?p21Text('fresh'):p21Text('offline'))+'\n'+p21Text('draft')+': '+p21Policy(a,d.mac)+' · '+p21Text('live')+': '+p21Policy(p21Inventory.applied,d.mac)+' · '+p21Text('rules')+': '+a.rules.filter(r=>r.mac===d.mac).length+' / '+((p21Inventory.applied?.rules||[]).filter(r=>r.mac===d.mac).length),
          actions:[{name:'edit',label:p21Text('edit')},{name:'reset',label:p21Text('reset')}]
        }));
        const groups=a.groups.map(g=>({key:g.id,title:g.name,description:g.macs.join(', ')+' → '+p21TargetName(g.target),actions:[{name:'remove',label:p21Text('remove')}]}));
        const rules=a.rules.map(r=>({key:r.id,title:r.mac+' · '+r.domain,description:r.match+' → '+p21TargetName(r.target)+' · '+r.priority,actions:[{name:'edit',label:p21Text('edit')},{name:'remove',label:p21Text('remove')}]}));
        ui.p21Rows={devices,groups,rules};
        const targets=[...state.config.upstreams.filter(u=>u.transport!=='native').map(u=>({value:'dns:'+u.id,label:displayName(u)})),...(state.config.dns_groups||[]).map(g=>({value:'group:'+g.id,label:'DNS '+g.name}))];
        ui.p21Model={rows:ui.p21Rows,targets,groups:a.groups.map(g=>({...g,targetValue:p21TargetValue(g.target)})),inventory:p21Inventory.devices||[],cache,discoveryStatus:(p21Inventory.issues||[]).join(' · ')+(p21Inventory.applied_revision?' · '+p21Text('live')+': '+p21Inventory.applied_revision:'')};
      }
      function p21RowsAction(kind,action,key){
        const a=p21Draft();
        if(kind==='devices'){
          if(action==='edit'){
            const d=aggregateP21Devices(p21Inventory.devices||[],a).find(x=>x.mac===key);
            if(!d)return;
            const configured=a.devices.find(x=>x.mac===key);
            ui.p21Selection={kind:'device',generation:++ui.p21SelectionSeq,mac:key,name:d.name||'',target:p21TargetValue(configured?.target),inherit:Boolean(configured?.inherit_domains)};
            return;
          }
          if(action==='reset'){state.config.advanced=dropDeviceOverrides(a,key);mark();show(p21Text('saved'));return;}
        }
        if(kind==='groups'&&action==='remove'){state.config.advanced=dropGroup(a,key);mark();return;}
        if(kind==='rules'){
          const r=a.rules.find(x=>x.id===key);if(!r)return;
          if(action==='edit'){
            ui.p21Selection={kind:'rule',generation:++ui.p21SelectionSeq,mac:r.mac,domain:r.domain,match:r.match,target:p21TargetValue(r.target),priority:r.priority};
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
        ui.modeView=mode;
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
      controller.commands={
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
        mode:(mode)=>{p21SetMode(mode);if(mode==='advanced'){ui.p21Section='devices';setPage('devices');}},
        refresh:()=>task(refreshStatus,false),
        p21Row:(kind,action,key)=>task(()=>p21RowsAction(kind,action,key),false),
        p21Navigate:(section)=>{
          if(!['devices','groups','rules','settings'].includes(section))return;
          ui.p21Section=section;setPage('devices');
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
            ui.p21Route=null;
            if(!payload.applied&&(state.dirty||editorPending()))throw Error(p21Text('saveFirst'));
            const body={operation:'evaluate',domain:String(payload.domain||'').trim(),mac:String(payload.mac||'').trim()?p21MAC(payload.mac):'',use_applied:!!payload.applied};
            ui.p21Route=null;
            const result=await api('service/devices/evaluate',opts('service-control',body,true));
            if(disposed)return;
            ui.p21ProbeMAC=body.mac;
            ui.p21Route=result;
            return;
          }
          throw Error('UNKNOWN_P21_COMMAND');
        },false)
      };
      navigationHooks.navigate=section=>controller.commands.p21Navigate(section);
      p21Setup();
      const params=new URLSearchParams(window.location.search);
      // Svelte root schedules visibility-aware polling; the controller exposes
      // DOM-free read operations and retains server/API safety semantics.
      controller.poll={
        health(){if(disposed||state.busy)return;void refreshHealth().catch(()=>{});void refreshNativeDNS().catch(()=>{});},
        logs(){if(disposed||state.busy||state.page!=='logs')return;void Promise.all([refreshStatus(),refreshLogs()]).then(()=>{if(!disposed)state.lastLogError='';}).catch(e=>{if(!disposed&&state.lastLogError!==e.message){state.lastLogError=e.message;show('LIVE: '+e.message,'bad');}});},
        overviewLogs(){if(disposed||state.busy||state.page!=='dns')return;void refreshLogs().catch(()=>{});},
        status(){if(disposed||state.busy)return;void refreshStatus().catch(()=>{});}
      };
      ui.initialized=true;
      setPage(params.get('view')==='logs'?'logs':'dns');
      load().catch(e=>{if(disposed)return;show('Не удалось открыть DNS Router: '+e.message,'bad');presentation.serviceBadge='Недоступен';presentation.serviceTone='bad';});
      cleanup=()=>{
        disposed=true;nativeDNSRead?.abort();nativeDNSRead=null;apiClient.dispose();
        for(const dispose of eventCleanups)dispose();
        controller.commands={};controller.poll={};navigationHooks.navigate=()=>{};settingsHooks.change=()=>{};settingsHooks.render=()=>{};discoveryHooks.render=()=>{};discoveryHooks.refresh=()=>Promise.resolve();discoveryHooks.snapshot=()=>null;
      };

    return () => cleanup();
}
