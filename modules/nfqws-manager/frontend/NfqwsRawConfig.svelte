<script>
  import {onMount} from 'svelte';
  import {highlightLibraryConfig} from './NfqwsConfigSyntax.js';
  import {statusRawConfig,rawDiff,rawStatistics,rawPreflight,rawLuaPayloadReplacement,rawValidationSummary,rawApiRequest,sha256RE} from './NfqwsRawConfigModel.js';
  let {active=false,snapshot=null,service={mutationReady:false},onChanged=()=>{},onNotice=()=>{}}=$props();
  let baseline=$state(null),value=$state(''),busy=$state(false),error=$state(''),diffOpen=$state(false);
  let validation=$state(null),validatedText=$state(null),syntaxMessage=$state('Проверка конфига ещё не запускалась. Это статический анализ, не nfqws2 --dry-run.');
  let luaCatalog=$state(null),luaSearch=$state(''),luaOpen=$state(false),full=$state(false),stale=$state(false);
  let editor=$state(null),gutter=$state(null),highlight=$state(null);
  let alive=false,seq=0,controller,api, lastSnapshot=null;
  const dirty=$derived(baseline!==null&&value!==baseline.content);
  const stats=$derived(rawStatistics(value));
  const changes=$derived(rawDiff(baseline?.content||'',value));
  const luaRows=$derived((Array.isArray(luaCatalog?.functions)?luaCatalog.functions:[]).filter(f=>!luaSearch||[f.name,f.file,f.standard,f.legacy,...(f.description||[])].join(' ').toLowerCase().includes(luaSearch.toLowerCase())).slice(0,80));
  function notify(text,type=''){if(alive)onNotice({text:String(text),type});}
  function aliveNow(){return alive&&!controller.signal.aborted;}
  function fail(err){if(aliveNow()){error=String(err?.message||err);notify(error,'error');}}
  function hydrate(next){
    if(!next||next===lastSnapshot)return;
    lastSnapshot=next;
    try{
      const fresh=statusRawConfig(next);
      if(dirty){stale=!!baseline&&(baseline.sha256!==fresh.sha256||baseline.content!==fresh.content);return;}
      baseline=fresh;value=fresh.content;stale=!sha256RE.test(fresh.sha256);validation=null;validatedText=null;
    }catch(e){if(aliveNow())fail(e);}
  }
  $effect(()=>{hydrate(snapshot);});
  function changed(){validation=null;validatedText=null;syntaxMessage='Черновик изменён: проверка устарела.';}
  function scrollEditor(){if(gutter)gutter.scrollTop=editor?.scrollTop||0;if(highlight){highlight.scrollTop=editor?.scrollTop||0;highlight.scrollLeft=editor?.scrollLeft||0;}}
  function keydown(e){
    if(e.key==='Tab'){e.preventDefault();const a=e.currentTarget.selectionStart,b=e.currentTarget.selectionEnd;value=value.slice(0,a)+'  '+value.slice(b);changed();queueMicrotask(()=>{if(editor){editor.selectionStart=editor.selectionEnd=a+2;}});}
    if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'){e.preventDefault();e.stopPropagation();if(dirty&&!busy)diffOpen=true;}
    if(e.key==='Escape'&&full)full=false;
  }
  async function checkSyntax(){
    if(!aliveNow()||!active||busy||!baseline)return;
    busy=true;error='';const current=value,id=++seq;validation=null;validatedText=null;
    syntaxMessage='Проверка синтаксиса и native dry-run…';
    try{
      const result=await api('/config/validate',{method:'POST',body:JSON.stringify({content:current})});
      if(!aliveNow()||id!==seq)return;
      if(current!==value){syntaxMessage='Конфиг изменён во время проверки. Повтори проверку.';return;}
      validation=rawValidationSummary(result);validatedText=current;syntaxMessage=validation.text;
    }catch(e){if(aliveNow()&&id===seq){fail(e);syntaxMessage='Проверка не выполнена';}}
    finally{if(aliveNow()&&id===seq)busy=false;}
  }
  async function luaLoad(){
    if(!aliveNow()||!active||busy)return;
    busy=true;error='';const id=++seq;
    try{
      const result=await api('/config/lua-catalog');
      if(!aliveNow()||id!==seq)return;
      if(!Array.isArray(result.functions))throw Error('NFQWS_LUA_CATALOG_INVALID');
      luaCatalog=result;
    }catch(e){if(aliveNow()&&id===seq)fail(e);}
    finally{if(aliveNow()&&id===seq)busy=false;}
  }
  function luaFix(){
    if(!editor||!validation?.unknownLua||validatedText!==value)return;
    try{
      const result=rawLuaPayloadReplacement(value,editor.selectionStart,editor.selectionEnd,validatedText);
      if(!window.confirm('Добавить payload=all только к выделенному аргументу в черновике?'))return;
      value=result.text;changed();diffOpen=true;
      queueMicrotask(()=>{if(editor){editor.selectionStart=result.selectionStart;editor.selectionEnd=result.selectionEnd;}});
    }catch(e){fail(e);}
  }
  async function save(){
    if(!aliveNow()||!active||busy||stale||!dirty||!baseline||!service.mutationReady)return;
    if(!window.confirm('Сохранить nfqws2.conf, создать backup и выполнить reload?'))return;
    busy=true;error='';const id=++seq,sent=value,reference={...baseline};
    try{
      // Client-side stale preflight is NOT atomic server-side CAS. Backend must
      // keep its existing backup / atomic-write / reload / rollback contract.
      const latest=await api('/status');
      if(!aliveNow()||id!==seq)return;
      rawPreflight(reference,latest,value,sent);
      if(!active)throw Error('NFQWS_CONFIG_VIEW_INACTIVE');
      const result=await api('/config',{method:'POST',body:JSON.stringify({content:sent,confirm:'NFQWS_CONFIG'})});
      if(!aliveNow()||id!==seq)return;
      diffOpen=false;
      notify('Конфигурация применена. Backup: '+String(result.backup||'—'),'good');
      // Only authoritative status can acknowledge an applied revision.
      if(result.status){
        onChanged(result.status);
        if(value===sent){baseline=statusRawConfig(result.status);value=baseline.content;stale=!sha256RE.test(baseline.sha256);}
      }else{
        onChanged(null);
        stale=true;
      }
    }catch(e){if(aliveNow()&&id===seq)fail(e);}
    finally{if(aliveNow()&&id===seq)busy=false;}
  }
  onMount(()=>{
    alive=true;controller=new AbortController();api=rawApiRequest(fetch,location.pathname,controller.signal);
    return()=>{alive=false;controller.abort();++seq;};
  });
</script>
<article id="editorPanel" class="panel" data-svelte-owned="raw-config">
  <div class="panel-head">
    <div><h2>Полный nfqws2.conf</h2><div id="configMeta" class="section-kicker mono">{baseline?`${baseline.path} · ${stats.bytes} B · SHA256 ${baseline.sha256.slice(0,20)}…`:'Конфигурация не загружена'}</div></div>
    <div class="toolbar">
      <button id="undoConfig" type="button" disabled={!dirty||busy} onclick={()=>{value=baseline.content;changed();stale=false;}}>Отменить</button>
      <button id="diffConfig" type="button" disabled={!dirty||busy} onclick={()=>diffOpen=true}>Diff</button>
      <button id="checkConfigSyntax" type="button" disabled={!baseline||busy} onclick={checkSyntax}>Проверить конфиг</button>
      <button id="save" type="button" class="primary" disabled={!dirty||busy||!service.mutationReady||stale} onclick={()=>diffOpen=true}>Сохранить и применить</button>
    </div>
  </div>
  <div class="setup-note warn" style="margin-bottom:10px">Полный nfqws2.conf. Изменения отправляются только после подтверждения. Серверный атомарный CAS не подтверждён.</div>
  <div id="configSyntaxStatus" class="setup-note" role="status" aria-live="polite" style="margin-bottom:10px">{syntaxMessage}</div>
  {#if validation?.issues?.length}
    <pre id="configSemanticIssues" role="status" style="white-space:pre-wrap">{validation.issues.map(i=>`${i.severity||'info'} · ${i.variable||''}: ${i.message||i.code||''}`).join('\n')}</pre>
  {/if}
  {#if validation?.unknownLua&&validatedText===value}
    <div id="nf1bLuaFixNote" class="setup-note">
      Выдели целиком аргумент --lua-desync=... без payload=.
      <button id="nf1bLuaPayloadFix" type="button" disabled={busy} onclick={luaFix}>Добавить payload=all к выделенному Lua</button>
    </div>
  {/if}
  <details id="nf1bLuaPanel" class="panel" style="margin:0 0 12px;padding:12px" bind:open={luaOpen}>
    <summary>Lua-функции на роутере · справочник</summary>
    <div class="toolbar" style="margin:10px 0"><input id="nf1bLuaQuery" type="search" aria-label="Поиск Lua" placeholder="Поиск функции" bind:value={luaSearch}><button id="nf1bLuaRefresh" type="button" disabled={busy} onclick={luaLoad}>Прочитать локальные Lua</button></div>
    <div id="nf1bLuaInfo" role="status">{luaCatalog?`Функций: ${luaCatalog.functions.length} · показано ${luaRows.length}`:'Для чтения нажми кнопку. Автоматического выполнения Lua нет.'}</div>
    <div id="nf1bLuaResults" class="v2-list">
      {#each luaRows as fn,i (i)}<div class="v2-row"><strong>{fn.name||'—'} · {fn.file||''}</strong><div class="section-kicker" style="white-space:pre-wrap">{[fn.standard,fn.legacy,...(fn.description||[]),...(fn.arguments||[]).map(x=>x.help||x.name)].filter(Boolean).join('\n')}</div></div>{/each}
    </div>
  </details>
  <div class={full?'editor-shell full-editor':'editor-shell'} id="configEditorPanel">
    <div class="editor-toolbar"><span id="editorState" class={dirty?'mono dirty':'mono good-text'}>{dirty?'UNSAVED':'CLEAN'}</span><div class="editor-toolbar-actions"><span>Ctrl+S → Diff · Tab = 2 spaces</span><button id="fullscreenConfig" type="button" onclick={()=>full=!full}>{full?'Закрыть fullscreen':'На весь экран'}</button></div></div>
    <div class="editor-main syntax-editor-body">
      <pre id="gutter" class="gutter" aria-hidden="true" bind:this={gutter}>{Array.from({length:stats.lines},(_,i)=>i+1).join('\n')}</pre>
      <div class="syntax-editor-body"><pre class="syntax-editor-highlight" aria-hidden="true" bind:this={highlight}>{@html highlightLibraryConfig(value)}</pre><textarea id="config" class="editor" spellcheck="false" bind:this={editor} bind:value oninput={changed} onscroll={scrollEditor} onkeydown={keydown} aria-label="Полный nfqws2.conf"></textarea></div>
    </div>
    <div class="editor-foot"><span id="editorStats">{stats.lines} lines · {stats.bytes} B</span><span>backup → atomic write → reload → rollback</span></div>
  </div>
  {#if stale}<div class="setup-note bad-text" role="alert">Конфигурация изменена на сервере. Черновик сохранён локально, применение заблокировано.</div>{/if}
  {#if error}<p class="setup-note bad-text" role="alert">{error}</p>{/if}
</article>
{#if diffOpen&&dirty}
  <div class="modal open" id="rawConfigDiffModal" role="presentation"><div class="dialog">
    <div class="dialog-head"><strong>Изменения активного nfqws2.conf</strong><button type="button" disabled={busy} onclick={()=>diffOpen=false}>Закрыть</button></div>
    <pre class="diff">{#each changes as row,i (i)}<span class={row.kind}>{row.text}{'\n'}</span>{/each}</pre>
    <div class="dialog-foot"><button type="button" disabled={busy} onclick={()=>diffOpen=false}>Отмена</button><button type="button" class="primary" disabled={busy||stale||!service.mutationReady} onclick={save}>Подтвердить сохранение и применение</button></div>
  </div></div>
{/if}
