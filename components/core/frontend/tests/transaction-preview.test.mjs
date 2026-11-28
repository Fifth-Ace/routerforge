import test from 'node:test';
import assert from 'node:assert/strict';
import { previewSelection, selectionKey, previewMatches, transactionProgress, executionRequest, updateRows, updateRunning } from '../src/lib/transaction-preview.js';
const commit = 'a'.repeat(40);
const row = { package: 'routerforge-core', installed_version: 'old', target_version: 'new' };
test('only server supplied exact release selection', () => {
  assert.deepEqual(previewSelection({release_commit:commit,eligible_packages:[row,row,{package:'foreign'}]}),[row]);
  assert.deepEqual(previewSelection({release_commit:'wrong',eligible_packages:[row]}),[]);
  assert.notEqual(selectionKey({release_commit:commit,eligible_packages:[row]}),selectionKey({release_commit:commit,eligible_packages:[{...row,installed_version:'changed'}]}));
});
test('preview is tied to selection and release; admission is checked separately', () => {
  const p={execution_available:false,plan:{commit,channel:'dev',target:'aarch64-3.10',packages:[{name:row.package}]},plan_sha256:'b'.repeat(64)};
  assert.equal(previewMatches(p,commit,[row.package]),true);
  assert.equal(previewMatches(p,'c'.repeat(40),[row.package]),false);
  assert.equal(previewMatches({...p,plan:{...p.plan,target:'mips-3.4'}},commit,[row.package]),false);
  assert.equal(previewMatches({...p,plan:{...p.plan,packages:{}}},commit,[row.package]),false);
  assert.equal(previewMatches(p,commit,[]),false);
  assert.equal(previewMatches({...p,execution_available:true},commit,[row.package]),true);
});
test('journal progress is verified packages, not speculative percent', () => {
  assert.deepEqual(transactionProgress({plan:{packages:[1,2]},journal:{next:1}}),{verified:1,total:2});
  assert.equal(transactionProgress({plan:{packages:[1]},journal:{next:2}}),null);
  assert.equal(transactionProgress({}),null);
});

test('execution requires current admitted plan and exact confirmation', () => {
  const snapshot={release_commit:commit,eligible_packages:[row],execution_available:true,slot:{state:'idle'}};
  const preview={execution_available:true,plan:{id:'1'.repeat(32),commit,channel:'dev',target:'aarch64-3.10',packages:[{name:row.package,version:row.target_version}]},plan_sha256:'b'.repeat(64),management:[{package:row.package,state:'management_ready'}]};
  assert.deepEqual(executionRequest(snapshot,preview,[row.package]),{id:preview.plan.id,expected_commit:commit,packages:[row.package],plan_sha256:preview.plan_sha256,confirm:'EXECUTE:'+preview.plan_sha256});
  for(const s of [{...snapshot,execution_available:false},{...snapshot,slot:{state:'applying'}},{...snapshot,slot:{state:'completed',busy:true}},{...snapshot,release_commit:'c'.repeat(40)},{...snapshot,eligible_packages:[]}]) assert.equal(executionRequest(s,preview,[row.package]),null);
  for(const p of [{...preview,execution_available:false},{...preview,plan:{...preview.plan,id:'bad'}}]) assert.equal(executionRequest(snapshot,p,[row.package]),null);
});

test('manual new plan after failure does not require management health',()=>{
 const snapshot={release_commit:commit,eligible_packages:[row],execution_available:true,slot:{state:'recovery_required'}};
 const preview={execution_available:true,plan:{id:'1'.repeat(32),commit,channel:'dev',target:'aarch64-3.10',packages:[{name:row.package,version:row.target_version}]},plan_sha256:'b'.repeat(64)};
 assert.ok(executionRequest(snapshot,preview,[row.package]));
 const slot={state:'preparing',plan:{packages:[{name:'a'},{name:'b'}]},journal:{downloaded:1,next:0}};
 assert.deepEqual(updateRows(slot).map(p=>p.state),['downloaded','downloading']);assert.ok(updateRunning(slot));
 assert.deepEqual(updateRows({...slot,state:'verifying',journal:{next:1}}).map(p=>p.state),['completed','verifying']);
 assert.equal(updateRunning({...slot,state:'recovery_required'}),false);
});

test('parallel download progress counts real bytes not queue positions',async()=>{
 const {downloadStats,progressRows}=await import('../src/lib/transaction-preview.js');
 const slot={state:'preparing',plan:{id:'job',packages:[{name:'a',size:100},{name:'b',size:200},{name:'c',size:100}]},journal:{next:0,downloaded:0}};
 const reports=[{id:'job',name:'b',bytes:200,state:'downloaded'},{id:'job',name:'c',bytes:50,state:'downloading',rate:20},{id:'old',name:'a',bytes:100,state:'downloaded'}];
 const s=downloadStats(slot,reports);
 assert.equal(s.bytes,250);assert.equal(s.percent,62);assert.equal(s.rate,20);
 assert.deepEqual(progressRows(slot,reports).map(x=>x.state),['queued','downloaded','downloading']);
 assert.equal(downloadStats({...slot,state:'applying'},reports).percent,100);
});
