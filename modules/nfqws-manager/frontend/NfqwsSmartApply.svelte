<script>
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
<div id="smartApplyModal" class="modal" class:open={screen.open} role="dialog" tabindex="-1" aria-modal="true" aria-label="Smart Apply / Semantic Merge" onkeydown={event=>{if(event.key==='Escape'){event.stopPropagation();void send('close');}}} onclick={event=>{if(event.target===event.currentTarget)void send('close');}}>
  <div class="dialog">
    <div class="dialog-head"><div><strong>Smart Apply / Semantic Merge</strong><div id="smartApplySource" class="section-kicker mono">{screen.source}</div></div><button id="closeSmartApply" type="button" disabled={screen.busy} onclick={()=>send('close')}>Закрыть</button></div>
    <div class="dialog-body">
      <div id="smartApplyPlan" class="smart-apply-plan" role="status" aria-live="polite">
        {#if screen.preparing}<div class="empty">Строю exact plan, проверяю SHA256 и зависимости…</div>
        {:else if screen.error}<div class="smart-apply-error" role="alert">{screen.error}</div>
        {:else if !screen.plan}<div class="empty">План ещё не построен.</div>
        {:else}
          <div class="smart-apply-head">
            <div class="smart-apply-stat"><span>Config</span><strong>{screen.plan.compat?.status||'—'}</strong></div>
            <div class="smart-apply-stat"><span>Dependencies</span><strong>{(screen.plan.lists||[]).length+(screen.plan.blobs||[]).length}</strong></div>
            <div class="smart-apply-stat"><span>Writes</span><strong>{screen.writes+1}</strong></div>
            <div class="smart-apply-stat"><span>Runtime</span><strong>{screen.plan.wasRunning?'RESTART':'STOPPED'}</strong></div>
          </div>
          {#if screen.blocked.length}<div class="smart-apply-error" role="alert">BLOCKED: {screen.blocked.map(x=>x.name).join(', ')}. Для локального кандидата источник отсутствующих зависимостей не угадывается.</div>{/if}
          <div class="smart-apply-warning"><b>TOCTOU guard:</b> apply разрешён только при точном active config SHA256 {String(screen.plan.expectedActiveSha||'').slice(0,16)}… Candidate SHA256 {String(screen.plan.configSha||'').slice(0,16)}…</div>
          <div class="smart-apply-section"><div class="smart-apply-section-head"><span>Semantic merge · сохраняем локальные runtime/environment параметры</span><span>{screen.plan.merge?.preserved?.length||0}</span></div><div class="smart-apply-rows">
            {#each screen.plan.merge?.preserved||[] as item (item.key)}
              <div class="smart-apply-row"><code>{item.key}</code><span>candidate: {item.candidate} → local: {item.active}</span><span class="smart-apply-state ok">PRESERVE</span></div>
            {:else}<div class="smart-apply-row"><span>Локальные environment assignments уже совпадают.</span><span></span><span class="smart-apply-state ok">SAME</span></div>{/each}
          </div></div>
          {#each [['Lists',screen.plan.lists||[]],['Blobs',screen.plan.blobs||[]]] as section (section[0])}
            <div class="smart-apply-section"><div class="smart-apply-section-head"><span>{section[0]}</span><span>{section[1].length}</span></div><div class="smart-apply-rows">
              {#each section[1] as item (item.name)}
                <div class="smart-apply-row"><code>{item.name}</code><span>{item.reason}</span><span class={'smart-apply-state '+item.state}>{String(item.state||'').toUpperCase()}</span></div>
              {:else}<div class="smart-apply-row"><span>Нет dependencies</span><span></span><span class="smart-apply-state ok">PASS</span></div>{/each}
            </div></div>
          {/each}
          <div class="smart-apply-warning"><b>Commit order:</b> dependencies → canonical config → {screen.plan.wasRunning?'controlled restart':'без запуска сервиса'} → SHA/runtime verify. Первый FAIL откатывает затронутые зависимости и конфигурацию.</div>
        {/if}
      </div>
    </div>
    <div class="dialog-foot"><span class="muted small">Одна транзакция: dependencies → config → restart(if running) → verify. Любой FAIL запускает rollback.</span><div class="toolbar"><button id="refreshSmartApply" type="button" disabled={!screen.open||screen.busy||screen.preparing||localBusy} onclick={()=>send('refresh')}>Пересчитать</button><button id="applySmartApply" type="button" class="primary" disabled={!screen.canApply||localBusy} onclick={()=>send('apply')}>Применить план</button></div></div>
  </div>
</div>
