<script>
  import { onMount, onDestroy } from 'svelte';
  import { getAppTransactionStatus, previewAppTransaction, startAppTransaction } from '$lib/api.js';
  import { previewSelection, selectionKey, previewMatches, transactionProgress, executionRequest } from '$lib/transaction-preview.js';
  export let locale = 'ru';
  export let executionPolicy = false;
  const executionWords = {ru:{start:'Выполнить подтверждённый план',confirm:'Запустить обновление выбранных пакетов? Core может перезапуститься.',unknown:'Запрос завершился без подтверждения. Проверьте журнал. Не повторяйте запуск автоматически.',accepted:'План принят. Следите за журналом.',admitted:'Исполнение допускается для подтверждённого плана.'},en:{start:'Execute confirmed plan',confirm:'Update selected packages? Core may restart.',unknown:'Request was not confirmed. Inspect the journal. Do not retry automatically.',accepted:'Plan accepted. Follow the journal.',admitted:'Execution is admitted for the confirmed plan.'}};
  const words = {
    ru: { title:'Транзакционное обновление', open:'Открыть планировщик', close:'Свернуть', locked:'Исполнение закрыто до приёмки на оборудовании. Планирование не устанавливает пакеты.', refresh:'Обновить статус', select:'Выбрать все', clear:'Снять выбор', preview:'Проверить план', loading:'Проверяем…', unavailable:'Статус недоступен. Последний результат может быть устаревшим.', noUpdates:'Нет допустимых обновлений из подтверждённого снимка Dev.', invalid:'План отклонён или снимок изменился. Обновите статус.', source:'При исполнении: GitHub, проверенный Forgejo — резерв.', order:'Порядок установки', download:'Объём загрузки', verified:'Проверено пакетов', current:'Текущая транзакция', history:'Завершённые транзакции', empty:'Нет записей', recovery:'Требуется восстановление. Автоматический повтор запрещён.', advisory:'Локальная проверка управления — не доказательство полного lifecycle.', unknown:'Неизвестно', bytes:'байт', ready:'Management API доступен', notReady:'Management API не подтверждён', versionConflict:'Версия изменилась или не допускается', installedUnavailable:'Установленная версия не подтверждена', notChecked:'Не проверено', idle:'Нет транзакции', busy:'Занято', queued:'В очереди', preparing:'Подготовка', prepared:'Подготовлено', applying:'Установка', verifying:'Проверка', completed:'Завершено', failed:'Ошибка', cancelled:'Отменено', recovery_required:'Требуется восстановление' },
    en: { title:'Transactional update', open:'Open planner', close:'Collapse', locked:'Execution is locked until hardware acceptance. Planning does not install packages.', refresh:'Refresh status', select:'Select all', clear:'Clear selection', preview:'Check plan', loading:'Checking…', unavailable:'Status unavailable. The last result may be stale.', noUpdates:'No eligible updates from a verified Dev snapshot.', invalid:'Plan rejected or snapshot changed. Refresh status.', source:'During execution: GitHub, verified Forgejo fallback.', order:'Installation order', download:'Download size', verified:'Verified packages', current:'Current transaction', history:'Completed transactions', empty:'No records', recovery:'Recovery required. Automatic replay is prohibited.', advisory:'Local management checks are not full lifecycle proofs.', unknown:'Unknown', bytes:'bytes', ready:'Management API ready', notReady:'Management API unverified', versionConflict:'Version changed or rejected', installedUnavailable:'Installed version unverified', notChecked:'Not checked', idle:'No transaction', busy:'Busy', queued:'Queued', preparing:'Preparing', prepared:'Prepared', applying:'Installing', verifying:'Verifying', completed:'Completed', failed:'Failed', cancelled:'Cancelled', recovery_required:'Recovery required' }
  };
  const nativeLabels = { management_ready:'ready', management_unavailable:'notReady', version_conflict:'versionConflict', installed_state_unavailable:'installedUnavailable', not_checked:'notChecked' };
  let expanded = false;
  let snapshot = null;
  let selected = [];
  let preview = null;
  let statusError = false;
  let previewError = false;
  let loading = false;
  let checking = false;
  let starting = false;
  let startMessage = '';
  let generation = 0;
  let timer;
  let controller;
  let disposed = false;
  $: w = words[locale] || words.en;
  $: e = executionWords[locale] || executionWords.en;
  $: startBody = executionRequest(snapshot,preview,selected);
  $: canStart = Boolean(startBody) && !starting && !checking && !loading && !statusError;
  $: rows = previewSelection(snapshot);
  $: progress = transactionProgress(snapshot?.slot);
  $: slotState = snapshot?.slot?.state || '';
  $: canPlan = !statusError && ['idle','completed'].includes(slotState) && selected.length > 0 && !loading && !checking && !starting;

  async function refresh() {
    if (loading || starting || disposed) return;
    loading = true;
    const requestGeneration = generation;
    const requestController = new AbortController();
    controller = requestController;
    try {
      const next = await getAppTransactionStatus(requestController.signal);
      if (disposed || generation !== requestGeneration) return;
      const changed = selectionKey(snapshot) !== selectionKey(next) || snapshot?.slot?.state !== next?.slot?.state;
      snapshot = next;
      executionPolicy = next?.execution_policy_active === true;
      if (!expanded && next?.slot?.state && !['idle','completed'].includes(next.slot.state)) { expanded = true; clearInterval(timer); timer = setInterval(() => { if (!checking && !starting) void refresh(); },5000); }
      if (changed) {
        selected = selected.filter((name) => previewSelection(next).some((row) => row.package === name));
        preview = null;
      }
      statusError = false;
    } catch (error) {
      if (disposed || generation !== requestGeneration) return;
      snapshot = { ...snapshot, slot: error?.payload?.state ? { state:error.payload.state } : snapshot?.slot };
      statusError = true;
      preview = null;
    } finally {
      if (generation === requestGeneration) loading = false;
    }
  }
  function togglePanel() {
    if (starting) return;
    expanded = !expanded;
    generation++;
    clearInterval(timer);
    controller?.abort();
    loading = false;
    checking = false;
    preview = null;
    previewError = false;
    if (expanded) {
      void refresh();
      timer = setInterval(() => { if (!checking && !starting) void refresh(); }, 5000);
    }
  }
  function setSelected(names) { selected = names; preview = null; previewError = false; }
  function toggle(name) { setSelected(selected.includes(name) ? selected.filter((x) => x !== name) : [...selected,name]); }
  async function checkPlan() {
    if (!canPlan) return;
    const g = generation;
    const key = selectionKey(snapshot);
    const commit = snapshot.release_commit;
    const names = [...selected];
    checking = true;
    preview = null;
    previewError = false;
    try {
      const result = await previewAppTransaction(commit,names);
      if (disposed || g !== generation) return;
      if (key !== selectionKey(snapshot) || !previewMatches(result,commit,names) || !['idle','completed'].includes(snapshot?.slot?.state)) throw new Error('stale preview');
      preview = result;
    } catch {
      if (!disposed && g === generation) previewError = true;
    } finally {
      if (!disposed && g === generation) checking = false;
    }
  }
  async function executePlan() {
    if (!canStart || !window.confirm(e.confirm)) return;
    const body = startBody;
    starting = true; startMessage = ''; preview = null;
    try { const result = await startAppTransaction(body); if (!disposed) startMessage = result?.id === body.id ? 'accepted' : 'unknown'; }
    catch { if (!disposed) startMessage = 'unknown'; }
    finally { if (!disposed) { starting = false; await refresh(); } }
  }
  onMount(() => { void refresh(); });
  onDestroy(() => { disposed = true; generation++; clearInterval(timer); controller?.abort(); });
</script>
<section class="transaction-panel" aria-label={w.title}>
  <div class="transaction-head">
    <strong>{w.title}</strong>
    <button type="button" class="button compact" aria-expanded={expanded} onclick={togglePanel}>{expanded ? w.close : w.open}</button>
  </div>
  {#if expanded}
    <p>{snapshot?.execution_available === true ? e.admitted : w.locked}</p>
    {#if startMessage}<p role="status">{e[startMessage]}</p>{/if}
    <div class="transaction-actions">
      <button type="button" class="button compact" disabled={loading || checking || starting} onclick={refresh}>{w.refresh}</button>
      <span>{w.current}: <strong>{w[slotState] || w.unknown}</strong></span>
      {#if progress}<span>{w.verified}: {progress.verified}/{progress.total}</span>{/if}
    </div>
    {#if statusError}<p role="status">{w.unavailable}</p>{/if}
    {#if slotState === 'recovery_required'}<p role="alert">{w.recovery}</p>{/if}
    {#if snapshot?.slot?.journal?.error_code}<code>{snapshot.slot.journal.error_code}</code>{/if}
    {#if snapshot?.slot?.plan?.id}<small class="mono">{snapshot.slot.plan.id}</small>{/if}
    {#if rows.length}
      <div class="transaction-actions">
        <button type="button" class="button compact" disabled={checking || starting || statusError} onclick={() => setSelected(rows.map((row) => row.package))}>{w.select}</button>
        <button type="button" class="button compact" disabled={checking || starting} onclick={() => setSelected([])}>{w.clear}</button>
      </div>
      <div class="transaction-rows">
        {#each rows as row (row.package)}
          <label class="transaction-row">
            <input type="checkbox" checked={selected.includes(row.package)} disabled={checking || starting || statusError} onchange={() => toggle(row.package)}/>
            <span>{row.package}<small class="mono">{row.installed_version} → {row.target_version}</small></span>
          </label>
        {/each}
      </div>
      <button type="button" class="button primary" disabled={!canPlan} onclick={checkPlan}>{checking ? w.loading : w.preview} ({selected.length})</button>
    {:else if !loading && !statusError}<p>{w.noUpdates}</p>{/if}
    {#if previewError}<p role="alert">{w.invalid}</p>{/if}
    {#if preview}
      <div class="transaction-result">
        <strong>{w.order}</strong>
        <small class="mono">{preview.plan.commit.slice(0,12)} · SHA256 {preview.plan_sha256.slice(0,12)}</small>
        <span>{w.download}: {preview.download_bytes} {w.bytes}</span>
        <ol>
          {#each preview.plan.packages as pkg (pkg.name)}
            <li>{pkg.name} <small class="mono">{pkg.version}</small><small>{w[nativeLabels[preview.management?.find((entry) => entry.package === pkg.name)?.state] || 'notChecked']}</small></li>
          {/each}
        </ol>
        <p>{w.source}</p><p>{w.advisory}</p>
        <button type="button" class="button primary" disabled={!canStart} onclick={executePlan}>{e.start}</button>
      </div>
    {/if}
    <details>
      <summary>{w.history}</summary>
      {#if snapshot?.history_available === false}<p>{w.unavailable}</p>
      {:else if snapshot?.history?.length}
        {#each snapshot.history as item (item.id)}<div class="transaction-history"><span class="mono">{item.id}</span> · {w.completed} · {item.package_count}</div>{/each}
      {:else}<p>{w.empty}</p>{/if}
    </details>
  {/if}
</section>
<style>
  .transaction-panel { border:1px solid var(--border, #526073); border-radius:8px; padding:12px; margin:10px 0; display:grid; gap:8px; min-width:0; }
  .transaction-head,.transaction-actions { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
  .transaction-head { justify-content:space-between; }
  .transaction-panel p { margin:0; font-size:.88em; }
  .transaction-rows,.transaction-result { display:grid; gap:6px; }
  .transaction-row { display:flex; align-items:center; gap:8px; min-width:0; }
  .transaction-row input { width:auto; flex:none; }
  small { display:block; overflow-wrap:anywhere; opacity:.8; }
  .transaction-result ol { margin:4px 0; padding-left:24px; }
  .transaction-result li { margin-bottom:6px; overflow-wrap:anywhere; }
  .transaction-history { overflow-wrap:anywhere; padding:4px 0; }
  code { overflow-wrap:anywhere; }
</style>
