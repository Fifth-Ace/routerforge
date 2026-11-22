import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';

const source=name=>readFileSync(new URL(name,import.meta.url),'utf8');

test('Sandbox Refresh callback resolves to a declared Svelte function before first mount',()=>{
  const app=source('./NFQWSManagerApp.svelte');
  const sandbox=source('./NfqwsSandbox.svelte');
  assert.match(app,/function strategyRefresh\(\)\{strategyLibraryRefreshSeq\+=1;\}/);
  assert.match(app,/onRefreshLibrary=\{strategyRefresh\}/);
  assert.match(sandbox,/await onRefreshLibrary\(\)/);
  assert.match(app,/refreshSeq=\{strategyLibraryRefreshSeq\}/);
});

test('Sandbox library invalidation is pure and never performs a mutation or reloads service',()=>{
  const app=source('./NFQWSManagerApp.svelte');
  const fn=app.match(/function strategyRefresh\(\)\{[^}]+\}/)?.[0];
  assert.ok(fn,'Svelte library invalidation function missing');
  const context={strategyLibraryRefreshSeq:3,fetch:()=>{throw Error('UNEXPECTED_IO')},runtime:{refresh:()=>{throw Error('UNEXPECTED_SERVICE_REFRESH')}}};
  const result=runInNewContext(`${fn};strategyRefresh();strategyLibraryRefreshSeq`,context);
  assert.equal(result,4);
  assert.equal(context.strategyLibraryRefreshSeq,4);
  assert.doesNotMatch(fn,/POST|fetch\(|runtime\.|\.control\(/);
});

test('Import refreshes registry AND same Svelte library owner without imperatively writing DOM',()=>{
  const app=source('./NFQWSManagerApp.svelte');
  assert.match(app,/function importedStrategiesChanged\(\)\{[\s\S]*?registryRefreshSeq\+=1;\s*strategyLibraryRefreshSeq\+=1;\s*\}/);
  assert.doesNotMatch(app,/function strategyRefresh\([^)]*\)\s*\{[^}]*\binnerHTML\b/);
});


test('all bare Svelte component property bindings in the root app resolve to lexical declarations',()=>{
  const app=source('./NFQWSManagerApp.svelte');
  function unresolved(text){
    const script=text.split('</script>')[0];
    const refs=[...text.matchAll(/\b[\w-]+\s*=\s*\{\s*([A-Za-z_$][\w$]*)\s*\}/g)].map(m=>m[1]);
    return [...new Set(refs)].filter(name=>{
      const escaped=name.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
      const own=new RegExp('\\b(?:function|const|let|var|class)\\s+'+escaped+'\\b');
      const named=new RegExp('\\bimport\\s+\\{[^}]*\\b'+escaped+'\\b');
      const direct=new RegExp('\\bimport\\s+'+escaped+'\\b');
      return !own.test(script)&&!named.test(script)&&!direct.test(script);
    });
  }
  assert.deepEqual(unresolved(app),[]);
  assert.deepEqual(unresolved(app.replace('onRefreshLibrary={strategyRefresh}','onRefreshLibrary={missingCallback}')),['missingCallback']);
});
