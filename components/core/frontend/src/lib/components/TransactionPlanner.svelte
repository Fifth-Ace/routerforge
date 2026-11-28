<script>
  import { onMount } from 'svelte';
  import { catalog } from '$lib/stores/catalog.js';
  import { appUpdates, openUpdateProgress, closeUpdateProgress, refreshUpdateProgress } from '$lib/stores/app-updates.js';
  import { updateRows, transactionProgress } from '$lib/transaction-preview.js';
  export let locale='ru';
  const words={
    ru:{title:'Обновление RouterForge',history:'История обновлений',collapse:'Свернуть',logs:'Технический журнал',queued:'В очереди',preparing:'Загрузка',downloading:'Загрузка',downloaded:'Загружен',prepared:'Пакеты проверены',applying:'Установка',verifying:'Проверка',completed:'Готово',failed:'Ошибка',cancelled:'Остановлено',recovery_required:'Прервано',busy:'Занято',idle:'Нет операций',loading:'Подготовка обновления',stale:'Связь с Core потеряна. Ожидаем восстановления…',unconfirmed:'Запуск не подтверждён. Проверьте состояние; автоматически не повторяем.',installed:'Обновлено',download:'Загрузка',install:'Установка',verify:'Проверка',done:'Готово',noLog:'Журнал пока пуст',empty:'Нет записей',refresh:'Обновить статус'},
    en:{title:'RouterForge update',history:'Update history',collapse:'Minimize',logs:'Technical log',queued:'Queued',preparing:'Downloading',downloading:'Downloading',downloaded:'Downloaded',prepared:'Packages verified',applying:'Installing',verifying:'Verifying',completed:'Completed',failed:'Failed',cancelled:'Stopped',recovery_required:'Interrupted',busy:'Busy',idle:'No updates',loading:'Preparing update',stale:'Core connection lost. Waiting for it to return…',unconfirmed:'Start not confirmed. Check status; no automatic retry.',installed:'Updated',download:'Download',install:'Install',verify:'Verify',done:'Done',noLog:'No log yet',empty:'No records',refresh:'Refresh status'}
  };
  $: w=words[locale] || words.en;
  $: slot=$appUpdates.snapshot?.slot;
  $: progress=$appUpdates.launching?{verified:0,total:$appUpdates.pending.length}:transactionProgress(slot) || {verified:0,total:slot?.plan?.packages?.length || $appUpdates.pending.length || 0};
  $: percent=progress.total?Math.floor(100*progress.verified/progress.total):0;
  $: rows=$appUpdates.launching?$appUpdates.pending.map(name=>({name,state:'queued'})):updateRows(slot);
  $: phase=$appUpdates.launching?'loading':slot?.state || 'idle';
  $: visible=Boolean($appUpdates.launching || $appUpdates.open || slot?.plan || $appUpdates.error || slot?.state==='recovery_required');
  $: dev=($catalog.release?.channel==='dev' && $catalog.release?.target==='aarch64-3.10') || ($catalog.modules || []).some(m=>m.id==='routerforge-core' && String(m.version||'').startsWith('1~dev.r'));
  $: steps=[{label:w.download,done:phase==='completed'||phase==='prepared'||phase==='applying'||phase==='verifying'||(slot?.journal?.downloaded>0&&slot.journal.downloaded===progress.total),active:phase==='preparing'}, {label:w.install,done:phase==='completed',active:phase==='applying'||phase==='prepared'}, {label:w.verify,done:phase==='completed',active:phase==='verifying'},{label:w.done,done:phase==='completed',active:false}];
  let timer,controller,stopped=false;
  let now=Date.now();
  $: elapsed=slot?.journal?.started_at?Math.max(0,Math.floor(((slot.journal.finished_at?slot.journal.finished_at*1000:now)-slot.journal.started_at*1000)/1000)):0;
  $: elapsedText=`${Math.floor(elapsed/60)}:${String(elapsed%60).padStart(2,'0')}`;
  function title(name){return name.replace(/^routerforge-/,'').split('-').map(x=>x==='dns'?'DNS':x==='nfqws'?'NFQWS':x==='core'?'Core':x.charAt(0).toUpperCase()+x.slice(1)).join(' ');}
  function shortVersion(version){return String(version||'').replace(/^1~dev\.(r\d+)\..*$/,'$1');}
  async function tick(){
    if(stopped)return;
    now=Date.now();
    if(dev || $appUpdates.running || $appUpdates.launching){controller=new AbortController();await refreshUpdateProgress(controller.signal);}
    if(!stopped)timer=setTimeout(tick,$appUpdates.running || $appUpdates.launching || $appUpdates.open ? 1800:12000);
  }
  async function manualRefresh(){if(!$appUpdates.launching)await refreshUpdateProgress();}
  onMount(()=>{void tick();return()=>{stopped=true;clearTimeout(timer);controller?.abort();};});
</script>
<svelte:window onkeydown={(event)=>{if(event.key==='Escape' && $appUpdates.open)closeUpdateProgress();}} />
{#if visible}
  <button class="button compact rf-update-indicator" onclick={openUpdateProgress} aria-label={w.title}>
    <span><i class="rf-indicator-dot" class:active={$appUpdates.running || $appUpdates.launching}></i> {w[phase] || phase} {#if progress.total}{progress.verified}/{progress.total}{/if}</span>
    {#if progress.total}<progress max={100} value={percent} aria-label={w.title}></progress>{/if}
  </button>
{/if}
{#if $appUpdates.open}
  <div class="rf-update-backdrop" role="presentation"></div>
  <section class="rf-update-dialog" role="dialog" aria-modal="true" aria-label={w.title}>
    <div class="rf-update-head"><div><strong>{w.title}</strong><small>{w[phase] || phase}{#if slot?.journal?.started_at && ($appUpdates.running || slot.journal.finished_at)} · {elapsedText}{/if}</small></div><button class="button compact" onclick={closeUpdateProgress}>{w.collapse}</button></div>
    {#if $appUpdates.launching}<div class="rf-note">{w.loading}…</div>{/if}
    {#if $appUpdates.stale}<div class="rf-note" role="status">{w.stale}</div>{/if}
    {#if $appUpdates.error}<div class="rf-note error" role="alert">{$appUpdates.error==='START_UNCONFIRMED'?w.unconfirmed:$appUpdates.error}</div>{/if}
    {#if progress.total}
      <div class="rf-update-count"><strong>{progress.verified} / {progress.total}</strong><strong>{percent}%</strong></div>
      <progress class="rf-main-progress" max={100} value={percent} aria-label={w.title}></progress>
      <div class="rf-update-steps">{#each steps as step}<span class:done={step.done} class:current={step.active}>{step.done?'✓':step.active?'●':'○'} {step.label}</span>{/each}</div>
    {/if}
    <div class="rf-package-list">
      {#each rows as row (row.name)}
        <div class="rf-package-row" class:current={['downloading','applying','verifying'].includes(row.state)}>
          <span class="rf-package-icon" class:done={row.state==='completed'} class:error={row.state==='failed'}>{row.state==='completed'?'✓':row.state==='failed'?'×':['downloading','applying','verifying'].includes(row.state)?'◉':'○'}</span>
          <div><strong>{title(row.name)}</strong><small>{shortVersion(row.version)}</small></div>
          <span class="rf-package-state" class:done={row.state==='completed'} class:error={row.state==='failed'}>{row.state==='completed'?w.installed:w[row.state] || row.state}</span>
        </div>
      {/each}
    </div>
    {#if slot?.journal?.error_code}<p class="rf-note error">{w[phase] || phase}: {slot.journal.error_code}</p>{/if}
    <details><summary>{w.logs}</summary><pre>{$appUpdates.snapshot?.log || w.noLog}</pre></details>
    <details><summary>{w.history}</summary>
      {#each $appUpdates.snapshot?.history || [] as row (row.id)}<div class="rf-history"><span>{row.commit?.slice(0,12)} · {row.package_count}</span><span>{w[row.state || 'completed']}</span></div>{:else}<small>{w.empty}</small>{/each}
    </details>
    <div class="rf-update-foot"><button class="button compact" disabled={$appUpdates.launching} onclick={manualRefresh}>{w.refresh}</button><button class="button" onclick={closeUpdateProgress}>{w.collapse}</button></div>
  </section>
{/if}
<style>
  .rf-update-indicator {display:grid;gap:3px;min-width:92px;max-width:155px;font-size:.72rem;white-space:nowrap;line-height:1.2;padding:.3rem .45rem}
  .rf-update-indicator progress{width:100%;height:4px;display:block;accent-color:var(--rf-good,#52d273)}
  .rf-indicator-dot{display:inline-block;width:7px;height:7px;border-radius:50%;background:var(--rf-muted,#999);margin-right:4px}.rf-indicator-dot.active{background:var(--rf-good,#52d273);animation:rf-pulse 1s alternate infinite}
  @keyframes rf-pulse{from{opacity:.5}to{opacity:1}}
  .rf-update-backdrop{position:fixed;inset:0;z-index:1200;background:rgba(0,0,0,.52)}
  .rf-update-dialog{position:fixed;z-index:1201;top:50%;left:50%;transform:translate(-50%,-50%);width:min(570px,calc(100vw - 24px));max-height:calc(100dvh - 24px);overflow:auto;display:grid;gap:12px;padding:18px;border:1px solid var(--rf-border,var(--border,#444));border-radius:12px;background:var(--rf-panel,var(--panel,#18191d));color:var(--rf-text,var(--text,#eee));font-family:inherit;font-size:.86rem;box-shadow:0 16px 60px rgba(0,0,0,.4);white-space:normal;box-sizing:border-box}
  .rf-update-head,.rf-update-count,.rf-update-foot,.rf-history{display:flex;justify-content:space-between;align-items:center;gap:10px}.rf-update-head>div{display:grid;gap:4px}.rf-update-head strong{font-size:1.05rem}.rf-update-count{font-size:1.2rem}.rf-update-dialog small{color:var(--rf-muted,var(--muted,#aaa));font-size:.74rem}.rf-main-progress{width:100%;height:8px;accent-color:var(--rf-good,#52d273)}
  .rf-update-steps{display:flex;justify-content:space-between;flex-wrap:wrap;gap:5px;color:var(--rf-muted,var(--muted,#aaa));font-size:.75rem}.rf-package-list{display:grid;gap:3px;border-top:1px solid var(--rf-border,var(--border,#444));padding-top:6px}.rf-package-row{display:grid;grid-template-columns:18px minmax(0,1fr) auto;align-items:center;gap:8px;padding:6px;border-radius:6px}.rf-package-row.current{background:var(--rf-hover,var(--hover,#282d35))}.rf-package-row>div{display:grid;min-width:0;gap:1px}.rf-package-row strong{overflow-wrap:anywhere}.rf-package-state{font-size:.74rem;text-align:right}.done{color:var(--rf-good,#52d273)}.current{color:var(--rf-accent,#66aaff)}.error{color:var(--rf-danger,#ff6969)}.rf-note{font-size:.78rem;margin:0;overflow-wrap:anywhere;color:var(--rf-muted,var(--muted,#aaa))}.rf-note.error{color:var(--rf-danger,#ff6969)}
  details{border-top:1px solid var(--rf-border,var(--border,#444));padding-top:8px}summary{cursor:pointer;font-weight:600}.rf-history{padding:5px 0}.rf-update-dialog pre{max-height:180px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font-size:.72rem;padding:8px;background:var(--rf-hover,var(--hover,#222));border-radius:6px;margin:8px 0 0}.rf-update-foot{justify-content:flex-end}
</style>
