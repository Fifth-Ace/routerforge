<script>
  import {onMount} from 'svelte';
  import {verifiedStrategyInventory,verifiedStrategyLibraryResponse,strategySaveBody,strategyDeleteBody,strategyRequestPath,strategyText,STRATEGY_SOURCES} from './NfqwsStrategyLibraryModel.js';
  let {active=false,refreshSeq=0,serviceBusy=false,onRequest=async()=>{},onOpenSandbox=()=>{},onNotice=()=>{}}=$props();
  let inventory=$state([]),appliedRefreshSeq=$state(-1);
  let id=$state(''),name=$state(''),source=$state('custom'),args=$state('');
  let original=$state(null),busy=$state(false),attempted=$state(false),error=$state('');
  let alive=false,seq=0,mounted=$state(false);
  const entries=$derived(verifiedStrategyInventory(inventory));
  const dirty=$derived(original!==null&&(name!==original.name||source!==original.source||args!==original.args));
  const busyAny=$derived(busy||serviceBusy);
  function notify(text,type=''){if(alive)onNotice({text:String(text),type});}
  function fail(e){if(alive){error=String(e?.message||e);notify(error,'error');}}
  function clear(){if(busyAny)return;id='';name='';source='custom';args='';original=null;error='';}
  function pick(entry){
    if(busyAny)return;
    if(dirty&&!window.confirm('Есть несохранённые изменения candidate. Переключиться без сохранения?'))return;
    id=entry.id;name=entry.name;source=entry.source;args=strategyText(entry.args);original={id,name,source,args};error='';
  }
  async function loadInventory(n){
    strategyRequestPath('/v2/strategies','GET');
    const response=await onRequest('/v2/strategies',{method:'GET'});
    if(!alive||n!==seq)return false;
    inventory=verifiedStrategyLibraryResponse(response);
    return true;
  }
  async function refresh(){
    if(!alive||busy)return;
    busy=true;error='';attempted=true;const n=++seq;
    // Record this attempt before I/O: failures show an error, not a retry storm.
    // A later Import increments refreshSeq and schedules another fetch.
    appliedRefreshSeq=refreshSeq;
    // A stale library must not authorize Save/Delete after a failed refresh.
    inventory=[];
    try{await loadInventory(n);}
    catch(e){if(alive&&n===seq)fail(e);}
    finally{if(alive&&n===seq)busy=false;}
  }
  async function send(path,body){strategyRequestPath(path,'POST');return onRequest(path,{method:'POST',body:JSON.stringify(body)});}
  async function save(){
    if(!alive||!active||busyAny)return;
    let payload;
    try{payload=strategySaveBody({id,name,source,args},entries);}catch(e){fail(e);return;}
    busy=true;error='';const n=++seq;
    try{
      const result=await send('/v2/strategies/save',payload);
      if(!alive||n!==seq)return;
      if(!result?.strategy?.id)throw Error('NFQWS_STRATEGY_SAVE_NO_SERVER_ID');
      const savedId=result.strategy.id;
      // Never leave a stale pre-mutation inventory as authority if GET fails.
      inventory=[];
      await loadInventory(n);if(!alive||n!==seq)return;
      // Authoritative server response, not a local success guess.
      id=savedId;name=result.strategy.name||payload.name;source=result.strategy.source||payload.source;args=strategyText(result.strategy.args||payload.args);
      original={id,name,source,args};notify(`Библиотека стратегий: ${name} сохранён.`,'good');
    }catch(e){if(alive&&n===seq)fail(e);}
    finally{if(alive&&n===seq)busy=false;}
  }
  async function remove(entry){
    if(!alive||!active||busyAny||!window.confirm(`Удалить ${entry.name} из Библиотеки стратегий?`))return;
    let payload;try{payload=strategyDeleteBody(entry.id,entries);}catch(e){fail(e);return;}
    busy=true;error='';const n=++seq;
    try{
      const result=await send('/v2/strategies/delete',payload);
      if(result?.ok!==true||result.deleted!==entry.id)throw Error('NFQWS_STRATEGY_DELETE_UNCONFIRMED');
      if(!alive||n!==seq)return;
      if(id===entry.id){id='';name='';source='custom';args='';original=null;}
      inventory=[];
      await loadInventory(n);if(!alive||n!==seq)return;
      notify('Candidate удалён.','good');
    }catch(e){if(alive&&n===seq)fail(e);}
    finally{if(alive&&n===seq)busy=false;}
  }
  function sandbox(entry){if(busyAny)return;onOpenSandbox({name:entry.name,source:entry.source,args:strategyText(entry.args)});}
  function sandboxDraft(){if(busyAny||!args.trim())return;onOpenSandbox({name:name.trim()||'Library editor',source,args});}
  onMount(()=>{alive=true;mounted=true;return()=>{alive=false;++seq;};});
  // Import invalidation and first activation are both owned by Svelte.
  $effect(()=>{if(mounted&&active&&!busy&&(!attempted||appliedRefreshSeq!==refreshSeq))void refresh();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-strategy-library">
  <div class="v2-hero"><div><h2>Свои стратегии</h2><div class="section-kicker">persistent Библиотека стратегий на роутере</div></div><span id="v2LibraryState" class="config-badge">{busy?'LOADING':error?'ERROR':attempted?'READY':'IDLE'}</span></div>
  <div class="v2-two">
    <article class="panel">
      <div class="panel-head"><div><h2>Сохранённые candidates</h2><div id="v2LibraryCount" class="section-kicker">{entries.length} / 64</div></div><button id="v2LibraryRefresh" type="button" disabled={busyAny} onclick={refresh}>Обновить</button></div>
      <div id="v2LibraryList" class="v2-list">
        {#each entries as entry (entry.id)}
          <div class="v2-row"><div><div class="v2-result-title">{entry.name}</div><div class="meta">{entry.source} · {entry.fingerprint.slice(0,16)}</div></div><div class="v2-args">{strategyText(entry.args)}</div>
            <div class="v2-library-actions"><button type="button" disabled={busyAny} onclick={()=>pick(entry)}>Редактировать</button><button type="button" disabled={busyAny} onclick={()=>sandbox(entry)}>Песочница</button><button type="button" class="danger" disabled={busyAny} onclick={()=>remove(entry)}>Удалить</button></div>
          </div>
        {:else}<div class="analysis-empty">{error||(busy||!attempted?'Загрузка…':'Библиотека стратегий пуста.')}</div>{/each}
      </div>
    </article>
    <article class="panel">
      <label class="v2-label">Название<input id="v2LibraryName" class="search" placeholder="YouTube strategy" bind:value={name} disabled={busyAny}></label>
      <label class="v2-label">Источник<select id="v2LibrarySource" class="search" bind:value={source} disabled={busyAny}>{#each STRATEGY_SOURCES as option (option)}<option value={option}>{option}</option>{/each}</select></label>
      <label class="v2-label">Args<textarea id="v2LibraryArgs" class="v2-textarea" placeholder="--filter-tcp=443 ..." bind:value={args} disabled={busyAny}></textarea></label>
      <input id="v2LibraryID" type="hidden" value={id}>
      <div class="toolbar"><button id="v2LibrarySave" class="primary" type="button" disabled={busyAny||!name.trim()||!args.trim()} onclick={save}>Сохранить</button><button id="v2LibraryClear" type="button" disabled={busyAny} onclick={clear}>Новая</button><button id="v2LibraryToSandbox" type="button" disabled={busyAny||!args.trim()} onclick={sandboxDraft}>В Sandbox</button></div>
      {#if dirty}<div class="section-kicker">UNSAVED — изменения не применены</div>{/if}
    </article>
  </div>
  {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  <div class="v2-boundary">Library хранит только bench-eligible candidate args и fingerprint. Active nfqws2.conf не меняется; никаких действий при mount.</div>
</div>
