// Native Strategy Library pure policy. Backend owns validation and persistence.
// This list mirrors v2StrategySource() in nfqws-manager-runtime/strategy_library_v2.go.
export const STRATEGY_SOURCES=Object.freeze(['catalog','custom','z2k','omn1z','import','builtin','memory','community','synthesized','mutated','curated']);
export function strategyTokens(input){
  const raw=String(input??'').match(/(?:[^\s"'\\]+|"(?:\\.|[^"])*"|'(?:\\.|[^'])*')+/g)||[];
  return raw.map(x=>{
    if(x.length>=2&&((x[0]==='"'&&x[x.length-1]==='"')||(x[0]==="'"&&x[x.length-1]==="'")))return x.slice(1,-1).replace(/\\(["'\\])/g,'$1');
    return x;
  }).filter(Boolean);
}
// Reversible editor projection: spaces/quotes in a backend argv element must
// not split into new elements when an existing strategy is saved again.
export const strategyText=args=>(Array.isArray(args)?args:[]).map(x=>{const value=String(x);return /[\s\"'\\]/.test(value)?JSON.stringify(value):value;}).join(' ');
export function verifiedStrategyInventory(snapshot){if(!Array.isArray(snapshot))throw Error('NFQWS_STRATEGY_INVENTORY_INVALID');return snapshot.filter(x=>x&&typeof x.id==='string'&&x.id&&typeof x.name==='string'&&Array.isArray(x.args)).map(x=>({id:x.id,name:x.name,source:String(x.source||'custom'),fingerprint:String(x.fingerprint||''),args:x.args.slice()}));}
// The backend returns {ok,version,count,max,strategies}. Reject partial,
// corrupt or stale responses rather than silently dropping invalid rows.
export function verifiedStrategyLibraryResponse(response){
  if(!response||response.ok!==true||response.version!==1||response.max!==64||
     !Array.isArray(response.strategies)||!Number.isInteger(response.count)||
     response.count!==response.strategies.length||response.count>64){
    throw Error('NFQWS_STRATEGY_LIBRARY_RESPONSE_INVALID');
  }
  const ids=new Set();
  for(const row of response.strategies){
    if(!row||!strategySafeID(row.id)||ids.has(row.id)||
       typeof row.name!=='string'||!row.name.trim()||row.name.length>120||/[\x00-\x1f\x7f]/.test(row.name)||
       !STRATEGY_SOURCES.includes(row.source)||
       !Array.isArray(row.args)||row.args.length===0||row.args.length>256||
       !row.args.every(value=>typeof value==='string')||
       typeof row.fingerprint!=='string'||!/^[a-f0-9]{64}$/.test(row.fingerprint)){
      throw Error('NFQWS_STRATEGY_LIBRARY_ENTRY_INVALID');
    }
    ids.add(row.id);
  }
  return verifiedStrategyInventory(response.strategies);
}
export function strategySaveBody(form,known=[]){
  const name=String(form?.name||'').trim(),source=String(form?.source||''),args=strategyTokens(form?.args||'');
  if(!name||[...name].length>120||/[\x00-\x1f\x7f]/.test(name)||!STRATEGY_SOURCES.includes(source)||!args.length||args.length>256)throw Error('NFQWS_STRATEGY_INVALID_DRAFT');
  const body={name,source,args,confirm:'ROUTERFORGE_V2_STRATEGY_SAVE'};
  if(form?.id){if(!strategySafeID(form.id)||!verifiedStrategyInventory(known).some(x=>x.id===form.id))throw Error('NFQWS_STRATEGY_STALE_SELECTION');body.id=form.id;}
  return body;
}
export function strategySafeID(id){return typeof id==='string'&&id.length>0&&id.length<=64&&!id.startsWith('.')&&/^[A-Za-z0-9_.-]+$/.test(id);}
export function strategyDeleteBody(id,known){if(!strategySafeID(id)||!verifiedStrategyInventory(known).some(x=>x.id===id))throw Error('NFQWS_STRATEGY_STALE_DELETE');return{id,confirm:'ROUTERFORGE_V2_STRATEGY_DELETE'};}
export function strategyRequestPath(path,method='GET'){
  if(method==='GET'&&path==='/v2/strategies')return path;
  if(method==='POST'&&['/v2/strategies/save','/v2/strategies/delete'].includes(path))return path;
  throw Error('NFQWS_STRATEGY_UNTRUSTED_API');
}
