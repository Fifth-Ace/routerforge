<script>

  import { onMount } from 'svelte';

  import { getPlatformAlerts, getPlatformEvents } from '$lib/api.js';

  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';



  const POLL_MS = 5000;



  let events = [];

  let alerts = [];

  let loading = true;

  let refreshing = false;

  let errorText = '';

  let timer = null;

  let requestEpoch = 0;

  let category = 'all';

  let componentFilter = '';



  $: locale = $settings.locale || 'ru';

  const eventCopyKeys = ['title', 'subtitle', 'refresh', 'auto', 'current', 'healthy', 'activeProblems', 'history', 'all', 'problems', 'recoveries', 'actions', 'component', 'anyComponent', 'empty', 'details', 'since', 'loadError', 'problem', 'recovery', 'action', 'normal'];
  $: copy = Object.fromEntries(eventCopyKeys.map((key) => [key, t(locale, `monitoring.events.${key}`)]));



  $: componentOptions = [...new Set([

    ...events.map((event) => event.component).filter(Boolean),

    ...alerts.map((alert) => alert.component).filter(Boolean)

  ])].sort();



  $: visibleEvents = events.filter((event) => {

    if (componentFilter && event.component !== componentFilter) return false;

    if (category === 'problems') return eventCategory(event) === 'problem';

    if (category === 'recoveries') return eventCategory(event) === 'recovery';

    if (category === 'actions') return eventCategory(event) === 'action';

    return true;

  });



  function componentName(value) {
    const id = String(value || 'core').toLowerCase();
    const key = `monitoring.events.components.${id}`;
    const label = t(locale, key);
    return label === key ? value || 'RouterForge' : label;
  }



  function humanAlert(alert) {
    const component = String(alert?.component || '');
    if (component === 'storage') return t(locale, 'monitoring.events.alerts.storage', {
      target: alert?.context?.mount || alert?.id || t(locale, 'monitoring.events.alerts.storageFallback')
    });
    if (component === 'thermal') return t(locale, 'monitoring.events.alerts.thermal', {
      target: alert?.context?.sensor || alert?.id || t(locale, 'monitoring.events.alerts.thermalFallback'),
      temperature: alert?.context?.temp_c != null ? ` · ${Number(alert.context.temp_c).toFixed(1)} °C` : ''
    });
    if (component === 'network') return t(locale, 'monitoring.events.alerts.network', {
      target: alert?.context?.interface || alert?.id || t(locale, 'monitoring.events.alerts.networkFallback')
    });
    if (component === 'watchdog') return t(locale, 'monitoring.events.alerts.watchdog', {
      target: alert?.context?.name || alert?.id || t(locale, 'monitoring.events.alerts.watchdogFallback')
    });
    return t(locale, 'monitoring.events.alerts.default');
  }



  function eventCategory(event) {

    const type = String(event?.type || '');

    if (event?.recovery || type.includes('recovered') || type.includes('rolled_back')) return 'recovery';

    if (

      event?.severity === 'critical' ||

      event?.severity === 'warning' ||

      type.includes('.down') ||

      type.includes('.failed') ||

      type.includes('.rejected') ||

      type.includes('.ambiguous')

    ) return 'problem';

    if (type.startsWith('service.action.') || type.startsWith('transaction.')) return 'action';

    return 'normal';

  }



  function humanEvent(event) {
    const type = String(event?.type || '');
    const name = componentName(event?.component);
    const object = event?.object_id || name;
    const action = event?.context?.action || '';
    const sensor = event?.context?.sensor || object;
    const service = event?.context?.name || object;
    const textOf = (key, vars = {}) => t(locale, `monitoring.events.historyMessages.${key}`, vars);
    const result = (key, vars = {}) => ({
      title: textOf(`${key}Title`, vars),
      text: textOf(`${key}Text`, vars)
    });

    if (type === 'module.health.down') return result('healthDown', { name });
    if (type === 'module.health.recovered') return result('healthRecovered', { name });
    if (type === 'module.health.up') return result('healthUp', { name });
    if (type === 'core.ready') return result('coreReady');
    if (type === 'device.network.down' || type === 'device.network.missing') return result('networkDown', { object });
    if (type === 'device.network.recovered') return result('networkRecovered', { object });

    if (type === 'device.storage.warning' || type === 'device.storage.critical') return {
      title: textOf('storageLowTitle', { object }),
      text: event?.context?.used_pct != null
        ? textOf('storageUsed', { percent: Number(event.context.used_pct).toFixed(1) })
        : textOf('storageLowText')
    };
    if (type === 'device.storage.recovered') return {
      title: textOf('storageRecoveredTitle', { object }),
      text: event?.context?.used_pct != null
        ? textOf('storageUsed', { percent: Number(event.context.used_pct).toFixed(1) })
        : textOf('storageRecoveredText')
    };
    if (type === 'device.thermal.warning' || type === 'device.thermal.critical') return {
      title: textOf('thermalHighTitle', { sensor }),
      text: event?.context?.temp_c != null
        ? textOf('thermalValue', { temperature: Number(event.context.temp_c).toFixed(1) })
        : textOf('thermalHighText')
    };
    if (type === 'device.thermal.recovered') return {
      title: textOf('thermalRecoveredTitle', { sensor }),
      text: event?.context?.temp_c != null
        ? textOf('thermalValue', { temperature: Number(event.context.temp_c).toFixed(1) })
        : textOf('thermalRecoveredText')
    };

    if (type === 'device.watchdog.failed') return result('watchdogFailed', { service });
    if (type === 'device.watchdog.recovered') return result('watchdogRecovered', { service });
    if (type === 'service.action.committed') return {
      title: textOf('serviceCommittedTitle', { object }),
      text: action ? textOf('serviceCommittedAction', { action }) : textOf('serviceCommittedText')
    };
    if (type === 'service.action.failed') return {
      title: textOf('serviceFailedTitle', { object }),
      text: action ? textOf('serviceFailedAction', { action }) : textOf('serviceFailedText')
    };
    if (type === 'service.action.rejected') return result('serviceRejected', { object });

    if (type.startsWith('transaction.')) {
      const state = type.slice('transaction.'.length);
      const states = { committed:'txCommitted', failed:'txFailed', rolled_back:'txRolledBack', ambiguous:'txAmbiguous' };
      const translatedState = states[state] ? textOf(states[state]) : state;
      return {
        title: textOf('transactionTitle', { state: translatedState }),
        text: object ? textOf('transactionObject', { object }) : textOf('transactionText')
      };
    }
    return {
      title: event?.message || textOf('defaultTitle'),
      text: textOf('defaultText', { name })
    };
  }



  function categoryLabel(kind) {

    if (kind === 'problem') return copy.problem;

    if (kind === 'recovery') return copy.recovery;

    if (kind === 'action') return copy.action;

    return copy.normal;

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



  function contextEntries(context) {

    if (!context || typeof context !== 'object' || Array.isArray(context)) return [];

    return Object.entries(context).sort(([a], [b]) => a.localeCompare(b));

  }



  async function load({ quiet = false } = {}) {

    const epoch = ++requestEpoch;

    if (quiet) refreshing = true;

    else loading = true;

    try {

      const [timeline, health] = await Promise.all([

        getPlatformEvents({ limit: 100 }),

        getPlatformAlerts()

      ]);

      if (epoch !== requestEpoch) return;

      events = Array.isArray(timeline?.events) ? timeline.events : [];

      alerts = Array.isArray(health?.alerts) ? health.alerts : [];

      errorText = '';

    } catch (error) {

      if (epoch !== requestEpoch) return;

      errorText = error?.payload?.error || error?.message || copy.loadError;

    } finally {

      if (epoch === requestEpoch) {

        loading = false;

        refreshing = false;

      }

    }

  }



  onMount(() => {

    load();

    timer = window.setInterval(() => {

      if (!document.hidden) load({ quiet: true });

    }, POLL_MS);

    return () => {

      requestEpoch += 1;

      if (timer) window.clearInterval(timer);

    };

  });

</script>



<div class="events-view">

  <section class="panel events-intro">

    <div class="panel-head">

      <div>

        <strong>{copy.title}</strong>

        <span>{copy.subtitle}</span>

      </div>

      <div class="events-tools">

        <span class="events-live"><i></i>{copy.auto}</span>

        <button class="events-refresh" onclick={() => load({ quiet: true })} disabled={refreshing}>

          {refreshing ? '…' : '↻'} {copy.refresh}

        </button>

      </div>

    </div>

  </section>



  {#if errorText}

    <section class="panel"><div class="empty events-error">{errorText}</div></section>

  {:else if loading}

    <section class="panel"><div class="empty">Loading…</div></section>

  {:else}

    <section class:healthy={alerts.length === 0} class="panel current-state">

      <div class="panel-head">

        <div><strong>{alerts.length === 0 ? copy.current : copy.activeProblems}</strong></div>

        <span class="state-chip {alerts.length === 0 ? 'good' : 'warn'}">

          {alerts.length === 0 ? 'OK' : alerts.length}

        </span>

      </div>

      {#if alerts.length === 0}

        <div class="events-healthy"><span class="status-dot good"></span><strong>{copy.healthy}</strong></div>

      {:else}

        <div class="active-list">

          {#each alerts as alert}

            <article class="active-problem" data-severity={alert.severity || 'warning'}>

              <div>

                <strong>{componentName(alert.component)}</strong>

                <span>{humanAlert(alert)}</span>

              </div>

              <small>{copy.since} {formatTime(alert.since)}</small>

            </article>

          {/each}

        </div>

      {/if}

    </section>



    <section class="panel event-history">

      <div class="panel-head history-head">

        <div><strong>{copy.history}</strong><span>{visibleEvents.length}</span></div>

        <div class="event-component-filter">

          <label for="event-component">{copy.component}</label>

          <select id="event-component" bind:value={componentFilter}>

            <option value="">{copy.anyComponent}</option>

            {#each componentOptions as component}

              <option value={component}>{componentName(component)}</option>

            {/each}

          </select>

        </div>

      </div>



      <div class="event-filters" role="group" aria-label={copy.history}>

        <button class:active={category === 'all'} onclick={() => category = 'all'}>{copy.all}</button>

        <button class:active={category === 'problems'} onclick={() => category = 'problems'}>{copy.problems}</button>

        <button class:active={category === 'recoveries'} onclick={() => category = 'recoveries'}>{copy.recoveries}</button>

        <button class:active={category === 'actions'} onclick={() => category = 'actions'}>{copy.actions}</button>

      </div>



      {#if visibleEvents.length === 0}

        <div class="empty">{copy.empty}</div>

      {:else}

        <div class="event-list">

          {#each visibleEvents as event}

            {@const kind = eventCategory(event)}

            {@const human = humanEvent(event)}

            <article class="event-row" data-kind={kind}>

              <span class="event-marker"></span>

              <div class="event-main">

                <div class="event-topline">

                  <div class="event-meta">

                    <span class="event-kind">{categoryLabel(kind)}</span>

                    <span>{componentName(event.component)}</span>

                  </div>

                  <time datetime={event.time || ''}>{formatTime(event.time)}</time>

                </div>

                <strong class="event-title">{human.title}</strong>

                <p>{human.text}</p>

                <details class="event-details">

                  <summary>{copy.details}</summary>

                  <dl>

                    <div><dt>{t(locale, 'monitoring.events.detailLabels.type')}</dt><dd>{event.type || '—'}</dd></div>

                    {#if event.object_id}<div><dt>{t(locale, 'monitoring.events.detailLabels.object')}</dt><dd>{event.object_id}</dd></div>{/if}

                    {#if event.transaction_id}<div><dt>{t(locale, 'monitoring.events.detailLabels.transaction')}</dt><dd>{event.transaction_id}</dd></div>{/if}

                    {#each contextEntries(event.context) as [key, value]}

                      <div><dt>{key}</dt><dd>{typeof value === 'object' ? JSON.stringify(value) : String(value)}</dd></div>

                    {/each}

                  </dl>

                </details>

              </div>

            </article>

          {/each}

        </div>

      {/if}

    </section>

  {/if}

</div>



<style>

  .events-view{display:grid;gap:12px;min-width:0}

  .events-intro .panel-head{align-items:center}

  .events-tools{display:flex;align-items:center;gap:10px;flex-wrap:wrap}

  .events-live{display:inline-flex;align-items:center;gap:6px;font-size:12px;color:var(--muted)}

  .events-live i{width:7px;height:7px;border-radius:50%;background:var(--good,#22c55e)}

  .events-refresh{border:1px solid var(--line);background:var(--panel);color:var(--text);border-radius:7px;padding:7px 10px;cursor:pointer}

  .events-refresh:hover:not(:disabled){border-color:var(--accent)}

  .events-refresh:disabled{opacity:.55;cursor:default}

  .current-state{border-color:rgba(245,158,11,.35)}

  .current-state.healthy{border-color:rgba(34,197,94,.28)}

  .events-healthy{display:flex;align-items:center;gap:9px;padding:8px 0;color:var(--text)}

  .active-list{display:grid;gap:8px}

  .active-problem{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;padding:10px 12px;border:1px solid rgba(245,158,11,.35);border-radius:8px}

  .active-problem[data-severity="critical"]{border-color:rgba(239,68,68,.5)}

  .active-problem>div{display:grid;gap:3px}

  .active-problem span,.active-problem small{color:var(--muted)}

  .history-head{align-items:end}

  .history-head>div:first-child{display:flex;gap:8px;align-items:baseline}

  .event-component-filter{display:flex;align-items:center;gap:8px}

  .event-component-filter label{font-size:12px;color:var(--muted)}

  .event-component-filter select{min-width:180px;border:1px solid var(--line);background:var(--panel);color:var(--text);border-radius:7px;padding:6px 8px}

  .event-filters{display:flex;gap:6px;flex-wrap:wrap;margin:2px 0 12px}

  .event-filters button{border:1px solid var(--line);background:transparent;color:var(--muted);border-radius:999px;padding:5px 10px;cursor:pointer}

  .event-filters button.active{color:var(--text);border-color:var(--accent);background:color-mix(in srgb,var(--accent) 10%,transparent)}

  .event-list{display:grid}

  .event-row{display:grid;grid-template-columns:8px minmax(0,1fr);gap:12px;padding:13px 0;border-top:1px solid var(--line)}

  .event-row:first-child{border-top:0}

  .event-marker{width:7px;height:7px;margin-top:7px;border-radius:50%;background:var(--muted)}

  .event-row[data-kind="problem"] .event-marker{background:var(--warn,#f59e0b)}

  .event-row[data-kind="recovery"] .event-marker{background:var(--good,#22c55e)}

  .event-row[data-kind="action"] .event-marker{background:var(--accent,#3b82f6)}

  .event-main{min-width:0}

  .event-topline{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}

  .event-meta{display:flex;gap:7px;align-items:center;flex-wrap:wrap;font-size:11px;color:var(--muted)}

  .event-kind{font-weight:700;color:var(--text)}

  .event-topline time{font-size:11px;color:var(--muted);white-space:nowrap}

  .event-title{display:block;margin-top:4px;font-size:14px}

  .event-main p{margin:3px 0 0;color:var(--muted);font-size:12px;line-height:1.45}

  .event-details{margin-top:7px}

  .event-details summary{cursor:pointer;color:var(--muted);font-size:11px}

  .event-details dl{display:grid;gap:4px;margin:8px 0 0;padding:9px 10px;border:1px solid var(--line);border-radius:7px}

  .event-details dl>div{display:grid;grid-template-columns:minmax(90px,.25fr) minmax(0,1fr);gap:10px}

  .event-details dt,.event-details dd{margin:0;font:11px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;overflow-wrap:anywhere}

  .event-details dt{color:var(--muted)}

  .events-error{color:var(--error,#ef4444)}

  @media(max-width:760px){

    .events-intro .panel-head,.history-head,.active-problem,.event-topline{align-items:flex-start;flex-direction:column}

    .event-component-filter{width:100%;align-items:stretch;flex-direction:column}

    .event-component-filter select{width:100%;min-width:0}

    .event-topline time{white-space:normal}

    .event-details dl>div{grid-template-columns:1fr;gap:2px}

  }

</style>