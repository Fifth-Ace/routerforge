// Pure Config Library policy: preserve existing backend confirm contracts.
// The backend has NO atomic If-Match on these endpoints. SHA preflight guards
// stale UIs but does not pretend to be server-side CAS.
const fullSHA = /^[a-f0-9]{64}$/i;
export function normalizeConfigName(value) {
  if(/[\x00-\x1f\x7f]/.test(String(value??'')))return '';
  let name = String(value ?? '').trim();
  if (!name) return '';
  if (!/\.conf$/i.test(name)) name += '.conf';
  return /^[a-z0-9][a-z0-9._-]*\.conf$/i.test(name) && name.length <= 160 ? name : '';
}
export function configLibraryDiff(before, after) {
  const a=String(before).split(/\r?\n/), b=String(after).split(/\r?\n/), out=[];
  for(let i=0;i<Math.max(a.length,b.length);i++) {
    if(a[i]===b[i]) {if(a[i]!==undefined)out.push({type:'same',text:'  '+a[i]});continue;}
    if(a[i]!==undefined)out.push({type:'del',text:'- '+a[i]});
    if(b[i]!==undefined)out.push({type:'add',text:'+ '+b[i]});
  }
  return out;
}
export function configFileStats(text){
  const value=String(text ?? ''), bytes=new TextEncoder().encode(value).length;
  return {lines:value.split(/\r?\n/).length,bytes};
}
export function formatLibraryBytes(size) {
  const n=Number(size);
  if(!Number.isFinite(n)||n<0)return '—';
  if(n<1024)return `${n} B`;
  return n<1048576 ? `${(n/1024).toFixed(1)} KB`:`${(n/1048576).toFixed(1)} MB`;
}
export function requireUnchangedLibraryFile(reference, current) {
  if(!reference || !current || !fullSHA.test(String(reference.sha256||'')) ||
     !fullSHA.test(String(current.sha256||'')) ||
     reference.name !== current.name || reference.sha256.toLowerCase()!==current.sha256.toLowerCase())
    throw new Error('CONFIG_LIBRARY_STALE_SHA: файл изменён; перечитай конфиг. Запись запрещена');
  return true;
}
const routes={
  'create-empty':['/configs/create','NFQWS_CONFIG_LIBRARY_CREATE'],
  'create-current':['/configs/create','NFQWS_CONFIG_LIBRARY_CREATE'],
  'rename':['/configs/rename','NFQWS_CONFIG_LIBRARY_RENAME'],
  'duplicate':['/configs/duplicate','NFQWS_CONFIG_LIBRARY_DUPLICATE'],
  'import-neighbor':['/configs/import-neighbor','NFQWS_CONFIG_LIBRARY_IMPORT'],
  'save':['/configs/save','NFQWS_CONFIG_LIBRARY_SAVE'],
  'delete':['/configs/delete','NFQWS_CONFIG_LIBRARY_DELETE'],
  'activate':['/configs/activate','NFQWS_CONFIG_LIBRARY_ACTIVATE']
};
export function libraryPostSpec(operation, fields) {
  const route=routes[operation];if(!route)throw Error('CONFIG_LIBRARY_UNKNOWN_OPERATION');
  const name=normalizeConfigName(fields?.name);
  if(!name)throw Error('CONFIG_LIBRARY_INVALID_NAME');
  const body={name,confirm:route[1]};
  if(operation==='create-empty'||operation==='create-current')Object.assign(body,{from_current:operation==='create-current',content:''});
  if(operation==='rename'||operation==='duplicate'){
    const newName=normalizeConfigName(fields?.newName);if(!newName)throw Error('CONFIG_LIBRARY_INVALID_NEW_NAME');
    body.new_name=newName;
  }
  if(operation==='import-neighbor'){
    const source=String(fields?.name||'');
    if(!/^[a-z0-9][a-z0-9._-]*\.(conf|cfg)$/i.test(source) || source.length>160)throw Error('CONFIG_LIBRARY_INVALID_NEIGHBOR');
    body.name=source;
    body.destination=normalizeConfigName(fields?.newName);
    if(!body.destination)throw Error('CONFIG_LIBRARY_INVALID_NEW_NAME');
  }
  if(operation==='save')body.content=String(fields?.content??'');
  return {path:route[0],options:{method:'POST',body:JSON.stringify(body)}};
}
export function libraryReadPath(name){
  const safe=normalizeConfigName(name);if(!safe||safe!==name)throw Error('CONFIG_LIBRARY_INVALID_NAME');
  return `/configs/read?name=${encodeURIComponent(safe)}`;
}
export function createLibraryRequest(fetcher,pathname,signal){
  if(typeof fetcher!=='function')throw Error('LIBRARY_FETCH_MISSING');
  const marker='/ui/',pos=String(pathname||'').indexOf(marker);
  const base=pos>=0?pathname.slice(0,pos):'/api/modules/nfqws-manager';
  return async(path,options={})=>{
    if(!/^\/(configs(?:\/|\?|$))/.test(path)||/[\\\r\n]/.test(path)||path.includes('..'))throw Error('CONFIG_LIBRARY_UNTRUSTED_API_PATH');
    if(signal?.aborted)throw Error('NFQWS_UI_UNMOUNTED');
    const response=await fetcher(base+path,{
      credentials:'same-origin',cache:'no-store',signal,
      headers:{Accept:'application/json','Content-Type':'application/json'},...options
    });
    let payload;try{payload=await response.json();}catch{throw Error('CONFIG_LIBRARY_INVALID_JSON');}
    if(!response.ok)throw Error('HTTP '+response.status+': '+String(payload?.error||'unknown error'));
    if(!payload||typeof payload!=='object'||Array.isArray(payload))throw Error('CONFIG_LIBRARY_INVALID_RESPONSE');
    return payload;
  };
}
