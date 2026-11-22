import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {dpiTestOptions,dpiStreamPath,dpiWebPayload,dpiEndProof,dpiReportSections} from './NfqwsDPIWorkspaceModel.js';
const sha='a'.repeat(64);
const form=()=>({tests:['6','0','2','0'],domains:'Example.com,openai.com\nFoo.net',concurrency:'20',profile:'ru',language:'ru',burst:5,burstTimeout:8,burstTLS:'1.3+1.2',burstALPN:'h2'});
test('Web DPI requests exact API and one-shot POST payload with production SHA',()=>{
  const data=dpiWebPayload(form(),sha);
  assert.equal(data.tests,'026');assert.equal(data.confirm,'ROUTERFORGE_DPI_DETECTOR_V5_RUN');
  assert.equal(data.expected_config_sha256,sha);assert.equal(data.trace,false);
  assert.deepEqual(data.domains,['example.com','openai.com','foo.net']);
  assert.equal(dpiStreamPath('/api/modules/nfqws-manager/ui/index.html'),'/api/modules/nfqws-manager/dpi-detector/v5/stream');
  assert.equal(dpiStreamPath('/'),'/api/modules/nfqws-manager/dpi-detector/v5/stream');
});
test('all 7 tests distinct; invalid, empty and duplicate/unknown options cannot bypass server',()=>{
  assert.equal(new Set(dpiTestOptions.map(o=>o.id)).size,7);
  assert.equal(dpiWebPayload({...form(),tests:dpiTestOptions.map(x=>x.id)},sha).tests,'0123456');
  for(const input of [[],['7'],['0','-1']])assert.throws(()=>dpiWebPayload({...form(),tests:input},sha));
  assert.throws(()=>dpiWebPayload(form(),'bad'),/SHA/);
  assert.throws(()=>dpiStreamPath('https://evil.example/ui/'),/PATH/);
  assert.throws(()=>dpiStreamPath('//evil/ui/'),/PATH/);
});
test('payload rejects invalid concurrency, burst, domains, and unknown fingerprint without writes',()=>{
  for(const [key,value] of [['concurrency',0],['concurrency',101],['burst',0],['burstTimeout',121],['fingerprint','untrusted'],['domains','localhost'],['domains','abc.local'],['language','xy']]){
    assert.throws(()=>dpiWebPayload({...form(),[key]:value},sha),undefined,`${key}:${value}`);
  }
});
test('DPI done requires explicit unchanged production and same exact SHA',()=>{
  assert.equal(dpiEndProof({type:'done',production_config_unchanged:true,production_config_sha256:sha,report_bytes:7},sha).bytes,7);
  for(const val of [{type:'chunk'}, {type:'error'}, {type:'done',production_config_sha256:sha}, {type:'done',production_config_unchanged:true,production_config_sha256:'b'.repeat(64)}, {type:'done',production_config_unchanged:false,production_config_sha256:sha}])assert.throws(()=>dpiEndProof(val,sha));
});
test('native report sections preserve full text and strip ANSI without HTML generation',()=>{
  const report='intro\nNetwork information\nASN X\nDNS availability\nOK\nFingerprint burst\nLast';
  const sections=dpiReportSections(report);
  assert.deepEqual(sections.map(x=>x.title),['Введение','Network','DNS','Fingerprint Burst']);
  assert.equal(sections.map(x=>x.text).join('\n'),report);
  assert.deepEqual(dpiReportSections('garbage'),[{id:'all',title:'Отчёт upstream',text:'garbage'}]);
  assert(!dpiReportSections('\u001b[31mNetwork information\u001b[0m').some(x=>x.text.includes('\u001b')));
});
test('Svelte owns workspace, stream and Stop; Runtime has no removed imperative DPI DOM',()=>{
  const root=new URL('.',import.meta.url);
  const component=readFileSync(new URL('NfqwsDPIWorkspace.svelte',root),'utf8');
  const runtime=readFileSync(new URL('NfqwsRuntime.js',root),'utf8');
  const app=readFileSync(new URL('NFQWSManagerApp.svelte',root),'utf8');
  assert(component.includes('consumeDpiNdjson(')&&component.includes('dpiWebPayload('));
  assert(component.includes('onMount(')&&component.includes('function stop(')&&component.includes('runController.abort()'));
  assert(app.includes('<NfqwsDPIWorkspace')&&app.includes('onRefresh={()=>runtime?.dpiRefresh()}'));
  assert(!runtime.includes('renderDPIDetectorWorkspace')&&!runtime.includes("$('dpiWebRun')")&&!runtime.includes('dpiParityReportHTML'));
  assert(!component.includes('innerHTML')&&!component.includes('{@html'));
});
