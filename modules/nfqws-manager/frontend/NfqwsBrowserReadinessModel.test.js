import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {waitForRenderedState} from './NfqwsBrowserReadinessModel.js';

// A GET reaching the fixture is insufficient evidence that Svelte has hydrated.
test('delayed Svelte DOM after earlier HTTP GET eventually passes',async()=>{
  let clock=0,rendered=false,checks=0;
  await waitForRenderedState(()=>{checks++;return rendered},{label:'BLOB_INVENTORY_RENDERED',timeoutMs:200,intervalMs:20,
    now:()=>clock,pause:async ms=>{clock+=ms;if(clock>=60)rendered=true;}});
  assert.equal(checks,4);
});
test('DOM never hydrates: timeout is FAIL even if GET occurred',async()=>{
  let clock=0,requests=1;
  await assert.rejects(waitForRenderedState(()=>false,{label:'BLOB_INVENTORY_RENDERED',timeoutMs:40,intervalMs:10,
    now:()=>clock,pause:async ms=>{clock+=ms;}}),/BROWSER_WAIT_TIMEOUT: BLOB_INVENTORY_RENDERED/);
  assert.equal(requests,1);
});
test('pre-existing JS exception interrupts polling instead of becoming PASS',async()=>{
  await assert.rejects(waitForRenderedState(()=>true,{beforeProbe:()=>{throw Error('ReferenceError: mount failed');}}),
    /ReferenceError: mount failed/);
});
test('poll does not accept unrelated non-ready DOM state',async()=>{
  let clock=0;const count='0';
  await assert.rejects(waitForRenderedState(()=>count==='1',{label:'EXPECTED_ONE',timeoutMs:5,intervalMs:1,
    now:()=>clock,pause:async ms=>{clock+=ms;}}),/EXPECTED_ONE/);
});
test('compiled Chromium gate waits for data-bound blob counter after the GET',()=>{
  const gate=readFileSync(fileURLToPath(new URL('../../../qa-nfqws/browser-owner-gate.mjs',import.meta.url)),'utf8');
  assert.match(gate,/await until\(async\(\)=>\{[\s\S]*?blobInstalledCount[\s\S]*?BLOB_INVENTORY_DOM_READY/);
  assert.match(gate,/BLOB_INVENTORY_REACTIVE/);
  assert.doesNotMatch(gate,/await until\(async\(\)=>\(await requestLog\(\)\)\.some\(x=>x\.path\.endsWith\('\/blobs'\)[\s\S]{0,120}BLOB_INVENTORY'\);\s*await gate/);
});
