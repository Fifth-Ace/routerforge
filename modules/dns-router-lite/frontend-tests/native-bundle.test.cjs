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
    assert.equal(w.__rfDnsLiteSvelteLoaded,true);
    assert.equal(w.__rfDnsLiteCardsLoaded,true);
    assert.equal(w.__rfDnsLiteNativeEditorLoaded,true);
    assert.equal(d.getElementById('dnsLiteApp').dataset.dnsMode,'basic');
    assert.ok(calls.length>0);
    assert.ok(calls.every(x=>x.method==='GET'), 'unexpected mutation while opening DNS UI');
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    await until(()=>d.querySelector('[data-native-svelte-domain-editor="true"]'));
    assert.equal(d.getElementById('editorEmpty').hidden,true);
    const editor=d.getElementById('editorDomains');
    editor.value='example.com';editor.dispatchEvent(new w.Event('input',{bubbles:true}));
    d.getElementById('saveDomains').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён'));
    assert.ok(calls.every(x=>x.method==='GET'), 'draft save must not touch DNS service');
    assert.deepEqual(errors,[], 'uncaught browser errors');
  } finally {w.close();}
});
