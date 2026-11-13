import test from 'node:test';
import assert from 'node:assert/strict';
import {
 normalizeConfigName,configLibraryDiff,configFileStats,requireUnchangedLibraryFile,
 libraryPostSpec,libraryReadPath,createLibraryRequest
} from './NfqwsConfigLibraryModel.js';
import {highlightLibraryConfig} from './NfqwsConfigSyntax.js';
const sha='a'.repeat(64);
test('library filenames are normalized, allow only .conf and reject traversal',()=>{
 assert.equal(normalizeConfigName('youtube'),'youtube.conf');
 assert.equal(normalizeConfigName('youtube.conf'),'youtube.conf');
 for(const x of ['','../a.conf','../../etc/passwd','a/b.conf','a\\b.conf','💀.conf','-hidden.conf','\nA.conf','foo;rm.conf'])assert.equal(normalizeConfigName(x),'',x);
});
test('all eight library mutation operations preserve exact backend confirmation token',()=>{
 const cases={
 'create-empty':['/configs/create','NFQWS_CONFIG_LIBRARY_CREATE'],
 'create-current':['/configs/create','NFQWS_CONFIG_LIBRARY_CREATE'],
 rename:['/configs/rename','NFQWS_CONFIG_LIBRARY_RENAME'],duplicate:['/configs/duplicate','NFQWS_CONFIG_LIBRARY_DUPLICATE'],
 'import-neighbor':['/configs/import-neighbor','NFQWS_CONFIG_LIBRARY_IMPORT'],save:['/configs/save','NFQWS_CONFIG_LIBRARY_SAVE'],
 delete:['/configs/delete','NFQWS_CONFIG_LIBRARY_DELETE'],activate:['/configs/activate','NFQWS_CONFIG_LIBRARY_ACTIVATE']};
 for(const [operation,[path,confirm]] of Object.entries(cases)){
  const result=libraryPostSpec(operation,{name:operation==='import-neighbor'?'old.cfg':'old.conf',newName:'new.conf',content:'NFQWS_ARGS="abc"'});
  assert.equal(result.path,path);assert.equal(result.options.method,'POST');
  const body=JSON.parse(result.options.body);assert.equal(body.confirm,confirm);
 }
});
test('library CRUD cannot use untrusted name or operation',()=>{
 for(const type of ['save','delete','activate','duplicate','rename'])assert.throws(()=>libraryPostSpec(type,{name:'../escape',newName:'ok'}));
 assert.throws(()=>libraryPostSpec('kill',{name:'ok'}),/UNKNOWN_OPERATION/);
 assert.throws(()=>libraryPostSpec('import-neighbor',{name:'a.tmp',newName:'a.conf'}),/INVALID_NEIGHBOR/);
 assert.throws(()=>libraryReadPath('../foo'));
});
test('SHA read-before-write detects changed and missing hashes fail closed',()=>{
 assert.ok(requireUnchangedLibraryFile({name:'a.conf',sha256:sha},{name:'a.conf',sha256:sha}));
 for(const next of [{name:'a.conf',sha256:'b'.repeat(64)},{name:'a.conf',sha256:''},{name:'b.conf',sha256:sha}])assert.throws(()=>requireUnchangedLibraryFile({name:'a.conf',sha256:sha},next),/STALE_SHA/);
});
test('library diff/UTF8 byte stats preserve Cyrillic and escaping',()=>{
 assert.deepEqual(configLibraryDiff('a\nb','a\nc').map(v=>v.type),['same','del','add']);
 assert.equal(configFileStats('Привет').bytes,new TextEncoder().encode('Привет').length);
 assert.ok(highlightLibraryConfig('VALUE="<script>alert(1)</script>"').includes('&lt;script&gt;'));
 assert.ok(!highlightLibraryConfig('VALUE="<img onerror=alert(1)>"').includes('<img'));
});
test('client does no HTTP on creation, requires explicit call and same origin',async()=>{
 const calls=[];
 const fetcher=async(url,opts)=>{calls.push({url,opts});return{ok:true,status:200,json:async()=>({files:[]})}};
 const req=createLibraryRequest(fetcher,'/api/modules/nfqws-manager/ui/index.html',new AbortController().signal);
 assert.equal(calls.length,0);
 await req('/configs');
 assert.equal(calls[0].url,'/api/modules/nfqws-manager/configs');
 assert.equal(calls[0].opts.credentials,'same-origin');
 assert.equal(calls[0].opts.method,undefined);
 await assert.rejects(req('https://evil.example/configs'),/UNTRUSTED_API_PATH/);
 assert.equal(calls.length,1);
});
test('client preserves 403/409 and does not reinterpret errors as success',async()=>{
 for(const status of [403,409,428]){
  const req=createLibraryRequest(async()=>({ok:false,status,json:async()=>({error:'denied'})}),'/ui/index.html',new AbortController().signal);
  await assert.rejects(req('/configs'),new RegExp(`HTTP ${status}: denied`));
 }
});
test('abort/unmount refuses every network call',async()=>{
 const ctrl=new AbortController();ctrl.abort();let calls=0;
 const req=createLibraryRequest(async()=>{calls++;return {ok:true,json:async()=>({})}},'/ui/',ctrl.signal);
 await assert.rejects(req('/configs'),/UNMOUNTED/);assert.equal(calls,0);
});
test('JSON parse errors fail closed',async()=>{
 const req=createLibraryRequest(async()=>({ok:true,status:200,json:async()=>{throw Error('bad')}}),'/ui/',new AbortController().signal);
 await assert.rejects(req('/configs'),/INVALID_JSON/);
});
