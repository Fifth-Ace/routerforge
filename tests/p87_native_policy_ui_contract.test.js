'use strict';
const fs=require('node:fs');
const path=require('node:path');
const assert=require('node:assert/strict');
const test=require('node:test');
const root=path.join(__dirname,'..','modules','dns-router-runtime');
const js=fs.readFileSync(path.join(root,'frontend','app.js'),'utf8');
const html=fs.readFileSync(path.join(root,'frontend','index.html'),'utf8');
const api=fs.readFileSync(path.join(root,'api.go'),'utf8');
const policies=fs.readFileSync(path.join(root,'native_policies_p87.go'),'utf8');
const probe=fs.readFileSync(path.join(root,'native_probe_p87.go'),'utf8');

test('P8.7 policy discovery is explicitly requested and read only',()=>{
  assert.match(js,/request\('keenetic\/native-policies'\)/);
  assert.match(api,/\/v1\/keenetic\/native-policies/);
  assert.match(api,/ReadOnlyNativePoliciesP87\(r\.Context\(\)\)/);
  assert.match(policies,/"show ip policy"/);
  assert.match(policies,/exec\.CommandContext/);
  assert.doesNotMatch(policies,/system configuration save|no ip policy|ip name-server/);
  assert.match(policies,/DNSProbePerformed\s+bool/);
});

test('P8.7 displays real names with safety-qualified routing state',()=>{
  for(const id of ['p87NativeProbe','p87PolicySummary','p87ProbeResult']) assert.ok(html.includes('id="'+id+'"'),id);
  assert.match(js,/description\+' \('\+name\+'\)'/);
  assert.match(js,/default_absent/);
  assert.match(js,/default_present/);
  assert.match(js,/маршрут неизвестен/);
  assert.match(js,/labelFor\(item\.name\)/);
  assert.match(js,/p87PolicySummary'\)\.textContent/);
  assert.doesNotMatch(js,/\.innerHTML\s*=/);
});

test('P8.7 DNS probe only via explicit click and fixed zone',()=>{
  assert.match(js,/el\('p87NativeProbe'\)\.addEventListener\('click'/);
  assert.match(js,/request\('keenetic\/native-dns\/probe\?'/);
  assert.match(js,/parts\[1\]!=='.'/);
  assert.match(api,/\/v1\/keenetic\/native-dns\/probe/);
  assert.match(probe,/ResolveNativeDestinationP83/);
  assert.match(probe,/example\.com/);
  assert.match(probe,/DialContext\(ctx,\s*"udp4",\s*addr\)/);
  assert.doesNotMatch(probe,/exec\.Command|ndmc|opkg/);
});
