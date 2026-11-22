import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {autoSetupPath,autoSetupTarget,autoSetupObserved,autoSetupDiagnosisStop,autoSetupStageSummary,autoSetupErrorCode,autoSetupLabel,autoSetupBlockerText,autoSetupTCP16Body} from './NfqwsAutoSetupModel.js';
const sha='f'.repeat(64);
test('Auto Setup GET and POST only exact same-origin endpoints, never arbitrary URL',()=>{
  assert.equal(autoSetupPath('/v2/observed-targets'),'/v2/observed-targets');
  assert.equal(autoSetupPath('/v2/detect','POST'),'/v2/detect');
  for(const url of ['https://evil.test','//evil.test','/v2/selector','/status?foo=1'])assert.throws(()=>autoSetupPath(url));
  assert.throws(()=>autoSetupPath('/v2/detect','GET'));
});
test('All three target sources are exact with no silent fallback',()=>{
  assert.equal(autoSetupTarget({source:'manual',target:'a.example'}),'a.example');
  assert.equal(autoSetupTarget({source:'observed',observed:'b.example'}),'b.example');
  assert.equal(autoSetupTarget({source:'list',listTarget:'c.example'}),'c.example');
  for(const source of ['unknown','',null])assert.throws(()=>autoSetupTarget({source,target:'a.example'}));
});
test('Empty, arbitrary whitespace and control target fail closed before inspect',()=>{
  for(const target of ['','x\ny','a b','a'.repeat(254)])assert.throws(()=>autoSetupTarget({target}));
});
test('Observed filters IPv4-only values and preserves source metadata',()=>{
  const rows=[{target:'one.example',count:3},{destination:'1.2.3.4',count:2},{destination:'two.example',reason:'tls'}];
  const out=autoSetupObserved(rows);assert.deepEqual(out.map(x=>x.name),['one.example','two.example']);assert.equal(out[0].count,3);
  assert.throws(()=>autoSetupObserved(null));
});
test('Healthy baseline must not start Selector or mutate configuration',()=>{
  assert.equal(autoSetupDiagnosisStop({classification:'clear'})?.[0],'TARGET_ALREADY_HEALTHY');
  assert.equal(autoSetupDiagnosisStop({diagnostic:{code:'CLEAR_END_TO_END'}})?.[0],'TARGET_ALREADY_HEALTHY');
});
test('DNS and routing faults must stop before unsafe bypass candidate search',()=>{
  assert.equal(autoSetupDiagnosisStop({diagnostic:{code:'DNS_NXDOMAIN'}})?.[0],'DNS_FAILURE');
  assert.equal(autoSetupDiagnosisStop({diagnostic:{fault_domain:'routing'}})?.[0],'ROUTING_FAILURE');
  assert.equal(autoSetupDiagnosisStop({diagnostic:{code:'HTTP_STREAM_CUTOFF_12_20K_SUSPECTED'}}),null);
});
test('Diagnostic stage is explicitly DNS TCP TLS HTTP',()=>{
  assert.equal(autoSetupStageSummary({stages:{dns:{state:'PASS'},tcp:{state:'FAIL'},tls:{state:'skipped'},http:{state:'WORKING'}}}),'DNS ✓ · TCP ✕ · TLS — · HTTP ✓');
});
test('Failure categories remain deterministic and never imply success',()=>{
  assert.equal(autoSetupErrorCode('cleanup baseline failed'),'CLEANUP_FAILURE');
  assert.equal(autoSetupErrorCode('active config changed'),'CONFIG_CHANGED_WHILE_TESTING');
  assert.equal(autoSetupErrorCode('rolled back'),'APPLY_ROLLBACK');
  assert.equal(autoSetupErrorCode('capability gates'),'DEPENDENCY_MISSING');
  assert.equal(autoSetupErrorCode('preview expired'),'PREVIEW_STALE');
});
test('Diagnostic status labels and blocker remain readable without masking code',()=>{
  assert.equal(autoSetupLabel('TARGET_ALREADY_HEALTHY'),'Сайт уже работает');
  assert.match(autoSetupBlockerText('active management ssh session'),/SSH/);
});
test('TCP16 only uses exact SHA and explicit safe probe token',()=>{
  assert.deepEqual(autoSetupTCP16Body(sha),{scan_sni:true,expected_config_sha256:sha,confirm:'ROUTERFORGE_TCP16_PROBE_V2'});
  assert.throws(()=>autoSetupTCP16Body('bad'));
});
test('Native Svelte owns Auto Setup endpoints, status and Apply, never raw HTML',()=>{
  const app=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
  const ui=readFileSync(new URL('./NfqwsAutoSetup.svelte',import.meta.url),'utf8');
  const legacy=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
  assert.ok(app.includes('<NfqwsAutoSetup active='));
  assert.ok(ui.includes('selector.runExternal'));
  assert.ok(ui.includes('selector.previewExternal'));
  assert.ok(ui.includes('selector.applyExternal'));
  assert.ok(ui.includes('data-svelte-owned="nfqws-auto-setup-v1"'));
  assert.ok(!ui.includes('{@html'));
  assert.ok(!legacy.includes("$('v2AutoSetup"));
  assert.ok(!legacy.includes('function runV2AutoSetup'));
});
test('No browser persistence, implicit save or automatic Apply on component mount',()=>{
  const ui=readFileSync(new URL('./NfqwsAutoSetup.svelte',import.meta.url),'utf8');
  const mount=ui.match(/onMount\(\(\)=>\{[^\n]*/)?.[0]||'';
  assert.ok(!mount.includes('apply('));
  assert.ok(!ui.includes('localStorage'));
  assert.ok(!ui.includes('fetch('));
});
