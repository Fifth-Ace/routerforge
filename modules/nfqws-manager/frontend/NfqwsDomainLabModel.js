// Read-only V2 diagnostics/Production Test Lab contract. No DOM and no implicit I/O.
const routes = Object.freeze({
  '/v2/site-diagnose': ['POST'],
  '/v2/target-sources': ['GET'],
  '/v2/targets/resolve': ['POST'],
  '/v2/inspect-target': ['POST'],
  '/v2/detect': ['POST']
});
export function diagnosticRoute(path, method='GET') {
  if (!Object.hasOwn(routes, path) || !routes[path].includes(method)) throw Error('NFQWS_DIAGNOSTIC_ROUTE_DENIED');
  return path;
}
export function diagnosticTarget(value) {
  const s=String(value??'').trim().toLowerCase().replace(/\.$/,'');
  if (!s||s.length>253||!(/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/.test(s))||s.includes('..')||s.split('.').some(x=>x.length>63||x.startsWith('-')||x.endsWith('-'))) throw Error('NFQWS_DIAGNOSTIC_TARGET_INVALID');
  return s;
}
export function diagnosticBody(target){return {target:diagnosticTarget(target)};}
export function diagnosticTone(value){
  const s=String(value??'').toUpperCase();
  if (['PASS','WORKING','CLEAR','DONE','OK'].includes(s)) return 'good';
  if (['FAILED','FAIL','DNS_FAILURE','TCP_FAILURE','TLS_FAILURE','HTTP_FAILURE','ERROR','INCONCLUSIVE'].includes(s))return 'bad';
  if (['UNSTABLE','PARTIAL','HTTP_RESTRICTED','PARTIAL_16K_SUSPECTED','CHECK'].includes(s))return 'warn';
  return 'info';
}
function obj(x){return x&&typeof x==='object'&&!Array.isArray(x)?x:{};}
export function domainProjection(raw, requested){
  const d=obj(raw),inspect=obj(d.inspect),detect=obj(d.detect),context=obj(d.context),diag=obj(detect.diagnostic),guide=obj(detect.guidance);
  const target=diagnosticTarget(inspect.target||detect.target||requested);
  const summary=[['Classification',detect.classification||'—'],['Diagnostic',diag.code||'—'],['Fault domain',diag.fault_domain||'—'],['Confidence',diag.confidence||'—'],['Strategy relevant',diag.strategy_relevant?'YES':'NO'],['Production',inspect.runtime_running?'RUNNING':'STOPPED'],['Binding',inspect.binding_verdict||'—'],['IPv4',detect.destination_ipv4||'—']];
  const stages=['dns','tcp','tls','http'].map(name=>{const row=obj(obj(detect.stages)[name]);return {name,state:String(row.state||'skipped'),detail:String(row.detail||'—'),meta:[row.status_code?`HTTP ${row.status_code}`:'',Number(row.bytes)>0?`${row.bytes} B`:'',Number.isFinite(Number(row.latency_ms))?`${row.latency_ms} ms`:''].filter(Boolean).join(' · ')}});
  const lists=Array.isArray(inspect.list_matches)?inspect.list_matches:[];
  const profiles=Array.isArray(inspect.production_profile_matches)?inspect.production_profile_matches:[];
  return {target,state:String(detect.classification||'DONE').toUpperCase(),classification:String(detect.classification||''),classificationText:String(detect.classification_text||detect.classification||'готово'),summary,stages,lists,profiles,context:{title:String(context.title||'Контекст диагностики'),rows:[['Связь со стратегией',String(context.binding_detail||'Недостаточно сведений о профилях')],['Ограничение проверки',String(context.network_scope||'Сеть клиента не проверена')],...(context.tcp16_detail?[['Передача данных',String(context.tcp16_detail)]]:[])]},guidance:{title:String(guide.title||'Диагностика завершена'),summary:String(guide.summary||diag.suggested_action||''),scope:String(guide.scope||'Проверка выполняется с роутера, не с клиентского устройства.'),runtimeNotice:!inspect.runtime_running&&guide.next_action==='selector'?'NFQWS2 сейчас не запущен. Прежде чем подбирать стратегию, проверь работу сервиса.':'',nextLabel:String(guide.next_label||'Подобрать рабочую стратегию'),needsSelector:context.next_action==='selector'&&!!inspect.runtime_running}};
}
export function labBounds(rounds,concurrency){
  const r=Number(rounds),c=Number(concurrency);
  if(!Number.isInteger(r)||r<1||r>5||!Number.isInteger(c)||c<1||c>4)throw Error('NFQWS_LAB_BOUNDS_INVALID');
  return {rounds:r,concurrency:c};
}
export function labResolveBody(source,listName,text,known=[]){
  if(source==='list'){
    if(!Array.isArray(known)||!known.some(x=>x&&x.name===listName)||typeof listName!=='string')throw Error('NFQWS_LAB_STALE_LIST');
    return {source:'list',list_name:listName,limit:64};
  }
  if(source!=='manual')throw Error('NFQWS_LAB_SOURCE_INVALID');
  const targets=[...new Set(String(text??'').split(/\r?\n/).map(s=>s.trim()).filter(Boolean).map(diagnosticTarget))];
  if(!targets.length||targets.length>64)throw Error('NFQWS_LAB_TARGET_COUNT_INVALID');
  return {source:'manual',targets,limit:64};
}
export function labResolvedTargets(data){
  if(!Array.isArray(data?.targets)||data.targets.length<1||data.targets.length>64)throw Error('NFQWS_LAB_RESOLUTION_INVALID');
  return [...new Set(data.targets.map(diagnosticTarget))];
}
export function labStatistics(rows,done,total){
  let ok=0,fail=0;
  for(const row of rows||[])for(const entry of row?.attempts||[]){if(entry.ok)ok++;else fail++;}
  return {targets:rows?.length||0,ok,fail,progress:total?`${Math.round(done/total*100)}%`:'0%',ratio:(ok+fail)?ok/(ok+fail):0};
}
export function labRowView(row){
  const attempts=Array.isArray(row?.attempts)?row.attempts:[],ok=attempts.filter(x=>x.ok).length,last=attempts.at(-1),state=attempts.length&&ok===attempts.length?'WORKING':ok?'UNSTABLE':'FAILED';
  return {target:String(row?.target||''),ok,total:attempts.length,state,tone:diagnosticTone(state),last:String(last?.classification_text||last?.classification||''),binding:String(row?.inspect?.binding_verdict||'—'),lists:Array.isArray(row?.inspect?.list_matches)?row.inspect.list_matches.length:0};
}
