<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {processingHostname,processingListFiles,processingSummary,processingLiveProof,processingStrategyPlan,processingFastPayload,processingReport} from './NfqwsProcessingMapModel.js';
  let {active=false,serviceBusy=false,inventory=[],onRequest=async()=>{},onStaticPath=()=>{},onNavigate=()=>{},onOpenList=()=>{},onLab=()=>{},onNotice=()=>{}}=$props();
  let rawDomain=$state(''),checking=$state(false),liveRunning=$state(false),phase=$state('IDLE'),problem=$state('');
  let progress=$state(''),result=$state(null),live=$state(null),strategies=$state(null);
  let life=false,epoch=0,controller=null;
  const summary=$derived(processingSummary(result));
  const liveProof=$derived(processingLiveProof(live,liveRunning));
  const plan=$derived(processingStrategyPlan(strategies,live));
  const locked=$derived(serviceBusy||checking||liveRunning);
  const hits=$derived(summary.lists?.hits||[]);
  function notify(text,type=''){if(life)onNotice({text:String(text),type});}
  function cancel(){const wasRunning=checking||liveRunning;++epoch;controller?.abort();controller=null;checking=false;liveRunning=false;if(wasRunning){phase='STOPPED';progress='Проверка остановлена';}}
  onMount(()=>{life=true;return()=>{life=false;cancel();};});
  $effect(()=>{if(!active&&(checking||liveRunning))cancel();});
  async function loadLists(n,signal,status){
    const names=processingListFiles(Array.isArray(status?.lists)?status.lists:inventory),queue=[...names],contents={},errors=[];
    let completed=0;
    progress=`Списки: 0/${names.length}`;
    async function worker(){
      while(queue.length&&life&&active&&n===epoch&&!signal.aborted){
        const name=queue.shift();if(!name)break;
        try{
          const item=await onRequest(`/list?name=${encodeURIComponent(name)}`,{method:'GET',signal});
          if(n===epoch&&!signal.aborted)contents[name]=String(item?.content||'');
        }catch(e){if(n===epoch&&!signal.aborted)errors.push(`${name}: ${String(e?.message||e)}`);}
        completed+=1;if(n===epoch)progress=`Списки: ${completed}/${names.length}`;
      }
    }
    await Promise.all(Array.from({length:Math.min(4,Math.max(1,names.length))},()=>worker()));
    return{contents,errors};
  }
  async function run(){
    if(!life||!active||locked)return;
    const domain=processingHostname(rawDomain);
    if(!domain){problem=rfUi('Некорректный публичный hostname',"Invalid public host name");phase='FAIL';notify(problem,'error');return;}
    const n=++epoch,abort=new AbortController();controller=abort;
    checking=true;problem='';result=null;live=null;strategies=null;phase='RUNNING';progress=rfUi('Проверка config, списков и HTTPS…',"Checking configuration, lists and HTTPS…");
    try{
      const latest=await onRequest('/status',{method:'GET',signal:abort.signal});
      const files=await loadLists(n,abort.signal,latest);
      if(!life||!active||n!==epoch||abort.signal.aborted)return;
      const path=onStaticPath(domain,files.contents,latest?.config_sha256);
      let probe=null,keenetic=null,inv=null;
      try{probe=await onRequest('/check',{method:'POST',body:JSON.stringify({url:`https://${domain}/`}),signal:abort.signal});}
      catch(e){probe={reachable:false,error:String(e?.message||e)};}
      try{keenetic=await onRequest('/keenetic',{method:'GET',signal:abort.signal});}catch{}
      try{inv=await onRequest('/bench-strategies',{method:'GET',signal:abort.signal});}catch{}
      if(!life||!active||n!==epoch||abort.signal.aborted)return;
      strategies=inv;
      result={domain,staticPath:path,probe,keenetic,listErrors:files.errors,lists:files.contents,running:latest?.running===true,configSHA:String(latest?.config_sha256||'')};
      phase='DONE';progress=`${domain} · ${Object.keys(files.contents).length} прочитанных list-файлов`;
      notify(rfUi('Processing Map: read-only диагностика завершена.',"Processing map: read-only diagnostics complete."),'good');
    }catch(e){if(life&&n===epoch&&!abort.signal.aborted){problem=String(e?.message||e);phase='FAIL';notify(`Processing Map: ${problem}`,'error');}}
    finally{if(controller===abort)controller=null;if(life&&n===epoch)checking=false;}
  }
  async function runLive(){
    if(!life||!active||locked||!result?.running||result.listErrors.length)return;
    const n=++epoch,abort=new AbortController();controller=abort;
    liveRunning=true;live=null;problem='';
    try{
      const latest=await onRequest('/status',{method:'GET',signal:abort.signal});
      const payload=processingFastPayload(result.domain,latest);
      // A changed config invalidates the read-only map; never reuse a stale result.
      if(payload.expected_config_sha256!==result.configSHA.toLowerCase())throw Error('PROCESSING_MAP_STALE_CONFIG_SHA');
      const answer=await onRequest('/bench-autotune',{method:'POST',body:JSON.stringify(payload),signal:abort.signal});
      if(!life||!active||n!==epoch||abort.signal.aborted)return;
      if(answer?.ok!==true)throw Error('PROCESSING_LIVE_NO_SUCCESS');
      if(answer?.cleanup_baseline_after!==true)throw Error('PROCESSING_LIVE_CLEANUP_NOT_PROVEN');
      live=answer;const proof=processingLiveProof(answer);
      notify(proof.proven?rfUi('Live NFQWS: candidate path и cleanup подтверждены.',"Live NFQWS: candidate path and cleanup verified."):rfUi('Live NFQWS: cleanup подтверждён, но TLS через candidate не доказан.',"Live NFQWS: cleanup verified but candidate TLS behavior was not proven."),proof.proven?'good':'warn');
    }catch(e){if(life&&n===epoch&&!abort.signal.aborted){live={error:String(e?.message||e)};notify(`Live NFQWS: ${live.error}`,'error');}}
    finally{if(controller===abort)controller=null;if(life&&n===epoch)liveRunning=false;}
  }
  async function copyReport(guided=false){
    if(!result)return;
    try{
      const data=processingReport(result,strategies,live);
      const content=guided?[
        'RouterForge Guided Diagnostics',`Domain: ${data.domain}`,`Verdict: ${summary.title}`,summary.detail,`Next: ${summary.next}`,
        `Config SHA256: ${data.active_config_sha256}`,`Static path: ${data.static_path.verdict}`,`Router HTTPS: ${summary.probe.label}`,
        `Live NFQWS: ${liveProof.label}`,`List read errors: ${data.list_errors.length}`,data.observability_boundary
      ].join('\n'):JSON.stringify(data,null,2);
      await navigator.clipboard.writeText(content);notify('Processing Map: отчёт скопирован.','good');
    }catch(e){notify(`Copy: ${String(e?.message||e)}`,'error');}
  }
  function openRoute(tab){
    if(tab==='prodlab-v2'&&result?.domain)onLab({target:result.domain});
    onNavigate(tab);
  }
  function openList(name,line){onNavigate('lists');onOpenList(String(name||''),Number(line)||1);}
</script>
<div class="testlab-shell" data-svelte-owned="nfqws-processing-map">
  <div class="testlab-hero">
    <div><div class="testlab-title-row"><h2>{rfUi("Диагностика сайта","Website diagnostics")}</h2><span class="testlab-state" class:running={checking||liveRunning}>{phase}</span><span class="config-badge">PRODUCTION READ-ONLY</span></div><div class="preflight-sub">{rfUi("HTTPS с роутера и тестовая NFQUEUE — разные пути. Ни один из них не доказывает автоматически путь пакета LAN-клиента через production q300.","HTTPS from the router and the test NFQUEUE use different paths. Neither proves the actual path of a LAN client's packet through the active queue.")}</div></div>
    <div class="toolbar"><button onclick={()=>copyReport(false)} disabled={!result}>{rfUi("Копировать JSON","Copy JSON")}</button><button class="primary" onclick={run} disabled={locked||!active}>{rfUi("Проверить","Check")}</button><button onclick={cancel} disabled={!checking&&!liveRunning}>Stop</button></div>
  </div>
  <div class="guided-shell">
    <div class="guided-steps" aria-label={rfUi("Этапы диагностики","Diagnostic stages")}><div class="guided-step"><span class="guided-step-num">1</span><span><strong>{rfUi("Домен","Domain")}</strong><span>{rfUi("Публичный hostname","Public hostname")}</span></span></div><div class="guided-step"><span class="guided-step-num">2</span><span><strong>{rfUi("Текущая конфигурация","Current configuration")}</strong><span>{rfUi("Списки · HTTP · WAN","Lists · HTTP · WAN")}</span></span></div><div class="guided-step"><span class="guided-step-num">3</span><span><strong>Live NFQWS</strong><span>{rfUi("Временная NFQUEUE · обязательный cleanup","Temporary NFQUEUE · mandatory cleanup")}</span></span></div></div>
    {#if result}
      <article class="guided-result visible" aria-live="polite"><div class="guided-result-head"><div><div class="guided-result-title">{summary.title}</div><div class="guided-result-detail">{summary.detail}</div></div><div class="guided-result-tools"><button onclick={runLive} disabled={!result.running||locked||result.listErrors.length>0}>{rfUi("Проверить через NFQWS","Test through NFQWS")}</button><button onclick={()=>copyReport(true)}>{rfUi("Копировать поддержку","Copy support details")}</button><span class="guided-result-badge" class:good={summary.kind==='good'} class:bad={summary.kind==='bad'}>{summary.kind.toUpperCase()}</span></div></div>
      <div class="guided-evidence"><div class="guided-evidence-item"><span>nfqws2</span><strong>{result.running?'RUNNING':'STOPPED'}</strong></div><div class="guided-evidence-item"><span>{rfUi("Списки","Lists")}</span><strong>{summary.lists.found} {rfUi("совпадений","matches")}</strong></div><div class="guided-evidence-item"><span>WAN</span><strong>{result.keenetic?.default_ipv4_interface||result.keenetic?.config_isp_interface||'не определён'}</strong></div><div class="guided-evidence-item"><span>{rfUi("HTTP с роутера","HTTP from router")}</span><strong>{summary.probe.label}</strong></div><div class="guided-evidence-item"><span>Live NFQWS</span><strong>{liveProof.label}</strong></div></div>
      <div class="guided-next"><div class="guided-next-title">{rfUi("Что это значит","What this means")}</div><div class="guided-next-text">{summary.next}</div><div class="guided-actions">{#each summary.actions as action (action.tab)}<button onclick={()=>openRoute(action.tab)}>{action.title}</button>{/each}</div></div>
      <details class="processing-technical"><summary>{rfUi("Технические детали · границы наблюдаемости","Technical details · limits of observation")}</summary><div class="guided-timeline"><div class="guided-timeline-row info"><span>Static path</span><span>{result.staticPath?.verdict||'UNKNOWN'} · {result.staticPath?.reason||''}</span></div><div class="guided-timeline-row info"><span>HTTPS probe</span><span>{summary.probe.detail}</span></div><div class="guided-timeline-row info"><span>LAN packet</span><span>{rfUi("Не наблюдался на входе и выходе production NFQUEUE.","Not observed at the input and output of the active NFQUEUE.")}</span></div></div></details></article>
    {/if}
  </div>
  <div class="testlab-layout">
    <article class="panel testlab-targets"><div class="panel-head"><h2>{rfUi("Домен","Domain")}</h2><span class="section-kicker">public DNS hostname</span></div><input class="search" style="width:100%;max-width:none" bind:value={rawDomain} onkeydown={e=>{if(e.key==='Enter')run();}} placeholder="youtube.com" disabled={locked}/><div class="testlab-note">{rfUi("Обычная проверка — только чтение. Fast NFQWS запускается исключительно по отдельной кнопке, не меняет рабочую q300 и требует доказательства очистки.","The standard check is read only. Fast NFQWS runs only when explicitly requested, does not modify the active queue, and must verify cleanup.")}</div><div class="section-kicker">{progress}</div>{#if problem}<div class="bad-text">{problem}</div>{/if}</article>
    <article class="panel"><div class="testlab-summary"><div class="testlab-stat"><span>{rfUi("Режим","Mode")}</span><strong>{result?.staticPath?.mode||'—'}</strong></div><div class="testlab-stat"><span>{rfUi("Списки","Lists")}</span><strong>{result?`${summary.lists.found} найдено · ${summary.lists.active} активных`:'—'}</strong></div><div class="testlab-stat"><span>HTTP</span><strong>{result?summary.probe.label:'—'}</strong></div><div class="testlab-stat"><span>{rfUi("Сервис","Service")}</span><strong>{result?(result.running?'RUNNING':'STOPPED'):'—'}</strong></div></div><div class="diagnostics" style="margin-top:12px">{#if result}<div class="diagnostic" class:good={summary.kind==='good'} class:bad={summary.kind==='bad'}><strong>{summary.title}</strong><div>{summary.detail}</div></div>{#if result.listErrors.length}<div class="diagnostic warn"><strong>{rfUi("Списки: неполные данные","Lists: incomplete data")}</strong>{#each result.listErrors as error}<div>{error}</div>{/each}</div>{/if}{:else}<div class="analysis-empty">{rfUi("Введите домен и нажмите «Проверить».","Enter a domain and click Check.")}</div>{/if}</div></article>
  </div>
  <article class="panel processing-how">
    <div class="processing-how-head"><h2>{rfUi("Как выполняется проверка","How the check works")}</h2><p>{rfUi("Router-side HTTPS probe не форсирует production q300. Live создаёт узкое временное правило и candidate в отдельной тестовой NFQUEUE (30000–30127), после чего требуется cleanup.","An HTTPS check from the router does not exercise production queue 300. Live testing creates a limited temporary rule and candidate in an isolated test NFQUEUE (30000–30127); cleanup must be verified afterward.")}</p></div>
    <div class="processing-route-grid"><section class="processing-route-card direct"><div class="processing-route-head"><div class="processing-route-title">{rfUi("1 · Обычный HTTPS","1 · Standard HTTPS")}</div><span class="processing-route-badge" class:good={result&&summary.probe.tone==='good'}>{result?summary.probe.label:'НЕ ЗАПУСКАЛАСЬ'}</span></div><div class="processing-route-flow"><div class="processing-route-node"><span>{rfUi("RouterForge → системный маршрут → HTTPS :443","RouterForge → system route → HTTPS :443")}</span><strong>{result?.domain||'Ожидание'}</strong></div><div class="processing-route-node"><span>{rfUi("Результат","Result")}</span><strong>{result?summary.probe.detail:'Проверка не запускалась'}</strong></div></div></section>
    <section class="processing-route-card live"><div class="processing-route-head"><div class="processing-route-title">2 · Live NFQWS</div><span class="processing-route-badge" class:good={liveProof.proven}>{liveProof.label}</span></div><div class="processing-route-flow"><div class="processing-route-node"><span>{rfUi("Тестовый socket → NFQUEUE 30000–30127 → candidate → TLS","Test socket → NFQUEUE 30000–30127 → candidate → TLS")}</span><strong>{liveProof.detail}</strong></div></div><div class="processing-strategy-box"><div class="processing-strategy-head"><strong>{rfUi("План проверки","Test plan")}</strong><span>{rfUi("Fast · baseline + до 3 профилей","Quick · baseline and up to 3 profiles")}</span></div><div class="processing-strategy-list">{#each plan as item (item.name)}<div class="processing-strategy-row"><div class="processing-strategy-id">{item.name}</div><div class="processing-strategy-main"><div class="processing-strategy-title">{item.args}</div></div><div class="processing-strategy-result"><strong>{item.result?'EXECUTED':'PENDING'}</strong></div></div>{/each}</div></div></section></div>
    <div class="processing-safety-strip"><div class="processing-safety-item good"><span>Production config</span><strong>{rfUi("не записывается","not recorded")}</strong></div><div class="processing-safety-item good"><span>Production q300</span><strong>{rfUi("не тестируется","not tested")}</strong></div><div class="processing-safety-item" class:good={liveProof.cleanup}><span>Live cleanup</span><strong>{liveProof.cleanup?'PASS':live?'НЕ ДОКАЗАН':'не запускался'}</strong></div></div>
  </article>
  <article class="panel" style="margin-top:14px"><div class="panel-head"><h2>{rfUi("Подробный путь и найденные списки","Detailed path and matched lists")}</h2><span class="section-kicker">{result?.domain||'статический анализ + live evidence'}</span></div><div class="static-path-steps">{#if result}<div class="static-path-step neutral"><div class="static-path-index">1</div><strong>{rfUi("ДОМЕН","DOMAIN")}</strong><span>{result.domain}</span></div><div class="static-path-step neutral"><div class="static-path-index">2</div><strong>{rfUi("СПИСКИ","LISTS")}</strong><span>{summary.lists.found} {rfUi("совпадений ·","matches ·")} {summary.lists.active} {rfUi("активных ·","active ·")} {summary.lists.unknown.length} {rfUi("неподтвержденных","unverified")}</span></div>{#each hits as hit,i (`${hit.file}-${hit.line}-${i}`)}<div class="static-path-step" class:include={hit.effective==='include'} class:exclude={hit.effective==='exclude'}><div class="static-path-index">{i+3}</div><div><div class="static-path-role">{hit.effective||'neutral'}</div><span class="inspector-meta">{hit.relation}</span></div><div><div class="static-path-value">{hit.file}:{hit.line}</div><div class="static-path-meta">{hit.value} · {hit.role?.reason||hit.detail||''}</div></div><button class="static-path-open" onclick={()=>openList(hit.file,hit.line)}>{rfUi("Открыть","Open")}</button></div>{/each}<div class="static-path-step neutral"><div class="static-path-index">{hits.length+3}</div><strong>LIVE NFQWS</strong><span>{liveProof.label} · {liveProof.detail}</span></div><div class="static-path-step neutral"><div class="static-path-index">{hits.length+4}</div><strong>{rfUi("HTTP с роутера","HTTP from router")}</strong><span>{summary.probe.label} · {summary.probe.detail}</span></div>{:else}<div class="analysis-empty">{rfUi("Путь ещё не построен.","Path has not been analyzed yet.")}</div>{/if}</div></article>
</div>
