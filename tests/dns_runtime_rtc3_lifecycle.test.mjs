// Exact-source executable state/HTTP regression. Browser/Go/IPK remain separate gates.
import {test,beforeEach,afterEach} from 'node:test';
import assert from 'node:assert/strict';

globalThis.$state = value => value; // Node-only shim; browser gate uses actual compiled Svelte.
const root = '../modules/dns-router-runtime/frontend/';
const store = await import(root+'DnsRuntimeStore.svelte.js');
const shadowModule = await import(root+'DnsRuntimeShadowState.svelte.js');
const configModule = await import(root+'DnsRuntimeConfigLogic.js');
const {draft,readDraft,commitDraft,markDirty} = store;
const {shadow,beginShadowView,refreshShadow,controlShadow} = shadowModule;
const {createConfigFlow,invalidateConfigFlows} = configModule;
let endView=()=>{};
let requests=[];
function deferred(){let resolve,reject;const promise=new Promise((r,j)=>{resolve=r;reject=j});return {promise,resolve,reject};}
function response(data,status=200,contentType='application/json'){
  return new Response(JSON.stringify(data),{status,headers:{'content-type':contentType}});
}
function getRequestByIndex(i){return requests[i];}
function queueFetch(){
  globalThis.fetch=(path,opts={})=>{
    const d=deferred();requests.push({path,opts,d});return d.promise;
  };
}
function config(){return {schema_version:1,upstreams:[],rules:[]};}
function configFlow(request,events){return createConfigFlow({readDraft,request,commit:commitDraft,confirmAction:()=>true,notify:msg=>events.push(msg)});}
beforeEach(()=>{
  requests=[]; queueFetch(); globalThis.confirm=()=>true; invalidateConfigFlows();
  draft.config=config();draft.revision='r1';draft.dirty=false;draft.generation=0;draft.loaded=true;draft.feedback='';
  shadow.destroyed=true;shadow.busy=false;shadow.result=null;shadow.error='';shadow.status='';
});
afterEach(()=>{endView(); shadow.destroyed=true;});

test('RTC3-28 delayed shadow status from previous mount cannot overwrite remount',async()=>{
  endView=beginShadowView();
  const old=refreshShadow(); assert.equal(requests.length,1);
  assert.equal(requests[0].path,'../shadow/status');
  endView(); assert.equal(requests[0].opts.signal.aborted,true);
  endView=beginShadowView();
  const recent=refreshShadow(); assert.equal(requests.length,2);
  requests[1].d.resolve(response({lifecycle:{active:false},tag:'new'}));await recent;
  requests[0].d.resolve(response({lifecycle:{active:true},tag:'old'}));await old;
  assert.equal(shadow.result.tag,'new');assert.equal(shadow.error,'');
});

test('RTC3-28 aborted shadow GET failure after unmount must not publish feedback',async()=>{
  endView=beginShadowView();const old=refreshShadow();const previous=draft.feedback;
  endView();requests[0].d.reject(Error('simulated abort'));await old;
  assert.equal(draft.feedback,previous);assert.equal(shadow.result,null);
});

test('RTC3-24 refused confirmation means zero shadow mutations',async()=>{
  endView=beginShadowView();globalThis.confirm=()=>false;
  await controlShadow('start');assert.equal(requests.length,0);assert.equal(shadow.busy,false);
});

test('RTC3-25 shadow POST includes If-Match and explicit action; no double-click',async()=>{
  endView=beginShadowView();const pending=controlShadow('start');
  assert.equal(requests.length,1);
  assert.equal(requests[0].path,'../shadow/control');
  assert.equal(requests[0].opts.method,'POST');
  assert.equal(requests[0].opts.headers['If-Match'],'"r1"');
  assert.equal(requests[0].opts.headers['X-RouterForge-Action'],'shadow-control');
  await controlShadow('start');assert.equal(requests.length,1);
  requests[0].d.resolve(response({outcome:'started'}));
  // Success causes a read-only status refresh; never a repeat POST.
  await new Promise(r=>setImmediate(r)); assert.equal(requests.length,2);
  assert.equal(requests[1].path,'../shadow/status');
  requests[1].d.resolve(response({lifecycle:{active:true}})); await pending;
  assert.equal(shadow.busy,false);assert.equal(shadow.result.lifecycle.active,true);
  assert.match(draft.feedback,/started/);
});

test('RTC3-26 shadow denied (403) never reports success, refreshes status',async()=>{
  endView=beginShadowView(); const pending=controlShadow('apply');
  requests[0].d.resolve(response({error:'forbidden'},403));
  await new Promise(r=>setImmediate(r));assert.equal(requests.length,2);
  requests[1].d.resolve(response({lifecycle:{active:false}}));await pending;
  assert.match(draft.feedback,/HTTP 403/);assert.equal(shadow.result.lifecycle.active,false);
});

test('RTC3-27 POST completing after teardown does not publish stale success',async()=>{
  endView=beginShadowView();const pending=controlShadow('start');
  endView();endView=beginShadowView();
  const fresh=refreshShadow();
  requests[1].d.resolve(response({lifecycle:{active:false}}));await fresh;
  requests[0].d.resolve(response({outcome:'started'}));
  await new Promise(r=>setImmediate(r));assert.equal(requests.length,3);
  assert.equal(requests[2].path,'../shadow/status');
  requests[2].d.resolve(response({lifecycle:{active:true}}));await pending;
  assert.doesNotMatch(draft.feedback,/started/);
  assert.equal(shadow.result.lifecycle.active,true);
  assert.equal(requests.filter(x=>x.opts.method==='POST').length,1);
});

test('RTC3-29 root unmount invalidates a pending CAS save; never commits stale revision',async()=>{
  const events=[];let d=deferred();let commits=0;
  const original=readDraft();
  const f=createConfigFlow({readDraft,request:()=>d.promise,commit:(kind,payload)=>{commits++;return commitDraft(kind,payload)},confirmAction:()=>true,notify:msg=>events.push(msg)});
  const task=f.save(); invalidateConfigFlows();
  d.resolve({revision:'r2'});
  await assert.rejects(task,/после закрытия интерфейса/);
  assert.equal(commits,0);assert.deepEqual(readDraft(),original);assert.deepEqual(events,[]);
});

test('RTC3-17 pending reload after unmount never changes a new mount draft',async()=>{
  const events=[];let d=deferred();const f=configFlow(()=>d.promise,events);
  const task=f.reload();invalidateConfigFlows();markDirty();
  d.resolve({revision:'r2',config:config()});
  await assert.rejects(task,/после закрытия интерфейса/);
  assert.equal(draft.dirty,true);assert.equal(draft.revision,'r1');assert.deepEqual(events,[]);
});

test('RTC3-19 pending import preview after unmount never stages a change',async()=>{
  const events=[];let d=deferred();const f=configFlow(()=>d.promise,events);
  const task=f.previewImport({text:'example.org',upstream:'u1'});
  invalidateConfigFlows();d.resolve({base_revision:'r1',preview:{config:config(),added:1,duplicates:0,ignored:0,result_rules:1}});
  await assert.rejects(task,/после закрытия интерфейса/);
  assert.equal(draft.dirty,false);assert.deepEqual(events,[]);
});
