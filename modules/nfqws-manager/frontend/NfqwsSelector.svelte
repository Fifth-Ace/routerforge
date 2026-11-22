<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {selectorPath,selectorProgressPath,selectorRunBody,selectorProgress,selectorTone,selectorSource,selectorReason,selectorResultLabel,selectorWorking,selectorProfileQuality,selectorPreviewBody,selectorBindBody,selectorApplyBody,selectorCandidateMetrics} from './NfqwsSelectorModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,onRequest=async()=>{},onHistory=()=>{},onNotice=()=>{},onApplied=()=>{},onRegister=()=>{}}=$props();
  let mounted=$state(false),loading=$state(false),loaded=$state(false),attempted=$state(false),running=$state(false),binding=$state(false),previewing=$state(false),applying=$state(false);
  let state=$state(rfUi('ОЖИДАНИЕ',"WAITING")),error=$state(''),target=$state(''),mode=$state('normal'),concurrency=$state('2'),production=$state(true),autoPool=$state(true),libraryEnabled=$state(true),community=$state(false),asn=$state(''),extraName=$state(''),extraArgs=$state('');
  let session=$state('—'),phase=$state(rfUi('ОЖИДАНИЕ',"WAITING")),progressLabel=$state('0 / 0'),progressPercent=$state(0),progressText=$state('Готов к запуску.'),workers=$state('—'),cleanup=$state('—'),poolMeta=$state('RouterForge проверяет стратегии из встроенного пула реальными HTTPS-запросами.');
  let library=$state([]),result=$state(null),previewData=$state(null),previewText=$state(rfUi('Предпросмотр ещё не построен.',"No preview has been generated yet.")),bindID=$state(''),conflicts=$state([]),conflictError=$state(''),preparedText=$state('');
  let alive=false,epoch=0,runAbort=null,pollTimer=0,previewAbort=null,applyAbort=null,bindAbort=null,lastSeed=0,lastErrorText='';
  const locked=$derived(serviceBusy||running||loading||binding||previewing||applying);
  const working=$derived(selectorWorking(result));
  const bindVisible=$derived(!!result?.recommendation_available&&!result.apply_gate_eligible&&working.length>0);
  const bindCandidate=$derived(working.find(c=>String(c.candidate_id)===bindID)||working[0]);
  const applyVisible=$derived(!!result?.recommendation_available);
  const poolSummary=$derived(result?.auto_pool_enabled?`Проверено стратегий: ${(result.candidates||[]).length}.`:rfUi('Встроенный пул отключён.',"Built-in pool disabled."));
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
    lastSeed=seed.seq;invalidate();target=String(seed.target||'').trim();state=rfUi('ОЖИДАНИЕ',"WAITING");error='';
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
      confirmation?String(confirmation).replace('NF3B2_CONFIRM: ',rfUi('Подтверждение: ',"Confirmation:")):'',communityWarning?String(communityWarning).replace('NF3C_COMMUNITY: ','Сообщество: '):''].filter(Boolean).join(' · ');
  }
  function installResult(d,n,requested){
    if(!alive||n!==epoch)return;
    if(!d||!Array.isArray(d.candidates)||typeof d.recommendation_available!=='boolean')throw Error('NFQWS_SELECTOR_RESULT_INVALID');
    result=d;previewData=null;previewText=d.apply_gate_eligible?rfUi('Рабочая стратегия готова. Сначала откройте предпросмотр изменений.',"Working strategy ready. Review the change preview first."):rfUi('Для этой стратегии автоматическое применение пока недоступно.',"Automatic application is not available for this strategy.");
    cleanup=d.cleanup_baseline_after?rfUi('Готово',"Ready"):rfUi('Ошибка',"Error");workers=String(d.concurrency||'—');state='DONE';progressPercent=100;error='';
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
        phase=String(p.phase||'').toUpperCase()||rfUi('ОЖИДАНИЕ',"WAITING");progressLabel=prog.label;progressPercent=phase==='BASELINE'?5:Math.max(0,Math.min(100,prog.percent));
        progressText=rfUi(`${String(p.message||'')} · ${Math.floor((Date.now()-started)/1000)} с`,`${String(p.message||'')} · ${Math.floor((Date.now()-started)/1000)} s`);
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
    const controller=new AbortController();runAbort=controller;running=true;state=rfUi('ВЫПОЛНЯЕТСЯ',"RUNNING");session='—';phase=rfUi('ПОДГОТОВКА',"PREPARING");progressLabel='0 / 0';progressPercent=0;progressText=rfUi('Подготовка запуска · 0 с',"Preparing to start · 0 s");
    invalidatePrevious();
    const n=epoch;
    try{
      const [status,stored]=await Promise.all([request('/status','GET',undefined,controller.signal),request('/v2/strategies','GET',undefined,controller.signal)]);
      if(!alive||n!==epoch||controller.signal.aborted)return null;
      if(!Array.isArray(stored?.strategies))throw Error('NFQWS_SELECTOR_LIBRARY_INVALID');
      library=stored.strategies;loaded=true;
      const sessionValue=sessionID(),payload=selectorRunBody(view(override),library,status?.config_sha256,sessionValue);
      session=sessionValue;poolMeta=payload.auto_pool?({fast:rfUi('Быстрый: 30% пула',"Quick: 30% of pool"),normal:rfUi('Обычный: по 20 до двух рабочих',"Normal: batches of 20 until two work"),thorough:rfUi('Полный: весь пул',"Full: entire pool")})[payload.mode]+rfUi(' · Проверка выполняется реальными HTTPS-запросами.',"· This check uses real HTTPS requests."):rfUi('Встроенный пул отключён.',"Built-in pool disabled.");
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
      previewText=preparedText+rfUi(' Откройте предпросмотр перед применением.',"Open the preview before applying.");
      notify(rfUi('Выбранная стратегия подтверждена. Новый профиль готов к предпросмотру.',"Selected strategy verified. New profile ready for preview."),'good');return response;
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
      onApplied(status);result=null;previewData=null;previewText=rfUi('Применено и проверено.',"Applied and verified.");state=rfUi('ПРИМЕНЕНО',"APPLIED");
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
  <div class="v2-hero"><div><h2>{rfUi("Автоподбор стратегии","Automatic strategy selection")}</h2><div class="section-kicker">{rfUi("baseline → изолированная проверка → результат → предпросмотр → ручное применение","baseline → isolated test → result → preview → manual apply")}</div></div><span id="v2SelectorState" class="config-badge">{state}</span></div>
  <div class="v2-two">
    <article class="panel">
      <label class="v2-label">{rfUi("Цель","Target")}<input id="v2SelectorTarget" class="search" placeholder="youtube.com" autocomplete="off" bind:value={target} disabled={locked}></label>
      <div class="v2-inline"><label>{rfUi("Режим","Mode")}<select id="v2SelectorMode" class="search" bind:value={mode} disabled={locked}><option value="fast">{rfUi("Быстрый","Quick")}</option><option value="normal">{rfUi("Обычный","Normal")}</option><option value="thorough">{rfUi("Полный","Full")}</option></select></label><label>{rfUi("Потоки","Flows")}<select id="v2SelectorConcurrency" class="search" bind:value={concurrency} disabled={locked}><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="4">4</option></select></label></div>
      <label class="v2-check"><input id="v2SelectorProduction" type="checkbox" bind:checked={production} disabled={locked}> {rfUi("Профили текущей конфигурации nfqws2","Profiles from the current nfqws2 configuration")}</label>
      <label class="v2-check"><input id="v2SelectorAutoPool" type="checkbox" bind:checked={autoPool} disabled={locked}> {rfUi("Встроенный пул RouterForge","Built-in RouterForge pool")}</label>
      <label class="v2-check"><input id="v2SelectorLibrary" type="checkbox" bind:checked={libraryEnabled} disabled={locked}> {rfUi("Сохранённые пользовательские стратегии","Saved user strategies")}</label>
      <label class="v2-check"><input id="v2SelectorCommunity" type="checkbox" bind:checked={community} disabled={locked}> {rfUi("Сообщество по AS провайдера (необязательные внешние запросы)","Community recommendations by ISP AS (optional external requests)")}</label>
      <label class="v2-label">{rfUi("AS провайдера","ISP AS")}<input id="v2SelectorCommunityASN" class="search" type="number" min="1" max="4294967295" bind:value={asn} disabled={locked}></label>
      <div id="v2SelectorPoolMeta" class="v2-boundary">{poolMeta}</div>
      <label class="v2-label">{rfUi("Дополнительная стратегия","Additional strategy")}<input id="v2SelectorExtraName" class="search" placeholder={rfUi("Название","Name")} bind:value={extraName} disabled={locked}><textarea id="v2SelectorExtraArgs" class="v2-textarea compact" placeholder="--filter-tcp=443 ..." bind:value={extraArgs} disabled={locked}></textarea></label>
      <div class="toolbar"><button id="v2SelectorRun" class="primary" disabled={locked} onclick={()=>execute()}>{rfUi("Запустить подбор","Run selection")}</button><button id="v2SelectorStop" disabled={!running} onclick={stop}>{rfUi("Стоп","Stop")}</button><button type="button" disabled={locked} onclick={load}>{rfUi("Обновить стратегии","Refresh strategies")}</button></div>
    </article>
    <article class="panel"><div class="panel-head"><div><h2>{rfUi("Ход проверки","Check progress")}</h2><div id="v2SelectorSession" class="section-kicker mono">{session}</div></div></div><div class="testlab-progress"><span id="v2SelectorBar" style:width={`${progressPercent}%`}></span></div><div id="v2SelectorProgressText" class="v2-boundary">{progressText}</div><div class="v2-stats"><div><span>{rfUi("Этап","Stage")}</span><strong id="v2SelectorPhase">{phase}</strong></div><div><span>{rfUi("Проверено","Checked")}</span><strong id="v2SelectorCompleted">{progressLabel}</strong></div><div><span>{rfUi("Потоки","Flows")}</span><strong id="v2SelectorWorkers">{workers}</strong></div><div><span>{rfUi("Очистка","Cleanup")}</span><strong id="v2SelectorCleanup">{cleanup}</strong></div></div></article>
  </div>
  <article class="panel"><div class="panel-head"><div><h2>{rfUi("Результаты","Results")}</h2><div class="section-kicker">{rfUi("реальные HTTPS-запросы, изолированный NFQUEUE","real HTTPS requests, isolated NFQUEUE")}</div></div></div><div id="v2SelectorRecommendation" class="v2-recommendation">
    {#if result}<div class:good={result.recommendation_available} class:warn={!result.recommendation_available} class="v2-row"><div class="v2-result-title">{result.recommendation_available?result.recommended_candidate_name||result.recommended_candidate_id||'Рабочая стратегия':'Нет рекомендации'}</div><div><div class="value">{selectorReason(result.recommendation_reason)}</div><div class="meta">{selectorReason(result.apply_gate_reason)}</div><div class="meta">{poolSummary}</div></div></div>{:else}<div class="analysis-empty">{rfUi("Автоподбор ещё не запускался.","Automatic selection has not run yet.")}</div>{/if}</div>
    <div id="v2SelectorResults" class="v2-list">{#if result}{#each [result.baseline,...(result.candidates||[])].filter(Boolean) as candidate,i (i)}{@const metrics=selectorCandidateMetrics(candidate)}<div class={`v2-row ${selectorTone(candidate.result_class)}`}><div><div class="v2-result-title">{i===0?'Без обхода':candidate.candidate_name||candidate.candidate_id||'Стратегия'}</div><div class="meta">{selectorSource(candidate.candidate_source||'baseline')}</div></div><div><div class="value">{rfUi("успех","success")} {metrics.success}{rfUi("% · полный ответ","% · complete response")} {metrics.complete}%{metrics.ttfb?` · TTFB ${metrics.ttfb} мс`:''}</div><div class="v2-args">{(candidate.args||[]).join(' ')}</div>{#if metrics.error}<details class="v2-tech-details"><summary>{rfUi("Диагностика","Diagnostics")}</summary><pre>{metrics.error}</pre></details>{/if}</div><span class={`v2-pill ${selectorTone(candidate.result_class)}`}>{selectorResultLabel(candidate.result_class)}</span></div>{/each}{/if}</div>
  </article>
  {#if applyVisible}<article id="v2SelectorApplyPanel" class="panel"><div class="panel-head"><div><h2>{rfUi("Применение","Apply")}</h2><div class="section-kicker">{rfUi("строгая последовательность bind → preview → explicit apply","strict sequence: bind → preview → explicit apply")}</div></div></div><div id="v2SelectorApplyReason" class="v2-boundary">{selectorReason(result?.apply_gate_reason)}</div>
    {#if bindVisible}<div id="v2SelectorBindPanel" style="margin-top:10px"><div class="v2-bind-intro">{rfUi("Добавление нового профиля через","Adding a new profile using")} <code>--new</code>{rfUi(", без замены существующих блоков.",", without replacing existing blocks.")}</div><label class="v2-label">{rfUi("Рабочая стратегия","Active strategy")}<select id="v2SelectorBindStrategy" class="search" bind:value={bindID} disabled={locked}>{#each working as candidate (candidate.candidate_id)}<option value={candidate.candidate_id}>{candidate.candidate_name||candidate.candidate_id}</option>{/each}</select></label>{#if bindCandidate}<div id="v2SelectorBindStrategyDetail" class="v2-bind-profile-card"><strong>{bindCandidate.candidate_name||bindCandidate.candidate_id}</strong><div class="v2-bind-args">{(bindCandidate.args||[]).join(' ')}</div></div>{/if}<div id="v2SelectorBindConflicts" class="v2-bind-profile-card"><strong>{rfUi("Совпадения с production-профилями","Matches against active profiles")}</strong>{#if conflictError}<p>{conflictError}</p>{:else if conflicts.length}{#each conflicts as profile,i (i)}<div class="v2-bind-reason">{rfUi("Профиль","Profile")} {profile.index} · {selectorProfileQuality(profile).label}: {profile.match_reason}<div class="v2-bind-args">{(profile.args||[]).join(' ')}</div></div>{/each}{:else}<p>{rfUi("Совпадения не обнаружены или проверка ещё идёт.","No matches found, or the check is still running.")}</p>{/if}</div><div class="toolbar"><button id="v2SelectorPrepareProfileButton" disabled={locked||!bindID} onclick={bindCandidateToProfile}>{binding?'Перепроверяю…':'Проверить и подготовить новый профиль'}</button></div></div>{/if}
    {#if preparedText}<div class="v2-boundary">{preparedText}</div>{/if}<div class="toolbar"><button id="v2SelectorPreview" disabled={locked||!result?.apply_gate_eligible} onclick={()=>preview()}>{rfUi("Предпросмотр","Preview")}</button><button id="v2SelectorApply" class="primary" disabled={locked||!previewData?.changed} onclick={()=>apply()}>{rfUi("Применить стратегию","Apply strategy")}</button></div><pre id="v2SelectorPreviewBody" class="catalog-preview">{previewText}</pre></article>{/if}
  {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  <div class="v2-boundary">{rfUi("Никаких изменений при открытии экрана и запуске подбора. Применение доступно только после live verification, exact Preview и ручного подтверждения.","Opening this screen or running selection changes nothing. Applying requires a live verification, exact preview, and manual confirmation.")}</div>
</div>
