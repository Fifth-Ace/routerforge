<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {registryMemoryRoute,registryPaths,registryProjection,registryFiltered,registryDetails} from './NfqwsRegistryMemoryModel.js';
  let {active=false,refreshSeq=0,serviceBusy=false,onRequest=async()=>{},onCount=()=>{}}=$props();
  let attempted=$state(false),loading=$state(false),phase=$state(rfUi('ОЖИДАНИЕ',"WAITING")),error=$state('');
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
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){phase=rfUi('ОШИБКА',"ERROR");error=String(e?.message||e);}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  function cancel(){epoch++;abort?.abort();abort=null;loading=false;}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(active&&!attempted&&alive)void load();});
  $effect(()=>{if(refreshSeq>lastRefreshSeq){lastRefreshSeq=refreshSeq;attempted=false;if(active&&alive){if(loading)cancel();void load();}}});
  $effect(()=>{if(!active&&loading)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-strategy-registry">
  <div class="v2-hero"><div><h2>{rfUi("Реестр стратегий","Strategy registry")}</h2><div class="section-kicker">{rfUi("встроенные и пользовательские стратегии в одном списке","built-in and user strategies in one list")}</div></div><span id="v2RegistryState" class="config-badge">{phase}</span></div>
  <article class="panel"><div class="panel-head"><div><h2>{rfUi("База стратегий","Strategy library")}</h2><div class="section-kicker">{rfUi("только просмотр","read only")}</div></div><button id="v2RegistryRefresh" disabled={loading||serviceBusy} onclick={load}>{rfUi("Обновить","Refresh")}</button></div>
    <div class="registry-summary"><div><span>{rfUi("Стратегий","Strategies")}</span><strong id="v2RegistryCount">{data.count}</strong></div><div><span>{rfUi("Источников","Sources")}</span><strong id="v2RegistrySourcesCount">{data.sources.length}</strong></div><div><span>{rfUi("Протоколов","Protocols")}</span><strong id="v2RegistryProtocolsCount">{data.protocols.length}</strong></div><div><span>{rfUi("С историей","With history")}</span><strong id="v2RegistryEvidenceCount">{data.verified}</strong></div><div><span>{rfUi("Готовы к проверке","Ready for testing")}</span><strong id="v2RegistryAutomationReady">{data.automationReady}</strong></div><div><span>{rfUi("Лучшие по истории","Best by history")}</span><strong id="v2RegistryRecommendationTop">{data.recommendationTop}</strong></div></div>
    <div class="registry-filters"><input id="v2RegistrySearch" aria-label={rfUi("Поиск стратегий","Search strategies")} class="search" placeholder={rfUi("Поиск по имени, источнику, семейству или аргументам","Search name, source, family, or arguments")} bind:value={query}><select id="v2RegistryProtocol" aria-label={rfUi("Фильтр протокола","Protocol filter")} class="search" bind:value={protocol}><option value="">{rfUi("Все протоколы","All protocols")}</option>{#each data.protocols as name (name)}<option value={name}>{name}</option>{/each}</select><select id="v2RegistrySource" aria-label={rfUi("Фильтр источника","Source filter")} class="search" bind:value={source}><option value="">{rfUi("Все источники","All sources")}</option>{#each data.sources as name (name)}<option value={name}>{name}</option>{/each}</select></div>
  </article>
  <article class="panel" style="margin-top:12px"><div id="v2RegistryList" class="registry-list">{#each visible as item,i (i)}{@const view=registryDetails(item)}<div class="registry-row"><div class="registry-main"><strong>{item.name||item.id}</strong>{#if item.aliases?.length}<div class="registry-aliases">aliases: {item.aliases.join(' · ')}</div>{/if}<span class="registry-meta">{(item.sources||[]).join(' + ')} · {item.protocol||'—'} · {item.family||'—'}</span><span class="registry-meta">{view.capability} · {view.score}</span><span class={`registry-insight ${view.insightState.toLowerCase()}`}>{view.insightState} · {view.insightSummary}</span><span class={`registry-automation ${view.automationReady?'ready':'blocked'}`}>{view.automationText}</span>{#if view.recommendation}<span class={`registry-recommendation ${view.recommendation.primary?'primary':'hint'}`}>{view.recommendation.text}</span>{/if}</div><div class="registry-evidence"><strong>{view.evidence}</strong><span>{view.provenance}</span></div><details class="registry-args"><summary>Args · {view.fingerprint}</summary><pre>{view.args}</pre></details></div>{:else}<div class="analysis-empty">{rfUi("По текущему фильтру стратегий нет.","No strategies match the current filter.")}</div>{/each}{#if error}<div class="notice error" role="alert">{error}</div>{/if}</div></article>
  <div class="v2-boundary">{rfUi("Реестр ничего не применяет автоматически. Каждая стратегия перед использованием проходит актуальную проверку.","The registry never applies strategies automatically. Each strategy is checked before use.")}</div>
</div>
