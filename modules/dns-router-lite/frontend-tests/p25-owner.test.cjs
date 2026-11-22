'use strict';
const test=require('node:test');
const a=require('node:assert/strict');
const {fixture,until}=require('./fixture.cjs');

const zeroMutations=f=>a.equal(f.calls.filter(x=>x.opts.method&&x.opts.method!=='GET').length,0);

test('P25 owns one instance of service/cache/IP/security inputs; opening is GET-only',async()=>{
 const f=fixture(),d=f.w.document;
 try {
  await until(()=>d.getElementById('serviceListen')&&d.getElementById('p27RateLimit')&&d.getElementById('p241IPMode'));
  for(const id of ['serviceListen','cacheSize','allowedNetworks','p27RateLimit','p27RefuseAny','p27BogusIPs','p241IPEnabled','p241IPMode','p243Scope'])
    a.equal(d.querySelectorAll('#'+id).length,1,id);
  d.getElementById('p22ModeAdvanced').click();
  await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
  d.getElementById('p25SettingsNav').click();
  await until(()=>!d.getElementById('p25SettingsCard').hidden);
  d.querySelector('[data-settings-section="security"]').click();
  await until(()=>!d.getElementById('p25Settings_security').hidden);
  a.ok(d.getElementById('p25Settings_security').contains(d.getElementById('p27RateLimit')));
  zeroMutations(f);
 } finally {f.dom.window.close();}
});

test('P25 Svelte scope and note update without implicit DNS apply',async()=>{
 const f=fixture(),d=f.w.document;
 try {
  await until(()=>d.getElementById('p241IPMode'));
  d.getElementById('p22ModeAdvanced').click();
  await until(()=>d.getElementById('p25SettingsNav'));
  d.getElementById('p25SettingsNav').click();
  d.querySelector('[data-settings-section="ip"]').click();
  const scope=d.getElementById('p243Scope');
  scope.value='domains';scope.dispatchEvent(new f.w.Event('change',{bubbles:true}));
  await until(()=>!d.querySelector('#p241IPControls .p243-domains').hidden);
  a.equal(d.querySelector('#p241IPControls .p243-lists').hidden,true);
  a.match(d.getElementById('feedback').textContent,/несохранённые/);
  // Edit is allowed before selecting a default DNS, but explicit Save
  // must still refuse an incomplete config without a PUT to the backend.
  a.equal(d.getElementById('defaultDNS').value,'');
  zeroMutations(f);
  d.getElementById('saveButton').click();
  await until(()=>/Выбери DNS по умолчанию/.test(d.getElementById('feedback').textContent));
  zeroMutations(f);
 } finally {f.dom.window.close();}
});

test('P21/P22 de-duplicate a deferred discovery request; late response ignored on unmount',async()=>{
 const f=fixture({mode:'advanced',deferDevices:true}),d=f.w.document;
 try {
  await until(()=>f.calls.some(c=>c.path==='service/devices'));
  // Concurrent P21 mode/init/navigation and P22 view share a single owner.
  await new Promise(resolve=>setTimeout(resolve,60));
  a.equal(f.calls.filter(c=>c.path==='service/devices').length,1);
  f.w.close();f.releaseDevices();
  await new Promise(resolve=>setTimeout(resolve,35));
  a.deepEqual(f.errors,[]);
  zeroMutations(f);
 } finally {f.releaseDevices();f.w.close();}
});
