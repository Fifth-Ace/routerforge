<script>
  import {onMount} from 'svelte';
  import {registryMemoryRoute,tcp16Projection} from './NfqwsRegistryMemoryModel.js';
  let {active=false,serviceBusy=false,onRequest=async()=>{}}=$props();
  let attempted=$state(false),loading=$state(false),phase=$state('ОЖИДАНИЕ'),error=$state(''),data=$state({count:0,suspected:0,clear:0,workingSni:0,rows:[]});
  let alive=false,epoch=0,abort=null;
  async function load(){if(!alive||!active||loading)return;const n=++epoch,c=new AbortController();abort?.abort();abort=c;loading=true;attempted=true;phase='LOADING';error='';
    try{const result=await onRequest(registryMemoryRoute('/v2/tcp16-memory'),{method:'GET',signal:c.signal});if(!alive||n!==epoch||c.signal.aborted)return;data=tcp16Projection(result);phase='READ ONLY';}
    catch(e){if(alive&&n===epoch&&!c.signal.aborted){phase='ОШИБКА';error=String(e?.message||e);}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  function cancel(){epoch++;abort?.abort();abort=null;loading=false;}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(active&&!attempted&&alive)void load();});
  $effect(()=>{if(!active&&loading)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-tcp16-memory">
  <div class="v2-hero"><div><h2>История сетевых обрывов</h2><div class="section-kicker">наблюдения по обрыву соединения после 16–20 КБ</div></div><span id="v2TCP16State" class="config-badge">{phase}</span></div>
  <article class="panel"><div class="panel-head"><div><h2>Сетевая память</h2><div class="section-kicker">заполняется существующим «Разбором домена»</div></div><button id="v2TCP16Refresh" disabled={loading||serviceBusy} onclick={load}>Обновить</button></div>
    <div class="registry-summary"><div><span>Сетей</span><strong id="v2TCP16Count">{data.count}</strong></div><div><span>Подозрение TCP16</span><strong id="v2TCP16Suspected">{data.suspected}</strong></div><div><span>Чистых</span><strong id="v2TCP16Clear">{data.clear}</strong></div><div><span>Рабочих SNI</span><strong id="v2TCP16SNI">{data.workingSni}</strong></div></div>
  </article>
  <article class="panel" style="margin-top:12px"><div id="v2TCP16List" class="tcp16-memory-list">{#each data.rows as entry,i (i)}<div class="tcp16-memory-row"><div class="tcp16-memory-main"><strong>{entry.network}</strong><span>{entry.lastTarget} · {entry.ipv4}</span></div><div class={`tcp16-memory-state ${entry.suspected?'bad':'good'}`}><strong>{entry.suspected?'TCP16 SUSPECTED':'CLEAR'}</strong><span>{entry.confidence} · suspected {entry.suspectedCount} · clear {entry.clearCount}</span></div><div class="tcp16-memory-sni"><span>Working SNI</span><strong>{entry.sni}</strong></div><div class="tcp16-memory-time"><span>Последнее наблюдение</span><strong>{entry.lastObserved}</strong></div></div>{:else}<div class="analysis-empty">Наблюдений пока нет.</div>{/each}{#if error}<div class="notice error" role="alert">{error}</div>{/if}</div></article>
  <div class="v2-boundary">Только история наблюдений. Рабочая конфигурация NFQWS не изменяется.</div>
</div>
