import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {verifiedStrategyLibraryResponse} from './NfqwsStrategyLibraryModel.js';
const row={id:'strategy-1',name:'пример',source:'custom',args:['--filter-tcp=443'],fingerprint:'a'.repeat(64)};
const response={ok:true,version:1,max:64,count:1,strategies:[row]};
test('native Svelte Strategy Library accepts complete authoritative backend response without changing argv',()=>{
  const copy=structuredClone(response);const result=verifiedStrategyLibraryResponse(copy);
  assert.equal(result.length,1);assert.deepEqual(result[0].args,['--filter-tcp=443']);result[0].args[0]='edited';
  assert.deepEqual(copy,response);
  assert.deepEqual(verifiedStrategyLibraryResponse({ok:true,version:1,max:64,count:0,strategies:[]}),[]);
});
test('failed HTTP, malformed or partial library inventory does not become a success',()=>{
  assert.throws(()=>verifiedStrategyLibraryResponse({}));
  for(const patch of [null, {ok:false}, {version:2}, {max:63}, {count:2},{strategies:null},{strategies:[null]},
   {strategies:[{...row,id:'../bad'}]}, {strategies:[{...row,source:'unsupported'}]},
   {strategies:[{...row,fingerprint:'invalid'}]}, {strategies:[{...row,args:[]}]},
   {strategies:[row,row],count:2}]){
      const body=patch===null?null:{...structuredClone(response),...patch};
      assert.throws(()=>verifiedStrategyLibraryResponse(body));
   }
});
test('Strategy Library GET, save, delete and import invalidation stay Svelte-owned with no deleted runtime loader',()=>{
  const source=n=>readFileSync(new URL('./'+n,import.meta.url),'utf8');
  const library=source('NfqwsStrategyLibrary.svelte');
  const app=source('NFQWSManagerApp.svelte');
  const runtime=source('NfqwsRuntime.js');
  assert.match(library,/onRequest\('\/v2\/strategies',\{method:'GET'\}\)/);
  assert.match(library,/verifiedStrategyLibraryResponse\(response\)/);
  assert.match(library,/await loadInventory\(n\)/);
  assert.match(library,/appliedRefreshSeq!==refreshSeq/);
  assert.match(library,/appliedRefreshSeq=refreshSeq;\s*\/\/ A stale library/);
  assert.match(app,/refreshSeq=\{strategyLibraryRefreshSeq\}/);
  assert.match(app,/onRegistryInvalidate=\{importedStrategiesChanged\}/);
  assert.match(app,/function importedStrategiesChanged\(\)\{[\s\S]*strategyLibraryRefreshSeq\+=1;/);
  assert.doesNotMatch(runtime,/v2LoadLibrary\(|strategyLibraryRefresh\(|onStrategyLibrary\s*=/);
});
