<script>
  import {workspaceHeight} from './NfqwsUIActions.js';
  import {onMount,tick} from 'svelte';
  import {safeListName,newListName,renameListName,listGroup,sortedListInventory,groupListInventory,normalizeListEntries,listStatistics,listHTMLHighlight,validLoadedList,listMutation,checkedRenamePreview,listRenameApply,listSafeAPIPath} from './NfqwsListEditorModel.js';
  let {active=false,inventory=[],serviceBusy=false,tools={},onRequest=async()=>{},onStatus=()=>{},onNotice=()=>{},onDraft=()=>{},onRegister=()=>{},onTool=()=>{}}=$props();
  let search=$state(''),collapsed=$state({system:false,user:false,opkg:false});
  let selected=$state(''),baseline=$state(null),original=$state(''),value=$state('');
  let busy=$state(false),loading=$state(false),error=$state(''),full=$state(false);
  let createOpen=$state(false),stem=$state(''),createError=$state('');
  let renameOpen=$state(false),renamePlan=$state(null),renameText=$state(''),renameError=$state('');
  let editor=$state(null),gutter=$state(null),highlight=$state(null);
  let mounted=$state(false),alive=false,sequence=0;
  const available=$derived(sortedListInventory(inventory));
  const groups=$derived(groupListInventory(inventory,search));
  const dirty=$derived(!!baseline&&value!==original);
  const stats=$derived(listStatistics(value));
  const ready=$derived(!!baseline&&!busy&&!serviceBusy);
  const canWrite=$derived(ready);
  const canManage=$derived(ready&&!baseline?.protected);
  function publish(){if(alive)onDraft({selected,meta:baseline,original,value,dirty:!!baseline&&value!==original});}
  function notice(text,type=''){if(alive)onNotice({text:String(text),type});}
  function fail(e){if(!alive)return;error=String(e?.message||e);notice(error,'error');}
  function readContext(){
    const position=Math.max(0,Math.min(Number(editor?.selectionStart||0),value.length));
    return {selected,meta:baseline,original,value,dirty:!!baseline&&value!==original,
      caretLine:value.slice(0,position).split(/\r?\n/).length};
  }
  async function openAtLine(name,line){
    if(!alive||!safeListName(name)||!Number.isSafeInteger(line)||line<1)return false;
    if(selected!==name&&!(await select(name)))return false;
    await tick();
    if(!alive||selected!==name||!baseline||!editor?.isConnected)return false;
    const rows=value.split(/\r?\n/),target=Math.min(line,Math.max(1,rows.length));
    let start=0;for(let i=0;i<target-1;i++)start+=rows[i].length+(value.includes('\r\n')?2:1);
    editor.focus();editor.setSelectionRange(start,start+(rows[target-1]?.length||0));
    const lineHeight=parseFloat(getComputedStyle(editor).lineHeight)||16.5;
    editor.scrollTop=Math.max(0,(target-3)*lineHeight);onTool('cursor');return true;
  }
  async function request(path,body){const method=body===undefined?'GET':'POST';listSafeAPIPath(path,method);return onRequest(path,body===undefined?{}:{method,body:JSON.stringify(body)});}
  async function select(name,skipPrompt=false){
    if(!alive||busy||!safeListName(name))return false;
    if(dirty&&!skipPrompt&&!window.confirm(`Несохранённые изменения в ${selected}. Перейти к ${name} без сохранения?`))return false;
    const seq=++sequence;busy=true;loading=true;error='';selected=name;baseline=null;original='';value='';renamePlan=null;renameOpen=false;publish();
    collapsed={...collapsed,[listGroup({name})]:false};storeGroups();
    try{
      const response=validLoadedList(name,await request('/list?name='+encodeURIComponent(name)));
      if(!alive||seq!==sequence)return false;
      baseline=response;original=response.content;value=response.content;publish();await tick();return true;
    }catch(e){if(alive&&seq===sequence)fail(e);return false;}
    finally{if(alive&&seq===sequence){busy=false;loading=false;}}
  }
  function edit(e){value=e.currentTarget.value;renamePlan=null;publish();onTool('cursor');}
  function normalize(){if(!ready)return;value=normalizeListEntries(value);renamePlan=null;publish();}
  function revert(){if(!baseline)return;value=original;renamePlan=null;publish();}
  async function save(){
    if(!active||!canWrite||!dirty)return;
    const ref={...baseline},name=selected,sent=value;
    if(!window.confirm(`Сохранить ${name}? Перед записью будет создан safety backup.`))return;
    busy=true;error='';const seq=++sequence;
    try{
      const cmd=listMutation('save',ref,sent),result=await request(cmd.path,cmd.body);
      if(!alive||seq!==sequence)return;
      if(!result?.status)throw Error('NFQWS_LIST_SAVE_MISSING_AUTHORITATIVE_STATUS');
      onStatus(result.status);
      await selectAfterMutation(name);
      notice(`Список ${name} сохранён. Backup: ${String(result.backup||'—')}`,'good');
    }catch(e){if(alive&&seq===sequence)fail(e);}
    finally{if(alive&&seq===sequence)busy=false;}
  }
  async function selectAfterMutation(name){
    const seq=sequence;const response=validLoadedList(name,await request('/list?name='+encodeURIComponent(name)));
    if(!alive||seq!==sequence)return;
    selected=name;baseline=response;original=response.content;value=response.content;renamePlan=null;publish();await tick();
  }
  async function remove(){
    if(!active||!canManage||dirty)return;
    const name=selected,reference={...baseline};
    if(!window.confirm(`Удалить ${name}? Перед удалением будет создан backup.`))return;
    busy=true;error='';const seq=++sequence;
    try{
      const cmd=listMutation('delete',reference),result=await request(cmd.path,cmd.body);
      if(!alive||seq!==sequence)return;
      if(!result?.status)throw Error('NFQWS_LIST_DELETE_MISSING_STATUS');
      selected='';baseline=null;original='';value='';publish();onStatus(result.status);
      notice(`Список удалён. Backup: ${String(result.backup||'—')}`,'good');
    }catch(e){if(alive&&seq===sequence)fail(e);}
    finally{if(alive&&seq===sequence)busy=false;}
  }
  async function create(){
    if(!active||busy||serviceBusy)return;
    createError='';if(dirty&&!window.confirm(`Несохранённые изменения в ${selected}. Создать новый список и отбросить черновик?`))return;let name,cmd;
    try{name=newListName(stem);cmd=listMutation('create',null,stem);if(available.some(item=>item.name===name))throw Error('Такой список уже существует');}catch(e){createError=String(e.message||e);return;}
    busy=true;const seq=++sequence;
    try{
      const result=await request(cmd.path,cmd.body);
      if(!alive||seq!==sequence)return;
      if(!result?.status)throw Error('NFQWS_LIST_CREATE_MISSING_STATUS');
      createOpen=false;onStatus(result.status);await selectAfterMutation(name);
      notice(`Создан ${name}`,'good');
    }catch(e){if(alive&&seq===sequence)createError=String(e.message||e);}
    finally{if(alive&&seq===sequence)busy=false;}
  }
  async function renamePreview(){
    if(!active||!canManage||dirty)return;
    const initial=selected.replace(/\.list$/i,'')+'-renamed.list',answer=window.prompt('Новое имя списка:',initial);
    if(answer===null)return;
    let next;try{next=renameListName(answer);}catch(e){fail(e);return;}
    renamePlan=null;renameText='Проверяю список и связанные конфигурации…';renameError='';renameOpen=true;busy=true;
    const ref={...baseline},seq=++sequence;
    try{
      const query=new URLSearchParams({old_name:ref.name,new_name:next,expected_sha256:ref.sha256});
      const response=await request('/list/rename-preview?'+query.toString());
      if(!alive||seq!==sequence)return;
      const verified=checkedRenamePreview(ref,next,response);
      if(selected!==ref.name||baseline?.sha256!==ref.sha256||dirty)throw Error('NFQWS_LIST_RENAME_PREVIEW_STALE');
      renamePlan=verified;
      const impacts=verified.config_changes.map(x=>`${x.reference} · селекторов: ${x.selectors}\n  before ${x.before_sha256}\n  after  ${x.after_sha256}`);
      renameText=[`Список: ${verified.old_name} → ${verified.new_name}`,`SHA списка: ${verified.old_list_sha256}`,`Новое имя свободно: ДА`,`Ссылок для замены: ${verified.reference_count}`,`Затронутых конфигов: ${verified.config_changes.length}`,'',...(impacts.length?impacts:['Нет ссылок в конфигурациях.']),'','PREVIEW ONLY — ничего не записано.'].join('\n');
    }catch(e){if(alive&&seq===sequence){renameError=String(e.message||e);renameText='Preview заблокирован: '+renameError;}}
    finally{if(alive&&seq===sequence)busy=false;}
  }
  function closeRename(){renameOpen=false;renamePlan=null;renameText='';renameError='';}
  async function renameApply(){
    if(!active||!canManage||dirty||!renamePlan)return;
    const ref={...baseline},plan=renamePlan;let cmd;
    try{cmd=listRenameApply(plan,ref);}catch(e){fail(e);closeRename();return;}
    if(!window.confirm(`Переименовать ${plan.old_name} → ${plan.new_name}?\nСелекторов: ${plan.reference_count}; конфигов: ${plan.config_changes.length}.\nПеред записью backup, при ошибке rollback.`))return;
    busy=true;const seq=++sequence;renamePlan=null;
    try{
      const result=await request(cmd.path,cmd.body);
      if(!alive||seq!==sequence)return;
      if(!result?.ok||result.rolled_back)throw Error('NFQWS_LIST_RENAME_APPLY_UNCONFIRMED');
      const status=result.status||await request('/status');
      if(!status)throw Error('NFQWS_LIST_RENAME_MISSING_STATUS');
      onStatus(status);await selectAfterMutation(plan.new_name);closeRename();
      notice(`Список ${plan.old_name} → ${plan.new_name}. Backup: ${Array.isArray(result.backups)?result.backups.length:0}`,'good');
    }catch(e){if(alive&&seq===sequence){renameError=String(e.message||e);renameText='Операция заблокирована или откатилась: '+renameError+'\nОткрой Preview заново.';}}
    finally{if(alive&&seq===sequence)busy=false;}
  }
  function scrollEditor(e){if(gutter)gutter.scrollTop=e.currentTarget.scrollTop;if(highlight){highlight.scrollTop=e.currentTarget.scrollTop;highlight.scrollLeft=e.currentTarget.scrollLeft;}}
  function keyEditor(e){
    if(e.key==='Tab'){e.preventDefault();const a=e.currentTarget.selectionStart,b=e.currentTarget.selectionEnd;value=value.slice(0,a)+'  '+value.slice(b);publish();queueMicrotask(()=>{if(editor?.isConnected){editor.focus();editor.setSelectionRange(a+2,a+2);}});}
    if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'){e.preventDefault();e.stopPropagation();void save();}
    if(e.key==='Escape'&&full)toggleFull();
  }
  function shortcut(e){if(!active||!(e.ctrlKey||e.metaKey)||String(e.key||'').toLowerCase()!=='s')return;
    if(!document.getElementById('view-lists')?.classList.contains('active'))return;
    e.preventDefault();e.stopPropagation();if(dirty)void save();}
  function toggleFull(){full=!full;if(full){document.body.classList.add('syntax-fullscreen-open');document.documentElement.classList.add('syntax-fullscreen-open');}else{document.body.classList.remove('syntax-fullscreen-open');document.documentElement.classList.remove('syntax-fullscreen-open');}}
  function storeGroups(){try{localStorage.setItem('routerforge.nfqws.list-groups.v1',JSON.stringify(collapsed));}catch{}}
  function flipGroup(key){collapsed={...collapsed,[key]:!collapsed[key]};storeGroups();}
  onMount(()=>{
    alive=true;mounted=true;
    try{const state=JSON.parse(localStorage.getItem('routerforge.nfqws.list-groups.v1')||'{}');collapsed={...collapsed,...Object.fromEntries(['system','user','opkg'].filter(key=>typeof state[key]==='boolean').map(key=>[key,state[key]]))};}catch{}
    onRegister({select,readContext,openAtLine});window.addEventListener('keydown',shortcut,true);publish();
    return()=>{alive=false;++sequence;onRegister(null);window.removeEventListener('keydown',shortcut,true);if(full){document.body.classList.remove('syntax-fullscreen-open');document.documentElement.classList.remove('syntax-fullscreen-open');}};
  });
  $effect(()=>{if(mounted&&available.length&&!selected&&!busy)void select(available[0].name,true);});
</script>
<div id="listWorkspace" class="list-grid rf-workspace" use:workspaceHeight data-svelte-owned="nfqws-list-editor">
  <article class="panel">
    <div class="panel-head"><div><h2>Списки</h2><div class="section-kicker">Системные · Пользовательские · OPKG</div></div><button id="newList" class="primary" type="button" disabled={busy||serviceBusy} onclick={()=>{stem='';createError='';createOpen=true;}}>+ Новый</button></div>
    <input id="listSearch" class="search" placeholder="Поиск списка…" aria-label="Поиск списка" bind:value={search}>
    <div id="fileList" class="file-list">
      {#each groups as group (group.key)}
        <section class={`file-group ${group.key}${!search&&collapsed[group.key]?' collapsed':''}`}>
          <button type="button" class="file-group-header" onclick={()=>flipGroup(group.key)} aria-expanded={!collapsed[group.key]}><span class="file-group-header-main"><span class="file-group-chevron">▼</span><span class="file-group-title">{group.title}</span></span><span class="file-group-count">{group.items.length}</span></button>
          {#if search||!collapsed[group.key]}<div class="file-group-items">
            {#each group.items as item (item.name)}
              <button type="button" class={`file-item${selected===item.name?' active':''}`} data-group={group.key} disabled={busy||serviceBusy} onclick={()=>select(item.name)} style="width:100%;text-align:left;color:inherit;font:inherit">
                <div><div class="file-name"><span class="file-icon">{group.key==='system'?'◆':group.key==='opkg'?'◇':'≡'}</span>{item.name}</div><div class="file-meta">{item.modified_at||''} · {group.key==='system'?'служебный / upstream':group.key==='opkg'?'package copy':'пользовательский'}{item.name.toLowerCase().endsWith('.list-old')?' · old':''}</div></div>
                <div class="file-meta">{Number.isFinite(Number(item.size))?`${Math.max(0,Number(item.size))} B`:'—'}</div>
              </button>
            {/each}
          </div>{/if}
        </section>
      {:else}<div class="empty">Списки не найдены.</div>{/each}
    </div>
  </article>
  <article id="listEditorPanel" class="panel" class:full-editor={full}>
    <div class="panel-head"><div><h2 id="selectedListTitle">{selected||'Выберите список'}</h2><div id="selectedListMeta" class="section-kicker mono">{loading?'Loading…':baseline?`${baseline.path||selected} · ${stats.bytes} B${baseline.protected?' · protected':''}`:'—'}</div></div>
      <div class="toolbar">
        <button id="inspectListEntry" type="button" disabled={!ready||!tools.inspect} onclick={()=>onTool('inspect')}>Инспектор</button>
        <button id="staticPathEntry" type="button" disabled={!ready||!tools.path} onclick={()=>onTool('path')}>Путь</button>
        <button id="analyzeLists" type="button" disabled={!ready||!tools.analysis} onclick={()=>onTool('analysis')}>Пересечения</button>
        <button id="checkList" type="button" disabled={!ready} onclick={()=>onTool('check')}>Проверить</button>
        <button id="normalizeList" type="button" disabled={!ready} onclick={normalize}>Очистить дубли</button>
        <button id="fullscreenList" type="button" disabled={!baseline} onclick={toggleFull}>{full?'Закрыть fullscreen':'На весь экран'}</button>
        <button id="undoList" type="button" disabled={!dirty||busy} onclick={revert}>Отменить</button>
        <button id="saveList" class="primary" type="button" disabled={!canWrite||!dirty} onclick={save}>Сохранить</button>
        <button id="previewRenameList" type="button" disabled={!canManage||dirty} onclick={renamePreview}>Переименовать · Preview</button>
        <button id="deleteList" class="danger" type="button" disabled={!canManage||dirty} onclick={remove}>Удалить</button>
      </div>
    </div>
    <div class="list-editor-main"><pre id="listGutter" class="list-line-gutter" aria-hidden="true" bind:this={gutter}>{Array.from({length:Math.min(stats.lines,10000)},(_,i)=>i+1).join('\n')}</pre>
      <div class="list-editor-body"><pre id="listHighlight" class="list-editor-highlight" aria-hidden="true" bind:this={highlight}>{@html listHTMLHighlight(value)}</pre>
        <textarea id="listEditor" class="editor list-editor" spellcheck="false" placeholder="Выберите список слева." bind:this={editor} {value} disabled={!baseline||busy||serviceBusy} oninput={edit} onclick={()=>onTool('cursor')} onkeyup={()=>onTool('cursor')} onscroll={scrollEditor} onkeydown={keyEditor}></textarea>
      </div>
    </div>
    <div class="editor-foot"><div id="listStats" class="list-stats">{baseline?`${stats.lines} lines · ${stats.entries} entries · ${stats.bytes} B${dirty?' · UNSAVED':''}`:'—'}</div><span>save → safety backup → atomic write</span></div>
    {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  </article>
</div>
{#if createOpen}
  <div id="createModal" class="modal open" role="presentation"><form class="dialog small-dialog" onsubmit={e=>{e.preventDefault();void create();}}>
    <div class="dialog-head"><strong>Новый список</strong><button id="closeCreate" type="button" disabled={busy} onclick={()=>createOpen=false}>Закрыть</button></div>
    <div class="dialog-body"><label for="newListName" class="muted small">Имя файла</label><div style="display:flex;align-items:center;gap:7px;margin-top:6px"><input id="newListName" class="search" style="flex:1;max-width:none" bind:value={stem} placeholder="youtube" disabled={busy}><span class="mono muted">.list</span></div><div id="createError" class="bad-text small" role="alert">{createError}</div></div>
    <div class="dialog-foot"><button id="createCancel" type="button" disabled={busy} onclick={()=>createOpen=false}>Отмена</button><button id="createConfirm" class="primary" type="submit" disabled={busy}>Создать</button></div>
  </form></div>
{/if}
{#if renameOpen}
  <div id="renamePreviewModal" class="modal open" role="presentation"><div class="dialog">
    <div class="dialog-head"><div><strong>Переименование списка · Preview</strong><div class="section-kicker">Только просмотр · без изменения файлов</div></div><button id="closeRenamePreview" type="button" disabled={busy} onclick={closeRename}>Закрыть</button></div>
    <div class="dialog-body"><pre id="renamePreviewBody" class="diff">{renameText}</pre>{#if renameError}<div class="bad-text" role="alert">{renameError}</div>{/if}</div>
    <div class="dialog-foot"><span class="muted small">Запись только после явного подтверждения и проверки SHA.</span><button id="applyRenameList" class="primary" type="button" disabled={busy||!renamePlan||dirty} onclick={renameApply}>Применить</button><button id="renamePreviewClose" type="button" disabled={busy} onclick={closeRename}>Закрыть</button></div>
  </div></div>
{/if}
