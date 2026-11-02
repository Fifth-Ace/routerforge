'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'../frontend');
const html=fs.readFileSync(path.join(root,'index.html'),'utf8');
const js=fs.readFileSync(path.join(root,'p22-visual.js'),'utf8');
const css=fs.readFileSync(path.join(root,'p22-visual.css'),'utf8');
const original=fs.readFileSync(path.join(root,'app.js'),'utf8');

test('P22 screenshot layout: segmented mode, independent visible tabs, and route cards',()=>{
  for(const id of ['p22ModeBasic','p22ModeAdvanced','p22NavGroups','p22NavRules',
    'p22DevicesCard','p22GroupsCard','p22RulesCard','p22RouteCard','p22RouteSummary',
    'p22DevicePick','p22MemberSelect','p22RulePick','p22ProbePick','p22MemberChips']){
    assert.equal((html.match(new RegExp(`id="${id}"`,'g'))||[]).length,1,id);
  }
  assert.match(html,/p22-visual\.css/);assert.match(html,/p22-visual\.js/);
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
