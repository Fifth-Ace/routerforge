// Native Svelte observed-target actions. No writes occur during projection or mount.
const allowed={
  '/v2/observed-targets':'GET',
  '/v2/observed-targets/scan':'POST',
  '/v2/observed-targets/ignore':'POST',
  '/list?name=user.list':'GET',
  '/list/save':'POST'
};
export function observedPath(path,method='GET'){
  if(allowed[path]!==method)throw Error('NFQWS_OBSERVED_ENDPOINT_FORBIDDEN');
  return path;
}
export function observedTarget(row){
  return String(row?.target||row?.destination||'').trim();
}
export function observedDomain(row){
  const t=observedTarget(row).toLowerCase();
  if(!t||t.length>253||t.includes('..')||!t.includes('.')||t.startsWith('.')||t.endsWith('.'))return '';
  if(!/^[a-z0-9.-]+$/.test(t)||/^\d{1,3}(?:\.\d{1,3}){3}$/.test(t))return '';
  if(t.split('.').some(label=>!label||label.length>63||label.startsWith('-')||label.endsWith('-')))return '';
  if(t.split('.').every(label=>/^\d+$/.test(label)))return '';
  return t;
}
export function observedReason(reason){
  return ({tcp_syn_no_reply:'TCP SYN без SYN/ACK',tcp_unreplied:'TCP без ответа',tcp_rst_or_early_close:'TCP RST / раннее закрытие',udp_no_reply:'UDP egress без ответа'})[String(reason||'')]||String(reason||'suspected failure');
}
export function observedProject(payload){
  if(!payload||!Array.isArray(payload.observed))throw Error('NFQWS_OBSERVED_DATA_INVALID');
  const number=n=>Number.isFinite(Number(n))?Math.max(0,Number(n)):0;
  return {
    rows:payload.observed.filter(row=>row&&typeof row==='object').map(row=>({
      target:observedTarget(row),domain:observedDomain(row),active:!!row.active,
      device:String(row.device||'—'),protocol:String(row.protocol||''),port:number(row.port),
      reason:observedReason(row.reason),count:number(row.count),lastSeen:String(row.last_seen||'—')
    })),
    suppressed:number(payload.suppressed_count),ignored:number(payload.ignored_count),
    minEvents:number(payload.min_events||2),flowSource:String(payload.flow_source||'stored evidence')
  };
}
export function observedScanBody(){return{confirm:'ROUTERFORGE_OBSERVED_SCAN'};}
export function observedIgnoreBody(row){
  const target=observedTarget(row);
  if(!target||target.length>253||/[\x00-\x1f]/.test(target))throw Error('NFQWS_OBSERVED_TARGET_INVALID');
  return{target,ignore:true,confirm:'ROUTERFORGE_OBSERVED_IGNORE'};
}
export function observedAppendUserList(snapshot,row){
  const target=observedDomain(row);
  if(!target)throw Error('NFQWS_OBSERVED_DOMAIN_REQUIRED');
  if(typeof snapshot?.content!=='string'||!/^[a-f\d]{64}$/i.test(String(snapshot.sha256||'')))throw Error('NFQWS_OBSERVED_LIST_CAS_REQUIRED');
  if(snapshot.content.split(/\r?\n/).some(line=>line.trim().toLowerCase()===target))return null;
  const content=snapshot.content+(snapshot.content&&!snapshot.content.endsWith('\n')?'\n':'')+target+'\n';
  return{name:'user.list',content,expected_sha256:snapshot.sha256,confirm:'NFQWS_LIST_SAVE'};
}
