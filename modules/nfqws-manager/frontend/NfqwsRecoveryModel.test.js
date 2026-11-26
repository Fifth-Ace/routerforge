import test from 'node:test';
import assert from 'node:assert/strict';
import {safeListRecovery,nativeListRequired,bindingBackup,checkedRecoveryPlan,recoverReviewedList,verifiedRecoveryResult} from './NfqwsRecoveryModel.js';
const entry={id:'fixture',kind:'list',target:'/opt/etc/nfqws2/lists/custom.list',restorable:true,state:'CHANGED',sha256:'a'.repeat(64),current_sha256:'b'.repeat(64),size:12};
const plan={id:entry.id,target:'custom.list',current_sha256:entry.current_sha256,target_sha256:entry.sha256,can_commit:true,token:'c'.repeat(64),runtime:'NOT_VERIFIED',size:12};
const result={ok:true,readback_verified:true,sha256:entry.sha256,service_restarted:false,runtime:'NOT_VERIFIED',safety_backup:'preimage'};
test('scope excludes protected, binding, missing and old lists',()=>{
  assert.equal(safeListRecovery(entry),true);
  for(const name of ['user.list','RF-BINDING-X.list','custom.list-old','../custom.list','other/custom.list'])assert.equal(safeListRecovery({...entry,target:'/opt/etc/nfqws2/lists/'+name}),false);
  assert.equal(bindingBackup({...entry,target:'/opt/etc/nfqws2/lists/RF-BINDING-X.list'}),true);
  assert.equal(safeListRecovery({...entry,state:'MISSING'}),false);
  assert.equal(nativeListRequired({...entry,current_sha256:''}),true);
  assert.equal(safeListRecovery({...entry,current_sha256:''}),false);
  assert.equal(nativeListRequired({...entry,state:'CURRENT'}),true);
  assert.equal(nativeListRequired({...entry,state:'MISSING'}),false);
});
test('preview must exactly match the reviewed backup and live preimage',()=>{
  for(const delta of [{current_sha256:'d'.repeat(64)},{target:'other.list'},{id:'other'},{size:13},{can_commit:false},{token:''}])assert.throws(()=>checkedRecoveryPlan(entry,{...plan,...delta}),/MISMATCH/);
  assert.equal(checkedRecoveryPlan(entry,plan).confirm,'NFQWS_RECOVERY_LIST_SAVE');
});
test('cancel and stale selection perform no POST',async()=>{
  let calls=[];const api=async(path)=>{calls.push(path);return plan;};
  assert.equal(await recoverReviewedList(entry,api,()=>false,()=>true),null);
  assert.equal(calls.length,1);
  let active=true;
  await assert.rejects(recoverReviewedList(entry,async()=>{active=false;return plan;},()=>true,()=>active),/STALE/);
});
test('one explicit POST, exact token and verified result',async()=>{
  let calls=[];const api=async(path,opts)=>{calls.push([path,opts]);return opts?result:plan;};
  assert.equal(await recoverReviewedList(entry,api,()=>true,()=>true),'preimage');
  assert.equal(calls.length,2);assert.equal(calls[1][0],'/backups/recovery-save');
  assert.deepEqual(JSON.parse(calls[1][1].body),{id:entry.id,token:plan.token,confirm:'NFQWS_RECOVERY_LIST_SAVE'});
  assert.throws(()=>verifiedRecoveryResult(entry,{...result,readback_verified:false}),/UNCERTAIN/);
});
test('conflict never falls back or retries',async()=>{
  let calls=[];await assert.rejects(recoverReviewedList(entry,async(path,opts)=>{calls.push(path);if(opts)throw Error('409');return plan;},()=>true,()=>true),/409/);
  assert.equal(calls.length,2);assert.ok(!calls.includes('/backups/restore'));
});
