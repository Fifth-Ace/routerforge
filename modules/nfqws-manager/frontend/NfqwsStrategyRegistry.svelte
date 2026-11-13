<script>
  import {onMount} from 'svelte';
  import {registryMemoryRoute,registryPaths,registryProjection,registryFiltered,registryDetails} from './NfqwsRegistryMemoryModel.js';
  let {active=false,refreshSeq=0,serviceBusy=false,onRequest=async()=>{},onCount=()=>{}}=$props();
  let attempted=$state(false),loading=$state(false),phase=$state('ОЖИДАНИЕ'),error=$state('');
  let data=$state({entries:[],sources:[],protocols:[],count:0,verified:0,automationReady:0,recommendationTop:0});
  let query=$state(''),protocol=$state(''),source=$state('');
  let alive=false,epoch=0,abort=null,lastRefreshSeq=0;
  const visible=$derived(registryFiltered(data,query,protocol,source));
  async function load(){
    if(!alive||!active||loading)return;
    const n=++epoch,c=new AbortController();abort?.abort();abort=c;loading=true;attempted=true;phase='LOADING';error='';
    try{
      const result=await Promise.all(registryPaths().map(path=>onRequest(registryMemoryRoute(path),{method:'GET',signal:c.signal})));
      if(!alive||n!==epoch||c.signal.aborted)return;
      data=registryProjection(...result);phase='READ ONLY';onCount(data.count);
      if(protocol&&!data.protocols.includes(protocol))protocol='';if(source&&!data.sources.includes(source))source='';
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){phase='ОШИБКА';error=String(e?.message||e);}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  function cancel(){epoch++;abort?.abort();abort=null;loading=false;}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(active&&!attempted&&alive)void load();});
  $effect(()=>{if(refreshSeq>lastRefreshSeq){lastRefreshSeq=refreshSeq;attempted=false;if(active&&alive){if(loading)cancel();void load();}}});
  $effect(()=>{if(!active&&loading)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-strategy-registry">
  <div class="v2-hero"><div><h2>Реестр стратегий</h2><div class="section-kicker">встроенные и пользовательские стратегии в одном списке</div></div><span id="v2RegistryState" class="config-badge">{phase}</span></div>
  <article class="panel"><div class="panel-head"><div><h2>База стратегий</h2><div class="section-kicker">только просмотр</div></div><button id="v2RegistryRefresh" disabled={loading||serviceBusy} onclick={load}>Обновить</button></div>
    <div class="registry-summary"><div><span>Стратегий</span><strong id="v2RegistryCount">{data.count}</strong></div><div><span>Источников</span><strong id="v2RegistrySourcesCount">{data.sources.length}</strong></div><div><span>Протоколов</span><strong id="v2RegistryProtocolsCount">{data.protocols.length}</strong></div><div><span>С историей</span><strong id="v2RegistryEvidenceCount">{data.verified}</strong></div><div><span>Готовы к проверке</span><strong id="v2RegistryAutomationReady">{data.automationReady}</strong></div><div><span>Лучшие по истории</span><strong id="v2RegistryRecommendationTop">{data.recommendationTop}</strong></div></div>
    <div class="registry-filters"><input id="v2RegistrySearch" aria-label="Поиск стратегий" class="search" placeholder="Поиск по имени, источнику, семейству или аргументам" bind:value={query}><select id="v2RegistryProtocol" aria-label="Фильтр протокола" class="search" bind:value={protocol}><option value="">Все протоколы</option>{#each data.protocols as name (name)}<option value={name}>{name}</option>{/each}</select><select id="v2RegistrySource" aria-label="Фильтр источника" class="search" bind:value={source}><option value="">Все источники</option>{#each data.sources as name (name)}<option value={name}>{name}</option>{/each}</select></div>
  </article>
  <article class="panel" style="margin-top:12px"><div id="v2RegistryList" class="registry-list">{#each visible as item,i (i)}{@const view=registryDetails(item)}<div class="registry-row"><div class="registry-main"><strong>{item.name||item.id}</strong>{#if item.aliases?.length}<div class="registry-aliases">aliases: {item.aliases.join(' · ')}</div>{/if}<span class="registry-meta">{(item.sources||[]).join(' + ')} · {item.protocol||'—'} · {item.family||'—'}</span><span class="registry-meta">{view.capability} · {view.score}</span><span class={`registry-insight ${view.insightState.toLowerCase()}`}>{view.insightState} · {view.insightSummary}</span><span class={`registry-automation ${view.automationReady?'ready':'blocked'}`}>{view.automationText}</span>{#if view.recommendation}<span class={`registry-recommendation ${view.recommendation.primary?'primary':'hint'}`}>{view.recommendation.text}</span>{/if}</div><div class="registry-evidence"><strong>{view.evidence}</strong><span>{view.provenance}</span></div><details class="registry-args"><summary>Args · {view.fingerprint}</summary><pre>{view.args}</pre></details></div>{:else}<div class="analysis-empty">По текущему фильтру стратегий нет.</div>{/each}{#if error}<div class="notice error" role="alert">{error}</div>{/if}</div></article>
  <div class="v2-boundary">Реестр ничего не применяет автоматически. Каждая стратегия перед использованием проходит актуальную проверку.</div>
</div>
