// Same-origin API client. No HTTP request without an explicit caller.
export async function apiRequest(path, options = {}) {
  if(typeof path!=='string'||!path||path.startsWith('/')||path.includes('..')||/^[a-z]+:/i.test(path))
    throw Error('DNS_RUNTIME_API_PATH_INVALID');
  const {allow404=false, ...init}=options;
  const response=await fetch('../'+path,{cache:'no-store',credentials:'same-origin',...init});
  if(allow404&&response.status===404)return null;
  if(!(response.headers.get('content-type')||'').toLowerCase().includes('json'))
    throw Error('Некорректный JSON · '+path);
  let payload;
  try {payload=await response.json();}
  catch {throw Error('Некорректный JSON · '+path);}
  if(!response.ok)throw Error('HTTP '+response.status+': '+(payload?.error||'unknown error'));
  return payload;
}
export function actionOptions(action,body,extra={}){
  return {method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':action,...extra},body:JSON.stringify(body)};
}
