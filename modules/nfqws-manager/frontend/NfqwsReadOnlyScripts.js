// Read-only NFQWS Lua inventory; never writes files, services or router state.
export async function readNfqwsScripts(path,{signal,fetchImpl=fetch,base}={}) {
  if (path!=='inventory' && !(typeof path==='string' && path.startsWith('name:') &&
      /^[A-Za-z0-9_-][A-Za-z0-9._-]*\.lua$/i.test(path.slice(5)) && path.length<260))
    throw Error('NFQWS_SCRIPT_PATH_INVALID');
  const prefix=String(base||'/api/modules/nfqws-manager');
  if (!/^\/api\/modules\/nfqws-manager$/.test(prefix)) throw Error('NFQWS_SCRIPT_BASE_INVALID');
  if(signal?.aborted)throw Error('NFQWS_SCRIPT_CANCELLED');
  const suffix=path==='inventory'?'/scripts':'/scripts?name='+encodeURIComponent(path.slice(5));
  const response=await fetchImpl(prefix+suffix,{method:'GET',cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json'},signal});
  let data;
  try {data=await response.json();} catch {throw Error('NFQWS_SCRIPT_JSON_INVALID');}
  if(!response.ok)throw Error(String(data?.error||`HTTP ${response.status}`));
  return data;
}
