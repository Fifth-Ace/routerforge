'use strict';
const fs=require('node:fs');
const path=require('node:path');
const assert=require('node:assert/strict');
const test=require('node:test');
const vm=require('node:vm');
const base=path.resolve(__dirname,'..','modules','dns-router-runtime','frontend');
const html=fs.readFileSync(path.join(base,'index.html'),'utf8');
const js=fs.readFileSync(path.join(base,'app.js'),'utf8');
const css=fs.readFileSync(path.join(base,'module.css'),'utf8');

test('P8.8 starts in simple mode, with original advanced pages retained',()=>{
 for(const id of ['drSimpleWorkspace','drAdvancedWorkspace','drModeSimple','drModeAdvanced','drSimplePolicies','drSimpleDNS','drSimpleDomains','drSimpleAdd','drSimpleSave','drSimpleState','drSimpleRuleList','drSimpleScan']){
  assert.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1,'ID once: '+id);
 }
 assert.match(html,/<div id="drAdvancedWorkspace" hidden>/);
 assert.match(html,/id="drSimpleWorkspace"/);
 assert.ok(html.indexOf('id="drSimpleWorkspace"') < html.indexOf('id="drAdvancedWorkspace"'));
 for(const id of ['dr-tab-overview','dr-tab-policies','dr-tab-testing','dr-tab-keenetic','dr-tab-diagnostics','saveButton','managedTrialN','upstreamForm','drListForm','drGroupForm'])assert.match(html,new RegExp('id="'+id+'"'));
 assert.doesNotMatch(html,/<h1>[^<]*2\.0/);
 assert.match(html,/<h1>DNS Router<\/h1>/);
 assert.match(css,/#drAdvancedWorkspace\[hidden\]/);
 assert.match(js,/p88Id\('drModeAdvanced'\)\.addEventListener\('click'/);
 assert.match(js,/p88Id\('drModeSimple'\)\.addEventListener\('click'/);
});

test('P8.8 is policy-first and never implies DNS activation on save',()=>{
 assert.match(js,/request\('keenetic\/native-dns'\)/);
 assert.match(js,/request\('keenetic\/native-policies'\)/);
 assert.match(js,/p88PolicyName\(instance\.name\)/);
 assert.match(js,/p88RouteHint\(instance\.name\)/);
 assert.match(js,/RFNativeCatalogP89\.entries/);
 assert.match(js,/native:\/\//);
 assert.match(js,/config\.upstreams\.push\(upstream\)/);
 assert.match(js,/config\.rules\.push\(\{id:'domain-'/);
 assert.match(js,/p88Id\('saveButton'\)\.click\(\)/);
 assert.match(js,/Маршрутизация клиентского DNS автоматически НЕ включалась/);
 assert.match(html,/Активное подключение DNS — отдельный этап/);
 assert.doesNotMatch(js,/\.innerHTML\s*=/);
 assert.doesNotMatch(js,/ndmc.*(set|remove|save)/);
});

test('P8.8 performs DNS probe only after user explicitly clicks',()=>{
 const start=js.indexOf('  // P8.8: simple policy -> native DNS -> domain list.');
 const finish=js.indexOf('  load().catch(',start);
 assert.ok(start!==-1&&finish>start);
 const feature=js.slice(start,finish);
 assert.match(feature,/p88Id\('drSimpleProbe'\)\.addEventListener\('click',async/);
 assert.match(feature,/request\('keenetic\/native-dns\/probe\?'/);
 const scan=feature.slice(feature.indexOf('async function p88Scan()'),feature.indexOf("p88Id('drModeSimple').addEventListener"));
 assert.doesNotMatch(scan,/native-dns\/probe/);
 assert.doesNotMatch(scan,/config\.rules\.push|config\.upstreams\.push/);
});

test('P8.8 validates and de-duplicates domain batches before creating draft',()=>{
 const begin=js.indexOf('  function p88ParseDomains(raw){');
 const end=js.indexOf('  function p88PolicyName(name){',begin);
 assert.ok(begin!==-1 && end>begin);
 const parse=vm.runInNewContext('(()=>{'+js.slice(begin,end)+'return p88ParseDomains;})()');
 const basic=parse('YouTube.com\n GOOGLEVIDEO.COM, youtube.com # duplicate\n ytimg.com.');
 assert.equal(basic.error,undefined);
 assert.deepEqual(Array.from(basic.domains),['youtube.com','googlevideo.com','ytimg.com']);
 assert.equal(parse('https://example.com').error.startsWith('Некорректный'),true);
 assert.equal(parse('||ads.example.com^').error.startsWith('Некорректный'),true);
 assert.equal(parse('192.168.1.1').error.startsWith('Некорректный'),true);
 assert.ok(parse(Array.from({length:201},(_,i)=>'d'+i+'.example.com').join('\n')).error);
 assert.ok(parse(Array.from({length:501},()=> 'a.example.com').join('\n')).error);
 assert.deepEqual(Array.from(parse('   # only comment  ').domains),[]);
});
