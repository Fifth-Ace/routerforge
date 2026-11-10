'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const ui=path.resolve(__dirname,'../modules/dns-router-runtime/frontend');
const load=name=>import(pathToFileURL(path.join(ui,name)).href);
const read=name=>fs.readFileSync(path.join(ui,name),'utf8');
function sample(){return {upstreams:[
  {id:'native1',transport:'native',address:'native://System/./1.1.1.1'},
  {id:'native2',transport:'native',address:'native://Policy1/./9.9.9.9'},
  {id:'doh',transport:'doh',server_name:'dns.example.com',address:'10.1.1.1',path:'/dns-query'},
  {id:'udp',transport:'udp',address:'8.8.8.8'}
],rules:[
  {id:'r1',domain:'youtube.com',upstream:'doh',match:'suffix',priority:100},
  {id:'r2',domain:'googlevideo.com',upstream:'doh',match:'suffix',priority:100},
  {id:'r3',domain:'example.ru',upstream:'native1',match:'suffix',priority:100},
  {id:'r4',domain:'example.net',upstream:'native2',match:'suffix',priority:100},
  {id:'r5',domain:'bad.invalid',upstream:'missing',match:'suffix',priority:100}
]};}
const entries=[{selector:'native://System/./1.1.1.1',label:'Cloudflare DNS · System',zone:'.',address:'127.0.0.1:40500'}];

test('P10 grouped DNS view and list actions are native Svelte owners with no hidden activation',()=>{
  const app=read('DnsRuntimeApp.svelte');
  const actions=read('DnsRuntimeP88Actions.svelte');
  const views=read('DnsRuntimeP88Views.svelte');
  const logic=read('DnsRuntimeP10Logic.js');
  const css=read('module.css');
  assert.match(app,/id="drP10EditingLabel"/);
  assert.match(app,/<DnsRuntimeP88Views scope="rules"/);
  assert.match(views,/id="drSimpleRuleList"/);
  assert.match(views,/\{#each items as group/);
  assert.match(actions,/id="drP10CancelListEdit" onclick=\{cancel\}/);
  assert.match(actions,/onclick=\{add\}/);
  assert.match(logic,/System-зон Keenetic/);
  assert.match(css,/"dns rules" "dns domains"/);
  assert.doesNotMatch(views,/\.innerHTML\s*=|\bfetch\s*\(|\bndmc\b/);
});

test('P10 catalog preserves per-DNS domain ownership, other policies and broken references',async()=>{
  const c=await load('DnsRuntimeP10Catalog.js');
  const cfg=sample(),before=JSON.stringify(cfg);
  const groups=c.dnsGroups(cfg,'System',entries);
  const doh=groups.find(x=>x.id==='doh');
  assert.equal(doh.rules.length,2);
  assert.equal(doh.rules[0].domain,'youtube.com');
  assert.match(doh.info.title,/dns\.example\.com/);
  assert.match(doh.info.detail,/https:\/\/dns\.example\.com\/dns-query/);
  assert.equal(groups.find(x=>x.id==='native1').rules.length,1);
  assert.equal(groups.some(x=>x.id==='native2'),false);
  assert.equal(groups.find(x=>x.id==='missing').info.scope,'broken');
  assert.match(c.dnsSummary(cfg,entries),/doh.*2 доменов/);
  const other=c.dnsGroups(cfg,'Policy1',entries);
  assert.ok(other.some(x=>x.id==='native2'));
  assert.ok(!other.some(x=>x.id==='native1'));
  assert.equal(JSON.stringify(cfg),before,'read-only catalog must not mutate config');
});

test('P10 CAS group replacement refuses collisions, unsupported rules and stale selection',async()=>{
  const {planGroupReplacement}=await load('DnsRuntimeP10GroupLogic.js');
  const {acceptsDomains}=await load('DnsRuntimeNativeCatalog.js');
  const cfg=sample(),before=JSON.stringify(cfg);
  const chosen={type:'manual',upstreamId:'doh',value:'manual:doh',zone:'.'};
  const plan=planGroupReplacement(cfg,'doh','manual:doh',chosen,['youtube.com','new.example.com'],acceptsDomains,'rev-1');
  assert.equal(plan.revision,'rev-1');
  assert.equal(plan.baseRules,JSON.stringify(cfg.rules));
  assert.equal(plan.added,1);
  assert.equal(plan.removed,1);
  assert.equal(plan.rules.some(x=>x.domain==='googlevideo.com'&&x.upstream==='doh'),false);
  assert.equal(plan.rules.some(x=>x.domain==='example.net'&&x.upstream==='native2'),true);
  assert.equal(JSON.stringify(cfg),before,'planner cannot mutate base config');
  assert.throws(()=>planGroupReplacement(cfg,'doh','manual:other',chosen,['youtube.com'],acceptsDomains,'rev-1'),/измен/);
  assert.throws(()=>planGroupReplacement(cfg,'doh','manual:doh',chosen,['example.ru'],acceptsDomains,'rev-1'),/другому DNS/);
  assert.throws(()=>planGroupReplacement(cfg,'doh','manual:doh',chosen,['youtube.com','youtube.com'],acceptsDomains,'rev-1'),/уникальные/);
  const extended=sample();extended.rules[0].match='regex';
  assert.throws(()=>planGroupReplacement(extended,'doh','manual:doh',chosen,['youtube.com'],acceptsDomains,'rev-1'),/Расширенные/);
  assert.equal(JSON.stringify(cfg),before);
});
