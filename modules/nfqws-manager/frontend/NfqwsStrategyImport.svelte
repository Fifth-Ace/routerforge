<script>
  import {onMount} from 'svelte';
  import {IMPORT_LIMIT,IMPORT_CONFIRM,CONFIG_CREATE_CONFIRM,normalizeImportedResult,normalizeImportName,importStrategyItems,confirmedStrategyImport,candidateFromImport,importDiff,defaultImportName} from './NfqwsStrategyImportModel.js';
  let {active=false,status=null,serviceBusy=false,onRequest=async()=>{},onNotice=()=>{},onConfigOpen=()=>{},onSandbox=()=>{},onRegistryInvalidate=()=>{},onConfigSaved=()=>{}}=$props();
  let source=$state(''),sourceMeta=$state('Команда · .bat · .cmd · .txt · лог теста'),fileInput;
  let parsed=$state(null),phase=$state('IDLE'),error=$state(''),working=$state(false);
  let libraryName=$state(''),librarySaved=$state(''),profileIndex=$state('0'),libraryPhase=$state('WAITING'),strategyPhase=$state('WAITING');
  let parseSha=$state(''),parseConfig=$state('');let alive=false,epoch=0,requestAbort=null;
  const config=$derived(typeof status?.config==='string'?status.config:'');
  const candidate=$derived(parsed?.ready&&!status?.config_truncated&&parseSha===String(status?.config_sha256||'')?candidateFromImport(parsed,parseConfig):{ok:false,error:'Нужен READY preview и неизменённый SHA полностью загруженного active config.'});
  const diff=$derived(candidate.ok?importDiff(parseConfig,candidate.content):[]);
  const canSave=$derived(active&&!working&&!serviceBusy&&parsed?.ready===true&&candidate.ok&&!!normalizeImportName(libraryName)&&!!parseSha&&parseSha===String(status?.config_sha256||''));
  const canImport=$derived(active&&!working&&!serviceBusy&&parsed?.ready===true&&(parsed.profiles||[]).length>0);
  function notify(text,type=''){onNotice({text,type});}
  function cancel(){epoch++;requestAbort?.abort();requestAbort=null;working=false;}
  function resetResult(){cancel();parsed=null;phase='IDLE';error='';librarySaved='';libraryPhase='WAITING';strategyPhase='WAITING';parseSha='';parseConfig='';}
  function clear(){resetResult();source='';sourceMeta='Команда · .bat · .cmd · .txt · лог теста';libraryName='';profileIndex='0';}
  async function parse(){
    if(!active||working||!source.trim()||serviceBusy||!alive)return;
    const seq=++epoch,c=new AbortController();requestAbort?.abort();requestAbort=c;working=true;error='';phase='ПРОВЕРКА';parsed=null;librarySaved='';libraryPhase='WAITING';strategyPhase='WAITING';
    try{
      const result=normalizeImportedResult(await onRequest('/v2/import/parse',{method:'POST',body:JSON.stringify({content:source}),signal:c.signal}));
      if(!alive||seq!==epoch||c.signal.aborted)return;
      parsed=result;phase=result.ready?'ГОТОВО':result.preview?'ПРОВЕРЬТЕ':'ОЖИДАНИЕ';parseSha=String(status?.config_sha256||'');parseConfig=config;
      if(!libraryName.trim())libraryName=defaultImportName();profileIndex='0';
      if(result.type==='log'&&result.log){parsed={...result,warnings:[['warn',`Log parity: format=${result.log.format||'unknown'} · strategies=${Number(result.log.strategies||0)} · targets=${Number(result.log.targets||0)} · tests=${Number(result.log.tests||0)}`],...result.warnings]};}
      libraryPhase=result.ready?'READY':'CHECK';strategyPhase=result.ready&&result.profiles.length?'READY':'WAITING';
      notify(`Импорт стратегии: ${result.type} · ${result.profiles.length} profile(s) · ${result.ready?'READY':'CHECK'}`,result.ready?'good':'');
    }catch(e){if(alive&&seq===epoch&&!c.signal.aborted){phase='ОШИБКА';error=String(e?.message||e);notify(`Импорт стратегии: ${error}`,'error');}}
    finally{if(requestAbort===c)requestAbort=null;if(alive&&seq===epoch)working=false;}
  }
  async function fileChosen(event){
    const file=event.currentTarget?.files?.[0];if(event.currentTarget)event.currentTarget.value='';
    if(!file)return;if(file.size>IMPORT_LIMIT){error='Файл больше 256 KiB';return;}
    if(!/\.(?:bat|cmd|txt)$/i.test(file.name)){error='Допустимы .bat, .cmd или .txt';return;}
    resetResult();sourceMeta=`${file.name} · ${file.size} bytes`;
    try{const text=await file.text();if(!alive)return;source=text;if(active)void parse();}catch(e){error=String(e?.message||e);}
  }
  async function saveInactive(){
    if(!canSave||!alive)return;
    const name=normalizeImportName(libraryName),candidateBody=candidate.content,expected=parseSha;
    if(!globalThis.confirm(`Создать ${name} в Config Library? Active nfqws2.conf и runtime не изменятся.`))return;
    const seq=++epoch,c=new AbortController();requestAbort=c;working=true;libraryPhase='ПРОВЕРКА SHA';error='';
    try{
      const latest=await onRequest('/status',{method:'GET',signal:c.signal});
      if(!alive||seq!==epoch||c.signal.aborted)return;
      if(!expected||String(latest?.config_sha256||'')!==expected||latest?.config_truncated||String(latest?.config||'')!==parseConfig)throw Error('Active config изменился после Preview. Разберите исходник заново.');
      libraryPhase='СОХРАНЕНИЕ';
      await onRequest('/configs/create',{method:'POST',signal:c.signal,body:JSON.stringify({name,content:candidateBody,from_current:false,confirm:CONFIG_CREATE_CONFIRM})});
      if(!alive||seq!==epoch||c.signal.aborted)return;
      librarySaved=name;libraryPhase='СОХРАНЕНО';onConfigSaved();notify(`Импорт сохранён как ${name}; active config не изменён.`,'good');
    }catch(e){if(alive&&seq===epoch&&!c.signal.aborted){libraryPhase='ОШИБКА';error=String(e?.message||e);notify(error,'error');}}
    finally{if(requestAbort===c)requestAbort=null;if(alive&&seq===epoch)working=false;}
  }
  async function importProfiles(){
    if(!canImport||!alive)return;
    let items;try{items=importStrategyItems(parsed);}catch(e){error=String(e?.message||e);return;}
    if(!globalThis.confirm(`Добавить ${items.length} profiles в Strategy Library? Active nfqws2.conf не изменится.`))return;
    const seq=++epoch,c=new AbortController();requestAbort=c;working=true;strategyPhase='СОХРАНЕНИЕ';error='';
    try{
      const result=await onRequest('/v2/strategies/import',{method:'POST',signal:c.signal,body:JSON.stringify({items,confirm:IMPORT_CONFIRM})});
      if(!alive||seq!==epoch||c.signal.aborted)return;
      const ack=confirmedStrategyImport(result,items.length);
      strategyPhase='СОХРАНЕНО';onRegistryInvalidate();notify(`Импорт: added ${ack.added}, existing ${ack.existing}. Production не изменён.`,'good');
    }catch(e){if(alive&&seq===epoch&&!c.signal.aborted){strategyPhase='ОШИБКА';error=String(e?.message||e);notify(error,'error');}}
    finally{if(requestAbort===c)requestAbort=null;if(alive&&seq===epoch)working=false;}
  }
  function sendSandbox(){if(!parsed?.ready||!active||working)return;const index=Number(profileIndex),profile=parsed.profiles?.[index];if(!Number.isInteger(index)||!profile)return;onSandbox({kind:'candidate',args:profile,name:`Импортированный профиль ${index+1}`,source:'import'});}
  async function copyPreview(){if(!parsed?.preview||!active)return;try{await navigator.clipboard.writeText(parsed.preview);notify('Preview скопирован.','good');}catch(e){error=String(e?.message||e);}}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(!active&&working)cancel();});
  $effect(()=>{if(parsed?.ready&&parseSha&&status?.config_sha256&&parseSha!==String(status.config_sha256)){libraryPhase='STALE';}});
</script>
<div class="strategy-import-shell" data-svelte-owned="nfqws-strategy-import">
 <div class="strategy-import-hero"><div><div class="testlab-title-row"><h2 style="margin:0">Импорт стратегии</h2><span id="strategyImportState" class="testlab-state">{phase}</span><span class="config-badge">PREVIEW FIRST</span></div><div class="preflight-sub">Вставьте команду NFQWS/Windows или выберите файл. До явного подтверждения никаких изменений.</div></div><div class="toolbar"><input id="strategyImportFile" bind:this={fileInput} type="file" accept=".bat,.cmd,.txt" hidden onchange={fileChosen}><button id="strategyImportPick" disabled={working} onclick={()=>fileInput?.click()}>Выбрать файл</button><button id="strategyImportRun" class="primary" disabled={working||serviceBusy||!source.trim()} onclick={parse}>Разобрать</button></div></div>
 <div class="strategy-import-layout"><article class="panel strategy-import-input-panel"><div class="panel-head"><div><h2>Исходник</h2><div id="strategyImportSourceMeta" class="section-kicker">{sourceMeta}</div></div><button id="strategyImportClear" disabled={working} onclick={clear}>Очистить</button></div><textarea id="strategyImportInput" class="strategy-import-input" spellcheck="false" placeholder="@echo off&#10;start &quot;NFQWS&quot; /min winws.exe --filter-tcp=443 --dpi-desync=fake" bind:value={source} oninput={resetResult} disabled={working}></textarea><div class="strategy-import-note">Файл читается локально; backend получает текст только при явном «Разобрать».</div></article>
 <article class="panel"><div class="strategy-import-summary"><div><span>Тип</span><strong id="strategyImportType">{parsed?.type||'—'}</strong></div><div><span>Команд</span><strong id="strategyImportCommands">{parsed?.commands?.length||0}</strong></div><div><span>Профилей</span><strong id="strategyImportProfiles">{parsed?.profiles?.length||0}</strong></div><div><span>Статус</span><strong id="strategyImportVerdict">{parsed?.ready?'ГОТОВО':parsed?.preview?'ПРОВЕРЬТЕ':'—'}</strong></div></div><div id="strategyImportMessage" class="strategy-import-message">{parsed?.ready?'Preview получен. Сохранение требует отдельного подтверждения.':parsed?.preview?'Обнаружены неоднозначные места — сохранение заблокировано.':'Добавьте исходник и нажмите «Разобрать».'}</div><div id="strategyImportWarnings" class="strategy-import-warnings">{#each parsed?.warnings||[] as row,i (i)}<div class="strategy-import-warning" class:bad={row[0]==='bad'}>{row[1]}</div>{/each}{#if error}<div class="notice error" role="alert">{error}</div>{/if}</div></article></div>
 <div class="strategy-import-results"><article class="panel"><div class="panel-head"><div><h2>Преобразования</h2><div class="section-kicker">Каждое изменение явно</div></div></div><div id="strategyImportTransforms" class="strategy-import-list">{#each parsed?.transforms||[] as row,i (i)}<div class="strategy-import-row"><b>{row[0]}</b><span>{row[1]}</span></div>{:else}<div class="analysis-empty">Преобразований пока нет.</div>{/each}</div></article><article class="panel"><div class="panel-head"><div><h2>Зависимости</h2><div class="section-kicker">Списки, blobs и переменные</div></div></div><div id="strategyImportDeps" class="strategy-import-list">{#each parsed?.deps||[] as row,i (i)}<div class="strategy-import-row"><b>{row[0]}</b><span class="mono">{row[1]}</span></div>{:else}<div class="analysis-empty">Зависимости не определены.</div>{/each}</div></article></div>
 <article class="panel" style="margin-top:10px"><div class="panel-head"><div><h2>Предварительный результат</h2><div class="section-kicker">Не применён к nfqws2.conf</div></div><button id="strategyImportCopy" disabled={!parsed?.preview} onclick={copyPreview}>Копировать</button></div><pre id="strategyImportPreview" class="strategy-import-preview strategy-import-syntax-preview">{parsed?.preview||'Preview ещё не построен.'}</pre><div class="strategy-import-note">Неизвестные Windows-переменные и payload-имена не подставляются автоматически.</div></article>
 <article id="strategyImportLibraryPanel" class="panel strategy-import-library-panel"><div class="panel-head"><div><h2>Сохранить как вариант конфигурации</h2><div class="section-kicker">Config Library · inactive until separate activation</div></div><span id="strategyImportLibraryState" class="config-badge">{libraryPhase}</span></div><div class="strategy-import-library-grid"><div><label class="strategy-import-library-label">Имя файла<input id="strategyImportLibraryName" class="search" placeholder="strategy-import.conf" autocomplete="off" bind:value={libraryName} disabled={working}></label><div id="strategyImportLibraryPlan" class="strategy-import-note">{candidate.ok?`Конфигурация inactive; ${parsed?.profiles?.length||0} профилей в NFQWS_ARGS_CUSTOM. Ожидается точный SHA active config.`:candidate.error}</div><div class="strategy-import-library-actions"><button id="strategyImportLibrarySave" class="primary" disabled={!canSave} onclick={saveInactive}>Сохранить конфигурацию</button><button id="strategyImportLibraryOpen" disabled={!librarySaved} onclick={()=>onConfigOpen(librarySaved)}>Открыть библиотеку</button></div><div class="strategy-import-sandbox-handoff"><label>Профиль для Sandbox<select id="strategyImportSandboxProfile" class="search" bind:value={profileIndex} disabled={!canImport}>{#each parsed?.profiles||[] as _,i (i)}<option value={String(i)}>Профиль {i+1} из {parsed.profiles.length}</option>{/each}</select></label><button id="strategyImportSandboxOpen" disabled={!canImport} onclick={sendSandbox}>Проверить в песочнице</button></div><div class="strategy-import-strategy-import"><button id="strategyImportStrategyImport" disabled={!canImport} onclick={importProfiles}>Добавить профили в библиотеку стратегий</button><span id="strategyImportStrategyImportState" class="config-badge">{strategyPhase}</span></div><div class="strategy-import-note">Только inactive Config Library и Strategy Library; active nfqws2.conf не меняется. Library API не обеспечивает атомарный CAS.</div></div><div><div class="strategy-import-library-diff-title">Точный diff относительно active config</div><pre id="strategyImportLibraryDiff" class="strategy-import-library-diff">{#each diff as row,i (i)}<span class={row.kind}>{row.text}{'\n'}</span>{:else}Diff появится после READY preview.{/each}</pre></div></div></article>
</div>
