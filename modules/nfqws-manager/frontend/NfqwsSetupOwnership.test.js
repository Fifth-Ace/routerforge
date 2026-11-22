import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=name=>readFileSync(new URL(name,import.meta.url),'utf8');
test('Setup feedback, summary chips and diff have single Svelte owners, not legacy innerHTML writers',()=>{
 const runtime=source('NfqwsRuntime.js'),app=source('NFQWSManagerApp.svelte');
 for(const component of ['NfqwsSetupChips.svelte','NfqwsSetupDiagnostics.svelte','NfqwsSetupDiff.svelte']){
  assert.match(app,new RegExp(`<${component.replace('.svelte','')}`));
  const view=source(component);assert.match(view,/data-svelte-owned=/);assert.doesNotMatch(view,/@html|innerHTML|fetch\(|method:'POST'/);
 }
 for(const id of ['processingOverview','diagnosticSummary','flowDiagnostics','changeSummary','sumMain','sumQuic','sumUdp','sumIpset','setupState','flowQueue','flowUser','flowConfigVersion','diffBody'])
  assert.doesNotMatch(runtime,new RegExp(`\\$\\('${id}'\\)\\.(?:innerHTML|textContent|className|onclick)`),id);
 assert.doesNotMatch(runtime,/\$\('diffModal'\)\.classList/);
 assert.match(runtime,/emit\(onSetup,\{dirty,changes,diagnostics:diag,overview:processingOverviewData\(\)/);
 assert.match(runtime,/emit\(onDiff,\{open:diffOpen,lines:/);
});
test('Save remains two-step and mutation requires original explicit NFQWS_CONFIG confirmation',()=>{
 const app=source('NFQWSManagerApp.svelte'),runtime=source('NfqwsRuntime.js'),diff=source('NfqwsSetupDiff.svelte');
 assert.match(app,/onclick=\{\(\)=>setupAction\('preview'\)\}>Сохранить и применить/);
 assert.match(runtime,/if\(!confirm\('Сохранить настройки nfqws2.conf и выполнить безопасный reload\?'\)\)return/);
 assert.match(runtime,/confirm:'NFQWS_CONFIG'/);
 assert.match(runtime,/if\(!flowDirty\(\)\|\|busy\)return/);
 assert.match(diff,/onAction\('apply'\)/);
 assert.doesNotMatch(diff,/onRequest\(|method:/);
});
test('Critical diagnostic blocks apply without disabling visual Diff or reset',()=>{
 const runtime=source('NfqwsRuntime.js'),app=source('NFQWSManagerApp.svelte');
 assert.match(runtime,/canApply:dirty&&!busy&&diag.bad===0/);
 assert.match(app,/disabled=\{!setupView.canApply\|\|serviceBusy\}/);
 assert.match(app,/disabled=\{!setupView.dirty\|\|serviceBusy\} onclick=\{\(\)=>setupAction\('preview'\)\}/);
 assert.match(runtime,/NFQWS_SETUP_COMMAND_UNSUPPORTED/);
});
test('Unsafe plain text from diagnostics/diff uses escaped Svelte interpolation only',()=>{
 for(const name of ['NfqwsSetupDiff.svelte','NfqwsSetupDiagnostics.svelte','NfqwsSetupChips.svelte']){
  const src=source(name);
  assert.doesNotMatch(src,/@html|\.innerHTML|\.outerHTML|insertAdjacentHTML|new Function|eval\(/);
  assert.match(src,/\{(?:line\.t|item\[1\]|item\.label)\}/);
 }
});
