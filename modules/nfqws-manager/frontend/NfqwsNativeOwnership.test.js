import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
const src=name=>readFileSync(new URL('./'+name,import.meta.url),'utf8');
test('Svelte owns Inspector, Static Path, Cross List and Preflight modal DOM, not legacy runtime',()=>{
 const app=src('NFQWSManagerApp.svelte'),runtime=src('NfqwsRuntime.js'),native=src('NfqwsListTools.svelte');
 for(const name of ['entryInspectorModal','analysisModal','staticPathModal']){
   assert.doesNotMatch(runtime,new RegExp(`\\$\\('${name}'\\)`));
   assert.doesNotMatch(app,new RegExp(`<div id="${name}" class="modal">`));
   assert.match(native,new RegExp(`id="${name}"`));
 }
 assert.doesNotMatch(runtime,/\$\('listEditor'\)/);
 assert.match(app,/<NfqwsListTools inspector=/);
 assert.match(app,/<NfqwsPreflight view=/);
 assert.doesNotMatch(runtime,/\$\('preflightGroups'\)|\.innerHTML.*preflight/);
});
test('native list editor exposes cursor line and opening exact selected line as intentional API',()=>{
 const editor=src('NfqwsListEditor.svelte');
 assert.match(editor,/caretLine:/);assert.match(editor,/openAtLine/);assert.match(editor,/onRegister\(\{select,readContext,openAtLine\}\)/);
});
test('destructive cross-list cleanup requires same draft, complete inventory and exact SHA',()=>{
 const runtime=src('NfqwsRuntime.js');
 assert.match(runtime,/context\.value!==analysisDraftContent\|\|analysisIncomplete/);
 assert.match(runtime,/expected_sha256:expected,confirm:'NFQWS_LIST_SAVE'/);
 assert.match(runtime,/\^\[a-f0-9\]\{64\}\$/i);
 assert.match(src('NfqwsListTools.svelte'),/analysis\.incomplete/);
});
