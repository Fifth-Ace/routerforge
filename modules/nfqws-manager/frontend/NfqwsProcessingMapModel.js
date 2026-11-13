// Pure read-only Processing Map projections. Never perform requests or alter routing.
const HOST=/^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
export function processingHostname(raw){
  let value=String(raw||'').trim().toLowerCase().replace(/\.$/,'');
  if(value.includes('://')){try{const url=new URL(value);if(url.username||url.password||!['http:','https:'].includes(url.protocol))return'';value=url.hostname.replace(/\.$/,'')}catch{return''}}
  return HOST.test(value)?value:'';
}
export function processingListFiles(rows){
  if(!Array.isArray(rows))return[];
  return rows.filter(x=>/\.(?:list|list-opkg|list-old)$/i.test(String(x?.name||''))).map(x=>String(x.name));
}
export function processingMatches(path){
  const hits=(Array.isArray(path?.hits)?path.hits:[]).filter(x=>x?.file&&x.file!=='<query>');
  return {hits,found:hits.length,includes:hits.filter(x=>x.effective==='include'),excludes:hits.filter(x=>x.effective==='exclude'),unknown:hits.filter(x=>x.effective==='neutral'),active:hits.filter(x=>x.effective==='include'||x.effective==='exclude').length};
}
export function processingProbe(probe){
  if(!probe)return{tone:'info',label:'НЕ ЗАПУСКАЛАСЬ',detail:'Router-side HTTPS probe ещё не выполнялся.',reachable:false,contentOK:false};
  const status=Number(probe.status_code||0);
  if(!probe.reachable)return{tone:'bad',label:'ОШИБКА',detail:String(probe.error||'Соединение не установлено.'),reachable:false,contentOK:false};
  if(status>=200&&status<400)return{tone:'good',label:`HTTP ${status}`,detail:'Сайт доступен с роутера; путь LAN-клиента не доказан.',reachable:true,contentOK:true};
  if(status>=400&&status<600)return{tone:'warn',label:`HTTP ${status}`,detail:'Сайт ответил, но HTTP-код указывает на ограничение или ошибку.',reachable:true,contentOK:false};
  return{tone:'good',label:status?`HTTP ${status}`:'ДОСТУПЕН',detail:'Сайт достижим с роутера.',reachable:true,contentOK:true};
}
export function processingSummary(result){
  if(!result)return{kind:'info',title:'ОЖИДАНИЕ',detail:'Введите домен для диагностики.',next:'',actions:[],lists:processingMatches(null),probe:processingProbe(null)};
  const lists=processingMatches(result.staticPath),probe=processingProbe(result.probe),running=Boolean(result.running),mode=String(result.staticPath?.mode||'UNKNOWN'),errors=result.listErrors||[];
  let kind='warn',title='НУЖНА ДОПОЛНИТЕЛЬНАЯ ПРОВЕРКА',detail='Статический путь не доказывает фактическую обработку пакета.',next='Проверьте конфигурацию и тестовую NFQUEUE.',actions=[{tab:'preflight',title:'Preflight'}];
  if(!running){kind='bad';title='NFQWS2 ОСТАНОВЛЕН';detail='Обход не запущен. Проверки списков и маршрута остаются справочными.';next='Проверьте состояние сервиса.';actions=[{tab:'overview',title:'Обзор'}]}
  else if(errors.length){kind='warn';title='СПИСКИ ПРОЧИТАНЫ НЕ ПОЛНОСТЬЮ';detail=`Ошибки чтения: ${errors.length}. Нельзя делать однозначный вывод по отсутствующим спискам.`;next='Восстановите доступность списков.';actions=[{tab:'lists',title:'Списки'}]}
  else if(lists.includes.length&&lists.excludes.length){title='КОНФЛИКТ АКТИВНЫХ СПИСКОВ';detail='Include и exclude совпадения найдены одновременно.';next='Проверьте конфликтующие записи.';actions=[{tab:'lists',title:'Списки'}]}
  else if(lists.excludes.length){title='ДОМЕН ИСКЛЮЧЁН';detail='Активное исключение найдено в текущем конфиге.';next='Проверьте правила исключения.';actions=[{tab:'lists',title:'Списки'}]}
  else if(lists.found&&!lists.active){title='РОЛЬ СПИСКОВ НЕ ПОДТВЕРЖДЕНА';detail='Файл содержит домен, но активный конфиг не подтверждает применение.';next='Проверьте режим обработки.';actions=[{tab:'strategies',title:'Настройка'}]}
  else if(!lists.found&&['MODE_LIST','MODE_AUTO'].includes(mode)){title='НЕТ СОВПАДЕНИЙ В СПИСКАХ';detail=`Для ${mode} отсутствуют подтверждённые совпадения.`;next='Проверьте hostlist/ipset.';actions=[{tab:'lists',title:'Списки'}]}
  else if(probe.contentOK){kind='good';title='САЙТ ДОСТУПЕН С РОУТЕРА';detail=probe.detail;next='При проблеме у клиентов проверьте Production Test Lab.';actions=[{tab:'prodlab-v2',title:'Test Lab'}]}
  else if(probe.reachable){title='HTTPS-ПУТЬ ДОСТУПЕН';detail=probe.detail;next='Сверьте ответ с ожидаемым.';actions=[{tab:'prodlab-v2',title:'Test Lab'}]}
  else{kind='bad';title='HTTP С РОУТЕРА НЕ ПРОШЁЛ';detail=probe.detail;next='Проверьте WAN и Preflight.';actions=[{tab:'preflight',title:'Preflight'},{tab:'keenetic',title:'Keenetic'}]}
  return{kind,title,detail,next,actions,lists,probe,mode,running};
}
export function processingLiveProof(value,running=false){
  if(running)return{kind:'info',label:'LIVE',detail:'Запущена изолированная Fast-проверка.',proven:false,cleanup:false};
  if(!value)return{kind:'info',label:'НЕ ЗАПУСКАЛСЯ',detail:'Тестовая NFQUEUE ещё не запускалась.',proven:false,cleanup:false};
  if(value.error)return{kind:'bad',label:'ОШИБКА',detail:String(value.error),proven:false,cleanup:false};
  const clean=value.cleanup_baseline_after===true;
  const samples=[...(value?.candidates||[])].flatMap(c=>(c?.attempts||[]).map(a=>({...a,profile:c.source_profile_index})));
  const good=samples.find(a=>a.ok===true&&a.cleanup_proven===true&&Number(a.outbound_queue_packets)>0&&Number(a.inbound_queue_packets)>0);
  const packets=samples.reduce((sum,a)=>sum+Number(a.outbound_queue_packets||0)+Number(a.inbound_queue_packets||0),0);
  if(value.ok===true&&clean&&good)return{kind:'good',label:`ПРОЙДЕН · ${packets} пакетов`,detail:`Профиль ${good.profile} · тестовая NFQUEUE ${good.queue||'—'} · cleanup PASS.`,proven:true,cleanup:true};
  return{kind:'warn',label:'НЕ ДОКАЗАН',detail:String(value.recommendation_reason||'Нет подтверждения bidirectional packets и очистки.'),proven:false,cleanup:clean};
}
export function processingStrategyPlan(inv,live){
  const profiles=(Array.isArray(inv?.profiles)?inv.profiles:[]).filter(p=>p?.candidate_eligible===true).slice(0,3);
  const found=new Map((Array.isArray(live?.candidates)?live.candidates:[]).map(c=>[Number(c.source_profile_index),c]));
  const rows=[{name:'BASELINE',args:'Контроль без desync',result:live?.baseline||null}];
  for(const profile of profiles)rows.push({name:`PROFILE ${Number(profile.index)}`,args:(Array.isArray(profile.args)?profile.args:[]).join(' '),result:found.get(Number(profile.index))||null});
  return rows;
}
export function processingReport(result,inventory,live){
  if(!result)throw Error('PROCESSING_NO_RESULT');
  const s=processingSummary(result);
  return {generated_at:new Date().toISOString(),scope:'processing-map-read-only',domain:result.domain,active_config_sha256:result.configSHA||'',summary:{kind:s.kind,title:s.title,detail:s.detail},static_path:{mode:result.staticPath?.mode||'UNKNOWN',verdict:result.staticPath?.verdict||'UNKNOWN',reason:result.staticPath?.reason||'',hits:s.lists.hits},list_errors:result.listErrors||[],router_probe:result.probe||null,keenetic:result.keenetic||null,strategy_plan:processingStrategyPlan(inventory,live).map(x=>({name:x.name,args:x.args})),live_nfqws:live||null,observability_boundary:'Router-side HTTPS and isolated NFQUEUE candidate do not prove that a LAN client packet crossed production q300.'};
}
export function processingFastPayload(domain,latest){
  if(processingHostname(domain)!==domain)throw Error('PROCESSING_DOMAIN_INVALID');
  const sha=String(latest?.config_sha256||'').trim().toLowerCase();
  if(latest?.running!==true||!/^[a-f0-9]{64}$/.test(sha))throw Error('PROCESSING_CONFIG_SHA_OR_RUNTIME_UNAVAILABLE');
  return{mode:'fast',server_name:domain,expected_config_sha256:sha,confirm:'ROUTERFORGE_AUTOTUNE_TLS'};
}
