// Pure draft transformations. No fetch, mutation of inputs, router operation,
// applied state, global handler, timer or DOM access in this module.
export function cleanP21MAC(input, message='Invalid MAC') {
  const mac=String(input??'').trim().toLowerCase();
  if(!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/.test(mac)||mac==='00:00:00:00:00:00'||(parseInt(mac.slice(0,2),16)&1))throw Error(message);
  return mac;
}
const snapshot=a=>({devices:[...(a?.devices||[])],groups:[...(a?.groups||[])],rules:[...(a?.rules||[])]});
const replaceBy=(rows,key,value)=>{const i=rows.findIndex(x=>x.id===key);if(i<0)rows.push(value);else rows[i]=value;};
export function draftDevice(source,input,invalidMAC){
  const mac=cleanP21MAC(input.mac,invalidMAC),next=snapshot(source);
  const record={mac,name:String(input.name??'').trim(),...(input.target?{target:input.target,inherit_domains:Boolean(input.inherit_domains)}:{})};
  const i=next.devices.findIndex(x=>x.mac===mac);
  if(i<0)next.devices.push(record);else next.devices[i]=record;
  return next;
}
export function draftGroup(source,input,messages={}){
  const next=snapshot(source);
  const id=String(input.id??'');
  const name=String(input.name??'').trim();
  if(!id||!name)throw Error(messages.required||'Missing group');
  const macs=[...new Set(String(input.members??'').split(/[\s,]+/).filter(Boolean).map(mac=>cleanP21MAC(mac,messages.invalidMAC)))];
  if(next.groups.some(group=>group.id!==id&&group.macs.some(mac=>macs.includes(mac))))throw Error(messages.conflict||'MAC assigned twice');
  const record={id,name,macs,...(input.target?{target:input.target,inherit_domains:Boolean(input.inherit_domains)}:{})};
  replaceBy(next.groups,id,record);
  return next;
}
export function draftRule(source,input,messages={}){
  const next=snapshot(source);
  const mac=cleanP21MAC(input.mac,messages.invalidMAC);
  const domains=input.domains||[];
  const priority=Number(input.priority);
  const target=input.target;
  const match=input.match;
  if(domains.length!==1||!target||!Number.isInteger(priority)||priority<0||priority>1000000)throw Error(messages.required||'Invalid rule');
  const previous=next.rules.find(r=>r.mac===mac&&r.domain===domains[0]&&r.match===match);
  let id=previous?.id;
  if(!id){let i=1;const keys=new Set(next.rules.map(r=>r.id));while(keys.has('device-domain-'+i))i++;id='device-domain-'+i;}
  const record={id,mac,domain:domains[0],match,priority,target};
  replaceBy(next.rules,id,record);
  return next;
}
export function dropDeviceOverrides(source,mac){
  const next=snapshot(source);
  next.devices=next.devices.map(d=>d.mac===mac?(({target,inherit_domains,...other})=>other)(d):d);
  next.groups=next.groups.map(g=>({...g,macs:g.macs.filter(m=>m!==mac)}));
  next.rules=next.rules.filter(r=>r.mac!==mac);
  return next;
}
export function dropGroup(source,id){const next=snapshot(source);next.groups=next.groups.filter(g=>g.id!==id);return next;}
export function dropRule(source,id){const next=snapshot(source);next.rules=next.rules.filter(r=>r.id!==id);return next;}
export function aggregateP21Devices(inventory,source){
  const rows=new Map(),a=source||{};
  for(const row of inventory||[]){
    if(!row.mac)continue;
    if(!rows.has(row.mac))rows.set(row.mac,{mac:row.mac,name:row.name,ips:[],trusted:false});
    const record=rows.get(row.mac);record.ips.push(row.ip+' · '+row.state);record.trusted ||= Boolean(row.trusted);
  }
  for(const d of a.devices||[]){if(!rows.has(d.mac))rows.set(d.mac,{mac:d.mac,ips:[],trusted:false});rows.get(d.mac).name=d.name||rows.get(d.mac).name;}
  for(const g of a.groups||[])for(const mac of g.macs||[])if(!rows.has(mac))rows.set(mac,{mac,ips:[],trusted:false});
  for(const rule of a.rules||[])if(!rows.has(rule.mac))rows.set(rule.mac,{mac:rule.mac,ips:[],trusted:false});
  return [...rows.values()];
}
