// Native Svelte read-only target history with explicit, confirmed clear operation.
export function memoryReadPath(filter=''){
  if(typeof filter!=='string'||filter.length>253||/[\x00-\x1f]/.test(filter))throw Error('NFQWS_MEMORY_FILTER_INVALID');
  const target=filter.trim();
  if(target&&!/^[a-z0-9.:-]{1,253}$/i.test(target))throw Error('NFQWS_MEMORY_FILTER_INVALID');
  return '/v2/memory'+(target?'?target='+encodeURIComponent(target):'');
}
export function memoryClearBody(filter='',all=false){
  if(!all&&!String(filter||'').trim())throw Error('NFQWS_MEMORY_TARGET_REQUIRED');
  memoryReadPath(all?'':filter);
  return{target:all?'':filter.trim(),confirm:'ROUTERFORGE_V2_MEMORY_CLEAR'};
}
export function memoryProject(payload){
  if(!payload||!Array.isArray(payload.entries))throw Error('NFQWS_MEMORY_DATA_INVALID');
  const numeric=v=>Number.isFinite(Number(v))?Math.max(0,Number(v)):0;
  const p=payload.policy||{};
  return {
    policy:`policy v${p.version||'—'} · working TTL ${Math.round(numeric(p.working_max_age_seconds)/3600)}h · unstable TTL ${Math.round(numeric(p.unstable_max_age_seconds)/3600)}h · strong verified≥${numeric(p.strong_min_verified)} · trusted verified≥${numeric(p.trusted_min_verified)}`,
    rows:payload.entries.map(item=>({
      target:String(item.target||'—'),title:String(item.candidate_name||item.candidate_id||'candidate'),protocol:String(item.protocol||'https'),
      confidence:String(item.confidence||'BLOCKED'),reuse:!!item.reuse_eligible,source:String(item.candidate_source||''),
      verdict:String(item.result_class||''),verified:numeric(item.verified_count),streak:numeric(item.success_streak),
      rate:Math.round(numeric(item.success_rate)*100),age:numeric(item.age_seconds),reason:String(item.reuse_reason||''),
      args:Array.isArray(item.args)?item.args.map(String):[],fingerprint:String(item.fingerprint||'')
    }))
  };
}
