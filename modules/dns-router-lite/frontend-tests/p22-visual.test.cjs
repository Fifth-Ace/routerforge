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
