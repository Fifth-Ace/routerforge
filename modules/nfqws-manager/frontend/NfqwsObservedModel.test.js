import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {observedPath,observedTarget,observedDomain,observedProject,observedScanBody,observedIgnoreBody,observedAppendUserList} from './NfqwsObservedModel.js';
const sha='a'.repeat(64);
const source=name=>readFileSync(new URL('./'+name,import.meta.url),'utf8');
test('Observed API accepts only exact same-origin route/method pairs',()=>{
  assert.equal(observedPath('/v2/observed-targets'),'/v2/observed-targets');
  assert.equal(observedPath('/v2/observed-targets/scan','POST'),'/v2/observed-targets/scan');
  for(const p of ['//evil.test','https://evil.test','/list?name=other.list','/v2/observed-targets?foo=1'])assert.throws(()=>observedPath(p));
  assert.throws(()=>observedPath('/v2/observed-targets/scan'));
});
test('Observed projection never mutates source and keeps exact noise counts',()=>{
  const data={observed:[{target:'demo.example',device:'lan',count:3,reason:'tcp_unreplied'},{destination:'1.2.3.4'}],suppressed_count:2,ignored_count:5,min_events:2};
  const before=JSON.stringify(data),value=observedProject(data);
  assert.equal(JSON.stringify(data),before);assert.equal(value.rows.length,2);assert.equal(value.suppressed,2);assert.equal(value.ignored,5);
  assert.equal(value.rows[0].reason,'TCP без ответа');assert.equal(value.rows[1].domain,'');
  assert.throws(()=>observedProject({observed:'bad'}));
});
test('Only safe domain names can navigate or be appended, IP stays inert',()=>{
  assert.equal(observedTarget({target:' ExAmPlE.ORG '}),'ExAmPlE.ORG');
  assert.equal(observedDomain({target:' ExAmPlE.ORG '}),'example.org');
  for(const x of ['1.2.3.4','http://evil.test','invalid\n.example','no-domain','a..example','-bad.example','aa.example:443','[::1]'])assert.equal(observedDomain({target:x}), '');
});
test('Scan and Ignore use exact tokens and require explicit event',()=>{
  assert.deepEqual(observedScanBody(),{confirm:'ROUTERFORGE_OBSERVED_SCAN'});
  assert.deepEqual(observedIgnoreBody({target:'example.org'}),{target:'example.org',ignore:true,confirm:'ROUTERFORGE_OBSERVED_IGNORE'});
  assert.throws(()=>observedIgnoreBody({target:'bad\nname'}));
});
test('User list appends only after exact SHA, preserving UTF8 content',()=>{
  const list=observedAppendUserList({content:'# коммент\nold.example\n',sha256:sha},{target:'new.example'});
  assert.equal(list.content,'# коммент\nold.example\nnew.example\n');
  assert.equal(list.expected_sha256,sha);assert.equal(list.confirm,'NFQWS_LIST_SAVE');
  assert.equal(observedAppendUserList({content:'old.example\n\n',sha256:sha},{target:'new.example'}).content,'old.example\n\nnew.example\n');
  assert.equal(observedAppendUserList({content:'old.example\n',sha256:sha},{target:'OLD.example'}),null);
  assert.throws(()=>observedAppendUserList({content:'',sha256:''},{target:'new.example'}));
  assert.throws(()=>observedAppendUserList({content:'',sha256:sha},{target:'1.2.3.4'}));
});
test('Observed Svelte controls own their DOM, no legacy scan or write handler',()=>{
  const app=source('NFQWSManagerApp.svelte'),ui=source('NfqwsObserved.svelte'),legacy=source('NfqwsRuntime.js');
  assert.match(app,/<NfqwsObserved active=/);assert.match(ui,/data-svelte-owned="nfqws-observed-targets"/);
  assert.doesNotMatch(legacy,/function renderV2Observed\(|function scanV2Observed\(|\$\('v2ObservedLoad'\)\.onclick/);
  assert.doesNotMatch(ui,/{@html|\.innerHTML/);
  assert.match(ui,/onMount\(/);assert.match(ui,/if\(mounted&&active&&!attempted\)void load\(\)/);
  assert.match(ui,/if\(mounted&&!active&&controller\)cancel\(\)/);
  assert.match(ui,/function confirmAction\(\)/);assert.doesNotMatch(ui,/localStorage/);
});
test('Auto Setup uses true Svelte runes, never a local function named state',()=>{
  const ui=source('NfqwsAutoSetup.svelte');
  assert.match(ui,/function setWorkflowStatus\(/);
  assert.doesNotMatch(ui,/function state\(/);
  assert.ok((ui.match(/\$state\(/g)||[]).length>10);
  assert.doesNotMatch(ui,/(?<![$\w])state\(/);
});
