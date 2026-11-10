// Read-only source acceptance for DNS Runtime after retiring the unshipped
// imperative app.js. Full browser/CAS parity is exercised by separate gates.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const ui=path.join(root,'modules/dns-router-runtime/frontend');
const read=name=>fs.readFileSync(path.join(ui,name),'utf8');

test('DNS Runtime ships only one mounted native Svelte entry; legacy app.js absent',()=>{
  assert.equal(fs.existsSync(path.join(ui,'app.js')),false,'unshipped legacy source must be retired');
  const html=read('index.html');
  const main=read('main.js');
  assert.match(html,/src="svelte-mode\.js"/);
  assert.doesNotMatch(html,/src="app\.js"|<script[^>]*>\s*\(\(\)\s*=>/);
  assert.match(main,/import \{mount\} from 'svelte'/);
  assert.match(main,/import DnsRuntimeApp from '\.\/DnsRuntimeApp\.svelte'/);
  assert.match(main,/mount\(DnsRuntimeApp,\{target\}\)/);
  assert.doesNotMatch(main,/app\.js|innerHTML\s*=/);
});

test('Svelte 5 read-only props remain reactive without resetting after a user action',()=>{
  const source=read('DnsRuntimeReadOnly.svelte');
  assert.match(source,/\$effect\(\(\)\s*=>\s*\{/);
  assert.match(source,/output\s*=\s*initial\[kind\]\s*\|\|\s*''/);
  assert.match(source,/kind\s*===\s*'upstreams'/);
  assert.match(source,/if\s*\(busy\)\s*return/);
  assert.match(source,/method:\s*'POST'/);
  assert.match(source,/X-RouterForge-Action/);
  assert.doesNotMatch(source,/\.innerHTML\s*=/);
});

test('Native picker availability is reactive and no network check happens on mount',()=>{
  const source=read('DnsRuntimeNative.svelte');
  assert.match(source,/let checked=\$state\(false\)/);
  assert.match(source,/disabled=\{busy\|\|!checked\|\|!choices\.length\}/);
  assert.match(source,/onDestroy\(\(\)=>\{stopped=true;sequence\+\+;\}\)/);
  assert.match(source,/There is deliberately no onMount network request or automatic probe/);
});

test('ARIA tablist uses a generic container, with keyboard-friendly Svelte tabs',()=>{
  const source=read('DnsRuntimeApp.svelte');
  assert.match(source,/<div class="tabs" role="tablist" aria-label="Разделы DNS Router">/);
  assert.doesNotMatch(source,/<nav class="tabs" role="tablist"/);
  for(const tab of ['overview','policies','testing','keenetic','diagnostics']){
    assert.match(source,new RegExp('id="dr-tab-'+tab+'" role="tab"'));
    assert.match(source,new RegExp('aria-controls="dr-panel-'+tab+'"'));
  }
  assert.match(source,/onkeydown=\{e=>tabKey\(e,'overview'\)\}/);
});


// Fail closed for BOTH direct and constructed path expressions.
// The old gate matched only the literal "frontend/app.js" and missed
// path.join(ui,'app.js'), path.join(base,'app.js'), prefix+'app.js'.
function retiredRuntimeRead(source){
  const lines=source.split(/\r?\n/);
  return lines.some(line=>{
    if(!/app\.js/.test(line) || !/(?:readFileSync|readFile|\bread\(|\bimport\(|\brequire\(|path\.join|p\.join)/.test(line))return false;
    return /(?:readFileSync|readFile|\bread\(|\bimport\(|\brequire\(|path\.join|p\.join)/.test(line);
  });
}

test('Retired-source detector catches direct, alias, and joined historical reads',()=>{
  assert.equal(retiredRuntimeRead("fs.readFileSync(path.join(ui,'app.js'),'utf8');"),true);
  assert.equal(retiredRuntimeRead("fs.readFileSync(path.join(base,'app.js'),'utf8');"),true);
  assert.equal(retiredRuntimeRead("fs.readFileSync(path.join(root,prefix+'app.js'),'utf8');"),true);
  assert.equal(retiredRuntimeRead("const ui=read(path+'frontend/app.js')"),true);
  assert.equal(retiredRuntimeRead("assert.doesNotMatch(html,/src=\"app.js\"/);"),false);
});

test('ALL historical DNS Runtime JS tests run without reading retired app.js',()=>{
  const testsDir=path.join(root,'tests');
  const offenders=[];
  for(const file of fs.readdirSync(testsDir)){
    if(!/\.(?:js|mjs|cjs)$/.test(file) || file==='dns_runtime_svelte5_ownership.test.mjs')continue;
    const content=fs.readFileSync(path.join(testsDir,file),'utf8');
    if(!content.includes('dns-router-runtime'))continue;
    if(retiredRuntimeRead(content))offenders.push(file);
  }
  assert.deepEqual(offenders,[],'legacy test still depends on retired runtime app.js');
});
