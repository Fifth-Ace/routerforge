import test from 'node:test';
import assert from 'node:assert/strict';
import {domainMode,policyMode,updateModeDraft} from './NfqwsFlowModeModel.js';
const base=Object.freeze({NFQWS_EXTRA_ARGS:'$MODE_AUTO',POLICY_NAME:'Trusted',POLICY_EXCLUDE:'0',MODE_AUTO:'--hostlist=auto.list'});
test('all three MODE_* choices preserve upstream mode definitions',()=>{
 for(const name of ['MODE_LIST','MODE_AUTO','MODE_ALL']){const next=updateModeDraft(base,'domain',name);assert.equal(next.NFQWS_EXTRA_ARGS,'$'+name);assert.equal(next.MODE_AUTO,base.MODE_AUTO);}
 assert.equal(domainMode(base),'MODE_AUTO');
});
test('policy include/exclude preserves name, none clears name without mutating source',()=>{
 const exclude=updateModeDraft(base,'policy','exclude');assert.equal(exclude.POLICY_NAME,'Trusted');assert.equal(exclude.POLICY_EXCLUDE,'1');assert.equal(policyMode(exclude),'exclude');
 const none=updateModeDraft(base,'policy','none');assert.equal(none.POLICY_NAME,'');assert.equal(none.POLICY_EXCLUDE,'0');assert.equal(base.POLICY_NAME,'Trusted');
});
test('include mode with blank name remains staged and requires diagnostics before save',()=>{
 const next=updateModeDraft({POLICY_NAME:'',POLICY_EXCLUDE:'0'},'policy','include');
 assert.equal(next.POLICY_NAME,'');assert.equal(next.POLICY_EXCLUDE,'0');
});
test('unknown modes are refused with no mutation',()=>{
 for(const [kind,mode] of [['domain','MODE_TEST'],['domain',''],['policy','owner'],['unknown','x']]) assert.throws(()=>updateModeDraft(base,kind,mode),/NFQWS_.*_UNKNOWN|NFQWS_.*_INVALID/);
 assert.equal(base.NFQWS_EXTRA_ARGS,'$MODE_AUTO');
});
