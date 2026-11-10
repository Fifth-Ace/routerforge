// Single reactive authority for the DNS Runtime draft, revision and UI.
// Evaluation never fetches or mutates the router; all writes require explicit UI actions.
import { dnsGroups, dnsChoice, dnsSummary, isSimpleRule } from './DnsRuntimeP10Catalog.js';
import { entries as nativeEntries, acceptsDomains } from './DnsRuntimeNativeCatalog.js';
import { planDomainAddition } from './DnsRuntimeP88ActionLogic.js';
import { planGroupReplacement } from './DnsRuntimeP10GroupLogic.js';
import { parseDomainBatch } from './DnsRuntimeP92Logic.js';

const clone = value => JSON.parse(JSON.stringify(value));
const policyNameRE=/^(System|Policy(?:0|[1-9][0-9]{0,2}))$/;
export const draft=$state({
  config:{schema_version:1,upstreams:[],rules:[]}, revision:'', dirty:false,
  generation:0, feedback:'', loaded:false, ruleSearch:'', edit:{scope:'',item:null,version:0}
});
export const simple=$state({
  inventory:null,catalog:null,policy:'System',selected:'',domains:'',
  editingId:null,status:'',notice:'',probeNotice:'',probeEligible:false
});
export const shell=$state({mode:'simple',tab:'overview',lite:false,liteActive:false, liteKnown:false});
export const apiOperations=$state({config:false});

export function feedback(message){draft.feedback=String(message||'');}
export function readDraft(){return {revision:draft.revision,dirty:draft.dirty,generation:draft.generation,config:clone(draft.config)};}
export function markDirty(message='Изменения не сохранены. DNS-трафик не затронут.'){
  draft.dirty=true;draft.generation++;simple.editingId=null;
  feedback(message);
}
export function commitDraft(action,payload){
  if(!payload || payload.previousRevision!==draft.revision)throw Error('Черновик изменён: CAS или версия не совпадают');
  if(action==='saved'){
    if(typeof payload.revision!=='string'||!payload.revision)throw Error('Нет ревизии сохранения');
    const stale=payload.generation!==draft.generation;
    draft.revision=payload.revision;draft.dirty=stale;draft.generation++;
    return {stale};
  }
  if(payload.generation!==draft.generation)throw Error('Черновик изменён во время операции');
  if(action==='loaded'){
    if(draft.dirty!==payload.dirty||!payload.config||typeof payload.revision!=='string')throw Error('Черновик изменён во время загрузки');
    draft.config=clone(payload.config);draft.revision=payload.revision;draft.dirty=false;draft.loaded=true;draft.generation++;
    return {loaded:true};
  }
  if(action==='imported'){
    if(draft.dirty||!payload.config)throw Error('Черновик не готов к импорту');
    draft.config=clone(payload.config);markDirty();return {staged:true};
  }
  if(action==='edited'){
    if(!payload.config||!Array.isArray(payload.config.upstreams)||!Array.isArray(payload.config.rules))throw Error('Недопустимый черновик редактора');
    draft.config=clone(payload.config);markDirty();return {staged:true};
  }
  throw Error('UNKNOWN_CONFIG_COMMIT_ACTION');
}
export function editRow(scope,item){
  if(!['upstream','rule'].includes(scope)||!item?.id)throw Error('Редактор DNS: объект не найден');
  draft.edit={scope,item:clone(item),version:draft.edit.version+1};
}
export function removeRow(scope,index){
  const config=clone(draft.config);
  const items=scope==='upstream'?config.upstreams:scope==='rule'?config.rules:null;
  if(!items||!Number.isInteger(index)||index<0||index>=items.length)throw Error('Строка DNS изменилась');
  if(scope==='upstream'){
    const id=items[index].id;
    if(config.service?.default_upstream===id||config.rules.some(r=>r.upstream===id)||
       items.some(u=>(u.fallbacks||[]).includes(id))||
       (config.dns_groups||[]).some(g=>(g.servers||[]).includes(id))) {
      throw Error('Нельзя удалить используемый пул. Сначала убери правила и резервные ссылки.');
    }
  }
  items.splice(index,1);draft.config=config;markDirty();
}
export function formsSnapshot(){return {
  upstreams:(draft.config.upstreams||[]).map(u=>({id:u.id})),
  listener:draft.config.listener||{mode:'loopback',listen:'127.0.0.1:55353',allowed_clients:[]}
};}
export function nativeDNS(){return nativeEntries(simple.inventory,simple.policy);}
export function nativeChoices(){return nativeDNS().filter(x=>x.usable).map(x=>({value:x.selector,label:x.label,zone:x.zone,hint:x.hint,type:'native'}));}
export function choices(){return nativeChoices().concat((draft.config.upstreams||[]).filter(x=>['doh','dot','udp','doq'].includes(x.transport)).map(x=>({
  value:'manual:'+x.id,label:x.transport.toUpperCase()+' · '+(x.server_name||x.address)+' · DNS Router (не Keenetic)',zone:'.',type:'manual',upstreamId:x.id
})));}
export function groups(){return dnsGroups(draft.config,simple.policy,nativeDNS());}
export function summary(){return dnsSummary(draft.config,nativeDNS());}
export function policyName(name){
  if(name==='System')return 'Системная';
  return simple.catalog?.available?(simple.catalog.policies||[]).find(x=>x.name===name)?.description||name:name;
}
export function policyRoute(name){
  if(name==='System')return 'Системный DNS';
  const policy=simple.catalog?.available?(simple.catalog.policies||[]).find(x=>x.name===name):null;
  return policy?.ipv4_route_status==='default_absent'?'Нет маршрута по умолчанию':
    policy?.ipv4_route_status==='default_present'?'Маршрут найден · DNS не проверен':'Маршрут не проверен';
}
export function updateProbe(){
  const item=choices().find(x=>x.value===simple.selected);
  simple.probeEligible=!!(item&&item.zone==='.'&&item.type==='native');
  simple.probeNotice=item?.type==='manual'?'Ручной DNS находится в конфигурации DNS Router; он НЕ привязан к выбранной политике Keenetic. Для проверки используй расширенную диагностику.':
    item&&item.zone!=='.'?'DNS ограничен зоной '+item.zone+'. Проверка example.com для этой зоны недопустима.':'';
}
export function refreshChoices(){
  const available=choices();
  if(!available.some(x=>x.value===simple.selected))simple.selected=available[0]?.value||'';
  updateProbe();
}
export function selectPolicy(name){
  const instance=(simple.inventory?.instances||[]).find(x=>x.name===name&&x.valid&&policyNameRE.test(x.name));
  if(!instance)return;
  simple.policy=name;refreshChoices();
}
export function acceptScan(inventory,catalog){
  simple.inventory=inventory;simple.catalog=catalog;
  const instances=inventory?.available?(inventory.instances||[]).filter(x=>x.valid&&policyNameRE.test(x.name)):[];
  if(!instances.some(x=>x.name===simple.policy))simple.policy=instances.some(x=>x.name==='System')?'System':instances[0]?.name||'';
  refreshChoices();
  simple.status=!inventory?.available?'DNS-инвентаризация недоступна. Создание правил заблокировано.':
    !catalog?.available?'DNS найдены, но имена пользовательских политик недоступны; проверь выбранную политику.':'';
}
export function scanError(message){simple.inventory=null;simple.catalog=null;simple.status=message;refreshChoices();}
export function cancelEditing(){simple.editingId=null;simple.domains='';simple.notice='';}
export function changeDNS(value){
  if(simple.editingId)cancelEditing();
  simple.selected=value;updateProbe();
}
export function chooseDNS(id,confirmAction=()=>false){
  const g=groups().find(x=>x.id===id),selector=g?dnsChoice(g):'';
  if(!selector||!choices().some(x=>x.value===selector))return;
  if(simple.domains.trim()&&!confirmAction('Очистить введённые, но не добавленные домены?'))return;
  cancelEditing();simple.selected=selector;updateProbe();
}
export function editDNS(id,confirmAction=()=>false){
  const g=groups().find(x=>x.id===id),selector=g?dnsChoice(g):'';
  if(!selector||!choices().some(x=>x.value===selector))return;
  if(g.rules.length>200||g.rules.some(r=>!isSimpleRule(r)))return;
  if(simple.domains.trim()&&!confirmAction('Несохранённый текст в поле исчезнет.'))return;
  simple.selected=selector;updateProbe();simple.editingId=g.id;
  simple.domains=g.rules.map(r=>r.domain).join('\n');
  simple.notice='Редактируется только список DNS: '+g.info.title+'. Удаление строк изменит ЧЕРНОВИК; активный DNS отдельно не меняется.';
}
export function stageDomains(confirmAction=()=>false){
  const parsed=parseDomainBatch(simple.domains);
  if(parsed.error){simple.status=parsed.error;return false;}
  if(simple.editingId){
    const chosen=choices().find(x=>x.value===simple.selected),id=simple.editingId;
    const plan=planGroupReplacement(draft.config,id,simple.selected,chosen,parsed.domains,acceptsDomains,draft.revision);
    if(plan.changed&&!confirmAction('Обновить список только этого DNS?\nУдалить доменов: '+plan.removed+'\nДобавить доменов: '+plan.added+'\nИзменения останутся в черновике.'))return false;
    if(id!==simple.editingId||plan.upstreamId!==id||plan.revision!==draft.revision||plan.baseRules!==JSON.stringify(draft.config.rules||[]))throw Error('DNS_GROUP_CAS_STALE: обнови черновик перед повтором');
    if(plan.changed){draft.config.rules=clone(plan.rules);markDirty();simple.status='Список DNS '+id+' обновлён в черновике: '+plan.count+' доменов. Нажми «Сохранить черновик». ';}
    else simple.status='Список уже совпадает с черновиком.';
    cancelEditing();return true;
  }
  if(!parsed.domains.length){simple.status='Добавь хотя бы один домен.';return false;}
  const chosen=choices().find(x=>x.value===simple.selected);
  const plan=planDomainAddition(clone(draft.config),simple.selected,chosen,parsed.domains,acceptsDomains);
  draft.config.upstreams=clone(plan.upstreams);draft.config.rules=clone(plan.rules);
  const count=plan.count;simple.domains='';markDirty();simple.status='Добавлено '+count+' доменов в черновик. Нажми «Сохранить настройки». DNS Keenetic не изменён.';
  return true;
}

export async function loadConfig(request,signal) {
  if(typeof request!=='function')throw Error('DNS_RUNTIME_LOAD_DEPENDENCY_MISSING');
  if(signal?.aborted)return {cancelled:true};
  const before=readDraft();
  const response=await request('config',signal?{signal}:undefined);
  // Guard even when a mock or an older fetch implementation ignores abort.
  if(signal?.aborted)return {cancelled:true};
  if(!response?.config||!Array.isArray(response.config.upstreams)||!Array.isArray(response.config.rules)||
     typeof response.revision!=='string'||!response.revision)throw Error('Неверный ответ config');
  if(before.generation!==draft.generation||before.revision!==draft.revision||before.dirty!==draft.dirty)
    throw Error('Черновик изменён во время загрузки; перезапись отменена');
  commitDraft('loaded',{previousRevision:before.revision,generation:before.generation,
    dirty:before.dirty,config:response.config,revision:response.revision});
  refreshChoices();feedback('Конфигурация загружена · runtime меняется только при ручном старте/применении');
}
