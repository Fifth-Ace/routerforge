'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'modules/dns-router-runtime/frontend/index.html'),'utf8');
const js=fs.readFileSync(path.join(root,'modules/dns-router-runtime/frontend/app.js'),'utf8');
const api=fs.readFileSync(path.join(root,'modules/dns-router-runtime/api.go'),'utf8');

test('P85 has inventory-backed, non-mutating native picker and read-only status',()=>{
 for(const id of ['p85NativePicker','p85NativeEndpoint','p85NativeScan','p85NativeMessage','p85NativeStatus']){
  assert.match(html,new RegExp(`id="${id}"`));
  assert.match(js,new RegExp(`'${id}'`));
 }
 assert.match(js,/request\('keenetic\/native-dns'\)/);
 assert.match(js,/request\('keenetic\/native-bridge\/status'\)/);
 assert.match(api,/"\/v1\/keenetic\/native-bridge\/status"/);
 assert.match(api,/http\.MethodGet/);
 assert.match(js,/status\.textContent/);
 assert.match(html,/Policy0/);
 assert.doesNotMatch(js,/innerHTML\s*=/);
});

test('P85 keeps native strict: no TLS/HTTP fields or fallback',()=>{
 assert.match(js,/v\.transport==='native'/);
 assert.match(js,/f\.transport==='native'/);
 assert.match(js,/node\.readOnly=native/);
 assert.match(js,/native:\/\//);
 assert.match(html,/encrypted-only/);
});
