<script>
  import { rfUi } from './rf-localization.js';
  import {nfqwsSmartApplyView} from './NfqwsSmartApplyModel.js';
  let {view={}, onAction=()=>{}}=$props();
  let screen=$derived(nfqwsSmartApplyView(view));
  let localBusy=$state(false);
  async function send(name){
    if(name==='close'){if(!screen.busy)await onAction('close');return;}
    if(localBusy||screen.busy||screen.preparing)return;
    localBusy=true;try{await onAction(name);}finally{localBusy=false;}
  }
</script>
<div id="smartApplyModal" class="modal" class:open={screen.open} role="dialog" tabindex="-1" aria-modal="true" aria-label={rfUi("Безопасное применение настроек","Safe configuration apply")} onkeydown={event=>{if(event.key==='Escape'){event.stopPropagation();void send('close');}}} onclick={event=>{if(event.target===event.currentTarget)void send('close');}}>
  <div class="dialog">
    <div class="dialog-head"><div><strong>{rfUi("Безопасное применение настроек","Safe configuration apply")}</strong><div id="smartApplySource" class="section-kicker mono">{screen.source}</div></div><button id="closeSmartApply" type="button" disabled={screen.busy} onclick={()=>send('close')}>{rfUi("Закрыть","Close")}</button></div>
    <div class="dialog-body">
      <div id="smartApplyPlan" class="smart-apply-plan" role="status" aria-live="polite">
        {#if screen.preparing}<div class="empty">{rfUi("Строю exact plan, проверяю SHA256 и зависимости…","Preparing the exact plan and checking SHA256 and dependencies…")}</div>
        {:else if screen.error}<div class="smart-apply-error" role="alert">{screen.error}</div>
        {:else if !screen.plan}<div class="empty">{rfUi("План ещё не построен.","No plan has been prepared yet.")}</div>
        {:else}
          <div class="smart-apply-head">
            <div class="smart-apply-stat"><span>{rfUi("Конфигурация","Configuration")}</span><strong>{screen.plan.compat?.status||'—'}</strong></div>
            <div class="smart-apply-stat"><span>{rfUi("Зависимости","Dependencies")}</span><strong>{(screen.plan.lists||[]).length+(screen.plan.blobs||[]).length}</strong></div>
            <div class="smart-apply-stat"><span>{rfUi("Изменения","Changes")}</span><strong>{screen.writes+1}</strong></div>
            <div class="smart-apply-stat"><span>{rfUi("Сервис","Service")}</span><strong>{screen.plan.wasRunning?'RESTART':'STOPPED'}</strong></div>
          </div>
          {#if screen.blocked.length}<div class="smart-apply-error" role="alert">{rfUi('Недостающие зависимости:','Missing dependencies:')} {screen.blocked.map(x=>x.name).join(', ')}. {rfUi('Неизвестные зависимости требуют ручной проверки.','Unknown dependencies require manual review.')}</div>{/if}
          <div class="smart-apply-warning"><b>{rfUi('Проверка согласованности:','Consistency check:')}</b> {rfUi('Применение разрешено только при совпадении SHA256 активной конфигурации ','Applying requires a matching SHA256 for the active configuration ')}{String(screen.plan.expectedActiveSha||'').slice(0,16)}… {rfUi('SHA256 подготовленного варианта ','Candidate SHA256 ')}{String(screen.plan.configSha||'').slice(0,16)}…</div>
          <div class="smart-apply-section"><div class="smart-apply-section-head"><span>{rfUi("Объединение настроек · сохраняем локальные параметры сервиса и среды","Safe merge · preserve local runtime and environment settings")}</span><span>{screen.plan.merge?.preserved?.length||0}</span></div><div class="smart-apply-rows">
            {#each screen.plan.merge?.preserved||[] as item (item.key)}
              <div class="smart-apply-row"><code>{item.key}</code><span>{rfUi("Кандидат:","Candidate:")} {item.candidate} → {rfUi("Локально:","Local:")} {item.active}</span><span class="smart-apply-state ok">{rfUi("Сохранить","Keep")}</span></div>
            {:else}<div class="smart-apply-row"><span>{rfUi("Локальные environment assignments уже совпадают.","Local environment settings already match.")}</span><span></span><span class="smart-apply-state ok">{rfUi("Совпадает","Matches")}</span></div>{/each}
          </div></div>
          {#each [['Lists',screen.plan.lists||[]],['Blobs',screen.plan.blobs||[]]] as section (section[0])}
            <div class="smart-apply-section"><div class="smart-apply-section-head"><span>{section[0]}</span><span>{section[1].length}</span></div><div class="smart-apply-rows">
              {#each section[1] as item (item.name)}
                <div class="smart-apply-row"><code>{item.name}</code><span>{item.reason}</span><span class={'smart-apply-state '+item.state}>{String(item.state||'').toUpperCase()}</span></div>
              {:else}<div class="smart-apply-row"><span>{rfUi("Нет dependencies","No dependencies")}</span><span></span><span class="smart-apply-state ok">{rfUi("Готово","Ready")}</span></div>{/each}
            </div></div>
          {/each}
          <div class="smart-apply-warning"><b>{rfUi('Порядок применения:','Apply order:')}</b> {rfUi("Зависимости → конфигурация → ","Dependencies → configuration → ")}{screen.plan.wasRunning?rfUi("управляемый перезапуск","controlled restart"):rfUi("без перезапуска","no restart")} {rfUi("→ SHA/runtime verify. Первый FAIL откатывает затронутые зависимости и конфигурацию.","→ verify checksum and service state. Any failure rolls back affected dependencies and configuration.")}</div>
        {/if}
      </div>
    </div>
    <div class="dialog-foot"><span class="muted small">{rfUi("Одна транзакция: dependencies → config → restart(if running) → verify. Любой FAIL запускает rollback.","One transaction: dependencies → configuration → restart if running → verify. Any failure triggers a rollback.")}</span><div class="toolbar"><button id="refreshSmartApply" type="button" disabled={!screen.open||screen.busy||screen.preparing||localBusy} onclick={()=>send('refresh')}>{rfUi("Пересчитать","Recalculate")}</button><button id="applySmartApply" type="button" class="primary" disabled={!screen.canApply||localBusy} onclick={()=>send('apply')}>{rfUi("Применить план","Apply plan")}</button></div></div>
  </div>
</div>
