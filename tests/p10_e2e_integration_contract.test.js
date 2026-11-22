'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const root = path.resolve(__dirname, '..');
const read = p => fs.readFileSync(path.join(root,p),'utf8');
const front = 'modules/dns-router-runtime/frontend/';
const load = p => import(pathToFileURL(path.join(root, front, p)).href);

function makeFlow(createP10Flow, opts={}) {
  const calls=[];
  const draft={revision:'rev-p10',generation:0,dirty:false,config:{schema_version:1,
    upstreams:[{id:'u1',transport:'udp',address:'1.1.1.1:53'}],
    rules:[{id:'r1',domain:'example.com',match:'suffix',upstream:'u1',priority:100}]}};
  const flow=createP10Flow({
    request: async (endpoint, config)=>{
      calls.push({endpoint,config});
      if(endpoint==='keenetic/managed-ownership/status')return opts.owner||{state:'disabled'};
      if(endpoint==='shadow/status')return {lifecycle:{active:false}};
      if(endpoint==='keenetic/durable/disable')return opts.disable||{state:'disabled',baseline_restored:true};
      throw Error('UNEXPECTED_ENDPOINT:'+endpoint);
    },
    readDraft:()=>draft,commit:()=>{throw Error('UNEXPECTED_DRAFT_MUTATION');},
    getSimple:()=>({policy:opts.policy||'System',pending:false,summary:'example.com'}),
    stagePending:()=>{throw Error('UNEXPECTED_PENDING_STAGE');},
    confirmAction:()=>opts.confirm!==false,
    report:()=>{},signal:()=>{}
  });
  return {flow,calls};
}

test('P10 real Svelte control: explicit native action buttons and no legacy bridge', () => {
  const ui=read(front+'DnsRuntimeP10.svelte');
  assert.match(ui,/createP10Flow/);
  for(const id of ['drP92Apply','drP10Disable'])assert.equal((ui.match(new RegExp('id="'+id+'"','g'))||[]).length,1);
  assert.match(ui,/disabled=\{busy\|\|!known/);
  assert.match(ui,/await flow\.activate\(\)/);
  assert.match(ui,/await flow\.disable\(\)/);
});

test('P10 PolicyN rejects activation before any IO, mutation or confirmation', async()=>{
  const {createP10Flow}=await load('DnsRuntimeP10Logic.js');
  const {flow,calls}=makeFlow(createP10Flow,{policy:'Policy1'});
  await assert.rejects(flow.activate(),/только для System/);
  assert.deepEqual(calls,[]);
});

test('P10 cancelled disable does not access DNS or mutate state', async()=>{
  const {createP10Flow}=await load('DnsRuntimeP10Logic.js');
  const {flow,calls}=makeFlow(createP10Flow,{confirm:false});
  assert.deepEqual(await flow.disable(),{cancelled:true});
  assert.deepEqual(calls,[]);
});

test('P10 verified ledger SHA required before any durable disable', async()=>{
  const {createP10Flow}=await load('DnsRuntimeP10Logic.js');
  const {flow,calls}=makeFlow(createP10Flow,{owner:{state:'verified_active'}});
  await assert.rejects(flow.disable(),/владения DNS/);
  assert.deepEqual(calls.map(x=>x.endpoint),['keenetic/managed-ownership/status']);
});

test('P10 reversible disable: CAS+ledger match, restore baseline before stopping shadow', async()=>{
  const {createP10Flow}=await load('DnsRuntimeP10Logic.js');
  const {flow,calls}=makeFlow(createP10Flow,{owner:{state:'verified_active',ledger_hash:'sha256:exact'}});
  // Request stub only permits exactly these two mutating endpoints, after ownership read.
  const a=flow.disable();
  await assert.rejects(a,/UNEXPECTED_ENDPOINT:shadow\/control/);
  assert.deepEqual(calls.map(x=>x.endpoint),[
    'keenetic/managed-ownership/status','keenetic/durable/disable','shadow/control'
  ]);
  const request=calls[1].config;
  assert.equal(request.method,'POST');
  assert.equal(request.headers['X-RouterForge-Action'],'durable-disable');
  assert.equal(request.headers['If-Match'],'"rev-p10"');
  assert.equal(request.headers['X-RouterForge-Ledger-Match'],'sha256:exact');
  const {flow:failed,calls:blocked}=makeFlow(createP10Flow,{owner:{state:'verified_active',ledger_hash:'sha256:exact'},disable:{state:'disabled',baseline_restored:false}});
  await assert.rejects(failed.disable(),/не подтвердил восстановление/);
  assert.deepEqual(blocked.map(x=>x.endpoint),['keenetic/managed-ownership/status','keenetic/durable/disable']);
});

test('P10 backend capacity, capability and no implicit startup remain hard guards',()=>{
  const n=read('modules/dns-router-runtime/transaction_n.go'),o2=read('modules/dns-router-runtime/managed_o2.go');
  const o3=read('modules/dns-router-runtime/managed_o3.go'),listener=read('modules/dns-router-runtime/listener_l.go');
  const main=read('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go');
  const api=read('modules/dns-router-runtime/api.go');
  const init=read('modules/dns-router-runtime/packaging/S95routerforge-dns-router-dev');
  assert.match(n,/const managedMaxZonesN = 8/);
  assert.match(o2,/MaxZones: 63/);
  assert.match(o3,/managedZonesLimitedN\(cfg, 63\)/);
  assert.match(listener,/BuildDomainSplitPlanLWithLimit\(cfg, 32\)/);
  assert.match(main,/owned\.State == "active"/);
  assert.match(api,/LifetimeEnabled/);
  assert.match(api,/ArmLifetimeO3B/);
  assert.match(init,/--service-control "\$operation"/);
  assert.doesNotMatch(init,/--ndmc-durable-experimental/);
});
