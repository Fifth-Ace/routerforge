<script>
  import {workspaceHeight} from './NfqwsUIActions.js';
  import {onDestroy,untrack} from 'svelte';
  import {readNfqwsScripts} from './NfqwsReadOnlyScripts.js';
  let {active=false,openRequest={seq:0,name:''},onCount=()=>{},onNotice=()=>{}}=$props();
  let inventory=$state([]),root=$state('/opt/etc/nfqws2/lua'),loaded=$state(false),loading=$state(false);
  let query=$state(''),selected=$state(''),content=$state(''),meta=$state('только просмотр'),error=$state('');
  let fullscreen=$state(false),fetchBusy=$state(false),sequence=0;
  const controller=new AbortController();
  let loadPromise=null;
  let visible=$derived(inventory.filter(f=>!query||String(f.name||'').toLowerCase().includes(query.trim().toLowerCase())));
  function formatSize(n){n=Number(n||0);return n<1024?`${n} B`:n<1048576?`${(n/1024).toFixed(1)} KB`:`${(n/1048576).toFixed(1)} MB`;}
  function load(){
    if(loaded||controller.signal.aborted)return Promise.resolve();
    if(loadPromise)return loadPromise;
    loading=true;error='';
    loadPromise=(async()=>{
      try{
        const result=await readNfqwsScripts('inventory',{signal:controller.signal});
        if(controller.signal.aborted)return;
        inventory=Array.isArray(result.files)?result.files:[];
        root=String(result.root||result.roots?.[0]||'/opt/etc/nfqws2/lua');
        loaded=true;onCount(inventory.length);
      }catch(e){if(!controller.signal.aborted)error=String(e?.message||e);}
      finally{loading=false;loadPromise=null;}
    })();
    return loadPromise;
  }
  async function selectScript(name){
    if(!name||controller.signal.aborted)return;
    const current=++sequence;
    selected=name;meta='Loading…';fetchBusy=true;content='';
    try{
      const data=await readNfqwsScripts('name:'+name,{signal:controller.signal});
      if(controller.signal.aborted||current!==sequence)return;
      content=String(data.content||'');
      meta=`${data.path||name} · ${formatSize(data.size)} · read-only`;
    }catch(e){if(!controller.signal.aborted&&current===sequence){meta=String(e?.message||e);error=meta;}}
    finally{if(current===sequence)fetchBusy=false;}
  }
  async function openFromSetup(name){
    await load();
    if(controller.signal.aborted)return;
    const found=inventory.find(f=>String(f.name)===String(name));
    if(found)await selectScript(found.name);
    else{selected='';content='';meta='Скрипт упомянут в BASE_ARGS, но не найден в runtime inventory';error=String(name);}
  }
  $effect(()=>{if(active)untrack(()=>{void load();});});
  $effect(()=>{const request=openRequest;if(request.seq)untrack(()=>{void openFromSetup(request.name);});});
  $effect(()=>{
    document.body.classList.toggle('syntax-fullscreen-open',fullscreen);
    document.documentElement.classList.toggle('syntax-fullscreen-open',fullscreen);
  });
  onDestroy(()=>{
    controller.abort();++sequence;
    document.body.classList.remove('syntax-fullscreen-open');
    document.documentElement.classList.remove('syntax-fullscreen-open');
  });
  async function copy(){try{if(!navigator.clipboard?.writeText)throw Error('Clipboard API недоступен');await navigator.clipboard.writeText(content);onNotice('Lua-скрипт скопирован','good')}catch(e){onNotice(String(e?.message||e),'error')}}
</script>
<section id="view-scripts" class="view" class:active>
  <div id="scriptWorkspace" class="script-layout rf-workspace" use:workspaceHeight>
    <article class="panel">
      <div class="panel-head"><div><h2>Lua-скрипты</h2><div id="scriptRoot" class="section-kicker mono">{root}</div></div></div>
      <input id="scriptSearch" class="search" placeholder="Поиск скрипта…" bind:value={query} />
      <div id="scriptList" class="script-list">
        {#if loading}<div class="empty">Загрузка Lua inventory…</div>
        {:else if error && !loaded}<div class="empty bad-text">{error}</div>
        {:else if !visible.length}<div class="empty">Lua-скрипты не найдены.</div>
        {:else}
          {#each visible as f (f.name)}
            <button type="button" class="script-item" class:active={selected===f.name} onclick={()=>selectScript(f.name)}>
              <div><div class="script-name">{f.name}</div><div class="script-meta">{f.modified_at||''} · read-only</div></div><div class="script-meta">{formatSize(f.size)}</div>
            </button>
          {/each}
        {/if}
      </div>
    </article>
    <article id="scriptPanel" class="panel script-panel" class:script-fullscreen={fullscreen}>
      <div class="panel-head"><div><h2 id="selectedScriptTitle">{selected||'Выберите скрипт'}</h2><div id="selectedScriptMeta" class="section-kicker mono">{meta}</div></div><div class="toolbar"><button type="button" id="copyScript" disabled={!selected||fetchBusy} onclick={copy}>Копировать</button><button type="button" id="fullscreenScript" disabled={!selected||fetchBusy} onclick={()=>fullscreen=!fullscreen}>{fullscreen?'Закрыть fullscreen':'На весь экран'}</button></div></div>
      {#if selected}
        <div id="scriptCode" class="script-code-shell">
          {#each content.split(/\r?\n/) as line,i (i)}
            <div class="script-code-line"><span class="script-line-number">{i+1}</span><code>{line}</code></div>
          {/each}
        </div>
      {:else}<div id="scriptCode" class="script-reader-empty">Lua-скрипты nfqws2 доступны только для чтения.</div>{/if}
    </article>
  </div>
</section>
