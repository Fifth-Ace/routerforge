import test from 'node:test';
import assert from 'node:assert/strict';
import {visibleLiteLogs,qtypeName,liteLogEndpoint} from './DnsLiteLogModel.js';
const upstreams=[{id:'cloud',transport:'doh',address:'203.0.113.1:443',server_name:'example.org'}];
const rows=[
  {type:1,name:'example.org',client:'192.0.2.2',assigned:'cloud',used:'cloud',success:true},
  {type:255,name:'blocked.example',client:'192.0.2.3',assigned:'client-acl',used:'client-acl',success:false},
  {type:28,name:'other.example',client:'192.0.2.4',assigned:'cloud',used:'cloud',success:true,used_endpoint:'remote:853'}
];
test('log filter works by name, client and resolved DNS endpoint',()=>{
  assert.equal(visibleLiteLogs(rows,{filter:'EXAMPLE.ORG'},upstreams).length,2); // both use this upstream
  assert.equal(visibleLiteLogs(rows,{filter:'192.0.2.3'},upstreams).length,1);
  assert.equal(visibleLiteLogs(rows,{filter:'remote:853'},upstreams).length,1);
  assert.equal(visibleLiteLogs(rows,{filter:'Кэш'},upstreams).length,0);
});
test('query type, refusal and error-only filters share one Svelte model',()=>{
  assert.equal(visibleLiteLogs(rows,{type:'ANY',errorsOnly:true},upstreams).length,1);
  assert.equal(visibleLiteLogs(rows,{type:'ANY',errorsOnly:false},upstreams).length,1);
  assert.equal(visibleLiteLogs(rows,{type:'A',errorsOnly:true},upstreams).length,0);
  assert.equal(qtypeName(255),'ANY');assert.equal(qtypeName(28),'AAAA');
});
test('logs are immutable and reverse chronological like live view',()=>{
  const before=structuredClone(rows);
  const got=visibleLiteLogs(rows,{},upstreams);
  assert.equal(got[0].name,'other.example');assert.deepEqual(rows,before);
});
test('endpoint labels fail closed and never assume TLS encryption',()=>{
  assert.match(liteLogEndpoint(rows[0],'used',upstreams),/doh/);
  assert.equal(liteLogEndpoint(rows[1],'used',upstreams),'Отказ ACL');
  assert.equal(liteLogEndpoint({cache:true},'used',upstreams),'Кэш · локально');
});
