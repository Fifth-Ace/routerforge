import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {dpiStatusProjection,dpiBytes} from './NfqwsDPIStatusModel.js';
test('DPI status cannot claim READY without exact confirmed installed and execution_ready',()=>{
  const valid={installed:true,execution_ready:true,path:'/opt/bin/dpi-detector',version:'5.1',size_bytes:2048};
  assert.equal(dpiStatusProjection(valid).state,'READY');
  assert.equal(dpiStatusProjection({...valid,execution_ready:'true'}).ready,false);
  assert.equal(dpiStatusProjection({...valid,installed:'true'}).ready,false);
  assert.equal(dpiStatusProjection(valid,true).ready,false);
  assert.equal(dpiStatusProjection(valid,false,'network error').state,'ERROR');
  assert.equal(dpiStatusProjection({installed:false,execution_ready:true}).state,'NOT INSTALLED');
  assert.equal(dpiStatusProjection(null).state,'IDLE');
  assert.equal(dpiStatusProjection([]).state,'ERROR');
});
test('DPI status projection is read-only, bytes are validated, and error is visible',()=>{
  const original={installed:true,execution_ready:false,warning:'no interpreter',host_arch:'aarch64',path:'<img src=x onerror=alert(1)>',size_bytes:1048576};
  const before=structuredClone(original);
  const view=dpiStatusProjection(original);assert.equal(view.state,'INCOMPATIBLE');assert.match(view.items[0].detail,/<img/);
  assert.match(view.items[1].detail,/no interpreter/);assert.deepEqual(original,before);
  assert.equal(dpiBytes(1048576),'1.0 MB');assert.equal(dpiBytes(-2),'—');assert.equal(dpiBytes('wat'),'—');
  assert.match(dpiStatusProjection(null,false,'timeout').items[0].detail,/timeout/);
});
test('Svelte now renders both status badges and diagnostic details without legacy HTML ownership',()=>{
  const legacy=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
  const app=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
  const native=readFileSync(new URL('./NfqwsDPIStatus.svelte',import.meta.url),'utf8');
  assert.match(app,/onDpiStatus:value=>\{dpiInfo=value;\}/);
  const keenetic=readFileSync(new URL('./NfqwsKeenetic.svelte',import.meta.url),'utf8');
  assert.match(app,/<NfqwsKeenetic active=/);
  assert.match(app,/dpiInfo=\{dpiInfo\}/);
  assert.match(keenetic,/<NfqwsDPIStatus snapshot=\{dpiInfo.snapshot\}/);
  assert.doesNotMatch(keenetic,/@html|innerHTML/);
  assert.match(native,/data-svelte-owned="nfqws-dpi-status"/);
  assert.match(native,/data-svelte-owned="nfqws-dpi-keenetic-status"/);
  assert.match(native,/\{item\.detail\}/);
  assert.doesNotMatch(native,/@html|innerHTML/);
  for(const id of ['dpiDetectorStatus','dpiDetectorState','dpiModuleSummary','dpiModuleState','dpiDetectorTabState']){
    assert.doesNotMatch(legacy,new RegExp("\\$\\('"+id+"'\\)\\.(?:innerHTML|textContent|className)"),id);
  }
});
