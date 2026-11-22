// Behavioural model/API checks for the R15->native Runtime replacement.
// These run without a browser. Chromium/component/compiled-IPK remain separate gates.
import {test, beforeEach} from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath,pathToFileURL} from 'node:url';
import path from 'node:path';
const here=path.dirname(fileURLToPath(import.meta.url));
const root=path.resolve(here,'../modules/dns-router-runtime/frontend');
// Node-only shim: the real build compiles $state with Svelte 5.
globalThis.$state=value=>value;
const store=await import(pathToFileURL(path.join(root,'DnsRuntimeStore.svelte.js')).href);
const {apiRequest}=await import(pathToFileURL(path.join(root,'DnsRuntimeApi.js')).href);
const {createConfigFlow}=await import(pathToFileURL(path.join(root,'DnsRuntimeConfigLogic.js')).href);
const {entries,acceptsDomains}=await import(pathToFileURL(path.join(root,'DnsRuntimeNativeCatalog.js')).href);
const {configFlow,scanNative,stopRuntimeRequests}=await import(pathToFileURL(path.join(root,'DnsRuntimeCommands.js')).href);
const {draft,simple,readDraft,commitDraft,loadConfig,markDirty,removeRow,stageDomains,refreshChoices,acceptScan,editDNS,cancelEditing}=store;
const config={schema_version:1,upstreams:[{id:'u1',transport:'dot',address:'9.9.9.9:853'}],rules:[]};
const clone=x=>JSON.parse(JSON.stringify(x));
function reset(){
  draft.config=clone(config);draft.revision='r0';draft.dirty=false;draft.generation=0;draft.loaded=true;
  draft.feedback='';draft.edit={scope:'',item:null,version:0};
  simple.inventory=null;simple.catalog=null;simple.policy='System';simple.selected='manual:u1';
  simple.domains='';simple.editingId=null;simple.notice='';simple.status='';
}
function flow(request,confirmAction=()=>true){return createConfigFlow({readDraft,request,commit:commitDraft,confirmAction,notify:()=>{}});}
function json(body,status=200,ctype='application/json'){
  return new Response(JSON.stringify(body),{status,headers:{'content-type':ctype}});
}
beforeEach(()=>{reset();globalThis.window={confirm:()=>true};});
test('module evaluation has no implicit HTTP mutation',()=>{
  assert.equal(typeof configFlow.save,'function');assert.equal(typeof apiRequest,'function');
});
test('snapshot is independent of reactive draft',()=>{
  const s=readDraft();s.config.upstreams[0].id='external';assert.equal(draft.config.upstreams[0].id,'u1');
});
test('load starts with read-only GET and accepts revision',async()=>{
  draft.revision='';draft.loaded=false;
  const calls=[];
  await loadConfig(async(p,options)=>{calls.push([p,options]);return {config:clone(config),revision:'new'};});
  assert.deepEqual(calls,[['config',undefined]]);assert.equal(draft.revision,'new');assert.equal(draft.dirty,false);
});
test('slow initial load never overwrites later editor changes',async()=>{
  let resume;
  const task=loadConfig(()=>new Promise(r=>{resume=r}));
  markDirty();resume({config:clone(config),revision:'new'});
  await assert.rejects(task,/изменён во время загрузки/);
  assert.equal(draft.revision,'r0');assert.equal(draft.dirty,true);
});
test('CAS save includes If-Match, Content-Type and explicit action',async()=>{
  let sent;
  await flow(async(p,o)=>{sent={p,o};return {revision:'r1'};}).save();
  assert.equal(sent.p,'config');assert.equal(sent.o.method,'PUT');
  assert.equal(sent.o.headers['If-Match'],'"r0"');
  assert.equal(sent.o.headers['X-RouterForge-Action'],'save');
  assert.equal(draft.revision,'r1');assert.equal(draft.dirty,false);
});
test('in-flight save does NOT erase a newer dirty draft',async()=>{
  let resume;
  const task=flow(()=>new Promise(r=>{resume=r})).save();
  const generation=draft.generation;
  commitDraft('edited',{previousRevision:'r0',generation,config:{...clone(config),rules:[{id:'a',domain:'example.com',match:'suffix',upstream:'u1',priority:100}]}});
  resume({revision:'r1'});
  assert.deepEqual(await task,{stale:true});
  assert.equal(draft.revision,'r1');assert.equal(draft.dirty,true);
  assert.equal(draft.config.rules.length,1);
});
test('a 409 conflict retains draft revision, data and dirty bit',async()=>{
  markDirty();const old=readDraft();
  await assert.rejects(flow(async()=>{throw Error('HTTP 409: conflict');}).save(),/409/);
  assert.deepEqual(readDraft(),old);
});
test('a 403 permission denial never commits or clears dirty',async()=>{
  markDirty();const old=readDraft();
  await assert.rejects(flow(async()=>{throw Error('HTTP 403: denied');}).save(),/403/);
  assert.deepEqual(readDraft(),old);
});
test('empty revision from save is hard failure',async()=>{
  markDirty();await assert.rejects(flow(async()=>({revision:''})).save(),/не подтвердил ревизию/);
  assert.equal(draft.dirty,true);assert.equal(draft.revision,'r0');
});
test('double click save: only one HTTP operation is permitted',async()=>{
  let resume;let calls=0;
  const f=flow(()=>{calls++;return new Promise(r=>{resume=r});});
  const pending=f.save();
  await assert.rejects(f.save(),/Другая операция/);
  assert.equal(calls,1);
  resume({revision:'r1'});await pending;
});
test('reload refuses to overwrite dirty without explicit confirmation',async()=>{
  markDirty();let requests=0;
  const answer=await flow(async()=>{requests++;},()=>false).reload();
  assert.deepEqual(answer,{cancelled:true});assert.equal(requests,0);
});
test('reload detects an edit made during its GET',async()=>{
  let resume;const pending=flow(()=>new Promise(r=>{resume=r})).reload();
  markDirty();resume({config:clone(config),revision:'r1'});
  await assert.rejects(pending,/изменён во время загрузки/);
  assert.equal(draft.dirty,true);
});
test('import preview requires matching base revision, no local apply',async()=>{
  const fields={text:'example.com',upstream:'u1',match:'suffix',mode:'append'};
  const p=flow(async()=>({base_revision:'stale',preview:{config:clone(config),added:1,duplicates:0,ignored:0,result_rules:1}}));
  await assert.rejects(p.previewImport(fields),/Конфигурация изменилась/);
  assert.equal(draft.dirty,false);
});
test('import confirmation cancelled leaves draft untouched',async()=>{
  const before=readDraft();
  const f=flow(async()=>({base_revision:'r0',preview:{config:clone(config),added:1,duplicates:0,ignored:0,result_rules:1}}),()=>false);
  const result=await f.previewImport({text:'example.com',upstream:'u1'});
  assert.equal(result.staged,false);assert.deepEqual(readDraft(),before);
});
test('protected upstream cannot be removed while a rule references it',()=>{
  draft.config.rules=[{id:'one',domain:'example.com',upstream:'u1'}];
  assert.throws(()=>removeRow('upstream',0),/Нельзя удалить используемый пул/);
  assert.equal(draft.config.upstreams.length,1);
});
test('protected upstream cannot be removed while Lite default references it',()=>{
  draft.config.service={default_upstream:'u1'};
  assert.throws(()=>removeRow('upstream',0),/Нельзя удалить используемый пул/);
});
test('simple-domain staging changes draft only, not network',()=>{
  simple.selected='manual:u1';simple.domains='example.com\ntest.example.com';
  assert.equal(stageDomains(),true);
  assert.equal(draft.config.rules.length,2);assert.equal(draft.dirty,true);
  assert.equal(simple.domains,'');
});
test('duplicate domain is rejected without changing draft',()=>{
  simple.selected='manual:u1';simple.domains='example.com';stageDomains();
  const before=readDraft();simple.domains='example.com';
  assert.throws(()=>stageDomains(),/уже есть в правилах/);
  assert.deepEqual(readDraft(),before);
});
test('group edit cancel does not change persisted draft',()=>{
  simple.selected='manual:u1';simple.domains='example.com';stageDomains();
  const before=readDraft();editDNS('u1');
  assert.equal(simple.editingId,'u1');cancelEditing();
  assert.deepEqual(readDraft(),before);
});
test('native zone selector never accepts an unrelated domain',()=>{
  assert.equal(acceptsDomains({usable:true,zone:'ru'},['example.com']).ok,false);
  assert.equal(acceptsDomains({usable:true,zone:'ru'},['example.ru']).ok,true);
});
test('native inventory does not treat unproven encryption as proven',()=>{
  const inventory={available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[{domain:'.',upstream_hint:'quad9',address:'127.0.0.1:40500'}]}]};
  const list=entries(inventory,'System');assert.equal(list.length,1);
  assert.equal(list[0].usable,true);assert.equal(list[0].encryptionProven,false);
});
test('untrusted API path rejected before network',async()=>{
  globalThis.fetch=()=>{throw Error('SHOULD_NOT_FETCH');};
  await assert.rejects(apiRequest('https://evil.example/config'),/PATH_INVALID/);
  await assert.rejects(apiRequest('../config'),/PATH_INVALID/);
});
test('API 403/409 preserves status and server reason',async()=>{
  globalThis.fetch=async()=>json({error:'stale'},409);
  await assert.rejects(apiRequest('config'),/HTTP 409: stale/);
  globalThis.fetch=async()=>json({error:'permission denied'},403);
  await assert.rejects(apiRequest('config'),/HTTP 403: permission denied/);
});
test('API invalid JSON hard fails, not successful action',async()=>{
  globalThis.fetch=async()=>new Response('{invalid',{status:200,headers:{'content-type':'application/json'}});
  await assert.rejects(apiRequest('config'),/Некорректный JSON/);
});
test('API 404 allowlist is explicitly opt-in',async()=>{
  globalThis.fetch=async()=>json({error:'not found'},404);
  await assert.rejects(apiRequest('lite/status'),/HTTP 404/);
  assert.equal(await apiRequest('lite/status',{allow404:true}),null);
});
test('API cancellation propagates and never retries',async()=>{
  let calls=0;globalThis.fetch=async(_url,options)=>{calls++;throw options.signal.reason;};
  const controller=new AbortController();controller.abort(new Error('cancelled'));
  await assert.rejects(apiRequest('config',{signal:controller.signal}),/cancelled/);
  assert.equal(calls,1);
});

// Lifecycle regression: a component can unmount while read-only requests are
// in-flight. No response may commit to shared Svelte state after that point.
test('unmount cancels initial GET without committing late server revision',async()=>{
  let resolve;
  const before=readDraft();
  const controller=new AbortController();
  const pending=loadConfig(()=>new Promise(r=>{resolve=r;}),controller.signal);
  controller.abort();
  resolve({config:clone(config),revision:'late-server-revision'});
  assert.deepEqual(await pending,{cancelled:true});
  assert.deepEqual(readDraft(),before);
});
test('already-aborted mount cannot issue any initial config GET',async()=>{
  let calls=0;
  const controller=new AbortController();controller.abort();
  assert.deepEqual(await loadConfig(()=>{calls++;},controller.signal),{cancelled:true});
  assert.equal(calls,0);
});
test('unmount invalidates native scan; remount starts immediately; stale result ignored',async()=>{
  stopRuntimeRequests();
  const initial=[];
  let startNew=false;
  const dnsResult={available:true,instances:[{name:'System',valid:true,endpoints:[]}]};
  const policiesResult={available:true,policies:[{name:'System',description:'Current'}]};
  globalThis.fetch=async url=>{
    const path=String(url);
    if(startNew)return json(path.includes('native-dns')?dnsResult:policiesResult);
    return new Promise(resolve=>initial.push(()=>resolve(json({available:false}))));
  };
  const first=scanNative();
  assert.equal(initial.length,2);
  const controller=new AbortController();controller.abort();
  stopRuntimeRequests();
  startNew=true;
  await scanNative();
  assert.equal(simple.inventory?.available,true);
  assert.equal(simple.catalog?.policies?.[0]?.description,'Current');
  initial.forEach(fn=>fn());
  await first;
  assert.equal(simple.inventory?.available,true);
  assert.equal(simple.catalog?.policies?.[0]?.description,'Current');
  stopRuntimeRequests();
});
