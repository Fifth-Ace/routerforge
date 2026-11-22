<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {registryMemoryRoute,tcp16Projection} from './NfqwsRegistryMemoryModel.js';
  let {active=false,serviceBusy=false,onRequest=async()=>{}}=$props();
  let attempted=$state(false),loading=$state(false),phase=$state(rfUi('ОЖИДАНИЕ',"WAITING")),error=$state(''),data=$state({count:0,suspected:0,clear:0,workingSni:0,rows:[]});
  let alive=false,epoch=0,abort=null;
  async function load(){if(!alive||!active||loading)return;const n=++epoch,c=new AbortController();abort?.abort();abort=c;loading=true;attempted=true;phase='LOADING';error='';
    try{const result=await onRequest(registryMemoryRoute('/v2/tcp16-memory'),{method:'GET',signal:c.signal});if(!alive||n!==epoch||c.signal.aborted)return;data=tcp16Projection(result);phase='READ ONLY';}
    catch(e){if(alive&&n===epoch&&!c.signal.aborted){phase=rfUi('ОШИБКА',"ERROR");error=String(e?.message||e);}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  function cancel(){epoch++;abort?.abort();abort=null;loading=false;}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(active&&!attempted&&alive)void load();});
  $effect(()=>{if(!active&&loading)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-tcp16-memory">
  <div class="v2-hero"><div><h2>{rfUi("История сетевых обрывов","Connection interruption history")}</h2><div class="section-kicker">{rfUi("наблюдения по обрыву соединения после 16–20 КБ","observations of connections interrupted after 16–20 KB")}</div></div><span id="v2TCP16State" class="config-badge">{phase}</span></div>
  <article class="panel"><div class="panel-head"><div><h2>{rfUi("Сетевая память","Network history")}</h2><div class="section-kicker">{rfUi("заполняется существующим «Разбором домена»","populated by the existing Domain Analysis")}</div></div><button id="v2TCP16Refresh" disabled={loading||serviceBusy} onclick={load}>{rfUi("Обновить","Refresh")}</button></div>
    <div class="registry-summary"><div><span>{rfUi("Сетей","Networks")}</span><strong id="v2TCP16Count">{data.count}</strong></div><div><span>{rfUi("Подозрение TCP16","Suspected TCP16 issue")}</span><strong id="v2TCP16Suspected">{data.suspected}</strong></div><div><span>{rfUi("Чистых","Clean")}</span><strong id="v2TCP16Clear">{data.clear}</strong></div><div><span>{rfUi("Рабочих SNI","Working SNI")}</span><strong id="v2TCP16SNI">{data.workingSni}</strong></div></div>
  </article>
  <article class="panel" style="margin-top:12px"><div id="v2TCP16List" class="tcp16-memory-list">{#each data.rows as entry,i (i)}<div class="tcp16-memory-row"><div class="tcp16-memory-main"><strong>{entry.network}</strong><span>{entry.lastTarget} · {entry.ipv4}</span></div><div class={`tcp16-memory-state ${entry.suspected?'bad':'good'}`}><strong>{entry.suspected?'TCP16 SUSPECTED':'CLEAR'}</strong><span>{entry.confidence} · suspected {entry.suspectedCount} · clear {entry.clearCount}</span></div><div class="tcp16-memory-sni"><span>Working SNI</span><strong>{entry.sni}</strong></div><div class="tcp16-memory-time"><span>{rfUi("Последнее наблюдение","Latest observation")}</span><strong>{entry.lastObserved}</strong></div></div>{:else}<div class="analysis-empty">{rfUi("Наблюдений пока нет.","No observations yet.")}</div>{/each}{#if error}<div class="notice error" role="alert">{error}</div>{/if}</div></article>
  <div class="v2-boundary">{rfUi("Только история наблюдений. Рабочая конфигурация NFQWS не изменяется.","Observation history only. Active NFQWS configuration remains unchanged.")}</div>
</div>
