'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const root = path.resolve(__dirname, '..');
const front = path.join(root, 'modules/dns-router-runtime/frontend');
const read = name => fs.readFileSync(path.join(front,name),'utf8');
const load = name => import(pathToFileURL(path.join(front,name)).href);

test('P92 desktop 40/60 policy strip and responsive DNS layout are preserved',()=>{
  const css=read('module.css');
  assert.match(css,/#drSimpleWorkspace \.dr-simple-policies\{display:flex;flex-wrap:nowrap/);
  assert.match(css,/overflow-x:auto/);
  assert.match(css,/grid-template-columns:minmax\(0,2fr\) minmax\(0,3fr\)/);
  assert.match(css,/grid-template-areas:"policies policies" "dns rules" "dns domains"/);
  assert.match(css,/@media\(max-width:920px\)/);
  assert.match(css,/grid-template-areas:"policies" "dns" "rules" "domains"/);
});

test('P92 the real Svelte tree owns exactly one domain editor and action',()=>{
  const app=read('DnsRuntimeApp.svelte'),p92=read('DnsRuntimeP92.svelte');
  const p10=read('DnsRuntimeP10.svelte'),p91=read('DnsRuntimeP91.svelte');
  assert.match(app,/id="drSimpleWorkspace"/);
  assert.match(app,/<DnsRuntimeP92\s*\/>/);
  assert.match(app,/<DnsRuntimeP10\s*\/>/);
  for(const id of ['drP92Check','drP92Clear','drP92Deduplicate','drP92Normalize','drP92Import','drP92Export','drP92File','drP92Preview','drP92Count','drP92Message'])
    assert.equal((p92.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  assert.equal((p10.match(/id="drP92Apply"/g)||[]).length,1);
  assert.match(p91,/id="drP90Endpoint"/);
});

test('P92 pure parser normalizes and deduplicates; never stages a DNS mutation',async()=>{
  const {parseDomainBatch,previewDomainBatch,mergeDomainFile,exportDomainText}=await load('DnsRuntimeP92Logic.js');
  assert.deepEqual(parseDomainBatch('Example.COM.\nexample.com\nsecond.net # comment').domains,['example.com','second.net']);
  const preview=previewDomainBatch('Example.COM.\nsecond.net');
  assert.equal(preview.ok,true);assert.equal(preview.domains.length,2);
  assert.equal(mergeDomainFile('example.com','SECOND.net',20),'example.com\nsecond.net');
  assert.equal(exportDomainText('EXAMPLE.com'),'example.com\n');
  assert.match(parseDomainBatch('https://example.com').error,/Некорректный домен/);
  assert.throws(()=>mergeDomainFile('','example.com',256*1024+1),/Файл слишком большой/);
});

test('P92 controls have explicit handlers, cannot implicitly apply or fetch',()=>{
  const p92=read('DnsRuntimeP92.svelte');
  for(const fn of ['check','clear','canonicalize','importFile','exportFile'])
    assert.match(p92,new RegExp('function '+fn+'\\('));
  assert.match(p92,/onclick=\{check\}/);
  assert.match(p92,/onclick=\{exportFile\}/);
  assert.doesNotMatch(p92,/fetch\s*\(|ndmc|shadowApply|shadowStart|innerHTML\s*=/);
  assert.match(p92,/if\(file\.size>256\*1024\)throw Error/);
});
