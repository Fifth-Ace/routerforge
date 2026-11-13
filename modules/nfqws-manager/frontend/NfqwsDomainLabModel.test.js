import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {diagnosticRoute,diagnosticTarget,diagnosticBody,domainProjection,diagnosticTone,labBounds,labResolveBody,labResolvedTargets,labStatistics,labRowView} from './NfqwsDomainLabModel.js';
const component=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
const controller=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
const domain=readFileSync(new URL('./NfqwsDomainTest.svelte',import.meta.url),'utf8');
const lab=readFileSync(new URL('./NfqwsProductionLab.svelte',import.meta.url),'utf8');
test('domain input accepts known DNS names while rejecting paths and forged values',()=>{
  assert.equal(diagnosticTarget(' YouTube.com. '),'youtube.com');
  assert.deepEqual(diagnosticBody('WWW.Example.org.'),{target:'www.example.org'});
  for (const value of ['', 'http://example.com','a..com','bad_name.com','-bad.com','x'.repeat(64)+'.com','a;rm -rf','example.com/#','/api'])assert.throws(()=>diagnosticTarget(value),/INVALID/);
});
test('diagnostic request paths admit only read-only GET and explicit diagnostic POSTs',()=>{
  for(const p of ['/v2/site-diagnose','/v2/targets/resolve','/v2/detect','/v2/inspect-target'])assert.equal(diagnosticRoute(p,'POST'),p);
  assert.equal(diagnosticRoute('/v2/target-sources','GET'),'/v2/target-sources');
  for(const path of ['/status','/v2/selector/apply','https://evil.test','/list/save','/v2/target-sources'])assert.throws(()=>diagnosticRoute(path,'POST'),/DENIED/);
});
test('diagnostic projection retains stages, profiles, current ownership and next action without HTML',()=>{
  const data={inspect:{target:'example.com',runtime_running:true,binding_verdict:'BOUND',list_matches:[{name:'foo.list',path:'/lists/foo.list',relation:'included'}],production_profile_matches:[{index:2,args:['--hostlist=/tmp/a','--dpi-desync=fake'],candidate_ready:true}]},detect:{classification:'clear',classification_text:'OK',stages:{dns:{state:'PASS',detail:'resolved',latency_ms:3},tls:{state:'FAIL',detail:'EOF'}}},context:{title:'Контекст',binding_detail:'profile bound',next_action:'selector'},};
  const v=domainProjection(data,'example.com');
  assert.equal(v.target,'example.com');assert.equal(v.stages.length,4);assert.equal(v.stages[2].state,'FAIL');assert.equal(v.lists.length,1);assert.equal(v.profiles[0].index,2);assert.equal(v.guidance.needsSelector,true);assert.equal(v.summary.find(x=>x[0]==='Production')[1],'RUNNING');
  assert.equal(diagnosticTone('TLS_FAILURE'),'bad');assert.equal(diagnosticTone('PARTIAL'),'warn');
});
test('domain rejects malformed server target instead of rewriting another target',()=>{
  assert.throws(()=>domainProjection({inspect:{target:'https://evil.com'}},'good.com'),/INVALID/);
});
test('lab guards concurrency rounds and resolves only approved inventory lists',()=>{
  assert.deepEqual(labBounds(5,4),{rounds:5,concurrency:4});
  for(const x of [[0,4],[1,0],[6,4],[1,5],[1.5,1],['bad',2]])assert.throws(()=>labBounds(...x),/INVALID/);
  const inventory=[{name:'folder/foo.list'}];
  assert.deepEqual(labResolveBody('list','folder/foo.list','',inventory),{source:'list',list_name:'folder/foo.list',limit:64});
  assert.throws(()=>labResolveBody('list','other.list','',inventory),/STALE/);
  assert.deepEqual(labResolveBody('manual','', 'example.com\nEXAMPLE.com\nyoutube.com'),{source:'manual',targets:['example.com','youtube.com'],limit:64});
  assert.throws(()=>labResolveBody('manual','', 'http://bad.test'),/INVALID/);
  assert.throws(()=>labResolveBody('external','', 'example.com'),/SOURCE/);
});
test('lab validates server targets and progress, preserving partial success',()=>{
  assert.deepEqual(labResolvedTargets({targets:['example.com','example.com','youtube.com']}),['example.com','youtube.com']);
  assert.throws(()=>labResolvedTargets({targets:[]}),/INVALID/);
  assert.throws(()=>labResolvedTargets({targets:['/etc/passwd']}),/INVALID/);
  const rows=[{target:'example.com',attempts:[{ok:true},{ok:false,classification:'DNS_FAILURE'}],inspect:{binding_verdict:'BOUND',list_matches:[{}]}},{target:'youtube.com',attempts:[{ok:true}]}];
  assert.deepEqual(labStatistics(rows,3,4),{targets:2,ok:2,fail:1,progress:'75%',ratio:2/3});
  assert.equal(labRowView(rows[0]).state,'UNSTABLE');assert.equal(labRowView(rows[0]).lists,1);assert.equal(labRowView(rows[1]).state,'WORKING');
});
test('Svelte owns domain/lab I/O, cancellation and navigation instead of legacy DOM renderer',()=>{
  for(const needle of ['<NfqwsDomainTest active=','<NfqwsProductionLab active=','onDomainSeed:','onLabSeed:'])assert.ok(component.includes(needle),needle);
  for(const needle of ['data-svelte-owned="nfqws-domain-test"','/v2/site-diagnose','onSelector(result.target)','abort?.abort()'])assert.ok(domain.includes(needle),needle);
  for(const needle of ['data-svelte-owned="nfqws-production-lab"','/v2/detect','/v2/inspect-target','onHistory(\'v2-prodlab\'','const locked=$derived','function stop()'])assert.ok(lab.includes(needle),needle);
  for(const forbidden of ['function runV2Domain(', 'function runV2Lab(', "$('v2LabSource')", "$('v2DomainTarget')", "$('v2LabStop').onclick"])assert.ok(!controller.includes(forbidden),forbidden);
});
