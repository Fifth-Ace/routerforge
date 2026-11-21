<script>
  import { rfUi } from './rf-localization.js';
  // Only this Svelte component owns the editable cron row inputs.
  let {controls = $bindable(), rows = $bindable(), schedulerError, schedulerLoaded, labels, onReload, onChanged, onReset, onApply} = $props();
</script>
      <section class="panel scheduler-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("ПЛАНИРОВЩИК","SCHEDULER")}</span>
            <h2>{rfUi("Расписание Antiscan","Antiscan schedule")}</h2>
            <p>{rfUi("Редактор управляет только штатными задачами из","The editor only manages upstream scheduled tasks from")} <span class="mono">VALID_TASKS</span>{rfUi(". Произвольные команды и shell-текст не принимаются.",". Arbitrary commands and shell code are not accepted.")}</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="schedulerState" class={controls["schedulerState"].className} hidden={controls["schedulerState"].hidden}>{#if controls["schedulerState"].textContent !== undefined}{controls["schedulerState"].textContent}{:else}{rfUi("НЕ ЗАГРУЖЕНО","NOT LOADED")}{/if}</span>
            <button id="reloadScheduler" class={controls["reloadScheduler"].className} type="button" hidden={controls["reloadScheduler"].hidden} disabled={controls["reloadScheduler"].disabled} onclick={onReload}>{#if controls["reloadScheduler"].textContent !== undefined}{controls["reloadScheduler"].textContent}{:else}{rfUi("↻ Обновить","↻ Refresh")}{/if}</button>
          </div>
        </div>
        <div id="schedulerMeta" class={controls["schedulerMeta"].className} hidden={controls["schedulerMeta"].hidden}>{#if controls["schedulerMeta"].textContent !== undefined}{controls["schedulerMeta"].textContent}{:else}{rfUi("Откройте вкладку, чтобы загрузить ascn_crontab.conf и активный crontab.","Open this tab to load ascn_crontab.conf and the active crontab.")}{/if}</div>
        <div id="schedulerTaskRows" class={controls["schedulerTaskRows"].className} hidden={controls["schedulerTaskRows"].hidden}>
{#if schedulerError}<div class="notice bad">{schedulerError}</div>
{:else if controls["schedulerTaskRows"].textContent !== undefined}
  {controls["schedulerTaskRows"].textContent}
{:else if rows.length}
  {#each rows as item (item.task)}
  <article class="scheduler-row" data-scheduler-task={item.task} data-automatic={String(item.automatic)}>
    <label class="scheduler-toggle">
      <input data-scheduler-enabled type="checkbox" role="switch" bind:checked={item.enabled} disabled={item.automatic} onchange={onChanged}>
      <span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span>
      <span><strong>{(labels[item.task]||[item.task])[0]}</strong><small class="mono">{item.task}</small></span>
    </label>
    <div class="scheduler-expression-wrap">
      <input data-scheduler-expression class="mono" bind:value={item.schedule} spellcheck="false" disabled={item.automatic} aria-label={'Cron '+item.task} oninput={onChanged}>
      <small>{(labels[item.task]||[item.task,'Штатная задача Antiscan.'])[1]}</small>
    </div>
    {#if item.automatic}<span class="state info">{rfUi("АВТОМАТИЧЕСКИ","AUTOMATICALLY")}</span>{:else}<span class="state neutral">{rfUi("5 полей cron","5-field cron expression")}</span>{/if}
  </article>
  {/each}
{:else if !schedulerLoaded}{rfUi("Расписание ещё не загружено.","Schedule has not been loaded yet.")}{/if}
</div>
        <div class="scheduler-note">
          <strong>{rfUi("Автоматическая задача retry_load_geo","Automatic retry_load_geo task")}</strong>
          <span>{rfUi("Этой строкой управляет сам upstream Antiscan при ошибках Geo. RouterForge показывает и сохраняет её, но не позволяет вручную подменять расписание.","The upstream Antiscan project maintains this task after Geo errors. RouterForge displays and preserves it but does not allow manual schedule replacement.")}</span>
        </div>
        <div class="scheduler-actions">
          <button id="resetScheduler" class={controls["resetScheduler"].className} type="button" disabled={controls["resetScheduler"].disabled} hidden={controls["resetScheduler"].hidden} onclick={onReset}>{#if controls["resetScheduler"].textContent !== undefined}{controls["resetScheduler"].textContent}{:else}{rfUi("Сбросить изменения","Discard changes")}{/if}</button>
          <button id="applyScheduler" class={controls["applyScheduler"].className} type="button" disabled={controls["applyScheduler"].disabled} hidden={controls["applyScheduler"].hidden} onclick={onApply}>{#if controls["applyScheduler"].textContent !== undefined}{controls["applyScheduler"].textContent}{:else}{rfUi("Применить расписание","Apply schedule")}{/if}</button>
        </div>
      </section>
