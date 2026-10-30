'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const ui=path.join(root,'modules/dns-router-runtime/frontend');
const vm=require('node:vm');
const appSource=fs.readFileSync(path.join(ui,'app.js'),'utf8');
const start=appSource.indexOf('  // P8.9 NATIVE CATALOG BEGIN');
const end=appSource.indexOf('  // P8.9 NATIVE CATALOG END',start);
assert.ok(start>=0 && end>start, 'catalog embedded in the actual shipped app.js');
const c=vm.runInNewContext('(()=>{const window={};'+appSource.slice(start,end)+';return window.RFNativeCatalogP89;})()');
function sample(){return {available:true,parsed_instances:2,instances:[
  {name:'System',valid:true,endpoints:[
    {address:'127.0.0.1:40500',domain:'.',upstream_hint:'9.9.9.9'},
    {address:'127.0.0.1:40503',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'},
    {address:'127.0.0.1:40504',domain:'su'},
    {address:'127.0.0.1:40505',domain:'xn--p1ai',upstream_hint:'dns.example'}]},
  {name:'Policy1',valid:true,endpoints:[{address:'127.0.0.1:40534',domain:'.',upstream_hint:'1.1.1.1'}]}
]};}
test('P89 exposes root, zone-bound and unlabelled native endpoints without inventing transport',()=>{
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
test('P89 allows only queries inside the originally declared Keenetic zone',()=>{
  const [root,ru,,xn]=c.entries(sample(),'System');
  assert.equal(c.acceptsDomains(root,['youtube.com','googlevideo.com']).ok,true);
  assert.equal(c.acceptsDomains(ru,['example.ru','a.example.ru']).ok,true);
  assert.equal(c.acceptsDomains(ru,['youtube.com']).ok,false);
  assert.equal(c.acceptsDomains(xn,['example.xn--p1ai']).ok,true);
  assert.equal(c.acceptsDomains(xn,['example.ru']).ok,false);
  assert.equal(c.acceptsDomains(null,['example.ru']).ok,false);
});
test('P89 fail-closed on incomplete, duplicate, invalid and unsafe inventories',()=>{
  const missing=sample();missing.available=false;
  assert.equal(c.entries(missing,'System').every(x=>!x.usable),true);
  const dup=sample();dup.instances[0].endpoints.push({address:'127.0.0.1:40509',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'});
  assert.equal(c.entries(dup,'System').filter(x=>x.zone==='ru').every(x=>!x.usable),true);
  const bad=sample();bad.instances[0].valid=false;
  assert.equal(c.entries(bad,'System').every(x=>!x.usable),true);
  assert.equal(c.entries(sample(),'Policy../../../x').length,0);
  assert.equal(c.entries(sample(),'Policy9').length,0);
});
test('P89 simple UI integrates complete endpoint list, safe domain gate and no implicit DNS request',()=>{
  const js=fs.readFileSync(path.join(ui,'app.js'),'utf8');
  const html=fs.readFileSync(path.join(ui,'index.html'),'utf8');
  assert.match(js,/RFNativeCatalogP89\.entries/);
  assert.match(js,/RFNativeCatalogP89\.acceptsDomains/);
  assert.match(js,/drSimpleDNSInventory/);
  assert.match(js,/p89NativeEntries/);
  assert.match(html,/id="drSimpleDNSInventory"/);
  assert.match(js,/P8\.9 NATIVE CATALOG BEGIN/);
  assert.doesNotMatch(js,/\.innerHTML\s*=/);
  assert.doesNotMatch(js,/ndmc.*(set|remove|save)/);
});
