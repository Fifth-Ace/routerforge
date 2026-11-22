// Design-time guard for *coverage definitions*, never evidence of browser PASS.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const dir=path.dirname(fileURLToPath(import.meta.url));
const text=fs.readFileSync(path.join(dir,'browser-e2e.mjs'),'utf8');
const ids=[...text.matchAll(/await saveCase\('RTC3-(\d{2})'/g)].map(m=>m[1]);
test('34 exact RT-C3 browser case IDs are present without duplicates',()=>{
 assert.equal(ids.length,34);
 assert.equal(new Set(ids).size,34);
 assert.deepEqual([...ids].sort(),Array.from({length:34},(_,i)=>String(i+1).padStart(2,'0')));
});
test('browser suite relies on actual compiled Svelte and CDP events, not synthetic DOM clicks',()=>{
 assert.match(text,/svelte-mode\.js/);
 assert.match(text,/component-gate\.js/);
 assert.match(text,/Input\.dispatchMouseEvent/);
 assert.match(text,/Input\.insertText/);
 assert.match(text,/window\.__rfRuntimeComponentTest\.unmount\(\)/);
 assert.doesNotMatch(text,/\.click\(\).*?\.dispatchEvent/);
});
test('mock is loopback-only and fail-closed for unmocked browser mutations',()=>{
 assert.match(text,/server\.listen\(0,'127\.0\.0\.1'/);
 assert.match(text,/Fetch\.failRequest/);
 assert.match(text,/UNMOCKED_MUTATION_ATTEMPT/);
 assert.match(text,/UNEXPECTED_EXTERNAL_BROWSER_REQUEST_BLOCKED/);
 assert.match(text,/fixture\.unknownWrites\.push\(call\)/);
});
test('case completion is verified from independent on-disk evidence, not case declaration',()=>{
 const evidence=fs.readFileSync(path.join(dir,'verify-browser-evidence.mjs'),'utf8');
 assert.match(evidence,/EXPECTED_CASE_IDS/);
 assert.match(evidence,/case_\.status\s*!==\s*'PASS'/);
 assert.match(evidence,/CASE_LOG_MISSING/);
 assert.match(evidence,/SCOPE_MISMATCH/);
});
