import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
import {memoryReadPath,memoryClearBody,memoryProject} from './NfqwsTargetMemoryModel.js';
const source=name=>readFileSync(new URL('./'+name,import.meta.url),'utf8');
test('read-only memory path accepts only validated optional filter',()=>{
 assert.equal(memoryReadPath(),'/v2/memory');
 assert.equal(memoryReadPath('example.com'),'/v2/memory?target=example.com');
 for(const v of ['https://evil.test','foo\nbar','foo?x=1','x'.repeat(254)])assert.throws(()=>memoryReadPath(v));
});
test('clear requires target or explicit all and exact backend token',()=>{
 assert.throws(()=>memoryClearBody(''));
 assert.deepEqual(memoryClearBody('example.com'),{target:'example.com',confirm:'ROUTERFORGE_V2_MEMORY_CLEAR'});
 assert.deepEqual(memoryClearBody('',true),{target:'',confirm:'ROUTERFORGE_V2_MEMORY_CLEAR'});
 assert.throws(()=>memoryClearBody('example.com\n'));
});
test('memory projection keeps policy/read-only evidence, no source mutation',()=>{
 const d={entries:[{target:'a.example',verified_count:2,reuse_eligible:true,args:['--x','<tag>'],success_rate:.75}],policy:{working_max_age_seconds:3600,strong_min_verified:2}};
 const before=JSON.stringify(d),v=memoryProject(d);
 assert.equal(JSON.stringify(d),before);assert.equal(v.rows[0].rate,75);assert.equal(v.rows[0].reuse,true);assert.match(v.policy,/working TTL 1h/);assert.deepEqual(v.rows[0].args,['--x','<tag>']);
 assert.throws(()=>memoryProject({entries:{}}));
});
test('Svelte owns memory list and two-phase clear; old DOM writer removed',()=>{
 const app=source('NFQWSManagerApp.svelte'),ui=source('NfqwsTargetMemory.svelte'),legacy=source('NfqwsRuntime.js');
 assert.match(app,/<NfqwsTargetMemory active=/);assert.match(ui,/data-svelte-owned="nfqws-target-memory-v2"/);
 assert.doesNotMatch(legacy,/function renderV2TargetMemory\(|function clearV2TargetMemory\(|\$\('v2MemoryRefresh'\)\.onclick/);
 assert.match(ui,/function stageClear\(/);assert.match(ui,/function confirmClear\(/);
 assert.doesNotMatch(ui,/{@html|\.innerHTML|localStorage/);
});
