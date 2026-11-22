import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createLibraryRequest} from './NfqwsConfigLibraryModel.js';

const source=readFileSync(new URL('./NfqwsConfigLibrary.svelte',import.meta.url),'utf8');

test('saved-configs Svelte onMount initialises its API before the reactive initial refresh',()=>{
  const at=source.indexOf('onMount(()=>{');
  assert.ok(at>=0,'Svelte onMount hook missing');
  const end=source.indexOf('\n  });',at);
  assert.ok(end>at,'onMount hook not closed');
  const boot=source.slice(at,end);
  const init='api=createLibraryRequest(window.fetch.bind(window),window.location.pathname,abort.signal);';
  const abort=boot.indexOf('abort=new AbortController();');
  const call=boot.indexOf(init);
  const mounted=boot.indexOf('mounted=true');
  assert.ok(abort>=0 && call>abort && mounted>call,'API must be initialised after AbortController and before mounted');
  assert.equal((source.match(/api=createLibraryRequest\(/g)||[]).length,1,'API must be initialised exactly once');
});

test('saved-configs initial inventory stays same-origin, GET-only and abortable',async()=>{
  const controller=new AbortController();
  const calls=[];
  const fetcher=async(url,options)=>{
    calls.push({url,options});
    return {ok:true,status:200,json:async()=>({root:'/opt/etc/nfqws2/routerforge-configs',files:[]})};
  };
  const api=createLibraryRequest(fetcher,'/api/modules/nfqws-manager/ui/index.html',controller.signal);
  assert.equal(calls.length,0,'creating request helper must not initiate traffic');
  const [files,neighbors]=await Promise.all([api('/configs'),api('/configs/neighbors')]);
  assert.ok(Array.isArray(files.files) && Array.isArray(neighbors.files));
  assert.deepEqual(calls.map(c=>c.url),[
    '/api/modules/nfqws-manager/configs',
    '/api/modules/nfqws-manager/configs/neighbors'
  ]);
  for(const {options} of calls){
    assert.equal(options.credentials,'same-origin');
    assert.equal(options.method,undefined,'page open must not write');
    assert.equal(options.signal,controller.signal);
  }
  controller.abort();
  await assert.rejects(api('/configs'),/NFQWS_UI_UNMOUNTED/);
  assert.equal(calls.length,2,'aborted component must not fetch');
});
