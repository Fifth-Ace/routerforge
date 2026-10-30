'use strict';
const t=require('node:test'),a=require('node:assert/strict'),fs=require('node:fs'),p=require('node:path'),vm=require('node:vm');
const root=p.resolve(__dirname,'..','modules/dns-router-runtime');
const js=fs.readFileSync(p.join(root,'frontend/app.js'),'utf8');
const html=fs.readFileSync(p.join(root,'frontend/index.html'),'utf8');
const api=fs.readFileSync(p.join(root,'api.go'),'utf8');
const native=fs.readFileSync(p.join(root,'native_bridge_p83.go'),'utf8');
const cert=fs.readFileSync(p.join(root,'certificates_p90.go'),'utf8');
const policy=fs.readFileSync(p.join(root,'policy.go'),'utf8');
const inventory=fs.readFileSync(p.join(root,'native_inventory_p82.go'),'utf8');
const body=js.slice(js.indexOf('  // P8.9 NATIVE CATALOG BEGIN'),js.indexOf('  // P8.9 NATIVE CATALOG END'));
const catalogue=vm.runInNewContext('(()=>{const window={};'+body+';return window.RFNativeCatalogP89;})()');
t('P9.0 DoH URL with @dnsm has a backend-provided opaque selector and readable label',()=>{
 const input={available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[{address:'127.0.0.1:40506',domain:'.',upstream_hint:'https://dns.dns-ai.ru/dns-query@dnsm',transport_hint:'doh',selector_hint:'sha256-'+'a'.repeat(64)}]}]};
 const [c]=catalogue.entries(input,'System');
 a.equal(c.usable,true);a.equal(c.transport,'doh');a.match(c.label,/https:\/\/dns\.dns-ai\.ru\/dns-query ·/);a.doesNotMatch(c.label,/@dnsm/);
 a.equal(c.selector,'native://System/./sha256-'+'a'.repeat(64));a.equal(c.encryptionProven,false);
});
t('P9.0 preserves legacy plaintext hints, fail-closed zone and incomplete inventory',()=>{
 const inv={available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[{address:'127.0.0.1:40500',domain:'.',upstream_hint:'1.1.1.1'}]}]};
 const [x]=catalogue.entries(inv,'System');a.equal(x.selector,'native://System/./1.1.1.1');a.equal(x.usable,true);
 a.equal(catalogue.acceptsDomains({usable:true,zone:'ru'},['youtube.com']).ok,false);
 inv.available=false;a.equal(catalogue.entries(inv,'System')[0].usable,false);
});
t('P9.0 explicit verified certificate inspection, public PEM snapshot, no auto trust',()=>{
 for(const id of ['drP90Transport','drP90Endpoint','drP90IP','drP90Inspect','drP90Store','drP90Status'])a.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1);
 a.match(js,/request\('certificates\/inspect',jsonOptions\('certificate-inspect'/);
 a.match(js,/certificate_pem:p90Inspected\.pem/);
 a.match(js,/p90InspectedKey/);
 a.match(js,/p90AllChoices/);
 a.match(api,/\/v1\/certificates\/inspect/);
 a.match(api,/requireMutation\(w, r, "certificate-inspect"\)/);
 a.match(policy,/CertificatePEM string/);
 a.match(policy,/validateSavedCertificateP90/);
 a.match(cert,/tls\.Client\(conn,/);
 a.doesNotMatch(cert,/InsecureSkipVerify/);
 a.doesNotMatch(cert,/exec\.Command|ndmc|system configuration save/);
 a.doesNotMatch(js,/\.innerHTML\s*=/);
});
t('P9.0 native selector is fresh inventory-bound, never the raw DoH URL as a socket',()=>{
 a.match(inventory,/SelectorHint = nativeSelectorHintP90\(hint\)/);
 a.match(inventory,/TransportHint = "doh"/);
 a.match(native,/nativeHintMatchesP90\(selector\.Hint, ep\.UpstreamHint\)/);
 a.match(js,/selector_hint/);
 a.match(js,/entry\.transport==='doh'/);
});
