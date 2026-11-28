<script>
  import { onMount, afterUpdate } from 'svelte';
  import { fade } from 'svelte/transition';
  import { catalog } from '$lib/stores/catalog.js';
  import { appUpdates, openUpdateProgress, closeUpdateProgress, refreshUpdateProgress } from '$lib/stores/app-updates.js';
  import { progressRows, downloadStats, transactionProgress } from '$lib/transaction-preview.js';
  import { updateIndicatorState } from '$lib/app-center-ux.js';
  export let locale='ru';
  const words={
    ru:{title:'Обновление RouterForge',history:'История обновлений',collapse:'Свернуть',logs:'Технический журнал',queued:'В очереди',preparing:'Загрузка',downloading:'Загрузка',downloaded:'Загружен',prepared:'Пакеты проверены',applying:'Установка',verifying:'Проверка',completed:'Готово',failed:'Ошибка',cancelled:'Остановлено',recovery_required:'Прервано',busy:'Занято',idle:'Нет операций',loading:'Подготовка обновления',stale:'Связь с Core потеряна. Ожидаем восстановления…',unconfirmed:'Запуск не подтверждён. Проверьте состояние; автоматически не повторяем.',installed:'Обновлено',download:'Загрузка',install:'Установка',verify:'Проверка',done:'Готово',noLog:'Журнал пока пуст',empty:'Нет записей',refresh:'Обновить статус'},
    en:{title:'RouterForge update',history:'Update history',collapse:'Minimize',logs:'Technical log',queued:'Queued',preparing:'Downloading',downloading:'Downloading',downloaded:'Downloaded',prepared:'Packages verified',applying:'Installing',verifying:'Verifying',completed:'Completed',failed:'Failed',cancelled:'Stopped',recovery_required:'Interrupted',busy:'Busy',idle:'No updates',loading:'Preparing update',stale:'Core connection lost. Waiting for it to return…',unconfirmed:'Start not confirmed. Check status; no automatic retry.',installed:'Updated',download:'Download',install:'Install',verify:'Verify',done:'Done',noLog:'No log yet',empty:'No records',refresh:'Refresh status'}
  };
  $: w=words[locale] || words.en;
  $: slot=$appUpdates.snapshot?.slot;
  $: progress=$appUpdates.launching?{verified:0,total:$appUpdates.pending.length}:transactionProgress(slot) || {verified:0,total:slot?.plan?.packages?.length || $appUpdates.pending.length || 0};
  $: percent=progress.total?Math.floor(100*progress.verified/progress.total):0;
  $: downloads=$appUpdates.launching?downloadStats(null):downloadStats(slot,$appUpdates.snapshot?.downloads);
  $: rows=$appUpdates.launching?$appUpdates.pending.map(name=>({name,state:'queued'})):progressRows(slot,$appUpdates.snapshot?.downloads);
  $: downloading=phase==='preparing';
  $: installing=['applying','verifying','prepared'].includes(phase);
  $: currentPackage=rows.find(row=>['applying','verifying'].includes(row.state));
  $: phase=$appUpdates.launching?'loading':slot?.state || 'idle';
  let dismissed='';
  $: indicator=updateIndicatorState({running:$appUpdates.running,launching:$appUpdates.launching,slot,error:$appUpdates.error,now,dismissed});
  $: if ($appUpdates.launching) dismissed='';
  function dismissResult(){dismissed=slot?.plan?.id || 'launch-error';try{sessionStorage.setItem('rf:update-dismissed',dismissed);}catch{}}
  $: dev=($catalog.release?.channel==='dev' && $catalog.release?.target==='aarch64-3.10') || ($catalog.modules || []).some(m=>m.id==='routerforge-core' && String(m.version||'').startsWith('1~dev.r'));
  let timer,controller,stopped=false,polling=false,fast=false;
  let now=Date.now();
  let logElement;
  let followLog=true;
  afterUpdate(()=>{if(logElement && followLog)logElement.scrollTop=logElement.scrollHeight;});
  function logScroll(){if(logElement)followLog=logElement.scrollHeight-logElement.scrollTop-logElement.clientHeight<32;}
  function sizeText(bytes){const n=Math.max(0,Number(bytes)||0);return n<1024*1024?`${(n/1024).toFixed(0)} ${locale==='ru'?'КБ':'KB'}`:`${(n/1024/1024).toFixed(1)} ${locale==='ru'?'МБ':'MB'}`;}
  function rowPercent(row){return row.size?Math.min(100,Math.floor(100*(row.bytes||0)/row.size)):0;}
  function localizedError(code){const labels={ru:{prepare_failed:'Не удалось скачать один из пакетов',install_uncertain:'OPKG не завершил установку',verify_failed:'Не удалось подтвердить установленную версию',asset_check_failed:'Проверка файла не прошла'},en:{prepare_failed:'A package could not be downloaded',install_uncertain:'OPKG did not complete installation',verify_failed:'Installed version could not be confirmed',asset_check_failed:'Package verification failed'}};return (labels[locale]||labels.en)[code]||code;}

  $: elapsed=slot?.journal?.started_at?Math.max(0,Math.floor(((slot.journal.finished_at?slot.journal.finished_at*1000:now)-slot.journal.started_at*1000)/1000)):0;
  $: elapsedText=`${Math.floor(elapsed/60)}:${String(elapsed%60).padStart(2,'0')}`;
  function title(name){return name.replace(/^routerforge-/,'').split('-').map(x=>x==='dns'?'DNS':x==='nfqws'?'NFQWS':x==='core'?'Core':x.charAt(0).toUpperCase()+x.slice(1)).join(' ');}
  function shortVersion(version){return String(version||'').replace(/^1~dev\.(r\d+)\..*$/,'$1');}
  async function tick(){
    if(stopped || polling)return;
    polling=true;
    now=Date.now();
    if(dev || $appUpdates.running || $appUpdates.launching){controller=new AbortController();await refreshUpdateProgress(controller.signal);}
    polling=false;
    if(!stopped)timer=setTimeout(tick,$appUpdates.running || $appUpdates.launching || $appUpdates.open ? 1000:12000);
  }
  onMount(()=>{
    const unsub=appUpdates.subscribe(s=>{const next=Boolean(s.running || s.launching || s.open);if(next===fast)return;fast=next;if(!polling && !stopped){clearTimeout(timer);void tick();}});
    try{dismissed=sessionStorage.getItem('rf:update-dismissed') || '';}catch{}
    const clock=setInterval(()=>{now=Date.now();},1000);
    void tick();return()=>{stopped=true;clearInterval(clock);clearTimeout(timer);controller?.abort();unsub();};
  });
</script>
<svelte:window onkeydown={(event)=>{if(event.key==='Escape' && $appUpdates.open)closeUpdateProgress();}} />
{#if indicator}
  <div class="rf-header-update" class:result-error={indicator==='error'} class:result-ok={indicator==='success'}>
    <button class="rf-header-update-trigger" type="button" onclick={openUpdateProgress} aria-label={w.title}>
      {#if indicator==='active'}<span class="rf-header-spinner" aria-hidden="true"></span>{:else}<span aria-hidden="true">{indicator==='success'?'✓':'!'}</span>{/if}
      <span>{indicator==='error'?w.failed:indicator==='success'?w.done:(w[phase] || phase)} {#if progress.total && indicator==='active'}{progress.verified}/{progress.total}{/if}</span>
      {#if indicator==='active' && progress.total}<span class="rf-header-meter" aria-hidden="true"><span style={`width:${downloading?downloads.percent:percent}%`}></span></span>{/if}
    </button>
    {#if indicator==='error'}<button class="rf-header-dismiss" type="button" aria-label={locale==='ru'?'Закрыть уведомление':'Dismiss notification'} onclick={dismissResult}>×</button>{/if}
  </div>
{/if}
{#if $appUpdates.open}
  <div class="rf-update-backdrop" role="presentation" transition:fade={{duration:160}}></div>
  <section class="rf-update-dialog" role="dialog" aria-modal="true" aria-label={w.title} transition:fade={{duration:180}}>
    <header class="rf-update-head">
      <div><strong>{w.title}</strong><small>{w[phase] || phase}{#if !$appUpdates.launching && slot?.journal?.started_at && ($appUpdates.running || slot.journal.finished_at)} · {elapsedText}{/if}</small></div>
      <button class="button compact" onclick={closeUpdateProgress}>{$appUpdates.running || $appUpdates.launching?w.collapse:locale==='ru'?'Закрыть':'Close'}</button>
    </header>
    <div class="rf-update-body">
      {#if $appUpdates.stale}<p class="rf-note" role="status">{w.stale}</p>{/if}
      {#if $appUpdates.error}<p class="rf-note error" role="alert">{$appUpdates.error==='START_UNCONFIRMED'?w.unconfirmed:$appUpdates.error}</p>{/if}
      {#if progress.total}
        <div class="rf-progress-group">
          <div class="rf-progress-label"><strong>{w.download}</strong><strong>{downloads.percent}%</strong></div>
          <div class="rf-track" role="progressbar" aria-label={w.download} aria-valuemin={0} aria-valuemax={100} aria-valuenow={downloads.percent}><i class:flow={downloading} style={`width:${downloads.percent}%`}></i></div>
          <div class="rf-progress-caption"><span>{#if downloads.total>0}{sizeText(downloads.bytes)} / {sizeText(downloads.total)} · {downloads.finished}/{progress.total}{:else}{locale==='ru'?'Ожидание загрузки':'Waiting for download'}{/if}</span>{#if downloading}<span>{locale==='ru'?'Средняя: ':'Avg: '}{sizeText(downloads.rate)}/{locale==='ru'?'с':'s'}</span>{/if}</div>
        </div>
        <div class="rf-progress-group">
          <div class="rf-progress-label"><strong>{w.install}</strong><strong>{progress.verified}/{progress.total} · {percent}%</strong></div>
          <div class="rf-track" role="progressbar" aria-label={w.install} aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}><i style={`width:${percent}%`}></i>{#if installing && percent<100}<b style={`left:${percent}%;width:${Math.min(100-percent,100/progress.total)}%`}></b>{/if}</div>
          <div class="rf-progress-caption"><span>{#if currentPackage}<span class="rf-spinner"></span> {title(currentPackage.name)} · {w[currentPackage.state]}{:else if downloading || $appUpdates.launching}{locale==='ru'?'Ожидает установки':'Waiting to install'}{:else}{w[phase] || phase}{/if}</span></div>
        </div>
      {/if}
      <div class="rf-package-list">
        {#each rows as row (row.name)}
          <div class="rf-package-row" class:current={['downloading','applying','verifying'].includes(row.state)}>
            <span class="rf-package-icon" class:done={['completed','downloaded'].includes(row.state)} class:error={row.state==='failed'}>{#if ['downloading','applying','verifying'].includes(row.state)}<span class="rf-spinner"></span>{:else}{['completed','downloaded'].includes(row.state)?'✓':row.state==='failed'?'×':'○'}{/if}</span>
            <div class="rf-package-copy"><div><strong>{title(row.name)}</strong> <small>{shortVersion(row.version)}</small></div>
              {#if row.state==='downloading'}<div class="rf-track sub" role="progressbar" aria-label={title(row.name)} aria-valuemin={0} aria-valuemax={100} aria-valuenow={rowPercent(row)}><i class="flow" style={`width:${rowPercent(row)}%`}></i></div>{/if}
            </div>
            <span class="rf-package-state" class:done={['completed','downloaded'].includes(row.state)} class:error={row.state==='failed'}>{#if row.state==='downloading'}{rowPercent(row)}% <small>{row.source || ''}</small>{:else}{row.state==='completed'?w.installed:w[row.state] || row.state}{/if}</span>
          </div>
        {/each}
      </div>
      {#if !$appUpdates.launching && slot?.journal?.error_code}<p class="rf-note error" role="alert">{localizedError(slot.journal.error_code)}</p>{/if}
      <details><summary>{w.logs}</summary><pre bind:this={logElement} onscroll={logScroll}>{(!$appUpdates.launching && $appUpdates.snapshot?.log) || w.noLog}</pre></details>
    </div>
  </section>
{/if}
<style>
  .rf-header-update{display:inline-flex;align-items:center;gap:2px;flex:none;max-width:100%;font-family:Inter,system-ui,sans-serif;font-size:.72rem;line-height:1.2;color:var(--rf-muted,var(--muted));white-space:nowrap}
  .rf-header-update-trigger,.rf-header-dismiss{appearance:none;border:0;background:transparent;color:inherit;font:inherit;cursor:pointer;padding:3px 4px;display:inline-flex;align-items:center;gap:5px;white-space:nowrap}
  .rf-header-update-trigger:hover,.rf-header-dismiss:hover{color:var(--rf-text,var(--text))}.rf-header-update.result-ok{color:var(--rf-good,#52d273)}.rf-header-update.result-error{color:var(--rf-danger,#ff6969)}
  .rf-header-spinner{width:11px;height:11px;border:1.5px solid currentColor;border-right-color:transparent;border-radius:50%;animation:rf-spin .85s linear infinite;flex:none}
  .rf-header-meter{display:inline-block;width:26px;height:3px;border-radius:2px;background:var(--rf-hover,var(--hover));overflow:hidden}.rf-header-meter>span{display:block;height:3px;background:var(--rf-good,#52d273);transition:width .35s ease;border-radius:2px}
  @media(prefers-reduced-motion:reduce){.rf-header-spinner{animation:none}.rf-header-meter>span{transition:none}}
  .rf-update-backdrop{position:fixed;inset:0;z-index:1200;background:rgba(0,0,0,.52)}
  .rf-update-dialog{position:fixed;z-index:1201;top:50%;left:50%;transform:translate(-50%,-50%);width:min(600px,calc(100vw - 24px));max-height:min(760px,calc(100dvh - 32px));display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--rf-border,var(--border,#444));border-radius:12px;background:var(--rf-panel,var(--panel,#18191d));color:var(--rf-text,var(--text,#eee));font-family:Inter,system-ui,sans-serif;font-size:.86rem;box-shadow:0 16px 60px rgba(0,0,0,.4);white-space:normal;box-sizing:border-box;animation:rf-enter .2s ease-out}
  @keyframes rf-enter{from{transform:translate(-50%,-47%) scale(.985)}to{transform:translate(-50%,-50%) scale(1)}}
  .rf-update-head,.rf-progress-label,.rf-progress-caption{display:flex;justify-content:space-between;align-items:center;gap:10px}
  .rf-update-head{padding:12px 16px;flex:none}.rf-update-head{border-bottom:1px solid var(--rf-border,var(--border,#444))}.rf-update-head>div{display:grid;gap:4px}.rf-update-head strong{font-size:1.05rem}.rf-update-body{overflow:auto;min-height:0;display:grid;gap:12px;padding:12px 16px;scrollbar-width:thin}
  .rf-update-dialog small,.rf-progress-caption{color:var(--rf-muted,var(--muted,#aaa));font-size:.72rem}.rf-progress-group{display:grid;gap:5px}.rf-progress-label{font-size:.82rem}.rf-track{display:block;position:relative;height:8px;width:100%;border-radius:99px;background:var(--rf-hover,var(--hover,#282d35));overflow:hidden}.rf-track.mini{height:4px}.rf-track.sub{height:4px;margin-top:4px}.rf-track i{display:block;height:100%;border-radius:99px;background:var(--rf-good,#52d273);transition:width .45s cubic-bezier(.2,.8,.2,1);position:relative}.rf-track i.flow::after,.rf-track b{content:'';position:absolute;inset:0;background:repeating-linear-gradient(120deg,transparent 0 8px,rgba(255,255,255,.24) 8px 13px,transparent 13px 20px);background-size:40px 100%;animation:rf-flow .8s linear infinite}.rf-track b{right:auto;background-color:rgba(82,210,115,.19);transition:left .35s,width .35s}
  @keyframes rf-flow{to{background-position:40px 0}}
  .rf-spinner{display:inline-block;width:12px;height:12px;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:rf-spin .85s linear infinite;vertical-align:-2px;flex:none}@keyframes rf-spin{to{transform:rotate(360deg)}}
  .rf-package-list{display:grid;gap:2px;border-top:1px solid var(--rf-border,var(--border,#444));padding-top:5px}.rf-package-row{display:grid;grid-template-columns:16px minmax(0,1fr) auto;align-items:center;gap:8px;min-height:31px;padding:4px 6px;border-radius:6px;transition:background-color .2s,color .2s}.rf-package-row.current{background:var(--rf-hover,var(--hover,#282d35))}.rf-package-copy{min-width:0}.rf-package-copy strong{overflow-wrap:anywhere;font-size:.83rem}.rf-package-state{font-size:.74rem;text-align:right;white-space:nowrap}.rf-package-state small{display:block}.done{color:var(--rf-good,#52d273)}.current{color:var(--rf-accent,#66aaff)}.error{color:var(--rf-danger,#ff6969)}.rf-note{font-size:.78rem;margin:0;overflow-wrap:anywhere;color:var(--rf-muted,var(--muted,#aaa))}.rf-note.error{color:var(--rf-danger,#ff6969)}
  details{border-top:1px solid var(--rf-border,var(--border,#444));padding-top:8px}summary{cursor:pointer;font-weight:600;font-size:.8rem}.rf-update-dialog pre{max-height:145px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font-family:'Roboto Mono',monospace;font-size:.71rem;padding:8px;background:var(--rf-hover,var(--hover,#222));border-radius:6px;margin:8px 0 0}
  @media(prefers-reduced-motion:reduce){.rf-update-dialog,.rf-spinner,.rf-track i.flow::after,.rf-track b{animation:none}.rf-track i,.rf-track b,.rf-package-row{transition:none}}
</style>
