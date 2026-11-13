// Read-only projections of TCP16 observation memory and the five-part strategy registry.
const registryEndpoints=['/v2/strategy-registry','/v2/strategy-scores','/v2/strategy-insights','/v2/policy-automation-gate','/v2/strategy-recommendations'];
export function registryMemoryRoute(path,method='GET'){
  if(method!=='GET'||!registryEndpoints.includes(path)&&path!=='/v2/tcp16-memory')throw Error('NFQWS_REGISTRY_READ_ONLY_ROUTE_DENIED');
  return path;
}
export function registryPaths(){return registryEndpoints.slice();}
const arr=x=>Array.isArray(x)?x:[];
const num=x=>Number.isFinite(Number(x))?Number(x):0;
export function registryPct(value){return `${Math.round(Math.max(0,Math.min(1,num(value)))*100)}%`;}
export function tcp16Projection(data){
  if(!data||typeof data!=='object'||!Array.isArray(data.entries))throw Error('NFQWS_TCP16_INVALID');
  return {count:num(data.count),suspected:num(data.suspected),clear:num(data.clear),workingSni:num(data.working_sni),rows:data.entries.map(entry=>({
    network:entry.asn?`AS${entry.asn} · ${entry.provider||''}`:String(entry.cidr||entry.network_key||'—'),
    lastTarget:String(entry.last_target||'—'),ipv4:String(entry.destination_ipv4||'—'),
    suspected:!!entry.cutoff_16k_suspected,confidence:String(entry.confidence||'NEW'),
    suspectedCount:num(entry.suspected_count),clearCount:num(entry.clear_count),
    sni:String(entry.working_sni||'—'),lastObserved:String(entry.last_observed||'—')
  }))};
}
export function registryProjection(registry,scores,insights,automation,recommendations){
  if(!registry||!Array.isArray(registry.entries))throw Error('NFQWS_REGISTRY_INVALID');
  const scoresMap=new Map(arr(scores?.entries).map(x=>[x.fingerprint,x.score]));
  const insightsMap=new Map(arr(insights?.entries).map(x=>[x.fingerprint,x.insight]));
  const autoMap=new Map(arr(automation?.candidates).map(x=>[x.fingerprint,x]));
  const recMap=new Map(arr(recommendations?.items).map(x=>[x.fingerprint,x]));
  const entries=registry.entries.map(item=>({...item,sources:arr(item.sources),aliases:arr(item.aliases),args:arr(item.args),provenance:arr(item.provenance),score:scoresMap.get(item.fingerprint)||null,insight:insightsMap.get(item.fingerprint)||null,automation:autoMap.get(item.fingerprint)||null,recommendation:recMap.get(item.fingerprint)||null}));
  return {entries,sources:arr(registry.sources),protocols:arr(registry.protocols),count:num(registry.count),verified:entries.filter(x=>num(x?.evidence?.verified_count)>0).length,automationReady:num(automation?.ready_count),recommendationTop:num(recommendations?.primary_count)};
}
export function registryFiltered(view,query='',protocol='',source=''){
  const q=String(query||'').trim().toLowerCase();
  return arr(view?.entries).filter(item=>{
    if(protocol&&item.protocol!==protocol)return false;
    if(source&&!arr(item.sources).includes(source))return false;
    if(!q)return true;
    return [item.name,item.family,item.protocol,arr(item.sources).join(' '),arr(item.aliases).join(' '),arr(item.args).join(' ')].join(' ').toLowerCase().includes(q);
  });
}
export function registryDetails(item){
  const evidence=item?.evidence||{},caps=item?.capabilities||{},insight=item?.insight||{},auto=item?.automation||{},rec=item?.recommendation||null;
  return {
    evidence:evidence.verified_count?`${evidence.targets||0} target · ${evidence.verified_count} verify · ${registryPct(evidence.success_rate)} · ${evidence.confidence||'—'}`:'нет verified evidence',
    provenance:arr(item?.provenance).map(x=>`${x.source||x.kind}${x.repository?' · '+x.repository:''}${x.ref?' @ '+String(x.ref).slice(0,12):''}`).join(' | ')||'local',
    capability:arr(caps.bench_transports).length?`bench: ${caps.bench_transports.join(', ')} · desync ${num(caps.desync_count)}`:'bench: not eligible',
    score:Number.isFinite(Number(item?.score?.total))?`score ${item.score.total}/100`:'score —',
    insightState:String(insight.state||'—'),insightSummary:String(insight.summary||'Нет insight.'),
    automationReady:!!auto.eligible,automationText:auto.eligible?'AUTOMATION GATE READY':`AUTOMATION BLOCKED${arr(auto.reasons).length?' · '+auto.reasons.join(', '):''}`,
    recommendation:rec?{primary:!!rec.primary,text:`HISTORICAL ${rec.primary?'TOP ':''}#${rec.rank} · ${rec.reason||''}`} : null,
    args:arr(item?.args).join('\n'),fingerprint:String(item?.fingerprint||'').slice(0,12)
  };
}
