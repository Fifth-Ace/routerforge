import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const root = new URL('../', import.meta.url);
const source = readFileSync(new URL('NetworkToolsApp.svelte', root), 'utf8');
const entry = readFileSync(new URL('main.js', root), 'utf8');
const html = readFileSync(new URL('index.html', root), 'utf8');
const legacyIds = JSON.parse(readFileSync(new URL('legacy_dom_ids.json', import.meta.url), 'utf8'));
const idSet = new Set([...source.matchAll(/id="([^"]+)"/g)].map(x=>x[1]));
test('Network Tools Svelte 5 owns UI, original app.js is never loaded', () => {
  assert.match(source, /\$state\(/);assert.match(source, /onMount\(/);
  assert.match(entry, /from '\.\/NetworkToolsApp\.svelte'/);
  assert.doesNotMatch(entry + html, /app\.js/);
  assert.doesNotMatch(source, /import\s+.*legacy.*app\.js/);
});
test('all original static DOM ID contracts remain available', () => {
  for(const id of legacyIds) assert.ok(idSet.has(id), `missing id ${id}`);
});
test('doctor, routes, flows and probes are Svelte owned',()=>{
  for(const phrase of ['runDoctor(', 'runRoute(', 'refreshFlows(', 'runProbe(', 'onkeydown', 'onclick'])
    assert.ok(source.includes(phrase),`missing ${phrase}`);
});
test('safe original API client remains separate from frontend controller',()=>{
  assert.match(entry,/\.\/runtime\.js/);
  assert.doesNotMatch(entry,/\.\/app\.js/);
});
