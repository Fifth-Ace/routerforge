import {test} from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {processingHostname,processingListFiles,processingSummary,processingLiveProof,processingStrategyPlan,processingFastPayload,processingReport} from './NfqwsProcessingMapModel.js';
test('domains normalized and rejected strictly',()=>{
 assert.equal(processingHostname('HTTPS://Example.COM/ok'),'example.com');
 for(const bad of ['localhost','http://user:secret@example.com/','evil.com/path','../../x','bad..example.com'])assert.equal(processingHostname(bad),'');
});
test('only real list file extensions are read',()=>assert.deepEqual(processingListFiles([{name:'user.list'},{name:'ipset.list'},{name:'passwd'},{name:'a.list-old'}]),['user.list','ipset.list','a.list-old']));
test('missing lists never imply clean config, explicit incomplete evidence',()=>{
 const v=processingSummary({staticPath:{mode:'MODE_LIST',hits:[]},listErrors:['user.list: 403'],running:true,probe:{reachable:true,status_code:200}});
 assert.match(v.title,/НЕ ПОЛНОСТЬЮ/);assert.equal(v.kind,'warn');
});
test('include/exclude conflict always flagged',()=>{
 const v=processingSummary({staticPath:{mode:'MODE_LIST',hits:[{file:'a.list',effective:'include'},{file:'b.list',effective:'exclude'}]},listErrors:[],running:true,probe:{reachable:true,status_code:200}});
 assert.match(v.title,/КОНФЛИКТ/);
});
test('ordinary HTTPS probe is not live packet evidence',()=>{
 const v=processingSummary({staticPath:{mode:'MODE_ALL',hits:[]},listErrors:[],running:true,probe:{reachable:true,status_code:204}});
 assert.equal(v.kind,'good');assert.match(v.detail,/не доказан/);
});
test('live proof requires both directions, server success and cleanup',()=>{
 const data={ok:true,cleanup_baseline_after:true,candidates:[{source_profile_index:2,attempts:[{ok:true,cleanup_proven:true,queue:30001,outbound_queue_packets:2,inbound_queue_packets:3}]}]};
 assert.equal(processingLiveProof(data).proven,true);
 assert.equal(processingLiveProof({...data,cleanup_baseline_after:false}).proven,false);
 assert.equal(processingLiveProof({...data,candidates:[{attempts:[{ok:true,outbound_queue_packets:2,inbound_queue_packets:0}]}]}).proven,false);
});
test('only first three eligible strategies are displayed',()=>{
 const rows=processingStrategyPlan({profiles:Array.from({length:5},(_,i)=>({index:i,candidate_eligible:i!==1,args:['--x']}))},null);
 assert.deepEqual(rows.map(x=>x.name),['BASELINE','PROFILE 0','PROFILE 2','PROFILE 3']);
});
test('fast POST always explicit token and exact hex SHA',()=>{
 const sha='f'.repeat(64);assert.deepEqual(processingFastPayload('example.com',{running:true,config_sha256:sha}),{mode:'fast',server_name:'example.com',expected_config_sha256:sha,confirm:'ROUTERFORGE_AUTOTUNE_TLS'});
 for(const s of [{running:false,config_sha256:sha},{running:true,config_sha256:'x'}])assert.throws(()=>processingFastPayload('example.com',s));
});
test('report is JSON-only and explicitly preserves observability boundary',()=>{
 const report=processingReport({domain:'example.com',staticPath:{mode:'MODE_ALL',hits:[]},listErrors:[],running:true,probe:{reachable:true,status_code:200}},null,null);
 assert.match(report.observability_boundary,/q300/);assert.equal(report.live_nfqws,null);
});
test('Svelte owns UI, API commands and legacy no longer assigns Processing Map handlers',()=>{
 const s=fs.readFileSync(new URL('./NfqwsProcessingMap.svelte',import.meta.url),'utf8');const r=fs.readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
 assert.match(s,/\/bench-autotune/);assert.match(s,/\/check/);assert.match(s,/\/keenetic/);assert.match(s,/\/list\?name=/);
 assert.doesNotMatch(r,/\$\('runProcessingMap'\)\.onclick/);assert.doesNotMatch(r,/\$\('processingLiveCheck'\)\.onclick/);
});
