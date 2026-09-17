<script>

  import { onMount } from 'svelte';

  import { getPlatformAlerts, getPlatformEvents } from '$lib/api.js';

  import { settings } from '$lib/stores/settings.js';



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

  $: copy = locale === 'ru' ? {

    title: 'События',

    subtitle: 'Что происходило с системой: проблемы, восстановления и действия.',

    refresh: 'Обновить',

    auto: 'Обновляется автоматически',

    current: 'Сейчас',

    healthy: 'Проблем нет. Все наблюдаемые модули отвечают.',

    activeProblems: 'Активные проблемы',

    history: 'История',

    all: 'Все',

    problems: 'Проблемы',

    recoveries: 'Восстановления',

    actions: 'Действия',

    component: 'Компонент',

    anyComponent: 'Все компоненты',

    empty: 'Подходящих событий нет.',

    details: 'Технические детали',

    since: 'с',

    loadError: 'Не удалось загрузить события',

    problem: 'Проблема',

    recovery: 'Восстановлено',

    action: 'Действие',

    normal: 'Событие'

  } : {

    title: 'Events',

    subtitle: 'What happened in the system: problems, recoveries and actions.',

    refresh: 'Refresh',

    auto: 'Updates automatically',

    current: 'Now',

    healthy: 'No problems. All observed modules are responding.',

    activeProblems: 'Active problems',

    history: 'History',

    all: 'All',

    problems: 'Problems',

    recoveries: 'Recoveries',

    actions: 'Actions',

    component: 'Component',

    anyComponent: 'All components',

    empty: 'No matching events.',

    details: 'Technical details',

    since: 'since',

    loadError: 'Failed to load events',

    problem: 'Problem',

    recovery: 'Recovered',

    action: 'Action',

    normal: 'Event'

  };



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

    const ru = {

      core: 'RouterForge',

      monitoring: 'Мониторинг',

      dns: 'DNS',

      admin: 'Управление',

      'admin-service': 'Управление',

      'network-tools': 'Сетевые инструменты',

      network: 'Сеть'

    };

    const en = {

      core: 'RouterForge',

      monitoring: 'Monitoring',

      dns: 'DNS',

      admin: 'Management',

      'admin-service': 'Management',

      'network-tools': 'Network Tools',

      network: 'Network'

    };

    return (locale === 'ru' ? ru : en)[id] || value || 'RouterForge';

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



    if (locale === 'ru') {

      if (type === 'module.health.down') return { title: `${name} перестал отвечать`, text: 'RouterForge обнаружил проблему при проверке состояния модуля.' };

      if (type === 'module.health.recovered') return { title: `${name} снова работает`, text: 'Модуль восстановил работу после недоступности.' };

      if (type === 'module.health.up') return { title: `${name} доступен`, text: 'Проверка состояния прошла успешно.' };

      if (type === 'core.ready') return { title: 'RouterForge запущен и готов', text: 'Основные службы готовы к работе.' };

      if (type === 'service.action.committed') return { title: `Действие с сервисом ${object} выполнено`, text: action ? `Операция «${action}» завершена успешно.` : 'Операция завершена успешно.' };

      if (type === 'service.action.failed') return { title: `Не удалось выполнить действие с сервисом ${object}`, text: action ? `Операция «${action}» завершилась ошибкой.` : 'Операция завершилась ошибкой.' };

      if (type === 'service.action.rejected') return { title: `Действие с сервисом ${object} отклонено`, text: 'Защитная проверка не разрешила выполнить операцию.' };

      if (type.startsWith('transaction.')) {

        const state = type.slice('transaction.'.length);

        const labels = { committed: 'завершена', failed: 'завершилась ошибкой', rolled_back: 'отменена', ambiguous: 'требует проверки' };

        return { title: `Системная операция ${labels[state] || state}`, text: object ? `Объект: ${object}.` : 'Состояние системной операции изменилось.' };

      }

      return { title: event?.message || 'Системное событие', text: `${name} сообщил об изменении состояния.` };

    }



    if (type === 'module.health.down') return { title: `${name} stopped responding`, text: 'RouterForge detected a module health problem.' };

    if (type === 'module.health.recovered') return { title: `${name} is working again`, text: 'The module recovered after being unavailable.' };

    if (type === 'module.health.up') return { title: `${name} is available`, text: 'The health check completed successfully.' };

    if (type === 'core.ready') return { title: 'RouterForge is ready', text: 'Core services are ready.' };

    if (type === 'service.action.committed') return { title: `Service action completed: ${object}`, text: action ? `Action “${action}” completed successfully.` : 'The action completed successfully.' };

    if (type === 'service.action.failed') return { title: `Service action failed: ${object}`, text: action ? `Action “${action}” failed.` : 'The action failed.' };

    if (type === 'service.action.rejected') return { title: `Service action rejected: ${object}`, text: 'A safety check prevented the action.' };

    if (type.startsWith('transaction.')) return { title: `System operation: ${type.slice('transaction.'.length)}`, text: object ? `Object: ${object}.` : 'System operation state changed.' };

    return { title: event?.message || 'System event', text: `${name} reported a state change.` };

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

                <span>{locale === 'ru' ? 'Модуль не отвечает на проверку состояния.' : 'The module is not responding to health checks.'}</span>

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

                    <div><dt>type</dt><dd>{event.type || '—'}</dd></div>

                    {#if event.object_id}<div><dt>object</dt><dd>{event.object_id}</dd></div>{/if}

                    {#if event.transaction_id}<div><dt>transaction</dt><dd>{event.transaction_id}</dd></div>{/if}

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