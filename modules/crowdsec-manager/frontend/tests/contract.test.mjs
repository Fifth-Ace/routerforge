import { readFileSync } from 'node:fs';
import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { createHash } from 'node:crypto';
const root = new URL('../', import.meta.url);
const source = readFileSync(new URL('CrowdSecManagerApp.svelte', root), 'utf8');
const css = readFileSync(new URL('module.css', root));
const expectedCSS = '12adf3b30250459065db78d9f179ff7bcf9a0441';
function blobSha(bytes) {
  return createHash('sha1').update(`blob ${bytes.length}\0`).update(bytes).digest('hex');
}
const ids = [
  'refresh','verdict','bouncer-install','install-platform','install-form','install-url','install-key',
  'install-start','install-release','install-remove','install-preserve-wrap','install-keep-config',
  'install-progress','protection','protection-hint','running','installed','firewall','firewall-hint',
  'hook','hook-hint','init-status','lapi-form','lapi-url','lapi-key','key-present','tls-state',
  'lapi-probe','tls-waiver-panel','tls-waiver-status','tls-waiver-confirm-wrap','tls-waiver-confirm',
  'tls-waiver-accept','tls-waiver-revoke','hook-hint-long','hook-install','hook-remove','chain-hint',
  'counters','updated','sets','metrics-state','metrics','notices','toast'
];
test('all legacy element IDs retained, one per DOM owner', () => {
  const actual = Array.from(source.matchAll(/\bid="([^"]+)"/g), x=>x[1]);
  assert.deepEqual(actual, ids);
});
test('visual CSS byte-for-byte unchanged from pinned source', () => {
  assert.equal(blobSha(css), expectedCSS);
});
test('Svelte is the actual reactive controller, not legacy DOM wrapper', () => {
  for (const banned of ['document.getElementById', 'document.querySelectorAll', 'innerHTML',
     'window.eval', 'new Function(', 'app.js', 'onMount(() => import(']) {
    assert.equal(source.includes(banned), false, banned);
  }
  for (const required of ['onMount', 'bind:value', 'bind:checked', 'onsubmit=', 'onclick=', '{#each', 'API + path']) {
    assert.ok(source.includes(required), required);
  }
});
test('existing backend contract and user consent still explicit', () => {
  for (const required of ["api('overview')", "api('install/status')", "api('install/remove'",
    "api('install/start'", "action('tls-waiver'", "action('lapi'", "action('hook'",
    "action('service'", 'window.confirm(question)', '/api/modules/crowdsec-manager/']) {
    assert.ok(source.includes(required), required);
  }
});
