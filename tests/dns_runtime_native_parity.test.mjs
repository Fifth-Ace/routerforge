// Runtime Svelte 5: executable parity of the former P8.8-P9.2 helper contracts.
// This suite imports the real production pure functions, not legacy app.js snippets.
// Browser component/mutation parity and Go/IPK are independent mandatory gates.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath, pathToFileURL} from 'node:url';
import path from 'node:path';
const here=path.dirname(fileURLToPath(import.meta.url));
const ui=path.resolve(here,'../modules/dns-router-runtime/frontend');
const load=name=>import(pathToFileURL(path.join(ui,name)).href);
const {entries,acceptsDomains}=await load('DnsRuntimeNativeCatalog.js');
const {dnsGroups,dnsSummary,dnsChoice}=await load('DnsRuntimeP10Catalog.js');
const {planGroupReplacement}=await load('DnsRuntimeP10GroupLogic.js');
const {planDomainAddition,nativeProbeTarget,probeMessage}=await load('DnsRuntimeP88ActionLogic.js');
const {policyCards,inventoryCards,groupedCards}=await load('DnsRuntimeP88ViewModel.js');
const {parseDomainBatch,previewDomainBatch,mergeDomainFile,exportDomainText}=await load('DnsRuntimeP92Logic.js');
const {parseDnsFields,createP91Flow}=await load('DnsRuntimeP91Logic.js');
const copy=x=>structuredClone(x);
function inventory(){return {available:true,parsed_instances:2,instances:[
 {name:'System',valid:true,endpoints:[
  {address:'127.0.0.1:40500',domain:'.',upstream_hint:'9.9.9.9'},
  {address:'127.0.0.1:40503',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'},
  {address:'127.0.0.1:40504',domain:'su'},
  {address:'127.0.0.1:40505',domain:'xn--p1ai',upstream_hint:'dns.example'}]},
 {name:'Policy1',valid:true,endpoints:[{address:'127.0.0.1:40534',domain:'.',upstream_hint:'1.1.1.1'}]}
]};}
function cfg(){return {upstreams:[
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

test('P89 native inventory enumerates distinct root/zone endpoints without invented TLS evidence',()=>{
 const result=entries(inventory(),'System');
 assert.equal(result.length,4);
 assert.deepEqual(result.map(x=>x.zone),['.','ru','su','xn--p1ai']);
 assert.equal(result[0].selector,'native://System/./9.9.9.9');
 assert.equal(result[1].selector,'native://System/ru/common.dot.dns.yandex.net');
 assert.equal(result[2].usable,false);
 assert.match(result[2].reason,/идентификатора/);
 assert.ok(result.every(x=>x.encryptionProven===false));
 assert.equal(entries(inventory(),'Policy1').length,1);
});
test('P89 domain gate enforces native endpoint zone, including punycode',()=>{
 const [root,ru,,xn]=entries(inventory(),'System');
 assert.equal(acceptsDomains(root,['youtube.com','googlevideo.com']).ok,true);
 assert.equal(acceptsDomains(ru,['example.ru','a.example.ru']).ok,true);
 assert.equal(acceptsDomains(ru,['youtube.com']).ok,false);
 assert.equal(acceptsDomains(xn,['example.xn--p1ai']).ok,true);
 assert.equal(acceptsDomains(xn,['example.ru']).ok,false);
 assert.equal(acceptsDomains(null,['example.ru']).ok,false);
});
test('P89 inventory fails closed on incomplete, malformed, duplicate and untrusted policy',()=>{
 const absent=inventory();absent.available=false;
 assert.ok(entries(absent,'System').every(x=>!x.usable));
 const dup=inventory();dup.instances[0].endpoints.push({address:'127.0.0.1:40509',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'});
 assert.ok(entries(dup,'System').filter(x=>x.zone==='ru').every(x=>!x.usable));
 const invalid=inventory();invalid.instances[0].valid=false;
 assert.ok(entries(invalid,'System').every(x=>!x.usable));
 assert.deepEqual(entries(inventory(),'Policy../../../etc'),[]);
 assert.deepEqual(entries(inventory(),'Policy9'),[]);
});
test('P10 grouped DNS view preserves ownership, other policies and broken references',()=>{
 const a=dnsGroups(cfg(),'System'),b=dnsGroups(cfg(),'Policy1');
 assert.equal(a.find(x=>x.id==='doh').rules.length,2);
 assert.match(a.find(x=>x.id==='doh').info.detail,/https:\/\/dns.example.com\/dns-query/);
 assert.equal(a.find(x=>x.id==='native1').rules.length,1);
 assert.ok(!a.some(x=>x.id==='native2'));
 assert.equal(a.find(x=>x.id==='missing').info.scope,'broken');
 assert.ok(b.some(x=>x.id==='native2'));
 assert.ok(!b.some(x=>x.id==='native1'));
 assert.match(dnsSummary(cfg()),/doh.*2 доменов/i);
});
test('P10 group replacement never mutates config before confirmation',()=>{
 const original=cfg(),before=copy(original),picked={type:'manual',upstreamId:'doh',value:'manual:doh',zone:'.'};
 const planned=planGroupReplacement(original,'doh','manual:doh',picked,['youtube.com','new.example.com'],acceptsDomains,'r0');
 assert.deepEqual(original,before);
 assert.equal(planned.revision,'r0');
 assert.equal(planned.removed,1);assert.equal(planned.added,1);
 assert.ok(planned.rules.some(r=>r.upstream==='doh'&&r.domain==='new.example.com'));
 assert.ok(!planned.rules.some(r=>r.upstream==='doh'&&r.domain==='googlevideo.com'));
 assert.ok(planned.rules.some(r=>r.id==='r4'&&r.domain==='example.net'));
});
test('P10 replacement rejects changed selection, cross-DNS ownership and advanced rules',()=>{
 const original=cfg(),picked={type:'manual',upstreamId:'doh',value:'manual:doh',zone:'.'};
 assert.throws(()=>planGroupReplacement(original,'doh','manual:udp',picked,['youtube.com'],acceptsDomains,'r0'),/изменился/);
 assert.throws(()=>planGroupReplacement(original,'doh','manual:doh',picked,['example.ru'],acceptsDomains,'r0'),/принадлежит другому DNS/);
 const advanced=cfg();advanced.rules[0].match='exact';
 assert.throws(()=>planGroupReplacement(advanced,'doh','manual:doh',picked,['youtube.com'],acceptsDomains,'r0'),/Расширенные правила/);
 assert.deepEqual(original,cfg());
});
test('P10 replacement checks DNS zone and rejects duplicate domains',()=>{
 const original=cfg(),picked={type:'native',value:'native://System/./1.1.1.1',zone:'ru'};
 assert.throws(()=>planGroupReplacement(original,'native1','native://System/./1.1.1.1',picked,['example.com'],acceptsDomains,'r0'),/вне этой зоны/);
 assert.throws(()=>planGroupReplacement(original,'native1','native://System/./1.1.1.1',picked,['example.ru','example.ru'],acceptsDomains,'r0'),/уникальные/);
});
test('P88 draft action stages domains locally and deduplicates',()=>{
 const config=cfg(),before=copy(config);
 const choice={type:'manual',value:'manual:udp',upstreamId:'udp',zone:'.'};
 const planned=planDomainAddition(config,'manual:udp',choice,['new.example.com'],acceptsDomains);
 assert.deepEqual(config,before);
 assert.equal(planned.count,1);
 assert.equal(planned.rules.at(-1).upstream,'udp');
 assert.throws(()=>planDomainAddition(config,'manual:udp',choice,['youtube.com'],acceptsDomains),/уже есть/);
 assert.throws(()=>planDomainAddition(config,'manual:udp',choice,['a.example.com','a.example.com'],acceptsDomains),/дубликаты/);
});
test('P88 native probe permits only root zone and validates selector',()=>{
 const root='native://System/./9.9.9.9',list=[{value:root,zone:'.',type:'native'}];
 assert.match(nativeProbeTarget(root,list),/instance=System/);
 assert.throws(()=>nativeProbeTarget('native://System/ru/9.9.9.9',list),/только для DNS зоны/);
 assert.throws(()=>nativeProbeTarget('native://System/./x',list),/только для DNS зоны/);
 assert.match(probeMessage({dns_success:true,latency_ms:23}),/23 мс/);
 assert.match(probeMessage({dns_success:false,rcode:'SERVFAIL'}),/SERVFAIL/);
});
test('P88 read-only cards preserve unavailable-route warnings',()=>{
 const instances=[{name:'System',valid:true},{name:'Policy1',valid:true},{name:'Policy2',valid:false}];
 const catalog={available:true,policies:[{name:'Policy1',description:'WAN 2',ipv4_route_status:'default_absent'}]};
 const cards=policyCards(instances,catalog,'System');
 assert.equal(cards.length,2);
 assert.match(cards[1].route,/Нет маршрута/);
 assert.equal(inventoryCards([{label:'DNS X',zone:'ru',usable:false,reason:'bad'}])[0].details,'bad');
});
test('P88 grouped UI refuses editing unavailable and extended rules',()=>{
 const groups=dnsGroups(cfg(),'System');
 const cards=groupedCards(groups,['manual:doh'],'doh','System');
 assert.ok(cards.find(x=>x.id==='doh').canEdit);
 assert.equal(cards.find(x=>x.id==='native1').canEdit,false);
 assert.equal(cards.find(x=>x.id==='missing').canEdit,false);
 const advanced=groups.map(g=>({...g,rules:g.id==='doh'?[{id:'a',domain:'example.com',match:'exact',upstream:'doh',priority:1}]:g.rules}));
 assert.equal(groupedCards(advanced,['manual:doh']).find(x=>x.id==='doh').canEdit,false);
});
test('P92 import parser normalizes and deduplicates, preserves original input',()=>{
 const raw='Example.COM, example.com\nfoo.example.net # note\n';
 assert.deepEqual(parseDomainBatch(raw),{domains:['example.com','foo.example.net']});
 assert.equal(raw.startsWith('Example'),true);
 const preview=previewDomainBatch(raw);
 assert.equal(preview.ok,true);assert.equal(preview.domains.length,2);
});
test('P92 parser rejects URLs, IPs, filters and out-of-bounds batches',()=>{
 for(const bad of ['https://example.com','192.168.1.1','||example.com^','example.com/path','*.example.com']){
  assert.ok(parseDomainBatch(bad).error,bad);
 }
 assert.ok(parseDomainBatch('a.example\n'.repeat(501)).error);
 assert.ok(parseDomainBatch(Array.from({length:201},(_,i)=>`d${i}.example`).join('\n')).error);
});
test('P92 file merge and export are pure and size limited',()=>{
 assert.equal(mergeDomainFile('example.com','example.net\nexample.com',25),'example.com\nexample.net');
 assert.equal(exportDomainText('example.com\nexample.com'),'example.com\n');
 assert.throws(()=>mergeDomainFile('','x.example',256*1024+1),/256 КБ/);
 assert.throws(()=>exportDomainText('https://example.com'),/Некорректный/);
});
test('P91 manual DNS parser validates DoH/DoT and does not claim UDP encryption',()=>{
 assert.deepEqual(parseDnsFields({transport:'doh',endpoint:'https://dns.example.com/dns-query',ip:'1.1.1.1'}).path,'/dns-query');
 assert.equal(parseDnsFields({transport:'dot',endpoint:'dns.example.com',ip:'1.1.1.1'}).address,'1.1.1.1:853');
 assert.equal(parseDnsFields({transport:'udp',endpoint:'dns.example.com',ip:'8.8.8.8'}).transport,'udp');
 for(const bad of ['http://example.com/dns-query','https://user:pass@example.com/dns-query','https://example.com/dns-query?x=1']){
  assert.throws(()=>parseDnsFields({transport:'doh',endpoint:bad,ip:'1.1.1.1'}));
 }
 assert.throws(()=>parseDnsFields({transport:'doh',endpoint:'dns.example.com',ip:'not-ip'}),/IP/);
});
test('P91 lookup does not stage an unverified TLS upstream',async()=>{
 let fields={transport:'dot',endpoint:'dns.example.com',ip:'1.1.1.1'};
 let commits=0;
 const instance=createP91Flow({
  lookup:async()=>({address:'1.1.1.1:853',selected_ip:'1.1.1.1',ips:['1.1.1.1'],certificate:{verified:false,pem:''}}),
  readDraft:()=>({revision:'r0',generation:0,config:{upstreams:[]}}),
  commit:()=>{commits++},notify:()=>{},refresh:()=>{},getFields:()=>fields,signal:()=>{}
 });
 await assert.rejects(instance.inspect(),/TLS-сертификат не подтверждён/);
 assert.throws(()=>instance.store(),/Сначала получи IP и сертификат/);
 assert.equal(commits,0);
});
test('P91 verified DoH lookup stages a draft without changing original config',async()=>{
 const source={upstreams:[],rules:[]},before=copy(source);
 let fields={transport:'doh',endpoint:'https://dns.example.com/dns-query',ip:'1.1.1.1'};
 const commits=[];
 const instance=createP91Flow({
  lookup:async()=>({address:'1.1.1.1:443',selected_ip:'1.1.1.1',ips:['1.1.1.1'],certificate:{verified:true,pem:'PEM'}}),
  readDraft:()=>({revision:'r0',generation:0,config:source}),
  commit:(action,next)=>{commits.push([action,next])},notify:()=>{},refresh:()=>{},getFields:()=>fields,signal:()=>{}
 });
 await instance.inspect();const server=instance.store();
 assert.deepEqual(source,before);
 assert.equal(commits.length,1);assert.equal(commits[0][0],'edited');
 assert.equal(commits[0][1].previousRevision,'r0');
 assert.equal(commits[0][1].config.upstreams.length,1);
 assert.equal(server.certificate_pem,'PEM');
 assert.throws(()=>instance.store(),/Сначала получи IP и сертификат/);
});
test('P91 changed input during asynchronous lookup invalidates certificate',async()=>{
 let resume;const fields={transport:'dot',endpoint:'dns.example.com',ip:'1.1.1.1'};
 const instance=createP91Flow({
  lookup:()=>new Promise(r=>resume=r),readDraft:()=>({revision:'r0',generation:0,config:{upstreams:[]}}),
  commit:()=>{throw Error('SHOULD_NOT_COMMIT')},notify:()=>{},refresh:()=>{},getFields:()=>fields,signal:()=>{}
 });
 const task=instance.inspect();fields.endpoint='changed.example.com';
 resume({address:'1.1.1.1:853',selected_ip:'1.1.1.1',ips:['1.1.1.1'],certificate:{verified:true,pem:'PEM'}});
 await assert.rejects(task,/Адрес изменился/);
 assert.throws(()=>instance.store(),/Сначала получи IP и сертификат/);
});
