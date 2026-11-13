<script>
  import {onMount} from 'svelte';
  import {sandboxPath,sandboxTargetBody,sandboxSafeTargets,sandboxBenchBody,sandboxSaveBody,sandboxResult,sandboxArgsText} from './NfqwsSandboxModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,onRequest=async()=>{},onRefreshLibrary=async()=>{},onHistory=()=>{},onNotice=()=>{}}=$props();
  let mounted=$state(false), loaded=$state(false),attempted=$state(false),loading=$state(false),saving=$state(false),running=$state(false);
  let state=$state('ОЖИДАНИЕ'),error=$state(''),results=$state([]),targetSources=$state([]),profiles=$state([]),library=$state([]),server=$state(null);
  let targetSource=$state('manual'),listName=$state(''),targetsText=$state(''),candidateSource=$state('production'),profileIndex=$state(''),libraryId=$state(''),argsText=$state(''),attempts=$state('2'),saveName=$state(''),seedMeta=$state('выберите источник');
  let alive=false,epoch=0,runAbort=null,saveAbort=null,lastSeed=0;
  const locked=$derived(serviceBusy||loading||running||saving);
  const selected=$derived(candidateSource==='production'?profiles.find(p=>String(p.index)===String(profileIndex)):candidateSource==='library'?library.find(x=>x.id===libraryId):null);
  const visibleArgs=$derived(candidateSource==='custom'?argsText:sandboxArgsText(selected?.args||[]));
  function notify(text,type=''){if(alive)onNotice({text:String(text),type});}
  function safeError(e){return String(e?.message||e||'NFQWS_SANDBOX_UNKNOWN');}
  function request(path,method='GET',body,signal){sandboxPath(path,method);return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});}
  function view(){return{targetSource,listName,targetsText,candidateSource,profileIndex,libraryId,argsText,attempts,saveName};}
  function selectSource(value){if(running||saving)return;candidateSource=value;error='';}
  async function load(){
    if(!alive||!active||loading||running)return;
    const n=++epoch;loading=true;attempted=true;error='';
    try{
      const [sourceResult,profileResult,libraryResult,status]=await Promise.all([
        request('/v2/target-sources'),request('/bench-strategies'),request('/v2/strategies'),request('/status')]);
      if(!alive||n!==epoch)return;
      if(!Array.isArray(sourceResult?.sources)||!Array.isArray(profileResult?.profiles)||!Array.isArray(libraryResult?.strategies))throw Error('NFQWS_SANDBOX_INVENTORY_INVALID');
      targetSources=sourceResult.sources.filter(x=>typeof x?.name==='string');
      profiles=profileResult.profiles.filter(x=>x?.candidate_eligible);
      library=libraryResult.strategies.filter(x=>x&&typeof x.id==='string'&&Array.isArray(x.args));
      server=status;loaded=true;
      if(!targetSources.some(x=>x.name===listName))listName=targetSources[0]?.name||'';
      if(!profiles.some(x=>String(x.index)===String(profileIndex)))profileIndex=String(profiles[0]?.index??'');
      if(!library.some(x=>x.id===libraryId))libraryId=library[0]?.id||'';
    }catch(e){if(alive&&n===epoch){error=safeError(e);state='ОШИБКА';notify(error,'error');}}
    finally{if(alive&&n===epoch)loading=false;}
  }
  function acceptSeed(){
    if(!seed?.seq||seed.seq===lastSeed)return;
    lastSeed=seed.seq;
    if(seed.kind==='target'){
      targetSource='manual';targetsText=String(seed.target||'');seedMeta='Цель передана из наблюдений';
    }else if(seed.kind==='candidate'){
      candidateSource='custom';argsText=String(seed.args||'');saveName=String(seed.name||'');seedMeta=`${String(seed.source||'custom')} · импортировано в Sandbox`;
    }
  }
  function stop(){if(!runAbort)return;runAbort.abort();state='ОСТАНОВЛЕНО';}
  async function run(){
    if(!alive||!active||locked)return;
    const controller=new AbortController();runAbort=controller;running=true;state='RUNNING';results=[];error='';const n=++epoch;
    try{
      const targetsResponse=await request('/v2/targets/resolve','POST',sandboxTargetBody(view(),targetSources),controller.signal);
      const targets=sandboxSafeTargets(targetsResponse);
      if(!targets.length)throw Error('NFQWS_SANDBOX_NO_TARGETS');
      if(!alive||n!==epoch||controller.signal.aborted)return;
      targetsText=targets.join('\n');
      // Re-read exact config SHA immediately before constructing the candidate bench operation.
      server=await request('/status','GET',undefined,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return;
      const body=sandboxBenchBody(view(),{profiles,library,status:server});
      for(const target of targets){
        if(!alive||n!==epoch||controller.signal.aborted)break;
        try{
          const data=await request('/v2/bench','POST',{target,...body},controller.signal);
          if(!alive||n!==epoch||controller.signal.aborted)break;
          results=[...results,{target,data,view:sandboxResult(data,target)}];
        }catch(e){
          if(controller.signal.aborted||!alive||n!==epoch)break;
          results=[...results,{target,error:safeError(e)}];
        }
      }
      if(!alive||n!==epoch||controller.signal.aborted)return;
      const failures=results.filter(x=>x.error).length;
      state=failures?'PARTIAL':'DONE';
      const successes=results.filter(x=>x.data?.result?.success_rate>0).length;
      onHistory('v2-sandbox',{targets:results.length,success_ratio:results.length?successes/results.length:0},results.map(({target,data,error})=>({target,...(data?{data}:{error})})));
      notify(`Sandbox: завершено ${results.length}/${targets.length}; ошибок ${failures}. Production не менялся.`,failures?'error':'good');
    }catch(e){if(alive&&n===epoch&&!controller.signal.aborted){error=safeError(e);state='ОШИБКА';notify(error,'error');}}
    finally{if(runAbort===controller)runAbort=null;if(alive&&n===epoch){running=false;if(controller.signal.aborted)state='ОСТАНОВЛЕНО';}}
  }
  async function save(){
    if(!alive||!active||locked)return;
    let body;try{body=sandboxSaveBody(view(),{profiles,library});}catch(e){error=safeError(e);notify(error,'error');return;}
    const controller=new AbortController();saveAbort=controller;saving=true;error='';const n=++epoch;
    try{
      const result=await request('/v2/strategies/save','POST',body,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return;
      if(!result?.strategy?.id||!result.strategy.name)throw Error('NFQWS_SANDBOX_SAVE_UNCONFIRMED');
      await onRefreshLibrary();
      if(!alive||n!==epoch||controller.signal.aborted)return;
      const refreshed=await request('/v2/strategies','GET',undefined,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return;
      if(!Array.isArray(refreshed?.strategies))throw Error('NFQWS_SANDBOX_LIBRARY_REFRESH_INVALID');
      library=refreshed.strategies;notify(`Candidate сохранён: ${result.strategy.name}`,'good');
    }catch(e){if(alive&&n===epoch&&!controller.signal.aborted){error=safeError(e);notify(error,'error');}}
    finally{if(saveAbort===controller)saveAbort=null;if(alive&&n===epoch)saving=false;}
  }
  onMount(()=>{alive=true;mounted=true;return()=>{alive=false;++epoch;runAbort?.abort();saveAbort?.abort();};});
  $effect(()=>{if(mounted&&active&&!attempted&&!loading)void load();});
  $effect(()=>{if(mounted&&seed?.seq)acceptSeed();});
  // Leaving Sandbox stops any unobserved benchmark; unmount aborts all requests.
  $effect(()=>{if(mounted&&!active&&runAbort)stop();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-sandbox-v2">
  <div class="v2-hero"><div><h2>Песочница стратегий</h2><div class="section-kicker">изолированная проверка без изменения рабочей конфигурации</div></div><span id="v2SandboxState" class="config-badge">{state}</span></div>
  <div class="toolbar"><button type="button" disabled={locked} onclick={load}>Обновить источники</button><span class="section-kicker">{loading?'Загрузка…':seedMeta}</span></div>
  <div class="v2-two">
    <article class="panel">
      <div class="panel-head"><div><h2>Цели проверки</h2><div class="section-kicker">ручной ввод или список доменов</div></div></div>
      <label class="v2-label">Источник целей<select id="v2SandboxTargetSource" class="search" bind:value={targetSource} disabled={locked}><option value="manual">Ручной ввод</option><option value="list">Список</option></select></label>
      <label class="v2-label">Список<select id="v2SandboxTargetList" class="search" bind:value={listName} disabled={locked||targetSource!=='list'}>{#each targetSources as entry (entry.name)}<option value={entry.name}>{entry.name} · {entry.count||0}</option>{:else}<option value="">Нет списков</option>{/each}</select></label>
      <label class="v2-label">Цели<textarea id="v2SandboxTargets" class="v2-textarea compact" placeholder="youtube.com" bind:value={targetsText} disabled={locked||targetSource==='list'}></textarea></label>
      <div class="panel-head" style="margin-top:12px"><div><h2>Стратегия</h2><div id="v2SandboxSeedMeta" class="section-kicker">{seedMeta}</div></div></div>
      <label class="v2-label">Источник стратегии<select id="v2SandboxCandidateSource" class="search" value={candidateSource} onchange={e=>selectSource(e.currentTarget.value)} disabled={locked}><option value="production">Профиль nfqws2</option><option value="library">Библиотека стратегий</option><option value="custom">Свои аргументы</option></select></label>
      <label class="v2-label">Профиль nfqws2<select id="v2SandboxProfile" class="search" bind:value={profileIndex} disabled={locked||candidateSource!=='production'}>{#each profiles as profile (profile.index)}<option value={String(profile.index)}>Profile {profile.index}{profile.strategy_tags?.length?` · tags ${profile.strategy_tags.join(',')}`:''}</option>{:else}<option value="">Нет допустимых профилей</option>{/each}</select></label>
      <label class="v2-label">Сохранённая стратегия<select id="v2SandboxLibrary" class="search" bind:value={libraryId} disabled={locked||candidateSource!=='library'}>{#each library as candidate (candidate.id)}<option value={candidate.id}>{candidate.name} · {candidate.source}</option>{:else}<option value="">Library пуста</option>{/each}</select></label>
      <label class="v2-label">Аргументы<textarea id="v2SandboxArgs" class="v2-textarea compact" placeholder="Аргументы nfqws2 для проверки" value={visibleArgs} oninput={e=>{if(candidateSource==='custom')argsText=e.currentTarget.value;}} disabled={locked||candidateSource!=='custom'}></textarea></label>
      <div class="v2-inline"><label>Попытки<select id="v2SandboxAttempts" class="search" bind:value={attempts} disabled={locked}><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="5">5</option></select></label><label>Название<input id="v2SandboxSaveName" class="search" placeholder="Моя стратегия" bind:value={saveName} disabled={locked}></label></div>
      <div class="toolbar"><button id="v2SandboxRun" class="primary" type="button" disabled={locked||!loaded} onclick={run}>Запустить проверку</button><button id="v2SandboxStop" type="button" disabled={!running} onclick={stop}>Стоп</button><button id="v2SandboxSave" type="button" disabled={locked||!loaded} onclick={save}>Сохранить стратегию</button></div>
    </article>
    <article class="panel"><div id="v2SandboxResults" class="v2-list">
      {#each results as item, i (i)}
        {#if item.error}<div class="v2-row bad"><div class="v2-result-title">{item.target}</div><div class="value">{item.error}</div><span class="v2-pill bad">ERROR</span></div>
        {:else}<div class="v2-row {item.view.tone}"><div><div class="v2-result-title">{item.view.target}</div><div class="meta">{item.view.source}</div></div><div><div class="value">{item.view.status} · success {item.view.success}% · complete {item.view.complete}%</div><div class="meta">infra={item.view.infrastructure?'PASS':'FAIL'} · cleanup={item.view.cleanup?'PASS':'FAIL'} · packets={item.view.packets} · TTFB={item.view.ttfb} ms</div><div class="v2-args">{item.view.args}</div></div><span class="v2-pill {item.view.tone}">{item.view.status}</span></div>{/if}
      {:else}<div class="analysis-empty">{running?'Тестирование выполняется…':loaded?'Результатов пока нет.':'Источники ещё не загружены.'}</div>{/each}
    </div></article>
  </div>
  {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  <div class="v2-boundary">Bench проверяет кандидат через изолированный механизм. Работающий nfqws2.conf не изменяется. Сохранение candidate выполняется отдельно, по кнопке.</div>
</div>
