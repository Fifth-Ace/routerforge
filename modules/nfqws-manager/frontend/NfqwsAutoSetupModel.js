// Auto Setup remains non-mutating until the user explicitly confirms Selector Apply.
const endpoints={'/status':'GET','/v2/target-sources':'GET','/v2/observed-targets':'GET','/v2/targets/resolve':'POST','/v2/inspect-target':'POST','/v2/detect':'POST','/v2/tcp16-probe':'POST','/bench-capabilities':'GET'};
export function autoSetupPath(path,method='GET'){
  if(endpoints[path]!==method)throw Error('NFQWS_AUTO_SETUP_PATH_FORBIDDEN');
  return path;
}
export function autoSetupTarget({source='manual',target='',observed='',listTarget=''}={}){
  const value=String(source==='manual'?target:source==='observed'?observed:source==='list'?listTarget:'').trim();
  if(!value||value.length>253||/[\s\x00-\x1f]/.test(value))throw Error('NFQWS_AUTO_SETUP_TARGET_INVALID');
  return value;
}
export function autoSetupObserved(rows){
  if(!Array.isArray(rows))throw Error('NFQWS_AUTO_SETUP_OBSERVED_INVALID');
  return rows.map(x=>({...x,name:String(x?.target||x?.destination||'').trim()})).filter(x=>x.name&&!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(x.name));
}
export function autoSetupDiagnosisStop(detect){
  const diag=detect?.diagnostic||{},code=String(diag.code||'').toUpperCase(),fault=String(diag.fault_domain||'').toLowerCase();
  if(String(detect?.classification||'').toLowerCase()==='clear'||code==='CLEAR_END_TO_END')return['TARGET_ALREADY_HEALTHY','Baseline уже здоров; strategy mutation не нужна.','good'];
  if(code.startsWith('DNS_')||fault==='dns')return['DNS_FAILURE',diag.suggested_action||'DNS stage не прошёл.','bad'];
  if(fault==='routing'||code==='ROUTING_FAILURE')return['ROUTING_FAILURE',diag.suggested_action||'Проблема маршрута не должна маскироваться стратегией NFQWS.','bad'];
  return null;
}
export function autoSetupStageSummary(detect){
  return ['dns','tcp','tls','http'].map(k=>{const state=String(detect?.stages?.[k]?.state||'skipped').toUpperCase();return`${k.toUpperCase()} ${['PASS','OK','CLEAR','WORKING'].includes(state)?'✓':state==='SKIPPED'?'—':'✕'}`;}).join(' · ');
}
export function autoSetupErrorCode(message){
  const m=String(message||'').toLowerCase();
  if(m.includes('cleanup'))return'CLEANUP_FAILURE';
  if(m.includes('config changed')||m.includes('active config changed')||m.includes('stale'))return'CONFIG_CHANGED_WHILE_TESTING';
  if(m.includes('rollback')||m.includes('rolled back'))return'APPLY_ROLLBACK';
  if(m.includes('dependency')||m.includes('capability')||m.includes('not installed')||m.includes('missing'))return'DEPENDENCY_MISSING';
  if(m.includes('preview')&&(m.includes('expired')||m.includes('mismatch')))return'PREVIEW_STALE';
  return'WORKFLOW_ERROR';
}
export function autoSetupLabel(code){
  const labels={IDLE:'Готово к запуску',READY:'Цель выбрана',INSPECTING:'Проверяю сайт',TARGET_ALREADY_HEALTHY:'Сайт уже работает',DNS_FAILURE:'Проблема с DNS',ROUTING_FAILURE:'Проблема с маршрутом',LIVE_VERIFY:'Подбираю стратегию',DEPENDENCY_MISSING:'Автоподбор не запущен',NO_CANDIDATES_COMPILE:'Нет кандидатов для проверки',NO_CANDIDATE_WORKS:'Рабочая стратегия не найдена',CLEANUP_FAILURE:'Безопасная проверка остановлена',WINNER_VERIFIED:'Рабочая стратегия найдена',PREVIEW_READY:'Изменения готовы к просмотру',APPLY_SUCCESS:'Изменения применены',APPLY_ROLLBACK:'Изменения отменены',NO_CHANGES:'Изменения не требуются'};
  return labels[String(code||'').toUpperCase()]||String(code||'Состояние').replaceAll('_',' ');
}
export function autoSetupBlockerText(value){
  const s=String(value||''),l=s.toLowerCase();
  if(l.includes('active management ssh session'))return'Активна SSH-сессия управления; live-тесты специально заблокированы.';
  if(l.includes('candidate nfqws2 executable'))return'Не удалось подтвердить исполняемый файл nfqws2 для тестового процесса.';
  if(l.includes('firewall nfqueue inventory'))return'Не удалось безопасно прочитать NFQUEUE-правила firewall.';
  if(l.includes('kernel nfqueue binding inventory'))return'Не удалось подтвердить состояние NFQUEUE в ядре.';
  if(l.includes('no free queue'))return'Нет свободной тестовой NFQUEUE в зарезервированном диапазоне.';
  if(l.includes('cleanup baseline'))return'Не доказано, что тестовый NFQUEUE-диапазон чист до запуска.';
  if(l.includes('management ssh session inventory'))return'Не удалось безопасно определить активные SSH-сессии управления.';
  return s;
}
export function autoSetupTCP16Body(configSHA){
  if(!/^[a-f\d]{64}$/i.test(String(configSHA||'')))throw Error('NFQWS_AUTO_SETUP_SHA_REQUIRED');
  return{scan_sni:true,expected_config_sha256:configSHA,confirm:'ROUTERFORGE_TCP16_PROBE_V2'};
}
