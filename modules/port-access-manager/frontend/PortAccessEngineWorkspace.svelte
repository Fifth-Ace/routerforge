<script>
  import { rfUi } from './rf-localization.js';
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
    ['install', rfUi('Установить из Entware',"Install from Entware")],
    ['start', rfUi('Запустить',"Start")],
    ['stop', rfUi('Остановить',"Stop")],
    ['restart', rfUi('Перезапустить',"Restart")]
  ];
  let selected = $state('knockd');
  let services = $state({});
  let serviceMessage = $state(rfUi('Проверяем службы…',"Checking services…"));
  let serviceBusy = $state(false);
  let configMessage = $state(rfUi('Проверяем установленную конфигурацию…',"Checking installed configuration…"));
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
  let deployMessage = $state(rfUi('Защита не проверена.',"Protection not verified."));
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
        ? rfUi('Установка Entware выполняется · статус обновляется автоматически',"Entware installation running · status updates automatically")
        : rfUi('Состояние служб обновлено',"Service status refreshed");
    } catch (e) { serviceMessage = rfUi('Не удалось проверить службы: ',"Could not check services:") + e.message; }
  }
  function serviceText(d) {
    if (!d) return rfUi('Проверяем…',"Checking…");
    const job = d.installation || {state:'idle'};
    let text = d.installed ? ((d.running ? rfUi('Запущен',"Running") : rfUi('Не запущен',"Not running")) + ' · ' + d.script) : rfUi('Init-скрипт не найден',"Init script not found");
    if (job.state === 'running') text += rfUi(' · Установка Entware выполняется…',"· Installing Entware package…");
    if (job.state === 'failed') text += rfUi(' · Ошибка установки: ',"· Installation error:") + (job.error || rfUi('Смотри журнал Entware',"Check the Entware log"));
    if (job.state === 'completed') text += rfUi(' · Установка завершена',"· Installation completed");
    return text;
  }
  async function serviceAction(id, action) {
    if (serviceBusy || services[id]?.installation?.state === 'running') return;
    const warning = action === 'install'
      ? rfUi('Установить ',"Install") + id + rfUi(' через opkg? Скрипты стороннего пакета могут менять систему и запускать службу. Установка не гарантирует защиту.',"through opkg? Third-party package scripts may change the system and start services. Installation does not guarantee protection.")
      : rfUi('Выполнить ',"Run") + action + ' для ' + id + '? RouterForge не очищает firewall дополнительно. Init-скрипт Entware может изменить правила.';
    if (!window.confirm(warning)) return;
    serviceBusy = true;
    serviceMessage = rfUi('Выполняется ',"Running") + action + ' ' + id + '…';
    try {
      await request(engineURL(id), {action, confirm:'APPLY', firewall_policy:'preserve'});
      serviceBusy = false;
      await refreshServices();
    } catch (e) {
      serviceMessage = rfUi('Операция отклонена: ',"Operation rejected:") + e.message;
    } finally { serviceBusy = false; }
  }

  function configURL(id = selected) { return url('engine-config') + '?engine=' + encodeURIComponent(id); }
  function clearConfig() { configDigest = ''; configText = ''; configLoaded = false; }
  async function refreshConfig() {
    const id = selected;
    clearConfig();
    configBusy = true;
    configMessage = rfUi('Проверяем установленную конфигурацию…',"Checking installed configuration…");
    try {
      const d = await request(configURL(id));
      if (selected !== id) return;
      configDigest = d.sha256 || '';
      configMessage = rfUi('Конфигурация найдена (',"Configuration found (") + d.size + rfUi(' байт). Резервная копия: ',"bytes). Backup:") + (d.backup_available ? rfUi('есть',"present") : rfUi('пока нет',"not yet")) + rfUi('. Содержимое не загружено.',". Content has not been loaded.");
    } catch (e) { if (selected === id) configMessage = e.message; }
    finally { configBusy = false; }
  }
  async function configAction(action) {
    if (configBusy || !configDigest || (action === 'save' && !configLoaded)) {
      configMessage = rfUi('Сначала проверь и загрузи конфигурацию',"Check and load the configuration first"); return;
    }
    if (action === 'read' && !window.confirm(rfUi('Показать содержимое ',"Show content") + selected + rfUi('? В fwknopd могут находиться секретные ключи.',"? fwknopd configuration may contain secret keys."))) return;
    if (action === 'save' && !window.confirm(rfUi('Сохранить конфигурацию ',"Save configuration") + selected + '? Создаётся резервная копия. Для применения нужен перезапуск службы.')) return;
    if (action === 'restore' && !window.confirm(rfUi('Восстановить предыдущую конфигурацию ',"Restore previous configuration") + selected + '?')) return;
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
        configMessage = rfUi('Загружено. Изменения не применены к работающей службе.',"Loaded. Changes have not been applied to the running service.");
      } else if (action === 'save') {
        configDigest = d.sha256;
        configMessage = rfUi('Сохранено. Резервная копия создана. Перезапусти службу через управление движками.',"Saved. A backup was created. Restart the service from Engine management.");
      } else {
        clearConfig();
        configMessage = rfUi('Предыдущая версия восстановлена. Для применения перезапусти службу.',"Previous version restored. Restart the service to apply it.");
        await refreshConfig();
      }
    } catch (e) { if (selected === id) configMessage = e.message; }
    finally { configBusy = false; }
  }

  function invalidatePlan() { validated = null; wizardPlan = ''; onPlanInvalidated(); }
  function collect() {
    const entries = servicesText.split(/\r?\n/).filter(line => line.trim()).map(line => {
      const fields = line.split(',').map(part => part.trim());
      if (fields.length !== 3) throw new Error(rfUi('Ожидается имя,IPv4,порт в каждой строке',"Expected name,IPv4,port on each line"));
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
    invalidatePlan(); wizardBusy = true; wizardMessage = rfUi('Проверяем…',"Checking…");
    try {
      const d = await request(url('engine-wizard-preview'), req);
      if (d.applied !== false || d.preview_only !== true || d.engine !== req.engine) throw new Error(rfUi('Неверный контракт предпросмотра',"Invalid preview response"));
      validated = JSON.stringify(req);
      onPreviewReady({engine:req.engine});
      wizardMessage = rfUi('Проверено · НЕ применено',"Verified · NOT applied"); wizardPlan = d.plan;
    } catch (e) { wizardMessage = rfUi('Ошибка: ',"Error:") + e.message; }
    finally { wizardBusy = false; }
  }
  async function saveDraft() {
    if (wizardBusy) return;
    let req;
    try { req = collect(); } catch (e) {wizardMessage = 'Ошибка: ' + e.message;return;}
    if (!validated || JSON.stringify(req) !== validated) {wizardMessage = rfUi('Сначала проверь текущий план',"Check the current plan first");return;}
    if (!window.confirm(rfUi('Сохранить черновик ',"Save draft") + req.engine + '? Рабочие конфигурации и firewall не изменятся.')) return;
    wizardBusy = true;wizardMessage = rfUi('Сохраняем черновик…',"Saving draft…");
    try {
      const d = await request(url('engine-wizard-draft'), {draft:req, base_sha256:draftHashes[req.engine] || '',confirm:'SAVE_DRAFT'});
      if (d.applied !== false || d.service_started !== false) throw new Error(rfUi('Неверный ответ API',"Invalid API response"));
      draftHashes = {...draftHashes,[req.engine]:d.sha256};
      wizardMessage = rfUi('Черновик сохранён · защита НЕ применена',"Draft saved · protection NOT applied");
    } catch (e) {wizardMessage = rfUi('Ошибка сохранения: ',"Save error:") + e.message;}
    finally {wizardBusy = false;}
  }
  async function loadDraft() {
    if (wizardBusy) return;
    const id = selected;
    wizardBusy = true; wizardMessage = rfUi('Загружаем черновик…',"Loading draft…");
    try {
      const d = await request(url('engine-wizard-draft') + '?engine=' + encodeURIComponent(id));
      if (selected !== id) return;
      if (!d.exists) {draftHashes = {...draftHashes,[id]:''};wizardMessage = rfUi('Черновика ещё нет',"No draft yet");return;}
      if (d.applied !== false || d.engine !== id || d.draft.engine !== id) throw new Error(rfUi('Неверный ответ API',"Invalid API response"));
      draftHashes = {...draftHashes,[id]:d.sha256};
      wan = d.draft.interface || '';
      knock = (d.draft.knock || []).join(',');
      windowSeconds = d.draft.sequence_seconds || 15;
      ttl = d.draft.access_seconds;
      servicesText = (d.draft.services || []).map(s => [s.name,s.address,s.port].join(',')).join('\n');
      invalidatePlan();wizardMessage = rfUi('Черновик загружен · проверь план перед сохранением',"Draft loaded · review the plan before saving");
    } catch (e) {if (selected === id) wizardMessage = rfUi('Ошибка загрузки: ',"Load error:") + e.message;}
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
      deployMessage = d.enabled ? (!d.running ? rfUi('Защитный шлюз включён, но служба остановлена',"Protection gate is enabled, but the service is stopped") : d.confirmed ? rfUi('Защита включена и подтверждена',"Protection enabled and confirmed") : rfUi('ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ · watchdog 90 секунд',"AWAITING CONFIRMATION · 90-second watchdog")) : d.prepared ? rfUi('Конфигурация готова · защита выключена',"Configuration ready · protection disabled") : rfUi('Конфигурация не подготовлена',"Configuration not prepared");
    } catch (e) {if (id === selected) deployMessage = rfUi('Ошибка состояния: ',"Status error:") + e.message;}
  }
  async function deploy(action) {
    if (deployBusy || !deployKnown) return;
    let req;
    try {req = collect();} catch(e){deployMessage = 'Ошибка: ' + e.message;return;}
    if (action === 'prepare' && (!validated || JSON.stringify(req) !== validated)) {deployMessage = rfUi('Сначала проверь текущий план',"Check the current plan first");return;}
    const warnings = {
      prepare: rfUi('Создать рабочую конфигурацию ',"Create active configuration") + selected + '? Замена существующих файлов требует отдельного подтверждения.',
      enable: rfUi('ВКЛЮЧИТЬ защиту ',"ENABLE protection") + selected + rfUi('? Проверь доступ с внешнего клиента: без подтверждения watchdog откатит правила через 90 секунд.',"? Verify access from an external client. Without confirmation, the watchdog will revert rules after 90 seconds."),
      confirm: rfUi('Подтвердить защиту и отменить автоматический откат? Проверь, что без авторизации доступ закрыт, а после авторизации открыт.',"Confirm protection and disable automatic rollback? Verify that access is blocked without authorization and available after authorization."),
      disable: rfUi('Удалить только управляемые правила RouterForge? Служба Entware останется в текущем состоянии.',"Remove only RouterForge-managed rules? The Entware service will remain in its current state.")
    };
    if (!window.confirm(warnings[action])) return;
    const id=selected;
    deployBusy=true;deployKnown=false;deployMessage=rfUi('Выполняется: ',"Running:")+action+'…';
    try {
      const d=await request(deployURL(),{engine:id,action,confirm:'APPLY',wizard:action==='prepare'?req:{},replace_existing:replaceExisting});
      if (d.credential_once) {credentialOnce=d.credential_once; credentialOpen=true;}
      deployMessage=rfUi('Выполнено: ',"Completed:")+action+rfUi('. Проверяем состояние…',". Checking status…");
    } catch(e) {deployMessage=rfUi('Операция отклонена: ',"Operation rejected:")+e.message;}
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
  <h2 id="existing-engine-title">{rfUi("Установленные движки · Entware","Installed engines · Entware")}</h2>
  <p class="help">{rfUi("Управление уже установленными службами knockd и fwknopd через их init-скрипты. Настройки и правила firewall здесь не создаются; запуск применит имеющуюся конфигурацию службы.","Manage installed knockd and fwknopd services using their init scripts. This screen does not create firewall rules or configurations; starting a service uses its existing configuration.")}</p>
  <p class="help">{rfUi("Политика остановки:","Stop policy:")} <strong>{rfUi("не выполнять дополнительную очистку firewall (по умолчанию)","do not perform additional firewall cleanup (default)")}</strong>{rfUi(". Штатный init-скрипт службы может самостоятельно менять правила.",". The service's own init script may change rules independently.")}</p>
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
  <div class="refresh-bar"><button type="button" id="existing-engine-refresh" disabled={serviceBusy} onclick={refreshServices}>{rfUi("Обновить службы","Refresh services")}</button><span id="existing-engine-status" role="status">{serviceMessage}</span></div>
</section>

<section class="panel" id="engine-config-panel">
  <h2>{rfUi("Настройка установленных движков","Configure installed engines")}</h2>
  <p class="help">{rfUi("Редактирование существующих конфигураций knockd и fwknopd. Файл сохраняется с резервной копией, служба не перезапускается автоматически. В fwknopd могут храниться секретные ключи.","Edit existing knockd and fwknopd configurations. Files are backed up before saving; services do not restart automatically. fwknopd configuration may contain secret keys.")}</p>
  <div class="form-grid"><label>{rfUi("Движок","Engine")}<select id="engine-config-engine" value={selected} onchange={changeEngine}><option value="knockd">knockd</option><option value="fwknopd">fwknopd / SPA</option></select></label></div>
  <div class="refresh-bar"><button type="button" id="engine-config-refresh" disabled={configBusy} onclick={refreshConfig}>{rfUi("Обновить сведения","Refresh details")}</button><button type="button" id="engine-config-load" disabled={configBusy || !configDigest} onclick={() => configAction('read')}>{rfUi("Показать и редактировать файл","View and edit file")}</button></div>
  <textarea id="engine-config-editor" rows="14" disabled={!configLoaded || configBusy} bind:value={configText} spellcheck="false" autocomplete="off" style="width:100%;box-sizing:border-box;font-family:monospace;resize:vertical"></textarea>
  <div class="refresh-bar"><button type="button" id="engine-config-save" disabled={!configLoaded || configBusy} onclick={() => configAction('save')}>{rfUi("Сохранить конфигурацию","Save configuration")}</button><button type="button" id="engine-config-restore" disabled={!configDigest || configBusy} onclick={() => configAction('restore')}>{rfUi("Вернуть предыдущую версию","Restore previous version")}</button></div>
  <p id="engine-config-status" role="status" class="muted">{configMessage}</p>
</section>

<section class="panel" id="engine-wizard-panel">
  <h2>{rfUi("Мастер сервисов · knockd / fwknopd","Service wizard · knockd / fwknopd")}</h2>
  <p class="help">{rfUi("Проверка и сохранение черновика без изменения работающих конфигураций и firewall. Сохранённый черновик НЕ применён.","Check and save a draft without modifying active configurations or firewall. A saved draft is NOT applied.")}</p>
  <div class="form-grid">
    <label>{rfUi("WAN для knockd / fwknopd","WAN interface for knockd / fwknopd")}<input id="engine-wizard-interface" bind:value={wan} oninput={invalidatePlan} autocomplete="off"></label>
    <label>{rfUi("Стуки knockd (через запятую)","knockd knock sequence (comma-separated)")}<input id="engine-wizard-knock" bind:value={knock} oninput={invalidatePlan} autocomplete="off"></label>
    <label>{rfUi("Окно стуков, сек.","Knock window, sec")}<input id="engine-wizard-window" type="number" min="5" max="300" bind:value={windowSeconds} oninput={invalidatePlan}></label>
    <label>{rfUi("Время доступа, сек.","Access duration, sec")}<input id="engine-wizard-ttl" type="number" min="30" max="86400" bind:value={ttl} oninput={invalidatePlan}></label>
  </div>
  <label>{rfUi("Сервисы (по одному в строке: имя, IPv4, TCP-порт)","Services (one per line: name, IPv4, TCP port)")}<textarea id="engine-wizard-services" rows="4" bind:value={servicesText} oninput={invalidatePlan} spellcheck="false"></textarea></label>
  <div class="refresh-bar"><button id="engine-wizard-preview" type="button" disabled={wizardBusy} onclick={preview}>{rfUi("Проверить план","Check plan")}</button><button id="engine-wizard-draft-save" type="button" disabled={wizardBusy || !validated} onclick={saveDraft}>{rfUi("Сохранить черновик","Save draft")}</button><button id="engine-wizard-draft-load" type="button" disabled={wizardBusy} onclick={loadDraft}>{rfUi("Загрузить черновик","Load draft")}</button><span id="engine-wizard-result" role="status">{wizardMessage}</span></div>
  <div class="panel" id="rfpa-deploy-panel">
    <h3>{rfUi("Применение на Entware","Apply to Entware")}</h3>
    <p class="help">{rfUi("Сначала установи и настрой выбранный движок. Применение включает управляемый шлюз до разрешения DNAT. Через 90 секунд без подтверждения независимый watchdog откатит шлюз. Остановка службы через обычные кнопки не очищает firewall.","Install and configure the selected engine first. Applying enables a managed gate ahead of the DNAT rule. Without confirmation, an independent watchdog rolls the gate back after 90 seconds. Stopping the service with normal controls does not clear firewall rules.")}</p>
    <label><input type="checkbox" id="rfpa-replace-existing" bind:checked={replaceExisting}> {rfUi("Я подтверждаю замену существующей конфигурации этого движка (оригинал сохранится отдельно)","I confirm replacing this engine's existing configuration (the original will be preserved separately)")}</label>
    <div class="refresh-bar">
      <button type="button" id="rfpa-prepare" disabled={deployBusy || !deployKnown || deployData.prepared || deployData.enabled || !validated} onclick={() => deploy('prepare')}>{rfUi("Создать рабочую конфигурацию","Create active configuration")}</button>
      <button type="button" id="rfpa-enable" disabled={deployBusy || !deployKnown || !deployData.prepared || deployData.enabled} onclick={() => deploy('enable')}>{rfUi("Включить защиту · 90 секунд","Enable protection · 90 seconds")}</button>
      <button type="button" id="rfpa-confirm" disabled={deployBusy || !deployKnown || !deployData.enabled || deployData.confirmed} onclick={() => deploy('confirm')}>{rfUi("Подтвердить защиту","Confirm protection")}</button>
      <button type="button" id="rfpa-disable" disabled={deployBusy || !deployKnown || !deployData.enabled} onclick={() => deploy('disable')}>{rfUi("Убрать управляемую защиту","Remove managed protection")}</button>
      <button type="button" id="rfpa-refresh" disabled={deployBusy} onclick={refreshDeploy}>{rfUi("Обновить состояние","Refresh status")}</button>
    </div>
    <p id="rfpa-deploy-status" role="status" aria-live="polite" class="muted">{deployMessage}</p>
    <details id="rfpa-credentials" bind:open={credentialOpen}><summary>{rfUi("Однократно выданные SPA-ключи","One-time SPA keys")}</summary><p class="help">{rfUi("Скопируй сразу: после обновления страницы эти ключи не восстанавливаются через API. Не отправляй их в логи.","Copy the keys now: after reloading this page, they cannot be retrieved through the API. Never include them in logs.")}</p><pre id="rfpa-credential-value" style="white-space:pre-wrap;overflow-wrap:anywhere">{credentialOnce}</pre></details>
  </div>
  <pre id="engine-wizard-plan" style="white-space:pre-wrap;overflow-wrap:anywhere" aria-label={rfUi("План без применения","Preview without applying")}>{wizardPlan}</pre>
</section>
