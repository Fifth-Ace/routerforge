'use strict';
const test=require('node:test');
const a=require('node:assert/strict');
const {fixture,until,tick}=require('./fixture.cjs');
const noMutation=f=>a.equal(f.calls.filter(c=>c.opts.method&&c.opts.method!=='GET').length,0);

test('P21/P22 Svelte workspace owns exactly one device/group/rule form and nav entry',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.querySelector('#p22DevicesCard #p21SetDevice')&&d.querySelector('#p22GroupsCard #p21SetGroup')&&d.querySelector('#p22RulesCard #p21SetRule'));
  for(const id of ['p21MAC','p21Members','p21Priority','p21SetDevice','p21SetGroup','p21SetRule','p21Nav','p22NavGroups','p22NavRules'])a.equal(d.querySelectorAll('#'+id).length,1,id);
  noMutation(f);
 }finally{f.dom.window.close();}
});

test('P22 native navigation keeps one visible section and never mutates DNS',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p22NavGroups'));
  d.getElementById('p22NavGroups').click();
  await until(()=>!d.getElementById('p22GroupsCard').hidden);
  a.equal(d.getElementById('p22DevicesCard').hidden,true);
  a.equal(d.getElementById('p22RulesCard').hidden,true);
  a.equal(d.getElementById('p22NavGroups').getAttribute('aria-current'),'page');
  d.getElementById('p22NavRules').click();
  await until(()=>!d.getElementById('p22RulesCard').hidden);
  a.equal(d.getElementById('p22GroupsCard').hidden,true);
  noMutation(f);
 }finally{f.dom.window.close();}
});

test('P21 native forms reject invalid group and leave draft unchanged',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p21GroupName') && d.getElementById('feedback')?.textContent.includes('Настройки загружены'));
  d.getElementById('p21GroupName').value='Invalid';
  d.getElementById('p21Members').value='ff:ff:ff:ff:ff:ff';
  d.getElementById('p21GroupTarget').value='dns:cf';
  d.getElementById('p21SetGroup').click();
  await until(()=>d.getElementById('feedback').dataset.tone==='bad');
  a.equal(d.querySelectorAll('[data-p21-row="groups"]').length,0);
  noMutation(f);
 }finally{f.dom.window.close();}
});

test('P25 settings is a native navigation section, opening never applies DNS',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p25SettingsNav')&&d.getElementById('p22DevicesCard'));
  d.getElementById('p25SettingsNav').click();
  await until(()=>!d.getElementById('p25SettingsCard').hidden);
  a.equal(d.getElementById('p22DevicesCard').hidden,true);
  a.equal(d.getElementById('p22GroupsCard').hidden,true);
  a.equal(d.getElementById('p22RulesCard').hidden,true);
  d.getElementById('p21Nav').click();
  await until(()=>!d.getElementById('p22DevicesCard').hidden&&d.getElementById('p25SettingsCard').hidden);
  noMutation(f);
 }finally{f.dom.window.close();}
});

test('P22 route is empty until an explicit evaluation, and claims configuration only',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p21Probe'));
  a.equal(d.querySelectorAll('#p22RouteSummary .p22-route-chip').length,0);
  d.getElementById('p21ProbeDomain').value='example.com';
  d.getElementById('p21ProbeMAC').value='02:11:22:33:44:55';
  d.getElementById('p21Probe').click();
  await until(()=>d.querySelectorAll('#p22RouteSummary .p22-route-chip').length>=3);
  a.match(d.getElementById('p22RouteSummary').textContent,/не подтверждённый сетевой E2E/);
  a.equal(f.calls.filter(c=>c.path==='service/devices/evaluate').length,1);
  a.equal(f.calls.filter(c=>c.path==='service/gate/control'||c.path==='service/control').length,0);
 }finally{f.dom.window.close();}
});
