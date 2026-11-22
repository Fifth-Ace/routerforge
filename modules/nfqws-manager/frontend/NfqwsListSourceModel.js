// Pure view projection. Installation, SHA checks, CAS, snapshots and explicit
// confirmation remain server-guarded commands, never layout side effects.
export function nfqwsListSources(snapshot={}) {
  const local=Array.isArray(snapshot.local)?snapshot.local:[];
  const upstream=Array.isArray(snapshot.upstream)?snapshot.upstream:[];
  const byName=new Map(local.filter(x=>x?.name).map(x=>[String(x.name),x]));
  const upstreamRows=upstream.map(item=>{
    const installed=byName.get(String(item.name))||null;
    const verified=installed&&String(installed.sha256||'').toLowerCase()===String(item.sha256||'').toLowerCase();
    return {...item,installed,installState:!installed?'missing':verified?'ok':'update'};
  });
  const resolutionState=String(snapshot.resolution?.state||''),source=String(snapshot.resolution?.source||''),ref=String(snapshot.ref||''),error=String(snapshot.error||'');
  return {
    local,upstream:upstreamRows,
    root:String(snapshot.root||'/opt/etc/nfqws2/lists'),ref,error,loading:Boolean(snapshot.loading),
    localCount:local.length,upstreamCount:upstream.length,
    changedCount:upstreamRows.filter(x=>x.installState!=='ok').length,
    referencedCount:local.filter(x=>Boolean(x.referenced)).length,
    state:error?(resolutionState==='rate_limited'?'RATE LIMITED':'OFFLINE'):resolutionState==='stale_cache'?'STALE CACHE':ref?'SYNCED':'LOCAL',
    upstreamMeta:ref?`rndnaame/nfqws-menu · exact ref ${ref.slice(0,12)}${source?` · ${source}`:''}`:`rndnaame/nfqws-menu · ${error||resolutionState||'not synced'}`
  };
}
