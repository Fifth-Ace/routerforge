import { rfUi } from './rf-localization.js';
// DNS Lite business lifecycle extracted from the Svelte component.
// Svelte owns every reactive view and the only root; no global bridge.
// Guarded REST/CAS/Gate effects intentionally remain here pending full native UI ownership.
import {cleanP21MAC, draftDevice, draftGroup, draftRule, dropDeviceOverrides, dropGroup, dropRule, aggregateP21Devices} from './DnsP21Draft.js';
import {createDnsLiteApi} from './DnsLiteApi.js';
import {dnsLiveBadge} from './DnsLiteStatusModel.js';
import {hydrateLiteSettings,refreshLiteSettingsOptions,buildLiteServicePrefs,mergeLiteServicePrefs,parseLiteDomains} from './DnsLiteSettingsModel.js';
import {visibleLiteLogs} from './DnsLiteLogModel.js';
import {parseP32Stamp} from './DnsP32Stamp.js';

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
        domains:'',newName:'',renameName:'',title:rfUi('Список доменов',"Domain list"),
        choices:[],selected:'',editable:false,named:false,
        hint:rfUi('Изменения попадут в черновик и начнут действовать после применения.',"Changes will be saved as a draft and will take effect after applying.")
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
        if(t.cache)return rfUi('Кэш · локально',"Cache · local");
        const direct=t[field+'_endpoint'];if(direct)return direct;
        const id=t[field];
        if(id==='system-local')return rfUi('Локальный DNS Keenetic',"Keenetic local DNS");
        if(id==='client-acl'||id==='refused')return rfUi('Отказ ACL',"ACL denied");
        const u=state.config.upstreams.find(x=>x.id===id);
        return u ? (u.server_name?u.server_name+' ('+u.address+')':u.address)+' · '+u.transport+' ['+u.id+']' : (id||'—');
      };
      const protocolNames={doh:'DoH · HTTPS',dot:'DoT · TLS',doq:'DoQ · QUIC',udp:'UDP',tcp:'TCP',stamp:'DNS Stamp',native:rfUi('Старый DNS Keenetic',"Previous Keenetic DNS")};
      const isEncrypted = t => t==='doh'||t==='dot'||t==='doq'||t==='stamp';
      const isSimple = r => r.match==='suffix' && !r.group_id && Object.keys(r).every(k=>['id','domain','match','upstream','priority','list_id'].includes(k));
      const newId = (prefix,items) => {let i=1;const ids=new Set(items.map(x=>x.id));while(ids.has(prefix+'-'+i))i++;return prefix+'-'+i;};
      const displayName = u => u.server_name || (u.transport==='native'?rfUi('Наследованный DNS Keenetic',"Inherited Keenetic DNS"):u.address);
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
          show(rfUi('Есть несохранённые изменения. Действующий DNS не менялся.',"Unsaved changes exist. Active DNS was not changed."));
        }catch(err){show(err.message,'bad');}
      };
      const opts=(action,body,match)=>({method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...(match?{'If-Match':'"'+state.revision+'"'}:{})},body:JSON.stringify(body)});
      function mark(){draftVersion++;state.dirty=true;ui.p21Route=null;show(rfUi('Есть несохранённые изменения. Действующий DNS не менялся.',"Unsaved changes exist. Active DNS was not changed."));render();}
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
        presentation.healthP95=latencies.length?Math.max(...latencies)+rfUi(' мс',"ms"):'—';
        presentation.healthUpdated=rfUi('Обновлено ',"Updated")+new Date().toLocaleTimeString('ru-RU',{hour12:false});
      }
      async function refreshHealth(){
        const data=await api('service/upstreams/health');state.health=Array.isArray(data.upstreams)?data.upstreams:[];renderHealth();return data;
      }
      async function checkHealth(id=''){
        const result=await api('service/upstreams/check',opts('service-control',{operation:'check',upstream:id},true));
        state.health=Array.isArray(result.upstreams)?result.upstreams:[];renderHealth();
        const failed=id?state.health.find(h=>h.id===id&&h.status==='DOWN'):null;show(failed?rfUi('DNS недоступен: ',"DNS unavailable:")+(failed.last_error||rfUi('нет ответа',"no response")):id?rfUi('DNS проверен. Результат и RTT обновлены.',"DNS checked. Result and RTT updated."):rfUi('Проверка всех DNS завершена. Результаты обновлены.',"All DNS checks finished. Results updated."),failed?'bad':'good');
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
          if(!Array.isArray(data.upstreams))throw Error(rfUi('неверный формат',"invalid format"));
          presentation.nativeDNSHealth=rfUi('Модуль DNS доступен · ',"DNS module available ·")+data.upstreams.length+rfUi(' резолверов Keenetic · отдельная статистика',"Keenetic resolvers · separate statistics");
        }catch(_){if(!disposed&&!read.signal.aborted)presentation.nativeDNSHealth=rfUi('Модуль DNS недоступен — RouterForge DNS Router работает независимо',"DNS module unavailable — RouterForge DNS Router works independently");}
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
        editorDraft.choices=entries.map(e=>({id:e.id,label:e.name+' · '+listRules(state.editing,e.id).length+rfUi(' доменов',"domains")}));
        editorDraft.selected=state.editingList;
        const meta=(state.config.lists||[]).find(l=>l.id===state.editingList);
        editorDraft.renameName=meta?meta.name:'';
        editorDraft.named=Boolean(meta);
        editorDraft.editable=listIsEditable(state.editing,state.editingList);
        editorDraft.title=(meta?.name||rfUi('Основной список',"Primary list"))+' · '+displayName(state.config.upstreams.find(u=>u.id===state.editing));
        if(!editorDraft.editable)editorDraft.hint=rfUi('Этот URL/сложный список управляется отдельным механизмом. Ручное редактирование заблокировано.',"This URL/complex list is managed separately. Manual editing is blocked.");
        syncEditorSnapshot();
      }
      function switchList(key){
        if(editorPending()&&!confirm(rfUi('Несохранённый текст текущего списка будет потерян. Переключить?',"Unsaved text in the current list will be lost. Switch?"))){
          // Restore the visible select without altering the draft or the
          // previously selected list. This is a refused user operation.
          syncEditorSnapshot();
          return false;
        }
        if(!listEntries(state.editing).some(l=>l.id===key))throw Error(rfUi('Неизвестный список',"Unknown list"));
        state.editingList=key;
        const rules=listRules(state.editing,key);
        editorDraft.domains=rules.map(r=>r.domain).join('\n');
        state.editorBase=editorDraft.domains;
        editorDraft.hint=rfUi('Редактируется только выбранный список. Остальные списки и DNS не меняются.',"Only the selected list is edited. Other lists and DNS stay unchanged.");
        renderListToolbar();reportHeight();
        return true;
      }
      function editList(id){
        if(editorPending()){
          if(!confirm(rfUi('У выбранного DNS есть несохранённый текст списка. Перейти к другому DNS?',"The selected DNS has unsaved list text. Switch to another DNS?")))return false;
          state.editorBase=editorDraft.domains;
        }
        const u=state.config.upstreams.find(x=>x.id===id);
        if(!u||u.transport==='native')throw Error(rfUi('Legacy DNS нужно сначала преобразовать',"Convert the legacy DNS entry first"));
        // The first editor opening has no previous draft to discard.
        // Seed the comparison baseline before switching from null to DNS id.
        if(state.editing===null)state.editorBase=editorDraft.domains;
        const changed=state.editing!==id;
        state.editing=id;
        if(changed)state.editingList=listEntries(id)[0].id;
        switchList(state.editingList);render();reportHeight();return true;
      }
      function closeEditor(){
        if(editorPending()&&!confirm(rfUi('Закрыть редактор? Несохранённый текст будет потерян.',"Close editor? Unsaved text will be lost.")))return false;
        state.editing=null;state.editingList='__legacy__';state.editorBase=null;
        editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';
        render();
        syncEditorSnapshot();return true;
      }
      function saveList(){
        if(!state.editing)return;
        const id=state.editing,key=state.editingList,old=listRules(id,key);
        if(!listIsEditable(id,key))throw Error(rfUi('URL или сложный список нельзя менять через простой редактор',"URLs and complex lists cannot be edited in the basic editor"));
        const names=parseDomains(editorDraft.domains);
        const meta=(state.config.lists||[]).find(l=>l.id===key);
        const other=new Set(state.config.rules.filter(r=>!(r.upstream===id&&listKey(r)===key)).map(r=>r.domain));
        const collision=names.find(domain=>other.has(domain));
        if(collision)throw Error(rfUi('Домен ',"Domain")+collision+rfUi(' уже есть в другом списке или DNS. Сначала убери дубликат там.',"already exists in another list or DNS. Remove that duplicate first."));
        const oldMap=new Map(old.map(r=>[r.domain,r])),ids=new Set(state.config.rules.map(r=>r.id));
        const fresh=names.map(domain=>{
          if(oldMap.has(domain))return oldMap.get(domain);
          let n=1;while(ids.has('domain-'+n))n++;const uid='domain-'+n;ids.add(uid);
          return {id:uid,domain,match:'suffix',upstream:id,priority:meta?meta.priority:100,...(key==='__legacy__'?{}:{list_id:key})};
        });
        const unchanged=state.config.rules.filter(r=>!(r.upstream===id&&listKey(r)===key));
        if(unchanged.length+fresh.length>10000)throw Error(rfUi('Общий лимит: 10 000 правил',"Total limit: 10,000 rules"));
        state.config.rules=unchanged.concat(fresh);
        editorDraft.domains=names.join('\n');state.editorBase=editorDraft.domains;
        editorDraft.hint=rfUi('Список сохранён в черновик. Применение DNS выполняется отдельно.',"List saved in draft. DNS activation is a separate step.");
        mark();show(rfUi('Список обновлён: ',"List updated:")+names.length+rfUi(' доменов. Остальные списки и DNS не менялись.',"domains. Other lists and DNS were unchanged."));
      }
      function newList(){
        if(!state.editing||editorPending())throw Error(rfUi('Сначала сохрани текст открытого списка',"Save the text in the open list first"));
        const name=editorDraft.newName.trim(),all=state.config.lists||[];
        if(!name||Array.from(name).length>80)throw Error(rfUi('Название списка: от 1 до 80 символов',"List name must contain 1–80 characters"));
        if(all.length>=64)throw Error(rfUi('Лимит: 64 списка',"Limit: 64 lists"));
        if(all.some(l=>l.upstream===state.editing&&l.name.toLowerCase()===name.toLowerCase()))throw Error(rfUi('Список с таким названием уже существует',"A list with this name already exists"));
        const id=newId('list',all);
        state.config.lists=all.concat({id,name,kind:'paste',upstream:state.editing,match:'suffix',priority:100});
        state.editingList=id;editorDraft.newName='';switchList(id);mark();
        show(rfUi('Новый список создан в черновике. Добавь домены и сохрани список.',"New list created in draft. Add domains and save it."),'good');
      }
      function renameList(){
        if(!state.editing||editorPending())throw Error(rfUi('Сначала сохрани текст открытого списка',"Save the text in the open list first"));
        const list=(state.config.lists||[]).find(l=>l.id===state.editingList);
        if(!list)throw Error(rfUi('Основной список старого формата нельзя переименовать',"The legacy primary list cannot be renamed"));
        const name=editorDraft.renameName.trim();
        if(!name||Array.from(name).length>80)throw Error(rfUi('Название списка: от 1 до 80 символов',"List name must contain 1–80 characters"));
        if(state.config.lists.some(l=>l.id!==list.id&&l.upstream===list.upstream&&l.name.toLowerCase()===name.toLowerCase()))throw Error(rfUi('Такое название уже используется',"This name is already in use"));
        list.name=name;mark();show(rfUi('Имя списка изменено в черновике.',"List name changed in draft."),'good');
      }
      function deleteList(){
        if(!state.editing||editorPending())throw Error(rfUi('Сначала сохрани текст открытого списка',"Save the text in the open list first"));
        const list=(state.config.lists||[]).find(l=>l.id===state.editingList);
        if(!list)throw Error(rfUi('Основной список старого формата нельзя удалить через это действие',"The legacy primary list cannot be deleted with this action"));
        const size=listRules(state.editing,list.id).length;
        if(!confirm(rfUi('Удалить список «',"Delete list “")+list.name+'» и '+size+rfUi(' правил только ИЗ ЧЕРНОВИКА?',"rules FROM DRAFT only?")))return;
        state.config.lists=state.config.lists.filter(l=>l.id!==list.id);
        state.config.rules=state.config.rules.filter(r=>r.list_id!==list.id);
        state.editingList=listEntries(state.editing)[0].id;switchList(state.editingList);mark();
      }
      function removeDNS(id){
        const u=state.config.upstreams.find(x=>x.id===id);if(!u)return;
        if((state.config.dns_groups||[]).some(g=>g.servers?.includes(id))||state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))throw Error(rfUi('Этот DNS используется группой или резервной цепочкой. Сначала убери зависимость.',"This DNS is used by a group or fallback chain. Remove the dependency first."));
        if(p21UsesDNS(id))throw Error(rfUi('Этот DNS используется правилами устройств. Сначала измени их.',"This DNS is used by device rules. Update those rules first."));
        const rules=used(id);if(rules.some(r=>!isSimple(r)))throw Error(rfUi('Есть сложные правила, удаление через простой экран заблокировано.',"Complex rules exist; deletion is blocked in the basic editor."));
        if(!confirm(rfUi('Удалить ',"Delete")+displayName(u)+' и '+rules.length+' его доменов ИЗ ЧЕРНОВИКА?'))return;
        state.config.upstreams=state.config.upstreams.filter(x=>x.id!==id);state.config.rules=state.config.rules.filter(r=>r.upstream!==id);state.config.lists=(state.config.lists||[]).filter(l=>l.upstream!==id);
        if(state.config.service?.default_upstream===id){state.config.service=null;ui.defaultSelection='';settingsFields.defaultDNS='';}
        if(state.editing===id)closeEditor();mark();
      }
      function openDNSForm(id){
        if(editorPending())throw Error(rfUi('Сначала сохрани несохранённый текст списка',"Save the unsaved list text first"));
        const u=id?state.config.upstreams.find(x=>x.id===id):null;
        if(id&&(!u||u.transport==='native'))throw Error(rfUi('Этот DNS нельзя изменить через простой редактор',"This DNS cannot be edited in the basic editor"));
        state.editingDNSId=id||null;state.checked=null;
        ui.dnsForm.title=u?rfUi('Изменить DNS / обновить TLS',"Edit DNS / refresh TLS"):rfUi('Добавить DNS',"Add DNS");
        ui.dnsForm.button=u?'Сохранить DNS (черновик)':rfUi('Добавить DNS',"Add DNS");
        ui.dnsForm.protocol=u?u.transport:'doh';
        ui.dnsForm.http3Mode=u?.http3_mode||'off';ui.dnsForm.bootstrapIPs=(u?.bootstrap_ips||[]).join('\n');
        ui.dnsForm.address=!u?'':u.transport==='stamp'?u.stamp:u.transport==='doh'?'https://'+u.server_name+(u.path||'/dns-query'):
          isEncrypted(u.transport)?u.server_name:u.address.replace(/:53$/,'');
        resetDraftInspection();
        ui.dnsForm.certificateInfo=u?rfUi('Черновик: проверка не выполнялась. Активный DNS не изменён.',"Draft: no check run. Active DNS unchanged."):rfUi('Проверка не выполнялась.',"Check not run.");
        ui.dnsForm.certificateVisible=false;ui.dnsForm.ready=false;
        ui.dnsForm.opened=true;reportHeight();
      }
      function newFormData(){
        let transport=ui.dnsForm.protocol;let raw=ui.dnsForm.address.trim();
        if(!raw)throw Error(rfUi('Укажи адрес DNS',"Enter a DNS address"));
        let host='',path='';
        if(transport==='stamp'){const s=parseP32Stamp(raw);return {...s,path:'',http3Mode:'off',bootstrapIPs:[]};}
        if(transport==='doh'){
          let url;try{url=new URL(/^https:\/\//i.test(raw)?raw:'https://'+raw)}catch(_){throw Error(rfUi('Неверный DoH URL','Invalid DoH URL'));}
          if(url.protocol!=='https:'||url.username||url.password||url.search||url.hash||url.port||url.pathname.includes('..')||url.pathname.startsWith('//'))throw Error(rfUi('DoH: нужен HTTPS-адрес без нестандартного порта и параметров',"DoH requires an HTTPS URL without custom ports or query parameters"));
          host=url.hostname.toLowerCase();path=url.pathname==='/'?'/dns-query':url.pathname;
        }else{if(/[\/:?@\s]/.test(raw)&&transport!=='udp')throw Error(rfUi('Нужен домен DNS-сервера без протокола и порта',"Enter the DNS server host name without protocol or port"));host=raw.toLowerCase();}
        if(transport==='udp'||transport==='tcp'){if(!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(raw)||raw.split('.').some(n=>Number(n)>255))throw Error(rfUi('Для UDP укажи IPv4-адрес, например 9.9.9.9',"For UDP, enter an IPv4 address, e.g. 9.9.9.9"));host=raw;}
        if(isEncrypted(transport)&&(!/^[a-z0-9-]+(?:\.[a-z0-9-]+)+$/.test(host)))throw Error(rfUi('Для защищённого DNS требуется имя сервера',"Secure DNS requires a server name"));
        const bootstrapIPs=String(ui.dnsForm.bootstrapIPs||'').split(/[\n,;]/).map(x=>x.trim()).filter(Boolean);
        if(bootstrapIPs.length>4||new Set(bootstrapIPs).size!==bootstrapIPs.length)throw Error(rfUi('Допустимо не более 4 уникальных Bootstrap IP',"Up to four unique bootstrap IP addresses are allowed"));
        const http3Mode=transport==='doh'?(ui.dnsForm.http3Mode||'off'):'off';
        return {transport,host,path,http3Mode,bootstrapIPs};
      }
      function draftStatus(id,text,state='') {ui.dnsForm.checks[id]={text,state};}
      function resetDraftInspection(){
        for(const id of ['draftAddressState','draftIPState','draftTLSState','draftCertificateState','draftDNSState','draftRTTState'])draftStatus(id,rfUi('Не проверено',"Not checked"));
        ui.dnsForm.error='';
      }
      async function inspect(){
        state.checked=null;ui.dnsForm.ready=false;resetDraftInspection();
        let f;
        try {f=newFormData();}
        catch(e){draftStatus('draftAddressState',rfUi('Ошибка',"Error"),'bad');ui.dnsForm.error=e.message;return;}
        const key=ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim(),editId=state.editingDNSId;
        draftStatus('draftAddressState',f.host,'ok');draftStatus('draftIPState',rfUi('Получение…',"Fetching…"),'pending');
        draftStatus('draftTLSState',isEncrypted(f.transport)?rfUi('Ожидание…',"Waiting…"):rfUi('Не используется',"Not in use"),isEncrypted(f.transport)?'pending':'neutral');
        draftStatus('draftCertificateState',(f.transport==='udp'||f.transport==='tcp')?rfUi('Не требуется',"Not required"):rfUi('Ожидание…',"Waiting…"),(f.transport==='udp'||f.transport==='tcp')?'neutral':'pending');
        let r;
        try {r=f.transport==='stamp'?{selected_ip:f.host,address:f.address}:f.transport==='tcp'?{selected_ip:f.host,address:f.host+':53'}:await api('certificates/lookup',opts('certificate-lookup',{transport:f.transport,server_name:f.host}));}
        catch(e){
          if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
          draftStatus('draftIPState',rfUi('Не получен',"Not received"),'bad');
          if(isEncrypted(f.transport))draftStatus('draftTLSState',rfUi('Не подтверждён',"Not verified"),'bad');
          draftStatus('draftCertificateState',isEncrypted(f.transport)?rfUi('Не получен',"Not received"):rfUi('Не требуется',"Not required"),isEncrypted(f.transport)?'bad':'neutral');
          draftStatus('draftDNSState',rfUi('Недоступен',"Unavailable"),'bad');
          ui.dnsForm.error=e.message;
          ui.dnsForm.certificateInfo='Адрес / TLS: '+e.message;reportHeight();return;
        }
        if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
        if(!r.address||!r.selected_ip){draftStatus('draftIPState',rfUi('Не получен',"Not received"),'bad');ui.dnsForm.error=rfUi('Не найден IP DNS-сервера',"DNS server IP not found");return;}
        draftStatus('draftIPState',r.address,'ok');
        const preverified=!!r.certificate?.verified;
        if((f.transport==='doh'||f.transport==='dot')&&!preverified){draftStatus('draftTLSState',rfUi('Ошибка',"Error"),'bad');draftStatus('draftCertificateState','Не получен','bad');ui.dnsForm.error='TLS-сертификат не удалось проверить';return;}
        draftStatus('draftTLSState',preverified?rfUi('Проверен',"Verified"):f.transport==='doq'?rfUi('QUIC: проверяется запросом',"QUIC: verified on request"):rfUi('Не используется',"Not in use"),preverified?'ok':f.transport==='doq'?'pending':'neutral');
        draftStatus('draftCertificateState',preverified?rfUi('Получен · до ',"Received · until")+(r.certificate.not_after||'—'):f.transport==='doq'?rfUi('Проверяется при QUIC',"Verified when queried via QUIC"):rfUi('Не требуется',"Not required"),preverified?'ok':'neutral');
        const candidate={id:'p194-draft',transport:f.transport,address:r.address};
        if(f.transport==='stamp')candidate.stamp=f.stamp;
        if(f.transport==='doh'&&f.http3Mode!=='off')candidate.http3_mode=f.http3Mode;
        if(f.bootstrapIPs?.length)candidate.bootstrap_ips=f.bootstrapIPs;
        if(isEncrypted(f.transport)&&f.transport!=='stamp')candidate.server_name=f.host;
        if(f.transport==='doh')candidate.path=f.path;
        draftStatus('draftDNSState',rfUi('Проверка…',"Checking…"),'pending');
        let probe;
        try {probe=await api('service/upstreams/inspect',opts('service-control',{operation:'inspect',upstream:candidate}));}
        catch(e){probe={dns_available:false,status:'DOWN',last_error:e.message,tls_status:'UNCONFIRMED'};}
        if(key!==ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim()||editId!==state.editingDNSId)return;
        const available=probe.dns_available===true;
        draftStatus('draftDNSState',available?rfUi('Доступен',"Available"):rfUi('Недоступен',"Unavailable"),available?'ok':'bad');
        draftStatus('draftRTTState',available?String(probe.probe_latency_ms)+rfUi(' мс',"ms"):'—',available?'ok':'neutral');
        if(f.transport==='doq'||f.transport==='stamp'){
          draftStatus('draftTLSState',available?'Проверен при QUIC':rfUi('Не подтверждён',"Not verified"),available?'ok':'bad');
          draftStatus('draftCertificateState',available?rfUi('Проверен транспортом',"Verified by transport"):rfUi('Не получен',"Not received"),available?'ok':'bad');
        }
        ui.dnsForm.error=available?'':rfUi('DNS недоступен: ',"DNS unavailable:")+(probe.last_error||rfUi('нет корректного ответа',"no valid response"));
        state.checked={key,form:f,lookup:r,editId,available};
        ui.dnsForm.certificateInfo=(available?rfUi('✓ DNS доступен',"✓ DNS available"):rfUi('⚠ DNS недоступен',"⚠ DNS unavailable"))+' · IP '+r.selected_ip+' · '+(preverified?rfUi('TLS проверен',"TLS verified"):f.transport==='doq'?rfUi('DoQ TLS при запросе',"DoQ TLS checked on request"):rfUi('Без TLS',"Without TLS"));
        ui.dnsForm.certificateVisible=Boolean(r.certificate);
        ui.dnsForm.certificateText=r.certificate?[rfUi('Имя: ',"Name:")+r.certificate.server_name,rfUi('Кому: ',"Assigned to:")+r.certificate.subject,rfUi('Выдан: ',"Issued:")+r.certificate.issuer,rfUi('Действителен до: ',"Valid until:")+r.certificate.not_after,'SHA256: '+r.certificate.fingerprint_sha256].join('\n'):'';
        ui.dnsForm.ready=true;reportHeight();
      }
      function addDNS(){
        if(editorPending())throw Error(rfUi('Сначала сохрани изменённый список доменов справа.',"Save the edited domain list on the right first."));
        const c=state.checked,key=ui.dnsForm.protocol+'|'+ui.dnsForm.address.trim();
        if(!c||c.key!==key||c.editId!==state.editingDNSId)throw Error(rfUi('Сначала проверь именно этот адрес DNS и TLS',"Verify this DNS address and TLS first"));
        if(c.available!==true&&!confirm(rfUi('DNS недоступен. Сохранить его в черновик всё равно? Работающий DNS не изменится до применения.',"DNS unavailable. Save it to the draft anyway? Active DNS will not change until you apply.")))return;
        const f=c.form,r=c.lookup;
        if(f.transport==='stamp'&&state.config.upstreams.some(u=>u.id!==state.editingDNSId&&u.transport==='stamp'&&u.stamp===f.stamp))throw Error(rfUi('Такой DNS Stamp уже добавлен',"This DNS stamp has already been added"));
        const prev=state.editingDNSId?state.config.upstreams.find(u=>u.id===state.editingDNSId):null;
        if(state.editingDNSId&&!prev)throw Error(rfUi('Редактируемый DNS не найден',"The DNS entry being edited was not found"));
        if(state.config.upstreams.some(u=>u.id!==prev?.id&&u.transport===f.transport&&u.address===r.address&&u.server_name===(isEncrypted(f.transport)?f.host:'')&&(u.path||'')===f.path))throw Error(rfUi('Такой DNS уже добавлен',"This DNS server has already been added"));
        const u=prev||{id:newId('dns',state.config.upstreams)};
        u.transport=f.transport;u.address=r.address;
        delete u.server_name;delete u.path;delete u.certificate_pem;delete u.http3_mode;delete u.bootstrap_ips;delete u.stamp;
        if(isEncrypted(f.transport)&&f.transport!=='stamp')u.server_name=f.host;
        if(f.transport==='doh')u.path=f.path;
        if(f.transport==='doh'&&f.http3Mode!=='off')u.http3_mode=f.http3Mode;
        if(f.bootstrapIPs?.length)u.bootstrap_ips=f.bootstrapIPs;
        if(f.transport==='stamp')u.stamp=f.stamp;
        if(r.certificate?.verified)u.certificate_pem=r.certificate.pem;
        if(!prev)state.config.upstreams.push(u);
        state.editingDNSId=null;state.checked=null;ui.dnsForm.opened=false;ui.dnsForm.ready=false;
        resetDraftInspection();ui.dnsForm.certificateInfo=rfUi('Проверка ещё не выполнялась.',"No test has been run yet.");ui.dnsForm.address='';
        mark();
        if(!prev)editList(u.id);
        else if(state.editing===u.id){renderListToolbar();}
        show(prev?rfUi('DNS/TLS обновлены в черновике. ID, списки, резерв и настройки по умолчанию сохранены.',"DNS/TLS updated in draft. IDs, lists, fallbacks and default settings preserved."):rfUi('DNS добавлен. Его список открыт справа. Выбери DNS по умолчанию перед применением.',"DNS added. Its list is open on the right. Choose a default DNS before applying."),'good');
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
        if(saving)throw Error(rfUi('Сохранение DNS уже выполняется',"A DNS save operation is already running"));
        if(editorPending())throw Error(rfUi('Сначала нажми «Сохранить список» в редакторе доменов.',"First select Save list in the domain editor."));
        syncPrefs();
        if(!state.dirty)return;
        const version=draftVersion, revision=state.revision;
        const payload=JSON.stringify(state.config);
        saving=true;
        try {
          const data=await api('config',{method:'PUT',headers:{'Content-Type':'application/json',
            'X-RouterForge-Action':'save','If-Match':'"'+revision+'"'},body:payload});
          if(!data || typeof data.revision!=='string' || !data.revision)
            throw Error(rfUi('Сервер не вернул ревизию после сохранения DNS',"The server did not return a revision after saving DNS"));
          state.revision=data.revision;
          state.dirty=draftVersion!==version || JSON.stringify(state.config)!==payload;
          show(state.dirty?rfUi('Черновик изменился во время сохранения. Повтори сохранение последней версии.',"Draft changed during save. Save the latest revision again."):rfUi('Настройки сохранены. Действующий DNS не изменился. Для запуска нажми «Сохранить и запустить DNS».',"Settings saved. Active DNS unchanged. Choose “Save and start DNS” to start."),state.dirty?'':'good');
          render();
        } finally { saving=false; }
      }
      async function serviceCommand(path,body){return api('service/'+path,opts('service-control',body,true));}
      function hasUsedNative(){return state.config.upstreams.some(u=>u.transport==='native'&&(used(u.id).length||state.config.service?.default_upstream===u.id));}
      async function apply(){
        // Unsaved editor content blocks every save/start path before prefs or API.
        if(editorPending())throw Error(rfUi('Сначала нажми «Сохранить список» в редакторе доменов.',"First select Save list in the domain editor."));
        if(hasUsedNative())throw Error(rfUi('Есть старые native DNS. Сначала нажми «Преобразовать DNS» и проверь адреса.',"Legacy native DNS entries exist. Select Convert DNS and verify their addresses first."));
        const prefs=servicePrefs();
        if(!confirm(rfUi('Сохранить настройки и запустить DNS Router на ',"Save settings and start DNS Router on")+prefs.listen+rfUi('? Это ещё не переключит всю LAN. Для этого отдельно включи «Основной DNS для LAN».',"? This will not redirect the entire LAN. Enable “Default DNS for LAN” separately.")))return;
        await save();if(state.dirty)throw Error(rfUi('Черновик изменился во время сохранения. Сначала сохрани новые изменения.',"The draft changed during saving. Save the new changes first."));const s=await api('service/status');
        await serviceCommand('control',{operation:s.active?'apply':'start'});
        show(rfUi('DNS Router запущен. Проверь готовность, затем включи «Основной DNS для LAN». Отдельное тестовое устройство не требуется.',"DNS Router is running. Verify readiness, then enable “Default DNS for LAN”. No separate test device is required."),'good');await refreshStatus();
      }
      async function migrate(){
        if(state.dirty)throw Error(rfUi('Сначала сохрани изменения. Миграция выполняется только из сохранённого состояния.',"Save the changes first. Migration requires a saved configuration."));
        const s=await api('service/status');if(s.active||s.connection?.state!=='disconnected')throw Error(rfUi('Перед миграцией останови сервис и отключи тестовое устройство.',"Stop the service and disconnect the test device before migration."));
        if(!confirm(rfUi('Преобразовать используемые native DNS Keenetic в прямые DoH-соединения? При ошибке конвертации ничего не изменится.',"Convert Keenetic native DNS entries in use into direct DoH connections? On conversion error, nothing changes.")))return;
        await serviceCommand('migrate-native',{operation:'migrate'});await load();show(rfUi('DNS преобразованы. Проверь адреса и сертификаты перед применением.',"DNS entries converted. Verify addresses and certificates before applying."),'good');
      }
      function routingHint(s){
        const c=s.connection||{state:'disconnected'};
        const endpoint=s.listen||state.config.service?.listen||settingsFields.serviceListen;
        const upstream=Number(s.requests||0);
        if(s.system_gate?.active)return {tone:'good',text:rfUi('Основной DNS для LAN активен. Сервис обработал ',"Default DNS for LAN active. Service processed")+upstream+rfUi(' запросов. Это не аппаратное доказательство перехвата: проверь запрос устройства к внешнему DNS и соответствующую запись журнала.',"queries. This does not prove packet interception: verify an external DNS query from the device and its log entry.")};
        if(!s.active)return {tone:'bad',text:rfUi('DNS Router остановлен. На штатном DNS Keenetic (порт 53) интернет может продолжать работать, но журнал RouterForge пуст.',"DNS Router is stopped. Keenetic system DNS (port 53) may still work, but RouterForge's log will be empty.")};
        if(c.state==='recovery_required')return {tone:'bad',text:rfUi('DNS-профиль устройства требует восстановления: ',"Device DNS profile needs restoration:")+(c.error||rfUi('состояние не совпадает с Keenetic',"state differs from Keenetic"))+rfUi('. Не устанавливай и не обновляй пакет до исправления профиля.',". Do not install or update the package until the profile is repaired.")};
        if(c.state==='pending')return {tone:'bad',text:rfUi('Подключение устройства не завершено (pending). DNS Router слушает ',"Device connection is pending. DNS Router is listening")+endpoint+rfUi(', но профиль клиента Keenetic ещё не подтверждён. Сначала верни прежний DNS устройства или заверши подключение; не пытайся обновлять пакет с pending.',", but the Keenetic client profile is not yet verified. Restore the device's previous DNS or finish connecting it before updating the package.")};
        const denied=Number(s.denied_requests||0),acl=Number(s.acl_denied_requests||0),protection=Number(s.protection_denied_requests||0);
        if(denied>0 && denied===upstream){
          if(acl===denied)return {tone:'bad',text:rfUi('До DNS Router дошли ',"Reached DNS Router:")+denied+rfUi(' запросов, но ВСЕ отклонены (REFUSED) входным фильтром. Проверь разрешённые клиентские сети и корректность запросов; менять настройки Keenetic не требуется.',"queries, but ALL were rejected (REFUSED) by the ingress filter. Check allowed client networks and queries; no Keenetic settings change is needed.")};
          if(protection===denied)return {tone:'warn',text:rfUi('До DNS Router дошли ',"Reached DNS Router:")+denied+rfUi(' запросов, но ВСЕ отклонены защитой запросов (NOTIMP/REFUSED). Проверь настройки защиты и причины отказов в журнале.',"queries, but ALL were rejected by query protection (NOTIMP/REFUSED). Check protection settings and denial reasons in the log.")};
          return {tone:'warn',text:rfUi('До DNS Router дошли ',"Reached DNS Router:")+denied+rfUi(' запросов, но ВСЕ отклонены. Причины могут отличаться: открой журнал и проверь конкретные записи.',"queries, but ALL were rejected. Reasons may differ; inspect individual log records.")};
        }
        if(upstream===0)return {tone:'warn',text:rfUi('DNS Router слушает ',"DNS Router listening on")+endpoint+rfUi(', но получил 0 запросов. Планшет может использовать штатный DNS Keenetic (:53), свой Private DNS/DoH или другой DNS-профиль. ',", but received 0 queries. The tablet may be using Keenetic's system DNS (:53), its own Private DNS/DoH or another DNS profile.")+(c.state==='connected'?rfUi('Профиль назначен, но прохождение трафика не подтверждено.',"Profile assigned but traffic routing not verified."):rfUi('Тестовый клиент не подключён через RouterForge.',"Test client is not connected through RouterForge."))+rfUi(' Прямая кнопка «Проверить DNS» тестирует только сервис, не планшет.',"“Check DNS” tests the service only, not the tablet.")};
        if(c.state!=='connected')return {tone:'warn',text:rfUi('DNS Router получил ',"DNS Router received")+upstream+rfUi(' запросов, но клиентский профиль не подключён. Это может быть ручная проверка сервиса; маршрут планшета через Keenetic не доказан.',"queries, but the client profile is not connected. This may be a manual service check; the tablet route through Keenetic has not been proven.")};
        return {tone:'',text:rfUi('DNS Router получил ',"DNS Router received")+upstream+rfUi(' запросов на ',"queries for")+endpoint+rfUi('. Клиентский профиль Keenetic подключён, но чтобы подтвердить именно планшет, сопоставь время его DNS-запросов с журналом.',". The Keenetic client profile is connected, but to verify the tablet itself, compare query timestamps with the log.")};
      }
      function showRoutingHint(s){const hint=routingHint(s);presentation.routeAdvice=hint.text;presentation.routeTone=hint.tone;}
      function renderGate(s){
        const gate=s.system_gate||{state:'STOCK',active:false};
        const active=Boolean(gate.active),recovery=['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(gate.state),wanted=Boolean(gate.desired||s.desired?.interception_enabled);
        presentation.healthGate=active?'RouterForge':recovery?rfUi('Проверка',"Check"):'Keenetic';
        presentation.gatePanelState=recovery?'recovery':active?'active':'stock';
        presentation.gateBadge=gate.state==='CONFLICT'?rfUi('Конфликт',"Conflict"):gate.state==='DEGRADED'?rfUi('Восстановление',"Restoration"):recovery?rfUi('Требуется проверка',"Verification required"):active?rfUi('Активен',"Active"):rfUi('Выключен',"Disabled");
        presentation.gateTone=active?'good':recovery?'bad':'';
        ui.controlView.gateEnableDisabled=!s.active||active||recovery||state.dirty||editorPending();
        ui.controlView.gateDisableDisabled=(!active&&!recovery&&!wanted);
        ui.controlView.preflightDisabled=!s.active;
        ui.controlView.preflightLabel=wanted&&recovery?rfUi('Повторить восстановление',"Retry restoration"):rfUi('Проверить готовность',"Check readiness");
        ui.controlView.stopDisabled=!s.active;
        const hits=Number(s.requests||0),denied=Number(s.denied_requests||0);
        const coverage=gate.coverage||{},ipv6=coverage.ipv6_routed==='active_last_verified';
        const coverageText=active?(rfUi('Перехват: ',"Interception:")+(ipv6?rfUi('IPv4 и IPv6 global/ULA',"IPv4 and IPv6 global/ULA"):rfUi('только IPv4',"IPv4 only"))+rfUi('. Клиенты с IPv6 link-local source исключены. ',". Clients with IPv6 link-local source addresses are excluded.")):'';
        presentation.gateAdvice=recovery?(rfUi('Перехват не подтверждён. Желаемое состояние: ',"Interception not verified. Desired state:")+(wanted?rfUi('ВКЛ',"ON"):rfUi('ВЫКЛ',"OFF"))+'. '+(gate.last_error||rfUi('Проверяем listener, порядок правил и lease',"Checking listener, rule order and leases"))+rfUi('. Действие восстановления ограничено; сторонние правила не перезаписываются.',". Restoration is restricted; third-party rules are not overwritten.")):
          active?(coverageText+rfUi('Gate проверен: listener, порядок правил и действующие lease. Запросов сервиса: ',"Traffic routing verified: listener, rule order and active leases. Service queries:")+hits+rfUi(', ACL-отказов: ',", ACL denials:")+denied+rfUi('. Реальный путь устройства подтверждается отдельной проверкой, не этим счётчиком.',". Verify the device's actual route separately; this counter does not prove it.")):
          rfUi('Перехват выключен. Последний выбор ON/OFF сохраняется отдельно от черновика конфигурации; при обновлении и перезагрузке он восстанавливается после проверки готовности. Тестовое устройство не требуется.',"Interception disabled. The last ON/OFF choice is stored separately from the draft and restored after readiness checks on update/reboot. No test device is needed.");
      }
      async function refreshStatus(){const s=await api('service/status');if(s.mode!=='independent-dns')throw Error(rfUi('Загружен не DNS Lite. Установи актуальный DEV-пакет.',"The loaded module is not DNS Lite. Install a compatible RouterForge package."));state.status=s;ui.statusView=s;renderGate(s);
        const connection=s.connection||{state:'disconnected'},active=Boolean(s.active),same=active&&s.applied_revision===state.revision;
        presentation.serviceBadge=active?'DNS работает':rfUi('DNS остановлен',"DNS stopped");presentation.serviceTone=active?'good':'bad';
        presentation.runStatus=(active?rfUi('DNS работает: ',"DNS working:")+s.listen+rfUi(' · Основной ',"· Primary")+displayName(state.config.upstreams.find(u=>u.id===s.default_upstream)||{transport:'udp',address:s.default_upstream||'—'}):rfUi('Сервис остановлен.',"Service stopped."))+(active&&!same?rfUi(' · Есть неприменённая версия настроек.',"· There are unapplied settings."):'')+(s.last_error?rfUi(' · Ошибка: ',"· Error:")+s.last_error:'');
        presentation.connectionSummary=s.system_gate?.active?rfUi('Перехват LAN включён — проверь запросы реального устройства',"LAN interception enabled — verify queries from an actual device"):connection.state==='connected'?rfUi('Точечное устройство: ',"Specific device:")+connection.mac:connection.state==='recovery_required'?rfUi('Точечное подключение требует восстановления',"Device-specific connection requires restoration"):rfUi('Для LAN-режима отдельное устройство не требуется',"LAN mode does not require a separate device");
        presentation.clientState=connection.state==='connected'?connection.mac:connection.state==='disconnected'?rfUi('Не подключён',"Not connected"):connection.state;
        ui.controlView.connectDisabled=!active||!same||connection.state!=='disconnected';ui.controlView.disconnectDisabled=connection.state==='disconnected';ui.controlView.stopDisabled=!active;
        showRoutingHint(s);
        presentation.metricRequests=s.requests??0;presentation.metricCache=s.cache_hits??0;presentation.metricErrors=s.failures??0;presentation.metricRunning=active?rfUi('Работает',"Running"):'Выкл.';
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
        presentation.logUpdated=rfUi('Обновлено ',"Updated")+new Date().toLocaleTimeString('ru-RU',{hour12:false});
        renderLogs();
      }
      async function load(){const version=draftVersion;const d=await api('config');if(draftVersion!==version)throw Error(rfUi('Черновик изменился во время загрузки. Полученный ответ не применён.',"The draft changed while loading. The received response was not applied."));if(!d.config||!d.revision||!Array.isArray(d.config.rules)||!Array.isArray(d.config.upstreams))throw Error(rfUi('Неверный формат конфигурации DNS',"Invalid DNS configuration format"));state.config=d.config;hydrateLiteSettings(settingsFields,d.config);ui.defaultSelection=settingsFields.defaultDNS;state.revision=d.revision;state.dirty=false;state.editing=null;state.editingList='__legacy__';state.editorBase=null;editorDraft.domains='';editorDraft.choices=[];editorDraft.selected='';syncEditorSnapshot();render();const initial=state.config.upstreams.find(u=>u.transport!=='native');if(initial)editList(initial.id);await refreshStatus();await refreshHealth().catch(()=>{});await Promise.all([refreshNativeDNS(),refreshLogs().catch(()=>{})]);show(rfUi('Настройки загружены. Можно менять DNS и списки.',"Settings loaded. You can edit DNS servers and lists."),'good');}
      async function task(fn,refresh=true){if(state.busy)return;busy(true);try{await fn();}catch(e){show(e.message,'bad');}finally{try{if(refresh&&state.status){try{await refreshStatus();}catch(_){}}}finally{busy(false);}}}
      // Buttons, selects and log filters are Svelte-owned. This controller
      // exposes controller.commands only; it never installs a second DOM click listener.
      const controlOperations={
        'reload':()=>task(async()=>{if((state.dirty||editorPending())&&!confirm(rfUi('Отменить все несохранённые изменения, включая текст доменов?',"Discard all unsaved changes, including domain text?")))return;await load();}),
        'nav-dns':()=>setPage('dns'),
        'nav-logs':()=>setPage('logs'),
        'editor-choice':(value)=>{const chosen=String(value||'');if(!chosen)return state.editing?closeEditor():true;return editList(chosen);},
        'dns-add':()=>openDNSForm(null),
        'dns-cancel':()=>{ui.dnsForm.opened=false;state.editingDNSId=null;state.checked=null;ui.dnsForm.ready=false;},
        'dns-draft-change':(payload)=>{
          if(payload?.protocol!==undefined)ui.dnsForm.protocol=String(payload.protocol);
          if(payload?.address!==undefined)ui.dnsForm.address=String(payload.address);
          if(payload?.http3Mode!==undefined)ui.dnsForm.http3Mode=String(payload.http3Mode);
          if(payload?.bootstrapIPs!==undefined)ui.dnsForm.bootstrapIPs=String(payload.bootstrapIPs);
          state.checked=null;ui.dnsForm.ready=false;ui.dnsForm.certificateInfo=rfUi('Адрес изменён. Нажми «Проверить». ',"Address changed. Choose “Check”.");
          resetDraftInspection();ui.dnsForm.certificateVisible=false;ui.dnsForm.certificateText='';
        },
        'dns-protocol-change':()=>{},
        'dns-inspect':()=>task(inspect,false),
        'dns-commit':()=>task(addDNS,false),
        'default-change':(selection)=>{
          ui.defaultSelection=String(selection||'');settingsFields.defaultDNS=ui.defaultSelection;
          if(ui.defaultSelection&&!state.config.upstreams.some(u=>u.id===ui.defaultSelection))throw Error(rfUi('Выбранный DNS недоступен',"The selected DNS is unavailable"));
          syncPrefs(false);mark();
        },
        'dns-check-all':()=>task(async()=>{
          // Read status before the check: health completion remains the final
          // visible action (no post-check refresh can obscure its result).
          await refreshStatus();
          await checkHealth();
        },false),
        'save':()=>task(save),
        'apply':()=>task(apply),
        'dns-migrate':()=>task(migrate),
        'gate-preflight':()=>task(async()=>{
          const retry=Boolean(state.status?.system_gate?.desired)&&['DEGRADED','RECOVERY_REQUIRED','CONFLICT'].includes(state.status?.system_gate?.state);
          await api('service/gate/control',opts('service-control',{operation:retry?'retry':'preflight'},true));
          await refreshStatus();show(retry?rfUi('Контроллер повторно проверен. Реальный LAN-путь проверь отдельно.',"Controller rechecked. Verify the actual LAN packet path separately."):rfUi('Готовность подтверждена. Следующий шаг — «Включить для LAN». Настройки не изменены.',"Readiness verified. Next: “Enable for LAN”. Settings unchanged."),'good');
        }),
        'gate-enable':()=>task(async()=>{
          if(state.dirty || editorPending())throw Error(rfUi('Сначала сохрани и примени все изменения DNS.',"Save and apply all DNS changes first."));
          if(!confirm(rfUi('Включить RouterForge для обычных DNS-запросов ВСЕХ устройств LAN (UDP/TCP 53)? Приватные DoH/DoT приложений не перехватываются. Штатные настройки Keenetic останутся без изменений. Подключать отдельное тестовое устройство не нужно.',"Enable RouterForge for standard DNS queries from ALL LAN devices (UDP/TCP 53)? Private app DoH/DoT will not be intercepted. Keenetic system settings stay unchanged. No separate test device is needed.")))return;
          await api('service/gate/control',opts('service-control',{operation:'enable'},true));
          await refreshStatus();show(rfUi('Основной DNS для LAN включён. Проверь запросы устройств в журнале.',"Default DNS for LAN enabled. Check device queries in the log."),'good');
        }),
        'gate-disable':()=>task(async()=>{
          if(!confirm(rfUi('Отключить системный DNS Gate и вернуть обычную обработку Keenetic?',"Disable LAN DNS interception and restore normal Keenetic handling?")))return;
          await api('service/gate/control',opts('service-control',{operation:'disable'},true));
          await refreshStatus();show(rfUi('Перехват RouterForge отключён. Действуют обычные DNS-правила роутера.',"RouterForge interception disabled. Normal router DNS rules apply."),'good');
        }),
        'verify':()=>task(async()=>{const r=await serviceCommand('verify',{operation:'verify'});if(r.state!=='PASS')throw Error(rfUi('Проверка DNS: ',"DNS test:")+r.state);show(rfUi('DNS-сервис ответил на ',"DNS service responded to")+r.results.length+rfUi(' прямых UDP/TCP проверок. Для LAN-перехвата отдельно используй «Проверить готовность».',"direct UDP/TCP checks. For LAN interception, run the separate readiness check."),'good');}),
        'stop':()=>task(async()=>{if(!confirm(rfUi('Остановить DNS и вернуть прежний DNS подключённого устройства?',"Stop DNS and restore the connected device's previous DNS?")))return;await serviceCommand('control',{operation:'stop'});show(rfUi('DNS остановлен, прежний DNS устройства возвращён.',"DNS stopped; device's previous DNS restored."),'good');}),
        'connect':(data)=>task(async()=>{const mac=String(data?.mac||'').trim(),persist=Boolean(data?.persist);if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(mac))throw Error(rfUi('Укажи MAC в формате 02:11:22:33:44:55',"Enter MAC as 02:11:22:33:44:55"));if(!confirm(rfUi('Назначить DNS Router устройству ',"Assign DNS Router to device")+mac+'?'+(persist?rfUi(' Keenetic сохранит общую конфигурацию.',"Keenetic will save the system-wide configuration."):'')))return;await serviceCommand('connect',{mac,persist});show(rfUi('Устройство подключено. Открой журнал, затем попробуй открыть сайт на устройстве.',"Device connected. Open the log, then try visiting a site on the device."),'good');}),
        'disconnect':()=>task(async()=>{if(!confirm(rfUi('Вернуть прежний DNS выбранного устройства?',"Restore the selected device's previous DNS?")))return;await serviceCommand('disconnect',{operation:'disconnect'});show(rfUi('Прежний DNS восстановлен.',"Previous DNS restored."),'good');}),
        'log-refresh':()=>task(async()=>{await refreshStatus();await refreshLogs();}),
        'log-filter':(criteria)=>{ui.logCriteria={filter:String(criteria?.filter||''),type:String(criteria?.type||''),errorsOnly:Boolean(criteria?.errorsOnly)};renderLogs();},
        'log-pause':()=>{if(!state.logPaused){state.frozenLogs=state.logs.slice();state.logPaused=true;renderLogs();}else{state.logPaused=false;state.frozenLogs=[];refreshLogs().catch(e=>show(rfUi('Журнал: ',"Log:")+e.message,'bad'));}},
        'log-copy':()=>{const lines=(state.logPaused?state.frozenLogs:state.logs).map(t=>[t.time,t.client,t.name,qtypeName(t.type),traceEndpoint(t,'assigned'),traceEndpoint(t,'used'),t.transport,t.rcode,t.success?'PASS':'FAIL',t.latency_ms].join('\t')).join('\n');if(navigator.clipboard?.writeText)navigator.clipboard.writeText(lines).then(()=>show('Журнал скопирован.','good')).catch(()=>show('Не удалось скопировать журнал.','bad'));else show('Копирование недоступно в этом браузере.','bad');}
      };
      // P21: optional display mode. Policies live in the applied config, never here.
      let p21Inventory={devices:[],issues:[],applied:null,applied_revision:''};
      const p21English=(new URLSearchParams(location.search).get('lang')||document.documentElement.lang||'ru').toLowerCase().startsWith('en');
      const p21Labels={
        mode:[rfUi('Режим',"Mode"),'Mode'],basic:[rfUi('Обычный',"Normal"),'Basic'],advanced:['Расширенный','Advanced'],devices:['Устройства и правила','Devices and rules'],
        draftHelp:['Изменения здесь — черновик. Сначала сохранить, затем явно применить DNS.','Changes are drafts. Save, then explicitly apply DNS.'],refresh:['Обновить обнаружение','Refresh discovery'],save:[rfUi('Сохранить черновик',"Save draft"),'Save draft'],apply:[rfUi('Сохранить и применить',"Save and apply"),'Save and apply'],
        identityHelp:[rfUi('MAC — не аутентификация. Нужен свежий прямой L2-сосед. DHCP даёт имя и проверку противоречий. Клиенты за NAT, VPN или другим роутером не различаются; неизвестный клиент использует общие правила.',"A MAC address is not authentication. A recently observed direct Layer 2 neighbor is required. DHCP provides a name and helps detect conflicts. Clients behind NAT, VPN, or another router cannot be distinguished; unknown clients use shared rules."),'MAC is not authentication. A fresh direct LAN neighbor is required. DHCP only supplies names and conflict checks. Clients behind NAT, VPN or another router cannot be distinguished; unidentified clients use common rules.'],
        mac:['MAC устройства','Device MAC'],name:[rfUi('Название',"Name"),'Name'],target:['DNS или DNS-группа','DNS or DNS group'],inherit:['Наследовать общие доменные правила перед DNS устройства','Inherit common domain rules before the device default'],setDevice:['Добавить / обновить устройство в черновике','Add / update device draft'],groups:['Группы устройств','Device groups'],editGroup:['Редактируемая группа','Group to edit'],members:['MAC участников — по одному в строке','Member MACs, one per line'],groupInherit:['Наследовать общие доменные правила перед DNS группы','Inherit common domain rules before the group default'],setGroup:['Сохранить группу в черновике','Save group draft'],rules:['Правила домена для устройства','Per-device domain rules'],domain:[rfUi('Домен',"Domain"),'Domain'],match:[rfUi('Совпадение',"Match type"),'Match'],priority:['Приоритет при равной точности: меньше — раньше','Equal specificity: lower priority number wins'],setRule:['Добавить / обновить правило в черновике','Add / update rule draft'],explain:['Приоритеты и проверка маршрута','Precedence and route explanation'],
        precedence:[rfUi('1. Домен устройства → 2. DNS устройства → 3. Группа устройств → 4. Общие домены → 5. DNS по умолчанию. При наследовании общие домены проверяются перед DNS устройства/группы. Локальные зоны всегда обслуживает штатный локальный DNS.',"1. Device domain → 2. Device DNS → 3. Device group → 4. Shared domains → 5. Default DNS. When inherited, shared domains are checked before device/group DNS. Local zones always use the system local DNS."),'1. Device domain → 2. Device DNS → 3. Device group → 4. Common domain → 5. Default DNS. Explicit inheritance checks common domains before the device/group default. Protected local zones always use local system DNS.'],
        applied:[rfUi('Проверять применённые правила (иначе сохранённый черновик)',"Check applied rules (otherwise the saved draft)"),'Evaluate applied rules (otherwise saved draft)'],probe:[rfUi('Объяснить маршрут',"Explain route"),'Explain route'],notE2E:[rfUi('Это проверка конфигурации по выбранному MAC, не доказательство определения клиента или сетевого E2E.',"This checks configuration for the selected MAC, not client identity or end-to-end network behavior."),'This is configuration evaluation for a manually selected MAC, not verified client identity or network E2E.'],
        active:['Активны правила устройств','Device rules are active'],cacheOff:['Общий кэш отключён: изоляция применённых политик устройств.','Shared cache disabled to isolate applied device policies.'],cacheNormal:['Нет применённых политик устройств. Кэш работает по сохранённой настройке сервиса.','No applied device policies. Cache follows the service setting.'],common:['Общие правила','Common rules'],newGroup:['Новая группа','New group'],edit:[rfUi('Выбрать',"Select"),'Select'],remove:['Удалить из черновика','Remove from draft'],reset:['Вернуть к общим правилам','Reset to common rules'],unknown:['Не определён','Unidentified'],draft:['Черновик','Draft'],live:['Применено','Applied'],fresh:['Свежий L2-сосед','Fresh LAN neighbor'],offline:['Нет надёжного сопоставления','No reliable mapping'],saved:['Изменён только черновик. Для запуска нажми «Сохранить и применить».','Draft changed only. Use Save and apply to activate.'],saveFirst:['Сначала сохрани черновик для проверки его правил.','Save the draft before evaluating its rules.'],badMAC:['Укажи ненулевой unicast MAC в формате 02:11:22:33:44:55','Enter a nonzero unicast MAC, e.g. 02:11:22:33:44:55'],required:['Нужны название, корректный DNS и MAC','A name, valid DNS and MAC are required'],conflict:['MAC уже состоит в другой группе','MAC already belongs to another group']
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
            if(!u||u.transport==='native')throw Error(rfUi('DNS не найден или не поддерживает резервирование',"DNS not found or does not support fallback"));
            const allowed=state.config.upstreams.filter(x=>x.id!==id&&x.transport!=='native'&&!(x.fallbacks||[]).length);
            if(value&&!allowed.some(x=>x.id===value))throw Error(rfUi('Недопустимый резервный DNS',"Invalid fallback DNS"));
            if(value&&state.config.upstreams.some(x=>(x.fallbacks||[]).includes(id)))throw Error(rfUi('Этот DNS уже является резервным',"This DNS is already a fallback"));
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
            if(!state.editing||!listIsEditable(state.editing,state.editingList))throw Error(rfUi('Невозможно импортировать в этот список',"Cannot import into this list"));
            const names=parseDomains(String(value));
            editorDraft.domains=names.join('\n');
            syncEditorSnapshot();
            show(rfUi('Импортировано ',"Imported")+names.length+rfUi(' доменов. Нажми «Сохранить список».',"domains. Choose “Save list”."));
          },false);
          if(name==='import-error')return show(rfUi('Импорт: ',"Import:")+String(value),'bad');
          if(name==='export-error')return show(rfUi('Экспорт недоступен в этом браузере.',"Export is unavailable in this browser."),'bad');
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
      // P37: honor persisted Advanced on reload; do not land on the Basic DNS page.
      setPage(params.get('view')==='logs'?'logs':ui.modeView==='advanced'?'devices':'dns');
      load().catch(e=>{if(disposed)return;show(rfUi('Не удалось открыть DNS Router: ',"Unable to open DNS Router:")+e.message,'bad');presentation.serviceBadge=rfUi('Недоступен',"Unavailable");presentation.serviceTone='bad';});
      cleanup=()=>{
        disposed=true;nativeDNSRead?.abort();nativeDNSRead=null;apiClient.dispose();
        for(const dispose of eventCleanups)dispose();
        controller.commands={};controller.poll={};navigationHooks.navigate=()=>{};settingsHooks.change=()=>{};settingsHooks.render=()=>{};discoveryHooks.render=()=>{};discoveryHooks.refresh=()=>Promise.resolve();discoveryHooks.snapshot=()=>null;
      };

    return () => cleanup();
}
