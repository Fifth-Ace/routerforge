import test from 'node:test';
import assert from 'node:assert/strict';
import {startNFQWSRuntime} from './NfqwsRuntime.js';

// Executable no-browser command-layer smoke with a deliberately missing legacy
// #backupCount node. This catches boot DOM regressions missed by text grep.
test('NFQWS runtime boots without dead DOM mirrors, GET-only, and aborts on unmount', async()=>{
  const originals=new Map();
  const stub=(key,value)=>{originals.set(key,Object.getOwnPropertyDescriptor(globalThis,key));Object.defineProperty(globalThis,key,{configurable:true,writable:true,value});};
  const styles={setProperty(){}};
  const html={style:styles,dataset:{}};
  const body={};
  const calls=[];
  const document={documentElement:html,body,getElementById(){return null;},querySelector(){return null;},addEventListener(){},querySelectorAll(){return[];}};
  const win={addEventListener(){},removeEventListener(){}};
  let responseCount=0,doneResolve;
  const completed=new Promise(resolve=>doneResolve=resolve);
  const running=[];
  try{
    stub('document',document);stub('window',win);stub('location',{pathname:'/api/modules/nfqws-manager/ui/index.html',search:''});
    stub('localStorage',{getItem(){return null;},setItem(){},removeItem(){}});
    stub('ResizeObserver',class {observe(){} disconnect(){}});
    stub('requestAnimationFrame',cb=>{running.push(cb);return running.length;});
    stub('cancelAnimationFrame',()=>{});
    stub('parent',{postMessage(){}});
    stub('fetch',async (url,opts={})=>{
      const path=String(url);calls.push({path,method:opts.method||'GET',signal:opts.signal});
      const payload=path.endsWith('/status')?{
        available:true,running:false,mutation_ready:true,config:'',config_path:'/opt/etc/nfqws2/nfqws2.conf',
        target_name:'nfqws2',backup_count:7,lists:[],config_sha256:'a'.repeat(64)
      }: {files:[]};
      if(path.endsWith('/forbidden'))return {ok:false,status:403,headers:{get(){return 'application/json';}},text:async()=>JSON.stringify({error:'FORBIDDEN'})};
      if(path.endsWith('/conflict'))return {ok:false,status:409,headers:{get(){return 'application/json';}},text:async()=>JSON.stringify({error:'STALE_CAS'})};
      if(path.endsWith('/blobs/install'))return {ok:true,status:200,headers:{get(){return 'text/plain';}},text:async()=>'<bad-response/>'};
      return {ok:true,status:200,headers:{get(){return 'application/json';}},text:async()=>JSON.stringify(payload)};
    });
    const runtime=startNFQWSRuntime({
      onStatus:status=>{responseCount++;if(responseCount===1){assert.equal(status.backupCount,7);doneResolve();}},
      onFlowFieldsHydrate(){},onCustomHydrate(){},onOverview(){},onSetup(){},onPreflight(){},
      onListInventory(){},onLogs(){},onBusy(){},onHistory(){},onListTools(){},
      getCustomStrategies:()=>({blocks:[],labels:[]}),getFlowFields:()=>({})
    });
    await Promise.race([completed,new Promise((_,reject)=>setTimeout(()=>reject(Error('NFQWS_BOOT_TIMEOUT')),500))]);
    assert.ok(calls.some(c=>c.path.endsWith('/status')));
    assert.ok(calls.every(c=>c.method==='GET'),'boot must not POST or mutate');
    assert.ok(calls.every(c=>c.signal),'all boot requests must be abortable');
    await assert.rejects(runtime.listRequest('/forbidden'),/FORBIDDEN/);
    await assert.rejects(runtime.listRequest('/conflict'),/STALE_CAS/);
    await assert.rejects(runtime.listRequest('/blobs/install',{method:'POST',body:'{}'}),/NFQWS_BLOB_MUTATION_INVALID_RESPONSE/);
    const before=calls.length;
    runtime.dispose();
    for(const c of calls)assert.equal(c.signal.aborted,true);
    await new Promise(resolve=>setImmediate(resolve));
    assert.equal(calls.length,before,'no polling or hidden navigation after unmount');
    await assert.rejects(runtime.listRequest('/status'),/NFQWS_UI_UNMOUNTED/);
  } finally {
    for(const [key,descriptor] of originals){if(descriptor)Object.defineProperty(globalThis,key,descriptor);else delete globalThis[key];}
  }
});
