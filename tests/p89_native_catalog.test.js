'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const ui=path.resolve(__dirname,'../modules/dns-router-runtime/frontend');
const load=()=>import(pathToFileURL(path.join(ui,'DnsRuntimeNativeCatalog.js')).href);
const read=name=>fs.readFileSync(path.join(ui,name),'utf8');
function sample(){return {available:true,parsed_instances:2,instances:[
  {name:'System',valid:true,endpoints:[
    {address:'127.0.0.1:40500',domain:'.',upstream_hint:'9.9.9.9'},
    {address:'127.0.0.1:40503',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'},
    {address:'127.0.0.1:40504',domain:'su'},
    {address:'127.0.0.1:40505',domain:'xn--p1ai',upstream_hint:'dns.example'}]},
  {name:'Policy1',valid:true,endpoints:[{address:'127.0.0.1:40534',domain:'.',upstream_hint:'1.1.1.1'}]}
]};}

test('P89 native catalog is a shipped pure module and never invents encryption or routes',async()=>{
  const c=await load();
  const list=c.entries(sample(),'System');
  assert.equal(list.length,4);
  assert.deepEqual(list.map(x=>x.zone),['.','ru','su','xn--p1ai']);
  assert.equal(list[0].selector,'native://System/./9.9.9.9');
  assert.equal(list[1].selector,'native://System/ru/common.dot.dns.yandex.net');
  assert.equal(list[2].usable,false);
  assert.match(list[2].reason,/идентификатора/);
  for(const item of list){assert.equal(item.transport,'unknown');assert.equal(item.encryptionProven,false);}
  assert.equal(c.entries(sample(),'Policy1').length,1);
});

test('P89 only accepts originally declared DNS zones and refuses cross-policy guesses',async()=>{
  const c=await load();
  const [root,ru,,xn]=c.entries(sample(),'System');
  assert.equal(c.acceptsDomains(root,['youtube.com','googlevideo.com']).ok,true);
  assert.equal(c.acceptsDomains(ru,['example.ru','a.example.ru']).ok,true);
  assert.equal(c.acceptsDomains(ru,['youtube.com']).ok,false);
  assert.equal(c.acceptsDomains(xn,['example.xn--p1ai']).ok,true);
  assert.equal(c.acceptsDomains(xn,['example.ru']).ok,false);
  assert.equal(c.acceptsDomains(null,['example.ru']).ok,false);
  assert.equal(c.entries(sample(),'Policy../../../x').length,0);
  assert.equal(c.entries(sample(),'Policy9').length,0);
});

test('P89 fails closed on incomplete, duplicate and untrusted policy inventory',async()=>{
  const c=await load();
  const missing=sample();missing.available=false;
  assert.equal(c.entries(missing,'System').every(x=>!x.usable),true);
  const partial=sample();partial.parsed_instances=1;
  assert.equal(c.entries(partial,'System').every(x=>!x.usable),true);
  const dup=sample();dup.instances[0].endpoints.push({address:'127.0.0.1:40509',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'});
  assert.equal(c.entries(dup,'System').filter(x=>x.zone==='ru').every(x=>!x.usable),true);
  const invalid=sample();invalid.instances[0].valid=false;
  assert.equal(c.entries(invalid,'System').every(x=>!x.usable),true);
});

test('P89 Svelte store and catalog UI own the real inventory; no legacy source execution',()=>{
  const store=read('DnsRuntimeStore.svelte.js');
  const uiSource=read('DnsRuntimeP88Views.svelte');
  const html=read('index.html');
  const catalog=read('DnsRuntimeNativeCatalog.js');
  assert.match(store,/DnsRuntimeNativeCatalog\.js/);
  assert.match(store,/nativeEntries/);
  assert.match(store,/acceptsDomains/);
  assert.match(uiSource,/id="drSimpleDNSInventory"/);
  assert.doesNotMatch(html,/id="drSimpleDNSInventory"/);
  assert.match(catalog,/export \{entries, acceptsDomains\}/);
  assert.doesNotMatch(catalog,/\.innerHTML\s*=|\bfetch\s*\(|\bndmc\b/);
});
