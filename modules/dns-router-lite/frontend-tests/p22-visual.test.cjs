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

// P37 browser parity: prior tests clicked hidden DOM buttons; this checks
// the actual CSS activation marker and both navigation directions.
test('P37: shipped Svelte mode switch activates, hides legacy control, and navigates without DNS writes',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    const root=d.getElementById('dnsLiteApp');
    a.equal(root.classList.contains('p22-ready'),true,'functional Svelte switch must be visually enabled');
    a.equal(d.querySelectorAll('#p22ModeAdvanced').length,1);
    a.equal(d.querySelectorAll('#p22ModeBasic').length,1);
    const css=require('node:fs').readFileSync(require('node:path').join(__dirname,'../frontend/p22-visual.css'),'utf8');
    a.match(css,/#dnsLiteApp\.p22-ready \.p21-mode-label \{ display:none!important; \}/);
    a.match(css,/#dnsLiteApp:not\(\.p22-ready\) \.p22-mode-switch \{ display:none!important; \}/);
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>root.dataset.dnsMode==='advanced' && !d.getElementById('p21Page').hidden);
    a.equal(d.getElementById('pageDns').hidden,true);
    a.equal(d.getElementById('p22ModeAdvanced').getAttribute('aria-pressed'),'true');
    d.getElementById('p22ModeBasic').click();
    await until(()=>root.dataset.dnsMode==='basic' && !d.getElementById('pageDns').hidden);
    a.equal(d.getElementById('p21Page').hidden,true);
    a.ok(f.calls.every(c=>!c.opts.method || c.opts.method==='GET'),'mode navigation cannot mutate live DNS');
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P37: saved Advanced mode reopens Devices, not invisible Basic page',async()=>{
  const f=fixture({mode:'advanced'}),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
    a.equal(d.getElementById('dnsLiteApp').classList.contains('p22-ready'),true);
    a.equal(d.getElementById('p21Page').hidden,false);
    a.equal(d.getElementById('pageDns').hidden,true);
    a.equal(d.getElementById('p22ModeAdvanced').getAttribute('aria-pressed'),'true');
    a.ok(f.calls.every(c=>!c.opts.method || c.opts.method==='GET'));
  }finally{f.dom.window.close();}
});

test('P37: DNS Lite typography matches Core semantic text hierarchy',()=>{
  const css=require('node:fs').readFileSync(require('node:path').join(__dirname,'../frontend/p25-settings.css'),'utf8');
  a.match(css,/P37: DNS Lite typography parity with RouterForge Core/);
  for(const size of ['font-size:13px','font-size:20px','font-size:15px','font-size:11px'])
    a.ok(css.includes(size),'missing Core typography token: '+size);
});


test('P38: shared mode chrome retains navbar, title and button dimensions',async()=>{
  const fs=require('node:fs');
  const path=require('node:path');
  const css=fs.readFileSync(path.join(__dirname,'../frontend/p25-settings.css'),'utf8');
  const marker='/* P38: shared DNS Lite chrome and sizing across Basic and Advanced.';
  a.ok(css.includes(marker),'P38 style gate missing');
  const p38=css.slice(css.indexOf(marker));
  for(const expected of ['min-height:46px;','min-height:35px;','font-size:11px;','font-size:13px;'])
    a.ok(p38.includes(expected),'P38 semantic size missing: '+expected);
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    // Old mode-specific geometry deliberately precedes the P38 parity rules.
    const style=d.createElement('style');
    style.textContent='#dnsLiteApp .tabs{min-height:46px;padding:4px;gap:6px}' +
      '#dnsLiteApp[data-dns-mode="advanced"] .tabs{min-height:49px;padding:5px;gap:8px}' +
      '#dnsLiteApp .tabs .tab{min-height:35px;padding:7px 17px}' +
      '#dnsLiteApp[data-dns-mode="advanced"] .tabs .tab{min-height:37px;padding:8px 15px}' + p38;
    d.head.appendChild(style);
    const nav=d.querySelector('#dnsLiteApp .tabs');
    const tab=d.querySelector('#dnsLiteApp .tabs .tab');
    a.ok(nav && tab,'native tab must be present');
    const dimensions=()=>{
      const navCSS=f.w.getComputedStyle(nav);
      const tabCSS=f.w.getComputedStyle(tab);
      return [navCSS.minHeight,navCSS.paddingTop,navCSS.gap,tabCSS.minHeight,tabCSS.paddingTop,tabCSS.paddingLeft,tabCSS.fontSize];
    };
    const basic=dimensions();
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='advanced');
    const advanced=dimensions();
    a.deepEqual(advanced,basic,'switching mode must not resize the shared navigation');
    a.equal(d.getElementById('p21Page').hidden,false,'advanced content stays accessible');
    d.getElementById('p22ModeBasic').click();
    await until(()=>d.getElementById('dnsLiteApp').dataset.dnsMode==='basic');
    a.deepEqual(dimensions(),basic,'reverse navigation retains the same scale');
    a.ok(f.calls.every(c=>!c.opts.method||c.opts.method==='GET'),'mode switch is read-only');
    a.deepEqual(f.errors,[]);
  } finally {f.dom.window.close();}
});


// P39: the original Basic sizing is the reference; Advanced must grow to match.
// CSS is injected after the old shrunken P37/P38 rules to exercise cascade order.
test('P39: both DNS modes use readable 27/14/17/13 typography; switching is read-only',async()=>{
  const fs=require('node:fs'),path=require('node:path');
  const css=fs.readFileSync(path.join(__dirname,'../frontend/p25-settings.css'),'utf8');
  const marker='/* P39: readable DNS UI';
  a.ok(css.includes(marker),'P39 readable sizing layer missing');
  const p39=css.slice(css.indexOf(marker));
  for(const required of ['font-size:27px;','font-size:14px;','font-size:17px;','font-size:13px;','min-height:52px;'])
    a.ok(p39.includes(required),'missing readable dimension: '+required);
  a.doesNotMatch(p39,/\bzoom\s*:|transform\s*:\s*scale\s*\(/i,'do not simulate readability with zoom');
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    const root=d.getElementById('dnsLiteApp');
    const style=d.createElement('style');
    style.textContent='#dnsLiteApp.rfds-scope{font-size:13px}' +
      '#dnsLiteApp .module-heading-copy h1{font-size:20px}' +
      '#dnsLiteApp[data-dns-mode="advanced"] .module-heading-copy h1{font-size:20px}' +
      '#dnsLiteApp .tabs .tab{font-size:11px}' +
      '#dnsLiteApp[data-dns-mode="advanced"] .tabs .tab{font-size:11px}' + p39;
    d.head.appendChild(style);
    const h1=d.querySelector('#dnsLiteApp .module-heading-copy h1');
    const tab=d.querySelector('#dnsLiteApp .tabs .tab');
    const switchButton=d.getElementById('p22ModeAdvanced');
    const baseMetrics=()=>[root,h1,tab,switchButton].map(n=>f.w.getComputedStyle(n).fontSize);
    a.deepEqual(baseMetrics(),['14px','27px','13px','13px'],'Basic must be readable');
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>root.dataset.dnsMode==='advanced');
    a.deepEqual(baseMetrics(),['14px','27px','13px','13px'],'Advanced must match Basic, not shrink it');
    const deviceTitle=d.querySelector('#dnsLiteApp #p21Page .panel h2');
    if(deviceTitle)a.equal(f.w.getComputedStyle(deviceTitle).fontSize,'17px');
    d.getElementById('p22ModeBasic').click();
    await until(()=>root.dataset.dnsMode==='basic');
    a.deepEqual(baseMetrics(),['14px','27px','13px','13px']);
    a.ok(f.calls.every(c=>!c.opts.method||c.opts.method==='GET'),'navigation must not mutate DNS');
    a.deepEqual(f.errors,[]);
  } finally {f.dom.window.close();}
});


// P40 header regression: no min-content title compression and no mixed loose
// row of controls. All existing legacy IDs and real Svelte actions are kept.
test('P40: full-width heading, three grouped toolbar sections, and safe mode switches',async()=>{
  const fs=require('node:fs');
  const path=require('node:path');
  const html=fs.readFileSync(path.join(__dirname,'../frontend/index.html'),'utf8');
  const css=fs.readFileSync(path.join(__dirname,'../frontend/p25-settings.css'),'utf8');
  a.match(html,/class="top-actions dns-toolbar"/);
  a.match(html,/class="dns-toolbar-end"/);
  a.match(html,/class="dns-toolbar-links"/);
  a.match(css,/\/\* P40: responsive, grouped DNS header\./);
  const p40=css.slice(css.indexOf('/* P40: responsive, grouped DNS header.'));
  a.match(p40,/#dnsLiteApp \.module-heading,\s*#dnsLiteApp\[data-dns-mode="advanced"\] \.module-heading\s*\{\s*display:grid;/);
  a.match(p40,/grid-template-columns:minmax\(0,1fr\);/);
  a.match(p40,/@media\(max-width:1120px\)/);
  a.match(p40,/@media\(max-width:720px\)/);
  a.doesNotMatch(p40,/\bzoom\s*:|transform\s*:\s*scale\s*\(/i);
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    const root=d.getElementById('dnsLiteApp');
    await until(()=>root.classList.contains('p22-ready') && d.querySelectorAll('.dns-toolbar-mode').length===1 && d.querySelectorAll('.dns-toolbar-runtime').length===1);
    a.equal(d.querySelectorAll('.dns-toolbar-mode').length,1);
    a.equal(d.querySelectorAll('.dns-toolbar-runtime').length,1);
    a.equal(d.querySelectorAll('.dns-toolbar-end').length,1);
    a.equal(d.getElementById('rf-dns-lite-svelte-mode').closest('.dns-toolbar')!==null,true);
    for(const id of ['p22ModeBasic','p22ModeAdvanced','p21Mode','p21ActiveBadge','serviceBadge','nativeDNSLink','rf-lite-reload-slot']) {
      a.ok(d.getElementById(id),'existing DNS contract missing: '+id);
    }
    d.getElementById('p22ModeAdvanced').click();
    await until(()=>root.dataset.dnsMode==='advanced');
    a.equal(d.getElementById('p21Page').hidden,false);
    d.getElementById('p22ModeBasic').click();
    await until(()=>root.dataset.dnsMode==='basic');
    a.equal(d.getElementById('pageDns').hidden,false);
    a.equal(f.calls.some(c=>c.opts.method && c.opts.method!=='GET'),false,'visual toggle must not modify DNS');
    a.deepEqual(f.errors,[]);
  } finally {f.dom.window.close();}
});
