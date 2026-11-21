<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import NfqwsDPIStatus from './NfqwsDPIStatus.svelte';
  import {dpiStatusProjection} from './NfqwsDPIStatusModel.js';
  import {consumeDpiNdjson} from './NfqwsDpiStream.js';
  import {dpiTestOptions,dpiStreamPath,dpiWebPayload,dpiEndProof,dpiReportSections} from './NfqwsDPIWorkspaceModel.js';
  let {active=false,serviceBusy=false,snapshot=null,loading=false,error='',onRequest=async()=>{},onRefresh=()=>{},onNotice=()=>{}}=$props();
  let mounted=$state(false),preparing=$state(false),mode=$state('console'),selected=$state(['0','1','2']);
  let profile=$state('ru'),language=$state('ru'),concurrency=$state('20'),interfaceName=$state(''),proxy=$state(''),fingerprint=$state('');
  let domains=$state(''),domainsFile=$state(''),tcp16File=$state(''),burst=$state('5'),burstTimeout=$state('8'),burstTLS=$state('1.3+1.2'),burstALPN=$state('h2'),burstProfiles=$state('');
  let running=$state(false),stateLabel=$state('IDLE'),errText=$state(''),report=$state(''),meta=$state(null),startedAt=$state(0),elapsed=$state(0),streamTests=$state('');
  let legendOpen=$state(false),legendLoading=$state(false),legend=$state(''),legendError=$state('');
  let consoleVersion=$state(0),overlay=$state(false),consoleSrc=$state('');
  let reportView=$state('all'),iframeNode=$state(null),runController=null,life=false,epoch=0,clock=0;
  const status=$derived(dpiStatusProjection(snapshot,loading,error));
  const locked=$derived(serviceBusy||preparing||running||loading);
  const reportSections=$derived(report&& !running ?dpiReportSections(report):[]);
  const visibleReport=$derived(reportView==='all'?report:reportSections.find(x=>x.id===reportView)?.text||report);
  function notify(text,type=''){if(life)onNotice({text:String(text),type});}
  function source(){return `/api/modules/admin/ui/index.html?locale=${encodeURIComponent(document.documentElement.lang||'ru')}&view=terminal&embed=terminal&autorun=dpi-detector${consoleVersion?'&restart='+consoleVersion:''}`;}
  function refreshConsole(force=false){if(!life||!active||mode!=='console'||!status.ready)return;if(force)consoleVersion=Date.now();consoleSrc=source();}
  function stop(silent=false){
    if(runController){runController.abort();runController=null;}
    if(running){running=false;stateLabel='STOPPED';if(!silent)notify(rfUi('DPI Detector: запрос остановлен.',"DPI Detector: request stopped."),'warn');}
    if(clock){clearInterval(clock);clock=0;}
    ++epoch;
  }
  function toggleTest(id){if(locked)return;selected=selected.includes(id)?selected.filter(x=>x!==id):[...selected,id].sort();}
  function form(){return{tests:selected,domains,domainsFile,tcp16File,concurrency,language,profile,interface:interfaceName,proxy,fingerprint,burst,burstTimeout,burstProfiles,burstTLS,burstALPN};}
  async function launch(){
    if(!life||!active||locked||!status.ready)return;
    preparing=true;
    const preflightEpoch=epoch;
    let payload;
    try{
      const statusNow=await onRequest('/status',{method:'GET'});
      if(!life||!active||running||preflightEpoch!==epoch)return;
      payload=dpiWebPayload(form(),statusNow?.config_sha256);
    }catch(e){errText=String(e?.message||e);stateLabel='FAIL';notify(`DPI Detector: ${errText}`,'error');return;}
    finally{preparing=false;}
    if(!life||!active||preflightEpoch!==epoch)return;
    const n=++epoch,controller=new AbortController();runController=controller;
    stateLabel='LIVE';errText='';meta=null;report='';reportView='all';streamTests=payload.tests;startedAt=Date.now();elapsed=0;running=true;
    if(clock)clearInterval(clock);
    clock=setInterval(()=>{if(life&&n===epoch)elapsed=Math.floor((Date.now()-startedAt)/1000);},1000);
    let doneEvent=null,streamFailure='';
    try{
      await consumeDpiNdjson({url:dpiStreamPath(location.pathname),payload,signal:controller.signal,onEvent:event=>{
        if(!life||n!==epoch||controller.signal.aborted||!active)return;
        if(event.type==='chunk')report+=String(event.data||'');
        else if(event.type==='snapshot')report=String(event.report||report);
        else if(event.type==='cut'){meta={...(meta||{}),cut:true};}
        else if(event.type==='done')doneEvent=event;
        else if(event.type==='error')streamFailure=[event.error,event.detail].filter(Boolean).join(' · ')||'DPI_STREAM_FAILED';
      }});
      if(!life||n!==epoch||controller.signal.aborted)return;
      if(streamFailure)throw Error(streamFailure);
      meta=dpiEndProof(doneEvent,payload.expected_config_sha256);
      if(!report.trim())throw Error('DPI_REPORT_EMPTY');
      stateLabel='DONE';notify(meta.cut?rfUi('DPI Detector: отчёт усечён по лимиту.',"DPI Detector: report truncated at size limit."):rfUi('DPI Detector: upstream report получен; production SHA совпадает.',"DPI Detector: report received; active configuration SHA matches."),'good');
    }catch(e){
      if(life&&n===epoch){errText=controller.signal.aborted?'DPI_STREAM_CANCELLED':String(e?.message||e);stateLabel=controller.signal.aborted?'STOPPED':'FAIL';notify(`DPI Detector: ${errText}`,'error');}
    }finally{
      if(runController===controller)runController=null;
      if(life&&n===epoch)running=false;
      if(clock){clearInterval(clock);clock=0;}
    }
  }
  async function loadLegend(){
    if(!life||!active||legendLoading)return;
    legendOpen=true;legendLoading=true;legendError='';legend='';const n=epoch;
    try{
      const d=await onRequest(`/dpi-detector/v5/legend?lang=${encodeURIComponent(language)}`,{method:'GET'});
      if(life&&active&&n===epoch)legend=String(d?.legend||'—');
    }catch(e){if(life&&active&&n===epoch)legendError=String(e?.message||e);}
    finally{if(life&&n===epoch)legendLoading=false;}
  }
  async function copy(){if(!report)return;try{await navigator.clipboard.writeText(report);notify('DPI Detector: отчёт скопирован.','good');}catch(e){notify(`Copy: ${String(e?.message||e)}`,'error');}}
  function toggleOverlay(){overlay=!overlay;document.body.classList.toggle('dpi-console-overlay-open',overlay);}
  onMount(()=>{
    life=true;mounted=true;
    const onEsc=e=>{if(e.key==='Escape'&&overlay)toggleOverlay();};
    document.addEventListener('keydown',onEsc);
    return()=>{life=false;stop(true);document.removeEventListener('keydown',onEsc);document.body.classList.remove('dpi-console-overlay-open');};
  });
  $effect(()=>{if(!active){if(running||preparing)stop(true);if(overlay){overlay=false;document.body.classList.remove('dpi-console-overlay-open');}}});
  $effect(()=>{if(active&&mode==='console'&&status.ready&&mounted&&!consoleSrc)refreshConsole();});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-dpi-workspace">
  <div class="v2-hero"><div><h2>DPI Detector</h2><div class="section-kicker">{rfUi("Официальный DPI Detector · консоль и Native Live Web","Official DPI Detector · console and live web view")}</div></div><span class="config-badge" class:active={status.ready}>{status.state}</span></div>
  <div class="dpi-top-grid">
    <article class="panel dpi-mode-card">
      <div class="dpi-mode-head"><div><h2>{rfUi("Режим","Mode")}</h2><span>{rfUi("один upstream binary · два представления","One upstream binary · two views")}</span></div><button disabled={loading||running} onclick={()=>onRefresh()}>{rfUi("Обновить","Refresh")}</button></div>
      <div class="dpi-mode-switch">
        <button class:primary={mode==='console'} onclick={()=>mode='console'} disabled={running}><strong>{rfUi("Консоль","Console")}</strong><span>{rfUi("оригинальное меню DPI Detector","Original DPI Detector menu")}</span></button>
        <button class:primary={mode==='web'} onclick={()=>mode='web'} disabled={running}><strong>{rfUi("Графический режим","Graphical view")}</strong><span>native live report</span></button>
      </div>
      <NfqwsDPIStatus {snapshot} {loading} {error} mode="summary" />
    </article>
    <div class="strategy-import-credit dpi-upstream-credit"><div><strong>DPI Detector · Runnin4ik/dpi-detector</strong><span>{rfUi("Установленный через App Center /opt/bin/dpi-detector. RouterForge только вызывает компонент.","The component installed through App Center at /opt/bin/dpi-detector. RouterForge only launches it.")}</span></div><a href="https://github.com/Runnin4ik/dpi-detector" target="_blank" rel="noopener noreferrer">{rfUi("Открыть upstream","Open upstream")}</a></div>
  </div>
  {#if mode==='console'}
    <article class="panel" class:dpi-console-overlay={overlay}>
      <div class="panel-head"><div><h2>{rfUi("Консоль DPI Detector","DPI Detector console")}</h2><div class="section-kicker">{rfUi("Оригинальное меню диагностики","Original diagnostics menu")}</div></div><div class="toolbar"><button onclick={toggleOverlay}>{overlay?'Закрыть полный экран':'На весь экран'}</button><button disabled={!status.ready} onclick={()=>refreshConsole(true)}>{rfUi("Перезапустить DPI Detector","Restart DPI Detector")}</button></div></div>
      {#if consoleSrc}<iframe bind:this={iframeNode} title="DPI Detector native console" src={consoleSrc} style="display:block;width:100%;height:680px;border:1px solid var(--border);border-radius:8px;background:var(--bg)"></iframe>{:else}<div class="analysis-empty">{rfUi("Консоль доступна, когда установлен и проверен DPI Detector.","The console is available when DPI Detector is installed and verified.")}</div>{/if}
      <div class="v2-boundary">{rfUi("Консольный режим использует оригинальное терминальное меню. Данные Web и Console не запускают автоматически друг друга.","Console mode uses the original terminal menu. Web and Console modes do not start each other automatically.")}</div>
    </article>
  {:else}
    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>{rfUi("Web · тесты","Web tests")}</h2><div class="section-kicker">{rfUi("Выберите тесты 0–6; legend — отдельный GET","Select tests 0–6; the legend is retrieved separately")}</div></div><span class="config-badge" class:active={stateLabel==='DONE'}>{stateLabel}</span></div>
        <div class="dpi-test-grid">
          {#each dpiTestOptions as test (test.id)}
            <label class="dpi-test-card"><input type="checkbox" checked={selected.includes(test.id)} onchange={()=>toggleTest(test.id)} disabled={locked}/><span><strong>{test.id} · {test.name}</strong><small>{test.detail}</small></span></label>
          {/each}
          <button type="button" class="dpi-test-legend" onclick={loadLegend} disabled={legendLoading}><strong>{rfUi("7 · Обозначения","7 · Legend")}</strong><span>{rfUi("Справка upstream","Upstream reference")}</span></button>
        </div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>{rfUi("Общие параметры","Common settings")}</h2></div></div>
        <div class="v2-two">
          <label class="v2-label">{rfUi("Профиль","Profile")}<select class="search" bind:value={profile} disabled={locked}><option value="ru">RU</option><option value="global">Global</option><option value="ir">IR</option><option value="cn">CN</option></select></label>
          <label class="v2-label">{rfUi("Язык","Language")}<select class="search" bind:value={language} disabled={locked}><option value="ru">{rfUi("Русский","Russian")}</option><option value="en">English</option><option value="zh">中文</option><option value="fa">فارسی</option></select></label>
          <label class="v2-label">{rfUi("Параллельно","In parallel")}<select class="search" bind:value={concurrency} disabled={locked}><option>1</option><option>5</option><option>20</option><option>50</option><option>100</option></select></label>
          <label class="v2-label">{rfUi("Интерфейс","Interface")}<input class="search" bind:value={interfaceName} disabled={locked} placeholder="routing table / eth2.1"/></label>
        </div>
        <label class="v2-label">SOCKS/HTTP proxy<input class="search" bind:value={proxy} disabled={locked} placeholder={rfUi("не использовать","do not use")}/></label>
        <label class="v2-label">TLS fingerprint<select class="search" bind:value={fingerprint} disabled={locked}>
          <option value="">{rfUi("По умолчанию","Default")}</option>{#each ['rustls','chrome146','chrome131','chrome131_android','chrome123','chrome116','chrome107','chrome99_android','edge101','firefox147','firefox133','safari260','safari260_ios','safari184_ios','safari180','safari172_ios','safari170','safari155','safari153','tor145'] as fp (fp)}<option value={fp}>{fp}</option>{/each}
        </select></label>
      </article>
    </div>
    <div class="v2-two" style="margin-top:12px">
      <article class="panel"><div class="panel-head"><div><h2>{rfUi("Цели","Targets")}</h2><div class="section-kicker">-d / --domains / --tcp16</div></div></div><label class="v2-label">{rfUi("Домены · по одному на строку","Domains · one per line")}<textarea class="v2-textarea compact" bind:value={domains} disabled={locked} placeholder="example.com"></textarea></label><label class="v2-label">{rfUi("Файл доменов","Domain file")}<input class="search mono" bind:value={domainsFile} disabled={locked} placeholder="/opt/.../domains.txt"/></label><label class="v2-label">TCP16 targets<input class="search mono" bind:value={tcp16File} disabled={locked} placeholder="/opt/.../tcp16.json"/></label></article>
      <article class="panel"><div class="panel-head"><div><h2>Fingerprint / Burst</h2><div class="section-kicker">{rfUi("Используется test 6","Used by test 6")}</div></div></div><div class="v2-two">
        <label class="v2-label">{rfUi("Попытки","Attempts")}<input class="search" type="number" min="1" max="100" bind:value={burst} disabled={locked}/></label>
        <label class="v2-label">Timeout, sec<input class="search" type="number" min="1" max="120" bind:value={burstTimeout} disabled={locked}/></label>
        <label class="v2-label">TLS<select class="search" bind:value={burstTLS} disabled={locked}><option value="1.3+1.2">1.3 + 1.2</option><option value="1.3">1.3</option><option value="1.2">1.2</option></select></label>
        <label class="v2-label">ALPN<select class="search" bind:value={burstALPN} disabled={locked}><option value="h2">h2</option><option value="http/1.1">http/1.1</option></select></label>
      </div><label class="v2-label">Burst profiles<input class="search mono" bind:value={burstProfiles} disabled={locked} placeholder="all · rustls,chrome146"/></label><div class="v2-boundary">{rfUi("Подробный trace не включается автоматически. Только один подтверждённый stream POST после нажатия «Запустить».","Detailed trace output is not enabled automatically. A single streaming request starts only when you press Start.")}</div></article>
    </div>
    <article class="panel dpi-run-panel" style="margin-top:12px"><div class="panel-head"><div><h2>{rfUi("Запуск","Run")}</h2><div class="section-kicker">{rfUi("Native Live · исходный upstream report без HTML-инъекций","Live results · original upstream report displayed safely")}</div></div><div class="toolbar"><button class="primary" onclick={launch} disabled={locked||!active||!status.ready}>{rfUi("Запустить DPI Detector","Start DPI Detector")}</button><button onclick={()=>stop()} disabled={!running}>Stop</button></div></div>
      <div class="v2-boundary">{rfUi("nfqws2, VPN и proxy автоматически не отключаются. В выводе отражён текущий router path, не raw ISP truth.","nfqws2, VPN, and proxies are not disabled automatically. Results reflect the router's current network path, not necessarily the ISP's unfiltered path.")}</div>
      <div class="dpi-run-progress" class:running={running} class:bad={stateLabel==='FAIL'}><div class="dpi-run-progress-head"><strong>{stateLabel==='LIVE'?'Идёт один полный upstream run':stateLabel==='DONE'?'Завершено':stateLabel==='FAIL'?'Проверка завершилась ошибкой':'Ожидание'}</strong><span>{running?`${elapsed}s`:stateLabel}</span></div>{#if errText}<div class="dpi-run-progress-detail">{errText}</div>{/if}</div>
    </article>
    <article class="panel dpi-native-live-shell" style="margin-top:12px"><div class="dpi-native-live-head"><div><strong>DPI Detector · Native Live</strong><span>{rfUi("Оригинальный upstream output, отображается как текст","Original upstream output, displayed as plain text")}</span></div><div class="dpi-native-live-state"><span class="config-badge" class:active={stateLabel==='DONE'}>{stateLabel}</span><span class="mono">{meta?`${meta.duration} ms · ${meta.bytes} bytes${meta.cut?' · CUT':''}`:running?`${elapsed}s · streaming`:'—'}</span></div></div>
      {#if report&&!running&&reportSections.length>1}<div class="toolbar" style="margin-top:10px"><button class:primary={reportView==='all'} onclick={()=>reportView='all'}>{rfUi("Весь отчёт","Full report")}</button>{#each reportSections as s (s.id)}<button class:primary={reportView===s.id} onclick={()=>reportView=s.id}>{s.title}</button>{/each}<button onclick={copy}>{rfUi("Копировать","Copy")}</button></div>{/if}
      <div class="dpi-native-live-body"><pre style="white-space:pre-wrap;overflow-wrap:anywhere;margin:0">{visibleReport||'Выберите тесты и запустите DPI Detector.'}</pre></div>
      <div class="dpi-native-live-footer">{meta?.environmentWarning||meta?.exitWarning|| (running?'Данные поступают по мере выполнения.':stateLabel==='DONE'?'Production SHA подтверждён сервером.':'Запуск выполняется только вручную.')}</div>
    </article>
    {#if legendOpen}<article class="panel" style="margin-top:12px"><div class="panel-head"><div><h2>{rfUi("Обозначения результатов","Result legend")}</h2></div><button onclick={()=>legendOpen=false}>{rfUi("Закрыть","Close")}</button></div><pre class="v2-args">{legendLoading?'Загрузка…':legendError||legend||'—'}</pre></article>{/if}
  {/if}
</div>
