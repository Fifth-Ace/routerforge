<script>
  import { onDestroy } from 'svelte';
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let locale = 'ru';
  export let service;
  export let onSupport = () => {};
  export let onRecovery = () => {};
  let tr; $: tr = locale === 'ru' ? ((ru,en)=>ru) : ((ru,en)=>en);
  $: observation = service?.observation;
  $: nativeSupported = ['admin','dns','monitoring','network-tools','nfqws-manager','dns-router'].includes(observation?.module);
  $: recoveryPaths = [...new Set([...(service?.config_paths || []), ...(observation?.module === 'dns-router' ? ['/opt/etc/routerforge/dns-router/config.json'] : []), ...(observation?.module === 'core' ? ['/opt/etc/routerforge/logging.json','/opt/etc/routerforge/policy-catalog-v1.json'] : [])])];
  let health = null, history = null, busy = false, error = '', controller = null, alive = true;
  let bootLabels, presenceLabels, healthLabels, actionLabels, outcomeLabels;
  $: bootLabels = {eligible:tr('Разрешён по init-скрипту','Eligible by init script'),disabled:tr('Запрещён маркером','Disabled by marker'),abi_missing:tr('Нет ABI-маркера','ABI marker missing'),not_executable:tr('Скрипт неисполняемый','Script not executable'),unknown:tr('Не определён','Unknown')};
  $: presenceLabels = {observed:tr('Процесс обнаружен','Process observed'),not_detected:tr('Не обнаружен','Not detected'),unknown:tr('Не определено','Unknown'),zombie_only:tr('Только завершившиеся процессы','Only zombie/dead processes'),blocked_observed:tr('Есть процесс в состоянии D','D-state process observed')};
  $: healthLabels = {ok:tr('API отвечает: OK','API reports OK'),responded:tr('API отвечает','API responded'),degraded:tr('Ухудшение','Degraded'),warning:tr('Предупреждение','Warning'),error:tr('Ошибка','Error'),inactive:tr('Не активен','Inactive'),active:tr('Активен по статусу','Active per status'),unavailable:tr('Ответ недоступен','Response unavailable')};
  $: actionLabels = {start:tr('Запуск','Start'),stop:tr('Остановка','Stop'),restart:tr('Перезапуск','Restart')};
  $: outcomeLabels = {'service.action.committed':tr('Команда завершена','Command committed'),'service.action.failed':tr('Ошибка операции','Action failed'),'service.action.rejected':tr('Операция отклонена','Action rejected')};
  const date = value => value ? new Date(value).toLocaleString(locale === 'ru' ? 'ru-RU' : 'en-US') : '—';
  async function get(path,signal) {
    const r = await fetch(path,{method:'GET',credentials:'same-origin',cache:'no-store',redirect:'error',signal});
    if(r.status===401)markUnauthorized();
    if(!r.ok)throw new Error(`HTTP ${r.status}`);
    return r.json();
  }
  async function inspect() {
    if(busy||!alive||!observation)return;
    busy=true;error='';health=null;history=null;
    const c = new AbortController();controller=c;const id=service.id;
    const timer=setTimeout(()=>c.abort(),8000);
    try {
      // These reads are independent. A down module must not hide the action history.
      const tasks=[get('/api/platform/events?'+new URLSearchParams({component:'admin-service',object_id:id,limit:'20'}),c.signal)];
      if(nativeSupported)tasks.push(get('/api/modules/admin/service-inspector/health?'+new URLSearchParams({id}),c.signal));
      const results=await Promise.allSettled(tasks);
      if(!alive||service.id!==id)return;
      if(results[0].status==='fulfilled'){
        const v=results[0].value;
        history={readAt:new Date().toISOString(),rows:(Array.isArray(v.events)?v.events:[]).filter(e=>e.component==='admin-service'&&e.object_id===id&&['service.action.committed','service.action.failed','service.action.rejected'].includes(e.type)).slice(0,20).map(e=>({time:e.time,type:e.type,action:['start','stop','restart'].includes(e.context?.action)?e.context.action:''}))};
      }else error=tr('История недоступна.','History unavailable.');
      if(nativeSupported){if(results[1]?.status==='fulfilled'&&results[1].value.service_id===id&&results[1].value.health?.id===observation.module)health=results[1].value;else error+=(error?' ':'')+tr('Нативная проверка недоступна.','Native check unavailable.');}
    }finally{clearTimeout(timer);if(controller===c)controller=null;if(alive)busy=false;}
  }
  onDestroy(()=>{alive=false;controller?.abort()});
</script>
{#if observation}
  <section class="observation">
    <div class="head"><div><strong>{tr('Состояние и зависимости','Health and dependencies')}</strong><small>{tr('Наблюдение','Observed')}: {date(observation.observed_at)}</small></div><button class="mini" disabled={busy} onclick={inspect}>{busy ? '…' : tr('Проверить API и историю','Check API and history')}</button></div>
    <div class="facts">
      <div><small>{tr('Процессы','Processes')}</small><strong>{presenceLabels[observation.process_state]||'—'}</strong><small>{tr('Сопоставление по имени/пакету','Name/package correlation')}</small></div>
      <div><small>{tr('CPU от всей мощности','CPU share of whole system')}</small><strong>{observation.cpu_percent==null?'—':`${Number(observation.cpu_percent).toFixed(1)}%`}</strong><small>{observation.cpu_window_ms?`${(observation.cpu_window_ms/1000).toFixed(1)} s`:tr('Нужны два замера (обновите список)','Two samples required (refresh list)')}</small></div>
      <div><small>{tr('Автозапуск','Boot eligibility')}</small><strong>{bootLabels[observation.boot_eligibility]||'—'}</strong><small>{tr('Не доказательство запуска при загрузке','Not proof of startup on boot')}</small></div>
      <div><small>{tr('Нативный API','Native API')}</small><strong>{health?healthLabels[health.health.state]||'—':nativeSupported?tr('Не проверен','Not checked'):tr('Нет адаптера','No adapter')}</strong><small>{health?date(health.observed_at):tr('Независимо от наличия PID','Independent of PID presence')}</small></div>
    </div>
    {#if health && (health.health.running!==undefined || health.health.available!==undefined)}
      <p>{tr('Целевой runtime','Target runtime')}: {health.health.running==null?'—':health.health.running?tr('Активен','Active'):tr('Не активен','Inactive')} · {tr('Доступность цели','Target availability')}: {health.health.available==null?'—':health.health.available?tr('Доступна','Available'):tr('Недоступна','Unavailable')}</p>
    {/if}
    <p class="hint">{tr('CPU и RSS — оценка сопоставленных процессов; совпадения между службами возможны. Ответ API не доказывает работоспособность трафика.','CPU and RSS estimate correlated processes; services can overlap. An API response does not prove traffic delivery.')}</p>
    <details><summary>{tr('Заявленные зависимости пакета','Declared package dependencies')} · {observation.dependencies_known?observation.dependencies.length:'—'}</summary>
      {#if observation.dependencies_known}
        {#each observation.dependencies as dep}<div class="dep"><code>{dep.kind}: {dep.clause}</code><span>{dep.installed_candidates.length?tr('Найдены: ','Found: ')+dep.installed_candidates.join(', '):tr('Нет точного установленного имени','No exact installed name')}</span></div>{:else}<p>{tr('Зависимости не заявлены','No declared dependencies')}</p>{/each}
      {:else}<p>{tr('Метаданные пакета недоступны','Package metadata unavailable')}</p>{/if}
      <p class="hint">{tr('Версионные ограничения и виртуальные Provides не разрешаются; это не граф порядка запуска служб.','Version constraints and virtual Provides are not resolved; this is not a service startup order graph.')}</p>
    </details>
    <details open={history!==null}><summary>{tr('История операций Admin','Admin action history')} · {history?.rows.length??'—'}</summary>
      {#if history}
        {#each history.rows as row}<div class="dep"><span>{date(row.time)} · {actionLabels[row.action]||'—'}</span><strong>{outcomeLabels[row.type]}</strong></div>{:else}<p>{tr('В текущем буфере нет операций','No actions in current buffer')}</p>{/each}
      {:else}<p>{tr('Нажмите «Проверить API и историю».','Click Check API and history.')}</p>{/if}
      <p class="hint">{tr('Ограниченный буфер Core, сбрасывается при его перезапуске. Только наблюдавшиеся операции через Admin, не все реальные запуски.','Bounded Core process buffer; reset on Core restart. Observed Admin actions only, not every actual service start.')}</p>
    </details>
    {#if error}<p role="alert">{error}</p>{/if}
    <div class="links"><button class="mini" onclick={()=>onSupport(nativeSupported?observation.module:'')}>{nativeSupported?tr('Диагностика модуля ↗','Module diagnostics ↗'):tr('Общая диагностика ↗','General diagnostics ↗')}</button><button class="mini" disabled={!recoveryPaths.length} onclick={()=>onRecovery(recoveryPaths)}>{tr('Связанные снимки ↗','Related snapshots ↗')}</button></div>
  </section>
{/if}
<style>
 .observation{display:grid;gap:10px;border:1px solid var(--rf-border,#29313a);border-radius:6px;padding:12px;min-width:0}.head,.links,.dep{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.head,.dep{justify-content:space-between}.head>div,.facts>div{display:grid;gap:4px;min-width:0}.facts{display:grid;grid-template-columns:repeat(auto-fit,minmax(145px,1fr));gap:8px}.facts>div{background:var(--rf-bg,#0b0d10);border:1px solid var(--rf-border,#29313a);border-radius:5px;padding:8px}.observation small,.hint{color:var(--rf-muted,#8d98a4);font-size:11px}.observation p{margin:0}.dep{border-bottom:1px solid var(--rf-border,#29313a);padding:6px 0;overflow-wrap:anywhere}.observation details{min-width:0;border:1px solid var(--rf-border,#29313a);border-radius:5px;padding:8px}.observation code{overflow-wrap:anywhere}.links{justify-content:flex-end}
</style>
