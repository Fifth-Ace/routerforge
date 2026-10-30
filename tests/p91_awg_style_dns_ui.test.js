'use strict';
const t=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),p=require('node:path'),vm=require('node:vm');
const root=p.resolve(__dirname,'..','modules/dns-router-runtime');
const js=fs.readFileSync(p.join(root,'frontend/app.js'),'utf8');
const html=fs.readFileSync(p.join(root,'frontend/index.html'),'utf8');
const api=fs.readFileSync(p.join(root,'api.go'),'utf8');
const go=fs.readFileSync(p.join(root,'dns_lookup_p91.go'),'utf8');
t('P91 one-address flow, optional advanced IP & SNI, simple buttons',()=>{
 assert.match(html,/<summary>＋ Добавить DNS-сервер<\/summary>/);
 assert.match(html,/id="drP90Endpoint"/);
 assert.match(html,/id="drP90Inspect"[^>]*>Получить IP и сертификаты/);
 assert.match(html,/id="drP90Store"[^>]*>Добавить DNS/);
 assert.match(html,/id="drP91Advanced"/);
 assert.ok(html.indexOf('id="drP90IP"')>html.indexOf('id="drP91Advanced"'));
 for(const name of ['udp','dot','doh','doq'])assert.match(html,new RegExp('value="'+name+'"'));
 assert.doesNotMatch(html,/Публичный IPv4 для подключения/);
});
t('P91 backend resolves IP on button, verifies DoT/DoH, preserves explicit trust defaults',()=>{
 assert.match(api,/\/v1\/certificates\/lookup/);
 assert.match(api,/requireMutation\(w, r, "certificate-lookup"\)/);
 assert.match(go,/LookupDNSServerP91/);
 assert.match(go,/lookupIPsP91/);
 assert.match(go,/inspectTLSP91/);
 assert.doesNotMatch(go,/InsecureSkipVerify/);
 assert.match(js,/request\('certificates\/lookup',jsonOptions\('certificate-lookup'/);
 assert.match(js,/p88Id\('drP90IP'\)\.value=result\.selected_ip/);
 assert.match(js,/p91Fields\(false\)/);
 assert.match(js,/server\.certificate_pem=p90Inspected\.pem/);
 assert.match(js,/p91Reset\(\)/);
});
t('P91 backend never exposes a fake DoQ certificate',()=>{
 assert.match(go,/req\.Transport == "udp" \|\| req\.Transport == "doq"/);
 assert.match(js,/encrypted=f\.transport==='doh'\|\|f\.transport==='dot'/);
});
t('P91 manual field parser accepts DoH URL without requiring IPv4 first',()=>{
 const start=js.indexOf('  function p91Fields('),end=js.indexOf('  function p91CurrentKey()',start);
 assert.ok(start>=0&&end>start);
 const elements={drP90Transport:{value:'doh'},drP90Endpoint:{value:'https://dns.dns-ai.ru/dns-query'},drP90IP:{value:''},drP91SNI:{value:''}};
 const f=vm.runInNewContext('(()=>{let p91Lookup=null;const p88Id=id=>elements[id];'+js.slice(start,end)+';return p91Fields;})()',{elements,URL});
 const a=f(false);assert.equal(a.server_name,'dns.dns-ai.ru');assert.equal(a.path,'/dns-query');assert.equal(a.address,'');
 assert.throws(()=>f(true),/Сначала/);
 elements.drP90Transport.value='udp';elements.drP90Endpoint.value='1.1.1.1';
 assert.equal(f(false).server_name,'1.1.1.1');
});
