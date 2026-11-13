<script>
  import {onMount} from 'svelte';
  import {diagnosticRoute,diagnosticTarget,diagnosticBody,domainProjection,diagnosticTone} from './NfqwsDomainLabModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,onRequest=async()=>{},onSelector=()=>{},onNotice=()=>{}}=$props();
  let target=$state(''),working=$state(false),state=$state('IDLE'),result=$state(null),error=$state('');
  let alive=false,epoch=0,abort=null,lastSeed=0;
  const locked=$derived(serviceBusy||working);
  const tone=diagnosticTone;
  async function run(){
    if(!active||locked||!alive)return;
    let selected;try{selected=diagnosticTarget(target)}catch(e){error=String(e.message);return;}
    const n=++epoch,c=new AbortController();abort?.abort();abort=c;working=true;state='RUNNING';error='';result=null;
    try{
      diagnosticRoute('/v2/site-diagnose','POST');
      const data=await onRequest('/v2/site-diagnose',{method:'POST',body:JSON.stringify(diagnosticBody(selected)),signal:c.signal});
      if(!alive||n!==epoch||c.signal.aborted)return;
      result=domainProjection(data,selected);target=result.target;state=result.state;
      onNotice({text:`Проверка ${result.target}: ${result.classificationText}`,type:result.classification==='clear'?'good':''});
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){error=String(e?.message||e);state='ERROR';onNotice({text:`Проверка домена: ${error}`,type:'error'});}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)working=false;}
  }
  function cancel(){epoch++;abort?.abort();abort=null;working=false;}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(seed?.seq&&seed.seq!==lastSeed){lastSeed=seed.seq;cancel();result=null;state='IDLE';error='';try{target=diagnosticTarget(seed.target)}catch{target='';}}});
  $effect(()=>{if(!active&&working)cancel();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-domain-test">
  <div class="v2-hero"><div><h2>Проверить сайт</h2><div class="section-kicker">проверка доступа → понятная причина → следующее действие</div></div><span id="v2DomainState" class="config-badge">{state}</span></div>
  <div class="v2-input-row"><input id="v2DomainTarget" aria-label="Домен для проверки" class="search" placeholder="youtube.com" autocomplete="off" spellcheck="false" bind:value={target} disabled={locked} onkeydown={e=>{if(e.key==='Enter')void run();}}><button id="v2DomainRun" class="primary" disabled={locked||!target.trim()} onclick={run}>Проверить</button>{#if result?.guidance?.needsSelector}<button id="v2DomainToSelector" disabled={locked} onclick={()=>onSelector(result.target)}> {result.guidance.nextLabel}</button>{/if}</div>
  <article class="panel" id="v2DomainGuidance" role="status" aria-live="polite" style="margin-top:12px">
    {#if result}<div class="v2-result-title">{result.guidance.title}</div><div style="margin-top:8px">{result.guidance.summary}</div>{#if result.guidance.runtimeNotice}<div class="setup-note warn" style="margin-top:10px">{result.guidance.runtimeNotice}</div>{/if}<div class="section-kicker" style="margin-top:10px">{result.guidance.scope}</div>{:else}<div class="analysis-empty">Введите сайт и нажмите «Проверить». RouterForge подскажет, что делать дальше.</div>{/if}
    {#if error}<div class="notice error" role="alert">{error}</div>{/if}
  </article>
  <article id="v2DomainContext" class="panel" role="status" aria-live="polite" style="margin-top:12px">
    {#if result}<div class="v2-result-title">{result.context.title}</div>{#each result.context.rows as row,i (i)}<div class="v2-row" style="margin-top:8px"><div class="v2-result-title">{row[0]}</div><div class="value">{row[1]}</div></div>{/each}{:else}<div class="analysis-empty">Состояние правил и ограничения проверки появятся после диагностики.</div>{/if}
  </article>
  <div id="v2DomainSummary" class="v2-card-grid">{#each result?.summary||[] as item,i (i)}<div class="v2-card"><span>{item[0]}</span><strong>{item[1]}</strong></div>{:else}<div class="panel"><div class="analysis-empty">Рабочие настройки не изменяются.</div></div>{/each}</div>
  <article class="panel"><div class="panel-head"><div><h2>Этапы проверки</h2><div class="section-kicker">только чтение</div></div></div><div id="v2DomainStages" class="v2-stage-list">{#each result?.stages||[] as row (row.name)}<div class={`v2-row ${tone(row.state)}`}><div class="v2-result-title">{row.name.toUpperCase()}</div><div><div class="value">{row.detail}</div><div class="meta">{row.meta}</div></div><span class={`v2-pill ${tone(row.state)}`}>{row.state}</span></div>{:else}<div class="analysis-empty">Проверка ещё не запускалась.</div>{/each}</div></article>
  <div class="v2-two"><article class="panel"><div class="panel-head"><div><h2>Списки</h2><div class="section-kicker">где найден домен</div></div></div><div id="v2DomainLists" class="v2-list">{#each result?.lists||[] as row,i (i)}<div class="v2-row"><div class="v2-result-title">{row.name}</div><div class="meta">{row.path||''} · {row.relation||'present'}</div><span class="v2-pill info">MATCH</span></div>{:else}<div class="analysis-empty">Совпадений нет.</div>{/each}</div></article><article class="panel"><div class="panel-head"><div><h2>Активные правила</h2><div class="section-kicker">текущие профили NFQWS</div></div></div><div id="v2DomainProfiles" class="v2-list">{#each result?.profiles||[] as profile,i (i)}<div class="v2-row"><div class="v2-result-title">Profile {profile.index}</div><div><div class="value">{profile.match_reason||''}</div><div class="v2-args">{Array.isArray(profile.args)?profile.args.join(' '):''}</div></div><span class={`v2-pill ${profile.candidate_ready?'good':'warn'}`}>{profile.candidate_ready?'READY':'CHECK'}</span></div>{:else}<div class="analysis-empty">Production binding не подтверждён.</div>{/each}</div></article></div>
  <div class="v2-boundary">Проверка ничего не меняет. Подбор стратегии — только по явному нажатию.</div>
</div>
