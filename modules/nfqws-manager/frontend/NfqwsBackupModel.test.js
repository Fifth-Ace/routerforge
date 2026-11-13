import test from 'node:test';
import assert from 'node:assert/strict';
import {backupBytes,backupDiff,backupProjection} from './NfqwsBackupModel.js';

test('NFQWS backup view derives independent counts and search without mutating ledger',()=>{
  const files=[
    {id:'x',kind:'config',label:'Alpha',target:'/a',sha256:'abcd',state:'CHANGED',restorable:true},
    {id:'y',kind:'list',label:'Beta',target:'/b',sha256:'ffff',state:'MISSING',restorable:false},
    {id:'z',kind:'blob',label:'Gamma',target:'/c',sha256:'eeee',state:'OK',restorable:true}
  ];
  const saved=structuredClone(files);
  const p=backupProjection({files,selected:'x',safetyCount:2},'beta');
  assert.equal(p.total,3);assert.equal(p.changed,2);assert.equal(p.restorable,2);
  assert.deepEqual(p.files.map(x=>x.id),['y']);assert.equal(p.selected,'x');
  assert.deepEqual(files,saved);
});
test('NFQWS backup model retains textual diff semantics and never interpolates HTML',()=>{
  const p=backupProjection({detail:{entry:{id:'x'},current_content:'abc\n<script>',content:'abc\n&unsafe'}},'');
  assert.deepEqual(p.diff,[
    {c:'same',t:'  abc'},{c:'del',t:'- <script>'},{c:'add',t:'+ &unsafe'}]);
  assert.equal(p.isText,true);
});
test('NFQWS backup model handles missing and binary payloads without data writes',()=>{
  const p=backupProjection({detail:{entry:{id:'x',kind:'blob',size:2048}},error:'403 denied'});
  assert.equal(p.isText,false);assert.deepEqual(p.diff,[]);
  assert.equal(p.error,'403 denied');assert.equal(backupBytes(2048),'2.0 KB');
  assert.equal(backupBytes(NaN),'—');
});
test('NFQWS backup diff matches old line-indexed add/remove contract',()=>{
  assert.deepEqual(backupDiff('a\nb','a\nc'),[
    {c:'same',t:'  a'},{c:'del',t:'- b'},{c:'add',t:'+ c'}]);
});
