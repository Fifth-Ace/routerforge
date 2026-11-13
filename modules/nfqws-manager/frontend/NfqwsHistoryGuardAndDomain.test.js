import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {measuredAutoPickEligible,recoveryRegression,guardThresholdFraction} from './NfqwsRunHistoryModel.js';
const source=name=>readFileSync(new URL(name,import.meta.url),'utf8');
test('Auto-pick requires measured ACTIVE Test Lab eligibility on every lower bound',()=>{
  const row={runs:2,targets:3,reach:.75,stable:.60,median_score:80};
  assert.equal(measuredAutoPickEligible(row),true);
  for(const part of [{runs:1},{targets:2},{reach:.7499},{stable:.599},{median_score:'NaN'}])
    assert.equal(measuredAutoPickEligible({...row,...part}),false);
  assert.equal(measuredAutoPickEligible(null),false);
  assert.match(source('NfqwsRuntime.js'),/run.kind!=='testlab'/);
  assert.match(source('NfqwsRunHistory.svelte'),/no blind strategy ranking/);
  assert.doesNotMatch(source('NfqwsRunHistory.svelte'),/method:'POST'|fetch\(|@html|innerHTML/);
});
test('Recovery Guard compares absolute percentage-point drops with inclusive threshold, invalid input fails closed',()=>{
  const b={reach:.95,stable:.80};
  assert.deepEqual(recoveryRegression(b,{reach:.80,stable:.80},15).regressed,true);
  assert.equal(recoveryRegression(b,{reach:.81,stable:.80},15).regressed,false);
  assert.equal(recoveryRegression(b,{reach:.95,stable:.60},20).regressed,true);
  assert.equal(recoveryRegression(b,{reach:.95,stable:.80},20).regressed,false);
  for(const bad of [0,10,12,19,31,50,'lol',null,undefined])assert.throws(()=>guardThresholdFraction(bad),/UNSUPPORTED/);
  for(const bad of [{reach:-1,stable:.9},{reach:1.1,stable:0},{reach:'0.9',stable:.9},null])
    assert.throws(()=>recoveryRegression(b,bad,20),/INVALID/);
});
test('Smart Apply binds guard settings at confirmation and cannot silently trigger recovery without baseline',()=>{
  const runtime=source('NfqwsRuntime.js');
  assert.match(runtime,/recoveryBaseline=recoverySettings.enabled\?latestRecoveryBaselineForCurrent\(\):null/);
  assert.match(runtime,/if\(recoveryBaseline\)void runRecoveryGuard\(recoveryBaseline,recoveryStartedAt,p.item.name,recoverySettings.threshold\)/);
  assert.match(runtime,/if\(recoveryGuardRunning\|\|!baseline\)return/);
  assert.match(runtime,/confirm:'NFQWS_SMART_APPLY'/);
  assert.match(runtime,/confirm:'NFQWS_BACKUP_RESTORE'/);
  assert.doesNotMatch(runtime,/\$\('recoveryGuard(?:Enabled|Threshold|State|Last)'\)/);
});
test('History actions are exactly defined, local-only, and clear confirms user intent',()=>{
  const runtime=source('NfqwsRuntime.js');
  const svelte=source('NfqwsRunHistory.svelte');
  for(const action of ['refresh','settings','clear','copy','copy-one','delete-one'])assert.match(runtime,new RegExp(`name==='${action}'`));
  assert.match(runtime,/NFQWS_HISTORY_ACTION_UNSUPPORTED/);
  assert.match(runtime,/confirm\(`Удалить \$\{runHistory.length\} записей локальной истории\?`\)/);
  assert.match(runtime,/runHistoryMax=24/);
  assert.match(runtime,/emit\(onHistory,\{runs:entries/);
  assert.doesNotMatch(runtime,/\$\('runHistory(?:List|Count|Size|All|Sandbox|TestLab|LatestScore)'\)/);
  assert.match(svelte,/data-svelte-owned="nfqws-run-history"/);
  assert.doesNotMatch(svelte,/onRequest\(|fetch\(|@html|innerHTML/);
});
test('Domain batch check is Svelte-owned, concurrent limit four, stop aborts all requests',()=>{
  const runtime=source('NfqwsRuntime.js'),svelte=source('NfqwsDomainBatchCheck.svelte'),app=source('NFQWSManagerApp.svelte');
  assert.match(runtime,/const gen=\+\+checkGeneration,c=new AbortController\(\)/);
  assert.match(runtime,/Array\.from\(\{length:Math\.min\(4,domains.length\)\},worker\)/);
  assert.match(runtime,/checkAbort\?\.abort\(\)/);
  assert.match(runtime,/signal:c.signal/);
  assert.match(runtime,/if\(gen!==checkGeneration\|\|disposed\|\|c.signal.aborted\)return/);
  assert.doesNotMatch(runtime,/\$\('checkResults'\)\.innerHTML|\$\('checkBar'\)\.style/);
  assert.match(svelte,/data-svelte-owned="nfqws-domain-batch-check"/);
  assert.match(svelte,/onAction\('stop'\)/);
  assert.match(svelte,/onAction\('close'\)/);
  assert.match(app,/<NfqwsDomainBatchCheck snapshot=\{domainCheckView\}/);
  assert.doesNotMatch(svelte,/@html|innerHTML|fetch\(|method:'POST'/);
});
