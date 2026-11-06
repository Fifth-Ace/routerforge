import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const root = new URL('../', import.meta.url);
const source = readFileSync(new URL('AntiscanManagerApp.svelte', root), 'utf8');
const entry = readFileSync(new URL('main.js', root), 'utf8');
const html = readFileSync(new URL('index.html', root), 'utf8');
const legacyIds = JSON.parse(readFileSync(new URL('legacy_dom_ids.json', import.meta.url), 'utf8'));
const idSet = new Set([...source.matchAll(/id="([^"]+)"/g)].map(x=>x[1]));
test('Antiscan Svelte 5 owns UI, no imported legacy app.js', () => {
  assert.match(source,/\$state\(/); assert.match(source,/onMount\(/);
  assert.match(entry,/AntiscanManagerApp\.svelte/);
  assert.doesNotMatch(entry+html,/app\.js/);
});
test('all original static DOM ID contracts remain available', () => {
  for(const id of legacyIds)assert.ok(idSet.has(id),`missing id ${id}`);
});
test('config form follows browser string value contract for number inputs',()=>{
  assert.match(source,/k==='value'\?String\(target\.value \?\? ''\)/);
  assert.match(source,/configBaseSHA/);
  assert.match(source,/configDirty/);
});
test('service operations are bound to individual controls, not whole sections',()=>{
  assert.doesNotMatch(source,/<section[^>]*onclick=/);
  assert.match(source,/document\.addEventListener\('click', handleDelegation\)/);
  assert.match(source,/document\.removeEventListener\('click', handleDelegation\)/);
});
