// Explicit Runtime commands. UI components import these functions directly;
// no window bridge, background activation or DOM click indirection.
import {createConfigFlow} from './DnsRuntimeConfigLogic.js';
import {apiRequest,actionOptions} from './DnsRuntimeApi.js';
import {draft,simple,readDraft,commitDraft,feedback,summary,stageDomains,acceptScan,scanError} from './DnsRuntimeStore.svelte.js';

export const configFlow=createConfigFlow({
  readDraft,request:apiRequest,commit:commitDraft,
  confirmAction:message=>window.confirm(message),notify:feedback
});

let nativeScanSequence=0;
let activeNativeScan=null;
export async function scanNative(signal){
  if(signal?.aborted)return;
  // A duplicate click in one mount must not generate duplicate API reads.
  // After stop/unmount, a new mount is allowed to start immediately even if
  // the previous network implementation ignores AbortController.
  if(activeNativeScan&&!activeNativeScan.signal.aborted)return;
  const controller=new AbortController();
  const seq=++nativeScanSequence;
  const onAbort=()=>controller.abort();
  if(signal)signal.addEventListener('abort',onAbort,{once:true});
  activeNativeScan=controller;
  simple.status='Читаю DNS и политики Keenetic (только чтение)…';
  try{
    const [inventory,catalog]=await Promise.all([
      apiRequest('keenetic/native-dns',{signal:controller.signal}),
      apiRequest('keenetic/native-policies',{signal:controller.signal})
    ]);
    if(seq===nativeScanSequence&&!controller.signal.aborted)acceptScan(inventory,catalog);
  }catch(error){
    if(seq===nativeScanSequence&&!controller.signal.aborted)
      scanError('Не удалось прочитать Keenetic: '+String(error?.message||error));
  }finally{
    if(signal)signal.removeEventListener('abort',onAbort);
    if(activeNativeScan===controller)activeNativeScan=null;
  }
}

let liteApi=null;
export function registerLite(api){
  if(liteApi)throw Error('DNS_RUNTIME_LITE_DUPLICATE_OWNER');
  liteApi=api;
  return ()=>{if(liteApi===api)liteApi=null;};
}
export function getLite(){if(!liteApi)throw Error('DNS_RUNTIME_LITE_NOT_MOUNTED');return liteApi;}
export const p10Runtime={
  readDraft,commit:commitDraft,
  simple:()=>({policy:simple.policy,pending:Boolean(simple.domains.trim()),summary:summary()}),
  stagePending:()=>{if(simple.domains.trim()&&!stageDomains(message=>window.confirm(message)))
    throw Error('Не удалось добавить домены: '+simple.status);},
  isLite:()=>getLite().detect(),
  refreshLite:()=>getLite().refresh(),
  applyLite:()=>getLite().apply(),
  stopLite:()=>getLite().stop(),
  report:(message,bad=false)=>{simple.notice=String(message);if(bad)feedback(message);}
};
export const p91Runtime={
  readDraft,commit:commitDraft,
  lookup:body=>apiRequest('certificates/lookup',actionOptions('certificate-lookup',body)),
  refresh:()=>{},
  notify:feedback
};
export function stopRuntimeRequests(){
  ++nativeScanSequence;
  if(activeNativeScan){activeNativeScan.abort();activeNativeScan=null;}
}
