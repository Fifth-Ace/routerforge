// R10: side-effect-free presentation of native Keenetic policies and grouped DNS rules.
// Security-critical zone and CAS checks remain with the existing draft/action owner.
const validPolicy=/^(System|Policy(?:0|[1-9][0-9]{0,2}))$/;
export function policyCards(instances=[],catalog=null,selected='System') {
  const names=new Map((catalog?.available ? (catalog.policies||[]) : []).map(x=>[x.name,x]));
  return (Array.isArray(instances)?instances:[]).filter(x=>x?.valid && validPolicy.test(x.name)).map(instance=>{
    const name=instance.name,policy=names.get(name);
    const route=name==='System'?'Системный DNS':
      !policy?'Маршрут не проверен':policy.ipv4_route_status==='default_absent'?'Нет маршрута по умолчанию':
      policy.ipv4_route_status==='default_present'?'Маршрут найден · DNS не проверен':'Маршрут не проверен';
    return {name,title:name==='System'?'Системная':(policy?.description||name),
      subtitle:name==='System'?'Основная · System':name,route,selected:name===selected};
  });
}
export function inventoryCards(entries=[]) {
  return (Array.isArray(entries)?entries:[]).map((e,i)=>({key:String(i),title:String(e.label||''),
    meta:(e.usable?'Можно выбрать':'Только просмотр')+' · '+
      (e.zone==='.'?'Любые домены':'Только '+(e.zone||'неизвестная зона')+' и поддомены')+
      (e.transport==='doh'?' · DoH':''),
    details:e.reason||'Локальный DNS Keenetic · '+String(e.address||'') }));
}
export function groupedCards(groups=[],choices=[],editing=null,policy='System') {
  const allowed=new Set(choices);
  return (Array.isArray(groups)?groups:[]).map(g=>{
    const selector=g.u ? (g.u.transport==='native'?g.u.address:'manual:'+g.id):'';
    const available=Boolean(selector&&allowed.has(selector));
    const rules=Array.isArray(g.rules)?g.rules:[];
    const simple=rules.length<=200&&rules.every(r=>r.match==='suffix'&&r.priority===100&&
      typeof r.domain==='string'&&Object.keys(r).every(k=>['id','domain','match','upstream','priority'].includes(k)));
    return {id:g.id,title:g.info?.title||'НЕИЗВЕСТНЫЙ DNS',details:g.info?.detail||'',
      scope:g.info?.scope==='manual'?'Ручной DNS Router · не привязан к политике Keenetic':
        g.info?.scope==='native'?'Штатный DNS выбранной политики: '+policy:'ОШИБКА · отсутствует upstream',
      count:rules.length,domains:rules.map(r=>r.domain).join('\n'),available,canEdit:available&&simple,
      hasExtended:!simple,editing:editing===g.id};
  });
}
