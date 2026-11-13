// Svelte-owned V2 Sandbox input validation and read-only result projection.
// v2/bench is a candidate test only, never a production config mutation.
import {strategyTokens, strategyText, STRATEGY_SOURCES} from './NfqwsStrategyLibraryModel.js';
export const sandboxArgsText=strategyText;
const allowedPaths=new Map([
  ['/v2/target-sources','GET'],['/bench-strategies','GET'],['/v2/strategies','GET'],['/status','GET'],
  ['/v2/targets/resolve','POST'],['/v2/bench','POST'],['/v2/strategies/save','POST']
]);
export function sandboxPath(path,method='GET') {
  if(allowedPaths.get(path)!==method)throw Error('NFQWS_SANDBOX_API_NOT_ALLOWED');
  return path;
}
export function sandboxSafeTargets(result) {
  if(!result||!Array.isArray(result.targets)||result.targets.length>64)throw Error('NFQWS_SANDBOX_TARGET_RESPONSE_INVALID');
  const out=[];
  for(const item of result.targets) {
    if(typeof item!=='string'||!item.trim()||item.length>253||/[\x00-\x20\/\\]/.test(item))throw Error('NFQWS_SANDBOX_TARGET_RESPONSE_INVALID');
    if(!out.includes(item))out.push(item);
  }
  return out;
}
export function sandboxTargetBody(state,known) {
  if(state?.targetSource==='list') {
    const name=String(state?.listName||'');
    if(!Array.isArray(known)||!known.some(x=>x&&x.name===name))throw Error('NFQWS_SANDBOX_STALE_LIST');
    return{source:'list',list_name:name,limit:64};
  }
  if(state?.targetSource!=='manual')throw Error('NFQWS_SANDBOX_TARGET_SOURCE_INVALID');
  const targets=String(state?.targetsText||'').split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
  if(!targets.length||targets.length>64)throw Error('NFQWS_SANDBOX_TARGET_INPUT_INVALID');
  return{source:'manual',targets,limit:64};
}
export function sandboxBenchBody(state,{profiles=[],library=[],status}={}) {
  if(!status||typeof status.config_sha256!=='string'||!/^[a-f0-9]{64}$/i.test(status.config_sha256))throw Error('NFQWS_SANDBOX_STATUS_UNVERIFIED');
  const attempts=Number(state?.attempts);
  if(!Number.isInteger(attempts)||![1,2,3,5].includes(attempts))throw Error('NFQWS_SANDBOX_ATTEMPTS_INVALID');
  const body={attempts,expected_config_sha256:status.config_sha256,confirm:'ROUTERFORGE_V2_BENCH'};
  if(state?.candidateSource==='production') {
    const profile=profiles.find(p=>p&&p.candidate_eligible&&String(p.index)===String(state.profileIndex));
    if(!profile)throw Error('NFQWS_SANDBOX_PROFILE_STALE');
    body.profile_index=Number(profile.index);
  } else if(state?.candidateSource==='library') {
    const item=library.find(x=>x&&x.id===state.libraryId);
    if(!item||!Array.isArray(item.args)||!item.args.length)throw Error('NFQWS_SANDBOX_LIBRARY_STALE');
    body.args=item.args.slice();
  } else if(state?.candidateSource==='custom') {
    body.args=strategyTokens(state.argsText);
    if(!body.args.length)throw Error('NFQWS_SANDBOX_ARGS_EMPTY');
  } else throw Error('NFQWS_SANDBOX_SOURCE_INVALID');
  if(body.args&&body.args.length>256)throw Error('NFQWS_SANDBOX_ARGS_TOO_LARGE');
  return body;
}
export function sandboxSaveBody(state,{profiles=[],library=[]}={}) {
  let args=[],source=state?.candidateSource,name=String(state?.saveName||'').trim()||'Sandbox candidate';
  if(source==='production'){
    const p=profiles.find(x=>x&&x.candidate_eligible&&String(x.index)===String(state.profileIndex));
    if(!p||!Array.isArray(p.args))throw Error('NFQWS_SANDBOX_PROFILE_STALE');
    args=p.args.slice();source='custom';
  }else if(source==='library'){
    const p=library.find(x=>x&&x.id===state.libraryId);
    if(!p||!Array.isArray(p.args))throw Error('NFQWS_SANDBOX_LIBRARY_STALE');
    args=p.args.slice();source=p.source||'custom';name=String(state.saveName||'').trim()||p.name||name;
  }else if(source==='custom'){args=strategyTokens(state.argsText);}else throw Error('NFQWS_SANDBOX_SOURCE_INVALID');
  if(!args.length||args.length>256||new TextEncoder().encode(name).length>120||!STRATEGY_SOURCES.includes(source)||/[\x00-\x1f\x7f]/.test(name))throw Error('NFQWS_SANDBOX_SAVE_INVALID');
  return{name,source,args,confirm:'ROUTERFORGE_V2_STRATEGY_SAVE'};
}
export function sandboxResult(raw,target) {
  if(!raw||typeof raw!=='object')throw Error('NFQWS_SANDBOX_RESULT_INVALID');
  const value=raw.result||raw;
  if(!value||typeof value!=='object')throw Error('NFQWS_SANDBOX_RESULT_INVALID');
  const attempts=Array.isArray(value.attempts)?value.attempts:[];
  const packets=attempts.reduce((count,row)=>count+Math.max(0,Number(row?.outbound_queue_packets)||0)+Math.max(0,Number(row?.inbound_queue_packets)||0),0);
  const status=String(value.result_class||'—');
  const tone=['WORKING','PASS','OK'].includes(status)?'good':['PARTIAL','UNSTABLE'].includes(status)?'warn':['FAILED','ERROR','INCONCLUSIVE'].includes(status)?'bad':'info';
  const pct=value.success_rate;
  return{target:String(target||'candidate'),source:String(value.candidate_source||raw.source||''),status,tone,
    success:Math.round(Math.max(0,Math.min(1,Number(pct)||0))*100),complete:Math.round(Math.max(0,Math.min(1,Number(value.complete_rate)||0))*100),
    infrastructure:!!value.infrastructure_ok,cleanup:!!value.cleanup_proven,packets,ttfb:value.median_ttfb_ms??'—',args:sandboxArgsText(value.args||[])};
}
