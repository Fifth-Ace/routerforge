import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const source=name=>readFileSync(new URL(name,import.meta.url),'utf8');

test('overview quick tasks are owned by Svelte; legacy assignments cannot duplicate navigation',()=>{
  const app=source('NFQWSManagerApp.svelte'),runtime=source('NfqwsRuntime.js');
  for(const id of ['uxTaskBroken','uxTaskCheck','uxTaskAutoV2','uxTaskImport']){
    assert.match(app,new RegExp(`id="${id}" onclick=\\{\\(\\)=>openOverviewTask\\(`));
    assert.doesNotMatch(runtime,new RegExp(`\\$\\('${id}'\\)\\.onclick`));
  }
  assert.doesNotMatch(runtime,/function openOverviewTask\(/);
});
test('setup accordion and workspace resizing follow Svelte mount/unmount',()=>{
  const app=source('NFQWSManagerApp.svelte'),runtime=source('NfqwsRuntime.js');
  const editor=source('NfqwsListEditor.svelte'),scripts=source('NfqwsScripts.svelte');
  const actions=source('NfqwsUIActions.js');
  assert.equal((app.match(/use:setupAccordion=/g)||[]).length,13);
  assert.match(editor,/use:workspaceHeight/);assert.match(scripts,/use:workspaceHeight/);
  assert.match(actions,/removeEventListener\('click'/);
  assert.match(actions,/observer\?\.disconnect\(\)/);
  assert.doesNotMatch(runtime,/setupResizableWorkspace|addCollapseControls|setupCollapseTimers|renderAllEditorResources|mountSyntaxTextarea/);
  assert.doesNotMatch(runtime,/\$\('backupCount'\)/);
});
test('recovery, blob and Smart Apply commands remain guarded, never auto-write on mount',()=>{
  const r=source('NfqwsRuntime.js');
  for(const x of ['NFQWS_BLOB_MUTATION_INVALID_RESPONSE','NFQWS_BLOB_INSTALL_POSTVERIFY_MISMATCH',
    'NFQWS_BLOB_DELETE_POSTVERIFY_MISMATCH',"confirm:'NFQWS_SMART_APPLY'",'NFQWS_BACKUP_RESTORE']){
    assert.ok(r.includes(x),x);
  }
  assert.match(r,/const lifecycle = new AbortController\(\)/);
  assert.match(r,/lifecycle\.abort\(\)/);
  assert.doesNotMatch(source('NfqwsUIActions.js'),/fetch\(|method:\s*['"]POST|\.innerHTML/);
});
