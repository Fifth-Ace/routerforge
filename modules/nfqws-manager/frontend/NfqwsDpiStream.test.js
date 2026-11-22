import test from 'node:test';
import assert from 'node:assert/strict';
import {consumeDpiNdjson} from './NfqwsDpiStream.js';

function reply(chunks, status=200) {
  const stream=new ReadableStream({start(controller){for(const c of chunks)controller.enqueue(new TextEncoder().encode(c));controller.close();}});
  return {ok:status===200,status,body:stream,text:async()=>JSON.stringify({error:'DENIED'})};
}

test('NDJSON true chunk boundaries and explicit one-shot POST',async()=>{
  const got=[];let calls=0;
  await consumeDpiNdjson({url:'/api/modules/nfqws-manager/dpi-detector/v5/stream',payload:{confirm:'ROUTERFORGE_DPI_DETECTOR_V5_RUN'},onEvent:e=>got.push(e),fetchImpl:async (url,opt)=>{
    calls++;assert.equal(opt.method,'POST');assert.equal(opt.credentials,'same-origin');assert.equal(JSON.parse(opt.body).confirm,'ROUTERFORGE_DPI_DETECTOR_V5_RUN');
    return reply(['{"type":"ch','unk","data":"x"}\n{"type":"done"}\n']);
  }});
  assert.equal(calls,1);assert.deepEqual(got,[{type:'chunk',data:'x'},{type:'done'}]);
});

test('already aborted stream produces no network request',async()=>{
  const c=new AbortController();c.abort();let calls=0;
  await assert.rejects(consumeDpiNdjson({url:'/api/modules/nfqws-manager/dpi-detector/v5/stream',signal:c.signal,onEvent:()=>{},fetchImpl:()=>{calls++;}}),/CANCELLED/);
  assert.equal(calls,0);
});

test('403 does not emit success',async()=>{
  const got=[];
  await assert.rejects(consumeDpiNdjson({url:'/api/modules/nfqws-manager/dpi-detector/v5/stream',onEvent:e=>got.push(e),fetchImpl:async()=>reply([],403)}),/DENIED/);
  assert.equal(got.length,0);
});

test('malformed NDJSON lines are ignored but valid event is delivered',async()=>{
  const got=[];
  await consumeDpiNdjson({url:'/api/modules/nfqws-manager/dpi-detector/v5/stream',onEvent:e=>got.push(e),fetchImpl:async()=>reply(['broken\n{"type":"done"}'])});
  assert.deepEqual(got,[{type:'done'}]);
});

test('oversized NDJSON line fails closed',async()=>{
  await assert.rejects(consumeDpiNdjson({url:'/api/modules/nfqws-manager/dpi-detector/v5/stream',onEvent:()=>{},fetchImpl:async()=>reply(['x'.repeat(16*1024*1024+1)])}),/LINE_LIMIT/);
});

test('untrusted or external URL never issues network request',async()=>{
  let called=0;
  await assert.rejects(consumeDpiNdjson({url:'https://example.com/dpi-detector/v5/stream',onEvent:()=>{},fetchImpl:()=>{called++;}}),/PATH_INVALID/);
  assert.equal(called,0);
});
