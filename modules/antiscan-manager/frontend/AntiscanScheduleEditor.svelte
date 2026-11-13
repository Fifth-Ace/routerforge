<script>
  // Only this Svelte component owns the editable cron row inputs.
  let {controls = $bindable(), rows = $bindable(), schedulerError, schedulerLoaded, labels, onReload, onChanged, onReset, onApply} = $props();
</script>
      <section class="panel scheduler-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ПЛАНИРОВЩИК</span>
            <h2>Расписание Antiscan</h2>
            <p>Редактор управляет только штатными задачами из <span class="mono">VALID_TASKS</span>. Произвольные команды и shell-текст не принимаются.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="schedulerState" class={controls["schedulerState"].className} hidden={controls["schedulerState"].hidden}>{#if controls["schedulerState"].textContent !== undefined}{controls["schedulerState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadScheduler" class={controls["reloadScheduler"].className} type="button" hidden={controls["reloadScheduler"].hidden} disabled={controls["reloadScheduler"].disabled} onclick={onReload}>{#if controls["reloadScheduler"].textContent !== undefined}{controls["reloadScheduler"].textContent}{:else}↻ Обновить{/if}</button>
          </div>
        </div>
        <div id="schedulerMeta" class={controls["schedulerMeta"].className} hidden={controls["schedulerMeta"].hidden}>{#if controls["schedulerMeta"].textContent !== undefined}{controls["schedulerMeta"].textContent}{:else}Откройте вкладку, чтобы загрузить ascn_crontab.conf и активный crontab.{/if}</div>
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
    {#if item.automatic}<span class="state info">АВТОМАТИЧЕСКИ</span>{:else}<span class="state neutral">5 полей cron</span>{/if}
  </article>
  {/each}
{:else if !schedulerLoaded}Расписание ещё не загружено.{/if}
</div>
        <div class="scheduler-note">
          <strong>Автоматическая задача retry_load_geo</strong>
          <span>Этой строкой управляет сам upstream Antiscan при ошибках Geo. RouterForge показывает и сохраняет её, но не позволяет вручную подменять расписание.</span>
        </div>
        <div class="scheduler-actions">
          <button id="resetScheduler" class={controls["resetScheduler"].className} type="button" disabled={controls["resetScheduler"].disabled} hidden={controls["resetScheduler"].hidden} onclick={onReset}>{#if controls["resetScheduler"].textContent !== undefined}{controls["resetScheduler"].textContent}{:else}Сбросить изменения{/if}</button>
          <button id="applyScheduler" class={controls["applyScheduler"].className} type="button" disabled={controls["applyScheduler"].disabled} hidden={controls["applyScheduler"].hidden} onclick={onApply}>{#if controls["applyScheduler"].textContent !== undefined}{controls["applyScheduler"].textContent}{:else}Применить расписание{/if}</button>
        </div>
      </section>
