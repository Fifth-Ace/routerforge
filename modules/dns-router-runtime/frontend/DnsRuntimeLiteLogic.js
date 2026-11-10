/* RouterForge R09 — standalone DNS Lite service flow.
 * No IO at module evaluation or construction. No implicit DNS activation.
 * Native service and O2 ownership remain separate safety domains.
 */
import {createConfigFlow} from './DnsRuntimeConfigLogic.js';

function snapshot(readDraft) {
  const s=readDraft();
  if(!s || typeof s.revision!=='string'||!s.revision||!Number.isSafeInteger(s.generation)||
     !s.config||!Array.isArray(s.config.upstreams))throw Error('Черновик DNS ещё не загружен');
  return s;
}
function same(readDraft,before){
  const after=snapshot(readDraft);
  if(after.revision!==before.revision||after.generation!==before.generation||after.dirty!==before.dirty)
    throw Error('Черновик изменился во время запроса. Продолжение запрещено');
  return after;
}
function action(operation,body,revision){
  return {method:'POST',headers:{'Content-Type':'application/json',
    'X-RouterForge-Action':'service-control','If-Match':'"'+revision+'"'},
    body:JSON.stringify(body||{operation})};
}
function preferences(fields,draft){
  const cache=Number(fields?.cacheMiB);
  if(!Number.isInteger(cache)||cache<0||cache>8)throw Error('Кэш: целое число 0–8 МиБ');
  const id=String(fields?.defaultUpstream||'');
  if(!id || !draft.config.upstreams.some(x=>x.id===id))
    throw Error('Выбери существующий DNS-сервер по умолчанию');
  const listen=String(fields?.listen||'').trim();
  if(!listen)throw Error('Укажи адрес DNS-сервиса');
  const allowed=String(fields?.networks||'').split(',').map(x=>x.trim()).filter(Boolean);
  const result={listen,default_upstream:id,cache_bytes:cache*1024*1024,allowed_networks:allowed};
  if(draft.config.service?.default_group && draft.config.service.default_upstream===id)
    result.default_group=draft.config.service.default_group;
  return result;
}
function formatTraces(traces){
  return (traces||[]).slice().reverse().map(x=>
    String(x.time||'')+' '+String(x.name||'')+' / '+String(x.type||'')+' → '+
    (x.cache?'КЭШ':x.used||x.assigned||'—')+' / RCODE '+String(x.rcode??'—')+
    (x.success?'':' / ОШИБКА')).join('\n')||'Запросов пока нет.';
}

export function createLiteFlow({request,readDraft,commit,confirmAction,stagePending,notify,signal}){
  for(const fn of [request,readDraft,commit,confirmAction,stagePending,notify,signal])
    if(typeof fn!=='function')throw Error('DNS_LITE_DEPENDENCY_MISSING');
  let mode=null,checking=null,busy=false;
  const saveConfig=createConfigFlow({readDraft,request,commit,confirmAction,notify});
  async function exclusive(fn){
    if(busy)throw Error('Операция DNS Lite уже выполняется');
    busy=true;
    try{return await fn();}finally{busy=false;}
  }
  async function detect(){
    if(mode!==null)return mode;
    if(checking)return checking;
    checking=(async()=>{
      const result=await request('service/status',{allow404:true});
      mode=Boolean(result && result.mode==='independent-dns');
      signal({mode});
      return mode;
    })();
    try{return await checking;}finally{checking=null;}
  }
  async function refresh(){
    if(!(await detect()))return {mode:false,active:false};
    const service=await request('service/status');
    const traces=await request('service/traces');
    const result={...service,mode:true,traces:formatTraces(traces?.traces),connection:service.connection||{state:'disconnected'}};
    signal(result);
    return result;
  }
  function editPreferences(fields){
    const before=snapshot(readDraft);
    const service=preferences(fields,before);
    if(JSON.stringify(before.config.service||{})===JSON.stringify(service))return {changed:false};
    const config=structuredClone(before.config);
    config.service=service;
    commit('edited',{previousRevision:before.revision,generation:before.generation,config});
    notify('Настройки DNS-сервиса изменены только в черновике');
    return {changed:true};
  }
  async function operate(name,fields){
    return exclusive(async()=>{
      if(!(await detect()))throw Error('Самостоятельный DNS-сервис недоступен');
      if(name==='apply'){
        // Pending domains are validated by the existing P88 draft UI, never ignored.
        stagePending();
        editPreferences(fields);
        let before=snapshot(readDraft);
        if(before.dirty){
          const saved=await saveConfig.save();
          if(saved?.stale)throw Error('Черновик изменён во время сохранения. DNS не запускаем');
        }
        before=snapshot(readDraft);
        if(before.dirty)throw Error('Черновик DNS остаётся несохранённым');
        if(before.config.upstreams.some(x=>x.transport==='native')){
          await request('service/migrate-native',action('migrate',{operation:'migrate'},before.revision));
          const data=await request('config');
          if(!data?.config||typeof data.revision!=='string'||!data.revision)
            throw Error('После миграции Native нет подтверждённой конфигурации');
          same(readDraft,before);
          commit('loaded',{previousRevision:before.revision,generation:before.generation,
            dirty:before.dirty,revision:data.revision,config:data.config});
          before=snapshot(readDraft);
        }
        const status=await request('service/status');
        same(readDraft,before);
        await request('service/control',action(status.active?'apply':'start',
          {operation:status.active?'apply':'start'},before.revision));
        notify('DNS-сервис запущен или обновлён. Подключение устройства — отдельный шаг');
        return refresh();
      }
      if(name==='stop'){
        if(!confirmAction('Остановить самостоятельный DNS и восстановить прежнее назначение устройства?'))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/control',action('stop',{operation:'stop'},before.revision));
        notify('DNS-сервис остановлен. Автозапуск выключен');
        return refresh();
      }
      if(name==='verify'){
        const before=snapshot(readDraft);
        const result=await request('service/verify',action('verify',{operation:'verify'},before.revision));
        notify('DNS E2E: '+String(result?.state||'не подтверждён')+' · проверка тестового клиента отдельно');
        return refresh();
      }
      if(name==='connect'){
        const mac=String(fields?.mac||'').trim();
        const persist=Boolean(fields?.persist);
        if(!/^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/.test(mac))throw Error('Некорректный MAC-адрес');
        if(!confirmAction('Назначить '+mac+' профиль RF_DNS_LITE?'+
          (persist?' Настройки Keenetic будут сохранены.':' Назначение действует до перезагрузки.')))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/connect',action('connect',{mac,persist},before.revision));
        notify('Профиль назначен. Проверь DNS с указанного устройства');
        return refresh();
      }
      if(name==='disconnect'){
        if(!confirmAction('Восстановить прежнее DNS-назначение тестового устройства?'))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/disconnect',action('disconnect',{operation:'disconnect'},before.revision));
        notify('Прежнее назначение DNS устройства восстановлено');
        return refresh();
      }
      throw Error('DNS_LITE_UNKNOWN_OPERATION');
    });
  }
  return {detect,refresh,editPreferences,operate,isBusy:()=>busy};
}
