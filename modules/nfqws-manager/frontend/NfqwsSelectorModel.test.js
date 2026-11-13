import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {selectorPath,selectorProgressPath,selectorArgTokens,selectorRunBody,selectorWorking,selectorProgress,selectorPreviewBody,selectorBindBody,selectorApplyBody,selectorProfileQuality,selectorCandidateMetrics} from './NfqwsSelectorModel.js';
const sha='a'.repeat(64),nextSha='b'.repeat(64),session='ui-0123456789abcdef';
const defaults={target:'youtube.com',mode:'normal',concurrency:'2',production:true,autoPool:true,libraryEnabled:true,community:false,asn:'',extraName:'',extraArgs:''};
const library=[{id:'c1',name:'Standard',source:'local',args:['--filter-tcp=443','--hostlist-domains=a.test']}];
const working={candidate_id:'candidate-1',candidate_name:'Live',result_class:'WORKING',successes:2,cleanup_proven:true,infrastructure_ok:true};
const result={session_id:session,candidates:[working],apply_gate_eligible:false};
const preview={ok:true,preview_only:true,changed:true,active_config_sha256:sha,candidate_config_sha256:nextSha};
test('Selector only permits the same-origin exact API endpoints, no arbitrary URL or method',()=>{
  assert.equal(selectorPath('/status'),'\/status');
  assert.equal(selectorPath('/v2/selector','POST'),'/v2/selector');
  assert.equal(selectorProgressPath(session),'/v2/selector-progress?session='+session);
  for(const url of ['https://evil.test','//evil.test','/v2/selector-progress?session=ui-abcdef&evil=1','/v2/selector?x=1'])assert.throws(()=>selectorPath(url));
  assert.throws(()=>selectorPath('/v2/selector','GET'));
  assert.throws(()=>selectorProgressPath('../evil'));
});
test('Run is only explicit: token, exact SHA, default mode, own library and no mutation before action',()=>{
  const draft={...defaults};const copy=structuredClone(library);
  const b=selectorRunBody(draft,library,sha,session);
  assert.deepEqual(b,{mode:'normal',server_name:'youtube.com',expected_config_sha256:sha,concurrency:2,session_id:session,include_production:true,auto_pool:true,candidates:copy,community_enabled:false,community_asn:0,property_vector:null,confirm:'ROUTERFORGE_V2_SELECTOR'});
  assert.deepEqual(draft,defaults);assert.deepEqual(library,copy);
});
test('Manual quoted strategy argv remains stable in candidate vector',()=>{
  const args='--hostlist-domains="domain one" --dpi-desync=fake --foo=bar';
  assert.deepEqual(selectorArgTokens(args),['--hostlist-domains=domain one','--dpi-desync=fake','--foo=bar']);
  const b=selectorRunBody({...defaults,extraArgs:args,extraName:'Manual test'},[],sha,session);
  assert.deepEqual(b.candidates,[{name:'Manual test',source:'custom',args:['--hostlist-domains=domain one','--dpi-desync=fake','--foo=bar']}]);
});
test('Selector refuses all omitted, arbitrary mode/threads/ASN, stale or missing SHA',()=>{
  for(const patch of [{production:false,autoPool:false,libraryEnabled:false},{mode:'unknown'},{concurrency:'5'},{community:true,autoPool:false},{asn:'4294967296'},{asn:'1.5'},{target:'a\nb'}])assert.throws(()=>selectorRunBody({...defaults,...patch},[],sha,session));
  assert.throws(()=>selectorRunBody(defaults,library,'bad',session));
  assert.throws(()=>selectorRunBody(defaults,library,sha,'non-hex'));
});
test('Community requires user enabled switch and ASN stays bounded',()=>{
  const b=selectorRunBody({...defaults,community:true,asn:'12389'},[],sha,session);
  assert.equal(b.community_enabled,true);assert.equal(b.community_asn,12389);
  assert.equal(selectorRunBody({...defaults,community:false,asn:''},[],sha,session).community_asn,0);
});
test('Candidate must be verified WORKING, infrastructure and cleanup both proven for binding',()=>{
  assert.deepEqual(selectorWorking(result),[working]);
  for(const k of ['successes','cleanup_proven','infrastructure_ok']){
    assert.equal(selectorWorking({candidates:[{...working,[k]:k==='successes'?0:false}]}).length,0);
  }
  assert.deepEqual(selectorBindBody(result,'candidate-1',sha),{session_id:session,expected_config_sha256:sha,candidate_id:'candidate-1',confirm:'ROUTERFORGE_V2_SELECTOR_BIND'});
  assert.throws(()=>selectorBindBody(result,'unknown',sha));
  assert.throws(()=>selectorBindBody({...result,apply_gate_eligible:true},'candidate-1',sha));
});
test('Preview is blocked without exact gate and token; never auto applies',()=>{
  assert.throws(()=>selectorPreviewBody(result,sha));
  const body=selectorPreviewBody({...result,apply_gate_eligible:true,apply_gate_token:'token'},sha);
  assert.deepEqual(body,{token:'token',expected_config_sha256:sha,confirm:'ROUTERFORGE_AUTOTUNE_PREVIEW'});
  assert.throws(()=>selectorPreviewBody({...result,apply_gate_eligible:true,apply_gate_token:'token'},'notsha'));
});
test('Apply only after exact preview with explicit atomic preflight status SHA',()=>{
  const r={apply_gate_token:'signed-token'};
  assert.deepEqual(selectorApplyBody(r,preview,{config_sha256:sha}),{receipt_token:'signed-token',expected_active_sha256:sha,expected_candidate_sha256:nextSha,confirm:'ROUTERFORGE_AUTOTUNE_APPLY'});
  for(const p of [{...preview,changed:false},{...preview,preview_only:false},{...preview,active_config_sha256:'bad'}])assert.throws(()=>selectorApplyBody(r,p,{config_sha256:sha}));
  assert.throws(()=>selectorApplyBody(r,preview,{config_sha256:nextSha}));
  assert.throws(()=>selectorApplyBody({},preview,{config_sha256:sha}));
});
test('Progress is clamped, normal mode uses bounded 20 batch',()=>{
  assert.deepEqual(selectorProgress({completed:22,total:60},'normal'),{label:'2 / 20 · всего 22 / 60',percent:10});
  assert.equal(selectorProgress({completed:8,total:10},'fast').percent,80);
  assert.equal(selectorProgress({completed:21,total:20},'fast').percent,100);
  assert.equal(selectorProgress({},'thorough').percent,0);
});
test('Inspect conflicts are ordered by exact hostlist > auto list > ipset > generic',()=>{
  assert.ok(selectorProfileQuality({match_reason:'hostlist-domains=youtube.com'}).rank>selectorProfileQuality({match_reason:'include /opt/hosts.list'}).rank);
  assert.ok(selectorProfileQuality({match_reason:'include /opt/hosts.list'}).rank>selectorProfileQuality({match_reason:'ipset /opt/ipset'}).rank);
});
test('Results keep technical diagnostic text as data, not raw HTML',()=>{
  const x=selectorCandidateMetrics({success_rate:0.75,complete_rate:0.5,attempts:[{error:'<script>alert(1)</script>'}]});
  assert.equal(x.success,75);assert.equal(x.complete,50);assert.equal(x.error,'<script>alert(1)</script>');
  const svelte=readFileSync(new URL('./NfqwsSelector.svelte',import.meta.url),'utf8');
  assert.ok(svelte.includes('{metrics.error}'));
  assert.ok(!svelte.includes('{@html'));
});
test('Selector Svelte owns all UI commands, legacy adapter only for explicit Auto Setup',()=>{
  const svelte=readFileSync(new URL('./NfqwsSelector.svelte',import.meta.url),'utf8');
  const runtime=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
  const app=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
  assert.ok(svelte.includes('onRegister({runExternal:'));
  assert.ok(app.includes('<NfqwsSelector active='));
  for(const str of ["$('v2Selector",'function renderV2Selector','function pollV2Selector','function bindV2SelectorRecommendation'])assert.ok(!runtime.includes(str),str);
  assert.ok(app.includes('getSelector={()=>selectorController}'));
  assert.ok(!runtime.includes("$('v2AutoSetup"));
  assert.ok(!svelte.includes('fetch('));
});
