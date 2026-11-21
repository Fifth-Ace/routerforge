<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {memoryReadPath,memoryClearBody,memoryProject} from './NfqwsTargetMemoryModel.js';
  let {active=false,serviceBusy=false,onRequest=async()=>{},onCount=()=>{},onNotice=()=>{}}=$props();
  let mounted=$state(false),attempted=$state(false),loading=$state(false),clearing=$state(false);
  let filter=$state(''),snapshot=$state({policy:rfUi('Правила хранения ещё не загружены.',"Storage rules have not been loaded yet."),rows:[]}),phase=$state('IDLE'),error=$state(''),pending=$state(null);
  let alive=false,epoch=0,controller=null;
  const locked=$derived(serviceBusy||loading||clearing);
  function message(e){return String(e?.message||e||'NFQWS_MEMORY_ERROR');}
  function notice(text,type=''){if(alive)onNotice({text:String(text),type});}
  function cancel(){++epoch;controller?.abort();controller=null;loading=false;clearing=false;pending=null;}
  async function request(path,method='GET',body,signal){
    if(!alive||!(path.startsWith('/v2/memory?target=')||path==='/v2/memory'||path==='/v2/memory/clear'))throw Error('NFQWS_MEMORY_PATH_FORBIDDEN');
    if(method==='POST'&&path!=='/v2/memory/clear'||method==='GET'&&path==='/v2/memory/clear')throw Error('NFQWS_MEMORY_METHOD_FORBIDDEN');
    return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});
  }
  async function load(){
    if(!alive||loading||clearing)return;
    const n=++epoch,c=new AbortController();controller?.abort();controller=c;
    attempted=true;loading=true;phase='LOADING';error='';pending=null;
    try{
      const path=memoryReadPath(filter);
      const payload=await request(path,'GET',undefined,c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      snapshot=memoryProject(payload);onCount(snapshot.rows.length);phase='READ ONLY';
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=message(e);phase='ERROR';}}
    finally{if(controller===c)controller=null;if(alive&&n===epoch)loading=false;}
  }
  function stageClear(all){
    if(locked||!active)return;
    try{memoryClearBody(filter,all);pending={all,target:all?'':filter.trim()};error='';}
    catch(e){error=message(e);}
  }
  async function confirmClear(){
    if(!alive||!active||locked||!pending)return;
    const next=pending;pending=null;
    const n=++epoch,c=new AbortController();controller?.abort();controller=c;
    clearing=true;phase='CLEARING';error='';
    try{
      const body=memoryClearBody(next.target,next.all);
      await request('/v2/memory/clear','POST',body,c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      const payload=await request(memoryReadPath(filter),'GET',undefined,c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      snapshot=memoryProject(payload);onCount(snapshot.rows.length);phase='READ ONLY';
      notice(next.all?'Вся Target Memory очищена. Production не изменён.':rfUi(`${next.target}: история очищена. Production не изменён.`,`${next.target}: history cleared. Active configuration unchanged.`),'good');
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=message(e);phase='ERROR';notice(error,'error');}}
    finally{if(controller===c)controller=null;if(alive&&n===epoch)clearing=false;}
  }
  onMount(()=>{alive=true;mounted=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(mounted&&active&&!attempted)void load();});
  $effect(()=>{if(mounted&&!active&&controller)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-target-memory-v2">
  <div class="v2-hero"><div><h2>{rfUi("История подборов","Selection history")}</h2><div class="section-kicker">{rfUi("сохранённые результаты прошлых проверок","saved results of previous checks")}</div></div><span id="v2MemoryState" class="config-badge">{phase}{phase==='READ ONLY'?` · ${snapshot.rows.length} ENTRIES`:''}</span></div>
  <div class="toolbar"><input id="v2MemoryFilter" class="search" placeholder={rfUi("Фильтр по домену · example.com","Filter by domain · example.com")} bind:value={filter} disabled={locked} onkeydown={e=>{if(e.key==='Enter')void load();}}>
    <button id="v2MemoryRefresh" disabled={locked} onclick={load}>{rfUi("Обновить","Refresh")}</button>
    <button id="v2MemoryClearTarget" class="danger" disabled={locked} onclick={()=>stageClear(false)}>{rfUi("Очистить домен…","Clear domain…")}</button>
    <button id="v2MemoryClearAll" class="danger" disabled={locked} onclick={()=>stageClear(true)}>{rfUi("Очистить всю память…","Clear all history…")}</button></div>
  <div id="v2MemoryPolicy" class="v2-boundary">{snapshot.policy}</div>
  <div id="v2MemoryList" class="v2-list" style="margin-top:10px">
    {#each snapshot.rows as row,i (`${row.target}:${row.fingerprint}:${i}`)}
      <div class={`v2-row ${row.reuse?'good':'warn'}`}>
        <div class="v2-result-title">{row.target}</div>
        <div><div class="value">{row.title} · {row.protocol} · {row.confidence}</div><div class="meta">{row.source} · {row.verdict} · verified={row.verified} · streak={row.streak} · success={row.rate}% · age={row.age}s · {row.reason}</div><details><summary>args / fingerprint</summary><pre class="v2-args">{row.args.join(' ')}

{row.fingerprint}</pre></details></div><span class={`v2-pill ${row.reuse?'good':'warn'}`}>{row.reuse?'REUSE':'HISTORY'}</span>
      </div>
    {:else}<div class="analysis-empty">{rfUi("Target Memory пуста.","No saved target history.")}</div>{/each}
  </div>
  {#if pending}<div role="group" class="setup-note" aria-label={rfUi("Подтвердите очистку Target Memory","Confirm clearing target history")}><strong>{pending.all?'Очистить всю Target Memory?':`Очистить историю ${pending.target}?`}</strong><div>{rfUi("Production config и runtime не изменяются.","Active configuration and runtime remain unchanged.")}</div><div class="toolbar"><button disabled={locked} onclick={()=>pending=null}>{rfUi("Отмена","Cancel")}</button><button class="danger" disabled={locked} onclick={confirmClear}>{rfUi("Подтвердить очистку","Confirm clearing")}</button></div></div>{/if}
  {#if error}<div class="notice error" role="alert">{error}</div>{/if}
  <div class="v2-boundary">{rfUi("История используется только для справки. Новый подбор всегда проверяет текущее соединение заново. Очистка выполняется только после отдельного подтверждения.","History is for reference only. Each new selection checks the current connection again. Clearing history requires separate confirmation.")}</div>
</div>
