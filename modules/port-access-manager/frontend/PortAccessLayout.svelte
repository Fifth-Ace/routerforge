<script>
  import { rfUi } from './rf-localization.js';
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
    { id: 'protection', title: rfUi('Обзор',"Overview") },
    { id: 'access', title: rfUi('Доступ',"Access") },
    { id: 'engines', title: rfUi('Движки',"Engines") },
    { id: 'diagnostics', title: rfUi('Диагностика',"Diagnostics") }
  ];
  const methods = [
    { id: 'odin', title: 'odin · Keenetic', hint: rfUi('Защита проброшенных портов без дополнительного демона',"Protect forwarded ports without an additional daemon") },
    { id: 'knockd', title: 'knockd', hint: rfUi('Последовательность TCP-стуков, служба Entware',"TCP knock sequence, Entware service") },
    { id: 'fwknopd', title: 'fwknopd / SPA', hint: rfUi('Один зашифрованный пакет, служба Entware',"One encrypted packet, Entware service") }
  ];
  let active = $state('protection');
  let method = $state('odin');
  let engineStatus = $state(rfUi('Проверяем…',"Checking…"));
  let stateText = $state(rfUi('Проверяем…',"Checking…"));
  let configText = $state('—');
  let ndmText = $state('—');
  let statusKnown = $state(false);
  let latestService = $state({});
  let latestDeploy = $state({});
  let latestPreview = $state({});
  let accessItems = $state([]);
  let accessMessage = $state(rfUi('Список ещё не загружен',"List has not been loaded yet"));
  let accessBusy = $state(false);
  let activeRequest = 0;
  let destroyed = false;
  let stateRequest = null;
  let accessRequest = null;
  let accessSerial = 0;
  const health = $derived(!statusKnown ? rfUi('ПРОВЕРКА…',"CHECKING…")
    : /ожидает|watchdog/i.test(stateText) ? 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ'
    : /включена|подтвержден|запущен/i.test(stateText) ? 'РАБОТАЕТ'
    : rfUi('НЕ АКТИВНА',"INACTIVE"));
  const healthClass = $derived(health === rfUi('РАБОТАЕТ',"RUNNING") ? 'good' : health === 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ' ? 'warn' : '');

  function json(url, signal) {
    return fetch(url, { credentials:'same-origin', cache:'no-store', signal, headers:{Accept:'application/json'} })
      .then(async response => {
        const raw = await response.text();
        let data;
        try { data = raw ? JSON.parse(raw) : {}; }
        catch { throw Error(rfUi('Некорректный JSON (HTTP ',"Invalid JSON (HTTP") + response.status + ')'); }
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
        if (typeof data.enabled !== 'boolean' || typeof data.confirmed !== 'boolean') throw Error(rfUi('Неверный ответ odin API',"Invalid odin API response"));
        engineStatus = 'odin · Keenetic';
        stateText = data.enabled ? (data.confirmed ? rfUi('Включена · подтверждена',"Enabled · confirmed") : rfUi('Ожидает подтверждения · watchdog',"Awaiting confirmation · watchdog")) : rfUi('Выключена',"Disabled");
        configText = data.configured ? rfUi('Сохранена',"Saved") : rfUi('Не настроена',"Not configured");
        ndmText = data.enabled ? (data.confirmed ? rfUi('Подтверждено',"Confirmed") : rfUi('Watchdog · требует проверки',"Watchdog · verification required")) : rfUi('Не активен',"Inactive");
      } else {
        const data = await json('/api/modules/port-access-manager/engine-service?engine=' + encodeURIComponent(selected), controller.signal);
        if (destroyed || serial !== activeRequest || method !== selected) return;
        if (typeof data.installed !== 'boolean' || typeof data.running !== 'boolean') throw Error(rfUi('Неверный ответ службы',"Invalid service response"));
        engineStatus = selected === 'knockd' ? 'knockd' : 'fwknopd / SPA';
        stateText = !data.installed ? 'Не установлен' : data.running ? 'Запущен' : rfUi('Остановлен',"Stopped");
        configText = data.installed ? rfUi('Служба установлена',"Service installed") : rfUi('Служба отсутствует',"Service missing");
        ndmText = data.installed ? rfUi('Entware · отдельное управление защитой',"Entware · separate protection management") : rfUi('Не активен',"Inactive");
      }
      statusKnown = true;
    } catch (error) {
      if (destroyed || serial !== activeRequest || method !== selected) return;
      engineStatus = selected;
      stateText = rfUi('Нет связи с API: ',"Cannot reach the API:") + error.message;
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
    accessBusy = true; accessMessage = rfUi('Читаем KKD…',"Reading KKD…");
    try {
      const data = await json('/api/modules/port-access-manager/odin-access', controller.signal);
      if (destroyed || method !== 'odin' || serial !== accessSerial) return;
      if (typeof data.available !== 'boolean') throw Error(rfUi('Неверный ответ API',"Invalid API response"));
      if (!data.available) {
        accessItems = [];
        accessMessage = data.reason || rfUi('Авторизации недоступны',"Authorizations unavailable");
        return;
      }
      if (data.list !== 'KKD' || !Array.isArray(data.entries) ||
          !data.entries.every(entry => typeof entry.ip === 'string')) throw Error(rfUi('Неверный список KKD',"Invalid KKD list"));
      accessItems = data.entries.map(entry => entry.ip);
      accessMessage = rfUi('Активных записей: ',"Active entries:") + accessItems.length;
    } catch (error) {
      if (!destroyed && method === 'odin' && serial === accessSerial) {
        accessItems = []; accessMessage = rfUi('Ошибка чтения: ',"Read error:") + error.message;
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
    if (method === 'odin') return { text:rfUi('Защита odin',"odin protection"), detail:rfUi('Проверь настройки, сохрани их и включай защиту только после проверки доступа.',"Review and save settings; enable protection only after verifying access."), target:'odin-manager', page:'protection' };
    const service = latestService[method];
    const deploy = latestDeploy[method];
    if (!service) return {text:rfUi('Проверяем службу',"Checking service"),detail:rfUi('Ожидаем ответ Entware',"Waiting for Entware response"),target:'existing-engine-control',page:'engines'};
    if (service.installation?.state === 'running') return {text:rfUi('Установка выполняется',"Installing"),detail:rfUi('Дождись завершения установки Entware',"Wait for Entware installation to complete"),target:'existing-engine-control',page:'engines'};
    if (!service.installed) return {text:rfUi('Установить ',"Install") + method,detail:rfUi('Установка отдельно от включения защиты',"Installation is separate from enabling protection"),target:'existing-engine-control',page:'engines'};
    if (!deploy) return {text:rfUi('Проверяем защиту',"Checking protection"),detail:rfUi('Ожидаем состояние управляемого шлюза',"Waiting for managed gate status"),target:'rfpa-deploy-panel',page:'engines'};
    if (deploy.enabled && !deploy.running) return {text:rfUi('Служба остановлена',"Service stopped"),detail:rfUi('Не подтверждай защиту, пока служба не работает',"Do not confirm protection while the service is stopped"),target:'existing-engine-control',page:'engines'};
    if (deploy.enabled && deploy.confirmed) return {text:rfUi('Защита активна',"Protection active"),detail:rfUi('Подтверждена; отключение доступно ниже',"Confirmed; disable option available below"),target:'rfpa-deploy-panel',page:'engines'};
    if (deploy.enabled) return {text:rfUi('Подтверди после внешней проверки',"Confirm after external verification"),detail:rfUi('Watchdog откатит правила без подтверждения',"Watchdog will revert unconfirmed rules"),target:'rfpa-confirm',page:'engines'};
    if (deploy.prepared) return {text:rfUi('Включить защиту',"Enable protection"),detail:'После включения начнётся окно автоматического отката',target:'rfpa-enable',page:'engines'};
    if (latestPreview[method]) return {text:rfUi('Подготовить конфигурацию',"Prepare configuration"),detail:rfUi('Проверенный план не включает firewall',"Verified plan does not modify the firewall"),target:'rfpa-prepare',page:'engines'};
    return {text:rfUi('Проверить план',"Check plan"),detail:'Проверь WAN, порты и сервисы',target:'engine-wizard-preview',page:'engines'};
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

<div class="p54-root rf-pa-workspace" aria-label={rfUi("Контроль доступа","Access control")}>
  <section class="rf-pa-summary p55-panel" aria-label={rfUi("Текущее состояние защиты","Current protection status")}>
    <div class="rf-pa-summary-heading">
      <strong>{rfUi("Состояние защиты","Protection status")}</strong>
      <button type="button" onclick={refreshState}>{rfUi("↻ Обновить состояние","↻ Refresh status")}</button>
    </div>
    <div class="rf-pa-metrics">
      <div class="rf-pa-metric"><span>{rfUi("ДВИЖОК","ENGINE")}</span><b>{engineStatus}</b></div>
      <div class="rf-pa-metric"><span>{rfUi("ЗАЩИТА","PROTECTION")}</span><b>{stateText}</b></div>
      <div class="rf-pa-metric"><span>{rfUi("КОНФИГУРАЦИЯ","CONFIGURATION")}</span><b>{configText}</b></div>
      <div class="rf-pa-metric"><span>WATCHDOG / NDM</span><b>{ndmText}</b></div>
    </div>
    <div class="p55-head-actions" role="status"><span class={'p55-state ' + healthClass}>{health}</span></div>
  </section>
  <nav class="p54-navigation" aria-label={rfUi("Разделы контроля доступа","Access control sections")}>
    {#each pages as page (page.id)}
      <button type="button" class="p54-tab p55-tab" class:active={active === page.id}
        aria-current={active === page.id ? 'page' : undefined} onclick={() => show(page.id)}>{page.title}</button>
    {/each}
  </nav>
  <div class="rf-pa-selector p55-panel" aria-label={rfUi("Выбор способа защиты","Choose protection method")}>
    <h2>{rfUi("Способ защиты","Protection method")}</h2>
    <div class="rf-pa-choices">
      {#each methods as item (item.id)}
        <button type="button" class="rf-pa-choice" class:active={method === item.id} aria-pressed={method === item.id}
          onclick={() => choose(item.id)}><strong>{item.title}</strong><small>{item.hint}</small></button>
      {/each}
    </div>
  </div>
  <div class="rf-pa-next-step p55-panel">
    <strong>{step.text}</strong><p>{step.detail}</p>
    <button type="button" onclick={() => jump(step.target, step.page)}>{rfUi("Перейти к действию","Go to action")}</button>
  </div>
  <section id="p54-protection" class="p54-page" aria-label={rfUi("Обзор","Overview")} hidden={active !== 'protection'}>
    <div class="p54-heading p55-section-heading"><h2>{rfUi("Настроить защиту","Configure protection")}</h2><p>{rfUi("Для odin сохраняй настройки отдельно; включение и подтверждение требуют явного действия.","Save odin settings separately; enabling and confirming protection require explicit actions.")}</p></div>
    <div id="pa-svelte-odin" aria-label={rfUi("Управление защитой odin","Manage odin protection")}><PortAccessOdin onStateChanged={refreshState} /></div>
  </section>
  <section id="p54-access" class="p54-page" aria-label={rfUi("Доступ","Access")} hidden={active !== 'access'}>
    <div class="p54-heading p55-section-heading"><h2>{rfUi("Авторизованные IP · odin","Authorized IPs · odin")}</h2><p>{rfUi("Текущие разрешения KKD. Истечение доступа контролирует ядро; точное время не показано.","Current KKD permissions. Access expiration is handled by the kernel; the exact expiration time is not displayed.")}</p></div>
    <section class="panel rf-pa-odin-access">
      <div class="refresh-bar"><button type="button" disabled={accessBusy} onclick={refreshAccess}>{rfUi("↻ Обновить IP","↻ Refresh IPs")}</button><span role="status">{accessMessage}</span></div>
      <div class="rf-pa-access-list" aria-live="polite">
        {#if method !== 'odin'}<p class="muted">{rfUi("Для текущего движка список доступен в разделе «Диагностика».","For this engine, the list is available under Diagnostics.")}</p>
        {:else if !accessItems.length}<p class="muted">{rfUi("Авторизованных IP пока нет.","No authorized IPs yet.")}</p>
        {:else}{#each accessItems as ip, i (ip + ':' + i)}<div class="rf-pa-access-ip">{ip}</div>{/each}{/if}
      </div>
    </section>
    <button type="button" onclick={() => show('diagnostics')}>{rfUi("Диагностика и черновики","Diagnostics and drafts")}</button>
  </section>
  <section id="p54-engines" class="p54-page" aria-label={rfUi("Движки","Engines")} hidden={active !== 'engines'}>
    <div class="p54-heading p55-section-heading"><h2>{rfUi("Движки Entware","Entware engines")}</h2><p>{rfUi("Установка, настройка, проверка плана и защищённое включение.","Install, configure, validate the plan, and enable protection safely.")}</p></div>
    <div id="pa-svelte-engine-workspace"><PortAccessEngineWorkspace selectedEngine={method === 'odin' ? 'knockd' : method} onEngineChanged={(id) => { method = id; statusKnown = false; void refreshState(); }} onServiceState={updateService} onDeployState={updateDeploy} onPreviewReady={updatePreview} onPlanInvalidated={invalidatePreview} /></div>
  </section>
  <section id="p54-diagnostics" class="p54-page" aria-label={rfUi("Диагностика","Diagnostics")} hidden={active !== 'diagnostics'}>
    <div class="p54-heading p55-section-heading"><h2>{rfUi("Диагностика и инструменты","Diagnostics and tools")}</h2><p>{rfUi("Read-only проверки, предпросмотр, журнал операций и экспериментальные черновики.","Read-only checks, preview, operation log, and experimental drafts.")}</p></div>
    <div id="pa-svelte-engineering" aria-label={rfUi("Инженерная диагностика и черновики","Advanced diagnostics and drafts")}><PortAccessEngineering /></div>
    <div id="pa-svelte-status" aria-label={rfUi("Диагностика доступа","Access diagnostics")}><PortAccessStatus /></div>
    <div id="pa-svelte-log" aria-label={rfUi("Журнал операций","Operation log")}><PortAccessOperationLog /></div>
  </section>
</div>
