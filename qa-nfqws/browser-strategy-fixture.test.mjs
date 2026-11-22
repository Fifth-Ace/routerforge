import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {strategyFixtureReply,STRATEGY_LIBRARY_FIXTURE} from './browser-strategy-fixture.mjs';
import {verifiedStrategyLibraryResponse} from '../modules/nfqws-manager/frontend/NfqwsStrategyLibraryModel.js';

test('fixture GET exactly matches authoritative Go strategy library envelope',()=>{
 const res=strategyFixtureReply('GET','/v2/strategies');
 assert.equal(res.status,200);assert.deepEqual(verifiedStrategyLibraryResponse(res.payload),[]);
 assert.deepEqual(res.payload,STRATEGY_LIBRARY_FIXTURE);
});
test('fixture rejects wrong methods and never returns mutation success',()=>{
 assert.equal(strategyFixtureReply('POST','/v2/strategies').status,405);
 assert.equal(strategyFixtureReply('GET','/v2/strategies/save').status,405);
 assert.equal(strategyFixtureReply('GET','/v2/strategies/delete').status,405);
 for(const action of ['save','delete']){
   assert.deepEqual(strategyFixtureReply('POST',`/v2/strategies/${action}`,'{}'),{status:403,payload:{error:'CONFIRM_REQUIRED'}});
   assert.equal(strategyFixtureReply('POST',`/v2/strategies/${action}`,JSON.stringify({confirm:action==='save'?'ROUTERFORGE_V2_STRATEGY_SAVE':'ROUTERFORGE_V2_STRATEGY_DELETE'})).payload.error,'TEST_ONLY_NO_PERSISTENCE');
 }
 assert.equal(strategyFixtureReply('GET','/other'),null);
});
test('real Chromium harness uses identical strategy fixture for HTTP and CDP, and checks both navigation namespaces',()=>{
 const src=readFileSync(new URL('./browser-owner-gate.mjs',import.meta.url),'utf8');
 assert.match(src,/const strategyReply=strategyFixtureReply\(req.method,p,body\)/);
 assert.match(src,/case '\/v2\/strategies':payload=\$\{JSON\.stringify\(STRATEGY_LIBRARY_FIXTURE\)\};break;/);
 assert.match(src,/BROWSER_GATE PASS \/ STRATEGY_LIBRARY_READ_ONLY/);
 assert.ok(src.includes('[data-primary-tab="configuration"]'));
 assert.match(src,/CONFIG_LEAF_DIRECT_ROUTE/);
});
