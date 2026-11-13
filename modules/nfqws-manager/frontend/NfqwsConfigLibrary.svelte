<script>
  import {onMount} from 'svelte';
  import {normalizeConfigName,configLibraryDiff,configFileStats,formatLibraryBytes,requireUnchangedLibraryFile,libraryPostSpec,libraryReadPath,createLibraryRequest} from './NfqwsConfigLibraryModel.js';
  import {highlightLibraryConfig} from './NfqwsConfigSyntax.js';
  let {active=false,openRequest={seq:0,name:''},service={running:false,mutationReady:false},onCount=()=>{},onChanged=()=>{},onNotice=()=>{}}=$props();
  let files=$state([]),neighbors=$state([]),root=$state('/opt/etc/nfqws2/routerforge-configs'),neighborRoot=$state('/opt/etc/nfqws2');
  let busy=$state(false),loaded=$state(false),attempted=$state(false),error=$state(''),search=$state('');
  let selected=$state(''),baseline=$state(null),editor=$state(''),full=$state(false);
  let dialog=$state(null),nameInput=$state(''),dialogError=$state(''),showDiff=$state(false);
  let textarea=$state(null),highlight=$state(null),gutter=$state(null);
  let alive=false,requestSequence=0,handledOpen=0,abort;
  let mounted=$state(false),pendingOpen=$state('');
  const filtered=$derived(files.filter(x=>String(x.name||'').toLowerCase().includes(search.toLowerCase().trim())));
  const neighborFiltered=$derived(neighbors.filter(x=>String(x.name||'').toLowerCase().includes(search.toLowerCase().trim())));
  const dirty=$derived(!!selected&&!!baseline&&editor!==baseline.content);
  const selectedMeta=$derived(files.find(x=>x.name===selected)||baseline);
  const stats=$derived(configFileStats(editor));
  const changes=$derived(configLibraryDiff(baseline?.content??'',editor));
  let api;
  function errorText(e){return String(e?.message||e);}
  function notify(message,type=''){if(alive)onNotice({text:String(message),type});}
  function fail(e){if(alive){error=errorText(e);notify(error,'error');}}
  function isReady(){return alive&&!abort.signal.aborted;}
  async function refresh(){
    if(!isReady()||busy)return;
    busy=true;attempted=true;error='';const seq=++requestSequence;
    try{
      const [a,b]=await Promise.all([api('/configs'),api('/configs/neighbors')]);
      if(!isReady()||seq!==requestSequence)return;
      if(!Array.isArray(a.files)||!Array.isArray(b.files))throw Error('CONFIG_LIBRARY_INCOMPLETE_INVENTORY');
      files=a.files;neighbors=b.files;root=String(a.root||root);neighborRoot=String(b.root||neighborRoot);
      loaded=true;onCount(files.length);
      if(selected&&!files.some(x=>x.name===selected)){
        if(dirty)throw Error('CONFIG_LIBRARY_SELECTED_FILE_REMOVED: несохранённый черновик сохранён в UI');
        clearSelection();
      }
    }catch(e){if(isReady()&&seq===requestSequence)fail(e);}
    finally{if(isReady()&&seq===requestSequence)busy=false;}
  }
  function clearSelection(){selected='';baseline=null;editor='';full=false;}
  async function select(name,{skipPrompt=false}={}){
    if(busy||!isReady()||!name)return;
    if(dirty&&!skipPrompt&&!window.confirm('Есть несохранённые изменения. Переключиться без сохранения?'))return;
    busy=true;error='';const seq=++requestSequence;
    try{
      const data=await api(libraryReadPath(name));
      if(!isReady()||seq!==requestSequence)return;
      if(data.name!==name||typeof data.content!=='string'||!/^[a-f0-9]{64}$/i.test(data.sha256||''))
        throw Error('CONFIG_LIBRARY_INVALID_READ_OR_SHA');
      selected=name;baseline={...data,content:data.content};editor=data.content;
      if(!files.some(x=>x.name===name)){
        const inventory=await api('/configs');
        if(!isReady()||seq!==requestSequence)return;
        if(!Array.isArray(inventory.files))throw Error('CONFIG_LIBRARY_INCOMPLETE_INVENTORY');
        files=inventory.files;
      }
      onCount(files.length);
    }catch(e){if(isReady()&&seq===requestSequence)fail(e);}
    finally{if(isReady()&&seq===requestSequence)busy=false;}
  }
  function openName(mode,name=''){
    if(busy)return;
    dialog={mode,source:name};dialogError='';
    nameInput=mode==='rename'?name.replace(/\.conf$/i,''):
      mode==='duplicate'?name.replace(/\.conf$/i,'')+'-copy':
      mode==='import-neighbor'?name.replace(/\.(conf|cfg)$/i,''):'';
  }
  function closeName(){dialog=null;dialogError='';}
  async function verifyFreshFile(name,sha){
    const live=await api(libraryReadPath(name));
    requireUnchangedLibraryFile({name,sha256:sha},live);
    return live;
  }
  async function execute(kind,fields){
    if(busy||!isReady())return;
    // All POSTs originate in explicit button clicks; never from mount/refresh/effects.
    busy=true;error='';const seq=++requestSequence;
    try{
      if(['save','delete','activate','rename','duplicate'].includes(kind)){
        if(!baseline||fields.name!==baseline.name)throw Error('CONFIG_LIBRARY_NO_VERIFIED_SELECTION');
        await verifyFreshFile(baseline.name,baseline.sha256);
      }
      if(kind==='save'&&(!dirty||editor!==fields.content))throw Error('CONFIG_LIBRARY_STALE_DRAFT');
      if(kind==='activate'){
        if(dirty||!editor.trim())throw Error('Сначала сохрани непустой конфиг');
        if(!service.mutationReady)throw Error('NFQWS runtime не готов к активации');
        if(!window.confirm(`Активировать ${fields.name}? Текущий nfqws2.conf будет сохранён в backup.${service.running?' Сервис будет перезапущен.':' Сервис останется остановленным.'}`))return;
      }
      if(kind==='delete'&&!window.confirm(`Удалить ${fields.name}? Перед удалением будет создан backup.`))return;
      if(kind==='save'&&!window.confirm(`Сохранить изменения в ${fields.name}? Будет создан backup; активный сервис не переключается.`))return;
      const {path,options}=libraryPostSpec(kind,fields);
      const result=await api(path,options);
      if(!isReady()||seq!==requestSequence)return;
      closeName();showDiff=false;
      if(kind==='delete')clearSelection();
      else if(kind==='save')baseline={...baseline,content:editor};
      // A failed refresh after a successful POST must never be presented as a failed mutation.
      onChanged({activated:kind==='activate',kind,success:true});
      notify(kind==='activate'?'Конфиг активирован · '+(result.backup||'backup проверяется'):
        kind==='save'?'Библиотечный конфиг сохранён · backup '+(result.backup||'—'):
        'Операция Config Library выполнена: '+kind,'good');
      // Read back persisted state and authoritative SHA after writes.
      const [a,b]=await Promise.all([api('/configs'),api('/configs/neighbors')]);
      if(!isReady()||seq!==requestSequence)return;
      if(!Array.isArray(a.files)||!Array.isArray(b.files))throw Error('CONFIG_LIBRARY_POST_VERIFY_INVENTORY_FAILED');
      files=a.files;neighbors=b.files;loaded=true;onCount(files.length);
      const target=(kind==='create-empty'||kind==='create-current'||kind==='rename'||kind==='duplicate'||kind==='import-neighbor')?fields.newName||fields.name:
        kind==='delete'?'':selected;
      if(target){
        const next=await api(libraryReadPath(target));
        if(!isReady()||seq!==requestSequence)return;
        if(next.name!==target||typeof next.content!=='string'||!/^[a-f0-9]{64}$/i.test(next.sha256||''))throw Error('CONFIG_LIBRARY_POST_VERIFY_READ_FAILED');
        selected=target;baseline=next;editor=next.content;
      }
    }catch(e){if(isReady()&&seq===requestSequence)fail(e);}
    finally{if(isReady()&&seq===requestSequence)busy=false;}
  }
  function submitName(){
    if(!dialog||busy)return;
    if(dirty){dialogError='Сначала сохрани или отмени несохранённый библиотечный черновик';return;}
    const name=normalizeConfigName(nameInput);
    if(!name){dialogError='Используй буквы, цифры, точку, _ и -';return;}
    const mode=dialog.mode,source=dialog.source;
    if(mode==='rename'||mode==='duplicate'||mode==='import-neighbor'){
      if(mode==='rename'&&name===source){dialogError='Имя не изменилось';return;}
      if(files.some(x=>x.name===name)){dialogError='Такое имя уже существует';return;}
    } else if(files.some(x=>x.name===name)){dialogError='Такое имя уже существует';return;}
    void execute(mode,{name:source||name,newName:name});
  }
  function tabKey(e){
    if(e.key==='Escape'&&full){full=false;return;}
    if(e.key!=='Tab')return;
    e.preventDefault();const el=e.currentTarget,a=el.selectionStart,b=el.selectionEnd;
    editor=editor.slice(0,a)+'  '+editor.slice(b);
    queueMicrotask(()=>{if(el.isConnected){el.focus();el.setSelectionRange(a+2,a+2);}});
  }
  function syncScroll(e){if(highlight){highlight.scrollTop=e.currentTarget.scrollTop;highlight.scrollLeft=e.currentTarget.scrollLeft;}if(gutter)gutter.scrollTop=e.currentTarget.scrollTop;}
  function keyShortcut(e){
    if(!active||!(e.ctrlKey||e.metaKey)||e.key.toLowerCase()!=='s')return;
    e.preventDefault();e.stopPropagation();if(dirty&&!busy)showDiff=true;
  }
  onMount(()=>{
    abort=new AbortController();
    api=createLibraryRequest(window.fetch.bind(window),window.location.pathname,abort.signal);
    alive=true;mounted=true;
    window.addEventListener('keydown',keyShortcut,true);
    return()=>{alive=false;mounted=false;++requestSequence;abort.abort();window.removeEventListener('keydown',keyShortcut,true);if(full){document.body.classList.remove('syntax-fullscreen-open');document.documentElement.classList.remove('syntax-fullscreen-open');}};
  });
  $effect(()=>{if(active&&mounted&&!attempted)void refresh();});
  $effect(()=>{
    const next=openRequest;
    if(!mounted||!active||!next||next.seq===handledOpen)return;
    handledOpen=next.seq;
    if(next.name)pendingOpen=next.name;
  });
  $effect(()=>{
    if(!mounted||!active||!loaded||busy||!pendingOpen)return;
    const name=pendingOpen;pendingOpen='';void select(name);
  });
  function toggleFull(){
    full=!full;
    if(full){document.body.classList.add('syntax-fullscreen-open');document.documentElement.classList.add('syntax-fullscreen-open');}
    else if(!document.querySelector('.full-editor:not(#storedConfigEditorShell),.syntax-fullscreen,.script-fullscreen')){
      document.body.classList.remove('syntax-fullscreen-open');document.documentElement.classList.remove('syntax-fullscreen-open');
    }
  }

</script>
<div class="config-library-grid" data-svelte-owned="config-library">
  <article class="panel">
    <div class="panel-head"><div><h2>Полные конфиги</h2><div id="configLibraryRoot" class="section-kicker mono">{root}</div></div></div>
    <div class="setup-note config-library-note">Библиотечные <span class="mono">nfqws2.conf</span> не меняют сервис до явной активации. Запись выполняет backend с backup.</div>
    <div class="toolbar"><button id="newStoredConfig" class="primary" type="button" disabled={busy} onclick={()=>openName('create-empty')}>+ Пустой</button><button id="saveCurrentConfig" type="button" disabled={busy} onclick={()=>openName('create-current')}>Сохранить текущий как…</button></div>
    <input id="configLibrarySearch" class="search" style="width:100%;max-width:none;margin-top:9px" placeholder="Поиск конфига…" aria-label="Поиск конфига" bind:value={search}>
    <div id="configLibraryList" class="config-library-list">
      {#each filtered as item (item.name)}
        <button type="button" style="width:100%;text-align:left;color:inherit;font:inherit" class={`config-library-item${item.active?' active':''}${selected===item.name?' selected':''}`} disabled={busy} onclick={()=>select(item.name)} data-config-name={item.name}>
          <div style="min-width:0"><div class="config-library-name">{item.name}</div><div class="config-library-sub">{formatLibraryBytes(item.size)} · {String(item.sha256||'').slice(0,12)}…</div></div>
          <div class="config-library-badges">{#if item.active}<span class="config-badge active">ACTIVE</span>{/if}{#if item.empty}<span class="config-badge empty">EMPTY</span>{/if}</div>
        </button>
      {:else}<div class="empty">{error||(!loaded?'Загрузка…':'Полных конфигов пока нет.')}</div>{/each}
    </div>
    <div class="config-neighbor-head"><div><div class="config-neighbor-title">Конфиги рядом с nfqws2.conf</div><div id="neighborConfigRoot" class="section-kicker mono">{neighborRoot}</div></div><button type="button" disabled={busy} onclick={refresh} id="refreshNeighborConfigs">Обновить</button></div>
    <div id="neighborConfigList" class="config-neighbor-list">
      {#each neighborFiltered as item (item.name)}
        <div class="config-neighbor-item"><div style="min-width:0"><div class="config-library-name">{item.name}</div><div class="config-library-sub">{formatLibraryBytes(item.size)} · {String(item.sha256||'').slice(0,12)}…</div></div><button type="button" disabled={busy} onclick={()=>openName('import-neighbor',item.name)}>В библиотеку</button></div>
      {:else}<div class="config-neighbor-empty">Подходящих .conf/.cfg рядом с основным не найдено.</div>{/each}
    </div>
  </article>
  <article class="panel">
    <div class="panel-head"><div><h2 id="storedConfigTitle">{selected||'Выберите конфиг'}</h2><div id="storedConfigMeta" class="section-kicker mono">{baseline?`${baseline.path||selected} · ${formatLibraryBytes(baseline.size)} · SHA ${String(baseline.sha256||'').slice(0,16)}…${selectedMeta?.active?' · ACTIVE':''}`:'—'}</div></div>
      <div class="toolbar">
        <button id="renameStoredConfig" type="button" disabled={busy||!selected} onclick={()=>openName('rename',selected)}>Переименовать</button>
        <button id="duplicateStoredConfig" type="button" disabled={busy||!selected} onclick={()=>openName('duplicate',selected)}>Копия</button>
        <button id="deleteStoredConfig" class="danger" type="button" disabled={busy||!selected} onclick={()=>execute('delete',{name:selected})}>Удалить</button>
        <button id="undoStoredConfig" type="button" disabled={busy||!dirty} onclick={()=>editor=baseline.content}>Отменить</button>
        <button id="diffStoredConfig" type="button" disabled={busy||!dirty} onclick={()=>showDiff=true}>Diff</button>
        <button id="saveStoredConfig" type="button" disabled={busy||!dirty} onclick={()=>showDiff=true}>Сохранить</button>
        <button id="activateStoredConfig" class="primary" type="button" disabled={busy||!selected||dirty||!editor.trim()||!service.mutationReady} onclick={()=>execute('activate',{name:selected})}>Активировать</button>
      </div>
    </div>
    {#if !selected}<div id="configLibraryEditorEmpty" class="config-library-empty">Создай пустой конфиг, сохрани текущий как отдельный файл или скопируй найденный рядом с основным.</div>{/if}
    {#if selected}
      <div id="storedConfigEditorShell" class="editor-shell" class:full-editor={full}>
        <div class="editor-toolbar"><span id="storedConfigState" class:dirty={dirty} class="mono">{dirty?'UNSAVED':selectedMeta?.active?'ACTIVE':selectedMeta?.empty?'EMPTY':'CLEAN'}</span><div class="editor-toolbar-actions"><span>Ctrl+S → Diff · Tab = 2 spaces</span><button id="fullscreenStoredConfig" type="button" onclick={toggleFull}>{full?'Закрыть fullscreen':'На весь экран'}</button></div></div>
        <div class="editor-main"><pre id="storedConfigGutter" class="gutter" bind:this={gutter}>{Array.from({length:Math.min(stats.lines,10000)},(_,i)=>i+1).join('\n')}</pre>
          <div class="syntax-editor-body"><pre class="syntax-editor-highlight" bind:this={highlight} aria-hidden="true">{@html highlightLibraryConfig(editor)}</pre><textarea id="storedConfigEditor" class="editor" bind:this={textarea} bind:value={editor} disabled={busy} spellcheck="false" placeholder="Вставь сюда полный nfqws2.conf" style:height={full?'100%':`${Math.max(420,Math.min(1400,stats.lines*20))}px`} style:overflowY={stats.lines>60?'auto':'hidden'} onkeydown={tabKey} onscroll={syncScroll}></textarea></div>
        </div>
        <div class="editor-foot"><span id="storedConfigStats">{stats.lines} lines · {formatLibraryBytes(stats.bytes)}</span><span>library save ≠ activation · activate → backup → atomic switch → restart/verify</span></div>
      </div>
    {/if}
  </article>
</div>
{#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
{#if dialog}
  <div id="configNameModal" class="modal open" role="presentation">
    <form class="dialog small-dialog" onsubmit={e=>{e.preventDefault();submitName();}}>
      <div class="dialog-head"><strong id="configNameTitle">{dialog.mode==='create-current'?'Сохранить текущий как':dialog.mode==='create-empty'?'Новый пустой конфиг':dialog.mode==='rename'?'Переименовать конфиг':dialog.mode==='import-neighbor'?`Скопировать ${dialog.source} в библиотеку`:'Создать копию'}</strong><button id="closeConfigName" type="button" disabled={busy} onclick={closeName}>Закрыть</button></div>
      <div class="dialog-body"><label for="configNameInput" class="muted small">Имя файла</label><div style="display:flex;align-items:center;gap:7px;margin-top:6px"><input id="configNameInput" class="search" style="flex:1;max-width:none" bind:value={nameInput} placeholder="youtube" disabled={busy}><span class="mono muted">.conf</span></div><div id="configNameError" class="bad-text small" role="alert">{dialogError}</div></div>
      <div class="dialog-foot"><button id="configNameCancel" type="button" disabled={busy} onclick={closeName}>Отмена</button><button id="configNameConfirm" class="primary" type="submit" disabled={busy}>Продолжить</button></div>
    </form>
  </div>
{/if}
{#if showDiff&&dirty}
  <div id="storedConfigDiffModal" class="modal open" role="presentation">
    <div class="dialog"><div class="dialog-head"><strong>Изменения библиотечного конфига</strong><button type="button" disabled={busy} onclick={()=>showDiff=false}>Закрыть</button></div>
      <pre class="diff">{#each changes as line,i (i)}<span class={line.type}>{line.text}{'\n'}</span>{/each}</pre>
      <div class="dialog-foot"><button type="button" disabled={busy} onclick={()=>showDiff=false}>Отмена</button><button type="button" class="primary" disabled={busy||!dirty} onclick={()=>execute('save',{name:selected,content:editor})}>Сохранить в библиотеку</button></div>
    </div>
  </div>
{/if}
