import test from 'node:test';
import assert from 'node:assert/strict';
import {strategyResources,safeResourceName} from './NfqwsArgumentResourcesModel.js';
test('native Svelte args viewer detects exact hostlist and ipset names',()=>{
 const r=strategyResources('--hostlist=/opt/etc/nfqws2/lists/user.list --ipset=/opt/etc/nfqws2/lists/ipset.list',{knownLists:['user.list']});
 assert.deepEqual(r.map(x=>[x.name,x.status]),[['user.list','available'],['ipset.list','unverified']]);
});
test('base declarations and Lua are read-only, no implicit network or file read',()=>{
 const r=strategyResources('--blob=fake:/opt/share/fake.bin --lua-init=@/opt/share/fake.lua',{kind:'base'});
 assert.deepEqual(r.map(x=>x.type),['blobdef','lua']);assert.equal(r[1].name,'fake.lua');
});
test('unknown untrusted resource names are never suitable for command navigation',()=>{
 assert.equal(safeResourceName('user.list'),true);
 for(const item of ['../../etc/shadow','http://evil/a','..','a..b','x/y',''])assert.equal(safeResourceName(item),false);
});
test('source controls never silently claim missing list or blob exists',()=>{
 const r=strategyResources('--hostlist=/a/missing.list --dpi-desync=fake:blob=unknown',{knownLists:[]});
 assert.equal(r[0].status,'unverified');assert.equal(r[1].status,'unverified');
});
