'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),test=require('node:test');
const root=path.join(__dirname,'..','modules','dns-router-runtime');
const logic=fs.readFileSync(path.join(root,'frontend','DnsRuntimeNativeLogic.js'),'utf8');
const native=fs.readFileSync(path.join(root,'frontend','DnsRuntimeNative.svelte'),'utf8');
const api=fs.readFileSync(path.join(root,'api.go'),'utf8');
const policies=fs.readFileSync(path.join(root,'native_policies_p87.go'),'utf8');
const probe=fs.readFileSync(path.join(root,'native_probe_p87.go'),'utf8');

test('P8.7 PolicyN discovery is explicit read-only',()=>{
 assert.match(native,/onclick=\{scan\}/);
 assert.match(logic,/get\('keenetic\/native-policies'\)/);
 assert.match(api,/\/v1\/keenetic\/native-policies/);
 assert.match(api,/ReadOnlyNativePoliciesP87\(r\.Context\(\)\)/);
 assert.match(policies,/"show ip policy"/);
 assert.match(policies,/safety\.CommandContext/);
 assert.doesNotMatch(policies,/exec\.CommandContext/);
 assert.doesNotMatch(policies,/system configuration save|no ip policy|ip name-server/);
 assert.match(policies,/DNSProbePerformed\s+bool/);
});

test('P8.7 shows real policy names, qualified routes and no false availability',()=>{
 for(const id of ['p87NativeProbe','p87PolicySummary','p87ProbeResult'])assert.match(native,new RegExp('id="'+id+'"'));
 assert.match(logic,/found\.description\.trim/);
 assert.match(logic,/default_absent/);
 assert.match(logic,/default_present/);
 assert.match(logic,/маршрут неизвестен/);
 assert.match(native,/\{routes\}/);
 assert.doesNotMatch(native,/\.innerHTML\s*=/);
});

test('P8.7 probe only on explicit click, exactly one GET, root zone enforced',()=>{
 assert.match(native,/onclick=\{probe\}/);
 assert.match(logic,/get\('keenetic\/native-dns\/probe\?'/);
 assert.match(logic,/parsed\.zone!=='\.'/);
 assert.match(api,/\/v1\/keenetic\/native-dns\/probe/);
 assert.match(probe,/ResolveNativeDestinationP83/);
 assert.match(probe,/example\.com/);
 assert.match(probe,/DialContext\(ctx,\s*"udp4",\s*addr\)/);
 assert.doesNotMatch(probe,/exec\.Command|ndmc|opkg/);
});
