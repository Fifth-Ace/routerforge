<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {autoSetupPath,autoSetupTarget,autoSetupObserved,autoSetupDiagnosisStop,autoSetupStageSummary,autoSetupErrorCode,autoSetupLabel,autoSetupBlockerText,autoSetupTCP16Body} from './NfqwsAutoSetupModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,getSelector=()=>null,onRequest=async()=>{},onNotice=()=>{}}=$props();
  let mounted=$state(false),attempted=$state(false),loading=$state(false),running=$state(false),applying=$state(false),previewing=$state(false);
  let source=$state('manual'),target=$state(''),observedTarget=$state(''),listName=$state(''),listTarget=$state(''),mode=$state('normal'),concurrency=$state('2');
  let sources=$state([]),observed=$state([]),listTargets=$state([]),timeline=$state([]),status=$state({code:'IDLE',message:rfUi('Выберите target и запустите workflow.',"Choose a target and start the check."),tone:'info'}),winner=$state(null),previewData=$state(null),error=$state('');
  let alive=false,epoch=0,loadEpoch=0,runAbort=null,lastSeed=0,ownsSelector=false;
  const locked=$derived(serviceBusy||loading||running||applying||previewing);
  const canApply=$derived(!!previewData?.changed&&!locked);
  function notify(text,type=''){if(alive)onNotice({text:String(text),type});}
  function message(e){return String(e?.message||e||'NFQWS_AUTO_SETUP_UNKNOWN');}
  function request(path,method='GET',body,signal){
    if(!alive)throw Error('NFQWS_AUTO_SETUP_UNMOUNTED');
    autoSetupPath(path,method);
    return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});
  }
  function setWorkflowStatus(code,text,tone='info'){status={code,message:String(text||''),tone};}
  function push(code,title,detail='',tone='info'){timeline=[...timeline,{code,title,detail:String(detail||''),tone}];}
  function clear(){++epoch;++loadEpoch;runAbort?.abort();if(ownsSelector){getSelector()?.stopExternal?.();getSelector()?.clearPreview?.();ownsSelector=false;}running=false;applying=false;previewing=false;winner=null;previewData=null;timeline=[];error='';setWorkflowStatus('IDLE','Выберите target и запустите workflow.');}
  function acceptSeed(){
    if(!seed?.seq||lastSeed===seed.seq)return;
    lastSeed=seed.seq;clear();source='manual';target=String(seed.target||'').trim();setWorkflowStatus('READY',`Target получен из ${String(seed.source||'Observed Targets')}: ${target}`);
  }
  async function load(){
    if(!alive||!active||loading)return;
    loading=true;attempted=true;const n=++loadEpoch;
    try{
      const [listResult,observedResult]=await Promise.all([request('/v2/target-sources'),request('/v2/observed-targets')]);
      if(!alive||n!==loadEpoch)return;
      if(!Array.isArray(listResult?.sources)||!Array.isArray(observedResult?.observed))throw Error('NFQWS_AUTO_SETUP_INVENTORY_INVALID');
      sources=listResult.sources.filter(x=>x&&typeof x.name==='string');
      observed=autoSetupObserved(observedResult.observed);
      if(!sources.some(x=>x.name===listName))listName=sources[0]?.name||'';
      if(!observed.some(x=>x.name===observedTarget))observedTarget=observed[0]?.name||'';
    }catch(e){if(alive&&n===loadEpoch){error=message(e);setWorkflowStatus('DEPENDENCY_MISSING',error,'bad');}}
    finally{if(alive&&n===loadEpoch)loading=false;}
  }
  async function chooseSource(value){
    if(locked)return;source=value;previewData=null;winner=null;
    if(value==='list')await resolveList();
    if(value==='observed'&&!observed.length)await load();
  }
  async function resolveList(){
    if(!alive||!listName)return;
    const n=++loadEpoch;listTargets=[];
    try{
      const response=await request('/v2/targets/resolve','POST',{source:'list',list_name:listName,limit:64});
      if(!alive||n!==loadEpoch)return;
      if(!Array.isArray(response?.targets))throw Error('NFQWS_AUTO_SETUP_LIST_TARGETS_INVALID');
      listTargets=response.targets.filter(x=>typeof x==='string');
      listTarget=listTargets.includes(listTarget)?listTarget:listTargets[0]||'';
    }catch(e){if(alive&&n===loadEpoch){error=message(e);setWorkflowStatus(autoSetupErrorCode(error),error,'bad');}}
  }
  function validTarget(){return autoSetupTarget({source,target,observed:observedTarget,listTarget});}
  async function capabilityDetails(signal){
    try{const data=await request('/bench-capabilities','GET',undefined,signal);return(Array.isArray(data?.blockers)?data.blockers:[]).map(autoSetupBlockerText).join(' ')||'Безопасность стенда не подтверждена.';}
    catch(e){returnrfUi('Сведения о безопасном стенде недоступны: ',"Safe test environment details are unavailable:")+message(e);}
  }
  async function buildPreview(n=epoch){
    if(!alive||!winner?.apply_gate_eligible||previewing)return null;
    previewing=true;error='';
    try{
      const selector=getSelector();if(!selector?.previewExternal)throw Error('NFQWS_SELECTOR_SVELTE_UNAVAILABLE');
      const p=await selector.previewExternal();
      if(!alive||n!==epoch)return null;
      if(!p?.ok||!p.preview_only)throw Error(selector.errorMessage?.()||'NFQWS_AUTO_SETUP_PREVIEW_INVALID');
      previewData=p;push('PREVIEW_READY',rfUi('Exact deterministic Preview готов',"Exact configuration preview ready"),`${p.candidate_config_sha256||''} · production не изменён`,'good');
      setWorkflowStatus('PREVIEW_READY',rfUi('Winner проверен и exact config готов. Следующий шаг требует явного Apply.',"Winning strategy verified and exact configuration ready. The next step requires explicit Apply."),'good');return p;
    }catch(e){if(alive&&n===epoch){error=message(e);const code=autoSetupErrorCode(error);setWorkflowStatus(code,error,'bad');push(code,rfUi('Exact Preview не готов',"Verified preview is not ready"),error,'bad');}return null;}
    finally{if(alive&&n===epoch)previewing=false;}
  }
  async function run(){
    if(!alive||!active||locked)return;
    let chosen;try{chosen=validTarget();}catch(e){error=message(e);setWorkflowStatus('TARGET_REQUIRED','Выберите или введите target.','bad');return;}
    const n=++epoch,controller=new AbortController();runAbort=controller;running=true;winner=null;previewData=null;timeline=[];error='';
    if(ownsSelector){getSelector()?.stopExternal?.();getSelector()?.clearPreview?.();ownsSelector=false;}
    try{
      const initial=await request('/status','GET',undefined,controller.signal);
      if(!alive||n!==epoch||controller.signal.aborted)return;
      if(!/^[a-f\d]{64}$/i.test(String(initial?.config_sha256||'')))throw Error('NFQWS_AUTO_SETUP_STATUS_SHA_INVALID');
      setWorkflowStatus('INSPECTING',`Проверяю ${chosen}: production mapping + baseline DNS/TCP/TLS/HTTP.`);
      const [inspect,detect]=await Promise.all([request('/v2/inspect-target','POST',{target:chosen},controller.signal),request('/v2/detect','POST',{target:chosen},controller.signal)]);
      if(!alive||n!==epoch||controller.signal.aborted)return;
      const diag=detect?.diagnostic||{},stage=autoSetupStageSummary(detect);
      push('INSPECTED',rfUi('Проверена текущая конфигурация',"Current configuration checked"),`Подходящих профилей: ${(inspect.production_profile_matches||[]).length} · совпадений в списках: ${(inspect.list_matches||[]).length}`,'good');
      push('CHECKS',rfUi('Проверена цепочка доступа',"Connectivity path checked"),stage,detect.classification==='clear'?'good':'warn');
      push(diag.code||detect.classification||'BASELINE',autoSetupLabel(diag.code||detect.classification||'BASELINE'),diag.suggested_action||detect.classification_text||rfUi('Диагностика завершена.',"Diagnostics complete."),detect.classification==='clear'?'good':'warn');
      const stop=autoSetupDiagnosisStop(detect);
      if(stop){
        push('NO_CHANGES',rfUi('Стратегии NFQWS не подбирались',"NFQWS strategies have not been selected"),rfUi('Рабочая конфигурация осталась без изменений.',"Active configuration remains unchanged."),stop[0]==='TARGET_ALREADY_HEALTHY'?'good':'warn');
        setWorkflowStatus(stop[0],stop[0]==='TARGET_ALREADY_HEALTHY'?`Сайт уже работает. ${stage}. Ничего менять не требуется.`:stop[1]+' '+stage,stop[2]);
        return;
      }
      if(String(diag.code||'').toUpperCase()==='HTTP_STREAM_CUTOFF_12_20K_SUSPECTED'){
        setWorkflowStatus('TCP16_REVALIDATE',rfUi('Проверяю bounded TCP16 network probe.',"Running a limited TCP16 network probe."),'warn');
        const tcp16=await request('/v2/tcp16-probe','POST',autoSetupTCP16Body(initial.config_sha256),controller.signal);
        if(!alive||n!==epoch||controller.signal.aborted)return;
        push('TCP16_DONE',rfUi('TCP16 branch завершён',"TCP16 branch completed"),`detected=${Number(tcp16.detected_count||0)} · working SNI=${Number(tcp16.working_sni_count||0)} · production read-only=${!!tcp16.read_only_production}`,'good');
      }
      const selector=getSelector();if(!selector?.runExternal)throw Error('NFQWS_SELECTOR_SVELTE_UNAVAILABLE');
      setWorkflowStatus('LIVE_VERIFY','Строю bounded plan и запускаю isolated live Selector.');ownsSelector=true;
      const selected=await selector.runExternal({target:chosen,mode,concurrency,production:true,autoPool:true,libraryEnabled:true,community:false,asn:''});
      if(!alive||n!==epoch||controller.signal.aborted)return;
      if(!selected){
        const raw=selector.errorMessage?.()||rfUi('Автоподбор не завершился.',"Automatic selection did not finish.");
        if(raw.toLowerCase().includes('capability gates')){
          const details=await capabilityDetails(controller.signal);
          if(!alive||n!==epoch||controller.signal.aborted)return;
          setWorkflowStatus('DEPENDENCY_MISSING',rfUi(`Проблема найдена, но безопасный автоподбор не запущен. ${details}`,`Problem found, but safe automatic selection did not run. ${details}`),'bad');
          push('SELECTOR_BLOCKED',rfUi('Автоподбор не запущен',"Automatic selection has not started"),details,'bad');push('NO_CHANGES',rfUi('Ничего не изменено',"Nothing changed"),rfUi('Production-конфигурация осталась прежней.',"Active configuration remains unchanged."),'good');return;
        }
        const code=autoSetupErrorCode(raw);setWorkflowStatus(code,raw,'bad');push(code,rfUi('Автоподбор остановлен',"Automatic selection stopped"),raw,'bad');return;
      }
      if(!selected.cleanup_baseline_after){setWorkflowStatus('CLEANUP_FAILURE',rfUi('Isolated test не доказал cleanup baseline. Apply заблокирован.',"Isolated test did not confirm cleanup. Application blocked."),'bad');push('CLEANUP_FAILURE','Cleanup proof failed','','bad');return;}
      if(!Array.isArray(selected.candidates)||!selected.candidates.length){setWorkflowStatus('NO_CANDIDATES_COMPILE',rfUi('Нет пригодных кандидатов для live verification.',"No suitable candidates for live verification."),'bad');push('NO_CANDIDATES_COMPILE',rfUi('Candidate set пуст после compiler/gates',"Candidate set is empty after validation"),'','bad');return;}
      if(!selected.recommendation_available){setWorkflowStatus('NO_CANDIDATE_WORKS',selected.recommendation_reason||rfUi('Ни один кандидат не улучшил baseline.',"No candidate improved the baseline."),'warn');push('NO_CANDIDATE_WORKS',rfUi('Winner не найден',"No winning strategy found"),selected.recommendation_reason||'','warn');return;}
      if(!selected.apply_gate_eligible){setWorkflowStatus('WINNER_VERIFIED',rfUi('Winner подтверждён, но unique production slot для Safe Apply не доказан.',"Winning strategy verified, but a unique target slot for Safe Apply was not proven."),'warn');push('WINNER_VERIFIED','Winner live-verified',selected.apply_gate_reason||'Apply gate blocked','warn');return;}
      winner=selected;push('WINNER_VERIFIED',rfUi('Live winner повторно подтверждён',"Selected strategy reverified live"),`${selected.recommended_candidate_source||''} · ${selected.recommended_candidate_id||''}`,'good');
      // Run state deliberately remains read-only until explicit Apply button.
    }catch(e){if(alive&&n===epoch&&!controller.signal.aborted){error=message(e);const code=autoSetupErrorCode(error);setWorkflowStatus(code,error,'bad');push(code,rfUi('Workflow остановлен',"Workflow stopped"),error,'bad');}}
    finally{if(runAbort===controller)runAbort=null;if(alive&&n===epoch)running=false;}
    if(alive&&n===epoch&&winner&&!controller.signal.aborted)await buildPreview(n);
  }
  async function apply(){
    if(!alive||!active||!canApply)return;
    applying=true;const n=epoch;error='';
    try{
      const selector=getSelector();if(!selector?.applyExternal)throw Error('NFQWS_SELECTOR_SVELTE_UNAVAILABLE');
      const result=await selector.applyExternal();
      if(!alive||n!==epoch)return;
      if(!result){const msg=selector.errorMessage?.()||rfUi('Apply не выполнен или отменён.',"Apply was not performed or was canceled.");const code=autoSetupErrorCode(msg);setWorkflowStatus(code,msg,'bad');push(code,rfUi('Safe Apply не завершён',"Safe Apply did not complete"),msg,'bad');return;}
      if(result.rolled_back||!result.post_apply_verified){setWorkflowStatus('APPLY_ROLLBACK',rfUi('Smart Apply не подтвердил production.',"Smart Apply could not verify the live configuration."),'bad');push('APPLY_ROLLBACK',rfUi('Rollback или failed verify',"Rolled back or verification failed"),'','bad');return;}
      const latest=await request('/status');
      if(!alive||n!==epoch)return;
      if(String(latest.config_sha256||'').toLowerCase()!==String(result.config_sha256||'').toLowerCase()){
        setWorkflowStatus('APPLY_ROLLBACK',rfUi('Post-Apply production SHA не совпал с verified candidate.',"After applying, the live configuration SHA does not match the verified candidate."),'bad');push('APPLY_ROLLBACK','Production verification mismatch','','bad');return;
      }
      previewData=null;push('APPLY_SUCCESS','Production verification PASS',`config=${latest.config_sha256} · running=${!!latest.running}`,'good');
      setWorkflowStatus('APPLY_SUCCESS',rfUi('Exact candidate применён через Smart Apply; production verify прошёл.',"Selected configuration applied using Smart Apply; live verification passed."),'good');
      notify(rfUi('Автонастройка завершена: production проверен.',"Automatic setup completed: live configuration verified."),'good');
    }catch(e){if(alive&&n===epoch){error=message(e);const code=autoSetupErrorCode(error);setWorkflowStatus(code,error,'bad');push(code,rfUi('Apply workflow остановлен',"Applying changes was stopped"),error,'bad');}}
    finally{if(alive&&n===epoch)applying=false;}
  }
  onMount(()=>{alive=true;mounted=true;return()=>{alive=false;++epoch;++loadEpoch;runAbort?.abort();if(ownsSelector){getSelector()?.stopExternal?.();getSelector()?.clearPreview?.();}};});
  $effect(()=>{if(mounted&&active&&!attempted&&!loading)void load();});
  $effect(()=>{if(mounted&&seed?.seq)acceptSeed();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-auto-setup-v1">
  <div class="v2-hero"><div><h2>{rfUi("Автонастройка","Automatic setup")}</h2><div class="section-kicker">{rfUi("диагностика → live Selector → exact preview → ручное применение","diagnostics → live selection → exact preview → manual application")}</div></div><span id="v2AutoSetupState" class="config-badge">{autoSetupLabel(status.code)}</span></div>
  <div class="v2-two">
    <article class="panel"><div class="panel-head"><div><h2>{rfUi("1. Что проверить","1. What to check")}</h2><div class="section-kicker">{rfUi("введите домен либо выберите из наблюдений или списка","Enter a domain, or select one from observations or a list")}</div></div></div>
      <label class="v2-label">{rfUi("Источник","Source")}<select id="v2AutoSetupSource" class="search" value={source} onchange={e=>chooseSource(e.currentTarget.value)} disabled={locked}><option value="manual">{rfUi("Ручной ввод","Manual entry")}</option><option value="observed">{rfUi("Обнаруженный домен","Observed domain")}</option><option value="list">{rfUi("Список / домен","List / domain")}</option></select></label>
      {#if source==='manual'}<label id="v2AutoSetupManualWrap" class="v2-label">{rfUi("Домен","Domain")}<input id="v2AutoSetupTarget" class="search" placeholder="youtube.com" autocomplete="off" spellcheck="false" bind:value={target} disabled={locked}></label>{/if}
      {#if source==='observed'}<label id="v2AutoSetupObservedWrap" class="v2-label">{rfUi("Обнаруженный домен","Observed domain")}<select id="v2AutoSetupObserved" class="search" bind:value={observedTarget} disabled={locked}>{#each observed as row (row.name)}<option value={row.name}>{row.name} · {row.reason||''} · {row.count||0}</option>{:else}<option value="">{rfUi("Нет целей","No targets")}</option>{/each}</select></label>{/if}
      {#if source==='list'}<div id="v2AutoSetupListWrap"><label class="v2-label">{rfUi("Список","List")}<select id="v2AutoSetupList" class="search" bind:value={listName} onchange={resolveList} disabled={locked}>{#each sources as item (item.name)}<option value={item.name}>{item.name} · {item.count||0}</option>{:else}<option value="">{rfUi("Нет списков","No lists")}</option>{/each}</select></label><label class="v2-label">{rfUi("Домен","Domain")}<select id="v2AutoSetupListTarget" class="search" bind:value={listTarget} disabled={locked}>{#each listTargets as item (item)}<option value={item}>{item}</option>{:else}<option value="">{rfUi("Нет доступных целей","No available targets")}</option>{/each}</select></label></div>{/if}
      <div class="v2-inline"><label>{rfUi("Режим","Mode")}<select id="v2AutoSetupMode" class="search" bind:value={mode} disabled={locked}><option value="fast">{rfUi("Быстрый","Quick")}</option><option value="normal">{rfUi("Обычный","Normal")}</option><option value="thorough">{rfUi("Полный","Full")}</option></select></label><label>{rfUi("Потоки","Flows")}<select id="v2AutoSetupConcurrency" class="search" bind:value={concurrency} disabled={locked}><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="4">4</option></select></label></div>
      <div class="toolbar"><button id="v2AutoSetupRun" class="primary" disabled={locked} onclick={run}>{rfUi("Диагностировать и подобрать","Diagnose and select")}</button><button id="v2AutoSetupReset" disabled={applying} onclick={clear}>{rfUi("Сбросить / остановить","Reset / stop")}</button></div>
    </article>
    <article class="panel"><div class="panel-head"><div><h2>{rfUi("2. Что происходит","2. Progress and diagnostics")}</h2><div class="section-kicker">{rfUi("проверки, причины и следующий шаг","Checks, explanations, and next steps")}</div></div></div><div id="v2AutoSetupStatus" class="v2-recommendation"><div class={`v2-row ${status.tone}`}><div class="v2-result-title">{autoSetupLabel(status.code)}</div><div><div class="value">{status.message}</div><div class="meta">{rfUi("Технический код:","Technical code:")} {status.code}</div></div></div></div><div id="v2AutoSetupTimeline" class="v2-list" style="margin-top:10px">{#each timeline as item,i (i)}<div class={`v2-row ${item.tone}`}><div class="v2-result-title">{item.title||autoSetupLabel(item.code)}</div><div><div class="value">{item.detail}</div><div class="meta">{rfUi("Технический код:","Technical code:")} {item.code}</div></div><span class={`v2-pill ${item.tone}`}>{autoSetupLabel(item.code)}</span></div>{/each}</div></article>
  </div>
  {#if winner}<article id="v2AutoSetupWinner" class="panel"><div class="panel-head"><div><h2>{rfUi("3. Найденная стратегия","3. Suggested strategy")}</h2><div class="section-kicker">{rfUi("результат live verification","Live verification result")}</div></div></div><div id="v2AutoSetupWinnerBody" class="v2-recommendation"><div class="v2-row good"><strong>{winner.recommended_candidate_name||winner.recommended_candidate_id||'Winner'}</strong><div><div>{winner.recommendation_reason||'Live verified'}</div><div class="meta">{winner.recommended_candidate_source} · {winner.apply_gate_reason}</div></div></div></div></article>{/if}
  {#if previewData}<article id="v2AutoSetupPreviewPanel" class="panel"><div class="panel-head"><div><h2>{rfUi("4. Предпросмотр","4. Preview")}</h2><div class="section-kicker">{rfUi("точные изменения перед применением","Exact changes before applying")}</div></div></div><div id="v2AutoSetupPreviewMeta" class="v2-boundary">active={previewData.active_config_sha256} · candidate={previewData.candidate_config_sha256} · fingerprint={previewData.candidate_fingerprint||'—'}</div><pre id="v2AutoSetupPreviewBody" class="catalog-preview">{previewData.candidate_config||'Конфигурация не изменяется'}</pre><div class="toolbar"><button id="v2AutoSetupRebuildPreview" disabled={locked} onclick={()=>buildPreview()}>{rfUi("Обновить предпросмотр","Refresh preview")}</button><button id="v2AutoSetupApply" class="primary" disabled={!canApply} onclick={apply}>{rfUi("Применить изменения","Apply changes")}</button></div></article>{/if}
  {#if error}<div class="setup-note bad-text" role="alert">{error}</div>{/if}
  <div class="v2-boundary">{rfUi("Автонастройка ничего не применяет до последней кнопки. Все проверки, подбор и предпросмотр выполняются до явного подтверждения.","Automatic setup makes no changes until you confirm the final action. Checks, selection, and preview are performed first.")}</div>
</div>
