import test from 'node:test';
import assert from 'node:assert/strict';
import {createDnsLiteApi} from '../modules/dns-router-lite/frontend/DnsLiteApi.js';

const response = (body, status=200, type='application/json; charset=utf-8') => ({
  ok:status>=200 && status<300,status,headers:{get:key=>key==='content-type'?type:''},
  json:async()=>body
});

function mock(handler=()=>response({ok:true})) {
  const calls=[];
  const client=createDnsLiteApi({fetchImpl:async(url,options)=>{
    calls.push({url,options});return handler(url,options);
  }});
  return {client,calls};
}

test('DNS Lite boot is GET-only, same-origin, no implicit service start/Gate/stop',async()=>{
  const m=mock((url)=>response(url==='../config'?{config:{upstreams:[],rules:[]},revision:'sha256:a'}:{active:false}));
  for(const path of ['config','service/status','service/upstreams/health','service/traces','service/devices'])
    await m.client.request(path);
  assert.equal(m.calls.length,5);
  assert.ok(m.calls.every(c=>c.options.credentials==='same-origin'&&c.options.cache==='no-store'&&!c.options.method));
  assert.ok(m.calls.every(c=>!c.options.headers['X-RouterForge-Action']));
  m.client.dispose();
});

test('invalid paths and methods cannot escape the API base',async()=>{
  const m=mock();
  for(const path of ['../service/control','https://evil.test','service//status','/service/status','service/status?x=1'])
    await assert.rejects(()=>m.client.request(path),/DNS_LITE_INVALID_API_PATH/);
  await assert.rejects(()=>m.client.request('config',{method:'DELETE'}),/DNS_LITE_METHOD_NOT_ALLOWED/);
  assert.equal(m.calls.length,0);
});

test('mutations must explicitly declare an action',async()=>{
  const m=mock();
  await assert.rejects(()=>m.client.request('service/control',{method:'POST',body:'{}'}),/DNS_LITE_MUTATION_ACTION_REQUIRED/);
  await assert.rejects(()=>m.client.request('config',{method:'PUT',headers:{'X-RouterForge-Action':'save'},body:'{}'}),/DNS_LITE_CAS_REVISION_REQUIRED/);
  assert.equal(m.calls.length,0);
});

test('revision-pinned DNS save preserves If-Match and action header',async()=>{
  const m=mock(()=>response({revision:'sha256:after'}));
  const result=await m.client.request('config',{method:'PUT',headers:{'Content-Type':'application/json','X-RouterForge-Action':'save','If-Match':'"sha256:before"'},body:'{}'});
  assert.equal(result.revision,'sha256:after');
  assert.equal(m.calls[0].options.headers['If-Match'],'"sha256:before"');
  assert.equal(m.calls[0].options.headers['X-RouterForge-Action'],'save');
});

for(const status of [403,409]) {
  test(`HTTP ${status} rejects writes without swallowing status or body`,async()=>{
    const m=mock(()=>response({error:'denied-or-conflict'},status));
    await assert.rejects(()=>m.client.request('config',{method:'PUT',headers:{'X-RouterForge-Action':'save','If-Match':'"sha256:r"'},body:'{}'}),err=>{
      assert.equal(err.status,status);assert.match(err.message,/denied-or-conflict/);return true;
    });
  });
}

test('non-JSON reply fails even on HTTP 200',async()=>{
  const m=mock(()=>response({ok:true},200,'text/html'));
  await assert.rejects(()=>m.client.request('service/status'),/Ожидался JSON/);
});

test('late request completion after dispose cannot publish stale data',async()=>{
  let finish;
  const m=mock(()=>new Promise(resolve=>{finish=resolve;}));
  const pending=m.client.request('config');
  m.client.dispose();
  finish(response({config:{upstreams:[]},revision:'sha256:stale'}));
  await assert.rejects(pending,/DNS_LITE_DISPOSED/);
  await assert.rejects(()=>m.client.request('config'),/DNS_LITE_DISPOSED/);
});

test('disposal aborts active requests',async()=>{
  let signal;
  const client=createDnsLiteApi({fetchImpl:async(_url,opts)=>{signal=opts.signal;return new Promise(()=>{});}});
  void client.request('config').catch(()=>{});
  assert.equal(signal.aborted,false);
  client.dispose();
  assert.equal(signal.aborted,true);
});

test('timeout aborts active fetch and produces a bounded error',async()=>{
  const client=createDnsLiteApi({timeoutMs:5,fetchImpl:(_url,{signal})=>new Promise((resolve,reject)=>{
    signal.addEventListener('abort',()=>reject(new Error('aborted')),{once:true});
  })});
  await assert.rejects(()=>client.request('service/status'),/DNS_LITE_API_TIMEOUT: service\/status/);
  client.dispose();
});

test('an explicit service command is allowed with action and CAS token',async()=>{
  const m=mock();
  await m.client.request('service/gate/control',{method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':'service-control','If-Match':'"sha256:rev"'},body:JSON.stringify({operation:'preflight'})});
  assert.equal(m.calls[0].url,'../service/gate/control');
  assert.equal(m.calls[0].options.method,'POST');
  assert.match(m.calls[0].options.body,/preflight/);
});
