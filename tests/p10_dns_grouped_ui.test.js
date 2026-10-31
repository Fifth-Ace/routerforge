'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const root=path.resolve(__dirname,'..'),prefix='modules/dns-router-runtime/frontend/';
const js=fs.readFileSync(path.join(root,prefix+'app.js'),'utf8');
const html=fs.readFileSync(path.join(root,prefix+'index.html'),'utf8');
const css=fs.readFileSync(path.join(root,prefix+'module.css'),'utf8');
const start=js.indexOf('  // P10 UX F3: GROUP BY DNS UPSTREAM;');
const end=js.indexOf('  function p88RefreshStatus(){',start);
const fragment=js.slice(start,end);
test('P10 F3 grouped DNS view is installed in actual shipped frontend',()=>{
 assert.ok(start>0&&end>start);assert.match(js,/function p10UXGroups\(\)/);
 assert.match(js,/function p10UXReplaceDraft\(chosenDNS,parsed\)/);
 assert.match(html,/DNS-сервер → привязанный список доменов/);
 for(const id of ['drP10EditingLabel','drP10CancelListEdit'])assert.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1);
 assert.match(css,/"dns rules" "dns domains"/);
 assert.match(css,/"dns" "rules" "domains"/);
 assert.match(js,/DNS → домены \(все сохранённые правила\)/);
 assert.doesNotMatch(fragment,/\.innerHTML\s*=|fetch\(|request\(|ndmc|shadow\/control|durable\/apply/);
});
function groups(policy){
 const cfg={upstreams:[
  {id:'native1',transport:'native',address:'native://System/./1.1.1.1'},
  {id:'native2',transport:'native',address:'native://Policy1/./9.9.9.9'},
  {id:'doh',transport:'doh',server_name:'dns.example.com',address:'10.1.1.1',path:'/dns-query'},
  {id:'udp',transport:'udp',address:'8.8.8.8'}
 ],rules:[
  {id:'r1',domain:'youtube.com',upstream:'doh',match:'suffix',priority:100},
  {id:'r2',domain:'googlevideo.com',upstream:'doh',match:'suffix',priority:100},
  {id:'r3',domain:'example.ru',upstream:'native1',match:'suffix',priority:100},
  {id:'r4',domain:'example.net',upstream:'native2',match:'suffix',priority:100},
  {id:'r5',domain:'bad.invalid',upstream:'missing',match:'suffix',priority:100}
 ]};
 const stub=`(()=>{const config=data;const p88={policy:${JSON.stringify(policy)}};const p89NativeEntries=()=>[{selector:'native://System/./1.1.1.1',label:'Cloudflare DNS · System',zone:'.',address:'127.0.0.1:40500'}];`+fragment+';return {groups:p10UXGroups(),summary:p10UXApplySummary()};})()';
 return vm.runInNewContext(stub,{data:cfg});
}
test('P10 F3 preserves per-server domain ownership and does not hide broken references',()=>{
 const a=groups('System');
 const x=a.groups.find(x=>x.id==='doh');assert.equal(x.rules.length,2);
 assert.equal(x.rules[0].domain,'youtube.com');
 assert.match(x.info.title,/dns\.example\.com/);
 assert.match(x.info.detail,/https:\/\/dns\.example\.com\/dns-query/);
 assert.equal(a.groups.find(x=>x.id==='native1').rules.length,1);
 assert.ok(!a.groups.some(x=>x.id==='native2'));
 assert.equal(a.groups.find(x=>x.id==='missing').info.scope,'broken');
 assert.match(a.summary,/doh.*2 доменов/);
 const b=groups('Policy1');assert.ok(b.groups.some(x=>x.id==='native2'));
 assert.ok(!b.groups.some(x=>x.id==='native1'));
});
test('P10 F3 edit is explicit, scoped and preserves advanced rules',()=>{
 assert.match(fragment,/p10UXEditing=g\.id/);
 assert.match(fragment,/g\.rules\.some\(r=>!p10UXIsSimpleRule\(r\)\)/);
 assert.match(fragment,/r\.upstream!==p10UXEditing/);
 assert.match(fragment,/chosenDNS\.zone/);
 assert.match(fragment,/confirm\('Обновить список только этого DNS/);
 assert.match(fragment,/markDirty\(\)/);
 assert.match(fragment,/Object\.keys\(r\)\.every/);
 assert.match(js,/if\(p10UXEditing\)\{p10UXReplaceDraft\(chosenDNS,parsed\);return;\}/);
});
