import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const root = new URL('../', import.meta.url);
const read = file => readFileSync(new URL(file, root), 'utf8');
const source = read('NetworkToolsApp.svelte');
const entry = read('main.js');
const html = read('index.html');
const legacyIds = JSON.parse(readFileSync(new URL('legacy_dom_ids.json', import.meta.url), 'utf8'));
const idSet = new Set([...source.matchAll(/id="([^"]+)"/g)].map(x=>x[1]));
test('Network Tools Svelte 5 owns UI, original app.js is never loaded', () => {
  assert.match(source, /\$state\(/); assert.match(source, /onMount\(/);
  assert.match(entry, /from '\.\/NetworkToolsApp\.svelte'/);
  assert.doesNotMatch(entry + html, /app\.js/);
  assert.doesNotMatch(source, /import\s+.*legacy.*app\.js/);
});
test('all original static DOM ID contracts remain available', () => {
  for (const id of legacyIds) assert.ok(idSet.has(id), `missing id ${id}`);
});
test('doctor, routes, flows and probes are Svelte owned', () => {
  for (const phrase of ['runDoctor(', 'runRoute(', 'refreshFlows(', 'runProbe(', 'onkeydown', 'onclick'])
    assert.ok(source.includes(phrase), `missing ${phrase}`);
});
test('safe API adapter remains separate from Svelte controller and legacy app.js', () => {
  assert.match(entry, /mount\(NetworkToolsApp,\s*\{\s*target:/);
  assert.match(source, /import\s+\w+\s+from\s+['"]\.\/runtime\.js['"]/);
  const adapter = read('runtime.js');
  assert.match(adapter, /export default/);
  assert.doesNotMatch(entry + source, /import\s+[^\n]*['"]\.\/app\.js['"]/);
  assert.doesNotMatch(entry + html, /app\.js/);
});
