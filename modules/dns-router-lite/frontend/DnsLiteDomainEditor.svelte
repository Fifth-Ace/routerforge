<script>
  import {onMount} from 'svelte';
  let opened=$state(false);
  let title=$state('Список доменов');
  let choices=$state([]);
  let selected=$state('');
  let domains=$state('');
  let newName=$state('');
  let renameName=$state('');
  let editable=$state(false);
  let named=$state(false);
  let hint=$state('Изменения попадут в черновик и начнут действовать после применения.');
  let counter=$derived(domains.split(/\r?\n/).filter(x=>x.trim()).length);
  const byId=id=>document.getElementById(id);
  function snapshot(){
    const raw=window.__rfDnsLiteEditorSnapshot?.();
    if(!raw)return;
    opened=!!raw.opened;
    if(!opened)return;
    title=raw.title;
    choices=raw.choices;selected=raw.selected;
    domains=raw.domains;renameName=raw.renameName;
    editable=raw.editable;named=raw.named;hint=raw.hint;
  }
  function hiddenValue(id,value){const el=byId(id);if(el)el.value=value;}
  function clickLegacy(id){byId(id)?.click();}
  function selectList(event){
    hiddenValue('listChoice',event.currentTarget.value);
    byId('listChoice')?.dispatchEvent(new Event('change',{bubbles:true}));
    snapshot();
  }
  function domainInput(event){
    domains=event.currentTarget.value;
    hiddenValue('editorDomains',domains);
    byId('editorDomains')?.dispatchEvent(new Event('input',{bubbles:true}));
  }
  function createList(){hiddenValue('newListName',newName);clickLegacy('newListButton');snapshot();}
  function renameList(){hiddenValue('listRenameName',renameName);clickLegacy('renameListButton');snapshot();}
  function saveList(){hiddenValue('editorDomains',domains);clickLegacy('saveDomains');snapshot();}
  onMount(()=>{
    const legacy=byId('listEditor');
    if(!legacy)throw Error('DNS_LITE_LEGACY_EDITOR_CONTRACT_MISSING');
    const original=legacy.style.display;
    legacy.style.display='none';
    window.__rfDnsLiteNativeEditorLoaded=true;
    window.addEventListener('routerforge:dns-lite-editor-state',snapshot);
    snapshot();
    return ()=>{
      window.removeEventListener('routerforge:dns-lite-editor-state',snapshot);
      legacy.style.display=original;
      window.__rfDnsLiteNativeEditorLoaded=false;
    };
  });
</script>

{#if opened}
<div class="editor" data-native-svelte-domain-editor="true">
  <div class="section-title">
    <div><h3>{title}</h3><p>У каждого DNS может быть несколько независимых списков. Один домен на строку.</p></div>
    <button type="button" class="button subtle" onclick={()=>clickLegacy('closeEditor')}>Закрыть</button>
  </div>
  <div class="list-toolbar">
    <label class="field"><span>Список выбранного DNS</span>
      <select value={selected} onchange={selectList}>
        {#each choices as item (item.id)}<option value={item.id}>{item.label}</option>{/each}
      </select>
    </label>
    <label class="field"><span>Новый список</span><input maxlength="80" placeholder="Например, Стриминг" bind:value={newName}/></label>
    <button type="button" class="button" onclick={createList}>＋ Создать</button>
  </div>
  <div class="list-toolbar list-rename">
    <label class="field"><span>Название списка</span><input maxlength="80" placeholder="Название" bind:value={renameName} disabled={!named}/></label>
    <button type="button" class="button" disabled={!named} onclick={renameList}>Переименовать</button>
    <button type="button" class="button danger" disabled={!named} onclick={()=>clickLegacy('deleteListButton')}>Удалить список</button>
  </div>
  <textarea rows="16" spellcheck="false" placeholder={'youtube.com\ngooglevideo.com\nytimg.com'} value={domains} oninput={domainInput} readonly={!editable}></textarea>
  <div class="editor-meta"><span>{counter} строк (черновик)</span><span>Дубликаты удаляются при сохранении списка</span></div>
  <div class="actions editor-actions">
    <button type="button" class="button" disabled={!editable} onclick={()=>clickLegacy('importDomains')}>Импорт TXT</button>
    <button type="button" class="button" onclick={()=>{hiddenValue('editorDomains',domains);clickLegacy('exportDomains');}}>Экспорт TXT</button>
    <button type="button" class="button primary" disabled={!editable} onclick={saveList}>✓ Сохранить список</button>
  </div>
  <small class="muted">{hint}</small>
</div>
{/if}
