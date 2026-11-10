/* RouterForge R08: System-only O2 ownership transaction.
 * All mutations require explicit user action, exact CAS, and confirmation.
 * Module evaluation and construction perform zero IO.
 */
import {createConfigFlow} from './DnsRuntimeConfigLogic.js';
const TARGET='192.168.10.1:55358';
const ACL=['192.168.10.1','127.0.0.1'];

function validDraft(readDraft){
  const s=readDraft();
  if(!s||typeof s.revision!=='string'||!s.revision.trim()||!Number.isSafeInteger(s.generation)||
     !s.config||!Array.isArray(s.config.rules)||!Array.isArray(s.config.upstreams))
    throw Error('Черновик конфигурации DNS не готов');
  return s;
}
function validSystemConfig(draft){
  const {config}=draft;
  if(!config.rules.length)throw Error('Сначала загрузи хотя бы один домен');
  const owners=new Set();
  for(const rule of config.rules){
    const server=config.upstreams.find(x=>x.id===rule.upstream);
    if(!server)throw Error('DNS для правила '+rule.domain+' отсутствует');
    if(server.transport==='native'){
      if(!String(server.address||'').startsWith('native://System/'))
        throw Error('В конфигурации есть правило другой политики: '+rule.domain);
      if(!String(server.address||'').startsWith('native://System/./'))
        throw Error('Keenetic DNS ограничен зоной: '+rule.domain);
    }
    owners.add(server.id);
  }
  if(owners.size>128)throw Error('Слишком много DNS-серверов для P10');
  if(config.listener?.mode==='lan' && config.listener.listen!==TARGET)
    throw Error('Обнаружен другой LAN listener; автоматическая перезапись запрещена');
}
function jsonAction(action,body,revision,otherHeaders={}){
  const headers={'Content-Type':'application/json','X-RouterForge-Action':action,...otherHeaders};
  if(revision)headers['If-Match']='"'+revision+'"';
  return {method:'POST',headers,body:JSON.stringify(body)};
}
function checkedStatus(ownership,shadow){
  if(!ownership||typeof ownership.state!=='string'||!shadow||typeof shadow!=='object')
    throw Error('Статус DNS неполный; нельзя считать DNS выключенным');
  return {ownership,shadow,active:ownership.state==='verified_active'&&Boolean(shadow.lifecycle?.active)};
}
export function createP10Flow({request,readDraft,commit,getSimple,stagePending,confirmAction,report,signal}){
  for(const value of [request,readDraft,commit,getSimple,stagePending,confirmAction,report,signal])
    if(typeof value!=='function')throw Error('P10_DEPENDENCY_MISSING');
  let busy=false;
  const configFlow=createConfigFlow({readDraft,request,commit,confirmAction,notify:report});
  const emit=(text,isError=false)=>signal(text,isError);
  const requireSame=(old)=>{
    const current=validDraft(readDraft);
    if(current.revision!==old.revision||current.generation!==old.generation||current.dirty!==old.dirty)
      throw Error('Черновик изменился во время операции; продолжение отменено');
    return current;
  };
  async function status(){
    const [owner,shadow]=await Promise.all([
      request('keenetic/managed-ownership/status'),request('shadow/status')
    ]);
    return checkedStatus(owner,shadow);
  }
  async function serialized(fn){
    if(busy)throw Error('Операция DNS уже выполняется');
    busy=true;
    try{return await fn();}finally{busy=false;}
  }
  async function control(action,revision){
    return request('shadow/control',jsonAction('shadow-control',{operation:action},revision));
  }
  async function activate(){
    return serialized(async()=>{
      let startedHere=false,appliedHere=false,failure=null;
      try{
        const initial=getSimple();
        if(initial?.policy!=='System')throw Error('P10 разрешён только для System; PolicyN не подтверждены');
        // A pending visible list must become a draft, never silently bypass validation.
        if(initial.pending){
          stagePending();
          if(getSimple()?.pending)throw Error('Не удалось добавить текущие домены в черновик');
        }
        let draft=validDraft(readDraft);
        validSystemConfig(draft);
        let actual=await status();
        if(actual.ownership.state!=='disabled')
          throw Error('Обнаружена регистрация DNS ('+actual.ownership.state+'); повтор запрещён');
        // This confirmation MUST precede any save, listener or shadow mutation.
        if(!confirmAction('DSTest / System: сохранить listener '+TARGET+', запустить DNS Router и подготовить изменение DNS-зон Keenetic?'))
          return {cancelled:true};
        requireSame(draft);
        // Preserve the previous explicit P10 LAN-listener behavior, now CAS-bound.
        const next=structuredClone(draft.config);
        next.listener={mode:'lan',listen:TARGET,allowed_clients:[...ACL]};
        commit('edited',{previousRevision:draft.revision,generation:draft.generation,config:next});
        emit('Сохраняю DNS и доменные правила…');
        await configFlow.save();
        draft=validDraft(readDraft);
        if(draft.dirty)throw Error('Черновик изменён во время сохранения. DNS не применяется');
        validSystemConfig(draft);
        const revision=draft.revision;
        const live=await request('shadow/status');
        requireSame(draft);
        if(live.lifecycle?.active){
          if(live.lifecycle.applied_revision!==revision)await control('apply',revision);
        }else {await control('start',revision);startedHere=true;}
        emit('DNS Router запущен. Проверяю план зон Keenetic…');
        const plan=await request('keenetic/durable/prepare',
          jsonAction('durable-prepare',{operation:'prepare'},revision));
        if(!plan?.ready||!plan.token||!Array.isArray(plan.zones)||!plan.zones.length)
          throw Error('Нет подтверждённого безопасного плана: '+(plan?.conflicts||[]).join('; '));
        requireSame(draft);
        const summary=String(getSimple()?.summary||'');
        if(!confirmAction('Подключить '+plan.zones.length+' System-зон Keenetic? Изменится таблица DNS до отключения.\n\n'+summary))
          throw Error('Подключение DNS-зон отменено пользователем');
        // Token is used once. No automatic retries of uncertain applied state.
        const token=plan.token;
        const applied=await request('keenetic/durable/apply',
          jsonAction('durable-apply',{token},revision));
        if(applied?.state!=='active'||!applied.durable_change)
          throw Error('O2 не подтвердил изменение DNS');
        appliedHere=true;
        actual=await status();
        if(!actual.active)throw Error('Активное состояние DNS не подтверждено');
        emit('DNS ACTIVE · '+(actual.ownership.owned_zones||[]).length+' зон');
        return {active:true,planZones:plan.zones.length};
      }catch(error){
        failure=String(error?.message||error);
        // Stop only the shadow we started and only when O2 confirms NO ownership.
        if(startedHere&&!appliedHere){
          try{
            const owner=await request('keenetic/managed-ownership/status');
            if(owner?.state==='disabled')await control('stop',validDraft(readDraft).revision);
          }catch(_) { failure+=' · остановка shadow не подтверждена'; }
        }
        emit('DNS НЕ ПРИМЕНЁН / требуется проверить состояние: '+failure,true);
        throw Error(failure);
      }
    });
  }
  async function disable(){
    return serialized(async()=>{
      if(!confirmAction('Отключить DNS Router и восстановить исходные DNS-зоны Keenetic?'))
        return {cancelled:true};
      const draft=validDraft(readDraft);
      if(draft.dirty)throw Error('Сначала сохрани или отмени изменения черновика');
      const owner=await request('keenetic/managed-ownership/status');
      if(owner?.state!=='verified_active'||!owner.ledger_hash)
        throw Error('Нет точного подтверждённого владения DNS; откат заблокирован');
      requireSame(draft);
      const result=await request('keenetic/durable/disable',jsonAction('durable-disable',
        {operation:'disable'},draft.revision,{'X-RouterForge-Ledger-Match':owner.ledger_hash}));
      if(result?.state!=='disabled'||!result.baseline_restored)
        throw Error('O2 не подтвердил восстановление исходного DNS; shadow не останавливаем');
      // Only after authoritative restoration.
      await control('stop',draft.revision);
      emit('DNS отключён. Исходные зоны Keenetic восстановлены.');
      return {disabled:true};
    });
  }
  return {status,activate,disable,isBusy:()=>busy};
}
