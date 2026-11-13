<script>
  import {nfqwsLogModel,nfqwsLogTokens} from './NfqwsLogModel.js';
  let {active=false, snapshot={tail:'',path:'—',truncated:false},filterRequest={term:'',seq:0},onNotice=()=>{}}=$props();
  let query=$state('');
  $effect(()=>{if(filterRequest.seq>0)query=String(filterRequest.term||'');});
  let mode=$state('all');
  let model=$derived(nfqwsLogModel(snapshot.tail,query,mode));
  async function copyRaw(){
    try {
      if(!navigator.clipboard?.writeText)throw Error('Clipboard API недоступен');
      await navigator.clipboard.writeText(snapshot.tail||'');
      onNotice('Журнал скопирован','good');
    } catch(e) {onNotice(String(e?.message||e),'error');}
  }
</script>
<section id="view-logs" class="view" class:active>
  <article class="panel">
    <div class="panel-head"><div><h2>Логи</h2><div id="logMeta" class="section-kicker mono">{snapshot.path || '—'}{snapshot.truncated?' · tail truncated':''}</div></div><button id="copyLog" type="button" onclick={copyRaw}>Копировать raw</button></div>
    <div id="logSummary" class="log-summary">
      <span class="log-stat"><span>Строк</span><b>{model.stats.rows}</b></span>
      <span class="log-stat error"><span>Ошибки</span><b>{model.stats.errors}</b></span>
      <span class="log-stat warn"><span>Fail</span><b>{model.stats.fails}</b></span>
      <span class="log-stat info"><span>QUIC</span><b>{model.stats.quic}</b></span>
      <span class="log-stat good"><span>TCP / UDP</span><b>{model.stats.tcp} / {model.stats.udp}</b></span>
    </div>
    <div class="logbar"><input id="logSearch" class="search" placeholder="Хост, клиент, протокол, событие…" bind:value={query}><div class="toolbar"><button type="button" id="logAll" class:primary={mode==='all'} onclick={()=>mode='all'}>Все</button><button type="button" id="logErrors" class:primary={mode==='errors'} onclick={()=>mode='errors'}>Ошибки</button><button type="button" id="logWarnings" class:primary={mode==='warnings'} onclick={()=>mode='warnings'}>Fail / warnings</button></div></div>
    <div id="log" class="log-stream">
      {#if !model.rows.length}
        <div class="log-empty">По текущему фильтру записей нет.</div>
      {:else}
        <div class="log-columns-head"><span>Время</span><span>Хост</span><span>Профиль</span><span>Клиент</span><span>Proto</span><span>Событие</span></div>
        {#each model.rows as entry, index (index)}
          <div class="log-entry {entry.time?'':'raw'} {entry.severity}">
            {#if entry.time}
              <span class="log-time">{entry.time}</span><span class="log-host" title={entry.host}>{entry.host}</span><span class="log-profile">{entry.profile}</span><span class="log-client">{entry.client}</span><span class="log-proto">{entry.proto||'—'}</span>
              <span class="log-event">{#each nfqwsLogTokens(entry.event) as token, i (i)}<span class={token.kind}>{token.text}</span>{/each}</span>
            {:else}
              <span class="log-time">RAW</span><span class="log-raw-text">{entry.raw}</span>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  </article>
</section>
