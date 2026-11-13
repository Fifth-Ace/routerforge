// Pure Selector V2 UI model. The backend remains the authority for capability,
// session, active SHA, profile binding and atomic apply/rollback.
const shaPattern=/^[a-f\d]{64}$/i;
const methods={
  '/status':'GET', '/v2/strategies':'GET', '/v2/selector':'POST',
  '/v2/selector-bind':'POST','/v2/inspect-target':'POST',
  '/bench-autotune/apply-preview':'POST','/bench-autotune/apply':'POST'
};
export function selectorPath(path,method='GET'){
  if(typeof path!=='string'||methods[path]!==method)throw Error('NFQWS_SELECTOR_ENDPOINT_FORBIDDEN');
  return path;
}
export function selectorProgressPath(session){
  if(!/^ui-[0-9a-f]{16}$/.test(String(session)))throw Error('NFQWS_SELECTOR_SESSION_INVALID');
  return '/v2/selector-progress?session='+encodeURIComponent(session);
}
export function selectorArgTokens(value){
  const input=String(value||'');const tokens=[];let token='',quote='',started=false;
  for(let i=0;i<input.length;i++){
    const ch=input[i];
    if(ch==='\\'&&i+1<input.length&&[...new Set(['"',"'",'\\',' '])].includes(input[i+1])){
      token+=input[++i];started=true;continue;
    }
    if(quote){if(ch===quote){quote='';continue;}token+=ch;started=true;continue;}
    if(ch==='"'||ch==="'"){quote=ch;started=true;continue;}
    if(/\s/.test(ch)){if(started){tokens.push(token);token='';started=false;}continue;}
    token+=ch;started=true;
  }
  if(quote)throw Error('NFQWS_SELECTOR_ARGS_UNCLOSED_QUOTE');
  if(started)tokens.push(token);
  return tokens.filter(Boolean);
}
export function selectorRunBody(view,library=[],configSHA,session){
  if(!shaPattern.test(String(configSHA||'')))throw Error('NFQWS_SELECTOR_CONFIG_SHA_REQUIRED');
  if(!/^ui-[0-9a-f]{16}$/.test(String(session||'')))throw Error('NFQWS_SELECTOR_SESSION_INVALID');
  const target=String(view?.target||'').trim();
  if(!target||target.length>253||/[\s\x00-\x1f]/.test(target))throw Error('NFQWS_SELECTOR_TARGET_INVALID');
  const mode=String(view?.mode||'normal');
  if(!['fast','normal','thorough'].includes(mode))throw Error('NFQWS_SELECTOR_MODE_INVALID');
  const concurrency=Number(view?.concurrency||2);
  if(!Number.isInteger(concurrency)||concurrency<1||concurrency>4)throw Error('NFQWS_SELECTOR_CONCURRENCY_INVALID');
  const asText=String(view?.asn||'').trim(),asn=asText?Number(asText):0;
  if(asText&&(!/^\d{1,10}$/.test(asText)||!Number.isInteger(asn)||asn<1||asn>4294967295))throw Error('NFQWS_SELECTOR_ASN_INVALID');
  const production=!!view.production,autoPool=!!view.autoPool,community=!!view.community;
  if(community&&!autoPool)throw Error('NFQWS_SELECTOR_COMMUNITY_NEEDS_POOL');
  const candidates=[];
  if(view.libraryEnabled){
    if(!Array.isArray(library))throw Error('NFQWS_SELECTOR_LIBRARY_INVALID');
    for(const c of library){
      if(!c||typeof c.id!=='string'||!Array.isArray(c.args)||!c.args.every(x=>typeof x==='string'))throw Error('NFQWS_SELECTOR_LIBRARY_INVALID');
      candidates.push({id:c.id,name:c.name,source:c.source,args:c.args.slice()});
    }
  }
  const extra=selectorArgTokens(view.extraArgs);
  if(extra.length)candidates.push({name:String(view.extraName||'').trim()||'Manual candidate',source:'custom',args:extra});
  if(!production&&!autoPool&&!candidates.length)throw Error('NFQWS_SELECTOR_NO_CANDIDATES');
  return {mode,server_name:target,expected_config_sha256:configSHA,concurrency,session_id:session,
    include_production:production,auto_pool:autoPool,candidates,
    community_enabled:community,community_asn:asn,property_vector:null,confirm:'ROUTERFORGE_V2_SELECTOR'};
}
export function selectorProgress(p,mode='normal'){
  const completed=Math.max(0,Number(p?.completed||0)),total=Math.max(0,Number(p?.total||0));
  if(mode==='normal'&&total>0){
    const batch=completed>0?Math.floor((completed-1)/20):0,start=batch*20,size=Math.max(1,Math.min(20,total-start)),done=Math.max(0,Math.min(size,completed-start));
    return{label:`${done} / ${size} · всего ${completed} / ${total}`,percent:Math.round(done/size*100)};
  }
  return{label:`${completed} / ${total}`,percent:total?Math.min(100,Math.round(completed/total*100)):0};
}
export function selectorTone(value){
  const v=String(value||'').toUpperCase();
  if(['PASS','WORKING','DONE','OK'].includes(v))return'good';
  if(['FAILED','FAIL','ERROR','INCONCLUSIVE'].includes(v))return'bad';
  if(['PARTIAL','UNSTABLE'].includes(v))return'warn';
  return'info';
}
export function selectorResultLabel(value){
  const raw=String(value||'—'),map={WORKING:'Работает',FAILED:'Не работает',FAIL:'Не работает',INCONCLUSIVE:'Не удалось проверить',PARTIAL:'Частично',PASS:'Готово',DONE:'Готово',OK:'Готово'};
  return map[raw.toUpperCase()]||raw;
}
export function selectorSource(value){
  const raw=String(value||'').toLowerCase(),map={production:'Текущая конфигурация',curated:'RouterForge',other:'Встроенный пул',custom:'Пользовательская',library:'Библиотека',community:'Сообщество',baseline:'Без обхода'};
  return map[raw]||String(value||'');
}
export function selectorReason(value){
  const map={
    'baseline completed reliably; bypass strategy is not required for this target':'Сайт работает без обхода — дополнительная стратегия не требуется.',
    'candidate improved verified HTTP reachability over baseline':'Найдена рабочая стратегия, которая открывает сайт лучше исходного соединения.',
    'no candidate proved a working HTTP response through isolated NFQUEUE':'Среди проверенных стратегий рабочая не найдена.',
    'best candidate did not improve reliability/completeness over baseline':'Проверенные стратегии не дали результата лучше исходного соединения.',
    'no selector recommendation is available':'Рекомендация пока недоступна.',
    'verified candidate matches multiple production profiles; explicit slot selection is required':'Стратегия найдена, но нужно выбрать профиль nfqws2, к которому её применить.',
    'verified candidate has no unique production profile binding for this target':'Стратегия найдена, но для этого домена не определён единственный профиль nfqws2.'
  };
  return map[String(value||'')]||String(value||'');
}
export function selectorWorking(result){
  return (Array.isArray(result?.candidates)?result.candidates:[]).filter(c=>c?.result_class==='WORKING'&&Number(c.successes||0)>=1&&c.cleanup_proven===true&&c.infrastructure_ok===true);
}
export function selectorProfileQuality(profile){
  const reason=String(profile?.match_reason||'').toLowerCase();
  if(reason.includes('hostlist-domains='))return{rank:400,label:'ТОЧНЫЙ ДОМЕН'};
  if(/(^|;\s*)include-auto\s/.test(reason))return{rank:320,label:'HOSTLIST AUTO'};
  if(/(^|;\s*)include\s/.test(reason))return{rank:300,label:'HOSTLIST'};
  if(/(^|;\s*)ipset\s/.test(reason))return{rank:200,label:'IPSET'};
  if(reason.includes('target is not excluded'))return{rank:50,label:'ОБЩИЙ MATCH'};
  if(reason.includes('profile has no target selector'))return{rank:10,label:'БЕЗ SELECTOR'};
  return{rank:100,label:'MATCH'};
}
export function selectorPreviewBody(result,configSHA){
  if(!result?.apply_gate_eligible||!result.apply_gate_token)throw Error('NFQWS_SELECTOR_GATE_NOT_ELIGIBLE');
  if(!shaPattern.test(String(configSHA||'')))throw Error('NFQWS_SELECTOR_CONFIG_SHA_REQUIRED');
  return{token:result.apply_gate_token,expected_config_sha256:configSHA,confirm:'ROUTERFORGE_AUTOTUNE_PREVIEW'};
}
export function selectorBindBody(result,candidateID,configSHA){
  if(!result?.session_id||result.apply_gate_eligible||!selectorWorking(result).some(c=>c.candidate_id===candidateID))throw Error('NFQWS_SELECTOR_BIND_INVALID');
  if(!shaPattern.test(String(configSHA||'')))throw Error('NFQWS_SELECTOR_CONFIG_SHA_REQUIRED');
  return{session_id:result.session_id,expected_config_sha256:configSHA,candidate_id:candidateID,confirm:'ROUTERFORGE_V2_SELECTOR_BIND'};
}
export function selectorApplyBody(result,preview,latest){
  if(!result?.apply_gate_token||!preview?.ok||!preview?.preview_only||!preview?.changed)throw Error('NFQWS_SELECTOR_PREVIEW_REQUIRED');
  if(!shaPattern.test(String(preview.active_config_sha256||''))||!shaPattern.test(String(preview.candidate_config_sha256||'')))throw Error('NFQWS_SELECTOR_PREVIEW_SHA_INVALID');
  if(String(latest?.config_sha256||'').toLowerCase()!==String(preview.active_config_sha256).toLowerCase())throw Error('NFQWS_SELECTOR_STALE_CONFIG');
  return{receipt_token:result.apply_gate_token,expected_active_sha256:preview.active_config_sha256,expected_candidate_sha256:preview.candidate_config_sha256,confirm:'ROUTERFORGE_AUTOTUNE_APPLY'};
}
export function selectorCandidateMetrics(c){
  const attempts=Array.isArray(c?.attempts)?c.attempts:[];
  return{success:Math.round(Number(c?.success_rate||0)*100),complete:Math.round(Number(c?.complete_rate||0)*100),ttfb:Number(c?.median_ttfb_ms||0),bps:Number(c?.median_throughput_bps||0),error:attempts.map(a=>String(a?.error||'').trim()).find(Boolean)||''};
}
