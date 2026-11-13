// ClientHello presentation, filename and blob-safety rules; no DOM or IO.
export const HELLO_CONFIRM_INSTALL='NFQWS_BLOB_INSTALL';
export const HELLO_CONFIRM_REPLACE='NFQWS_BLOB_REPLACE';
export const HELLO_CAPTURE_SECONDS=5;
export function validHelloBlobName(input){const name=String(input||'').trim();return name.length<=255&&!name.startsWith('.')&&/^[A-Za-z0-9][A-Za-z0-9._-]*\.bin$/i.test(name)&&!name.includes('..');}
export function helloBlobName(sni='',captured=false){const safe=String(sni||'site').toLowerCase().replace(/[^a-z0-9]+/g,'_').replace(/^_+|_+$/g,'').slice(0,180)||'site';return `tls_clienthello_${captured?'from_':''}${safe}.bin`}
export function helloBlobId(name){const base=String(name||'blob').toLowerCase().replace(/\.bin$/,'').replace(/[^a-z0-9_]+/g,'_').replace(/^_+|_+$/g,'');return `rf_${base||'clienthello'}`}
export function helloUsage(generated,name,sni,capabilities){if(!generated?.valid||!validHelloBlobName(name))return'После создания здесь появятся параметры для nfqws2.';
 const id=helloBlobId(name),path=`/opt/etc/nfqws2/blobs/${name}`,rows=[`--blob=${id}:@${path}`];
 if(capabilities?.dynamic_clone_available){rows.push(`--lua-desync=tls_client_hello_clone:payload=tls_client_hello:dir=out:blob=rf_live_clienthello:sni_del:sni_first=${sni}:sni_snt_new=0:fallback=${id}`);rows.push('--lua-desync=fake:payload=tls_client_hello:dir=out:blob=rf_live_clienthello:tls_mod=rnd,dupsid');}
 else rows.push(`--lua-desync=fake:payload=tls_client_hello:dir=out:blob=${id}:tls_mod=rnd,dupsid`);
 return rows.join('\n');
}
export function helloCaptureRows(body){if(!body||typeof body!=='object'||Array.isArray(body))throw Error('NFQWS_CLIENTHELLO_CAPTURE_INVALID');return Array.isArray(body.candidates)?body.candidates.filter(x=>x&&typeof x==='object'):[]}
export function helloSavePlan(generated,name,files){const safe=String(name||'').trim();if(!generated?.valid||!validHelloBlobName(safe))throw Error('NFQWS_CLIENTHELLO_BLOB_NAME_OR_PAYLOAD_INVALID');if(!/^[a-f0-9]{64}$/i.test(String(generated.sha256||''))||typeof generated.content_base64!=='string'||!generated.content_base64)throw Error('NFQWS_CLIENTHELLO_BLOB_HASH_OR_CONTENT_INVALID');if(!Array.isArray(files))throw Error('NFQWS_CLIENTHELLO_INVENTORY_INVALID');const replace=files.some(x=>x?.name===safe);return {name:safe,content_base64:generated.content_base64,expected_sha256:generated.sha256,replace,confirm:replace?HELLO_CONFIRM_REPLACE:HELLO_CONFIRM_INSTALL};}
export function helloCapabilitiesText(d){if(!d||typeof d!=='object')return'Возможности не определены';const tcp=d.tcpdump_available?`tcpdump: ЕСТЬ · ${d.tcpdump_path||'PATH'}`:'tcpdump: НЕТ';const lua=d.lua_available?`zapret-antidpi.lua: НАЙДЕН${d.dynamic_clone_source?' · '+d.dynamic_clone_source:''}`:'zapret-antidpi.lua: НЕ НАЙДЕН';const clone=d.dynamic_clone_available?'tls_client_hello_clone: ЕСТЬ':`tls_client_hello_clone: НЕТ · ${d.clone_reason||'причина неизвестна'}`;return `${tcp} | ${lua} | ${clone}${d.lua_read_error?' · ошибка чтения: '+d.lua_read_error:''}`;}
