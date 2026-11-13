// Pure functions for the native Svelte owner of the active nfqws2.conf editor.
// No DOM, network, configuration writes or service actions at evaluation time.
export const sha256RE = /^[0-9a-f]{64}$/i;
export function statusRawConfig(status) {
  if (!status || typeof status !== 'object' || typeof status.config !== 'string')
    throw Error('NFQWS_CONFIG_STATUS_INVALID');
  return {
    content: status.config,
    path: String(status.config_path || '—'),
    sha256: String(status.config_sha256 || ''),
    size: Number.isFinite(Number(status.config_size)) ? Number(status.config_size) : 0
  };
}
export function rawDiff(before, after) {
  const a = String(before ?? '').split(/\r?\n/), b = String(after ?? '').split(/\r?\n/), out=[];
  for(let i=0; i<Math.max(a.length,b.length);i++) {
    if(a[i]===b[i]) {if(a[i]!==undefined)out.push({kind:'same',text:'  '+a[i]});continue;}
    if(a[i]!==undefined)out.push({kind:'del',text:'- '+a[i]});
    if(b[i]!==undefined)out.push({kind:'add',text:'+ '+b[i]});
  }
  return out;
}
export function rawStatistics(source) {
  const content=String(source ?? '');
  return {lines:content.split(/\r?\n/).length, bytes:new TextEncoder().encode(content).length};
}
export function rawPreflight(reference, actual, editor, sent) {
  const ref=statusRawConfig(reference), latest=statusRawConfig(actual);
  if(!sha256RE.test(ref.sha256) || !sha256RE.test(latest.sha256) ||
     ref.sha256.toLowerCase()!==latest.sha256.toLowerCase() ||
     ref.content!==latest.content || ref.path!==latest.path)
    throw Error('NFQWS_ACTIVE_CONFIG_STALE: серверный конфиг изменился, запись запрещена');
  if(typeof editor!=='string' || editor===ref.content || editor!==sent)
    throw Error('NFQWS_ACTIVE_CONFIG_DRAFT_CHANGED');
  return true;
}
export function rawLuaPayloadReplacement(editor, start, end, validatedSource) {
  const text=String(editor??''),a=Number(start),b=Number(end);
  if(text!==validatedSource || !Number.isInteger(a)||!Number.isInteger(b)||a<0||b<=a||b>text.length)
    throw Error('NFQWS_LUA_SELECTION_STALE');
  const selected=text.slice(a,b);
  if(!/^--lua-desync=(?:fake|multisplit|hostfakesplit|multidisorder)(?::[A-Za-z0-9_.,=+\-@]+)*$/.test(selected)||selected.includes('payload='))
    throw Error('NFQWS_LUA_SELECTION_INVALID');
  return {text:text.slice(0,a)+selected+':payload=all'+text.slice(b),selectionStart:a,selectionEnd:b+':payload=all'.length};
}
export function rawValidationSummary(response) {
  if(!response||typeof response!=='object'||typeof response.syntax_ok!=='boolean'||typeof response.semantic_ok!=='boolean')
    throw Error('NFQWS_CONFIG_VALIDATION_INCOMPLETE');
  const issues=Array.isArray(response.issues)?response.issues:[];
  const dry=String(response.native_dry_run||'not_performed');
  return {issues,unknownLua:issues.some(x=>x?.code==='UNKNOWN_PAYLOAD_LUA'),
    dry,valid:response.syntax_ok&&response.semantic_ok&&dry!=='failed',
    text:!response.syntax_ok?'Синтаксис shell: '+String(response.error||'ошибка'):
      !response.semantic_ok?'Ошибки семантики: сохранение будет отклонено':
      dry==='failed'?'Native dry-run отклонил параметры':
      dry==='passed'?'Синтаксис, параметры и native nfqws2 --dry-run: PASS':
      'Статический анализ: OK · native dry-run не подтверждён'};
}
export function rawApiRequest(fetcher, pathname, signal){
  if(typeof fetcher!=='function')throw Error('NFQWS_CONFIG_FETCH_MISSING');
  const path=String(pathname||''),marker='/ui/',pos=path.indexOf(marker),base=pos>=0?path.slice(0,pos):'/api/modules/nfqws-manager';
  return async (route, options={})=>{
    if(!['/status','/config','/config/validate','/config/lua-catalog'].includes(route))throw Error('NFQWS_CONFIG_UNTRUSTED_ROUTE');
    if(signal?.aborted)throw Error('NFQWS_CONFIG_UNMOUNTED');
    const response=await fetcher(base+route,{credentials:'same-origin',cache:'no-store',signal,
      headers:{Accept:'application/json','Content-Type':'application/json'},...options});
    let body;
    try{body=await response.json();}catch{throw Error('NFQWS_CONFIG_INVALID_JSON');}
    if(!response.ok)throw Error('HTTP '+response.status+': '+String(body?.error||'unknown error'));
    if(!body||typeof body!=='object'||Array.isArray(body))throw Error('NFQWS_CONFIG_INVALID_RESPONSE');
    return body;
  };
}
