import { rfUi } from './rf-localization.js';
/* RouterForge R09 — standalone DNS Lite service flow.
 * No IO at module evaluation or construction. No implicit DNS activation.
 * Native service and O2 ownership remain separate safety domains.
 */
import {createConfigFlow} from './DnsRuntimeConfigLogic.js';

function snapshot(readDraft) {
  const s=readDraft();
  if(!s || typeof s.revision!=='string'||!s.revision||!Number.isSafeInteger(s.generation)||
     !s.config||!Array.isArray(s.config.upstreams))throw Error(rfUi('Черновик DNS ещё не загружен',"DNS draft has not been loaded yet"));
  return s;
}
function same(readDraft,before){
  const after=snapshot(readDraft);
  if(after.revision!==before.revision||after.generation!==before.generation||after.dirty!==before.dirty)
    throw Error(rfUi('Черновик изменился во время запроса. Продолжение запрещено',"Draft changed during the request. Continuing is blocked"));
  return after;
}
function action(operation,body,revision){
  return {method:'POST',headers:{'Content-Type':'application/json',
    'X-RouterForge-Action':'service-control','If-Match':'"'+revision+'"'},
    body:JSON.stringify(body||{operation})};
}
function preferences(fields,draft){
  const cache=Number(fields?.cacheMiB);
  if(!Number.isInteger(cache)||cache<0||cache>8)throw Error(rfUi('Кэш: целое число 0–8 МиБ',"Cache: whole number from 0 to 8 MiB"));
  const id=String(fields?.defaultUpstream||'');
  if(!id || !draft.config.upstreams.some(x=>x.id===id))
    throw Error(rfUi('Выбери существующий DNS-сервер по умолчанию',"Select an existing default DNS server"));
  const listen=String(fields?.listen||'').trim();
  if(!listen)throw Error(rfUi('Укажи адрес DNS-сервиса',"Enter DNS service address"));
  const allowed=String(fields?.networks||'').split(',').map(x=>x.trim()).filter(Boolean);
  const result={listen,default_upstream:id,cache_bytes:cache*1024*1024,allowed_networks:allowed};
  if(draft.config.service?.default_group && draft.config.service.default_upstream===id)
    result.default_group=draft.config.service.default_group;
  return result;
}
function formatTraces(traces){
  return (traces||[]).slice().reverse().map(x=>
    String(x.time||'')+' '+String(x.name||'')+' / '+String(x.type||'')+' → '+
    (x.cache?rfUi('КЭШ',"CACHE"):x.used||x.assigned||'—')+' / RCODE '+String(x.rcode??'—')+
    (x.success?'':rfUi(' / ОШИБКА',"/ ERROR"))).join('\n')||rfUi('Запросов пока нет.',"No requests yet.");
}

export function createLiteFlow({request,readDraft,commit,confirmAction,stagePending,notify,signal}){
  for(const fn of [request,readDraft,commit,confirmAction,stagePending,notify,signal])
    if(typeof fn!=='function')throw Error('DNS_LITE_DEPENDENCY_MISSING');
  let mode=null,checking=null,busy=false;
  const saveConfig=createConfigFlow({readDraft,request,commit,confirmAction,notify});
  async function exclusive(fn){
    if(busy)throw Error(rfUi('Операция DNS Lite уже выполняется',"A DNS Lite operation is already running"));
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
    notify(rfUi('Настройки DNS-сервиса изменены только в черновике',"DNS service settings changed only in draft"));
    return {changed:true};
  }
  async function operate(name,fields){
    return exclusive(async()=>{
      if(!(await detect()))throw Error(rfUi('Самостоятельный DNS-сервис недоступен',"Standalone DNS service unavailable"));
      if(name==='apply'){
        // Pending domains are validated by the existing P88 draft UI, never ignored.
        stagePending();
        editPreferences(fields);
        let before=snapshot(readDraft);
        if(before.dirty){
          const saved=await saveConfig.save();
          if(saved?.stale)throw Error(rfUi('Черновик изменён во время сохранения. DNS не запускаем',"Draft changed while saving. DNS will not start"));
        }
        before=snapshot(readDraft);
        if(before.dirty)throw Error(rfUi('Черновик DNS остаётся несохранённым',"The DNS draft is still unsaved"));
        if(before.config.upstreams.some(x=>x.transport==='native')){
          await request('service/migrate-native',action('migrate',{operation:'migrate'},before.revision));
          const data=await request('config');
          if(!data?.config||typeof data.revision!=='string'||!data.revision)
            throw Error(rfUi('После миграции Native нет подтверждённой конфигурации',"No verified configuration after native DNS migration"));
          same(readDraft,before);
          commit('loaded',{previousRevision:before.revision,generation:before.generation,
            dirty:before.dirty,revision:data.revision,config:data.config});
          before=snapshot(readDraft);
        }
        const status=await request('service/status');
        same(readDraft,before);
        await request('service/control',action(status.active?'apply':'start',
          {operation:status.active?'apply':'start'},before.revision));
        notify(rfUi('DNS-сервис запущен или обновлён. Подключение устройства — отдельный шаг',"DNS service started or updated. Device connection is a separate step"));
        return refresh();
      }
      if(name==='stop'){
        if(!confirmAction(rfUi('Остановить самостоятельный DNS и восстановить прежнее назначение устройства?',"Stop standalone DNS and restore the device's previous assignment?")))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/control',action('stop',{operation:'stop'},before.revision));
        notify(rfUi('DNS-сервис остановлен. Автозапуск выключен',"DNS service stopped. Autostart disabled"));
        return refresh();
      }
      if(name==='verify'){
        const before=snapshot(readDraft);
        const result=await request('service/verify',action('verify',{operation:'verify'},before.revision));
        notify('DNS E2E: '+String(result?.state||rfUi('не подтверждён',"not verified"))+rfUi(' · проверка тестового клиента отдельно',"· test client verification is separate"));
        return refresh();
      }
      if(name==='connect'){
        const mac=String(fields?.mac||'').trim();
        const persist=Boolean(fields?.persist);
        if(!/^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/.test(mac))throw Error(rfUi('Некорректный MAC-адрес',"Invalid MAC address"));
        if(!confirmAction(rfUi('Назначить ',"Assign")+mac+rfUi(' профиль RF_DNS_LITE?',"RF_DNS_LITE profile?")+
          (persist?rfUi(' Настройки Keenetic будут сохранены.',"Keenetic settings will be saved."):rfUi(' Назначение действует до перезагрузки.',"Assignment lasts until reboot."))))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/connect',action('connect',{mac,persist},before.revision));
        notify(rfUi('Профиль назначен. Проверь DNS с указанного устройства',"Profile assigned. Verify DNS from the specified device"));
        return refresh();
      }
      if(name==='disconnect'){
        if(!confirmAction(rfUi('Восстановить прежнее DNS-назначение тестового устройства?',"Restore the test device's previous DNS assignment?")))
          return {cancelled:true};
        const before=snapshot(readDraft);
        await request('service/disconnect',action('disconnect',{operation:'disconnect'},before.revision));
        notify(rfUi('Прежнее назначение DNS устройства восстановлено',"Device's previous DNS assignment restored"));
        return refresh();
      }
      throw Error('DNS_LITE_UNKNOWN_OPERATION');
    });
  }
  return {detect,refresh,editPreferences,operate,isBusy:()=>busy};
}
