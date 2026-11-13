import test from 'node:test';
import assert from 'node:assert/strict';
import {readNfqwsScripts} from './NfqwsReadOnlyScripts.js';

test('Lua inventory is GET only and never starts a service mutation',async()=>{
  let called=0;
  const result=await readNfqwsScripts('inventory',{fetchImpl:async (url,opts)=>{
    called++;assert.equal(url,'/api/modules/nfqws-manager/scripts');assert.equal(opts.method,'GET');assert.equal(opts.credentials,'same-origin');
    return {ok:true,json:async()=>({files:[{name:'test.lua'}]})};
  }});
  assert.equal(called,1);assert.equal(result.files[0].name,'test.lua');
});

test('Lua file names are URL-encoded within read-only endpoint',async()=>{
  await readNfqwsScripts('name:hello-world.lua',{fetchImpl:async (url)=>{
    assert.equal(url,'/api/modules/nfqws-manager/scripts?name=hello-world.lua');return {ok:true,json:async()=>({content:'-- Lua'})};
  }});
});

test('403 and stale cancellation fail closed',async()=>{
  await assert.rejects(readNfqwsScripts('inventory',{fetchImpl:async()=>({ok:false,status:403,json:async()=>({error:'DENIED'})})}),/DENIED/);
  const c=new AbortController();c.abort();let calls=0;
  await assert.rejects(readNfqwsScripts('inventory',{signal:c.signal,fetchImpl:()=>{calls++;}}),/CANCELLED/);
  assert.equal(calls,0);
});

test('untrusted route or host is refused before network',async()=>{
  let calls=0;
  const f=()=>{calls++;};
  await assert.rejects(readNfqwsScripts('http://evil',{fetchImpl:f}),/PATH_INVALID/);
  await assert.rejects(readNfqwsScripts('name:../passwd',{fetchImpl:f}),/PATH_INVALID/);
  await assert.rejects(readNfqwsScripts('inventory',{base:'https://evil',fetchImpl:f}),/BASE_INVALID/);
  assert.equal(calls,0);
});
