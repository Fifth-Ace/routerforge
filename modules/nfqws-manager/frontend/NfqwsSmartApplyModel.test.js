import test from 'node:test';
import assert from 'node:assert/strict';
import {nfqwsSmartApplyView} from './NfqwsSmartApplyModel.js';
const SHA='e'.repeat(64),OTHER='f'.repeat(64);
const base=()=>({item:{name:'safe',source:'local'},compat:{status:'PASS'},lists:[],blobs:[],blocked:[],merge:{preserved:[]},expectedActiveSha:SHA,configSha:OTHER,ref:'main'});
test('NFQWS Smart Apply view requires exact SHA and explicit open state',()=>{
  assert.equal(nfqwsSmartApplyView({plan:base(),open:false}).canApply,false);
  assert.equal(nfqwsSmartApplyView({plan:base(),open:true}).canApply,true);
  assert.equal(nfqwsSmartApplyView({plan:{...base(),expectedActiveSha:'stale'},open:true}).canApply,false);
  assert.equal(nfqwsSmartApplyView({plan:{...base(),configSha:''},open:true}).canApply,false);
});
test('NFQWS Smart Apply fails closed on blocked dependencies, preparation and pending action',()=>{
  const p={...base(),lists:[{name:'a',state:'write'}],blobs:[{name:'b',state:'write'}],blocked:[{name:'missing',state:'blocked'}]};
  const view=nfqwsSmartApplyView({plan:p,open:true});
  assert.equal(view.canApply,false);assert.equal(view.writes,2);assert.equal(view.blocked.length,1);
  assert.equal(nfqwsSmartApplyView({plan:base(),open:true,busy:true}).canApply,false);
  assert.equal(nfqwsSmartApplyView({plan:base(),open:true,preparing:true}).canApply,false);
  assert.equal(nfqwsSmartApplyView({plan:base(),open:true,error:'403'}).error,'403');
});
