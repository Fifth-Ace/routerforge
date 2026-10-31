'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'modules/dns-router-runtime/frontend/index.html'),'utf8');
const css=fs.readFileSync(path.join(root,'modules/dns-router-runtime/frontend/module.css'),'utf8');
const js=fs.readFileSync(path.join(root,'modules/dns-router-runtime/frontend/app.js'),'utf8');
test('P92 horizontal scrolling policy strip retains native policy action',()=>{
 assert.match(css,/#drSimpleWorkspace \.dr-simple-policies\{display:flex;flex-wrap:nowrap/);
 assert.match(css,/overflow-x:auto/);
 assert.match(js,/function p88RenderPolicies\(\)/);
 assert.match(js,/p88\.policy=instance\.name;p88RenderPolicies\(\);p88RenderDNS\(\);p88RenderRules\(\)/);
});
test('P92 exact 40/60 desktop split and single-column responsive layout',()=>{
 assert.match(css,/grid-template-columns:minmax\(0,2fr\) minmax\(0,3fr\)/);
 assert.match(css,/grid-template-areas:"policies policies" "dns domains" "dns rules"/);
 assert.match(css,/#drSimpleWorkspace \.dr-simple-main\{display:contents\}/);
 assert.match(css,/@media\(max-width:920px\)/);
 assert.match(css,/grid-template-areas:"policies" "dns" "domains" "rules"/);
});
test('P92 domain editor controls are unique and retain the existing IDs',()=>{
 for(const id of ['drSimplePolicies','drSimpleDNS','drSimpleDNSInventory','drSimpleDomains','drSimpleAdd','drSimpleSave','drSimpleRuleList','drP90Inspect','drP90Store','drP92Check','drP92Clear','drP92Deduplicate','drP92Normalize','drP92Import','drP92Export','drP92File','drP92Preview','drP92Count','drP92Message','drP92Apply']){
  assert.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
 }
 assert.match(html,/id="drSimpleDomains" rows="12"/);
 assert.match(html,/id="drP92Apply">▶ Применить DNS/);
 assert.match(html,/id="drP10Disable"/);
});
test('P92 controls are wired, no implicit DNS runtime mutations',()=>{
 for(const id of ['drP92Check','drP92Clear','drP92Deduplicate','drP92Normalize','drP92Import','drP92File','drP92Export','drP92Apply'])assert.match(js,new RegExp("p88Id\\('"+id+"'\\)\\.addEventListener"));
 assert.match(js,/function p92Preview\(\)/);
 assert.match(js,/const parsed=p88ParseDomains\(p92Field\.value\)/);
 assert.match(js,/p88Id\('drModeAdvanced'\)\.click\(\)/);
 const section=js.slice(js.indexOf('// P9.2 UX-only'),js.indexOf('  // DPR2 P10: a real, explicit System DNS activation'));
 assert.doesNotMatch(section,/\.innerHTML\s*=|ndmc|shadowApply|shadowStart|saveButton.*click|fetch\(|request\(/);
 assert.match(section,/p92Field\.value=parsed\.domains\.join\('\\n'\)/);
 assert.match(js,/p88Id\('drP92Apply'\)\.addEventListener\('click',p10Activate\)/);
 assert.doesNotMatch(js,/p92Notify\('Реальное применение DNS выполняется отдельно/);
});
test('P92 existing advanced and P91 DNS form retained',()=>{
 assert.match(html,/id="drAdvancedWorkspace" hidden/);
 assert.match(html,/id="drP91Found"/);
 assert.match(js,/function p91Fields\(requireIP\)/);
 assert.match(js,/async function p88Scan\(\)/);
 assert.match(js,/p88Id\('drSimpleSave'\)\.addEventListener\('click',\(\)=>p88Id\('saveButton'\)\.click\(\)\)/);
});
