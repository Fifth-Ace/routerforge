export function appendPolicyDraft(original, values, insensitive=false){
  if(typeof original!=='string'||!Array.isArray(values)||values.length>4096||values.some(v=>typeof v!=='string'||!v||/[\s\\]/.test(v)))throw Error('INVALID_GROUP_VALUES');
  const normalize=x=>insensitive?x.toLowerCase():x;
  const seen=new Set(original.split(/\r?\n/).map(x=>normalize(x.trim()))), added=[];
  for(const v of values){const k=normalize(v);if(!seen.has(k)){seen.add(k);added.push(v)}}
  if(!added.length)return original;
  const nl=original.includes('\r\n')?'\r\n':'\n';
  const out=original+(original&&!original.endsWith('\n')?nl:'')+added.join(nl)+nl;
  if(new TextEncoder().encode(out).length>256*1024)throw Error('DRAFT_TOO_LARGE');
  return out;
}
