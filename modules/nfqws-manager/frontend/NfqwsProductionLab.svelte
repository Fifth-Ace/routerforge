<script>
  import {onMount} from 'svelte';
  import {diagnosticRoute,diagnosticTarget,labBounds,labResolveBody,labResolvedTargets,labStatistics,labRowView} from './NfqwsDomainLabModel.js';
  let {active=false,seed={seq:0},serviceBusy=false,onRequest=async()=>{},onHistory=()=>{},onNotice=()=>{}}=$props();
  let attempted=$state(false),loading=$state(false),running=$state(false),state=$state('ОЖИДАНИЕ'),error=$state('');
  let source=$state('manual'),listName=$state(''),targetsText=$state(''),rounds=$state('3'),concurrency=$state('4');
  let targetSources=$state([]),targets=$state([]),rows=$state([]),done=$state(0),total=$state(0);
  let alive=false,epoch=0,abort=null,lastSeed=0;
  const locked=$derived(serviceBusy||loading||running);
  const stats=$derived(labStatistics(rows,done,total));
  function notice(text,type=''){if(alive)onNotice({text:String(text),type});}
  async function request(path,method='GET',body,signal){diagnosticRoute(path,method);return onRequest(path,{method,...(body===undefined?{}:{body:JSON.stringify(body)}),...(signal?{signal}:{})});}
  function cancel(){++epoch;abort?.abort();abort=null;running=false;loading=false;}
  async function load(){
    if(!alive||!active||loading||running)return;
    const n=++epoch,c=new AbortController();abort?.abort();abort=c;loading=true;attempted=true;error='';
    try{const result=await request('/v2/target-sources','GET',undefined,c.signal);if(!alive||n!==epoch||c.signal.aborted)return;
      if(!Array.isArray(result?.sources))throw Error('NFQWS_LAB_SOURCES_INVALID');
      targetSources=result.sources.filter(x=>typeof x?.name==='string');if(!targetSources.some(x=>x.name===listName))listName=targetSources[0]?.name||'';
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted)error=String(e?.message||e);}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  async function resolve(c,n){const input=labResolveBody(source,listName,targetsText,targetSources);const data=await request('/v2/targets/resolve','POST',input,c.signal);if(!alive||n!==epoch||c.signal.aborted)return [];
    const resolved=labResolvedTargets(data);targets=resolved;targetsText=resolved.join('\n');return resolved;}
  async function resolveButton(){if(!alive||!active||locked)return;const n=++epoch,c=new AbortController();abort?.abort();abort=c;loading=true;error='';
    try{await resolve(c,n);}catch(e){if(alive&&n===epoch&&!c.signal.aborted)error=String(e?.message||e);}
    finally{if(abort===c)abort=null;if(alive&&n===epoch)loading=false;}
  }
  async function run(){
    if(!alive||!active||locked)return;
    let limits;try{limits=labBounds(rounds,concurrency);}catch(e){error=String(e.message);return;}
    const n=++epoch,c=new AbortController();abort?.abort();abort=c;running=true;state='RUNNING';error='';rows=[];done=0;
    try{
      const chosen=await resolve(c,n);if(!chosen.length||c.signal.aborted)return;
      total=chosen.length*limits.rounds;
      const work=chosen.map(target=>({target,attempts:[],inspect:null}));rows=work.map(x=>({...x}));let next=0;
      async function worker(){
        while(!c.signal.aborted){const i=next++;if(i>=work.length)return;const entry=work[i];
          try{entry.inspect=await request('/v2/inspect-target','POST',{target:entry.target},c.signal);}catch(e){if(c.signal.aborted)return;}
          for(let k=0;k<limits.rounds&&!c.signal.aborted;k++){
            try{const result=await request('/v2/detect','POST',{target:entry.target},c.signal);entry.attempts.push({ok:!!result?.ok,...result});}
            catch(e){if(c.signal.aborted)return;entry.attempts.push({ok:false,classification:'error',classification_text:String(e?.message||e)});}
            if(!alive||n!==epoch||c.signal.aborted)return;
            done++;rows=work.map(x=>({...x,attempts:[...x.attempts]}));
          }
        }
      }
      await Promise.all(Array.from({length:Math.min(limits.concurrency,chosen.length)},worker));
      if(!alive||n!==epoch||c.signal.aborted)return;
      const counts=labStatistics(work,done,total);state='DONE';
      onHistory('v2-prodlab',{targets:chosen.length,attempts:counts.ok+counts.fail,success_ratio:counts.ratio},work);
      notice(`Test Lab: ${chosen.length} целей · ${limits.rounds} повторов завершены.`,'good');
    }catch(e){if(alive&&n===epoch&&!c.signal.aborted){state='ERROR';error=String(e?.message||e);notice(`Test Lab: ${error}`,'error');}}
    finally{if(abort===c)abort=null;if(alive&&n===epoch){running=false;if(state==='RUNNING')state=c.signal.aborted?'STOPPED':'ОЖИДАНИЕ';}}
  }
  function stop(){if(!running)return;cancel();state='STOPPED';}
  onMount(()=>{alive=true;return()=>{alive=false;cancel();};});
  $effect(()=>{if(active&&!attempted&&alive)void load();});
  $effect(()=>{if(!active&&(loading||running)){cancel();state='STOPPED';}});
  $effect(()=>{if(seed?.seq&&seed.seq!==lastSeed){lastSeed=seed.seq;cancel();source='manual';try{targetsText=diagnosticTarget(seed.target)}catch{targetsText='';}targets=[];rows=[];done=0;total=0;state='ОЖИДАНИЕ';error='';}});
</script>
<div class="v2-shell" data-svelte-owned="nfqws-production-lab">
  <div class="v2-hero"><div><h2>Проверка текущей системы</h2><div class="section-kicker">несколько доменов без переключения стратегии</div></div><span id="v2LabState" class="config-badge">{state}</span></div>
  <div class="v2-two"><article class="panel"><div class="panel-head"><div><h2>Цели проверки</h2><div class="section-kicker">ручной ввод или список</div></div></div>
    <label class="v2-label">Источник<select id="v2LabSource" class="search" bind:value={source} disabled={locked}><option value="manual">Ручной ввод</option><option value="list">Список</option></select></label>
    <label class="v2-label">Список<select id="v2LabList" class="search" bind:value={listName} disabled={locked||source!=='list'}>{#each targetSources as entry (entry.name)}<option value={entry.name}>{entry.name} · {entry.count||0}</option>{:else}<option value="">Нет списков</option>{/each}</select></label>
    <textarea id="v2LabTargets" aria-label="Цели проверки по одной на строку" class="v2-textarea" placeholder="youtube.com&#10;discord.com" bind:value={targetsText} disabled={locked||source==='list'}></textarea>
    <div class="v2-inline"><label>Повторы <select id="v2LabRounds" class="search" bind:value={rounds} disabled={locked}><option value="1">1</option><option value="3">3</option><option value="5">5</option></select></label><label>Параллельно <select id="v2LabConcurrency" class="search" bind:value={concurrency} disabled={locked}><option value="1">1</option><option value="2">2</option><option value="4">4</option></select></label></div>
    <div class="toolbar"><button id="v2LabResolve" disabled={locked} onclick={resolveButton}>Загрузить цели</button><button id="v2LabRun" class="primary" disabled={locked||(!targetsText.trim()&&source!=='list')} onclick={run}>Запустить</button><button id="v2LabStop" disabled={!running} onclick={stop}>Стоп</button></div>
    {#if error}<div class="notice error" role="alert">{error}</div>{/if}
  </article><article class="panel"><div class="v2-stats"><div><span>Целей</span><strong id="v2LabCount">{targets.length}</strong></div><div><span>Успешных попыток</span><strong id="v2LabOK">{stats.ok}</strong></div><div><span>Ошибок</span><strong id="v2LabFail">{stats.fail}</strong></div><div><span>Прогресс</span><strong id="v2LabProgress">{stats.progress}</strong></div></div>
    <div id="v2LabResults" class="v2-list">{#each rows as row,i (i)}{@const view=labRowView(row)}<div class={`v2-row ${view.tone}`}><div class="v2-result-title">{view.target}</div><div><div class="value">{view.ok}/{view.total} successful · {view.last}</div><div class="meta">binding={view.binding} · lists={view.lists}</div></div><span class={`v2-pill ${view.tone}`}>{view.state}</span></div>{:else}<div class="analysis-empty">Нет результатов.</div>{/each}</div>
  </article></div><div class="v2-boundary">Проверка только на текущей конфигурации и не включает альтернативные стратегии. Запуск — по кнопке.</div>
</div>
