import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {readAntiscanConfigForm} from '../AntiscanConfigModel.js';

const root = new URL('../', import.meta.url);
const read = name => readFileSync(new URL(name, root), 'utf8');
const source = read('AntiscanManagerApp.svelte');
const entry = read('main.js');
const html = read('index.html');
const config = read('AntiscanConfigEditor.svelte');
const schedule = read('AntiscanScheduleEditor.svelte');
const api = read('AntiscanApi.js');
const legacyIds = JSON.parse(readFileSync(new URL('legacy_dom_ids.json', import.meta.url), 'utf8'));

function reachableSvelteFiles() {
  const visited = new Map();
  const walk = filename => {
    if (visited.has(filename)) return;
    const content = read(filename);
    visited.set(filename, content);
    for (const match of content.matchAll(/import\s+(\w+)\s+from\s+['"]\.\/(\w+\.svelte)['"]/g)) {
      const [, component, child] = match;
      assert.ok(content.includes(`<${component}`), `${filename} imports but does not mount ${component}`);
      walk(child);
    }
  };
  walk('AntiscanManagerApp.svelte');
  return visited;
}

test('Antiscan Svelte 5 owns UI, no imported legacy app.js', () => {
  assert.match(source, /\$state\(/);
  assert.match(source, /onMount\(/);
  assert.match(entry, /AntiscanManagerApp\.svelte/);
  assert.doesNotMatch(entry + html, /app\.js/);
});

test('all original static DOM ID contracts belong to exactly one reachable Svelte owner', () => {
  const ids = new Map();
  for (const [filename, content] of reachableSvelteFiles()) {
    for (const match of content.matchAll(/\bid="([^"]+)"/g)) {
      const id = match[1];
      assert.ok(!ids.has(id), `duplicate id ${id}: ${ids.get(id)} / ${filename}`);
      ids.set(id, filename);
    }
  }
  for (const id of legacyIds) assert.ok(ids.has(id), `missing id ${id}`);
  assert.equal(ids.get('schedulerState'), 'AntiscanScheduleEditor.svelte');
  assert.equal(ids.get('configForm'), 'AntiscanConfigEditor.svelte');
});

test('configuration number fields preserve numeric payload with Svelte bind:value', () => {
  assert.match(source, /readAntiscanConfigForm\(configControls\)/);
  assert.match(config, /id="cfgRecentTime"[^>]*type="number"[^>]*bind:value=\{controls\["cfgRecentTime"\]\.value\}/);
  assert.match(config, /oninput=\{onChanged\}/);
  assert.match(config, /onchange=\{onChanged\}/);
  const input = new Proxy({}, {get(_target, id) {
    return {value: id === 'cfgRecentTime' ? 12 : id === 'cfgRecentHitcount' ? ' 15 ' : id === 'cfgRecentLimit' ? '' : '', checked: false};
  }});
  const payload = readAntiscanConfigForm(input);
  assert.equal(payload.recent_connections_time_seconds, 12);
  assert.equal(payload.recent_connections_hitcount, 15);
  assert.equal(payload.recent_connections_limit, 0);
  assert.equal(typeof payload.recent_connections_hitcount, 'number');
});

test('protected actions use explicit Svelte button handlers and confirmed server mutations', () => {
  assert.doesNotMatch(source + config + schedule, /<section[^>]*onclick=/);
  assert.doesNotMatch(source + config + schedule, /document\.addEventListener\(['"]click['"]/);
  assert.doesNotMatch(source + config + schedule, /document\.querySelectorAll\(/);
  assert.match(source, /<AntiscanConfigEditor[^>]*bind:controls=\{configControls\}/);
  assert.match(source, /<AntiscanScheduleEditor[^>]*bind:controls=\{schedulerControls\}/);
  assert.match(config, /onclick=\{onApply\}/);
  assert.match(schedule, /onclick=\{onApply\}/);
  const applyConfigHead = source.split('async function applyConfigEditor() {')[1]?.split('configBusy = true')[0] || '';
  assert.match(applyConfigHead, /if\s*\(!window\.confirm\(/);
  assert.match(source, /base_sha256:\s*configBaseSHA,\s*confirm:\s*'APPLY_CONFIG'/);
  assert.match(api, /export function mutate\(/);
  assert.match(api, /method:\s*'POST'/);
  assert.match(api, /credentials:\s*'same-origin'/);
});
