'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const ui=path.resolve(__dirname,'../modules/dns-router-runtime/frontend');
const read=name=>fs.readFileSync(path.join(ui,name),'utf8');
const load=name=>import(pathToFileURL(path.join(ui,name)).href);

test('P88 simple UI is mounted by Svelte and uses explicit scan/probe/draft actions',()=>{
  const app=read('DnsRuntimeApp.svelte');
  const views=read('DnsRuntimeP88Views.svelte');
  const actions=read('DnsRuntimeP88Actions.svelte');
  const html=read('index.html');
  const css=read('module.css');
  for(const id of ['drSimpleWorkspace','drAdvancedWorkspace','drSimpleDNS','drSimpleDomains'])
    assert.equal((app.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  for(const id of ['drSimplePolicies','drSimpleDNSInventory','drSimpleRuleList'])
    assert.equal((views.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  for(const id of ['drSimpleScan','drSimpleProbe','drSimpleAdd','drP10CancelListEdit','drSimpleSave','drSimpleAdvanced'])
    assert.equal((actions.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  assert.match(app,/<DnsRuntimeP88Actions scope="scan"/);
  assert.match(app,/<DnsRuntimeP88Views scope="rules"/);
  assert.match(actions,/onclick=\{scan\}/);
  assert.match(actions,/onclick=\{probe\}/);
  assert.match(actions,/onclick=\{cancel\}/);
  assert.match(app,/hidden=\{shell\.mode!=='simple'\}/);
  assert.match(css,/grid-template-columns:minmax\(0,2fr\) minmax\(0,3fr\)/);
  assert.doesNotMatch(html,/id="drSimpleScan"|id="drSimpleRuleList"|id="upstreamForm"/);
  assert.match(html,/src="svelte-mode\.js"/);
});

test('P88 pure parse and draft addition are deduplicated, read-only and zone-scoped',async()=>{
  const {parseDomainBatch}=await load('DnsRuntimeP92Logic.js');
  const {planDomainAddition,nativeProbeTarget}=await load('DnsRuntimeP88ActionLogic.js');
  const {acceptsDomains}=await load('DnsRuntimeNativeCatalog.js');
  const parsed=parseDomainBatch('YouTube.com\nGOOGLEVIDEO.COM, youtube.com # duplicate\n ytimg.com.');
  assert.deepEqual(parsed.domains,['youtube.com','googlevideo.com','ytimg.com']);
  for(const bad of ['https://example.com','||ads.example.com^','192.168.1.1'])assert.match(parseDomainBatch(bad).error,/Некорректный/);
  assert.match(parseDomainBatch(Array.from({length:201},(_,i)=>'d'+i+'.example.com').join('\n')).error,/200/);
  const config={upstreams:[],rules:[]};
  const before=JSON.stringify(config);
  const selected='native://System/./1.1.1.1';
  const chosen={type:'native',value:selected,zone:'.'};
  const added=planDomainAddition(config,selected,chosen,parsed.domains,acceptsDomains);
  assert.equal(added.rules.length,3);
  assert.equal(added.upstreams.length,1);
  assert.equal(JSON.stringify(config),before,'P88 action must stage a new draft, not mutate input');
  assert.throws(()=>planDomainAddition(config,selected,chosen,['example.ru','example.ru'],acceptsDomains),/дубликат/);
  assert.throws(()=>planDomainAddition(config,'native://System/ru/1.1.1.1',{type:'native',value:'native://System/ru/1.1.1.1',zone:'ru'},['youtube.com'],acceptsDomains),/зон/);
  assert.throws(()=>nativeProbeTarget('native://System/ru/1.1.1.1',[{value:'native://System/ru/1.1.1.1',zone:'ru',type:'native'}]),/только|зоны/);
});

test('P88 no DNS activation or implicit network mutations in pure view/action modules',()=>{
  const action=read('DnsRuntimeP88ActionLogic.js');
  const catalog=read('DnsRuntimeP10Catalog.js');
  const actions=read('DnsRuntimeP88Actions.svelte');
  const views=read('DnsRuntimeP88Views.svelte');
  assert.doesNotMatch(action,/\bfetch\s*\(|\brequest\s*\(|\bndmc\b|service\/control|durable\/apply/);
  assert.doesNotMatch(catalog,/\bfetch\s*\(|\brequest\s*\(|\bndmc\b/);
  assert.doesNotMatch(views,/<script[^>]*src=|\.innerHTML\s*=/);
  assert.match(actions,/function advanced\(\)/);
  assert.match(actions,/async function scan\(\)/);
  assert.match(actions,/async function probe\(\)/);
  assert.doesNotMatch(actions,/(?<!['"])\bonMount\s*\(/);
});
