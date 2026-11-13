<script>
  import { onMount, tick } from 'svelte';
  import PortAccessOdin from './PortAccessOdin.svelte';
  import PortAccessEngineWorkspace from './PortAccessEngineWorkspace.svelte';
  import PortAccessEngineering from './PortAccessEngineering.svelte';
  import PortAccessStatus from './PortAccessStatus.svelte';
  import PortAccessOperationLog from './PortAccessOperationLog.svelte';

  // Native Svelte 5 workspace. No ui-rebuild/P39/P53 DOM controllers.
  // All protected actions remain owned by PortAccessOdin and
  // PortAccessEngineWorkspace. This component never writes firewall rules.
  const pages = [
    { id: 'protection', title: 'Обзор' },
    { id: 'access', title: 'Доступ' },
    { id: 'engines', title: 'Движки' },
    { id: 'diagnostics', title: 'Диагностика' }
  ];
  const methods = [
    { id: 'odin', title: 'odin · Keenetic', hint: 'Защита проброшенных портов без дополнительного демона' },
    { id: 'knockd', title: 'knockd', hint: 'Последовательность TCP-стуков, служба Entware' },
    { id: 'fwknopd', title: 'fwknopd / SPA', hint: 'Один зашифрованный пакет, служба Entware' }
  ];
  let active = $state('protection');
  let method = $state('odin');
  let engineStatus = $state('Проверяем…');
  let stateText = $state('Проверяем…');
  let configText = $state('—');
  let ndmText = $state('—');
  let statusKnown = $state(false);
  let latestService = $state({});
  let latestDeploy = $state({});
  let latestPreview = $state({});
  let accessItems = $state([]);
  let accessMessage = $state('Список ещё не загружен');
  let accessBusy = $state(false);
  let activeRequest = 0;
  let destroyed = false;
  let stateRequest = null;
  let accessRequest = null;
  let accessSerial = 0;
  const health = $derived(!statusKnown ? 'ПРОВЕРКА…'
    : /ожидает|watchdog/i.test(stateText) ? 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ'
    : /включена|подтвержден|запущен/i.test(stateText) ? 'РАБОТАЕТ'
    : 'НЕ АКТИВНА');
  const healthClass = $derived(health === 'РАБОТАЕТ' ? 'good' : health === 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ' ? 'warn' : '');

  function json(url, signal) {
    return fetch(url, { credentials:'same-origin', cache:'no-store', signal, headers:{Accept:'application/json'} })
      .then(async response => {
        const raw = await response.text();
        let data;
        try { data = raw ? JSON.parse(raw) : {}; }
        catch { throw Error('Некорректный JSON (HTTP ' + response.status + ')'); }
        if (!response.ok) throw Error(data.error || 'HTTP ' + response.status);
        return data;
      });
  }
  function show(page) {
    if (!pages.some(p => p.id === page)) return;
    active = page;
    if (page !== 'access' && accessRequest) {
      ++accessSerial;
      accessRequest.abort();
      accessRequest = null;
      accessBusy = false;
    }
    if (page === 'access' && method === 'odin') void refreshAccess();
  }
  function choose(id) {
    if (!methods.some(m => m.id === id)) return;
    method = id;
    if (id !== 'odin' && accessRequest) {
      ++accessSerial;
      accessRequest.abort();
      accessRequest = null;
      accessBusy = false;
    }
    statusKnown = false;
    active = id === 'odin' ? 'protection' : 'engines';
    // Engine selection is a reactive prop, never a synthetic DOM change event.
    void refreshState();
  }
  async function jump(targetId, page) {
    if (page) show(page);
    // Scrolling to a real section is not a command bridge. Wait for Svelte's flush.
    await tick();
    if (!destroyed) document.getElementById(targetId)?.scrollIntoView({ behavior:'smooth', block:'start' });
  }
  async function refreshState() {
    const serial = ++activeRequest;
    stateRequest?.abort();
    const controller = new AbortController();
    stateRequest = controller;
    const selected = method;
    try {
      if (selected === 'odin') {
        const data = await json('/api/modules/port-access-manager/odin', controller.signal);
        if (destroyed || serial !== activeRequest || method !== selected) return;
        if (typeof data.enabled !== 'boolean' || typeof data.confirmed !== 'boolean') throw Error('Неверный ответ odin API');
        engineStatus = 'odin · Keenetic';
        stateText = data.enabled ? (data.confirmed ? 'Включена · подтверждена' : 'Ожидает подтверждения · watchdog') : 'Выключена';
        configText = data.configured ? 'Сохранена' : 'Не настроена';
        ndmText = data.enabled ? (data.confirmed ? 'Подтверждено' : 'Watchdog · требует проверки') : 'Не активен';
      } else {
        const data = await json('/api/modules/port-access-manager/engine-service?engine=' + encodeURIComponent(selected), controller.signal);
        if (destroyed || serial !== activeRequest || method !== selected) return;
        if (typeof data.installed !== 'boolean' || typeof data.running !== 'boolean') throw Error('Неверный ответ службы');
        engineStatus = selected === 'knockd' ? 'knockd' : 'fwknopd / SPA';
        stateText = !data.installed ? 'Не установлен' : data.running ? 'Запущен' : 'Остановлен';
        configText = data.installed ? 'Служба установлена' : 'Служба отсутствует';
        ndmText = data.installed ? 'Entware · отдельное управление защитой' : 'Не активен';
      }
      statusKnown = true;
    } catch (error) {
      if (destroyed || serial !== activeRequest || method !== selected) return;
      engineStatus = selected;
      stateText = 'Нет связи с API: ' + error.message;
      configText = '—'; ndmText = '—'; statusKnown = false;
    } finally {
      if (stateRequest === controller) stateRequest = null;
    }
  }
  async function refreshAccess() {
    if (accessBusy || method !== 'odin') return;
    const serial = ++accessSerial;
    const controller = new AbortController();
    accessRequest = controller;
    accessBusy = true; accessMessage = 'Читаем KKD…';
    try {
      const data = await json('/api/modules/port-access-manager/odin-access', controller.signal);
      if (destroyed || method !== 'odin' || serial !== accessSerial) return;
      if (typeof data.available !== 'boolean') throw Error('Неверный ответ API');
      if (!data.available) {
        accessItems = [];
        accessMessage = data.reason || 'Авторизации недоступны';
        return;
      }
      if (data.list !== 'KKD' || !Array.isArray(data.entries) ||
          !data.entries.every(entry => typeof entry.ip === 'string')) throw Error('Неверный список KKD');
      accessItems = data.entries.map(entry => entry.ip);
      accessMessage = 'Активных записей: ' + accessItems.length;
    } catch (error) {
      if (!destroyed && method === 'odin' && serial === accessSerial) {
        accessItems = []; accessMessage = 'Ошибка чтения: ' + error.message;
      }
    } finally {
      if (accessRequest === controller) accessRequest = null;
      if (!destroyed && serial === accessSerial) accessBusy = false;
    }
  }
  function updateService(data) {
    if (!data || !['knockd','fwknopd'].includes(data.engine)) return;
    latestService = {...latestService, [data.engine]:data};
  }
  function updateDeploy(data) {
    if (!data || !['knockd','fwknopd'].includes(data.engine)) return;
    latestDeploy = {...latestDeploy, [data.engine]:data};
  }
  function updatePreview(data) {
    if (!data || !['knockd','fwknopd'].includes(data.engine)) return;
    latestPreview = {...latestPreview, [data.engine]:true};
  }
  function invalidatePreview() {
    if (method === 'odin') return;
    latestPreview = {...latestPreview,[method]:false};
  }
  const step = $derived.by(() => {
    if (method === 'odin') return { text:'Защита odin', detail:'Проверь настройки, сохрани их и включай защиту только после проверки доступа.', target:'odin-manager', page:'protection' };
    const service = latestService[method];
    const deploy = latestDeploy[method];
    if (!service) return {text:'Проверяем службу',detail:'Ожидаем ответ Entware',target:'existing-engine-control',page:'engines'};
    if (service.installation?.state === 'running') return {text:'Установка выполняется',detail:'Дождись завершения установки Entware',target:'existing-engine-control',page:'engines'};
    if (!service.installed) return {text:'Установить ' + method,detail:'Установка отдельно от включения защиты',target:'existing-engine-control',page:'engines'};
    if (!deploy) return {text:'Проверяем защиту',detail:'Ожидаем состояние управляемого шлюза',target:'rfpa-deploy-panel',page:'engines'};
    if (deploy.enabled && !deploy.running) return {text:'Служба остановлена',detail:'Не подтверждай защиту, пока служба не работает',target:'existing-engine-control',page:'engines'};
    if (deploy.enabled && deploy.confirmed) return {text:'Защита активна',detail:'Подтверждена; отключение доступно ниже',target:'rfpa-deploy-panel',page:'engines'};
    if (deploy.enabled) return {text:'Подтверди после внешней проверки',detail:'Watchdog откатит правила без подтверждения',target:'rfpa-confirm',page:'engines'};
    if (deploy.prepared) return {text:'Включить защиту',detail:'После включения начнётся окно автоматического отката',target:'rfpa-enable',page:'engines'};
    if (latestPreview[method]) return {text:'Подготовить конфигурацию',detail:'Проверенный план не включает firewall',target:'rfpa-prepare',page:'engines'};
    return {text:'Проверить план',detail:'Проверь WAN, порты и сервисы',target:'engine-wizard-preview',page:'engines'};
  });

  // Single Svelte root owns all five child views, polling and host callbacks.
  onMount(() => {
    const timer = window.setInterval(() => { if (!document.hidden) void refreshState(); }, 10000);
    const accessTimer = window.setInterval(() => { if (active === 'access' && method === 'odin' && !document.hidden) void refreshAccess(); }, 15000);
    const visibility = () => { if (!document.hidden) void refreshState(); };
    document.addEventListener('visibilitychange', visibility);
    void refreshState();
    void refreshAccess();
    return () => {
      destroyed = true;
      ++activeRequest;
      ++accessSerial;
      stateRequest?.abort();
      accessRequest?.abort();
      window.clearInterval(timer);
      window.clearInterval(accessTimer);
      document.removeEventListener('visibilitychange', visibility);
    };
  });
</script>

<div class="p54-root rf-pa-workspace" aria-label="Контроль доступа">
  <section class="rf-pa-summary p55-panel" aria-label="Текущее состояние защиты">
    <div class="rf-pa-summary-heading">
      <strong>Состояние защиты</strong>
      <button type="button" onclick={refreshState}>↻ Обновить состояние</button>
    </div>
    <div class="rf-pa-metrics">
      <div class="rf-pa-metric"><span>ДВИЖОК</span><b>{engineStatus}</b></div>
      <div class="rf-pa-metric"><span>ЗАЩИТА</span><b>{stateText}</b></div>
      <div class="rf-pa-metric"><span>КОНФИГУРАЦИЯ</span><b>{configText}</b></div>
      <div class="rf-pa-metric"><span>WATCHDOG / NDM</span><b>{ndmText}</b></div>
    </div>
    <div class="p55-head-actions" role="status"><span class={'p55-state ' + healthClass}>{health}</span></div>
  </section>
  <nav class="p54-navigation" aria-label="Разделы контроля доступа">
    {#each pages as page (page.id)}
      <button type="button" class="p54-tab p55-tab" class:active={active === page.id}
        aria-current={active === page.id ? 'page' : undefined} onclick={() => show(page.id)}>{page.title}</button>
    {/each}
  </nav>
  <div class="rf-pa-selector p55-panel" aria-label="Выбор способа защиты">
    <h2>Способ защиты</h2>
    <div class="rf-pa-choices">
      {#each methods as item (item.id)}
        <button type="button" class="rf-pa-choice" class:active={method === item.id} aria-pressed={method === item.id}
          onclick={() => choose(item.id)}><strong>{item.title}</strong><small>{item.hint}</small></button>
      {/each}
    </div>
  </div>
  <div class="rf-pa-next-step p55-panel">
    <strong>{step.text}</strong><p>{step.detail}</p>
    <button type="button" onclick={() => jump(step.target, step.page)}>Перейти к действию</button>
  </div>
  <section id="p54-protection" class="p54-page" aria-label="Обзор" hidden={active !== 'protection'}>
    <div class="p54-heading p55-section-heading"><h2>Настроить защиту</h2><p>Для odin сохраняй настройки отдельно; включение и подтверждение требуют явного действия.</p></div>
    <div id="pa-svelte-odin" aria-label="Управление защитой odin"><PortAccessOdin onStateChanged={refreshState} /></div>
  </section>
  <section id="p54-access" class="p54-page" aria-label="Доступ" hidden={active !== 'access'}>
    <div class="p54-heading p55-section-heading"><h2>Авторизованные IP · odin</h2><p>Текущие разрешения KKD. Истечение доступа контролирует ядро; точное время не показано.</p></div>
    <section class="panel rf-pa-odin-access">
      <div class="refresh-bar"><button type="button" disabled={accessBusy} onclick={refreshAccess}>↻ Обновить IP</button><span role="status">{accessMessage}</span></div>
      <div class="rf-pa-access-list" aria-live="polite">
        {#if method !== 'odin'}<p class="muted">Для текущего движка список доступен в разделе «Диагностика».</p>
        {:else if !accessItems.length}<p class="muted">Авторизованных IP пока нет.</p>
        {:else}{#each accessItems as ip, i (ip + ':' + i)}<div class="rf-pa-access-ip">{ip}</div>{/each}{/if}
      </div>
    </section>
    <button type="button" onclick={() => show('diagnostics')}>Диагностика и черновики</button>
  </section>
  <section id="p54-engines" class="p54-page" aria-label="Движки" hidden={active !== 'engines'}>
    <div class="p54-heading p55-section-heading"><h2>Движки Entware</h2><p>Установка, настройка, проверка плана и защищённое включение.</p></div>
    <div id="pa-svelte-engine-workspace"><PortAccessEngineWorkspace selectedEngine={method === 'odin' ? 'knockd' : method} onEngineChanged={(id) => { method = id; statusKnown = false; void refreshState(); }} onServiceState={updateService} onDeployState={updateDeploy} onPreviewReady={updatePreview} onPlanInvalidated={invalidatePreview} /></div>
  </section>
  <section id="p54-diagnostics" class="p54-page" aria-label="Диагностика" hidden={active !== 'diagnostics'}>
    <div class="p54-heading p55-section-heading"><h2>Диагностика и инструменты</h2><p>Read-only проверки, предпросмотр, журнал операций и экспериментальные черновики.</p></div>
    <div id="pa-svelte-engineering" aria-label="Инженерная диагностика и черновики"><PortAccessEngineering /></div>
    <div id="pa-svelte-status" aria-label="Диагностика доступа"><PortAccessStatus /></div>
    <div id="pa-svelte-log" aria-label="Журнал операций"><PortAccessOperationLog /></div>
  </section>
</div>
