'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const root=path.resolve(__dirname,'../modules/dns-router-runtime');
const frontend=path.join(root,'frontend');
const read=name=>fs.readFileSync(path.join(frontend,name),'utf8');
const logic=()=>import(pathToFileURL(path.join(frontend,'DnsRuntimeNativeLogic.js')).href);

test('P85 native picker, inventory and safe draft are owned by Svelte, not historical HTML',()=>{
  const forms=read('DnsRuntimePolicyForms.svelte');
  const picker=read('DnsRuntimeNative.svelte');
  const readonly=read('DnsRuntimeReadOnly.svelte');
  const html=read('index.html');
  assert.match(forms,/import DnsRuntimeNative/);
  assert.match(forms,/<DnsRuntimeNative bind:value=\{poolAddress\}/);
  assert.match(forms,/createDraftEditor/);
  assert.match(forms,/poolSNI='';poolPath='';poolFallbacks=''/);
  assert.doesNotMatch(forms,/upstreamForm|submitLegacy|syncLegacyTransport/);
  assert.doesNotMatch(html,/id="upstreamForm"|id="p85NativePicker"/);
  for(const id of ['p85NativePicker','p85NativeEndpoint','p85NativeScan','p85NativeMessage'])
    assert.equal((picker.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  assert.match(readonly,/id="p85NativeStatus"/);
  assert.match(picker,/onclick=\{scan\}/);
  assert.doesNotMatch(picker,/onMount\s*\(|\.innerHTML\s*=/);
});

test('P85 native discovery makes zero requests at construction, then exactly three GET requests on explicit scan',async()=>{
  const {createNativeReader,parseNativeSelector}=await logic();
  let calls=[];
  const inv={available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[
    {address:'127.0.0.1:40500',domain:'.',upstream_hint:'1.1.1.1'}]}]};
  const reader=createNativeReader({get:async p=>{
    calls.push(p);
    if(p==='keenetic/native-dns')return inv;
    if(p==='keenetic/native-bridge/status')return {available:false};
    if(p==='keenetic/native-policies')return {available:true,policies:[]};
    return {dns_success:true};
  }});
  assert.deepEqual(calls,[],'constructing native reader may not contact Keenetic');
  const result=await reader.scan();
  assert.deepEqual(calls,['keenetic/native-dns','keenetic/native-bridge/status','keenetic/native-policies']);
  assert.equal(result.choices.length,1);
  assert.equal(result.choices[0].selector,'native://System/./1.1.1.1');
  assert.equal(parseNativeSelector('native://System/./1.1.1.1').zone,'.');
  assert.equal(parseNativeSelector('native://Bad/./1.1.1.1'),null);
  await assert.rejects(reader.probe('native://System/ru/1.1.1.1'),/корневой зоны/);
  assert.equal(calls.length,3,'invalid zone must never issue a DNS probe');
  await reader.probe('native://System/./1.1.1.1');
  assert.equal(calls.length,4);
  assert.match(calls[3],/^keenetic\/native-dns\/probe\?instance=System&hint=1\.1\.1\.1$/);
});

test('P85 native contract has no implicit mutation and distinguishes read-only API',()=>{
  const source=read('DnsRuntimeNativeLogic.js');
  const picker=read('DnsRuntimeNative.svelte');
  const api=fs.readFileSync(path.join(root,'api.go'),'utf8');
  assert.match(source,/get\('keenetic\/native-dns'\)/);
  assert.match(source,/get\('keenetic\/native-bridge\/status'\)/);
  assert.match(source,/get\('keenetic\/native-policies'\)/);
  assert.match(api,/"\/v1\/keenetic\/native-bridge\/status"/);
  assert.match(api,/http\.MethodGet/);
  assert.match(picker,/method:'GET'/);
  assert.doesNotMatch(source,/\bPOST\b|\bPUT\b|\bDELETE\b|\bndmc\b|ip name-server/);
  assert.doesNotMatch(picker,/service\/control|keenetic\/durable|\.innerHTML\s*=/);
});
