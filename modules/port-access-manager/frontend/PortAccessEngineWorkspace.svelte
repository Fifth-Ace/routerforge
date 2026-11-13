<script>
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();
  // Svelte component callbacks replace document-wide CustomEvents and synthetic selection.
  let {
    selectedEngine = 'knockd',
    onEngineChanged = () => {},
    onServiceState = () => {},
    onDeployState = () => {},
    onPreviewReady = () => {},
    onPlanInvalidated = () => {}
  } = $props();

  const engines = ['knockd', 'fwknopd'];
  const actions = [
    ['install', 'Установить из Entware'],
    ['start', 'Запустить'],
    ['stop', 'Остановить'],
    ['restart', 'Перезапустить']
  ];
  let selected = $state('knockd');
  let services = $state({});
  let serviceMessage = $state('Проверяем службы…');
  let serviceBusy = $state(false);
  let configMessage = $state('Проверяем установленную конфигурацию…');
  let configDigest = $state('');
  let configText = $state('');
  let configLoaded = $state(false);
  let configBusy = $state(false);
  let wan = $state('eth3');
  let knock = $state('7777,8888,6666');
  let windowSeconds = $state(15);
  let ttl = $state(120);
  let servicesText = $state('web,192.168.10.103,18080');
  let wizardBusy = $state(false);
  let wizardMessage = $state('');
  let wizardPlan = $state('');
  let validated = $state(null);
  let draftHashes = $state({});
  let replaceExisting = $state(false);
  let deployBusy = $state(false);
  let deployKnown = $state(false);
  let deployData = $state({});
  let deployMessage = $state('Защита не проверена.');
  let credentialOnce = $state('');
  let credentialOpen = $state(false);

  const base = (() => {
    const path = window.location.pathname;
    let m = path.match(/^(.*\/api\/modules\/port-access-manager)\/(?:v1\/)?ui(?:\/.*)?$/);
    if (m) return m[1];
    m = path.match(/^(.*)\/v1\/ui(?:\/.*)?$/);
    if (m) return m[1] + '/v1';
    return '/api/modules/port-access-manager';
  })();
  const url = (suffix) => base + '/' + suffix;
  async function request(endpoint, data) {
    const opts = {credentials: 'same-origin', cache: 'no-store', headers: {Accept: 'application/json'}};
    if (data !== undefined) {
      opts.method = 'POST';
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(data);
    }
    return readSession.consume(endpoint, opts, async response => {
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error || 'HTTP ' + response.status);
      return body;
    });
  }
  function engineURL(id) { return url('engine-service') + '?engine=' + encodeURIComponent(id); }

  async function refreshServices() {
    if (serviceBusy) return;
    try {
      const next = {};
      for (const id of engines) {
        const result = await request(engineURL(id));
        next[id] = result;
        onServiceState({engine:id, installed:!!result.installed, running:!!result.running, installation:result.installation || {state:'idle'}});
      }
      services = next;
      serviceMessage = Object.values(next).some(d => d.installation?.state === 'running')
        ? 'Установка Entware выполняется · статус обновляется автоматически'
        : 'Состояние служб обновлено';
    } catch (e) { serviceMessage = 'Не удалось проверить службы: ' + e.message; }
  }
  function serviceText(d) {
    if (!d) return 'Проверяем…';
    const job = d.installation || {state:'idle'};
    let text = d.installed ? ((d.running ? 'Запущен' : 'Не запущен') + ' · ' + d.script) : 'Init-скрипт не найден';
    if (job.state === 'running') text += ' · Установка Entware выполняется…';
    if (job.state === 'failed') text += ' · Ошибка установки: ' + (job.error || 'Смотри журнал Entware');
    if (job.state === 'completed') text += ' · Установка завершена';
    return text;
  }
  async function serviceAction(id, action) {
    if (serviceBusy || services[id]?.installation?.state === 'running') return;
    const warning = action === 'install'
      ? 'Установить ' + id + ' через opkg? Скрипты стороннего пакета могут менять систему и запускать службу. Установка не гарантирует защиту.'
      : 'Выполнить ' + action + ' для ' + id + '? RouterForge не очищает firewall дополнительно. Init-скрипт Entware может изменить правила.';
    if (!window.confirm(warning)) return;
    serviceBusy = true;
    serviceMessage = 'Выполняется ' + action + ' ' + id + '…';
    try {
      await request(engineURL(id), {action, confirm:'APPLY', firewall_policy:'preserve'});
      serviceBusy = false;
      await refreshServices();
    } catch (e) {
      serviceMessage = 'Операция отклонена: ' + e.message;
    } finally { serviceBusy = false; }
  }

  function configURL(id = selected) { return url('engine-config') + '?engine=' + encodeURIComponent(id); }
  function clearConfig() { configDigest = ''; configText = ''; configLoaded = false; }
  async function refreshConfig() {
    const id = selected;
    clearConfig();
    configBusy = true;
    configMessage = 'Проверяем установленную конфигурацию…';
    try {
      const d = await request(configURL(id));
      if (selected !== id) return;
      configDigest = d.sha256 || '';
      configMessage = 'Конфигурация найдена (' + d.size + ' байт). Резервная копия: ' + (d.backup_available ? 'есть' : 'пока нет') + '. Содержимое не загружено.';
    } catch (e) { if (selected === id) configMessage = e.message; }
    finally { configBusy = false; }
  }
  async function configAction(action) {
    if (configBusy || !configDigest || (action === 'save' && !configLoaded)) {
      configMessage = 'Сначала проверь и загрузи конфигурацию'; return;
    }
    if (action === 'read' && !window.confirm('Показать содержимое ' + selected + '? В fwknopd могут находиться секретные ключи.')) return;
    if (action === 'save' && !window.confirm('Сохранить конфигурацию ' + selected + '? Создаётся резервная копия. Для применения нужен перезапуск службы.')) return;
    if (action === 'restore' && !window.confirm('Восстановить предыдущую конфигурацию ' + selected + '?')) return;
    const id = selected;
    configBusy = true;
    const oldDigest = configDigest;
    try {
      const d = await request(configURL(id), {
        action, base_sha256: oldDigest,
        confirm: action === 'read' ? 'SHOW_CONFIG' : action === 'save' ? 'SAVE_CONFIG' : 'RESTORE_CONFIG',
        content: action === 'save' ? configText : ''
      });
      if (selected !== id) return;
      if (action === 'read') {
        configText = d.content;
        configLoaded = true;
        configMessage = 'Загружено. Изменения не применены к работающей службе.';
      } else if (action === 'save') {
        configDigest = d.sha256;
        configMessage = 'Сохранено. Резервная копия создана. Перезапусти службу через управление движками.';
      } else {
        clearConfig();
        configMessage = 'Предыдущая версия восстановлена. Для применения перезапусти службу.';
        await refreshConfig();
      }
    } catch (e) { if (selected === id) configMessage = e.message; }
    finally { configBusy = false; }
  }

  function invalidatePlan() { validated = null; wizardPlan = ''; onPlanInvalidated(); }
  function collect() {
    const entries = servicesText.split(/\r?\n/).filter(line => line.trim()).map(line => {
      const fields = line.split(',').map(part => part.trim());
      if (fields.length !== 3) throw new Error('Ожидается имя,IPv4,порт в каждой строке');
      return {name: fields[0], address: fields[1], port: Number(fields[2])};
    });
    return {
      engine: selected,
      interface: wan.trim(),
      knock: selected === 'knockd' ? knock.trim().split(',').map(Number) : [],
      sequence_seconds: selected === 'knockd' ? Number(windowSeconds) : 0,
      access_seconds: Number(ttl), services:entries
    };
  }
  async function preview() {
    if (wizardBusy) return;
    let req;
    try { req = collect(); } catch (e) { wizardMessage = 'Ошибка: ' + e.message; return; }
    invalidatePlan(); wizardBusy = true; wizardMessage = 'Проверяем…';
    try {
      const d = await request(url('engine-wizard-preview'), req);
      if (d.applied !== false || d.preview_only !== true || d.engine !== req.engine) throw new Error('Неверный контракт предпросмотра');
      validated = JSON.stringify(req);
      onPreviewReady({engine:req.engine});
      wizardMessage = 'Проверено · НЕ применено'; wizardPlan = d.plan;
    } catch (e) { wizardMessage = 'Ошибка: ' + e.message; }
    finally { wizardBusy = false; }
  }
  async function saveDraft() {
    if (wizardBusy) return;
    let req;
    try { req = collect(); } catch (e) {wizardMessage = 'Ошибка: ' + e.message;return;}
    if (!validated || JSON.stringify(req) !== validated) {wizardMessage = 'Сначала проверь текущий план';return;}
    if (!window.confirm('Сохранить черновик ' + req.engine + '? Рабочие конфигурации и firewall не изменятся.')) return;
    wizardBusy = true;wizardMessage = 'Сохраняем черновик…';
    try {
      const d = await request(url('engine-wizard-draft'), {draft:req, base_sha256:draftHashes[req.engine] || '',confirm:'SAVE_DRAFT'});
      if (d.applied !== false || d.service_started !== false) throw new Error('Неверный ответ API');
      draftHashes = {...draftHashes,[req.engine]:d.sha256};
      wizardMessage = 'Черновик сохранён · защита НЕ применена';
    } catch (e) {wizardMessage = 'Ошибка сохранения: ' + e.message;}
    finally {wizardBusy = false;}
  }
  async function loadDraft() {
    if (wizardBusy) return;
    const id = selected;
    wizardBusy = true; wizardMessage = 'Загружаем черновик…';
    try {
      const d = await request(url('engine-wizard-draft') + '?engine=' + encodeURIComponent(id));
      if (selected !== id) return;
      if (!d.exists) {draftHashes = {...draftHashes,[id]:''};wizardMessage = 'Черновика ещё нет';return;}
      if (d.applied !== false || d.engine !== id || d.draft.engine !== id) throw new Error('Неверный ответ API');
      draftHashes = {...draftHashes,[id]:d.sha256};
      wan = d.draft.interface || '';
      knock = (d.draft.knock || []).join(',');
      windowSeconds = d.draft.sequence_seconds || 15;
      ttl = d.draft.access_seconds;
      servicesText = (d.draft.services || []).map(s => [s.name,s.address,s.port].join(',')).join('\n');
      invalidatePlan();wizardMessage = 'Черновик загружен · проверь план перед сохранением';
    } catch (e) {if (selected === id) wizardMessage = 'Ошибка загрузки: ' + e.message;}
    finally { wizardBusy = false; }
  }

  function deployURL(id) {return url('engine-deploy') + (id ? '?engine=' + encodeURIComponent(id) : '');}
  async function refreshDeploy() {
    if (deployBusy) return;
    const id = selected;
    deployKnown = false;
    try {
      const d = await request(deployURL(id));
      if (id !== selected) return;
      deployData = d;deployKnown = true;
      onDeployState({engine:id,prepared:!!d.prepared,enabled:!!d.enabled,confirmed:!!d.confirmed,running:!!d.running});
      deployMessage = d.enabled ? (!d.running ? 'Защитный шлюз включён, но служба остановлена' : d.confirmed ? 'Защита включена и подтверждена' : 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ · watchdog 90 секунд') : d.prepared ? 'Конфигурация готова · защита выключена' : 'Конфигурация не подготовлена';
    } catch (e) {if (id === selected) deployMessage = 'Ошибка состояния: ' + e.message;}
  }
  async function deploy(action) {
    if (deployBusy || !deployKnown) return;
    let req;
    try {req = collect();} catch(e){deployMessage = 'Ошибка: ' + e.message;return;}
    if (action === 'prepare' && (!validated || JSON.stringify(req) !== validated)) {deployMessage = 'Сначала проверь текущий план';return;}
    const warnings = {
      prepare: 'Создать рабочую конфигурацию ' + selected + '? Замена существующих файлов требует отдельного подтверждения.',
      enable: 'ВКЛЮЧИТЬ защиту ' + selected + '? Проверь доступ с внешнего клиента: без подтверждения watchdog откатит правила через 90 секунд.',
      confirm: 'Подтвердить защиту и отменить автоматический откат? Проверь, что без авторизации доступ закрыт, а после авторизации открыт.',
      disable: 'Удалить только управляемые правила RouterForge? Служба Entware останется в текущем состоянии.'
    };
    if (!window.confirm(warnings[action])) return;
    const id=selected;
    deployBusy=true;deployKnown=false;deployMessage='Выполняется: '+action+'…';
    try {
      const d=await request(deployURL(),{engine:id,action,confirm:'APPLY',wizard:action==='prepare'?req:{},replace_existing:replaceExisting});
      if (d.credential_once) {credentialOnce=d.credential_once; credentialOpen=true;}
      deployMessage='Выполнено: '+action+'. Проверяем состояние…';
    } catch(e) {deployMessage='Операция отклонена: '+e.message;}
    finally {deployBusy=false;await refreshDeploy();}
  }
  function setEngine(id, fromParent = false) {
    if (!engines.includes(id) || selected === id) return;
    selected = id;
    clearConfig();
    invalidatePlan();
    credentialOnce='';credentialOpen=false;replaceExisting=false;
    if (!fromParent) onEngineChanged(id);
    void refreshConfig();void refreshDeploy();
  }
  function changeEngine(event) {
    // bind:value already writes selected before change. Keep the confirmed selection
    // in a separate value so reset/refresh is shared with the parent-driven path.
    setEngine(event.currentTarget.value, false);
  }
  $effect(() => {
    const requested = selectedEngine;
    if (requested !== selected) setEngine(requested, true);
  });
  onMount(() => {
    void refreshServices();void refreshConfig();void refreshDeploy();
    const serviceInterval=window.setInterval(() => {if (!document.hidden) void refreshServices();},3000);
    const deployInterval=window.setInterval(() => {if (!document.hidden && !deployBusy) void refreshDeploy();},10000);
    return () => {readSession.close();window.clearInterval(serviceInterval);window.clearInterval(deployInterval);};
  });
</script>

<section class="panel" id="existing-engine-control" aria-labelledby="existing-engine-title">
  <h2 id="existing-engine-title">Установленные движки · Entware</h2>
  <p class="help">Управление уже установленными службами knockd и fwknopd через их init-скрипты. Настройки и правила firewall здесь не создаются; запуск применит имеющуюся конфигурацию службы.</p>
  <p class="help">Политика остановки: <strong>не выполнять дополнительную очистку firewall (по умолчанию)</strong>. Штатный init-скрипт службы может самостоятельно менять правила.</p>
  <div class="cards">
    {#each engines as id (id)}
      {@const d = services[id]}
      <div class="engine-card">
        <strong>{id === 'knockd' ? 'knockd' : 'fwknopd / SPA'}</strong>
        <p id={id + '-service-state'}>{serviceText(d)}</p>
        {#each actions as [action,title] (action)}
          <button type="button" data-engine-action={id} data-action={action} disabled={serviceBusy || !d || d.installation?.state === 'running' || (action === 'install' ? d.installed : !d.installed)} onclick={() => serviceAction(id,action)}>{title}</button>
        {/each}
      </div>
    {/each}
  </div>
  <div class="refresh-bar"><button type="button" id="existing-engine-refresh" disabled={serviceBusy} onclick={refreshServices}>Обновить службы</button><span id="existing-engine-status" role="status">{serviceMessage}</span></div>
</section>

<section class="panel" id="engine-config-panel">
  <h2>Настройка установленных движков</h2>
  <p class="help">Редактирование существующих конфигураций knockd и fwknopd. Файл сохраняется с резервной копией, служба не перезапускается автоматически. В fwknopd могут храниться секретные ключи.</p>
  <div class="form-grid"><label>Движок<select id="engine-config-engine" value={selected} onchange={changeEngine}><option value="knockd">knockd</option><option value="fwknopd">fwknopd / SPA</option></select></label></div>
  <div class="refresh-bar"><button type="button" id="engine-config-refresh" disabled={configBusy} onclick={refreshConfig}>Обновить сведения</button><button type="button" id="engine-config-load" disabled={configBusy || !configDigest} onclick={() => configAction('read')}>Показать и редактировать файл</button></div>
  <textarea id="engine-config-editor" rows="14" disabled={!configLoaded || configBusy} bind:value={configText} spellcheck="false" autocomplete="off" style="width:100%;box-sizing:border-box;font-family:monospace;resize:vertical"></textarea>
  <div class="refresh-bar"><button type="button" id="engine-config-save" disabled={!configLoaded || configBusy} onclick={() => configAction('save')}>Сохранить конфигурацию</button><button type="button" id="engine-config-restore" disabled={!configDigest || configBusy} onclick={() => configAction('restore')}>Вернуть предыдущую версию</button></div>
  <p id="engine-config-status" role="status" class="muted">{configMessage}</p>
</section>

<section class="panel" id="engine-wizard-panel">
  <h2>Мастер сервисов · knockd / fwknopd</h2>
  <p class="help">Проверка и сохранение черновика без изменения работающих конфигураций и firewall. Сохранённый черновик НЕ применён.</p>
  <div class="form-grid">
    <label>WAN для knockd / fwknopd<input id="engine-wizard-interface" bind:value={wan} oninput={invalidatePlan} autocomplete="off"></label>
    <label>Стуки knockd (через запятую)<input id="engine-wizard-knock" bind:value={knock} oninput={invalidatePlan} autocomplete="off"></label>
    <label>Окно стуков, сек.<input id="engine-wizard-window" type="number" min="5" max="300" bind:value={windowSeconds} oninput={invalidatePlan}></label>
    <label>Время доступа, сек.<input id="engine-wizard-ttl" type="number" min="30" max="86400" bind:value={ttl} oninput={invalidatePlan}></label>
  </div>
  <label>Сервисы (по одному в строке: имя, IPv4, TCP-порт)<textarea id="engine-wizard-services" rows="4" bind:value={servicesText} oninput={invalidatePlan} spellcheck="false"></textarea></label>
  <div class="refresh-bar"><button id="engine-wizard-preview" type="button" disabled={wizardBusy} onclick={preview}>Проверить план</button><button id="engine-wizard-draft-save" type="button" disabled={wizardBusy || !validated} onclick={saveDraft}>Сохранить черновик</button><button id="engine-wizard-draft-load" type="button" disabled={wizardBusy} onclick={loadDraft}>Загрузить черновик</button><span id="engine-wizard-result" role="status">{wizardMessage}</span></div>
  <div class="panel" id="rfpa-deploy-panel">
    <h3>Применение на Entware</h3>
    <p class="help">Сначала установи и настрой выбранный движок. Применение включает управляемый шлюз до разрешения DNAT. Через 90 секунд без подтверждения независимый watchdog откатит шлюз. Остановка службы через обычные кнопки не очищает firewall.</p>
    <label><input type="checkbox" id="rfpa-replace-existing" bind:checked={replaceExisting}> Я подтверждаю замену существующей конфигурации этого движка (оригинал сохранится отдельно)</label>
    <div class="refresh-bar">
      <button type="button" id="rfpa-prepare" disabled={deployBusy || !deployKnown || deployData.prepared || deployData.enabled || !validated} onclick={() => deploy('prepare')}>Создать рабочую конфигурацию</button>
      <button type="button" id="rfpa-enable" disabled={deployBusy || !deployKnown || !deployData.prepared || deployData.enabled} onclick={() => deploy('enable')}>Включить защиту · 90 секунд</button>
      <button type="button" id="rfpa-confirm" disabled={deployBusy || !deployKnown || !deployData.enabled || deployData.confirmed} onclick={() => deploy('confirm')}>Подтвердить защиту</button>
      <button type="button" id="rfpa-disable" disabled={deployBusy || !deployKnown || !deployData.enabled} onclick={() => deploy('disable')}>Убрать управляемую защиту</button>
      <button type="button" id="rfpa-refresh" disabled={deployBusy} onclick={refreshDeploy}>Обновить состояние</button>
    </div>
    <p id="rfpa-deploy-status" role="status" aria-live="polite" class="muted">{deployMessage}</p>
    <details id="rfpa-credentials" bind:open={credentialOpen}><summary>Однократно выданные SPA-ключи</summary><p class="help">Скопируй сразу: после обновления страницы эти ключи не восстанавливаются через API. Не отправляй их в логи.</p><pre id="rfpa-credential-value" style="white-space:pre-wrap;overflow-wrap:anywhere">{credentialOnce}</pre></details>
  </div>
  <pre id="engine-wizard-plan" style="white-space:pre-wrap;overflow-wrap:anywhere" aria-label="План без применения">{wizardPlan}</pre>
</section>
