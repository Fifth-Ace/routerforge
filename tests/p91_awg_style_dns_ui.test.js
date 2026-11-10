'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const root = path.resolve(__dirname, '..', 'modules/dns-router-runtime');
const frontend = path.join(root, 'frontend');
const read = name => fs.readFileSync(path.join(frontend, name), 'utf8');
const load = name => import(pathToFileURL(path.join(frontend, name)).href);

test('P91 Svelte controls: one endpoint, optional IP/SNI, no automatic activation', () => {
  const ui = read('DnsRuntimeP91.svelte');
  assert.match(ui, /<summary>＋ Добавить DNS-сервер<\/summary>/);
  for(const id of ['drP90Transport','drP90Endpoint','drP90Inspect','drP90Store','drP90IP','drP91SNI','drP91Advanced','drP91Found'])
    assert.equal((ui.match(new RegExp('id="'+id+'"', 'g'))||[]).length, 1, id);
  assert.ok(ui.indexOf('id="drP90IP"') > ui.indexOf('id="drP91Advanced"'));
  for(const transport of ['udp','dot','doh','doq'])assert.match(ui,new RegExp('value="'+transport+'"'));
  assert.match(ui, /disabled=\{busy\|\|!verified\}/);
  assert.doesNotMatch(ui, /Публичный IPv4 для подключения|fetch\s*\(|ndmc|shadowApply/);
});

test('P91 real parser accepts DoH URL without IP; requires verified IP before draft staging', async () => {
  const {parseDnsFields} = await load('DnsRuntimeP91Logic.js');
  const fields={transport:'doh',endpoint:'https://dns.dns-ai.ru/dns-query',ip:'',sni:''};
  const parsed=parseDnsFields(fields);
  assert.equal(parsed.server_name,'dns.dns-ai.ru');
  assert.equal(parsed.path,'/dns-query');
  assert.equal(parsed.address,'');
  assert.throws(()=>parseDnsFields(fields,true),/Сначала/);
  assert.equal(parseDnsFields({transport:'udp',endpoint:'1.1.1.1'}).server_name,'1.1.1.1');
  assert.throws(()=>parseDnsFields({...fields,endpoint:'https://dns.dns-ai.ru/../private'}),/Недопуст|Некоррект/);
});

test('P91 explicit lookup of non-TLS UDP keeps certificate unproven and only stages a draft', async () => {
  const {createP91Flow} = await load('DnsRuntimeP91Logic.js');
  const current={transport:'udp',endpoint:'1.1.1.1',ip:'',sni:''};
  let lookups=0, commits=0, operation='', payload;
  const flow=createP91Flow({
    lookup:async () => {lookups++;return {selected_ip:'1.1.1.1',ips:['1.1.1.1'],address:'1.1.1.1:53',certificate:{verified:false,pem:'untrusted'}};},
    readDraft:()=>({revision:'rev1',generation:0,config:{schema_version:1,upstreams:[],rules:[]}}),
    commit:(name,data)=>{commits++;operation=name;payload=data;},
    notify:()=>{},refresh:()=>{},getFields:()=>({...current}),
    signal:(data)=>{if(data.verified?.ip)current.ip=data.verified.ip;}
  });
  assert.equal(lookups,0);assert.equal(commits,0);
  await flow.inspect();
  const added=flow.store();
  assert.equal(lookups,1);assert.equal(commits,1);assert.equal(operation,'edited');
  assert.equal(added.transport,'udp');assert.equal(added.address,'1.1.1.1:53');
  assert.equal(added.certificate_pem,undefined);
  assert.equal(payload.previousRevision,'rev1');assert.equal(payload.generation,0);
});

test('P91 unverified DoH TLS certificate cannot be stored or treated as verified', async () => {
  const {createP91Flow} = await load('DnsRuntimeP91Logic.js');
  let commits=0;
  const fields={transport:'doh',endpoint:'https://dns.example.com/dns-query',ip:'',sni:''};
  const flow=createP91Flow({
    lookup:async()=>({selected_ip:'1.1.1.1',ips:['1.1.1.1'],address:'1.1.1.1:443',certificate:{verified:false,pem:'untrusted'}}),
    readDraft:()=>({revision:'r',generation:0,config:{schema_version:1,upstreams:[],rules:[]}}),
    commit:()=>{commits++;},notify:()=>{},refresh:()=>{},getFields:()=>fields,signal:()=>{}
  });
  await assert.rejects(flow.inspect(),/TLS-сертификат не подтверждён/);
  assert.throws(()=>flow.store(),/Сначала получи IP/);
  assert.equal(commits,0);
  const go=fs.readFileSync(path.join(root,'dns_lookup_p91.go'),'utf8');
  assert.match(go,/req\.Transport == "udp" \|\| req\.Transport == "doq"/);
  assert.doesNotMatch(go,/InsecureSkipVerify/);
});
