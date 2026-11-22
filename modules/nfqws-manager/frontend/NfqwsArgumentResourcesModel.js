// Read-only resource projection for native Svelte NFQWS argument editors.
// Exact names are UI hints, not authorization to read or write router paths.
function basename(path) {return String(path||'').replace(/^@/,'').split('/').pop()||'';}
export function strategyResources(value,{kind='strategy',baseArgs='',knownLists=[]}={}) {
  const text=String(value||''),items=[],seen=new Set();
  const append=(type,name,path='',status='')=>{
    const key=type+':'+name+':'+path;
    if(!name||seen.has(key))return;
    seen.add(key);items.push({type,name,path,status});
  };
  const blobDefs=new Map();let m;
  const br=/--blob=([^:\s]+):(@?\/[^\s]+)/g;
  while((m=br.exec(String(baseArgs||'')))!==null)blobDefs.set(m[1],m[2]);
  if(kind==='base'){
    while((m=br.exec(text))!==null)append('blobdef',m[1],m[2],'declared');
    const lr=/--lua-init=(@?\/[^\s]+)/g;
    while((m=lr.exec(text))!==null)append('lua',basename(m[1]),m[1],'read-only');
    return items;
  }
  const lists=new Set(Array.isArray(knownLists)?knownLists.map(String):[]);
  const listPattern=/--(?:hostlist|hostlist-exclude|hostlist-auto|ipset|ipset-exclude)=([^\s]+)/g;
  while((m=listPattern.exec(text))!==null){
    const path=m[1].replace(/["']/g,''),name=basename(path);
    append('list',name,path,lists.has(name)?'available':'unverified');
  }
  const blobs=/(?:^|:)blob=([^:\s,]+)/gm;
  while((m=blobs.exec(text))!==null){
    const name=m[1],inline=/^0x[0-9a-f]+$/i.test(name);
    append('blob',name,'',inline?'inline':blobDefs.has(name)?'declared':'unverified');
  }
  return items;
}
export function safeResourceName(name) {
  return typeof name==='string'&&/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(name)&&!name.includes('..');
}
