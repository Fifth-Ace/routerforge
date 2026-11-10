'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const {fixture, until, tick, html} = require('./fixture.cjs');
const editor = (d, key) => d.querySelector('[data-native-svelte-domain-editor="true"] [data-rf-editor="'+key+'"]');
const get = d => editor(d,'domains');
function type(f, str) {
  const e=get(f.w.document);
  assert.ok(e,'Svelte editor is mounted');
  e.value=str;
  e.dispatchEvent(new f.w.Event('input',{bubbles:true}));
}
function mutations(f){return f.calls.filter(c=>!['GET','HEAD'].includes(c.opts.method||'GET'));}

test('No hidden editor exists, visible Svelte input is the only owner',async()=>{
  const prompts=[];
  const f=fixture({confirm:msg=>{prompts.push(msg);return false;}}),d=f.w.document;
  try {
    await until(()=>get(d)?.value==='example.com');
    assert.deepEqual(prompts,[],'read-only initial editor must not ask to discard a non-existent draft');
    assert.doesNotMatch(html,/id="(?:listEditor|editorDomains|listChoice|saveDomains|newListName|editorHint)"/);
    assert.equal(d.querySelectorAll('#listEditor,#editorDomains,#listChoice,#saveDomains').length,0);
    assert.equal(d.querySelectorAll('[data-native-svelte-domain-editor="true"]').length,1);
    assert.deepEqual(mutations(f),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});

test('Refusing cross-DNS switch keeps visible unsaved draft and selected DNS',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>get(d)?.value==='example.com');
    type(f,'example.com\nunsaved.example');
    f.w.confirm=()=>false;
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    await tick();
    assert.equal(get(d).value,'example.com\nunsaved.example');
    assert.equal(d.getElementById('editorDNSChoice').value,'old');
    assert.equal(d.querySelector('[data-upstream-id="old"]').classList.contains('is-selected'),true);
    assert.deepEqual(mutations(f),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});

test('Refusing named-list switch retains draft and restores visible selection',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>get(d)?.value==='example.com');
    const newName=editor(d,'new-name');
    newName.value='Игры';newName.dispatchEvent(new f.w.Event('input',{bubbles:true}));
    editor(d,'new-list').click();
    await until(()=>editor(d,'list-choice')?.querySelectorAll('option').length===2);
    const choice=editor(d,'list-choice');
    const before=choice.value;
    assert.ok(before&&before!=='__legacy__');
    type(f,'steamcommunity.com\nmy-draft.example');
    f.w.confirm=()=>false;
    choice.value='__legacy__';
    choice.dispatchEvent(new f.w.Event('change',{bubbles:true}));
    await tick();
    assert.equal(choice.value,before,'cancel must restore Svelte select');
    assert.equal(get(d).value,'steamcommunity.com\nmy-draft.example');
    assert.deepEqual(mutations(f),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});

test('Saving editor commits local draft only; network update requires explicit apply',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>get(d)?.value==='example.com');
    type(f,'example.com\nexplicit.net');
    editor(d,'save-list').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён'));
    assert.equal(get(d).value,'example.com\nexplicit.net');
    assert.equal(f.getConfig().rules.some(r=>r.domain==='explicit.net'),false,'server configuration must remain unchanged');
    assert.deepEqual(mutations(f),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});
