import test from 'node:test';
import assert from 'node:assert/strict';
import { previewSelection, selectionKey, previewMatches, transactionProgress } from '../src/lib/transaction-preview.js';
const commit = 'a'.repeat(40);
const row = { package: 'routerforge-core', installed_version: 'old', target_version: 'new' };
test('only server supplied exact release selection', () => {
  assert.deepEqual(previewSelection({release_commit:commit,eligible_packages:[row,row,{package:'foreign'}]}),[row]);
  assert.deepEqual(previewSelection({release_commit:'wrong',eligible_packages:[row]}),[]);
  assert.notEqual(selectionKey({release_commit:commit,eligible_packages:[row]}),selectionKey({release_commit:commit,eligible_packages:[{...row,installed_version:'changed'}]}));
});
test('preview is tied to selection and release; executable response is not accepted', () => {
  const p={execution_available:false,plan:{commit,channel:'dev',target:'aarch64-3.10',packages:[{name:row.package}]},plan_sha256:'b'.repeat(64)};
  assert.equal(previewMatches(p,commit,[row.package]),true);
  assert.equal(previewMatches(p,'c'.repeat(40),[row.package]),false);
  assert.equal(previewMatches({...p,plan:{...p.plan,target:'mips-3.4'}},commit,[row.package]),false);
  assert.equal(previewMatches({...p,plan:{...p.plan,packages:{}}},commit,[row.package]),false);
  assert.equal(previewMatches(p,commit,[]),false);
  assert.equal(previewMatches({...p,execution_available:true},commit,[row.package]),false);
});
test('journal progress is verified packages, not speculative percent', () => {
  assert.deepEqual(transactionProgress({plan:{packages:[1,2]},journal:{next:1}}),{verified:1,total:2});
  assert.equal(transactionProgress({plan:{packages:[1]},journal:{next:2}}),null);
  assert.equal(transactionProgress({}),null);
});
