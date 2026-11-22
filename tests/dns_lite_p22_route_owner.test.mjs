import test from 'node:test';
import assert from 'node:assert/strict';
import {presentP22Route} from '../modules/dns-router-lite/frontend/DnsP22Route.js';

test('P22 route requires explicit configuration-only evidence',()=>{
  assert.equal(presentP22Route(null),null);
  assert.equal(presentP22Route({scope:'NETWORK_VERIFIED',network_verified:true}),null);
});
test('P22 route explains device, rule, DNS and reserves without claiming E2E',()=>{
  const r=presentP22Route({scope:'CONFIGURATION_ONLY',decision:{layer:'device_domain'},allowed_chain:[{id:'cf',address:'1.1.1.1'},{id:'backup',address:'9.9.9.9'}]},'02:11:22:33:44:55',[{mac:'02:11:22:33:44:55',name:'Laptop'}]);
  assert.equal(r.steps.length,4);
  assert.match(r.steps[0],/Laptop/);
  assert.match(r.steps[1],/Правило устройства/);
  assert.match(r.steps[2],/1.1.1.1/);
  assert.match(r.steps[3],/9.9.9.9/);
  assert.match(r.note,/не подтверждённый сетевой E2E/);
});
test('P22 route returns safe default for unknown identity and empty chain',()=>{
  const r=presentP22Route({scope:'CONFIGURATION_ONLY',decision:{layer:'default'},allowed_chain:[]},'02:11:22:33:44:88');
  assert.match(r.steps[0],/02:11:22:33:44:88/);
  assert.match(r.steps[2],/—/);
});
test('P22 route preserves Keenetic local namespace distinction',()=>{
  const r=presentP22Route({scope:'CONFIGURATION_ONLY',decision:{layer:'local_namespace'},allowed_chain:[]});
  assert.match(r.steps[2],/Локальный DNS Keenetic/);
});
test('P22 route localizes known evidence without converting it to network proof',()=>{
  const r=presentP22Route({scope:'CONFIGURATION_ONLY',decision:{layer:'device_group'},allowed_chain:[{server_name:'Cloudflare'}]},'',[],true);
  assert.match(r.steps[1],/Device group/);
  assert.match(r.note,/not a verified network E2E/);
});
test('P22 route never modifies source evaluation or inventory',()=>{
  const result=Object.freeze({scope:'CONFIGURATION_ONLY',decision:Object.freeze({layer:'device'}),allowed_chain:Object.freeze([Object.freeze({id:'one'})])});
  const devices=Object.freeze([Object.freeze({mac:'02:11:22:33:44:55',name:'Sample'})]);
  presentP22Route(result,devices[0].mac,devices);
  assert.equal(result.allowed_chain[0].id,'one');
  assert.equal(devices[0].name,'Sample');
});
