<script>
  let {snapshot={runs:[],count:0,latestScore:'—',sizeKb:'0.1',autoPick:[],winner:'',guard:{}},onAction=()=>{},guardEnabled=false,guardThreshold=20,onSettings=()=>{}}=$props();
  let filter=$state('all');
  const visible=$derived((snapshot.runs||[]).filter(x=>filter==='all'||x.kind===filter));
  const guardState=$derived(snapshot.guard?.running?'PROBING':!guardEnabled?'DISARMED':snapshot.guard?.armed?'ARMED':'NO BASELINE');
  const last=$derived(snapshot.guard?.last);
  const lastText=$derived(!last?'Recovery ещё не запускался.':last.state==='ERROR'?`ERROR · ${last.error}`:
    `${last.state} · ${last.label} · reach ${Math.round((last.reach_before||0)*100)}→${Math.round((last.reach_after||0)*100)}% · stable ${Math.round((last.stable_before||0)*100)}→${Math.round((last.stable_after||0)*100)}%${Array.isArray(last.restored)?` · restored ${last.restored.length} snapshot(s)`:''}`);
</script>

<div class="run-history-shell" data-svelte-owned="nfqws-run-history">
  <div class="run-history-hero">
    <div><div class="run-history-title-row"><h2 style="margin:0">Run Metrics / Scoring / History</h2><span class="config-badge active">BROWSER LOCAL</span></div>
      <div class="muted small" style="margin-top:5px">История завершённых Test Lab и Strategy Sandbox прогонов на этом браузере. Active runtime score и Sandbox readiness считаются раздельно.</div>
    </div>
    <div class="run-history-summary">
      <div class="run-history-stat"><span>Прогонов</span><strong>{snapshot.count||0}</strong></div>
      <div class="run-history-stat"><span>Последний score</span><strong>{snapshot.latestScore||'—'}</strong></div>
      <div class="run-history-stat"><span>Хранилище</span><strong>{snapshot.sizeKb||'0.1'} KB</strong></div>
    </div>
  </div>
  <div class="run-history-controls">
    <div class="run-history-filters">
      <button class:active={filter==='all'} onclick={()=>filter='all'}>Все</button>
      <button class:active={filter==='testlab'} onclick={()=>filter='testlab'}>Тестирование</button>
      <button class:active={filter==='sandbox'} onclick={()=>filter='sandbox'}>Песочница</button>
    </div>
    <div class="toolbar"><button disabled={!snapshot.count} onclick={()=>onAction('copy')}>Копировать JSON</button><button class="danger" disabled={!snapshot.count} onclick={()=>onAction('clear')}>Очистить историю</button></div>
  </div>
  <div class="run-history-score-note"><b>Test Lab SCORE 0–100</b>: reachability 50% + stability 25% + latency 15% + decisive static-path coverage 10%. Latency получает максимум до 250 ms и плавно снижается до нуля к 2000 ms. <b>Sandbox READINESS</b> — отдельная оценка только семантической готовности candidate; active HTTP baseline в неё не входит и не выдаётся за результат candidate.</div>
  <div class="run-history-list">
    {#if !visible.length}<div class="run-history-empty">Завершённых прогонов в этом фильтре пока нет.</div>{/if}
    {#each visible as entry (entry.id)}
      <details class="run-history-item"><summary>
        <div><div class="run-history-kind {entry.kind}">{entry.label}</div><div class="run-history-time">{entry.stamp}</div></div>
        <div class="run-history-main">{entry.sha.slice(0,12)||'no config SHA'} · {entry.mode}</div>
        <div><div class="run-history-score {entry.scoreClass}">{entry.score}</div><div class="run-history-time">{entry.scoreType}</div></div>
        <div class="run-history-metrics">{#each entry.chips as chip}<span class="run-history-chip">{chip}</span>{/each}</div>
        <div class="run-history-delta {entry.delta.cls}">{entry.delta.text}</div>
      </summary>
      <div class="run-history-body"><dl class="run-history-kv"><dt>Scope</dt><dd>{entry.scope}</dd><dt>Runtime</dt><dd>{entry.running?'RUNNING':'STOPPED'}</dd><dt>Config SHA</dt><dd class="mono">{entry.sha||'—'}</dd>
        {#each entry.details as detail}<dt>{detail[0]}</dt><dd>{detail[1]}</dd>{/each}
      </dl><div class="run-history-actions"><button type="button" onclick={()=>onAction('copy-one',entry.id)}>JSON</button><button type="button" class="danger" onclick={()=>onAction('delete-one',entry.id)}>Удалить</button></div></div>
      </details>
    {/each}
  </div>
  <div class="run-history-local">Хранится в localStorage текущего браузера, максимум 24 завершённых прогона. Это не router-side archive; постоянное серверное хранилище можно вынести в отдельный этап без смешивания с safety backups.</div>
  <div class="panel" style="margin-top:14px">
    <div class="panel-head"><div><h2>Auto-pick &amp; Recovery</h2><div class="section-kicker">Measured evidence only · no blind strategy ranking</div></div><div class="toolbar"><button onclick={()=>onAction('refresh')}>Пересчитать</button></div></div>
    <div class="setup-note">Auto-pick использует только завершённые <b>Test Lab</b> прогоны реального active runtime. Sandbox readiness не считается performance evidence и в выбор не попадает.</div>
    <div class="diagnostics" style="margin-top:10px">
      {#if !snapshot.autoPick?.length}<div class="analysis-empty">Нет измеренных Test Lab run. Сначала прогони реальный active runtime хотя бы два раза на ≥3 целях.</div>
      {:else}
        {#if snapshot.winner}<div class="setup-note" style="margin-bottom:8px"><b>Auto-pick:</b> {snapshot.winner}. Это лучший из <u>измеренных active runtime</u> по median ACTIVE SCORE; выбор не применяет config автоматически.</div>
        {:else}<div class="setup-note warn" style="margin-bottom:8px">Пока нет eligible config: нужно ≥2 Test Lab runs, ≥3 targets, reach ≥75%, stable ≥60%.</div>{/if}
        {#each snapshot.autoPick as row (row.sha)}
          <div class="diagnostic" class:good={row.eligible} class:warn={!row.eligible}><div class="diagnostic-icon">{row.winner?'★':'i'}</div><div><div class="diagnostic-title">{row.name} · median {Math.round(row.median_score)}</div><div class="diagnostic-detail">runs {row.runs} · targets {row.targets} · reach {Math.round(row.reach*100)}% · stable {Math.round(row.stable*100)}% · min {Math.round(row.min_score)} · latest {Math.round(row.latest_score)}{row.eligible?' · ELIGIBLE':' · NEED MORE EVIDENCE'}</div></div></div>
        {/each}
      {/if}
    </div>
    <div class="setup-note warn" style="margin-top:10px"><b>Recovery Guard:</b> после Smart Apply повторяет последний измеренный baseline на тех же целях. При существенной регрессии может автоматически восстановить persistent pre-apply snapshots. Guard не запускается без подходящего baseline.</div>
    <div class="form-row" style="margin-top:10px">
      <div class="setup-label"><strong>Recovery Guard</strong>Порог задаётся падением reachability/stability относительно последнего baseline текущего config SHA.</div>
      <div class="form-control">
        <label class="config-check"><input type="checkbox" checked={guardEnabled} onchange={e=>onSettings(e.currentTarget.checked,guardThreshold)}> Включить автоматический recovery после Smart Apply</label>
        <div style="display:flex;gap:8px;align-items:center;margin-top:7px"><span class="muted small">Порог</span>
          <select value={String(guardThreshold)} onchange={e=>onSettings(guardEnabled,Number(e.currentTarget.value))}><option value="15">15 п.п.</option><option value="20">20 п.п.</option><option value="25">25 п.п.</option><option value="30">30 п.п.</option></select>
          <span class="config-badge" class:active={guardState==='ARMED'||guardState==='PROBING'}>{guardState}</span>
        </div>
      </div>
    </div>
    <div class="muted small" style="margin-top:8px">{lastText}</div>
  </div>
</div>
