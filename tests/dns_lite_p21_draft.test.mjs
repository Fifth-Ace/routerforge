import test from 'node:test';
import assert from 'node:assert/strict';
import {
  cleanP21MAC,draftDevice,draftGroup,draftRule,
  dropDeviceOverrides,dropGroup,dropRule,aggregateP21Devices
} from '../modules/dns-router-lite/frontend/DnsP21Draft.js';
const mac='02:11:22:33:44:55',mac2='02:11:22:33:44:66';
const dns={upstream:'cf'};
const seed=()=>({devices:[{mac,name:'Laptop',target:dns,inherit_domains:true}],groups:[{id:'g1',name:'Office',macs:[mac],target:dns}],rules:[{id:'r1',mac,domain:'example.com',match:'suffix',priority:100,target:dns}]});
const copy=a=>structuredClone(a);

test('P21 rejects multicast, zero and malformed MAC; normalizes canonical MAC',()=>{
 for(const bad of ['','abc','00:00:00:00:00:00','01:11:22:33:44:55','gg:11:22:33:44:55'])assert.throws(()=>cleanP21MAC(bad),/Invalid MAC/);
 assert.equal(cleanP21MAC(' 02:11:22:33:44:AA '),'02:11:22:33:44:aa');
});
test('P21 device update is draft-only, pure and preserves other settings',()=>{
 const original=seed(),prior=copy(original);
 const output=draftDevice(original,{mac,name:'  Laptop 2  ',target:null});
 assert.deepEqual(original,prior);
 assert.equal(output.devices[0].name,'Laptop 2');
 assert.equal('target' in output.devices[0],false);
 assert.deepEqual(output.groups,prior.groups);
});
test('P21 invalid device rejects without draft creation or mutation',()=>{
 const input=seed(),original=copy(input);
 assert.throws(()=>draftDevice(input,{mac:'bad',name:'Broken'},'Invalid device'),/Invalid device/);
 assert.deepEqual(input,original);
});
test('P21 group members normalize and deduplicate; no implicit apply',()=>{
 const input=seed(),before=copy(input);
 const next=draftGroup(input,{id:'g1',name:' Office ',members:`${mac}\n${mac}\n${mac2}`,target:dns,inherit_domains:true});
 assert.deepEqual(next.groups[0].macs,[mac,mac2]);
 assert.deepEqual(input,before);
});
test('P21 group collision rejects atomically',()=>{
 const input=seed();input.groups.push({id:'g2',name:'Other',macs:[mac2]});const before=copy(input);
 assert.throws(()=>draftGroup(input,{id:'g1',name:'New',members:mac2},{conflict:'duplicate member'}),/duplicate member/);
 assert.deepEqual(input,before);
});
test('P21 malformed member rejects atomically',()=>{
 const input=seed(),before=copy(input);
 assert.throws(()=>draftGroup(input,{id:'g1',name:'New',members:'not-mac'},{invalidMAC:'Bad MAC'}),/Bad MAC/);
 assert.deepEqual(input,before);
});
test('P21 blank group name cannot modify state',()=>{
 const input=seed(),before=copy(input);
 assert.throws(()=>draftGroup(input,{id:'g1',name:' ',members:mac},{required:'Name required'}),/Name required/);
 assert.deepEqual(input,before);
});
test('P21 rule rejects bad priority/empty target and preserves draft',()=>{
 const input=seed(),before=copy(input);
 for(const [priority,target] of [[-1,dns],[1000001,dns],[2.5,dns],[100,null]])
  assert.throws(()=>draftRule(input,{mac,domains:['example.com'],priority,target,match:'suffix'}),/Invalid rule/);
 assert.deepEqual(input,before);
});
test('P21 rule creates stable ID and updates existing rule without duplication',()=>{
 const input=seed();const next=draftRule(input,{mac,domains:['example.com'],target:dns,priority:'42',match:'suffix'});
 assert.equal(next.rules.length,1);assert.equal(next.rules[0].id,'r1');assert.equal(next.rules[0].priority,42);
 const add=draftRule(next,{mac:mac2,domains:['hello.test'],target:dns,priority:1,match:'exact'});
 assert.equal(add.rules.length,2);assert.equal(add.rules[1].id,'device-domain-1');
});
test('P21 reset removes local device overrides, group membership and device rules only',()=>{
 const input=seed(),before=copy(input);const next=dropDeviceOverrides(input,mac);
 assert.deepEqual(input,before);assert.equal(next.devices[0].target,undefined);
 assert.deepEqual(next.groups[0].macs,[]);assert.equal(next.rules.length,0);
});
test('P21 deleting rule or group cannot mutate input or other arrays',()=>{
 const input=seed(),before=copy(input);
 const group=dropGroup(input,'g1'),rule=dropRule(input,'r1');
 assert.equal(group.groups.length,0);assert.equal(rule.rules.length,0);
 assert.deepEqual(group.rules,input.rules);assert.deepEqual(rule.groups,input.groups);assert.deepEqual(input,before);
});
test('P21 inventory aggregates duplicates once and includes draft-only devices',()=>{
 const output=aggregateP21Devices([{mac,name:'Router name',ip:'192.168.10.1',state:'REACHABLE',trusted:true},{mac,name:'Other',ip:'192.168.10.2',state:'STALE',trusted:false}],{devices:[{mac:mac2,name:'Offline'}],groups:[],rules:[]});
 assert.equal(output.length,2);assert.deepEqual(output[0].ips,['192.168.10.1 · REACHABLE','192.168.10.2 · STALE']);
 assert.equal(output[0].trusted,true);assert.equal(output[1].name,'Offline');
});
