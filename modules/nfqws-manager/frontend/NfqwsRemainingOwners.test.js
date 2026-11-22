import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {catalogVisible,blobSummary,verifiedBlobManifest} from './NfqwsCatalogBlobModel.js';
import {keeneticProjection,keeneticBytes} from './NfqwsKeeneticModel.js';
const src=name=>readFileSync(new URL('./'+name,import.meta.url),'utf8');
const sha='a'.repeat(64),ref='b'.repeat(40);
test('catalog search/filter is immutable and does not accidentally mix local/upstream',()=>{
 const items=[{id:'1',name:'local.conf',kind:'local',source:'local'},{id:'2',name:'up.conf',kind:'upstream',source:'menu',ref,sha256:sha}];
 const snapshot=structuredClone(items);
 assert.deepEqual(catalogVisible(items,'all','').map(x=>x.id),['1','2']);
 assert.deepEqual(catalogVisible(items,'upstream','UP.').map(x=>x.id),['2']);
 assert.deepEqual(catalogVisible(items,'local','UP').map(x=>x.id),[]);
 assert.deepEqual(items,snapshot);
 assert.deepEqual(catalogVisible(null,'all'),[]);
});
test('blob summary does not overclaim referenced and reports missing/update separately',()=>{
 const files=[{name:'a.bin',referenced:true},{name:'b.bin',referenced:'false'}];
 const deps=[{state:'ok'},{state:'missing'},{state:'update'}];
 assert.deepEqual(blobSummary(files,deps),{installed:2,required:3,missing:2,referenced:1});
 assert.deepEqual(blobSummary(null,{}),{installed:0,required:0,missing:0,referenced:0});
});
test('exact upstream ref/path/hash required before any blob POST; reject traversal and spoofing',()=>{
 const row={item:{kind:'upstream',ref},expected:{sha256:sha,path:'strategies/blobs/test.bin'}};
 assert.deepEqual(verifiedBlobManifest(row,'test.bin'),{ref,sha,path:'strategies/blobs/test.bin',file:'test.bin'});
 for(const bad of [
  {...row,item:{kind:'local',ref}},
  {...row,item:{kind:'upstream',ref:'main'}},
  {...row,expected:{sha256:'oops',path:row.expected.path}},
  {...row,expected:{sha256:sha,path:'strategies/blobs/../evil.bin'}},
  {...row,expected:{sha256:sha,path:'https://evil.invalid/a.bin'}},
 ])assert.throws(()=>verifiedBlobManifest(bad,'test.bin'));
 assert.throws(()=>verifiedBlobManifest(row,'wrong.bin'));
 assert.throws(()=>verifiedBlobManifest(row,'../../test.bin'));
});
test('Keenetic model preserves data without side-effects and never synthesizes readiness',()=>{
 const data={detected:true,ndmc_available:false,config_isp_interface:'eth1',default_ipv4_interface:'eth0',config_isp_matches_default:false,
  model:'<img src=x onerror=alert(1)>',interfaces:[{name:'eth0',default_v4:true,up:true}],policy_shape:'object',policy_items:2};
 const snapshot=structuredClone(data),v=keeneticProjection(data);
 assert.equal(v.state,'KEENETIC');assert.equal(v.ndmc,'NO');assert.equal(v.defaultInterface,'eth0');assert.match(v.binding[2].detail,/ничего не меняет автоматически/);
 assert.match(v.model,/<img/);assert.deepEqual(data,snapshot);
 assert.equal(keeneticProjection(null).state,'IDLE');assert.equal(keeneticBytes(-2),'—');assert.equal(keeneticBytes('NaN'),'—');
});
test('all four transferred UI screens are Svelte owners, no duplicate runtime DOM assignments',()=>{
 const runtime=src('NfqwsRuntime.js'),app=src('NFQWSManagerApp.svelte');
 for(const [component,marker,token] of [
  ['NfqwsCatalog.svelte','nfqws-catalog','<NfqwsCatalog snapshot={catalogView}'],
  ['NfqwsBlobs.svelte','nfqws-blobs','<NfqwsBlobs snapshot={blobView}'],
  ['NfqwsKeenetic.svelte','nfqws-keenetic','<NfqwsKeenetic active='],
  ['NfqwsRuntimeHealth.svelte','nfqws-runtime-health','<NfqwsRuntimeHealth active=']]){
   const file=src(component);assert.match(file,new RegExp(marker));assert.ok(app.includes(token));
   assert.doesNotMatch(file,/@html|innerHTML|\.onclick\s*=/,component);
 }
 for(const id of ['catalogList','catalogDetail','catalogUpstreamSource','catalogLocalSource',
  'blobInventoryList','blobDependencyList','blobInstalledCount',
  'keeneticNetwork','keeneticBinding','keeneticStorage','keeneticPolicy',
  'runtimeNFQueueHealth','runtimeAutomationPolicy','runtimeTransportProfiles'])
  assert.doesNotMatch(runtime,new RegExp(`\\$\\('${id}'\\)\\.(?:innerHTML|textContent|onclick|className)`),id);
 assert.match(runtime,/emit\(onCatalog,\{/);assert.match(runtime,/emit\(onBlobs,\{/);
});
test('negative mutation guards: exact SHA, explicit confirms, one-shot and server postverify',()=>{
 const runtime=src('NfqwsRuntime.js');
 for(const token of ['verifiedBlobManifest(row,name)','blobActionBusy=true','if(blobActionBusy||blobLoading)return',
  "confirm:replace?'NFQWS_BLOB_REPLACE':'NFQWS_BLOB_INSTALL'", "confirm:'NFQWS_BLOB_DELETE'",
  'NFQWS_BLOB_INSTALL_POSTVERIFY_MISMATCH','NFQWS_BLOB_DELETE_POSTVERIFY_MISMATCH',
  'NFQWS_BLOB_MUTATION_INVALID_RESPONSE', "if(!confirm(`Удалить unreferenced blob"])
   assert.ok(runtime.includes(token),token);
 assert.doesNotMatch(src('NfqwsBlobs.svelte'),/onRequest\(|fetch\(|method:'POST'|method:'DELETE'/);
 assert.doesNotMatch(src('NfqwsCatalog.svelte'),/fetch\(|onRequest\(|method:'POST'|@html/);
});
test('read-only diagnostic components request GET only and abort on tab switch',()=>{
 for(const name of ['NfqwsKeenetic.svelte','NfqwsRuntimeHealth.svelte']){
  const file=src(name);
  assert.match(file,/controller\?\.abort\(\)/);
  assert.match(file,/method:'GET',signal:c.signal/);
  assert.doesNotMatch(file,/method:'POST'|method:'DELETE'|\/action|\/restart|\/reload/);
 }
});
