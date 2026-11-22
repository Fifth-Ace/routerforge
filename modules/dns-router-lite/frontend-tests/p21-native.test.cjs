'use strict';
const test=require('node:test');
const a=require('node:assert/strict');
const {fixture,until}=require('./fixture.cjs');
const mac='02:11:22:33:44:55';
const dns={upstream:'cf'};
const advanced={devices:[{mac,name:'Configured',target:dns}],groups:[{id:'g1',name:'Staff',macs:[mac],target:dns}],rules:[{id:'r1',mac,domain:'example.com',match:'suffix',priority:100,target:dns}]};

test('P21 Svelte list presents discovered, configured, group and rule rows without implicit writes',async()=>{
 const f=fixture({mode:'advanced',advanced}),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('[data-p21-row="devices"]').length>0);
  a.equal(d.querySelectorAll('[data-p21-row="devices"]').length,1);
  a.equal(d.querySelectorAll('[data-p21-row="groups"]').length,1);
  a.equal(d.querySelectorAll('[data-p21-row="rules"]').length,1);
  a.match(d.querySelector('[data-p21-row="devices"]').textContent,/Configured/);
  a.equal(f.calls.some(c=>c.opts.method&&c.opts.method!=='GET'),false);
 }finally{f.dom.window.close();}
});

test('P21 Svelte row selection populates editor without applying',async()=>{
 const f=fixture({mode:'advanced',advanced}),d=f.w.document;
 try{
  await until(()=>d.querySelector('[data-p21-row="devices"] button'));
  d.querySelector('[data-p21-row="devices"] button').click();
  await until(()=>d.getElementById('p21MAC').value===mac);
  a.equal(d.getElementById('p21Target').value,'dns:cf');
  a.equal(f.getActive(),false);
  a.equal(f.calls.some(c=>c.opts.method&&c.opts.method!=='GET'),false);
 }finally{f.dom.window.close();}
});

test('P21 Svelte draft removal updates rows but does not save or apply live DNS',async()=>{
 const f=fixture({mode:'advanced',advanced}),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('[data-p21-row="rules"]').length===1);
  d.querySelector('[data-p21-row="rules"] .actions button:last-child').click();
  await until(()=>d.querySelectorAll('[data-p21-row="rules"]').length===0);
  a.equal(f.getActive(),false);
  a.equal(f.calls.some(c=>c.opts.method&&c.opts.method!=='GET'),false);
  a.equal(d.getElementById('feedback').textContent.includes('несохранённые'),true);
 }finally{f.dom.window.close();}
});

test('P22 Svelte member chips mirror draft textarea and remove only the selected MAC',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p22MemberChips')&&d.getElementById('p21Members'));
  const members=d.getElementById('p21Members');
  members.value=`${mac}\n02:11:22:33:44:66`;
  members.dispatchEvent(new f.w.Event('input',{bubbles:true}));
  await until(()=>d.querySelectorAll('#p22MemberChips .p22-chip').length===2);
  d.querySelector('#p22MemberChips .p22-chip').click();
  await until(()=>d.querySelectorAll('#p22MemberChips .p22-chip').length===1);
  a.equal(members.value,'02:11:22:33:44:66');
  a.ok(f.calls.every(c=>!c.opts.method||c.opts.method==='GET'));
 }finally{f.dom.window.close();}
});

test('P22 Svelte route explanation displays configuration-only evidence after explicit probe',async()=>{
 const f=fixture({mode:'advanced'}),d=f.w.document;
 try{
  await until(()=>d.getElementById('p21Probe')&&!d.getElementById('p21Probe').disabled);
  d.getElementById('p21ProbeDomain').value='example.com';
  d.getElementById('p21ProbeMAC').value=mac;
  d.getElementById('p21Probe').click();
  await until(()=>d.querySelectorAll('#p22RouteSummary .p22-route-chip').length>=3);
  a.match(d.getElementById('p22RouteSummary').textContent,/не подтверждённый сетевой E2E/);
  a.equal(f.calls.filter(c=>c.path==='service/devices/evaluate').length,1);
  a.equal(f.calls.some(c=>c.path==='service/gate/control'||c.path==='service/control'||c.path==='config'&&c.opts.method==='PUT'),false);
 }finally{f.dom.window.close();}
});
