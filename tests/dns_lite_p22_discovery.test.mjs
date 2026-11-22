import test from 'node:test';
import assert from 'node:assert/strict';
import {normalizeDiscoveredDevices} from '../modules/dns-router-lite/frontend/DnsP22Discovery.js';
const mac='02:11:22:33:44:55';
test('P22 empty or invalid discovery is read-only and fail-closed',()=>{
  for(const rows of [null,undefined,{},'text',[],[null,{}, {mac:'ff:ff:ff:ff:ff'}]])
    assert.deepEqual(normalizeDiscoveredDevices(rows),[]);
});
test('P22 canonicalizes MAC and keeps a single identity with trusted record preference',()=>{
  const rows=[{mac:mac.toUpperCase(),name:'Old',trusted:false,ip:'192.0.2.1'},
    {mac,name:'Current',trusted:true,ip:'192.0.2.2'},
    {mac,name:'Stale',trusted:false,ip:'192.0.2.3'}];
  assert.deepEqual(normalizeDiscoveredDevices(rows),[{mac,name:'Current',trusted:true,ip:'192.0.2.2'}]);
});
test('P22 never upgrades string trust evidence to verified',()=>{
  assert.equal(normalizeDiscoveredDevices([{mac,trusted:'true'}])[0].trusted,false);
});
test('P22 does not mutate raw inventory',()=>{
  const original=Object.freeze({mac,name:'Host',trusted:true});
  const rows=Object.freeze([original]);
  const value=normalizeDiscoveredDevices(rows);
  assert.notEqual(value[0],original);
  assert.deepEqual(value[0],{mac,name:'Host',ip:'',trusted:true});
});
test('P22 deterministic: trusted first and stable name order',()=>{
  const a={mac:'02:11:22:33:44:56',trusted:false,name:'Zed'};
  const b={mac:'02:11:22:33:44:57',trusted:true,name:'Bob'};
  const c={mac:'02:11:22:33:44:58',trusted:true,name:'Alice'};
  assert.deepEqual(normalizeDiscoveredDevices([a,b,c]).map(x=>x.mac),[c.mac,b.mac,a.mac]);
});
test('P22 bound on untrusted discovery input and invalid records',()=>{
  const rows=Array.from({length:5000},(_,i)=>({mac:'not-a-mac',name:String(i)}));
  rows[0]={mac,name:'Found',trusted:false};
  rows[4999]={mac:'02:11:22:33:44:56',name:'Excluded',trusted:true};
  assert.equal(normalizeDiscoveredDevices(rows).length,1);
});
