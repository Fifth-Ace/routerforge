import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {NFQWS_PRIMARY_DEFAULTS,NFQWS_PRIMARY_GROUPS,primaryForLeaf,resolveLeafTab,resolvePrimaryTab} from './NfqwsNavigationModel.js';

test('primary strategies and Configuration leaf strategies remain different routes',()=>{
 assert.equal(resolveLeafTab('strategies'),'strategies');
 assert.equal(primaryForLeaf('strategies'),'configuration');
 assert.equal(resolvePrimaryTab('strategies'),'strategy-library-v2');
 assert.equal(primaryForLeaf('strategy-library-v2'),'strategies');
 assert.equal(resolvePrimaryTab('configuration'),'strategies');
});
test('last visited group leaf is restored without hijacking a direct leaf click',()=>{
 const history={...NFQWS_PRIMARY_DEFAULTS,configuration:'config',strategies:'strategy-import'};
 assert.equal(resolvePrimaryTab('configuration',history),'config');
 assert.equal(resolvePrimaryTab('strategies',history),'strategy-import');
 assert.equal(resolveLeafTab('strategies'),'strategies');
 assert.equal(resolveLeafTab('config'),'config');
});
test('navigation forbids unknown tabs, group confusion and cross-group history',()=>{
 for(const name of ['strategys','__proto__','delete','',null,undefined,'../../router']){
   assert.throws(()=>resolveLeafTab(name),/NFQWS_NAV_UNKNOWN_LEAF/);
   assert.throws(()=>resolvePrimaryTab(name),/NFQWS_NAV_UNKNOWN_GROUP/);
 }
 assert.throws(()=>resolvePrimaryTab('configuration',{configuration:'strategy-library-v2'}),/NFQWS_NAV_INVALID_GROUP_HISTORY/);
 assert.throws(()=>resolvePrimaryTab('strategies',{strategies:'strategies'}),/NFQWS_NAV_INVALID_GROUP_HISTORY/);
 assert.equal(resolvePrimaryTab('menu',{}),'catalog');
 assert.equal(primaryForLeaf('overview'),'overview');
});
test('each Svelte leaf belongs to exactly one primary group',()=>{
 const seen=new Set();
 for(const [group,tabs] of Object.entries(NFQWS_PRIMARY_GROUPS)){
   assert.ok(tabs.length>0);assert.equal(tabs.includes(NFQWS_PRIMARY_DEFAULTS[group]),true);
   for(const tab of tabs){assert.equal(seen.has(tab),false,`DUPLICATE_LEAF ${tab}`);seen.add(tab);assert.equal(primaryForLeaf(tab),group);}
 }
});
test('Svelte binds direct leaf and primary group to separate resolvers',()=>{
 const source=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
 assert.match(source,/runtime\.navigate\(resolveLeafTab\(tab\)\)/);
 assert.match(source,/runtime\.navigate\(resolvePrimaryTab\(group,lastTab\)\)/);
 assert.match(source,/data-primary-tab="strategies" onclick=\{\(\) => navigatePrimary\('strategies'\)\}/);
 assert.match(source,/data-tab="strategies" onclick=\{\(\) => navigateTab\('strategies'\)\}/);
 assert.doesNotMatch(source,/runtime\.navigate\(lastTab\[tab\]\s*\|\|\s*tab\)/);
});
