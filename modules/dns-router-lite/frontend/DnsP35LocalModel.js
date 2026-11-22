// P35 input parser for the existing Svelte settings draft. No fetch or auto-apply.
const namePattern=/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/;
export function parseP35Records(text){
  if(typeof text!=='string'||text.length>32768)throw Error('P35: максимум 32 КБ записей');
  const out=[],seen=new Set();
  for(const [idx,line] of text.split(/\r?\n/).entries()){
    const raw=line.trim();if(!raw||raw.startsWith('#'))continue;
    const p=raw.split('|').map(x=>x.trim());
    if(p.length!==4)throw Error('P35: строка '+(idx+1)+' — формат NAME | TYPE | VALUE | TTL');
    const [name,type,value,ttlText]=p;
    if(name!==name.toLowerCase()||!namePattern.test(name)||name.length>253||!['A','AAAA','CNAME','PTR'].includes(type))throw Error('P35: некорректное имя или тип в строке '+(idx+1));
    if(type!=='PTR'&&!name.endsWith('.rf.home.arpa'))throw Error('P35: разрешены только *.rf.home.arpa');
    if(!value||value.length>253||/[\s]/.test(value))throw Error('P35: пустое или некорректное значение в строке '+(idx+1));
    if((type==='CNAME'||type==='PTR')&&(value!==value.toLowerCase()||!namePattern.test(value)||!value.endsWith('.rf.home.arpa')))throw Error('P35: CNAME/PTR должны указывать на *.rf.home.arpa');
    const ttl=Number(ttlText);
    if(!/^\d+$/.test(ttlText)||!Number.isInteger(ttl)||ttl<30||ttl>86400)throw Error('P35: TTL должен быть 30..86400');
    const key=name+'|'+type;
    if(seen.has(key))throw Error('P35: повтор записи '+key);
    seen.add(key);
    out.push({name,type,value,ttl});
    if(out.length>128)throw Error('P35: максимум 128 записей');
  }
  return out;
}
export function formatP35Records(records){
  return (Array.isArray(records)?records:[]).map(r=>[r.name,r.type,r.value,r.ttl].join(' | ')).join('\n');
}
