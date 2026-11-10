'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {JSDOM,VirtualConsole}=require('jsdom');
const frontend=path.resolve(__dirname,'../frontend');
const html=fs.readFileSync(path.join(frontend,'index.html'),'utf8');
const bundlePath=path.join(frontend,'build/svelte-mode.js');
const until=async(condition)=>{
  for(let i=0;i<100;i++) {if(condition())return;await new Promise(r=>setTimeout(r,30));}
  throw Error('NATIVE_SVELTE_UI_LOAD_TIMEOUT');
};
test('Svelte bundled DNS Lite: read-only boot, cards, draft list, mode, no unrequested DNS mutation', async()=>{
  assert.ok(fs.statSync(bundlePath).size>3000, 'built native bundle missing');
  const script=fs.readFileSync(bundlePath,'utf8');
  const errors=[],calls=[];
  const vc=new VirtualConsole();
  vc.on('jsdomError', e=>errors.push(e.message));
  const dom=new JSDOM(html,{url:'https://routerforge.test/api/modules/dns-router/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true,virtualConsole:vc});
  const w=dom.window, d=w.document;
  w.confirm=()=>true;
  w.scrollTo=()=>{};
  w.fetch=async(url,opts={})=>{
    const endpoint=String(url).replace(/^\.\.\//,'').split('?')[0];
    calls.push({endpoint,method:opts.method||'GET'});
    let response={};
    if(endpoint==='config')response={config:{schema_version:1,upstreams:[{id:'cf',transport:'doh',address:'1.1.1.1:443',server_name:'cloudflare-dns.com',path:'/dns-query'}],rules:[],lists:[],service:{listen:'192.168.10.1:55358',cache_bytes:1048576,allowed_networks:[]}},revision:'sha256:fixture'};
    else if(endpoint==='service/status')response={mode:'independent-dns',active:false,applied_revision:'',system_gate:{state:'STOCK',active:false,desired:false},connection:{state:'disconnected'},requests:0};
    else if(endpoint==='service/upstreams/health')response={upstreams:[]};
    else if(endpoint==='service/traces')response={traces:[]};
    else if(endpoint==='service/devices')response={devices:[],issues:[]};
    else if(endpoint==='service/gate/status')response={state:'STOCK',active:false,desired:false};
    return {ok:true,status:200,headers:{get:()=> 'application/json'},json:async()=>response};
  };
  try{
    w.eval(script);
    await until(()=>d.getElementById('serverCount').textContent==='1 DNS');
    await until(()=>d.querySelector('[data-upstream-id="cf"] .dns-edit'));
    assert.equal(Object.keys(w).some(k=>k.startsWith('__rfDnsLite')),false,
      'Svelte UI must not expose or consume permanent window bridges');
    assert.equal(d.querySelectorAll('[data-upstream-id="cf"]').length,1,
      'one Svelte owner per DNS card');
    // P12: the native Svelte Settings hub must exist on the compiled bundle,
    // preserve advanced/basic placement, and never trigger DNS mutation.
    await until(()=>d.getElementById('p25SettingsNav') && d.getElementById('p25SettingsCard'));
    const settings=d.getElementById('p25SettingsCard');
    assert.ok(settings.hidden,'Settings is hidden on basic boot');
    // P25 moves only Svelte-owned controls, not the former legacy
    // #additionalSettings container or foreign DOM handlers.
    const serviceFields=d.getElementById('serviceListen').closest('.p25-native-service-fields');
    const cacheFields=d.getElementById('cacheSize').closest('.p25-native-cache-field');
    const originalParent=serviceFields.parentElement;
    const originalCacheParent=cacheFields.parentElement;
    assert.equal(originalParent.id,'rf-dns-lite-basic-service-slot');
    assert.equal(originalCacheParent.id,'rf-dns-lite-basic-cache-slot');
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
    await until(()=>serviceFields.parentElement.id==='p25Settings_service');
    await until(()=>cacheFields.parentElement.id==='p25Settings_cache');
    assert.equal(d.querySelectorAll('#serviceListen').length,1);
    assert.equal(d.querySelectorAll('#cacheSize').length,1);
    d.getElementById('p25SettingsNav').click();
    await until(()=>settings.hidden===false);
    assert.equal(d.querySelectorAll('.p25-settings-link').length,8);
    d.querySelector('[data-settings-section="privacy"]').click();
    await until(()=>d.getElementById('p25Settings_privacy').hidden===false);
    d.getElementById('cacheSize').value='2';
    d.getElementById('p30ECSMode').value='strip';
    d.getElementById('p30ECSMode').dispatchEvent(new w.Event('change',{bubbles:true}));
    d.querySelector('[data-settings-section="ip"]').click();
    await until(()=>d.getElementById('p25Settings_ip').hidden===false);
    d.getElementById('p241IPEnabled').checked=true;
    d.getElementById('p241IPMode').value='round_robin';
    d.getElementById('p241IPMode').dispatchEvent(new w.Event('change',{bubbles:true}));
    await until(()=>/кэша|cached/i.test(d.getElementById('p241IPModeNote').textContent));
    d.getElementById('p243Scope').value='lists';
    d.getElementById('p243Scope').dispatchEvent(new w.Event('change',{bubbles:true}));
    await until(()=>d.querySelector('#p241IPControls .p243-lists').hidden===false);
    assert.equal(d.querySelector('#p241IPControls .p243-domains').hidden,true);
    assert.ok(calls.every(x=>x.method==='GET'),'Settings navigation and draft fields must be non-mutating');
    d.getElementById('p22ModeBasic').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='basic');
    await until(()=>serviceFields.parentElement===originalParent);
    await until(()=>cacheFields.parentElement===originalCacheParent);
    assert.ok(settings.hidden,'Settings hidden after basic switch');
    assert.equal(d.getElementById('cacheSize').value,'2','cache draft survives mode switch');
    assert.equal(d.getElementById('dnsLiteApp').dataset.dnsMode,'basic');
    assert.ok(calls.length>0);
    assert.ok(calls.every(x=>x.method==='GET'), 'unexpected mutation while opening DNS UI');
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    await until(()=>d.querySelector('[data-native-svelte-domain-editor="true"]'));
    assert.equal(d.getElementById('editorEmpty').hidden,true);
    const editor=d.querySelector('[data-native-svelte-domain-editor="true"] [data-rf-editor="domains"]');
    assert.ok(editor,'native editor input is mounted, not the hidden legacy mirror');
    editor.value='example.com';editor.dispatchEvent(new w.Event('input',{bubbles:true}));
    d.querySelector('[data-native-svelte-domain-editor="true"] [data-rf-editor="save-list"]').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён'));
    assert.ok(calls.every(x=>x.method==='GET'), 'draft save must not touch DNS service');
    assert.deepEqual(errors,[], 'uncaught browser errors');
  } finally {w.close();}
});
