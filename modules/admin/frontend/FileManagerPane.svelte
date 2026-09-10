<script>
  import { onMount } from 'svelte';
  import {
    adminFileChmod, adminFileCopy, adminFileDelete, adminFileDownloadURL,
    adminFileMkdir, adminFileMove, adminFileWrite, getAdminFiles, readAdminFile
  } from '$lib/api.js';
  import { bytes } from '$lib/utils.js';
  import FileEditorDrawer from './FileEditorDrawer.svelte';

  export let locale = 'ru';

  const VIEW_KEY='routerforge.admin.files.view';
  const WRITE_LIMIT=128*1024;
  let view='commander';
  let busy=false;
  let errorText='';
  let actionText='';

  let left={path:'/opt',entries:[],selected:null};
  let right={path:'/tmp',entries:[],selected:null};
  let active='left';

  let explorerPath='/opt';
  let explorerEntries=[];
  let explorerSearch='';

  let selectedFile=null;
  let editorContent='';
  let editorOriginal='';
  let editorBusy=false;

  $: editorDirty=!!selectedFile&&editorContent!==editorOriginal;
  $: editorReadOnly=!!selectedFile&&Number(selectedFile.size||0)>WRITE_LIMIT;
  $: explorerFiltered=explorerEntries.filter((entry)=>!explorerSearch.trim()||`${entry.name} ${entry.path}`.toLowerCase().includes(explorerSearch.trim().toLowerCase()));

  $: copy=locale==='ru'?{
    commander:'Commander',explorer:'Explorer',refresh:'Обновить',up:'Вверх',
    folder:'Папка',file:'Файл',copy:'Копировать',move:'Переместить',rename:'Переименовать',
    remove:'Удалить',chmod:'Права',download:'Скачать',actions:'Действия',name:'Имя',
    size:'Размер',modified:'Изменён',mode:'Права',search:'Поиск в папке…',
    regularOnly:'Копирование между панелями пока поддерживает обычные файлы; каталоги перемещаются.',
    noSelection:'Сначала выбери объект в активной панели.',overwrite:'Файл назначения уже существует.',
    dirty:'Есть несохранённые изменения',save:'Сохранить',close:'Закрыть'
  }:{
    commander:'Commander',explorer:'Explorer',refresh:'Refresh',up:'Up',
    folder:'Folder',file:'File',copy:'Copy',move:'Move',rename:'Rename',
    remove:'Delete',chmod:'Mode',download:'Download',actions:'Actions',name:'Name',
    size:'Size',modified:'Modified',mode:'Mode',search:'Search in folder…',
    regularOnly:'Cross-panel copy currently supports regular files; directories can be moved.',
    noSelection:'Select an item in the active panel first.',overwrite:'Destination already exists.',
    dirty:'Unsaved changes',save:'Save',close:'Close'
  };

  const quickRoots=[
    ['Entware /opt','/opt'],['Entware /opt/etc','/opt/etc'],['Entware /opt/var','/opt/var'],
    ['Binaries','/opt/bin'],['Logs','/opt/var/log'],['AWG Manager','/opt/etc/awg-manager'],['Temp','/tmp']
  ];

  function err(error){return error?.payload?.error||error?.message||String(error);}
  function join(parent,name){return `${parent.replace(/\/+$/,'')}/${name}`||`/${name}`;}
  function parent(path){
    const clean=path.replace(/\/+$/,'');
    if(clean==='/opt'||clean==='/tmp') return clean;
    const pos=clean.lastIndexOf('/');
    const value=pos<=0?clean:clean.slice(0,pos);
    if(clean.startsWith('/opt/')&&!value.startsWith('/opt')) return '/opt';
    if(clean.startsWith('/tmp/')&&!value.startsWith('/tmp')) return '/tmp';
    return value;
  }
  function base(path){const clean=path.replace(/\/+$/,'');return clean.slice(clean.lastIndexOf('/')+1);}

  function setView(next){
    view=next;
    localStorage.setItem(VIEW_KEY,next);
    if(next==='explorer') loadExplorer(explorerPath);
  }
  function setAction(value){actionText=value;setTimeout(()=>{if(actionText===value)actionText='';},3500);}

  async function loadPanel(side,path){
    busy=true;errorText='';
    try{
      const result=await getAdminFiles(path);
      const next={path:result.path||path,entries:result.entries||[],selected:null};
      if(side==='left') left=next; else right=next;
    }catch(error){errorText=err(error);}finally{busy=false;}
  }

  async function loadExplorer(path=explorerPath){
    busy=true;errorText='';
    try{
      const result=await getAdminFiles(path);
      explorerPath=result.path||path; explorerEntries=result.entries||[];
    }catch(error){errorText=err(error);}finally{busy=false;}
  }

  function select(side,entry){
    active=side;
    if(side==='left') left={...left,selected:entry}; else right={...right,selected:entry};
  }

  async function open(side,entry){
    if(entry.kind==='directory'){
      if(side==='explorer') return loadExplorer(entry.path);
      return loadPanel(side,entry.path);
    }
    if(entry.kind!=='file') return;
    editorBusy=true;errorText='';
    try{
      const result=await readAdminFile(entry.path);
      selectedFile=result;editorContent=result.content||'';editorOriginal=editorContent;
    }catch(error){errorText=err(error);}finally{editorBusy=false;}
  }

  async function saveEditor(){
    if(!selectedFile||!editorDirty||editorReadOnly)return;
    editorBusy=true;errorText='';
    try{
      const result=await adminFileWrite({
        path:selectedFile.path,confirm_path:selectedFile.path,content:editorContent,create:false,
        expected_size:selectedFile.size,expected_mtime_ns:selectedFile.mtime_ns
      });
      const fresh=await readAdminFile(result.path||selectedFile.path);
      selectedFile=fresh;editorContent=fresh.content||'';editorOriginal=editorContent;
      setAction(`${copy.save}: ${fresh.path}`);
      await refreshAll();
    }catch(error){errorText=err(error);}finally{editorBusy=false;}
  }

  async function createFolder(path,reload){
    const name=prompt(copy.folder);if(!name)return;
    const target=join(path,name);
    try{await adminFileMkdir({path:target,confirm_path:target});setAction(`${copy.folder}: ${target}`);await reload();}
    catch(error){errorText=err(error);}
  }
  async function createFile(path,reload){
    const name=prompt(copy.file);if(!name)return;
    const target=join(path,name);
    try{
      await adminFileWrite({path:target,confirm_path:target,content:'',create:true});
      setAction(`${copy.file}: ${target}`);await reload();
    }catch(error){errorText=err(error);}
  }

  async function renameEntry(entry,reload){
    const destination=prompt(copy.rename,entry.path);if(!destination||destination===entry.path)return;
    if(!confirm(`${entry.path}\n→ ${destination}?`))return;
    try{
      await adminFileMove({
        source:entry.path,destination,confirm_source:entry.path,confirm_destination:destination,
        expected_size:entry.size,expected_mtime_ns:entry.mtime_ns
      });
      setAction(`${copy.rename}: ${destination}`);await reload();
    }catch(error){errorText=err(error);}
  }

  async function deleteEntry(entry,reload){
    if(!confirm(`${copy.remove}: ${entry.path}?`))return;
    try{
      await adminFileDelete({path:entry.path,confirm_path:entry.path,expected_size:entry.size,expected_mtime_ns:entry.mtime_ns});
      setAction(`${copy.remove}: ${entry.path}`);await reload();
    }catch(error){errorText=err(error);}
  }

  async function chmodEntry(entry,reload){
    const mode=prompt(copy.chmod,entry.kind==='directory'?'0755':'0644');if(!mode)return;
    try{
      await adminFileChmod({path:entry.path,confirm_path:entry.path,mode,expected_size:entry.size,expected_mtime_ns:entry.mtime_ns});
      setAction(`${copy.chmod}: ${entry.path}`);await reload();
    }catch(error){errorText=err(error);}
  }

  async function transfer(kind){
    const sourcePanel=active==='left'?left:right;
    const destinationPanel=active==='left'?right:left;
    const entry=sourcePanel.selected;
    if(!entry){errorText=copy.noSelection;return;}
    const destination=join(destinationPanel.path,entry.name);
    if(kind==='copy'&&entry.kind!=='file'){errorText=copy.regularOnly;return;}
    if(!confirm(`${copy[kind]}:\n${entry.path}\n→ ${destination}?`))return;
    try{
      if(kind==='copy'){
        await adminFileCopy({
          source:entry.path,destination,confirm_source:entry.path,confirm_destination:destination,
          expected_size:entry.size,expected_mtime_ns:entry.mtime_ns
        });
      }else{
        await adminFileMove({
          source:entry.path,destination,confirm_source:entry.path,confirm_destination:destination,
          expected_size:entry.size,expected_mtime_ns:entry.mtime_ns
        });
      }
      setAction(`${copy[kind]}: ${destination}`);
      await Promise.all([loadPanel('left',left.path),loadPanel('right',right.path)]);
    }catch(error){errorText=err(error);}
  }

  async function refreshAll(){
    if(view==='commander')await Promise.all([loadPanel('left',left.path),loadPanel('right',right.path)]);
    else await loadExplorer(explorerPath);
  }

  onMount(()=>{
    const stored=localStorage.getItem(VIEW_KEY);
    if(stored==='explorer'||stored==='commander')view=stored;
    loadPanel('left',left.path);loadPanel('right',right.path);
    if(view==='explorer')loadExplorer(explorerPath);
  });
</script>

<section class="fm-shell">
  <div class="fm-toolbar">
    <div class="view-toggle">
      <button class:active={view==='commander'} onclick={()=>setView('commander')}>◫ {copy.commander}</button>
      <button class:active={view==='explorer'} onclick={()=>setView('explorer')}>☷ {copy.explorer}</button>
    </div>
    <button onclick={refreshAll} disabled={busy}>↻ {copy.refresh}</button>
    {#if view==='commander'}
      <button onclick={()=>transfer('copy')}>{copy.copy}</button>
      <button onclick={()=>transfer('move')}>{copy.move}</button>
    {/if}
    <span class="spacer"></span>
    {#if view==='explorer'}<input class="fm-search" bind:value={explorerSearch} placeholder={copy.search}/>{/if}
  </div>

  {#if actionText}<div class="fm-ok">{actionText}</div>{/if}
  {#if errorText}<div class="fm-error">{errorText}</div>{/if}

  {#if view==='commander'}
    <div class="commander-grid">
      {#each [['left',left],['right',right]] as [side,panel]}
        <section class:active-panel={active===side} class="commander-panel" onclick={()=>active=side}>
          <div class="panel-path">
            <button onclick={()=>loadPanel(side,parent(panel.path))}>↑</button>
            <strong class="mono">{panel.path}</strong>
            <span class="spacer"></span>
            <button onclick={()=>createFolder(panel.path,()=>loadPanel(side,panel.path))}>＋D</button>
            <button onclick={()=>createFile(panel.path,()=>loadPanel(side,panel.path))}>＋F</button>
          </div>
          <div class="commander-list">
            <button class="file-row parent-row" onclick={()=>loadPanel(side,parent(panel.path))}><span>📁 ..</span><span></span><span></span></button>
            {#each panel.entries as entry (entry.path)}
              <button class:selected={panel.selected?.path===entry.path} class="file-row" onclick={()=>select(side,entry)} ondblclick={()=>open(side,entry)}>
                <span class="entry-name">{entry.kind==='directory'?'📁':entry.kind==='file'?'📄':'↗'} {entry.name}</span>
                <span class="mono">{entry.kind==='file'?bytes(entry.size||0):'—'}</span>
                <span class="mono">{entry.mode||''}</span>
              </button>
            {/each}
          </div>
          {#if panel.selected}
            <div class="panel-actions">
              {#if panel.selected.kind==='file'}<button onclick={()=>open(side,panel.selected)}>Edit</button>{/if}
              <button onclick={()=>renameEntry(panel.selected,()=>loadPanel(side,panel.path))}>Rename</button>
              <button onclick={()=>chmodEntry(panel.selected,()=>loadPanel(side,panel.path))}>Chmod</button>
              <button class="danger" onclick={()=>deleteEntry(panel.selected,()=>loadPanel(side,panel.path))}>{copy.remove}</button>
            </div>
          {/if}
        </section>
      {/each}
    </div>
    <div class="commander-help mono"><span>LEFT ↔ RIGHT</span><span>double-click — open</span><span class="spacer"></span><span>{copy.regularOnly}</span></div>
  {:else}
    <div class="explorer-shell">
      <aside class="quick-tree">
        <strong>STRUCTURE</strong>
        {#each quickRoots as [label,path]}
          <button class:active={explorerPath===path||explorerPath.startsWith(`${path}/`)} onclick={()=>loadExplorer(path)}>› 📁 {label}</button>
        {/each}
      </aside>
      <section class="explorer-main">
        <div class="explorer-path">
          <button onclick={()=>loadExplorer(parent(explorerPath))}>↑</button>
          <span class="mono">{explorerPath}</span><span class="spacer"></span>
          <button onclick={()=>createFolder(explorerPath,()=>loadExplorer(explorerPath))}>＋ {copy.folder}</button>
          <button onclick={()=>createFile(explorerPath,()=>loadExplorer(explorerPath))}>＋ {copy.file}</button>
        </div>
        <div class="explorer-table">
          <div class="explorer-head"><span>{copy.name}</span><span>{copy.size}</span><span>{copy.modified}</span><span>{copy.mode}</span><span>{copy.actions}</span></div>
          <button class="explorer-row" onclick={()=>loadExplorer(parent(explorerPath))}><span>📁 ..</span><span>—</span><span>—</span><span>—</span><span></span></button>
          {#each explorerFiltered as entry (entry.path)}
            <div class="explorer-row">
              <button class="entry-open" onclick={()=>open('explorer',entry)}>{entry.kind==='directory'?'📁':entry.kind==='file'?'📄':'↗'} {entry.name}</button>
              <span class="mono">{entry.kind==='file'?bytes(entry.size||0):'—'}</span>
              <span class="mono">{new Date(entry.modified_at).toLocaleString()}</span>
              <span class="mono">{entry.mode||''}</span>
              <span class="row-actions">
                {#if entry.kind==='file'}<a href={adminFileDownloadURL(entry.path)}>↓</a>{/if}
                <button onclick={()=>renameEntry(entry,()=>loadExplorer(explorerPath))}>✎</button>
                <button onclick={()=>chmodEntry(entry,()=>loadExplorer(explorerPath))}>◈</button>
                <button class="danger" onclick={()=>deleteEntry(entry,()=>loadExplorer(explorerPath))}>×</button>
              </span>
            </div>
          {/each}
        </div>
      </section>
    </div>
  {/if}
</section>

{#if selectedFile}
  <FileEditorDrawer file={selectedFile} bind:content={editorContent} dirty={editorDirty} readOnly={editorReadOnly} busy={editorBusy} locale={locale} onSave={saveEditor} onClose={()=>{selectedFile=null;editorContent='';editorOriginal='';}} />
{/if}

<style>
  .fm-shell{border:1px solid var(--border-color,rgba(127,127,127,.22));border-radius:.72rem;overflow:hidden;background:rgba(0,0,0,.04)}
  .fm-toolbar,.panel-path,.explorer-path,.panel-actions,.commander-help{display:flex;align-items:center;gap:.45rem}
  .fm-toolbar{padding:.65rem .75rem;border-bottom:1px solid var(--border-color,rgba(127,127,127,.2))}
  button,.fm-search,.fm-toolbar a{font:inherit;color:inherit;border:1px solid var(--border-color,rgba(127,127,127,.3));background:rgba(127,127,127,.05);border-radius:.42rem;padding:.38rem .58rem}
  button{cursor:pointer}button:hover{background:rgba(127,127,127,.12)}button:disabled{opacity:.4;cursor:not-allowed}.danger{border-color:rgba(230,75,75,.45);color:#ff8f8f}
  .view-toggle{display:flex}.view-toggle button{border-radius:0}.view-toggle button:first-child{border-radius:.42rem 0 0 .42rem}.view-toggle button:last-child{border-radius:0 .42rem .42rem 0}.view-toggle button.active{background:rgba(45,210,105,.13);border-color:rgba(45,210,105,.5);color:#71e89c}
  .spacer{flex:1}.fm-search{min-width:16rem}.fm-ok,.fm-error{padding:.5rem .75rem;font-size:.78rem}.fm-ok{color:#71e89c}.fm-error{color:#ff8f8f}
  .commander-grid{display:grid;grid-template-columns:1fr 1fr;gap:1px;background:var(--border-color,rgba(127,127,127,.2));min-height:32rem}
  .commander-panel{min-width:0;background:var(--panel-color,#0f1720);outline:2px solid transparent;outline-offset:-2px}.commander-panel.active-panel{outline-color:#5de4c7}
  .panel-path{padding:.55rem .6rem;border-bottom:1px solid rgba(127,127,127,.18)}.panel-path strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .commander-list{max-height:55vh;overflow:auto;padding:.25rem}
  .file-row{width:100%;display:grid;grid-template-columns:minmax(0,1fr) 7rem 9rem;gap:.5rem;align-items:center;border:0;border-radius:.28rem;padding:.36rem .48rem;text-align:left}
  .file-row.selected{background:#16472b;color:#dffff0}.file-row:hover{background:rgba(93,228,199,.08)}.entry-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.parent-row{opacity:.7}
  .panel-actions{padding:.5rem .6rem;border-top:1px solid rgba(127,127,127,.18);flex-wrap:wrap}
  .commander-help{padding:.42rem .7rem;background:#0b1017;color:#718091;font-size:.66rem;flex-wrap:wrap}
  .explorer-shell{display:grid;grid-template-columns:14rem 1fr;min-height:34rem}.quick-tree{padding:.9rem;border-right:1px solid rgba(127,127,127,.18);display:flex;flex-direction:column;gap:.2rem}.quick-tree>strong{font-size:.68rem;color:#7b8998;margin-bottom:.4rem}.quick-tree button{border:0;text-align:left}.quick-tree button.active{background:#16472b;color:#8bf0ad}
  .explorer-main{min-width:0}.explorer-path{padding:.65rem;border-bottom:1px solid rgba(127,127,127,.18)}
  .explorer-table{max-height:58vh;overflow:auto}.explorer-head,.explorer-row{display:grid;grid-template-columns:minmax(16rem,1fr) 7rem 12rem 10rem 9rem;gap:.6rem;align-items:center;padding:.45rem .65rem;border-bottom:1px solid rgba(127,127,127,.16)}
  .explorer-head{position:sticky;top:0;z-index:2;background:#111923;font-size:.7rem;font-weight:800;text-transform:uppercase}.explorer-row{font-size:.78rem}.explorer-row>button{border:0;background:none;text-align:left;padding:0}.entry-open{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:650}.row-actions{display:flex;gap:.28rem}.row-actions a,.row-actions button{display:inline-grid;place-items:center;width:1.75rem;height:1.75rem;padding:0;text-decoration:none}
  @media(max-width:900px){.commander-grid{grid-template-columns:1fr}.explorer-shell{grid-template-columns:1fr}.quick-tree{display:none}.explorer-head,.explorer-row{grid-template-columns:minmax(10rem,1fr) 6rem 8rem}.explorer-head span:nth-child(3),.explorer-row>span:nth-child(3),.explorer-head span:nth-child(4),.explorer-row>span:nth-child(4){display:none}}
</style>
