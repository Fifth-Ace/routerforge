<script>
  import { onMount } from 'svelte';
  import { getPlatformAlerts, getPlatformEvents } from '$lib/api.js';
  import { settings } from '$lib/stores/settings.js';

  const POLL_MS = 5000;

  let response = null;
  let events = [];
  let alertsResponse = null;
  let alerts = [];
  let loading = true;
  let refreshing = false;
  let errorText = '';
  let timer = null;
  let requestEpoch = 0;

  let severity = '';
  let component = '';
  let eventType = '';
  let recovery = '';
  let objectID = '';
  let transactionID = '';

  $: locale = $settings.locale || 'ru';
  $: copy = locale === 'ru' ? {
    title: 'Инциденты и события',
    subtitle: 'Единая временная шкала Core: здоровье модулей, действия сервисов и транзакции.',
    refresh: 'Обновить',
    apply: 'Применить',
    reset: 'Сбросить',
    all: 'Все',
    severity: 'Уровень',
    component: 'Компонент',
    type: 'Тип события',
    recovery: 'Восстановление',
    object: 'Объект',
    transaction: 'Транзакция',
    yes: 'Да',
    no: 'Нет',
    buffered: 'В буфере',
    shown: 'Показано',
    capacity: 'Ёмкость',
    info: 'Инфо',
    warning: 'Предупреждения',
    critical: 'Критические',
    empty: 'Событий по выбранным фильтрам нет.',
    context: 'Контекст',
    auto: 'Автообновление 5 сек',
    newest: 'Сначала новые',
    filterTransaction: 'Фильтр по транзакции',
    filterObject: 'Фильтр по объекту',
    alertsTitle: 'Активные предупреждения',
    alertsHealthy: 'Активных проблем здоровья модулей нет.',
    alertsSince: 'С',
    alertsAge: 'Возраст',
    loadError: 'Не удалось загрузить временную шкалу событий'
  } : {
    title: 'Incidents & events',
    subtitle: 'Unified Core timeline for module health, service actions and transactions.',
    refresh: 'Refresh',
    apply: 'Apply',
    reset: 'Reset',
    all: 'All',
    severity: 'Severity',
    component: 'Component',
    type: 'Event type',
    recovery: 'Recovery',
    object: 'Object',
    transaction: 'Transaction',
    yes: 'Yes',
    no: 'No',
    buffered: 'Buffered',
    shown: 'Shown',
    capacity: 'Capacity',
    info: 'Info',
    warning: 'Warnings',
    critical: 'Critical',
    empty: 'No events match the selected filters.',
    context: 'Context',
    auto: 'Auto refresh 5s',
    newest: 'Newest first',
    filterTransaction: 'Filter by transaction',
    filterObject: 'Filter by object',
    alertsTitle: 'Active health alerts',
    alertsHealthy: 'No active module health problems.',
    alertsSince: 'Since',
    alertsAge: 'Age',
    loadError: 'Failed to load the event timeline'
  };

  function query() {
    return {
      limit: 100,
      severity,
      component: component.trim(),
      type: eventType.trim(),
      recovery,
      object_id: objectID.trim(),
      transaction_id: transactionID.trim()
    };
  }

  function messageFor(error) {
    return error?.payload?.error || error?.message || copy.loadError;
  }

  async function load({ quiet = false } = {}) {
    const epoch = ++requestEpoch;
    if (quiet) refreshing = true;
    else loading = true;

    try {
      const [next, nextAlerts] = await Promise.all([getPlatformEvents(query()), getPlatformAlerts()]);
      if (epoch !== requestEpoch) return;
      response = next;
      events = Array.isArray(next?.events) ? next.events : [];
      errorText = '';
    } catch (error) {
      if (epoch !== requestEpoch) return;
      errorText = messageFor(error);
    } finally {
      if (epoch === requestEpoch) {
        loading = false;
        refreshing = false;
      }
    }
  }

  function resetFilters() {
    severity = '';
    component = '';
    eventType = '';
    recovery = '';
    objectID = '';
    transactionID = '';
    load();
  }

  function focusTransaction(id) {
    transactionID = String(id || '');
    objectID = '';
    load();
  }

  function focusObject(id) {
    objectID = String(id || '');
    transactionID = '';
    load();
  }

  function formatTime(value) {
    if (!value) return '—';
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return String(value);
    return new Intl.DateTimeFormat(locale === 'ru' ? 'ru-RU' : 'en-US', {
      dateStyle: 'short',
      timeStyle: 'medium'
    }).format(parsed);
  }

  function severityLabel(value) {
    if (value === 'critical') return copy.critical;
    if (value === 'warning') return copy.warning;
    return copy.info;
  }

  function contextEntries(context) {
    if (!context || typeof context !== 'object' || Array.isArray(context)) return [];
    return Object.entries(context).sort(([a], [b]) => a.localeCompare(b));
  }

  onMount(() => {
    load();
    timer = window.setInterval(() => load({ quiet: true }), POLL_MS);
    return () => {
      requestEpoch += 1;
      if (timer) window.clearInterval(timer);
    };
  });
</script>

<section class="incident-panel" aria-label={copy.title}>
  <div class="incident-head">
    <div>
      <div class="incident-eyebrow">P21 · EVENT ENGINE</div>
      <h2>{copy.title}</h2>
      <p>{copy.subtitle}</p>
    </div>
    <div class="incident-head-actions">
      <span class="incident-live"><i></i>{copy.auto}</span>
      <button class="incident-button" onclick={() => load({ quiet: true })} disabled={refreshing}>
        {refreshing ? '…' : '↻'} {copy.refresh}
      </button>
    </div>
  </div>

<section class="health-alerts" class:healthy={alerts.length === 0}>
    <div class="health-alerts-head">
      <strong>{copy.alertsTitle}</strong>
      <span>{alertsResponse?.status || '—'} · {alertsResponse?.active_count ?? alerts.length}</span>
    </div>
    {#if alerts.length === 0}
      <div class="health-alerts-empty">{copy.alertsHealthy}</div>
    {:else}
      <div class="health-alert-list">
        {#each alerts as alert}
          <article class="health-alert-card" data-severity={alert.severity || 'warning'}>
            <div>
              <strong>{alert.component || alert.id}</strong>
              <span>{alert.message || '—'}</span>
            </div>
            <div class="health-alert-meta">
              <span>{copy.alertsSince}: {formatTime(alert.since)}</span>
              <span>{copy.alertsAge}: {alert.age_seconds ?? 0}s</span>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <div class="incident-summary">
    <div><span>{copy.buffered}</span><strong>{response?.total_buffered ?? 0}</strong></div>
    <div><span>{copy.shown}</span><strong>{response?.count ?? events.length}</strong></div>
    <div><span>{copy.capacity}</span><strong>{response?.capacity ?? '—'}</strong></div>
    <div class="severity info"><span>{copy.info}</span><strong>{response?.severity_count?.info ?? 0}</strong></div>
    <div class="severity warning"><span>{copy.warning}</span><strong>{response?.severity_count?.warning ?? 0}</strong></div>
    <div class="severity critical"><span>{copy.critical}</span><strong>{response?.severity_count?.critical ?? 0}</strong></div>
  </div>

  <form class="incident-filters" onsubmit={(event) => { event.preventDefault(); load(); }}>
    <label>
      <span>{copy.severity}</span>
      <select bind:value={severity}>
        <option value="">{copy.all}</option>
        <option value="info">{copy.info}</option>
        <option value="warning">{copy.warning}</option>
        <option value="critical">{copy.critical}</option>
      </select>
    </label>
    <label>
      <span>{copy.component}</span>
      <input bind:value={component} placeholder="dns / admin-service / network-tools" />
    </label>
    <label>
      <span>{copy.type}</span>
      <input bind:value={eventType} placeholder="module.health.down" />
    </label>
    <label>
      <span>{copy.recovery}</span>
      <select bind:value={recovery}>
        <option value="">{copy.all}</option>
        <option value="true">{copy.yes}</option>
        <option value="false">{copy.no}</option>
      </select>
    </label>
    <label>
      <span>{copy.object}</span>
      <input bind:value={objectID} placeholder="service / module" />
    </label>
    <label>
      <span>{copy.transaction}</span>
      <input bind:value={transactionID} placeholder="tx-…" />
    </label>
    <div class="incident-filter-actions">
      <button class="incident-button primary" type="submit">{copy.apply}</button>
      <button class="incident-button" type="button" onclick={resetFilters}>{copy.reset}</button>
    </div>
  </form>

  <div class="incident-list-head">
    <strong>{copy.newest}</strong>
    {#if response?.mode}<span>{response.mode}</span>{/if}
  </div>

  {#if errorText}
    <div class="incident-state error">{errorText}</div>
  {:else if loading}
    <div class="incident-state">Loading…</div>
  {:else if events.length === 0}
    <div class="incident-state">{copy.empty}</div>
  {:else}
    <div class="incident-list">
      {#each events as event}
        <article class:recovery={event.recovery} class="incident-card">
          <div class="incident-card-rail" data-severity={event.severity || 'info'}></div>
          <div class="incident-card-main">
            <div class="incident-card-top">
              <div class="incident-badges">
                <span class="incident-badge severity" data-severity={event.severity || 'info'}>
                  {severityLabel(event.severity)}
                </span>
                <span class="incident-badge component">{event.component || 'core'}</span>
                {#if event.recovery}<span class="incident-badge recovered">RECOVERY</span>{/if}
              </div>
              <time datetime={event.time || ''}>{formatTime(event.time)}</time>
            </div>

            <div class="incident-type">{event.type || 'event'}</div>
            <div class="incident-message">{event.message || '—'}</div>

            {#if event.object_id || event.transaction_id}
              <div class="incident-links">
                {#if event.object_id}
                  <button type="button" onclick={() => focusObject(event.object_id)} title={copy.filterObject}>
                    OBJ · {event.object_id}
                  </button>
                {/if}
                {#if event.transaction_id}
                  <button type="button" onclick={() => focusTransaction(event.transaction_id)} title={copy.filterTransaction}>
                    TX · {event.transaction_id}
                  </button>
                {/if}
              </div>
            {/if}

            {#if contextEntries(event.context).length}
              <details class="incident-context">
                <summary>{copy.context}</summary>
                <dl>
                  {#each contextEntries(event.context) as [key, value]}
                    <div>
                      <dt>{key}</dt>
                      <dd>{typeof value === 'object' ? JSON.stringify(value) : String(value)}</dd>
                    </div>
                  {/each}
                </dl>
              </details>
            {/if}
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>

<style>
  .incident-panel{display:grid;gap:1rem;min-width:0}
  .incident-head{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem;padding:1.2rem;border:1px solid var(--border-color,#293346);border-radius:1rem;background:linear-gradient(135deg,rgba(59,130,246,.08),rgba(15,23,42,.02))}
  .incident-head h2{margin:.2rem 0 .3rem;font-size:1.25rem}
  .incident-head p{margin:0;max-width:52rem;opacity:.72;line-height:1.45}
  .incident-eyebrow{font-size:.68rem;letter-spacing:.14em;font-weight:800;opacity:.56}
  .incident-head-actions{display:flex;align-items:center;gap:.65rem;flex-wrap:wrap;justify-content:flex-end}
  .incident-live{display:inline-flex;align-items:center;gap:.4rem;font-size:.76rem;opacity:.72;white-space:nowrap}
  .incident-live i{width:.5rem;height:.5rem;border-radius:999px;background:#22c55e;box-shadow:0 0 0 .18rem rgba(34,197,94,.12)}
  .incident-button{border:1px solid var(--border-color,#334155);border-radius:.65rem;background:var(--panel-bg,rgba(15,23,42,.25));color:inherit;padding:.48rem .72rem;font:inherit;cursor:pointer}
  .incident-button:hover:not(:disabled){border-color:#64748b}
  .incident-button:disabled{opacity:.5;cursor:default}
  .incident-button.primary{background:rgba(59,130,246,.14);border-color:rgba(59,130,246,.5)}
.health-alerts{display:grid;gap:.65rem;padding:.9rem 1rem;border:1px solid rgba(245,158,11,.38);border-radius:.9rem;background:rgba(245,158,11,.06)}
  .health-alerts.healthy{border-color:rgba(34,197,94,.28);background:rgba(34,197,94,.045)}
  .health-alerts-head{display:flex;align-items:center;justify-content:space-between;gap:.75rem}
  .health-alerts-head span{font-size:.72rem;text-transform:uppercase;letter-spacing:.06em;opacity:.6}
  .health-alerts-empty{font-size:.82rem;opacity:.7}
  .health-alert-list{display:grid;gap:.45rem}
  .health-alert-card{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem;padding:.65rem .75rem;border:1px solid rgba(245,158,11,.28);border-radius:.7rem}
  .health-alert-card[data-severity="critical"]{border-color:rgba(239,68,68,.42);background:rgba(239,68,68,.045)}
  .health-alert-card>div:first-child{display:grid;gap:.18rem;min-width:0}
  .health-alert-card>div:first-child span{font-size:.78rem;opacity:.72;overflow-wrap:anywhere}
  .health-alert-meta{display:grid;gap:.15rem;text-align:right;font-size:.7rem;opacity:.62;white-space:nowrap}
  .incident-summary{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:.65rem}
  .incident-summary>div{display:grid;gap:.2rem;padding:.8rem .9rem;border:1px solid var(--border-color,#293346);border-radius:.8rem;min-width:0}
  .incident-summary span{font-size:.72rem;opacity:.62;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .incident-summary strong{font-size:1.08rem}
  .incident-summary .info strong{color:#60a5fa}.incident-summary .warning strong{color:#f59e0b}.incident-summary .critical strong{color:#ef4444}
  .incident-filters{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.75rem;padding:1rem;border:1px solid var(--border-color,#293346);border-radius:1rem}
  .incident-filters label{display:grid;gap:.35rem;min-width:0}
  .incident-filters label>span{font-size:.72rem;opacity:.66}
  .incident-filters input,.incident-filters select{width:100%;box-sizing:border-box;border:1px solid var(--border-color,#334155);border-radius:.6rem;background:var(--input-bg,rgba(15,23,42,.25));color:inherit;padding:.55rem .65rem;font:inherit;min-width:0}
  .incident-filter-actions{display:flex;align-items:end;gap:.5rem;grid-column:1/-1}
  .incident-list-head{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:0 .15rem}
  .incident-list-head span{font-size:.72rem;text-transform:uppercase;letter-spacing:.08em;opacity:.5}
  .incident-list{display:grid;gap:.65rem}
  .incident-card{position:relative;display:grid;grid-template-columns:.28rem minmax(0,1fr);overflow:hidden;border:1px solid var(--border-color,#293346);border-radius:.9rem;background:var(--panel-bg,rgba(15,23,42,.16))}
  .incident-card.recovery{border-color:rgba(34,197,94,.42)}
  .incident-card-rail[data-severity="info"]{background:#3b82f6}.incident-card-rail[data-severity="warning"]{background:#f59e0b}.incident-card-rail[data-severity="critical"]{background:#ef4444}
  .incident-card-main{display:grid;gap:.45rem;padding:.85rem 1rem;min-width:0}
  .incident-card-top{display:flex;align-items:flex-start;justify-content:space-between;gap:.75rem}
  .incident-card-top time{font-size:.72rem;opacity:.58;white-space:nowrap}
  .incident-badges{display:flex;align-items:center;gap:.35rem;flex-wrap:wrap}
  .incident-badge{display:inline-flex;align-items:center;border:1px solid var(--border-color,#334155);border-radius:999px;padding:.18rem .46rem;font-size:.66rem;font-weight:750;letter-spacing:.03em}
  .incident-badge.severity[data-severity="info"]{color:#60a5fa;border-color:rgba(96,165,250,.36)}
  .incident-badge.severity[data-severity="warning"]{color:#f59e0b;border-color:rgba(245,158,11,.38)}
  .incident-badge.severity[data-severity="critical"]{color:#ef4444;border-color:rgba(239,68,68,.4)}
  .incident-badge.component{opacity:.72}.incident-badge.recovered{color:#22c55e;border-color:rgba(34,197,94,.4)}
  .incident-type{font:600 .78rem ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;opacity:.68;overflow-wrap:anywhere}
  .incident-message{font-size:.94rem;line-height:1.45;overflow-wrap:anywhere}
  .incident-links{display:flex;gap:.4rem;flex-wrap:wrap}
  .incident-links button{border:0;background:transparent;color:#60a5fa;padding:0;font:600 .7rem ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;cursor:pointer;overflow-wrap:anywhere;text-align:left}
  .incident-links button:hover{text-decoration:underline}
  .incident-context{margin-top:.1rem;border-top:1px solid var(--border-color,#293346);padding-top:.45rem}
  .incident-context summary{cursor:pointer;font-size:.76rem;opacity:.68}
  .incident-context dl{display:grid;gap:.25rem;margin:.55rem 0 0}
  .incident-context dl>div{display:grid;grid-template-columns:minmax(7rem,.28fr) minmax(0,1fr);gap:.7rem}
  .incident-context dt{font:600 .7rem ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;opacity:.56}
  .incident-context dd{margin:0;font:400 .72rem ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;overflow-wrap:anywhere}
  .incident-state{padding:2rem 1rem;text-align:center;border:1px dashed var(--border-color,#334155);border-radius:.9rem;opacity:.66}
  .incident-state.error{color:#ef4444;border-color:rgba(239,68,68,.42);opacity:1}
  @media(max-width:900px){.incident-summary{grid-template-columns:repeat(3,minmax(0,1fr))}.incident-filters{grid-template-columns:repeat(2,minmax(0,1fr))}}
  @media(max-width:680px){.incident-head{flex-direction:column}.incident-head-actions{justify-content:flex-start}.incident-summary{grid-template-columns:repeat(2,minmax(0,1fr))}.incident-filters{grid-template-columns:1fr}.incident-card-top{flex-direction:column}.incident-card-top time{white-space:normal}.incident-context dl>div{grid-template-columns:1fr;gap:.1rem}}
</style>