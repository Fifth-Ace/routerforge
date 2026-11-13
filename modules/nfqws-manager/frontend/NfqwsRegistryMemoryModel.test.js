import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {registryMemoryRoute,registryPaths,registryPct,tcp16Projection,registryProjection,registryFiltered,registryDetails} from './NfqwsRegistryMemoryModel.js';
const files=['NfqwsTCP16Memory.svelte','NfqwsStrategyRegistry.svelte','NfqwsRuntime.js','NFQWSManagerApp.svelte'].map(name=>readFileSync(new URL(name,import.meta.url),'utf8'));
const [memory,registry,legacy,app]=files;
test('registry/memory only allow 6 exact read-only endpoints',()=>{
  assert.equal(registryPaths().length,5);
  for(const path of [...registryPaths(),'/v2/tcp16-memory'])assert.equal(registryMemoryRoute(path,'GET'),path);
  for(const method of ['POST','DELETE','PATCH'])assert.throws(()=>registryMemoryRoute('/v2/strategy-registry',method),/DENIED/);
  assert.throws(()=>registryMemoryRoute('https://untrusted.test'),/DENIED/);
});
test('tcp16 projection preserves suspected vs clear, counts and original text',()=>{
  const raw={count:2,suspected:1,clear:1,working_sni:1,entries:[{asn:64500,provider:'Test',cutoff_16k_suspected:true,working_sni:'a.example.com',last_target:'x.test',confidence:'HIGH',suspected_count:3,clear_count:0},{cidr:'192.0.2.0/24',cutoff_16k_suspected:false}]};
  const out=tcp16Projection(raw);assert.equal(out.count,2);assert.equal(out.rows[0].network,'AS64500 · Test');assert.equal(out.rows[0].suspected,true);assert.equal(out.rows[1].suspected,false);assert.equal(out.rows[1].sni,'—');
  assert.deepEqual(raw.entries[0].asn,64500);assert.throws(()=>tcp16Projection({count:1}),/INVALID/);
});
test('registry joins exact fingerprints without mutating API results',()=>{
  const original={count:2,sources:['import','local'],protocols:['tcp'],entries:[{fingerprint:'same',name:'first',protocol:'tcp',sources:['local'],args:['--dpi-desync=fake'],evidence:{verified_count:3}},{fingerprint:'different',name:'second',protocol:'udp',sources:['import'],args:['--dpi-desync=split']} ]};
  const result=registryProjection(original,{entries:[{fingerprint:'same',score:{total:80}}]},{entries:[{fingerprint:'different',insight:{state:'CHECK',summary:'retest'}}]},{ready_count:1,candidates:[{fingerprint:'same',eligible:true}]},{primary_count:1,items:[{fingerprint:'same',primary:true,rank:1}]});
  assert.equal(result.count,2);assert.equal(result.verified,1);assert.equal(result.automationReady,1);assert.equal(result.recommendationTop,1);
  assert.equal(result.entries[0].score.total,80);assert.equal(result.entries[1].score,null);assert.equal(result.entries[1].insight.state,'CHECK');assert.equal(original.entries[0].score,undefined);
});
test('search, protocol and source filtering keep same semantics',()=>{
  const model={entries:[{name:'alpha',protocol:'tcp',sources:['local'],args:['--filter-tcp=443'],aliases:['fast']},{name:'beta',protocol:'udp',sources:['import'],args:['--filter-udp=443']} ]};
  assert.equal(registryFiltered(model,'fast').length,1);assert.equal(registryFiltered(model,'','udp').length,1);assert.equal(registryFiltered(model,'','','local').length,1);assert.equal(registryFiltered(model,'beta','tcp').length,0);
  assert.equal(registryPct(1.2),'100%');assert.equal(registryPct(-1),'0%');assert.equal(registryPct(.55),'55%');
});
test('registry evidence and automation stay advisory, never claim backend readiness',()=>{
  const x=registryDetails({evidence:{verified_count:0},automation:{eligible:false,reasons:['blocked']},args:['a','b'],fingerprint:'0000000000000000'});
  assert.match(x.evidence,/нет verified/);assert.match(x.automationText,/BLOCKED/);assert.equal(x.args,'a\nb');assert.equal(x.fingerprint.length,12);
});
test('read-only Svelte owns registry, memory, filters and no legacy DOM writes',()=>{
  for(const item of ['<NfqwsTCP16Memory active=','<NfqwsStrategyRegistry active=','onRegistryInvalidate={importedStrategiesChanged}'])assert.ok(app.includes(item),item);
  for(const item of ['data-svelte-owned="nfqws-tcp16-memory"','registryMemoryRoute','abort?.abort()'])assert.ok(memory.includes(item),item);
  for(const item of ['data-svelte-owned="nfqws-strategy-registry"','registryPaths()','registryFiltered','onCount(data.count)','refreshSeq'])assert.ok(registry.includes(item),item);
  for(const item of ['function renderTCP16MemoryV1(', 'function v2RegistryRender(', "$('v2RegistrySearch')", "$('v2TCP16List')"])assert.ok(!legacy.includes(item),item);
});
