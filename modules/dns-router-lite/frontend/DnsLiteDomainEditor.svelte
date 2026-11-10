<script>
  let {snapshot = {}, onAction = () => {}} = $props();
  let newName = $state('');
  let renameName = $state('');
  let domains = $state('');
  let lastIdentity = '';
  let importInput = $state(null);
  async function importTxt(event) {
    const input = event.currentTarget;
    const file = input.files?.[0];
    if (!file) return;
    try {
      if (file.size > 256*1024) throw Error('Максимум 256 КБ');
      command('import-data', await file.text());
    } catch (e) { command('import-error', e?.message || String(e)); }
    finally { input.value = ''; }
  }
  function exportTxt() {
    if (typeof URL.createObjectURL !== 'function') {command('export-error'); return;}
    const url = URL.createObjectURL(new Blob([domains.endsWith('\n') ? domains : domains+'\n'], {type:'text/plain;charset=utf-8'}));
    try { const anchor = document.createElement('a'); anchor.href=url; anchor.download='routerforge-domains.txt'; anchor.click(); }
    finally { URL.revokeObjectURL(url); }
  }
  let opened = $derived(Boolean(snapshot.opened));
  let title = $derived(snapshot.title || 'Список доменов');
  let choices = $derived(snapshot.choices || []);
  let selected = $derived(snapshot.selected || '');
  let editable = $derived(Boolean(snapshot.editable));
  let named = $derived(Boolean(snapshot.named));
  let hint = $derived(snapshot.hint || 'Изменения попадут в черновик и начнут действовать после применения.');
  let counter = $derived(domains.split(/\r?\n/).filter(x=>x.trim()).length);
  $effect(() => {
    const generation = snapshot.generation || 0;
    const identity = `${snapshot.selected || ''}|${generation}|${snapshot.opened ? 'open':'closed'}`;
    if (identity !== lastIdentity) {
      lastIdentity = identity;
      domains = snapshot.domains || '';
      renameName = snapshot.renameName || '';
      newName = snapshot.newName || '';
    }
  });
  function command(action, value) { return onAction(action,value); }
</script>

{#if opened}
<div class="editor" data-native-svelte-domain-editor="true">
  <div class="section-title">
    <div><h3>{title}</h3><p>У каждого DNS может быть несколько независимых списков. Один домен на строку.</p></div>
    <button type="button" class="button subtle" onclick={()=>command('close')}>Закрыть</button>
  </div>
  <div class="list-toolbar">
    <label class="field"><span>Список выбранного DNS</span>
      <select data-rf-editor="list-choice" value={selected} onchange={e=>{if(command('select',e.currentTarget.value)===false)e.currentTarget.value=selected;}}>
        {#each choices as item (item.id)}<option value={item.id}>{item.label}</option>{/each}
      </select>
    </label>
    <label class="field"><span>Новый список</span><input data-rf-editor="new-name" maxlength="80" placeholder="Например, Стриминг" bind:value={newName} oninput={e=>command('new-name',e.currentTarget.value)}/></label>
    <button data-rf-editor="new-list" type="button" class="button" onclick={()=>command('create',newName)}>＋ Создать</button>
  </div>
  <div class="list-toolbar list-rename">
    <label class="field"><span>Название списка</span><input data-rf-editor="rename-name" maxlength="80" placeholder="Название" bind:value={renameName} oninput={e=>command('rename-name',e.currentTarget.value)} disabled={!named}/></label>
    <button data-rf-editor="rename-list" type="button" class="button" disabled={!named} onclick={()=>command('rename',renameName)}>Переименовать</button>
    <button data-rf-editor="delete-list" type="button" class="button danger" disabled={!named} onclick={()=>command('delete')}>Удалить список</button>
  </div>
  <textarea data-rf-editor="domains" rows="16" spellcheck="false" placeholder={'youtube.com\ngooglevideo.com\nytimg.com'} value={domains} oninput={e=>{domains=e.currentTarget.value;command('input',domains);}} readonly={!editable}></textarea>
  <div class="editor-meta"><span data-rf-editor="count">{counter} строк (черновик)</span><span>Дубликаты удаляются при сохранении списка</span></div>
  <div class="actions editor-actions">
    <button type="button" class="button" disabled={!editable} onclick={()=>importInput?.click()}>Импорт TXT</button>
    <button type="button" class="button" onclick={exportTxt}>Экспорт TXT</button>
    <button data-rf-editor="save-list" type="button" class="button primary" disabled={!editable} onclick={()=>command('save',domains)}>✓ Сохранить список</button>
    <input type="file" accept=".txt,text/plain" hidden bind:this={importInput} onchange={importTxt}/>
  </div>
  <small class="muted">{hint}</small>
</div>
{/if}
