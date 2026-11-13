<script>
  import {onMount} from 'svelte';
  import {selectorPath,selectorProgressPath,selectorRunBody,selectorProgress,selectorTone,selectorSource,selectorReason,selectorResultLabel,selectorWorking,selectorProfileQuality,selectorPreviewBody,selectorBindBody,selectorApplyBody,selectorCandidateMetrics} from './NfqwsSelectorModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,onRequest=async()=>{},onHistory=()=>{},onNotice=()=>{},onApplied=()=>{},onRegister=()=>{}}=$props();
  let mounted=$state(false),loading=$state(false),loaded=$state(false),attempted=$state(false),running=$state(false),binding=$state(false),previewing=$state(false),applying=$state(false);
  let state=$state('ОЖИДАНИЕ'),error=$state(''),target=$state(''),mode=$state('normal'),concurrency=$state('2'),production=$state(true),autoPool=$state(true),libraryEnabled=$state(true),community=$state(false),asn=$state(''),extraName=$state(''),extraArgs=$state('');
  let session=$state('—'),phase=$state('ОЖИДАНИЕ'),progressLabel=$state('0 / 0'),progressPercent=$state(0),progressText=$state('Готов к запуску.'),workers=$state('—'),cleanup=$state('—'),poolMeta=$state('RouterForge проверяет стратегии из встроенного пула реальными HTTPS-запросами.');
  let library=$state([]),result=$state(null),previewData=$state(null),previewText=$state('Предпросмотр ещё не построен.'),bindID=$state(''),conflicts=$state([]),conflictError=$state(''),preparedText=$state('');
  let alive=false,epoch=0,runAbort=null,pollTimer=0,previewAbort=null,applyAbort=null,bindAbort=null,lastSeed=0,lastErrorText='';
  const locked=$derived(serviceBusy||running||loading||binding||previewing||applying);
  const working=$derived(selectorWorking(result));
  const bindVisible=$derived(!!result?.recommendation_available&&!result.apply_gate_eligible&&working.length>0);
  const bindCandidate=$derived(working.find(c=>String(c.candidate_id)===bindID)||working[0]);
  const applyVisible=$derived(!!result?.recommendation_available);
  const poolSummary=$derived(result?.auto_pool_enabled?`Проверено стратегий: ${(result.candidates||[]).length}.`:'Встроенный пул отключён.');
  function notify(text,type=''){if(alive)onNotice({text:String(text),type});}
  function errorText(e){return String(e?.message||e||'NFQWS_SELECTOR_UNKNOWN');}
  function request(path,method='GET',body,signal){
    if(!alive)throw Error('NFQWS_SELECTOR_UNMOUNTED');
    if(path.startsWith('/v2/selector-progress?session='))selectorProgressPath(path.split('session=')[1]);
    else selectorPath(path,method);
    return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});
  }
  function sessionID(){const b=new Uint8Array(8);if(!globalThis.crypto?.getRandomValues)throw Error('NFQWS_SELECTOR_CRYPTO_UNAVAILABLE');globalThis.crypto.getRandomValues(b);return'ui-'+Array.from(b,x=>x.toString(16).padStart(2,'0')).join('');}
  function view(override={}){return{target,mode,concurrency,production,autoPool,libraryEnabled,community,asn,extraName,extraArgs,...override};}
  function stop(){if(runAbort){runAbort.abort();state='ОСТАНОВЛЕНО';}clearInterval(pollTimer);pollTimer=0;}
  function invalidate(){stop();++epoch;previewAbort?.abort();applyAbort?.abort();bindAbort?.abort();result=null;previewData=null;conflicts=[];bindID='';previewText='Предпросмотр ещё не построен.';preparedText='';}
  function acceptSeed(){
    if(!seed?.seq||seed.seq===lastSeed)return;
    lastSeed=seed.seq;invalidate();target=String(seed.target||'').trim();state='ОЖИДАНИЕ';error='';
  }
  async function load(){
    if(!alive||!active||loading||running)return;
    loading=true;attempted=true;const n=epoch;
    try{
      const data=await request('/v2/strategies');
      if(!alive||n!==epoch)return;
      if(!Array.isArray(data?.strategies))throw Error('NFQWS_SELECTOR_LIBRARY_INVALID');
      library=data.strategies.filter(c=>c&&typeof c.id==='string'&&Array.isArray(c.args));loaded=true;
    }catch(e){if(alive&&n===epoch){error=errorText(e);notify(error,'error');}}
    finally{if(alive&&n===epoch)loading=false;}
  }
  function updatePoolWarnings(d,requested){
    const warnings=Array.isArray(d?.pool_warnings)?d.pool_warnings:[];
    const memory=warnings.find(x=>String(x).startsWith('NF3B1_MEMORY_BUDGET:'));
    const confirmation=warnings.find(x=>String(x).startsWith('NF3B2_CONFIRM:'));
    const communityWarning=warnings.find(x=>String(x).startsWith('NF3C_COMMUNITY:'));
    poolMeta=[memory?`Воркеры: запрошено ${requested}, запущено ${Number(d.concurrency||0)}. ${String(memory).replace('NF3B1_MEMORY_BUDGET: ','')}`:poolMeta,
      confirmation?String(confirmation).replace('NF3B2_CONFIRM: ','Подтверждение: '):'',communityWarning?String(communityWarning).replace('NF3C_COMMUNITY: ','Сообщество: '):''].filter(Boolean).join(' · ');
  }
  function installResult(d,n,requested){
    if(!alive||n!==epoch)return;
    if(!d||!Array.isArray(d.candidates)||typeof d.recommendation_available!=='boolean')throw Error('NFQWS_SELECTOR_RESULT_INVALID');
    result=d;previewData=null;previewText=d.apply_gate_eligible?'Рабочая стратегия готова. Сначала откройте предпросмотр изменений.':'Для этой стратегии автоматическое применение пока недоступно.';
    cleanup=d.cleanup_baseline_after?'Готово':'Ошибка';workers=String(d.concurrency||'—');state='DONE';progressPercent=100;error='';
    updatePoolWarnings(d,requested);
    const all=d.candidates,ok=all.filter(c=>c.result_class==='WORKING').length;
    onHistory('v2-selector',{targets:1,candidates:all.length,working:ok,success_ratio:all.length?ok/all.length:0,recommendation:!!d.recommendation_available},d);
    if(d.recommendation_available&&!d.apply_gate_eligible&&d.session_id){
      bindID=String(working.find(c=>String(c.candidate_id)===String(d.recommended_candidate_id))?.candidate_id||working[0]?.candidate_id||'');
      void inspectConflicts(d,n);
    }
    notify(`Автоподбор завершён: ${d.recommendation_reason||'готово'}`,'good');
  }
  async function inspectConflicts(d,n){
    try{const inspected=await request('/v2/inspect-target','POST',{target:d.server_name});if(alive&&epoch===n&&String(result?.session_id||'')===String(d?.session_id||'')){conflicts=(Array.isArray(inspected?.production_profile_matches)?inspected.production_profile_matches:[]).slice().sort((a,b)=>selectorProfileQuality(b).rank-selectorProfileQuality(a).rank||Number(a.index)-Number(b.index));conflictError='';}}
    catch(e){if(alive&&epoch===n){conflictError=errorText(e);conflicts=[];}}
  }
  function poll(sessionID,signal,n,started,chosenMode){
    clearInterval(pollTimer);
    let pending=false;
    async function tick(){
      if(pending||signal.aborted||!alive||epoch!==n)return;
      pending=true;
      try{
        const p=await request(selectorProgressPath(sessionID),'GET',undefined,signal);
        if(!alive||epoch!==n||signal.aborted)return;
        const prog=selectorProgress(p,chosenMode);
        phase=String(p.phase||'').toUpperCase()||'ОЖИДАНИЕ';progressLabel=prog.label;progressPercent=phase==='BASELINE'?5:Math.max(0,Math.min(100,prog.percent));
        progressText=`${String(p.message||'')} · ${Math.floor((Date.now()-started)/1000)} с`;
        if(p.done){clearInterval(pollTimer);pollTimer=0;}
      }catch(e){if(signal.aborted)return;/* A temporary progress error cannot be used as a result. */}
      finally{pending=false;}
    }
    void tick();pollTimer=setInterval(()=>{void tick();},900);
  }
  async function execute(override={},external=false){
    if(!alive||runAbort||running||applying||binding||previewing||(!active&&!external))return null;
    if(serviceBusy){lastErrorText='NFQWS_SELECTOR_SERVICE_BUSY';return null;}
    lastErrorText='';error='';
    const controller=new AbortController();runAbort=controller;running=true;state='ВЫПОЛНЯЕТСЯ';session='—';phase='ПОДГОТОВКА';progressLabel='0 / 0';progressPercent=0;progressText='Подготовка запуска · 0 с';
    invalidatePrevious();
    const n=epoch;
    try{
      const [status,stored]=await Promise.all([request('/status','GET',undefined,controller.signal),request('/v2/strategies','GET',undefined,controller.signal)]);
      if(!alive||n!==epoch||controller.signal.aborted)return null;
      if(!Array.isArray(stored?.strategies))throw Error('NFQWS_SELECTOR_LIBRARY_INVALID');
      library=stored.strategies;loaded=true;
      const sessionValue=sessionID(),payload=selectorRunBody(view(override),library,status?.config_sha256,sessionValue);
      session=sessionValue;poolMeta=payload.auto_pool?({fast:'Быстрый: 30% пула',normal:'Обычный: по 20 до двух рабочих',thorough:'Полный: весь пул'})[payload.mode]+' · Проверка выполняется реальными HTTPS-запросами.':'Встроенный пул отключён.';
      poll(sessionValue,controller.signal,n,Date.now(),payload.mode);
      const data=await request('/v2/selector','POST',payload,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return null;
      installResult(data,n,payload.concurrency);return data;
    }catch(e){
      if(alive&&n===epoch){lastErrorText=controller.signal.aborted?'Автоподбор остановлен.':errorText(e);state=controller.signal.aborted?'ОСТАНОВЛЕНО':'ОШИБКА';error=lastErrorText;if(!external)notify(lastErrorText,'error');}
      return null;
    }finally{
      clearInterval(pollTimer);pollTimer=0;
      if(runAbort===controller)runAbort=null;
      if(alive&&n===epoch)running=false;
    }
  }
  function invalidatePrevious(){++epoch;previewAbort?.abort();applyAbort?.abort();bindAbort?.abort();result=null;previewData=null;conflicts=[];bindID='';previewText='Предпросмотр ещё не построен.';preparedText='';}
  async function bindCandidateToProfile(){
    if(!alive||!result||!bindVisible||locked)return null;
    const n=epoch,controller=new AbortController();bindAbort=controller;binding=true;error='';
    try{
      const status=await request('/status','GET',undefined,controller.signal);
      if(!alive||epoch!==n||controller.signal.aborted)return null;
      const body=selectorBindBody(result,bindID,status?.config_sha256);
      const response=await request('/v2/selector-bind','POST',body,controller.signal);
      if(!alive||epoch!==n||controller.signal.aborted)return null;
      if(!response?.apply_gate_eligible||!response?.apply_gate_token)throw Error('NFQWS_SELECTOR_BIND_UNCONFIRMED');
      result={...result,apply_gate_eligible:true,apply_gate_token:response.apply_gate_token,apply_gate_expires_at:response.apply_gate_expires_at||'',apply_gate_reason:response.apply_gate_reason||''};
      preparedText=`Профиль подготовлен после повторной проверки (${Number(response.verification_successes||0)}/${Number(response.verification_attempts||0)} успешных).`;
      previewText=preparedText+' Откройте предпросмотр перед применением.';
      notify('Выбранная стратегия подтверждена. Новый профиль готов к предпросмотру.','good');return response;
    }catch(e){if(alive&&n===epoch&&!controller.signal.aborted){error=errorText(e);notify(error,'error');}return null;}
    finally{if(bindAbort===controller)bindAbort=null;if(alive&&n===epoch)binding=false;}
  }
  async function preview(external=false){
    if(!alive||!result?.apply_gate_eligible||!result?.apply_gate_token||locked||(!active&&!external))return null;
    const controller=new AbortController();previewAbort=controller;previewing=true;lastErrorText='';error='';const n=epoch;
    try{
      const status=await request('/status','GET',undefined,controller.signal);
      const body=selectorPreviewBody(result,status?.config_sha256);
      const p=await request('/bench-autotune/apply-preview','POST',body,controller.signal);
      if(!alive||epoch!==n||controller.signal.aborted)return null;
      if(!p?.ok||!p.preview_only)throw Error('NFQWS_SELECTOR_PREVIEW_CONTRACT_INVALID');
      previewData=p;
      previewText=[`Источник: ${selectorSource(p.candidate_source||'production')}`,`Транспорт: ${p.transport||'https'}`,`Сессия: ${p.session_id||'—'}`,`Отпечаток: ${p.candidate_fingerprint||'—'}`,`Ресурсы: ${(p.required_resources||[]).join(', ')||'нет'}`,
        p.placement_kind==='existing_list'?`Список: ${p.placement_list}\nБыло SHA: ${p.placement_list_sha_before}\nБудет SHA: ${p.placement_list_sha_after}\nИзменение: ${p.placement_diff}\nКонфигурация nfqws2 остаётся прежней.`:'',String(p.candidate_config||'')].filter(Boolean).join('\n\n');
      notify(`Предпросмотр готов · ${String(p.candidate_config_sha256||'').slice(0,16)}…`,'good');return p;
    }catch(e){if(alive&&epoch===n&&!controller.signal.aborted){lastErrorText=errorText(e);error=lastErrorText;previewData=null;notify('Предпросмотр: '+lastErrorText,'error');}return null;}
    finally{if(previewAbort===controller)previewAbort=null;if(alive&&epoch===n)previewing=false;}
  }
  async function apply(external=false){
    if(!alive||!previewData?.changed||!result?.apply_gate_token||locked||(!active&&!external))return null;
    const p=previewData;
    if(!globalThis.confirm(`Применить найденную стратегию?\nИзменение: ${p.placement_kind==='existing_list'?'добавление сайта в '+p.placement_list:'профиль nfqws2'}\nДомен: ${p.server_name}\nИсточник: ${selectorSource(p.candidate_source||'production')}\nОтпечаток: ${p.candidate_fingerprint||'—'}\nТекущая конфигурация: ${p.active_config_sha256}\nНовая конфигурация: ${p.candidate_config_sha256}\n\nБудет создана резервная копия и выполнена проверка результата.`))return null;
    const controller=new AbortController();applyAbort=controller;applying=true;lastErrorText='';error='';const n=epoch;
    try{
      const latest=await request('/status','GET',undefined,controller.signal);
      const body=selectorApplyBody(result,p,latest);
      const response=await request('/bench-autotune/apply','POST',body,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return null;
      if(response?.rolled_back||!response?.post_apply_verified)throw Error('NFQWS_SELECTOR_APPLY_NOT_VERIFIED');
      const status=response.status||await request('/status','GET',undefined,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return null;
      if(status?.config_sha256&&response?.config_sha256&&status.config_sha256!==response.config_sha256)throw Error('NFQWS_SELECTOR_APPLY_STATUS_MISMATCH');
      onApplied(status);result=null;previewData=null;previewText='Применено и проверено.';state='ПРИМЕНЕНО';
      notify(`Применено: production проверен; других сайтов: ${Number(response.regression_guards_verified||0)}.`,'good');return response;
    }catch(e){if(alive&&epoch===n&&!controller.signal.aborted){lastErrorText=errorText(e);error=lastErrorText;previewData=null;result=null;notify('Safe Apply: '+lastErrorText,'error');}return null;}
    finally{if(applyAbort===controller)applyAbort=null;if(alive&&epoch===n)applying=false;}
  }
  function clearPreview(){previewAbort?.abort();applyAbort?.abort();previewData=null;previewText='Предпросмотр ещё не построен.';}
  onMount(()=>{
    alive=true;mounted=true;onRegister({runExternal:spec=>execute(spec,true),previewExternal:()=>preview(true),applyExternal:()=>apply(true),stopExternal:stop,clearPreview,errorMessage:()=>lastErrorText});
    return()=>{onRegister(null);alive=false;stop();++epoch;previewAbort?.abort();applyAbort?.abort();bindAbort?.abort();};
  });
  $effect(()=>{if(mounted&&active&&!attempted&&!loading)void load();});
  $effect(()=>{if(mounted&&seed?.seq)acceptSeed();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-selector-v2">
  <div class="v2-hero"><div><h2>Автоподбор стратегии</h2><div class="section-kicker">baseline → изолированная проверка → результат → предпросмотр → ручное применение</div></div><span id="v2SelectorState" class="config-badge">{state}</span></div>
  <div class="v2-two">
    <article class="panel">
      <label class="v2-label">Цель<input id="v2SelectorTarget" class="search" placeholder="youtube.com" autocomplete="off" bind:value={target} disabled={locked}></label>
      <div class="v2-inline"><label>Режим<select id="v2SelectorMode" class="search" bind:value={mode} disabled={locked}><option value="fast">Быстрый</option><option value="normal">Обычный</option><option value="thorough">Полный</option></select></label><label>Потоки<select id="v2SelectorConcurrency" class="search" bind:value={concurrency} disabled={locked}><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="4">4</option></select></label></div>
      <label class="v2-check"><input id="v2SelectorProduction" type="checkbox" bind:checked={production} disabled={locked}> Профили текущей конфигурации nfqws2</label>
      <label class="v2-check"><input id="v2SelectorAutoPool" type="checkbox" bind:checked={autoPool} disabled={locked}> Встроенный пул RouterForge</label>
      <label class="v2-check"><input id="v2SelectorLibrary" type="checkbox" bind:checked={libraryEnabled} disabled={locked}> Сохранённые пользовательские стратегии</label>
      <label class="v2-check"><input id="v2SelectorCommunity" type="checkbox" bind:checked={community} disabled={locked}> Сообщество по AS провайдера (необязательные внешние запросы)</label>
      <label class="v2-label">AS провайдера<input id="v2SelectorCommunityASN" class="search" type="number" min="1" max="4294967295" bind:value={asn} disabled={locked}></label>
      <div id="v2SelectorPoolMeta" class="v2-boundary">{poolMeta}</div>
      <label class="v2-label">Дополнительная стратегия<input id="v2SelectorExtraName" class="search" placeholder="Название" bind:value={extraName} disabled={locked}><textarea id="v2SelectorExtraArgs" class="v2-textarea compact" placeholder="--filter-tcp=443 ..." bind:value={extraArgs} disabled={locked}></textarea></label>
      <div class="toolbar"><button id="v2SelectorRun" class="primary" disabled={locked} onclick={()=>execute()}>Запустить подбор</button><button id="v2SelectorStop" disabled={!running} onclick={stop}>Стоп</button><button type="button" disabled={locked} onclick={load}>Обновить стратегии</button></div>
    </article>
    <article class="panel"><div class="panel-head"><div><h2>Ход проверки</h2><div id="v2SelectorSession" class="section-kicker mono">{session}</div></div></div><div class="testlab-progress"><span id="v2SelectorBar" style:width={`${progressPercent}%`}></span></div><div id="v2SelectorProgressText" class="v2-boundary">{progressText}</div><div class="v2-stats"><div><span>Этап</span><strong id="v2SelectorPhase">{phase}</strong></div><div><span>Проверено</span><strong id="v2SelectorCompleted">{progressLabel}</strong></div><div><span>Потоки</span><strong id="v2SelectorWorkers">{workers}</strong></div><div><span>Очистка</span><strong id="v2SelectorCleanup">{cleanup}</strong></div></div></article>
  </div>
  <article class="panel"><div class="panel-head"><div><h2>Результаты</h2><div class="section-kicker">реальные HTTPS-запросы, изолированный NFQUEUE</div></div></div><div id="v2SelectorRecommendation" class="v2-recommendation">
    {#if result}<div class:good={result.recommendation_available} class:warn={!result.recommendation_available} class="v2-row"><div class="v2-result-title">{result.recommendation_available?result.recommended_candidate_name||result.recommended_candidate_id||'Рабочая стратегия':'Нет рекомендации'}</div><div><div class="value">{selectorReason(result.recommendation_reason)}</div><div class="meta">{selectorReason(result.apply_gate_reason)}</div><div class="meta">{poolSummary}</div></div></div>{:else}<div class="analysis-empty">Автоподбор ещё не запускался.</div>{/if}</div>
    <div id="v2SelectorResults" class="v2-list">{#if result}{#each [result.baseline,...(result.candidates||[])].filter(Boolean) as candidate,i (i)}{@const metrics=selectorCandidateMetrics(candidate)}<div class={`v2-row ${selectorTone(candidate.result_class)}`}><div><div class="v2-result-title">{i===0?'Без обхода':candidate.candidate_name||candidate.candidate_id||'Стратегия'}</div><div class="meta">{selectorSource(candidate.candidate_source||'baseline')}</div></div><div><div class="value">успех {metrics.success}% · полный ответ {metrics.complete}%{metrics.ttfb?` · TTFB ${metrics.ttfb} мс`:''}</div><div class="v2-args">{(candidate.args||[]).join(' ')}</div>{#if metrics.error}<details class="v2-tech-details"><summary>Диагностика</summary><pre>{metrics.error}</pre></details>{/if}</div><span class={`v2-pill ${selectorTone(candidate.result_class)}`}>{selectorResultLabel(candidate.result_class)}</span></div>{/each}{/if}</div>
  </article>
  {#if applyVisible}<article id="v2SelectorApplyPanel" class="panel"><div class="panel-head"><div><h2>Применение</h2><div class="section-kicker">строгая последовательность bind → preview → explicit apply</div></div></div><div id="v2SelectorApplyReason" class="v2-boundary">{selectorReason(result?.apply_gate_reason)}</div>
    {#if bindVisible}<div id="v2SelectorBindPanel" style="margin-top:10px"><div class="v2-bind-intro">Добавление нового профиля через <code>--new</code>, без замены существующих блоков.</div><label class="v2-label">Рабочая стратегия<select id="v2SelectorBindStrategy" class="search" bind:value={bindID} disabled={locked}>{#each working as candidate (candidate.candidate_id)}<option value={candidate.candidate_id}>{candidate.candidate_name||candidate.candidate_id}</option>{/each}</select></label>{#if bindCandidate}<div id="v2SelectorBindStrategyDetail" class="v2-bind-profile-card"><strong>{bindCandidate.candidate_name||bindCandidate.candidate_id}</strong><div class="v2-bind-args">{(bindCandidate.args||[]).join(' ')}</div></div>{/if}<div id="v2SelectorBindConflicts" class="v2-bind-profile-card"><strong>Совпадения с production-профилями</strong>{#if conflictError}<p>{conflictError}</p>{:else if conflicts.length}{#each conflicts as profile,i (i)}<div class="v2-bind-reason">Профиль {profile.index} · {selectorProfileQuality(profile).label}: {profile.match_reason}<div class="v2-bind-args">{(profile.args||[]).join(' ')}</div></div>{/each}{:else}<p>Совпадения не обнаружены или проверка ещё идёт.</p>{/if}</div><div class="toolbar"><button id="v2SelectorPrepareProfileButton" disabled={locked||!bindID} onclick={bindCandidateToProfile}>{binding?'Перепроверяю…':'Проверить и подготовить новый профиль'}</button></div></div>{/if}
    {#if preparedText}<div class="v2-boundary">{preparedText}</div>{/if}<div class="toolbar"><button id="v2SelectorPreview" disabled={locked||!result?.apply_gate_eligible} onclick={()=>preview()}>Предпросмотр</button><button id="v2SelectorApply" class="primary" disabled={locked||!previewData?.changed} onclick={()=>apply()}>Применить стратегию</button></div><pre id="v2SelectorPreviewBody" class="catalog-preview">{previewText}</pre></article>{/if}
  {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  <div class="v2-boundary">Никаких изменений при открытии экрана и запуске подбора. Применение доступно только после live verification, exact Preview и ручного подтверждения.</div>
</div>
