import test from 'node:test';
import assert from 'node:assert/strict';
import {customSnapshot,editCustom,serializeCustom,customSummaryTokens} from './NfqwsCustomStrategiesModel.js';
const initial=()=>customSnapshot(['--filter-tcp=443 --dpi-desync=fake','--filter-udp=443 --hostlist=user.list'],['Main','QUIC']);
test('custom strategies add/edit only in local Svelte snapshot, unchanged original',()=>{
 const src=initial();const added=editCustom(src,'add'); assert.equal(added.blocks.length,3);assert.deepEqual(src,initial());
 const changed=editCustom(added,'text',2,'--filter-tcp=80'); assert.equal(changed.blocks[2],'--filter-tcp=80');assert.equal(added.blocks[2],'--filter-tcp=443 --dpi-desync=fake');
});
test('move keeps each label coupled to its strategy and preserves serialization order',()=>{
 const src=initial(), next=editCustom(src,'down',0); assert.deepEqual(next.labels,['QUIC','Main']); assert.equal(next.blocks[1],src.blocks[0]); assert.ok(serializeCustom(next).indexOf('filter-udp')<serializeCustom(next).indexOf('filter-tcp'));
});
test('clone and remove are local and retain labels',()=>{
 const src=initial();const clone=editCustom(src,'clone',0); assert.equal(clone.labels[1],'Main — копия'); assert.deepEqual(editCustom(clone,'remove',1),src);
});
test('invalid and stale indexes fail closed',()=>{
 for(const command of ['text','remove','clone','up','down']) assert.throws(()=>editCustom(initial(),command,77),/STALE_INDEX/);
 assert.throws(()=>editCustom(initial(),'up',0),/OUT_OF_RANGE/);
 assert.throws(()=>editCustom(initial(),'down',1),/OUT_OF_RANGE/);
});
test('labels sanitize control chars and never appear in NFQWS argv',()=>{
 const s=editCustom(initial(),'label',0,'Hello\r\n --new');assert.equal(s.labels[0],'Hello  --new');assert.ok(!serializeCustom(s).includes('Hello'));
});
test('custom summary renders plain tokens (never HTML)',()=>{
 const src='<img onerror=alert(1)>'; const tokens=customSummaryTokens(`--filter-tcp=443 --hostlist=${src}`); assert.equal(tokens[1].value,'<img');assert.equal(tokens[0].value,'443');
});
test('duplicate labels, empty strategies and literal --new respect existing serialization contract',()=>{
 const s=customSnapshot(['  --filter-tcp=443  ',' ', '--dpi-desync=fake'],['x','x','z']);assert.equal(serializeCustom(s),'--filter-tcp=443\n--new\n--dpi-desync=fake');
});
test('malformed state is refused without network or mutation',()=>{
 assert.throws(()=>customSnapshot(['x'],[]),/SNAPSHOT_INVALID/);
 assert.throws(()=>customSnapshot(['x'],[42]),/SNAPSHOT_INVALID/);
 assert.throws(()=>editCustom(initial(),'unknown',0),/COMMAND_UNKNOWN/);
});
