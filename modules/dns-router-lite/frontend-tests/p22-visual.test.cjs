'use strict';
const test=require('node:test');
const a=require('node:assert/strict');
const {fixture,until}=require('./fixture.cjs');

test('advanced mode uses native Svelte mode controls and direct read-only discovery',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    const before=f.calls.filter(c=>c.opts.method&&c.opts.method!=='GET').length;
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
    await until(()=>!!d.getElementById('p25SettingsNav'));
    a.equal(d.getElementById('p22ModeAdvanced').getAttribute('aria-pressed'),'true');
    a.equal(d.getElementById('p21Mode').value,'advanced');
    a.equal(f.calls.filter(c=>c.opts.method&&c.opts.method!=='GET').length,before);
  } finally {f.dom.window.close();}
});

test('nine IP Intelligence strategies are available without modifying live DNS',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>!!d.getElementById('p25SettingsNav'));
    d.getElementById('p25SettingsNav').click();
    await until(()=>!d.getElementById('p25SettingsCard').hidden);
    d.querySelector('[data-settings-section="ip"]').click();
    await until(()=>!d.getElementById('p25Settings_ip').hidden);
    const modes=[...d.getElementById('p241IPMode').options].map(x=>x.value);
    for(const mode of ['original','round_robin','random','fastest','health_aware','adaptive','sticky','weighted','healthy_only'])
      a.ok(modes.includes(mode),'missing algorithm: '+mode);
    a.ok(f.calls.every(c=>!c.opts.method||c.opts.method==='GET'),'read-only settings open');
  } finally {f.dom.window.close();}
});

test('Svelte-owned domain editor keeps draft local and does not start DNS',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.querySelector('[data-upstream-id="cf"] .dns-edit'));
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    await until(()=>!!d.querySelector('[data-native-svelte-domain-editor]'));
    const input=d.querySelector('[data-native-svelte-domain-editor] textarea');
    input.value='other.net\nnew.example';
    input.dispatchEvent(new f.w.Event('input',{bubbles:true}));
    d.querySelector('[data-native-svelte-domain-editor] .button.primary').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён'));
    a.equal(f.getActive(),false);
    a.ok(f.calls.every(c=>!c.opts.method||c.opts.method==='GET'));
  }finally{f.dom.window.close();}
});

test('Gate cannot be enabled during initial page mount',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    a.equal(d.getElementById('gateBadge').textContent,'Выключен');
    a.equal(d.getElementById('gateEnable').disabled,true);
    a.equal(f.calls.some(c=>c.path==='service/gate/control'),false);
  }finally{f.dom.window.close();}
});

test('device discovery and basic/advanced navigation do not implicitly write config',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
    d.getElementById('p22ModeBasic').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='basic');
    a.equal(f.calls.filter(c=>c.opts.method&&c.opts.method!=='GET').length,0);
  }finally{f.dom.window.close();}
});

// Full lifecycle regression: an outstanding GET must not touch detached DOM
// or generate unhandled promise rejections after the Svelte root unmounts.
test('advanced discovery in flight is cancelled on unmount without DOM writes',async()=>{
  const f=fixture({deferDevices:true}),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>f.calls.some(c=>c.path==='service/devices'));
    f.w.close(); // test fixture dispatches real pagehide before closing JSDOM
    f.releaseDevices(); // force a late response from a fetch which ignores abort
    await new Promise(resolve=>setTimeout(resolve,35));
    a.deepEqual(f.errors,[]);
  } finally {f.releaseDevices();f.w.close();}
});
