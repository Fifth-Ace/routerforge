import { rfUi } from './rf-localization.js';
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
  if(!probe)return{tone:'info',label:rfUi('НЕ ЗАПУСКАЛАСЬ',"NOT RUN"),detail:rfUi('Router-side HTTPS probe ещё не выполнялся.',"An HTTPS check from the router has not been run yet."),reachable:false,contentOK:false};
  const status=Number(probe.status_code||0);
  if(!probe.reachable)return{tone:'bad',label:rfUi('ОШИБКА',"ERROR"),detail:String(probe.error||rfUi('Соединение не установлено.',"Connection could not be established.")),reachable:false,contentOK:false};
  if(status>=200&&status<400)return{tone:'good',label:`HTTP ${status}`,detail:rfUi('Сайт доступен с роутера; путь LAN-клиента не доказан.',"The site is reachable from the router, but the LAN client's path is unverified."),reachable:true,contentOK:true};
  if(status>=400&&status<600)return{tone:'warn',label:`HTTP ${status}`,detail:rfUi('Сайт ответил, но HTTP-код указывает на ограничение или ошибку.',"The site responded, but its HTTP status indicates a restriction or error."),reachable:true,contentOK:false};
  return{tone:'good',label:status?`HTTP ${status}`:rfUi('ДОСТУПЕН',"REACHABLE"),detail:rfUi('Сайт достижим с роутера.',"The site is reachable from the router."),reachable:true,contentOK:true};
}
export function processingSummary(result){
  if(!result)return{kind:'info',title:rfUi('ОЖИДАНИЕ',"WAITING"),detail:rfUi('Введите домен для диагностики.',"Enter a domain to diagnose."),next:'',actions:[],lists:processingMatches(null),probe:processingProbe(null)};
  const lists=processingMatches(result.staticPath),probe=processingProbe(result.probe),running=Boolean(result.running),mode=String(result.staticPath?.mode||'UNKNOWN'),errors=result.listErrors||[];
  let kind='warn',title=rfUi('НУЖНА ДОПОЛНИТЕЛЬНАЯ ПРОВЕРКА',"FURTHER CHECKS REQUIRED"),detail=rfUi('Статический путь не доказывает фактическую обработку пакета.',"Static route analysis does not prove the packet was actually processed."),next=rfUi('Проверьте конфигурацию и тестовую NFQUEUE.',"Check configuration and the test NFQUEUE."),actions=[{tab:'preflight',title:'Preflight'}];
  if(!running){kind='bad';title=rfUi('NFQWS2 ОСТАНОВЛЕН',"NFQWS2 STOPPED");detail=rfUi('Обход не запущен. Проверки списков и маршрута остаются справочными.',"The bypass was not tested. List and route checks are informational only.");next=rfUi('Проверьте состояние сервиса.',"Check service status.");actions=[{tab:'overview',title:rfUi('Обзор',"Overview")}]}
  else if(errors.length){kind='warn';title=rfUi('СПИСКИ ПРОЧИТАНЫ НЕ ПОЛНОСТЬЮ',"LISTS ONLY PARTIALLY READ");detail=rfUi(`Ошибки чтения: ${errors.length}. Нельзя делать однозначный вывод по отсутствующим спискам.`,`Read errors: ${errors.length}. Missing lists cannot be interpreted conclusively.`);next=rfUi('Восстановите доступность списков.',"Restore access to the lists.");actions=[{tab:'lists',title:rfUi('Списки',"Lists")}]}
  else if(lists.includes.length&&lists.excludes.length){title=rfUi('КОНФЛИКТ АКТИВНЫХ СПИСКОВ',"CONFLICTING ACTIVE LISTS");detail=rfUi('Include и exclude совпадения найдены одновременно.',"Matching include and exclude rules were found simultaneously.");next=rfUi('Проверьте конфликтующие записи.',"Check conflicting entries.");actions=[{tab:'lists',title:rfUi('Списки',"Lists")}]}
  else if(lists.excludes.length){title=rfUi('ДОМЕН ИСКЛЮЧЁН',"DOMAIN EXCLUDED");detail=rfUi('Активное исключение найдено в текущем конфиге.',"An active exclusion was found in the current configuration.");next=rfUi('Проверьте правила исключения.',"Check exclusion rules.");actions=[{tab:'lists',title:rfUi('Списки',"Lists")}]}
  else if(lists.found&&!lists.active){title=rfUi('РОЛЬ СПИСКОВ НЕ ПОДТВЕРЖДЕНА',"LIST ROLE NOT VERIFIED");detail=rfUi('Файл содержит домен, но активный конфиг не подтверждает применение.',"The file contains the domain, but active configuration does not confirm the rule is applied.");next=rfUi('Проверьте режим обработки.',"Check the processing mode.");actions=[{tab:'strategies',title:rfUi('Настройка',"Configuration")}]}
  else if(!lists.found&&['MODE_LIST','MODE_AUTO'].includes(mode)){title=rfUi('НЕТ СОВПАДЕНИЙ В СПИСКАХ',"NO LIST MATCHES");detail=rfUi(`Для ${mode} отсутствуют подтверждённые совпадения.`,`No verified matches were found for ${mode}.`);next=rfUi('Проверьте hostlist/ipset.',"Check the host list and ipsets.");actions=[{tab:'lists',title:rfUi('Списки',"Lists")}]}
  else if(probe.contentOK){kind='good';title=rfUi('САЙТ ДОСТУПЕН С РОУТЕРА',"SITE REACHABLE FROM ROUTER");detail=probe.detail;next=rfUi('При проблеме у клиентов проверьте Production Test Lab.',"If clients have issues, run the active configuration tests.");actions=[{tab:'prodlab-v2',title:'Test Lab'}]}
  else if(probe.reachable){title=rfUi('HTTPS-ПУТЬ ДОСТУПЕН',"HTTPS PATH REACHABLE");detail=probe.detail;next=rfUi('Сверьте ответ с ожидаемым.',"Compare the response with the expected result.");actions=[{tab:'prodlab-v2',title:'Test Lab'}]}
  else{kind='bad';title=rfUi('HTTP С РОУТЕРА НЕ ПРОШЁЛ',"ROUTER HTTP CHECK FAILED");detail=probe.detail;next=rfUi('Проверьте WAN и Preflight.',"Check WAN connectivity and readiness checks.");actions=[{tab:'preflight',title:'Preflight'},{tab:'keenetic',title:'Keenetic'}]}
  return{kind,title,detail,next,actions,lists,probe,mode,running};
}
export function processingLiveProof(value,running=false){
  if(running)return{kind:'info',label:'LIVE',detail:rfUi('Запущена изолированная Fast-проверка.',"An isolated quick check has started."),proven:false,cleanup:false};
  if(!value)return{kind:'info',label:rfUi('НЕ ЗАПУСКАЛСЯ',"NOT RUN"),detail:rfUi('Тестовая NFQUEUE ещё не запускалась.',"The test NFQUEUE has not run yet."),proven:false,cleanup:false};
  if(value.error)return{kind:'bad',label:rfUi('ОШИБКА',"ERROR"),detail:String(value.error),proven:false,cleanup:false};
  const clean=value.cleanup_baseline_after===true;
  const samples=[...(value?.candidates||[])].flatMap(c=>(c?.attempts||[]).map(a=>({...a,profile:c.source_profile_index})));
  const good=samples.find(a=>a.ok===true&&a.cleanup_proven===true&&Number(a.outbound_queue_packets)>0&&Number(a.inbound_queue_packets)>0);
  const packets=samples.reduce((sum,a)=>sum+Number(a.outbound_queue_packets||0)+Number(a.inbound_queue_packets||0),0);
  if(value.ok===true&&clean&&good)return{kind:'good',label:`ПРОЙДЕН · ${packets} пакетов`,detail:`Профиль ${good.profile} · тестовая NFQUEUE ${good.queue||'—'} · cleanup PASS.`,proven:true,cleanup:true};
  return{kind:'warn',label:rfUi('НЕ ДОКАЗАН',"NOT VERIFIED"),detail:String(value.recommendation_reason||rfUi('Нет подтверждения bidirectional packets и очистки.',"Bidirectional packets and cleanup have not been verified.")),proven:false,cleanup:clean};
}
export function processingStrategyPlan(inv,live){
  const profiles=(Array.isArray(inv?.profiles)?inv.profiles:[]).filter(p=>p?.candidate_eligible===true).slice(0,3);
  const found=new Map((Array.isArray(live?.candidates)?live.candidates:[]).map(c=>[Number(c.source_profile_index),c]));
  const rows=[{name:'BASELINE',args:rfUi('Контроль без desync',"Consistency checks"),result:live?.baseline||null}];
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
