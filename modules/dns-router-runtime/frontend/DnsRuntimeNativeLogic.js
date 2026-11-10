/* R07: Keenetic Native inventory and explicit ONE-query probe.
 * GET only; no automatic HTTP requests, no Keenetic configuration commands, no DNS runtime mutation.
 * DNS draft mutations remain under R05 createDraftEditor + CAS.
 */
const VALID_POLICY=/^(?:System|Policy(?:0|[1-9][0-9]{0,2}))$/;
const VALID_ZONE=/^(?:\.|[a-z0-9_-]+(?:\.[a-z0-9_-]+)*)$/i;
const VALID_HINT=/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,119}$/;
const HASH_HINT=/^sha256-[0-9a-f]{64}$/i;
const fail=x=>{throw Error(x)};

export function parseNativeSelector(selector){
  const value=String(selector||'');
  if(!value.startsWith('native://'))return null;
  const parts=value.slice(9).split('/');
  if(parts.length!==3 || !VALID_POLICY.test(parts[0]) || !VALID_ZONE.test(parts[1]) ||
     !(VALID_HINT.test(parts[2]) || HASH_HINT.test(parts[2])))return null;
  return {instance:parts[0],zone:parts[1],hint:parts[2],selector:value};
}

function policyPresentation(name,policies){
  const found=policies.find(p=>p.name===name);
  if(name==='System')return {label:'Системная (System)',route:'системный DNS; маршрут не проверен'};
  const label=found&&typeof found.description==='string'&&found.description.trim()?
    found.description.trim()+' ('+name+')':name+' (имя неизвестно)';
  let route='маршрут неизвестен';
  if(found?.ipv4_route_status==='default_present')route='default route есть (не DNS-проверка)';
  if(found?.ipv4_route_status==='default_absent')route='нет default route';
  return {label,route};
}

export function catalogNativeInventory(inventory,catalog){
  const policies=Array.isArray(catalog?.policies)&&catalog?.available?catalog.policies:[];
  const instances=Array.isArray(inventory?.instances)?inventory.instances:[];
  const summary=instances.filter(x=>x.valid && VALID_POLICY.test(String(x.name||'')))
    .map(x=>{const n=policyPresentation(x.name,policies);return n.label+' — '+n.route;})
    .join(' | ')+(catalog?.available?' · NDMC RO':' · имена/маршруты NDMC недоступны');
  if(!inventory?.available || !instances.length)return {choices:[],summary,available:false};
  const choices=[],seen=new Set(),duplicate=new Set();
  for(const inst of instances){
    if(!inst?.valid || !VALID_POLICY.test(String(inst.name||'')))continue;
    const p=policyPresentation(inst.name,policies);
    for(const ep of Array.isArray(inst.endpoints)?inst.endpoints:[]){
      const zone=String(ep?.domain||'');
      const hint=String(ep?.selector_hint||ep?.upstream_hint||'');
      if(!VALID_ZONE.test(zone) || !(VALID_HINT.test(hint)||HASH_HINT.test(hint)))continue;
      // DoH URLs are not eligible as raw socket selectors: only opaque backend hints.
      if(/^https?:\/\//i.test(String(ep?.upstream_hint||'')) && !HASH_HINT.test(hint))continue;
      const selector='native://'+inst.name+'/'+zone+'/'+hint;
      if(!parseNativeSelector(selector))continue;
      if(seen.has(selector)){duplicate.add(selector);continue;}
      seen.add(selector);
      choices.push({selector,instance:inst.name,zone,hint,
        label:p.label+' / '+zone+' / '+String(ep?.upstream_hint||'без подписи')+' — '+p.route+
          ' (текущий '+String(ep?.address||'неизвестен')+')'});
    }
  }
  return {choices:choices.filter(x=>!duplicate.has(x.selector)),summary,available:true};
}

export function createNativeReader({get}){
  if(typeof get!=='function')fail('NATIVE_READER_GET_REQUIRED');
  async function scan(){
    // The caller must explicitly invoke scan on a user click.
    const [inventory,bridge,catalog]=await Promise.all([
      get('keenetic/native-dns'),get('keenetic/native-bridge/status'),get('keenetic/native-policies')]);
    const result=catalogNativeInventory(inventory,catalog);
    return {...result,bridge};
  }
  async function probe(selector){
    // Never probe during scan; only root-zone upstreams can be probed.
    const parsed=parseNativeSelector(selector);
    if(!parsed||parsed.zone!=='.')fail('Для DNS-проверки требуется upstream корневой зоны «.»');
    const qs=new URLSearchParams({instance:parsed.instance,hint:parsed.hint});
    return get('keenetic/native-dns/probe?'+qs.toString());
  }
  return {scan,probe};
}
