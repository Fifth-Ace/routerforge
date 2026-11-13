// DNS Lite live log view: pure filtering and accounting, no DOM or network.
export const DNS_TYPES = Object.freeze({1:'A',28:'AAAA',15:'MX',16:'TXT',5:'CNAME',12:'PTR',33:'SRV',65:'HTTPS',64:'SVCB',255:'ANY'});
export const qtypeName = value => typeof value==='string'?value:DNS_TYPES[value]||'TYPE '+String(value??'—');
export function liteLogEndpoint(trace,field,upstreams=[]) {
  if(trace.cache)return 'Кэш · локально';
  const direct=trace[field+'_endpoint'];if(direct)return String(direct);
  const id=trace[field];
  if(id==='system-local')return 'Локальный DNS Keenetic';
  if(id==='client-acl'||id==='refused')return 'Отказ ACL';
  const u=upstreams.find(x=>x.id===id);
  return u?`${u.server_name?u.server_name+' ('+u.address+')':u.address} · ${u.transport} [${u.id}]`:String(id||'—');
}
export function visibleLiteLogs(rows, criteria={}, upstreams=[]) {
  const text=String(criteria.filter||'').trim().toLowerCase(),type=String(criteria.type||'');
  const errorsOnly=Boolean(criteria.errorsOnly);
  return (Array.isArray(rows)?rows:[]).slice().reverse().filter(t=>{
    if(type&&qtypeName(t.type)!==type)return false;
    if(errorsOnly&&t.success)return false;
    if(!text)return true;
    return [t.name,t.client,t.device_name,t.device_mac,t.assigned,t.used,
      t.assigned_endpoint,t.used_endpoint,liteLogEndpoint(t,'assigned',upstreams),liteLogEndpoint(t,'used',upstreams)]
      .some(x=>String(x||'').toLowerCase().includes(text));
  });
}
