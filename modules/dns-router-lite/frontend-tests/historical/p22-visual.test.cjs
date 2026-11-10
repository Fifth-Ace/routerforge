'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'../frontend');
const html=fs.readFileSync(path.join(root,'index.html'),'utf8');
const js=actualController('DnsP22Visual.svelte');
const css=fs.readFileSync(path.join(root,'p22-visual.css'),'utf8');
const original=actualController('DnsLiteController.svelte');


function actualController(name){
  const source=fs.readFileSync(path.join(root,name),'utf8');
  const start='/* RF_ACTUAL_CONTROLLER_START */';
  const end='/* RF_ACTUAL_CONTROLLER_END */';
  if(source.split(start).length!==2||source.split(end).length!==2) throw Error('SOURCE_COMPONENT_CONTROLLER_MARKERS_INVALID: '+name);
  const body=source.split(start)[1].split(end)[0];
  const m=source.match(/const state = \$state\((\{[^\n]+\})\);/);
  return m ? body.replace('(() => {', '(() => {\n  const state = '+m[1]+';') : body;
}
test('P22 screenshot layout: segmented mode, independent visible tabs, and route cards',()=>{
  for(const id of ['p22ModeBasic','p22ModeAdvanced','p22NavGroups','p22NavRules',
    'p22DevicesCard','p22GroupsCard','p22RulesCard','p22RouteCard','p22RouteSummary',
    'p22DevicePick','p22MemberSelect','p22RulePick','p22ProbePick','p22MemberChips']){
    assert.equal((html.match(new RegExp(`id="${id}"`,'g'))||[]).length,1,id);
  }
  assert.match(html,/p22-visual\.css/);assert.match(html,/svelte-mode\.js/);
  assert.match(html,/Что такое правила домена\?/);
  assert.match(css,/p22-mode-switch/);
  assert.match(css,/p22-rule-fields/);
  assert.match(css,/@media\(max-width:560px\)/);
});
test('P22 old editor, Gate and tested P21 service controls remain in the DOM',()=>{
  for(const id of ['p21Mode','p21Nav','navDns','navLogs','p21Page','p21Devices','p21Groups','p21Rules',
    'p21MAC','p21Name','p21Target','p21Inherit','p21SetDevice','p21GroupID','p21Members',
    'p21GroupTarget','p21SetGroup','p21RuleMAC','p21Domain','p21Match','p21RuleTarget',
    'p21Priority','p21SetRule','p21ProbeMAC','p21ProbeDomain','p21ProbeApplied',
    'p21Probe','p21Explanation','gateEnable','gateDisable','editorDomains','p21Save','p21Apply']){
    assert.equal((html.match(new RegExp(`id="${id}"`,'g'))||[]).length,1,id);
  }
  assert.match(original,/function p21Render\(/);
  assert.match(original,/function p21SetMode\(/);
});
test('P22 discovery is GET only; UI does not touch Gate or Keenetic and uses textContent',()=>{
  assert.match(js,/fetch\('\.\.\/service\/devices'/);
  assert.match(js,/method:'GET'/);
  assert.doesNotMatch(js,/innerHTML\s*=/);
  assert.doesNotMatch(js,/service\/gate\/control|service\/connect|dns-proxy filter|ndmc|iptables|opkg/);
  assert.doesNotMatch(js,/method\s*:\s*['"](?:POST|PUT|DELETE|PATCH)/);
  assert.match(js,/p21Mode/);
  assert.match(js,/dispatchEvent\(new Event\('change'/);
  assert.match(js,/CONFIGURATION_ONLY/);
  assert.match(js,/not a verified network E2E/);
});
test('P22 no change to old P21 backend data model or revision workflow',()=>{
  assert.match(original,/service\/devices\/evaluate/);
  assert.match(original,/X-RouterForge-Action/);
  assert.match(original,/If-Match/);
  assert.match(js,/p21Members/);
  assert.match(js,/p21RuleMAC/);
  assert.match(js,/p21ProbeMAC/);
  assert.match(css,/data-dns-mode="advanced"/);
});

test('P22 FIX2: one visible mode control and advanced landing, not old DNS tab',()=>{
  assert.match(css, /p22-ready \.p21-mode-label \{ display:none!important/);
  assert.match(js, /if \(nativeSelect\.value === 'advanced'\) tabs\.devices\.click\(\)/);
  assert.match(js, /if \(app\.dataset\.dnsMode === 'advanced'\) tabs\.devices\.click\(\)/);
  assert.match(js, /app\.classList\.add\('p22-ready'\)/);
});

test('P22 FIX2 runtime: enhanced script activates Advanced screen and populates device selection',async()=>{
  const {JSDOM}=require('jsdom');
  const dom=new JSDOM(html,{url:'https://routerforge.test/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true});
  try{
    const w=dom.window,d=w.document,app=d.getElementById('dnsLiteApp');
    app.dataset.dnsMode='basic';
    const mode=d.getElementById('p21Mode');
    mode.addEventListener('change',()=>{
      app.dataset.dnsMode=mode.value;
      if(mode.value==='basic')d.getElementById('p21Page').hidden=true;
    });
    d.getElementById('p21Nav').addEventListener('click',()=>{
      d.getElementById('p21Page').hidden=false;
      d.getElementById('pageDns').hidden=true;
    });
    const apiCalls=[];
    w.fetch=async (url,opts={})=>{
      apiCalls.push({url:String(url),method:opts.method});
      return {ok:true,json:async()=>({devices:[{mac:'02:11:22:33:44:55',name:'Laptop',ip:'192.168.10.18',trusted:true}],issues:[]})};
    };
    w.eval(js);
    assert.equal(app.classList.contains('p22-ready'),true,'visual JS must actually boot');
    assert.equal(d.getElementById('p21Nav').textContent,'Устройства');
    d.getElementById('p22ModeAdvanced').click();
    assert.equal(app.dataset.dnsMode,'advanced');
    assert.equal(d.getElementById('p21Page').hidden,false,'advanced selection opens the workspace');
    assert.equal(d.getElementById('p22ModeAdvanced').getAttribute('aria-pressed'),'true');
    await new Promise(resolve=>setTimeout(resolve,30));
    const pick=d.getElementById('p22DevicePick');
    assert.equal(pick.options.length,2,'real discovered devices appear in the picker');
    pick.value='02:11:22:33:44:55';
    pick.dispatchEvent(new w.Event('change'));
    assert.equal(d.getElementById('p21MAC').value,'02:11:22:33:44:55');
    assert.equal(d.getElementById('p21Name').value,'Laptop');
    assert.ok(apiCalls.length>0);
    assert.ok(apiCalls.every(x=>x.url==='../service/devices'&&x.method==='GET'),'enhancer is read-only');
  }finally{dom.window.close();}
});


test('P23: Advanced tabs isolate their own editor while save/apply stay globally available',async()=>{
  const {JSDOM}=require('jsdom');
  const dom=new JSDOM(html,{url:'https://routerforge.test/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true});
  try {
    const w=dom.window,d=w.document,$=id=>d.getElementById(id);
    const app=$('dnsLiteApp');app.dataset.dnsMode='basic';
    let changes=0;
    $('p21Mode').addEventListener('change',()=>{
      changes++;
      app.dataset.dnsMode=$('p21Mode').value;
      if(app.dataset.dnsMode==='basic'){$('p21Page').hidden=true;$('pageDns').hidden=false;$('pageLogs').hidden=true;}
    });
    $('p21Nav').addEventListener('click',()=>{
      $('p21Page').hidden=false;$('pageDns').hidden=true;$('pageLogs').hidden=true;
    });
    $('navDns').addEventListener('click',()=>{
      $('p21Page').hidden=true;$('pageDns').hidden=false;$('pageLogs').hidden=true;
    });
    $('navLogs').addEventListener('click',()=>{
      $('p21Page').hidden=true;$('pageDns').hidden=true;$('pageLogs').hidden=false;
    });
    const calls=[];
    w.fetch=async (url,options={})=>{
      calls.push({url:String(url),method:options.method});
      return {ok:true,json:async()=>({devices:[],issues:[]})};
    };
    w.eval(js);
    $('p22ModeAdvanced').click();
    await new Promise(resolve=>setTimeout(resolve,25));
    assert.equal(changes,1,'mode changed only through the existing P21 control');
    assert.equal($('p21Page').hidden,false);
    const devices=$('p22DevicesCard'),groups=$('p22GroupsCard'),rules=$('p22RulesCard'),route=$('p22RouteCard');
    assert.equal(devices.hidden,false);
    assert.equal(groups.hidden,true);
    assert.equal(rules.hidden,true);
    assert.equal(route.parentElement,rules,'route check belongs only to Domain rules');
    const toolbar=$('p23DraftToolbar');
    assert.equal(toolbar.parentElement,$('p21Page'),'actions must live outside tab panels');
    for(const action of ['p21Refresh','p21Save','p21Apply']) {
      assert.equal(toolbar.contains($(action)),true,action+' must stay available on every tab');
    }
    assert.equal(toolbar.contains($('p21Cache')),false);
    assert.equal($('p21Cache').parentElement,$('p21Page'));
    $('p21MAC').value='02:11:22:33:44:55';
    $('p22NavGroups').click();
    assert.equal(devices.hidden,true);
    assert.equal(groups.hidden,false);
    assert.equal(rules.hidden,true);
    assert.equal($('p22NavGroups').getAttribute('aria-current'),'page');
    assert.equal($('p21Nav').hasAttribute('aria-current'),false);
    $('p21GroupName').value='TV';
    $('p22NavRules').click();
    assert.equal(devices.hidden,true);
    assert.equal(groups.hidden,true);
    assert.equal(rules.hidden,false);
    assert.equal(route.hidden,false);
    assert.equal($('p22NavRules').getAttribute('aria-current'),'page');
    $('p21Domain').value='youtube.com';
    $('p21Nav').click();
    assert.equal(devices.hidden,false);
    assert.equal(groups.hidden,true);
    assert.equal(rules.hidden,true);
    assert.equal($('p21MAC').value,'02:11:22:33:44:55','device draft is preserved');
    assert.equal($('p21GroupName').value,'TV','group draft is preserved');
    assert.equal($('p21Domain').value,'youtube.com','rule draft is preserved');
    $('navLogs').click();
    assert.equal($('p21Page').hidden,true);
    assert.equal($('pageLogs').hidden,false);
    $('p22NavGroups').click();
    assert.equal($('p21Page').hidden,false);
    assert.equal(groups.hidden,false);
    assert.equal(devices.hidden,true);
    $('p22ModeBasic').click();
    assert.equal($('p21Page').hidden,true);
    assert.equal($('pageDns').hidden,false);
    assert.equal($('p22ModeBasic').getAttribute('aria-pressed'),'true');
    $('p22ModeAdvanced').click();
    assert.equal(devices.hidden,false,'Advanced always opens Devices by default');
    assert.equal(groups.hidden,true);
    assert.equal(rules.hidden,true);
    // The view fires mocked async GET discovery on mode changes. Let the
    // fetch/json Promise chain and MutationObserver microtasks settle before
    // disposing jsdom, otherwise a callback accesses a destroyed document.
    await new Promise(resolve=>setImmediate(resolve));
    assert.match($('p22DeviceDiscovery').textContent,
      /Нет доступных устройств|No devices discovered/,
      'mocked discovery must finish before the fixture is disposed');
    assert.ok(calls.every(c=>c.method==='GET' && c.url==='../service/devices'),
      'tab changes and mode switch may only perform read-only discovery');
  } finally {
    // Also drain pending UI microtasks when an assertion fails mid-scenario.
    await new Promise(resolve=>setImmediate(resolve));
    dom.window.close();
  }
});

test('P23: every advanced tab remains independent of backend policies and Core assets',()=>{
  assert.match(js,/cards\[key\]\.hidden\s*=\s*!selected/);
  assert.match(js,/className = 'p22-draft-toolbar'/);
  assert.doesNotMatch(js,/scrollIntoView\(/);
  assert.match(css,/#p21Page > \.panel\[hidden\]/);
  assert.match(css,/Core design tokens stay authoritative/);
  assert.match(original,/service\/devices\/evaluate/);
});

// P24.0: static UI contract (no asynchronous jsdom teardown).
test('P24.0: compact UI keeps existing tab wiring and hides empty route details',()=>{
  assert.ok(js.includes('label.textContent = t('),'active-tab toolbar label');
  assert.ok(js.includes('route.hidden = true;'),'empty route starts hidden');
  assert.ok(js.includes('route.hidden = false;'),'valid route becomes visible');
  assert.ok(css.includes('P24.0: compact native UI'),'native UI patch');
  assert.ok(css.includes('#p22RouteSummary[hidden]'),'empty result hidden in CSS');
  assert.ok(css.includes('#p22RouteCard.p22-inline-route'),'route panel without double border');
  assert.ok(css.includes('@media (max-width:560px)'),'mobile layout');
  assert.doesNotMatch(js,/service\/gate\/control|method\s*:\s*['"]POST['"]/);
});

// P25: Advanced-only settings hub preserves basic mode, real service fields,
// P21 apply and all existing independent tabs.
test('P25 Svelte native Settings: reactive categories, preserved controls and read-only effects',()=>{
  const hub=fs.readFileSync(path.join(root,'DnsP25Settings.svelte'),'utf8');
  const sheet=fs.readFileSync(path.join(root,'p25-settings.css'),'utf8');
  assert.match(hub,/let active = \$state\(false\)/);
  assert.match(hub,/let section = \$state\('service'\)/);
  assert.match(hub,/let ipRows = \$state\(\[\]\)/);
  assert.match(hub,/let performanceStatus = \$state\(/);
  assert.ok(hub.includes('id="p25SettingsNav"'));
  assert.ok(hub.includes('id="p25SettingsCard"'));
  assert.ok(hub.includes('data-settings-section={item.id}'));
  assert.ok(hub.includes("{#each sections as item"));
  assert.equal((hub.match(/id="p25Settings_(service|cache|ip|upstream|security|privacy|dns64|performance)"/g)||[]).length,8);
  assert.ok(hub.includes("fetch('../service/ip-health',{method:'GET',cache:'no-store'})"));
  assert.ok(hub.includes("fetch('../service/status',{method:'GET',cache:'no-store'})"));
  assert.ok(hub.includes('clearInterval(timer)'));
  assert.ok(hub.includes('observer?.disconnect()'));
  assert.ok(hub.includes("returnTo(cacheField,originalCache)"));
  assert.ok(hub.includes("returnTo(additional,originalAdditional)"));
  assert.ok(hub.includes('id="p28Enabled"'));
  assert.ok(hub.includes('id="p30DNSSECMode"'));
  assert.ok(hub.includes('id="p31DNS64Prefix"'));
  assert.ok(hub.includes('id="p29Concurrent"'));
  assert.ok(hub.includes('P24.0 contains nine IP ordering strategies'));
  assert.ok(sheet.includes('.p25-settings-layout'));
  assert.doesNotMatch(hub,/RF_ACTUAL_CONTROLLER_START|document\.createElement|\.innerHTML|method\s*:\s*['"](?:POST|PUT|PATCH|DELETE)/i);
  assert.ok(!html.includes('p25-settings.js'));
});

// P24.3: real scope/hysteresis inputs, read-only IP monitor, no simple-mode mutation.
test('P24.3 IP scope and health HTTP controls are advanced-only',()=>{
  const panel=fs.readFileSync(path.join(root,'DnsP25Settings.svelte'),'utf8');
  const app=actualController('DnsLiteController.svelte');
  for(const id of ['p243Scope','p243Domains','p243ListIDs','p243Recoveries','p243MinDown','p243HTTPPath']){
    assert.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
    assert.ok(app.includes("byId('"+id+"')"),id+' must be wired');
  }
  assert.ok(panel.includes('scope = $state('));
  assert.ok(panel.includes("method:'GET'"));
  assert.doesNotMatch(panel,/method\s*:\s*['"](?:POST|PUT|PATCH|DELETE)/i);
  assert.ok(html.includes('option value="http"'));
  assert.ok(html.includes('option value="https"'));
  assert.ok(html.includes('p241-ip-controls'));
});

// P24.1: real config controls live only in the advanced settings hub.
test('P24.1 IP Intelligence: native Svelte Settings and guarded draft behavior',()=>{
  const hub=fs.readFileSync(path.join(root,'DnsP25Settings.svelte'),'utf8');
  const src=actualController('DnsLiteController.svelte');
  assert.ok(html.includes('id="p241IPControls"'));
  assert.ok(src.includes("if(byId('p241IPEnabled').checked){"));
  assert.ok(src.includes("prefs.ip_intelligence={enabled:true,mode:byId('p241IPMode').value};"));
  assert.ok(src.includes('delete merged.ip_intelligence'));
  assert.ok(src.includes("byId('p241IPMode').value=state.config.service?.ip_intelligence?.mode||'original'"));
  assert.ok(hub.includes('P24.0 contains nine IP ordering strategies'));
  assert.ok(hub.includes("round_robin:['Циклически меняет"));
  assert.ok(hub.includes("byId('p25Settings_ip')?.appendChild(byId('p241IPControls'))"));
  assert.ok(hub.includes('if(newMode){'));
  assert.ok(hub.includes('if(!advanced) return;'));
});

test('P24.2: health settings and monitor are advanced-only and read-only',()=>{
  const app=actualController('DnsLiteController.svelte');
  const hub=fs.readFileSync(path.join(root,'DnsP25Settings.svelte'),'utf8');
  const sheet=fs.readFileSync(path.join(root,'p25-settings.css'),'utf8');
  const {JSDOM}=require('jsdom');const doc=new JSDOM(html).window.document;
  for(const id of ['p242HealthEnabled','p242HealthProtocol','p242HealthPort','p242HealthInterval','p242HealthTimeout','p242HealthFailures','p242MonitorRows','p242MonitorStatus']){
    assert.equal(doc.querySelectorAll('#'+id).length,1,id);
    assert.equal(doc.getElementById(id).closest('#p241IPControls')!==null,true,id+' in advanced UI');
  }
  assert.ok(app.includes('interval_seconds:asInt'));
  assert.ok(app.includes('failure_threshold:asInt'));
  assert.ok(app.includes('p242HealthEnabled'));
  assert.ok(hub.includes("fetch('../service/ip-health',{method:'GET',cache:'no-store'})"));
  assert.ok(hub.includes('ipRows=monitorRowsFrom(data)'));
  assert.ok(sheet.includes('.p242-monitor-row'));
  assert.doesNotMatch(hub,/innerHTML\s*=/);
});

// P27: query protection must not be a decorative planned entry.
test('P27 query protection is a real advanced-only config section',()=>{
  const hub=fs.readFileSync(path.join(root,'DnsP25Settings.svelte'),'utf8');
  const app=actualController('DnsLiteController.svelte');
  for(const id of ['p27RateLimit','p27RefuseAny','p27BogusIPs']){
    assert.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
    assert.ok(app.includes("byId('"+id+"')"),id+' must read/write draft');
  }
  assert.ok(hub.includes("byId('p25Settings_security')?.appendChild(byId('p27SecurityControls'))"));
  assert.ok(hub.includes("id:'security',ru:'Защита запросов'"));
  assert.ok(app.includes('prefs.protection='));
  assert.ok(app.includes('delete merged.protection'));
  assert.ok(html.includes('p27SecurityControls'));
});
