<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {observedPath,observedProject,observedScanBody,observedIgnoreBody,observedAppendUserList} from './NfqwsObservedModel.js';
  let {active=false,serviceBusy=false,onRequest=async()=>{},onTarget=()=>{},onNotice=()=>{}}=$props();
  let mounted=$state(false),attempted=$state(false),loading=$state(false),working=$state(false);
  let snapshot=$state({rows:[],suppressed:0,ignored:0,minEvents:2,flowSource:'stored evidence'});
  let phase=$state('IDLE'),error=$state(''),pending=$state(null);
  let alive=false,epoch=0,controller=null;
  const locked=$derived(serviceBusy||loading||working);
  function notice(text,type=''){if(alive)onNotice({text:String(text),type});}
  function message(e){return String(e?.message||e||'NFQWS_OBSERVED_ERROR');}
  async function request(path,method='GET',body,signal){
    if(!alive)throw Error('NFQWS_OBSERVED_UNMOUNTED');
    observedPath(path,method);
    return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});
  }
  function cancel(){++epoch;controller?.abort();controller=null;pending=null;loading=false;working=false;}
  async function load(){
    if(!alive||loading||working)return;
    const n=++epoch,c=new AbortController();controller?.abort();controller=c;
    loading=true;attempted=true;phase='LOADING';error='';pending=null;
    try{
      const data=await request('/v2/observed-targets','GET',undefined,c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      snapshot=observedProject(data);phase='READ ONLY';
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=message(e);phase='ERROR';}}
    finally{if(controller===c)controller=null;if(alive&&n===epoch)loading=false;}
  }
  async function scan(){
    if(!active||locked)return;
    const n=++epoch,c=new AbortController();controller?.abort();controller=c;
    working=true;phase='SCANNING';error='';pending=null;
    try{
      const data=await request('/v2/observed-targets/scan','POST',observedScanBody(),c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      snapshot=observedProject(data);phase='READY';notice(`Observed: ${snapshot.rows.length} visible · ${snapshot.suppressed} suppressed.`,'good');
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=message(e);phase='ERROR';}}
    finally{if(controller===c)controller=null;if(alive&&n===epoch)working=false;}
  }
  function select(action,row){
    if(locked||!active)return;
    if(['setup','diagnose','sandbox','selector'].includes(action)){
      if(row.domain)onTarget(action,row.domain);
      return;
    }
    if(action==='ignore'||action==='list'){
      if(action==='list'&&!row.domain)return;
      pending={action,row};
    }
  }
  async function confirmAction(){
    if(!active||locked||!pending)return;
    const {action,row}=pending;pending=null;
    const n=++epoch,c=new AbortController();controller?.abort();controller=c;
    working=true;error='';phase='UPDATING';
    try{
      if(action==='ignore'){
        await request('/v2/observed-targets/ignore','POST',observedIgnoreBody(row),c.signal);
      }else if(action==='list'){
        const original=await request('/list?name=user.list','GET',undefined,c.signal);
        if(!alive||n!==epoch||c.signal.aborted)return;
        const body=observedAppendUserList(original,row);
        if(body)await request('/list/save','POST',body,c.signal);
        else notice(rfUi(`${row.domain} уже присутствует в user.list.`,`${row.domain} is already in user.list.`),'good');
      }else throw Error('NFQWS_OBSERVED_ACTION_UNKNOWN');
      if(!alive||n!==epoch||c.signal.aborted)return;
      const updated=await request('/v2/observed-targets','GET',undefined,c.signal);
      if(!alive||n!==epoch||c.signal.aborted)return;
      snapshot=observedProject(updated);phase='READY';
      notice(action==='ignore'?rfUi(`${row.target} скрыт.`,`${row.target} hidden.`):rfUi(`${row.domain} обработан через CAS user.list.`,`${row.domain} processed in user.list with a concurrency check.`),'good');
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=message(e);phase='ERROR';notice(error,'error');}}
    finally{if(controller===c)controller=null;if(alive&&n===epoch)working=false;}
  }
  onMount(()=>{alive=true;mounted=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(mounted&&active&&!attempted)void load();});
  $effect(()=>{if(mounted&&!active&&controller)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-observed-targets">
  <div class="v2-hero"><div><h2>{rfUi("Проблемные адреса","Affected addresses")}</h2><div class="section-kicker">{rfUi("повторные подозрительные сетевые сбои, не доказанная блокировка","repeated suspicious network failures, not confirmed blocking")}</div></div><span id="v2ObservedState" class="config-badge">{phase}</span></div>
  <article class="panel">
    <div class="panel-head"><div><h2>{rfUi("Найденные проблемы в локальной сети","Detected issues on the local network")}</h2><div class="section-kicker">{rfUi("показываются после повторного независимого failure signal","shown after repeated independent failures")}</div></div>
      <div class="toolbar"><button id="v2ObservedLoad" disabled={locked} onclick={load}>{rfUi("Обновить","Refresh")}</button><button id="v2ObservedScan" class="primary" disabled={locked} onclick={scan}>{rfUi("Проверить сетевые соединения","Check network connections")}</button></div>
    </div>
    <div id="v2ObservedMeta" class="v2-boundary">visible={snapshot.rows.length} · suppressed={snapshot.suppressed} · ignored={snapshot.ignored} · min events={snapshot.minEvents} · source={snapshot.flowSource}</div>
    <div id="v2ObservedList" class="v2-list">
      {#each snapshot.rows as row,i (`${row.target}:${row.device}:${i}`)}
        <div class={`v2-row ${row.active?'warn':'info'}`}>
          <div><div class="v2-result-title">{row.target||'—'}</div><div class="meta">device={row.device} · {row.protocol}/{row.port} · {row.reason} · count={row.count} · last={row.lastSeen}</div></div>
          <div class="toolbar"><button disabled={locked||!row.domain} onclick={()=>select('setup',row)}>Auto Setup</button><button disabled={locked||!row.domain} onclick={()=>select('diagnose',row)}>Diagnose</button><button disabled={locked||!row.domain} onclick={()=>select('sandbox',row)}>{rfUi("Песочница","Sandbox")}</button><button disabled={locked||!row.domain} onclick={()=>select('selector',row)}>Auto Select</button><button disabled={locked||!row.domain} onclick={()=>select('list',row)}>{rfUi("В user.list","In user.list")}</button><button disabled={locked} onclick={()=>select('ignore',row)}>Ignore</button></div>
          <span class={`v2-pill ${row.active?'warn':'info'}`}>{row.active?'ACTIVE':'SEEN'}</span>
        </div>
      {:else}<div class="analysis-empty">{rfUi("Повторных suspected failures пока нет. Первый шумовой сигнал не показывается.","No recurring suspected failures yet. One-off events are filtered out.")}</div>{/each}
    </div>
    {#if pending}
      <div class="setup-note" role="group" aria-label={rfUi("Подтверждение действия","Confirm action")}>
        <strong>{pending.action==='ignore'?'Скрыть обнаруженную цель?':'Добавить домен в user.list?'}</strong>
        <div>{pending.action==='ignore'?pending.row.target:pending.row.domain}</div>
        <div class="toolbar"><button disabled={locked} onclick={()=>pending=null}>{rfUi("Отмена","Cancel")}</button><button class="primary" disabled={locked} onclick={confirmAction}>{rfUi("Подтвердить","Confirm")}</button></div>
      </div>
    {/if}
    {#if error}<div class="notice error" role="alert">{error}</div>{/if}
  </article>
  <div class="v2-boundary">{rfUi("Открытие страницы ничего не изменяет. Сканирование — только по кнопке. Ignore и запись в user.list требуют отдельного подтверждения.","Opening this page does not change anything. Scanning is manual. Ignoring an entry or writing to user.list requires separate confirmation.")}</div>
</div>
