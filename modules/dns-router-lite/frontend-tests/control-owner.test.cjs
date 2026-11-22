'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const {fixture,until,tick}=require('./fixture.cjs');

function mutation(call) {return !['GET','HEAD'].includes(call.opts?.method||'GET');}

test('DNS/Gate/Editor/Log controls have one Svelte owner and boot is GET-only',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount')?.textContent.includes('2 DNS'));
    for(const id of ['reloadButton','navDns','navLogs','defaultDNS','gatePreflight','gateEnable','gateDisable','dnsAddPanel','inspectDNS','addDnsToggle','editorDNSChoice','logFilter','logPause','copyLogs']) {
      assert.equal(d.querySelectorAll('#'+id).length,1,'single Svelte control '+id);
    }
    assert.deepEqual(f.calls.filter(mutation),[],'initial mount cannot mutate DNS or Gate');
  } finally {f.w.close();}
});

test('native log toolbar filters and pauses without POST or PUT',async()=>{
  const f=fixture({active:true}),d=f.w.document;
  try {
    await until(()=>d.getElementById('logPause')&&d.getElementById('navLogs'));
    d.getElementById('navLogs').click();
    d.getElementById('logFilter').value='example';
    d.getElementById('logFilter').dispatchEvent(new f.w.Event('input',{bubbles:true}));
    d.getElementById('logPause').click();
    await until(()=>d.getElementById('liveBadge').textContent.includes('ПАУЗА'));
    assert.deepEqual(f.calls.filter(mutation),[]);
  } finally {f.w.close();}
});

test('DNS draft and default selection do not send network mutations',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('defaultDNS')?.querySelectorAll('option').length>=3);
    d.getElementById('defaultDNS').value='cf';
    d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change',{bubbles:true}));
    d.getElementById('addDnsToggle').click();
    await until(()=>d.getElementById('dnsAddPanel').hidden===false);
    d.getElementById('newAddress').value='https://dns.example.com/dns-query';
    d.getElementById('newAddress').dispatchEvent(new f.w.Event('input',{bubbles:true}));
    assert.equal(d.getElementById('commitDNS').disabled,true);
    assert.deepEqual(f.calls.filter(mutation),[]);
  } finally {f.w.close();}
});

test('Gate remains OFF until explicit user action, preflight retains declared action',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount')?.textContent.includes('2 DNS'));
    assert.equal(f.calls.filter(x=>x.path==='service/gate/control').length,0);
    assert.equal(d.getElementById('gatePreflight').disabled,true,'Gate preflight is unavailable while DNS service is stopped');
    // Explicit user action starts DNS first. The test must never bypass
    // the real Gate readiness contract by clicking a disabled button.
    d.getElementById('defaultDNS').value='old';
    d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change',{bubbles:true}));
    d.getElementById('applyButton').click();
    await until(()=>f.getActive() && !d.getElementById('gatePreflight').disabled);
    assert.equal(f.calls.filter(x=>x.path==='service/gate/control').length,0);
    d.getElementById('gatePreflight').click();
    await until(()=>f.calls.some(x=>x.path==='service/gate/control'));
    const calls=f.calls.filter(x=>x.path==='service/gate/control');
    assert.equal(calls.length,1);
    assert.equal(calls[0].opts.headers['X-RouterForge-Action'],'service-control');
    assert.equal(JSON.parse(calls[0].opts.body).operation,'preflight');
    assert.equal(f.calls.filter(x=>x.path==='service/gate/control'&&JSON.parse(x.opts.body).operation==='enable').length,0);
  } finally {f.w.close();}
});
