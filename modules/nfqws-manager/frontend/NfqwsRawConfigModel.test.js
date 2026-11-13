import {test} from 'node:test';
import assert from 'node:assert/strict';
import {rawStatistics,rawDiff,statusRawConfig,rawPreflight,rawLuaPayloadReplacement,rawValidationSummary,rawApiRequest} from './NfqwsRawConfigModel.js';
const h='f'.repeat(64);
const snapshot=(content='x=1', sha=h)=>({config:content,config_path:'/opt/etc/nfqws2/nfqws2.conf',config_sha256:sha});

test('native raw config snapshot rejects incomplete and malformed status',()=>{
  assert.throws(()=>statusRawConfig(null),/INVALID/);
  assert.throws(()=>statusRawConfig({config:7}),/INVALID/);
  assert.equal(statusRawConfig(snapshot()).content,'x=1');
});
test('raw diff keeps added deleted lines and escaped source data',()=>{
  assert.deepEqual(rawDiff('a\n<script>','a\nnew'),[
    {kind:'same',text:'  a'},{kind:'del',text:'- <script>'},{kind:'add',text:'+ new'}
  ]);
});
test('raw statistics measure actual UTF8 bytes, CRLF and lines',()=>{
  assert.deepEqual(rawStatistics('тест\r\nok'),{lines:2,bytes:12});
});
test('raw preflight enforces unchanged server SHA, content, path and exact in-flight editor snapshot',()=>{
  assert.equal(rawPreflight(snapshot(),snapshot(),'x=2','x=2'),true);
  assert.throws(()=>rawPreflight(snapshot(),snapshot('x=3'),'x=2','x=2'),/STALE/);
  assert.throws(()=>rawPreflight(snapshot(),snapshot('x=1','a'.repeat(64)),'x=2','x=2'),/STALE/);
  assert.throws(()=>rawPreflight(snapshot(),{...snapshot(),config_path:'/other'},'x=2','x=2'),/STALE/);
  assert.throws(()=>rawPreflight(snapshot(),snapshot(),'x=3','x=2'),/DRAFT_CHANGED/);
  assert.throws(()=>rawPreflight(snapshot(),snapshot(),'x=1','x=1'),/DRAFT_CHANGED/);
  assert.throws(()=>rawPreflight(snapshot('x=1','bad'),snapshot(),'x=2','x=2'),/STALE/);
});
test('native Lua-fix touches only exact validated selection in draft',()=>{
  const original='one --lua-desync=fake:foo=bar two',a=original.indexOf('--lua'),b=a+'--lua-desync=fake:foo=bar'.length;
  const r=rawLuaPayloadReplacement(original,a,b,original);
  assert.equal(r.text,'one --lua-desync=fake:foo=bar:payload=all two');
  assert.equal(r.selectionStart,a);
  assert.throws(()=>rawLuaPayloadReplacement(original,a,b,'other'),/STALE/);
  assert.throws(()=>rawLuaPayloadReplacement(original,-1,b,original),/STALE/);
  assert.throws(()=>rawLuaPayloadReplacement('x --lua-desync=unknown',2,22,'x --lua-desync=unknown'),/INVALID|STALE/);
});
test('semantic error and dry-run fail stay visible, no false success',()=>{
  assert.equal(rawValidationSummary({syntax_ok:true,semantic_ok:true,issues:[],native_dry_run:'passed'}).valid,true);
  assert.equal(rawValidationSummary({syntax_ok:true,semantic_ok:true,issues:[],native_dry_run:'failed'}).valid,false);
  assert.equal(rawValidationSummary({syntax_ok:true,semantic_ok:false,issues:[{code:'UNKNOWN_PAYLOAD_LUA'}]}).unknownLua,true);
  assert.throws(()=>rawValidationSummary({syntax_ok:true}),/INCOMPLETE/);
});
test('native raw client never fetches without explicit call and allows only four exact paths',async()=>{
  const seen=[];
  const api=rawApiRequest(async(url,options)=>{seen.push({url,options});return {ok:true,status:200,json:async()=>({valid:true})}},'/api/modules/nfqws-manager/ui/index.html',new AbortController().signal);
  assert.equal(seen.length,0);
  await api('/status');await api('/config/validate',{method:'POST',body:'{}'});
  assert.equal(seen.length,2);
  assert.equal(seen[0].url,'/api/modules/nfqws-manager/status');
  assert.equal(seen[0].options.credentials,'same-origin');
  await assert.rejects(api('/../admin'),/UNTRUSTED/);
  await assert.rejects(api('/action'),/UNTRUSTED/);
  assert.equal(seen.length,2);
});
test('native raw client rejects HTTP 403/409 and invalid JSON and does not retry',async()=>{
  let calls=0;
  const api=rawApiRequest(async()=>{calls++;return {ok:false,status:409,json:async()=>({error:'revision conflict'})}},'/ui/index.html',new AbortController().signal);
  await assert.rejects(api('/config'),/HTTP 409.*revision conflict/);assert.equal(calls,1);
  const bad=rawApiRequest(async()=>({ok:true,status:200,json:async()=>{throw Error('bad')}}),'/ui/index.html',new AbortController().signal);
  await assert.rejects(bad('/status'),/INVALID_JSON/);
});
test('native raw client abort/unmount prevents even starting network operations',async()=>{
  const controller=new AbortController();controller.abort();let calls=0;
  const api=rawApiRequest(async()=>{calls++;return {}},'/ui/index.html',controller.signal);
  await assert.rejects(api('/status'),/UNMOUNTED/);assert.equal(calls,0);
});

test('native raw UI owns all editor actions and legacy controller no longer assigns its DOM handlers',async()=>{
  const {readFileSync}=await import('node:fs');
  const {fileURLToPath}=await import('node:url');
  const {dirname,join}=await import('node:path');
  const here=dirname(fileURLToPath(import.meta.url));
  const legacy=readFileSync(join(here,'NfqwsRuntime.js'),'utf8');
  const ui=readFileSync(join(here,'NfqwsRawConfig.svelte'),'utf8');
  const app=readFileSync(join(here,'NFQWSManagerApp.svelte'),'utf8');
  assert.match(app,/<NfqwsRawConfig\s/);
  assert.match(ui,/data-svelte-owned="raw-config"/);
  assert.match(ui,/id="save"/);
  assert.match(ui,/id="checkConfigSyntax"/);
  assert.match(ui,/rawPreflight\(/);
  for(const id of ['config','configMeta','save','undoConfig','diffConfig','checkConfigSyntax','nf1bLuaResults','nf1bLuaRefresh']) {
    assert.doesNotMatch(legacy,new RegExp('\\$\\(\\x27'+id+'\\x27\\)'));
  }
  assert.doesNotMatch(legacy,/\b(?:resizeRawEditor|mountRawSyntaxEditor|saveConfig|nf1bLuaLoad)\(/);
});
