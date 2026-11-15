<script>
  import { onMount } from 'svelte';
  import {
    getRouterforgeSyslogStatus, getRouterforgeSyslogConfig, getRouterforgeSyslogFiles, getRouterforgeSyslogTail,
    planRouterforgeSyslog, getRouterforgeSyslogSummary,
    applyRouterforgeSyslog, routerforgeSyslogExportURL, entwarePackageAction,
    preflightEntwarePackage, adminServiceAction
  } from '$lib/api.js';
  export let locale = 'ru';
  const tr = (ru,en) => locale === 'en' ? en : ru;
  let status = null;
  let draft = null;
  let files = [];
  let chosen = null;
  let lines = [];
  let filter = '';
  let busy = '';
  let error = '';
  let notice = '';
  let acceptAdoption = false;
  let plan = null;
  let planBusy = false;
  let summary = null;
  let dirty = false;
  let periodic = null;
  let loadEpoch = 0;
  let tailEpoch = 0;
  let configPreview = null;
  let configBusy = false;
  let configVisible = false;
  let simpleMode = true;
  let storageKind = 'usb';
  let newDeviceIP = '';
  let newDeviceName = '';
  let firstLoadDone = false;
  let smb = null;
  let smbBusy = false;
  let smbError = '';
  let smbHost = '';
  let smbShare = '';
  let smbUser = '';
  let smbPassword = '';
  let smbDomain = '';
  let smbRemember = true;
  const syslogSMBPath='/api/modules/admin/syslog/smb/';
  async function smbRequest(path, body = null) {
    const response=await fetch(syslogSMBPath+path,{credentials:'same-origin',cache:'no-store',...(body ? {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)} : {})});
    const data=await response.json();
    if(!response.ok) throw new Error(data?.error||`HTTP ${response.status}`);
    return data;
  }
  async function loadSMB() {
    try {smb=await smbRequest('status');smbError='';}catch(e){smbError=e.message;}
  }
  async function smbConnect() {
    if(smbBusy) return;
    if(!confirm(tr('Подключить указанную SMB-папку к RouterForge? Службы syslog-ng и DNS не перезапускаются.','Connect this SMB share? syslog-ng and DNS are not restarted.')))return;
    smbBusy=true;smbError='';
    try {
      const connected=await smbRequest('connect',{host:smbHost,share:smbShare,username:smbUser,password:smbPassword,domain:smbDomain,remember:smbRemember,confirm:'CONNECT SYSLOG SMB'});
      smbPassword='';
      await refresh();
      await loadSMB();
      storageKind='smb';draft.root=connected.root;mark();
      notice=tr('NAS подключён. Проверь настройки ниже и сохрани план отдельно.','NAS connected. Review and save the Syslog plan separately.');
    }catch(e){smbError=e.message;}
    finally{smbBusy=false;}
  }
  async function installSMBClient() {
    if (smbBusy || busy || smb?.client_ready) return;
    smbBusy=true; smbError=''; notice='';
    try {
      const p=await preflightEntwarePackage('cifsmount','install');
      if(!p.allowed) throw new Error(p.reason || tr('Установка CIFS-клиента недоступна через App Center.','The CIFS client cannot be installed through App Center.'));
      if(!confirm(tr('Установить CIFS-клиент cifsmount из Entware? Это изменит установленные пакеты, но не подключит NAS и не перезапустит Syslog.','Install the Entware cifsmount package? This changes installed packages but will not mount NAS or restart Syslog.')))return;
      const result=await entwarePackageAction('cifsmount','install');
      if(!result.installed) throw new Error(tr('Установка не подтверждена App Center.','App Center did not confirm installation.'));
      await loadSMB();
      notice=smb?.client_ready
        ? tr('CIFS-клиент установлен. Теперь можно настроить NAS.','CIFS client installed. You can now configure NAS.')
        : tr('Пакет установлен, но mount.cifs пока не найден. Проверь поддержку на странице App Center.','Package installed, but mount.cifs is still missing. Check App Center.');
    }catch(e){smbError=getError(e);}finally{smbBusy=false;}
  }
  async function smbDisconnect() {
    if(smbBusy || !confirm(tr('Отключить SMB-папку? Сначала останови Syslog, если он записывает сюда.','Disconnect SMB share? Stop Syslog first if it writes here.')))return;
    smbBusy=true;smbError='';
    try {
      await smbRequest('disconnect',{confirm:'DISCONNECT SYSLOG SMB'});
      await loadSMB();await refresh();
      if(storageKind==='smb')changeStorage('smb');
      notice=tr('SMB-папка отключена.','SMB share disconnected.');
    }catch(e){smbError=e.message;}
    finally{smbBusy=false;}
  }
  $: availableStorage = (status?.storage_options || []).filter((item) => item.kind === storageKind);
  $: hasSeparateEntware = (status?.storage_options || []).some((item) => item.kind === 'local');
  $: entwareOnUSB = (status?.storage_options || []).some((item) => item.kind === 'usb' && item.root === '/opt/var/log/routerforge-syslog');
  $: selectedStorage = (status?.storage_options || []).find((item) => item.root === draft?.root);
  $: namedDevices = Object.entries(draft?.device_names || {}).sort((a,b) => a[0].localeCompare(b[0]));
  $: networkSources = (status?.source_details || []).filter((entry) => entry.kind === 'network');
  $: localSources = (status?.source_details || []).filter((entry) => entry.kind === 'local');
  function changeStorage(kind) {
    storageKind = kind;
    const options = (status?.storage_options || []).filter((v) => v.kind === kind);
    draft.root = options[0]?.root || '';
    mark();
    if (kind==='smb') void loadSMB();
  }
  function nameDevice(ip, label) {
    draft.device_names = { ...(draft.device_names || {}), [ip]: label };
    mark();
  }
  function addDevice() {
    const ip = newDeviceIP.trim(), name = newDeviceName.trim().replace(/\s+/g,'_');
    if (!/^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/.test(ip)) {
      error=tr('Укажи корректный IPv4-адрес устройства.','Enter a valid IPv4 address.');return;
    }
    if(!/^[\p{L}\p{N}_-]{1,48}$/u.test(name)) {
      error=tr('Название: буквы, цифры, дефис или подчёркивание (до 48 символов).','Name: letters, digits, hyphen or underscore (max 48).');return;
    }
    if(namedDevices.length>=8 && !draft.device_names?.[ip]) {error=tr('Не больше 8 именных устройств.','At most 8 named devices.');return;}
    if (Object.entries(draft.device_names || {}).some(([existing, v]) => existing !== ip && v.toLowerCase()===name.toLowerCase())) {
      error=tr('Названия устройств не должны повторяться.','Device names must be unique.');return;
    }
    nameDevice(ip,name);newDeviceIP='';newDeviceName='';error='';
  }
  function removeDevice(ip) {
    const entries={...(draft.device_names||{})};delete entries[ip];draft.device_names=entries;mark();
  }
  function sourceChoice() {
    if(!draft || !status || status.sidecar_included) return;
    // Never select kernel or ambiguous source merely because it exists.
    if (!networkSources.some((v)=>v.name===draft.remote_source)) draft.remote_source=networkSources[0]?.name || '';
    if (!localSources.some((v)=>v.name===draft.local_source)) draft.local_source=localSources[0]?.name || '';
    if (draft.local && !draft.local_source) draft.local=false;
  }
  function selectHeadIP(ip){
    const old=draft.head_ip;
    const previous=draft.device_names?.[old];
    draft.head_ip=ip;
    if(previous && !draft.device_names?.[ip]) {const names={...draft.device_names};delete names[old];names[ip]=previous;draft.device_names=names;}
    mark();
  }


  $: visibleLines = lines.filter((line) => !filter || line.toLowerCase().includes(filter.toLowerCase()));

  async function refresh({ initial = false } = {}) {
    const epoch = ++loadEpoch;
    if (!status) error = '';
    try {
      const current = await getRouterforgeSyslogStatus();
      if (epoch !== loadEpoch) return;
      status = current;
      if (configPreview && configPreview.sha256 !== current.config_sha256) {
        configPreview = null;
        configVisible = false;
      }
      if (initial || !dirty) {
        draft = structuredClone(current.settings);
        draft.rotate_hours = draft.rotate_hours || 24;
        draft.device_names = draft.device_names || {};
        if(!firstLoadDone) {
          const currentKind=(current.storage_options||[]).find(v=>v.root===draft.root)?.kind;
          storageKind=currentKind || (draft.root.startsWith('/opt/mnt/') ? 'usb' : 'local');
          // First-run only: prefer the detected Entware USB if the previous
          // default points nowhere. Never silently migrate saved sidecars.
          if(!currentKind && !current.sidecar_included) {
            const firstUSB=(current.storage_options||[]).find(v=>v.kind==='usb');
            if(firstUSB){storageKind='usb';draft.root=firstUSB.root;}
          }
          firstLoadDone=true;
        }
        sourceChoice();
      }
      if(plan && (plan.parent_sha256!==current.config_sha256 || plan.sidecar_sha256!==current.sidecar_sha256)){plan=null;acceptAdoption=false;}
      if (current.installed && current.managed) {
        const list = await getRouterforgeSyslogFiles();
        if (epoch !== loadEpoch) return;
        files = list.files || [];
        try {summary=await getRouterforgeSyslogSummary();}catch(_){summary=null;}
        if (!chosen || !files.some((f) => f.date === chosen.date && f.source === chosen.source)) {
          chosen = files[0] || null;
          await reloadTail();
        }
      } else { files=[];chosen=null;lines=[];summary=null; }
      error='';
    } catch (e) { if (epoch === loadEpoch) error = getError(e); }
  }
  function getError(e) { return e?.payload?.detail || e?.payload?.error || e?.message || 'Request failed'; }
  async function reloadTail() {
    const epoch=++tailEpoch;
    if (!chosen) {lines=[];return;}
    try {const result=await getRouterforgeSyslogTail(chosen.date,chosen.source,300);if(epoch===tailEpoch) lines=result.lines||[];}
    catch(e){if(epoch===tailEpoch) error=getError(e);}
  }
  function selectFile(file) { chosen=file; void reloadTail(); }
  function mark() {dirty=true;plan=null;acceptAdoption=false;}
  async function toggleConfigPreview() {
    if (configVisible) { configVisible=false; return; }
    if (!status?.config_exists || configBusy) return;
    if (configPreview?.sha256 === status.config_sha256) { configVisible=true; return; }
    configBusy=true;
    error='';
    try {
      const existing = await getRouterforgeSyslogConfig();
      if (existing.sha256 !== status.config_sha256) {
        error=tr('Конфигурация изменилась с момента проверки. Обнови состояние.','Configuration changed since status check. Refresh first.');
        return;
      }
      configPreview=existing;
      configVisible=true;
    } catch(e) { error=getError(e); }
    finally { configBusy=false; }
  }
  async function install() {
    if (busy) return;
    busy='install'; error='';notice='';
    try {
      const p=await preflightEntwarePackage('syslog-ng','install');
      if (!p.allowed) throw new Error(p.reason || tr('App Center отклонил установку','Installation not permitted by App Center'));
      if (!confirm(tr('Установить пакет syslog-ng из Entware?','Install syslog-ng from Entware?'))) return;
      await entwarePackageAction('syslog-ng','install');
      notice=tr('Пакет установлен. Конфигурация не изменялась.','Package installed. Configuration unchanged.');
      dirty=false;
      await refresh({initial:true});
    } catch(e){error=getError(e);} finally{busy='';}
  }
  async function prepare() {
    if(!draft || busy || !status) return;
    planBusy=true;error='';notice='';plan=null;acceptAdoption=false;
    try {
      const p=await planRouterforgeSyslog({settings:draft});
      if(p.parent_sha256!==status.config_sha256){throw new Error(tr('Основной конфиг изменился: обнови состояние.','Main config changed: refresh status.'));}
      plan=p;
      notice=tr('План готов. Подтверди подключение дополнения: службы не запускаются.','Plan ready. Confirm the add-on; no service is started.');
    }catch(e){error=getError(e);}finally{planBusy=false;}
  }
  async function apply() {
    if (!draft || busy || !status || !plan || !acceptAdoption) return;
    if(!confirm(tr('Подключить отдельный файл RouterForge к существующему syslog-ng.conf? Резервные копии создаются автоматически. НИ ОДНА служба не запускается.','Attach a separate RouterForge config to the existing syslog-ng.conf? Backups are automatic. No services will be started.'))) return;
    busy='apply';error='';notice='';
    try {
      const result=await applyRouterforgeSyslog({settings:draft,expected_sha256:plan.parent_sha256,expected_sidecar_sha256:plan.sidecar_sha256,plan_sha256:plan.plan_sha256,confirm:'APPLY SYSLOG SIDECAR'});
      notice=tr('Дополнение сохранено без замены основного конфига. Для активации вручную запусти/перезапусти syslog-ng.','Add-on saved without replacing the main config. Manually start/restart syslog-ng to activate.');
      dirty=false;acceptAdoption=false;plan=null;
      await refresh({initial:true});
    }catch(e){error=getError(e);}finally{busy='';}
  }
  async function serviceAction(action) {
    if (busy || !status?.service_id) return;
    if (!confirm(tr('Выполнить','Execute')+' '+action+' '+status.service_id+'?')) return;
    busy='service';error='';notice='';
    try {await adminServiceAction(status.service_id,action);notice=tr('Действие выполнено','Action complete')+': '+action;await refresh();}
    catch(e){error=getError(e);}finally{busy='';}
  }
  onMount(() => {
    void loadSMB();
    refresh({initial:true});
    periodic=window.setInterval(() => {if (!document.hidden && !busy) {void refresh();if(chosen)void reloadTail();}},10000);
    return () => {loadEpoch++;tailEpoch++;if(periodic)clearInterval(periodic);};
  });
</script>

<div class="rf-syslog">
  <section class="panel">
    <div class="panel-head">
      <div><strong>{tr('Системные журналы','System logs')}</strong><span>{tr('Управление syslog-ng из Entware · без собственной службы RouterForge','Entware syslog-ng integration · no additional RouterForge service')}</span></div>
      <button class="button" onclick={() => refresh()} disabled={!!busy}>↻ {tr('Обновить','Refresh')}</button>
    </div>
    {#if error}<div class="rf-syslog-msg rf-error" role="alert">{error}</div>{/if}
    {#if notice}<div class="rf-syslog-msg rf-ok" role="status">{notice}</div>{/if}
    {#if !status}
      <div class="rf-syslog-empty">{error ? tr('Не удалось получить состояние. Проверь версию RouterForge Core и права доступа, затем нажми «Обновить».','Status unavailable. Check RouterForge Core version and permissions, then refresh.') : tr('Получаем состояние...','Loading status...')}</div>
    {:else}
      <div class="rf-syslog-status">
        <div><small>{tr('Пакет','Package')}</small><strong>{status.installed ? 'syslog-ng '+(status.version||'') : tr('Не установлен','Not installed')}</strong></div>
        <div><small>{tr('Служба','Service')}</small><strong>{status.running ? tr('Работает','Running') : tr('Остановлена','Stopped')}</strong></div>
        <div><small>{tr('Конфигурация','Config')}</small><strong>{status.sidecar_included ? tr('Дополнение RouterForge','RouterForge add-on') : status.managed ? 'RouterForge' : status.config_exists ? tr('Сторонняя','Existing / unmanaged') : tr('Нет','Missing')}</strong></div>
        <div><small>{tr('Хранилище','Storage')}</small><strong>{status.storage_ready ? 'READY' : 'UNAVAILABLE'}</strong></div>
      </div>
    {/if}
  </section>

  {#if status && !status.installed}
    <section class="panel rf-syslog-space">
      <strong>{tr('Требуется пакет Entware syslog-ng','Entware syslog-ng required')}</strong>
      <p>{tr('RouterForge не устанавливает пакет автоматически. Для работы нужен системный сборщик syslog-ng.','RouterForge does not automatically install packages. syslog-ng is required.')}</p>
      <button class="button primary" onclick={install} disabled={!!busy}>{busy==='install'?'…':tr('Установить syslog-ng','Install syslog-ng')}</button>
      <span class="rf-hint">{tr('Операция выполняется через App Center и его действующую политику безопасности.','Uses App Center with its existing safety policy.')}</span>
      <a href="/apps">{tr('Открыть App Center','Open App Center')}</a>
    </section>
  {:else if status && draft}
    <section class="panel rf-service-controls">
      <div class="rf-service-row">
        <div class="rf-service-title"><strong>{tr('Управление службой','Service controls')}</strong><span>{status.service_id||tr('Скрипт службы не обнаружен','Service script not found')}</span></div>
        <div class="rf-service-actions">
          <button class="button" disabled={!!busy||!status.service_id||status.running||!status.sidecar_included||!status.storage_ready} onclick={() => serviceAction('start')}>{tr('Начать сбор','Start collecting')}</button>
          <button class="button" disabled={!!busy||!status.service_id||!status.running||!status.sidecar_included||!status.storage_ready} onclick={() => serviceAction('restart')}>{tr('Перезапустить','Restart')}</button>
          <button class="button" disabled={!!busy||!status.service_id||!status.running} onclick={() => serviceAction('stop')}>{tr('Остановить','Stop')}</button>
        </div>
      </div>
      <p class="rf-hint">{!status.sidecar_included ? tr('Для начала сбора сначала настройте и сохраните правила ниже. Существующий Syslog не запускается автоматически.','Configure and save the settings below before starting collection. The existing Syslog service is not started automatically.') : !status.storage_ready ? tr('Хранилище недоступно. Сбор нельзя начать до восстановления накопителя.','Storage unavailable. Collection cannot start until the disk is available.') : tr('Сбор включается только вручную. Обновление RouterForge не запускает службу.','Collection starts only when you choose it; RouterForge updates never start the service.')}</p>
    </section>

    <section class="panel rf-syslog-space">
      <div class="rf-syslog-heading"><div><strong>{tr('Настройка сбора','Collector configuration')}</strong><span>{status.config_path}</span></div><span class="rf-chip">{status.managed?'ROUTERFORGE / SAVED':'EXTERNAL / NOT IMPORTED'}</span></div>
      {#if status.config_exists}
        <div class="rf-existing-toolbar">
          <div><strong>{tr('Текущая конфигурация syslog-ng','Existing syslog-ng configuration')}</strong><small>{tr('Реальный файл на роутере · только просмотр','Actual router file · read-only')}</small></div>
          <button class="button" onclick={toggleConfigPreview} disabled={configBusy || !!busy}>{configBusy?'…':configVisible?tr('Скрыть','Hide'):tr('Просмотреть / импортировать для просмотра','View / import read-only')}</button>
        </div>
        {#if configVisible && configPreview}
          <div class="rf-existing-preview">
            <div class="rf-preview-meta"><span>{configPreview.path} · {configPreview.total_bytes} B · SHA256 {configPreview.sha256.slice(0,12)}…</span><strong>READ ONLY</strong></div>
            <p class="rf-hint">{tr('Показан текст существующего файла без преобразования и записи. Он может содержать пароли и ключи — не публикуй его содержимое.','This is the original file, displayed without conversion or changes. It may contain credentials; do not share it.')}</p>
            {#if configPreview.truncated}<p class="rf-syslog-msg rf-warning">{tr('Показаны первые 64 КиБ. Полный файл не изменён.','Only the first 64 KiB are shown. The original file is unchanged.')}</p>{/if}
            <pre class="rf-config-preview">{configPreview.content}</pre>
          </div>
        {/if}
      {:else}
        <p class="rf-hint">{tr('Файл syslog-ng.conf отсутствует.','No syslog-ng.conf exists yet.')}</p>
      {/if}
      <div class="rf-easy-shell">
        <div class="rf-easy-head"><strong>{tr('Настройка за три шага','Three-step setup')}</strong><span>{tr('Сложные параметры мы определяем автоматически.','Technical settings are selected automatically.')}</span></div>
        <div class="rf-easy-step">
          <div class="rf-step-title"><span class="rf-step-number">1</span><div><strong>{tr('Где хранить журналы?','Where should logs be saved?')}</strong><small>{tr('Выбирай подключённый накопитель или готовую SMB-папку.','Choose a mounted disk or existing SMB share.')}</small></div></div>
          <div class="rf-radio-pills" role="group" aria-label={tr('Тип хранилища','Storage type')}>
            <button type="button" class:chosen={storageKind==='usb'} onclick={()=>changeStorage('usb')}>{tr('USB','USB')}</button>
            <button type="button" class:chosen={storageKind==='smb'} onclick={()=>changeStorage('smb')}>SMB / NAS</button>
            {#if hasSeparateEntware}<button type="button" class:chosen={storageKind==='local'} onclick={()=>changeStorage('local')}>{tr('Другое хранилище Entware','Other Entware storage')}</button>{/if}
          </div>
          {#if availableStorage.length}
            <label class="rf-block">{tr('Накопитель','Storage')}
              <select value={draft.root} onchange={(event)=>{draft.root=event.currentTarget.value;mark();}}>
                {#each availableStorage as item (item.root)}<option value={item.root}>{item.label} · {item.root}{item.free_bytes ? ` · ${(item.free_bytes/1073741824).toFixed(1)} GB ${tr('свободно','free')}` : ''}</option>{/each}
                {#if draft.root && !availableStorage.some(o=>o.root===draft.root)}<option value={draft.root}>{draft.root} ({tr('недоступно','unavailable')})</option>{/if}
              </select>
            </label>
            {#if selectedStorage}
              <p class="rf-hint">{tr('Каталог журналов:','Log folder:')} <code>{selectedStorage.root}</code>{selectedStorage.free_bytes > 0 ? ` · ${(selectedStorage.free_bytes/1073741824).toFixed(1)} ${tr('ГБ свободно','GB free')}` : ''}</p>
              {#if storageKind==='usb' && entwareOnUSB && selectedStorage.root === '/opt/var/log/routerforge-syslog'}
                <p class="rf-syslog-msg rf-ok">{tr('Это ваша флешка с Entware. Дополнительный USB-накопитель не требуется.','This is your Entware USB drive. No additional USB disk is needed.')}</p>
              {/if}
            {/if}
          {:else if storageKind !== 'smb'}
            <div class="rf-syslog-msg rf-warning">{tr('Подходящие накопители не обнаружены. Проверь подключение и нажми «Обновить».','No suitable mounted storage. Connect a disk and refresh.')}</div>
          {/if}
          {#if storageKind==='smb'}
            <div class="rf-smb-wizard">
              <div class="rf-smb-title"><strong>{tr('Подключить сетевую папку','Connect a network share')}</strong><button type="button" class="button" onclick={loadSMB} disabled={smbBusy}>↻ {tr('Проверить поддержку','Check support')}</button></div>
              {#if smbError}<div class="rf-syslog-msg rf-warning" role="alert">{smbError}</div>{/if}
              {#if smb?.connected}
                <div class="rf-syslog-msg rf-ok">{tr('SMB-папка подключена','SMB share connected')}{smb.host ? `: ${smb.host}/${smb.share}` : ''}. {tr('Каталог журналов','Log folder')}: {smb.mount_root}</div>
                <div class="rf-actions"><button type="button" class="button" onclick={smbDisconnect} disabled={smbBusy}>{tr('Отключить SMB','Disconnect SMB')}</button></div>
              {:else}
                {#if smb && (!smb.client_ready || !smb.kernel_ready)}
                  <div class="rf-syslog-msg rf-warning">{!smb.client_ready ? tr('Не установлен SMB-клиент (mount.cifs).','SMB client (mount.cifs) is missing.') : tr('CIFS-клиент установлен.','CIFS client installed.')} {smb.kernel_ready ? '' : tr('Поддержка CIFS в ядре KeeneticOS не подтверждена; установка пакета Entware сама по себе её не добавит.','CIFS kernel support is not detected; installing an Entware package alone cannot provide it.')}</div>
                  {#if !smb.client_ready}
                    <div class="rf-actions">
                      <button type="button" class="button primary" onclick={installSMBClient} disabled={smbBusy || !!busy}>{smbBusy?'…':tr('Установить поддержку SMB','Install SMB client')}</button>
                      <a href="/apps">{tr('Открыть App Center','Open App Center')}</a>
                    </div>
                    <p class="rf-hint">{tr('Установка cifsmount — только с вашего согласия. Она не добавляет отсутствующий драйвер CIFS в прошивку KeeneticOS.','Installing cifsmount requires your consent; it cannot add a missing CIFS kernel driver to KeeneticOS.')}</p>
                  {/if}
                {/if}
                <p class="rf-hint">{tr('Укажи адрес NAS, имя сетевой папки и отдельную учётную запись с доступом только к журналам.','Enter NAS address, share name and a dedicated account with log-only permissions.')}</p>
                <div class="rf-smb-fields">
                  <label>{tr('Адрес NAS','NAS address')}<input bind:value={smbHost} placeholder="192.168.10.20" autocomplete="off" /></label>
                  <label>{tr('Сетевая папка','Shared folder')}<input bind:value={smbShare} placeholder="RouterLogs" autocomplete="off" /></label>
                  <label>{tr('Пользователь','Username')}<input bind:value={smbUser} autocomplete="off" /></label>
                  <label>{tr('Пароль','Password')}<input type="password" bind:value={smbPassword} autocomplete="new-password" /></label>
                  <label>{tr('Домен (необязательно)','Domain (optional)')}<input bind:value={smbDomain} placeholder="WORKGROUP" autocomplete="off" /></label>
                </div>
                <label class="rf-smb-remember"><input type="checkbox" bind:checked={smbRemember} />{tr('Автоподключение после перезагрузки (учётные данные хранятся в защищённом файле на Entware, доступном root)','Reconnect after reboot (credentials saved in a root-only Entware file)')}</label>
                <div class="rf-actions"><button type="button" class="button primary" onclick={smbConnect} disabled={smbBusy || !smb?.client_ready || !smb?.kernel_ready || !smbHost.trim() || !smbShare.trim() || !smbUser || !smbPassword}>{smbBusy?'…':tr('Подключить и проверить запись','Connect & test write access')}</button></div>
              {/if}
              <p class="rf-hint">{tr('SMB3. Пароль не появляется в логах или команде процесса. Если NAS пропадёт, запись на флеш-память не переключается.','SMB3. Password is not passed via process arguments or logs. No fallback to internal flash if NAS disconnects.')}</p>
            </div>
          {/if}
        </div>
        <div class="rf-easy-step">
          <div class="rf-step-title"><span class="rf-step-number">2</span><div><strong>{tr('Как назвать устройства?','Name your devices')}</strong><small>{tr('IP нужен для записи, а имя — чтобы легко находить файлы.','IP is used for routing; the name is used in filenames.')}</small></div></div>
          <div class="rf-device-list">
            {#each namedDevices as [ip,label] (ip)}
              <div class="rf-device-row"><span class="mono">{ip}</span><span>{label}</span><button type="button" class="button" onclick={()=>removeDevice(ip)} aria-label={tr('Удалить имя','Remove name')}>×</button></div>
            {/each}
            {#if !namedDevices.length}<p class="rf-hint">{tr('Пока без имён: файлы будут называться по IP.','No aliases yet: filenames will use IP addresses.')}</p>{/if}
          </div>
          <div class="rf-device-add">
            <input aria-label="IPv4" placeholder="192.168.10.2" bind:value={newDeviceIP} />
            <input aria-label={tr('Название устройства','Device name')} placeholder={tr('Ретранслятор_Спальня','Bedroom_Repeater')} bind:value={newDeviceName} />
            <button type="button" class="button" onclick={addDevice}>{tr('Добавить','Add')}</button>
          </div>
          <p class="rf-hint">{tr('Для головного Keenetic тоже можно добавить имя, например Главный_Keenetic. Устройства без имени сохраняются по IP.','You can name the main Keenetic too. Unnamed devices use their IPs.')}</p>
          <p class="rf-syslog-msg rf-hint">{tr('Важно: RouterForge принимает журналы, но не включает их отправку на других устройствах. Настройте пересылку Syslog на каждом Keenetic и ретрансляторе:','Important: RouterForge receives logs but does not enable forwarding on other devices. Configure Syslog forwarding on each Keenetic and repeater:')} <strong>{draft.head_ip || '192.168.10.1'}:{draft.port || 514}</strong> (UDP).</p>
        </div>
        <div class="rf-easy-step">
          <div class="rf-step-title"><span class="rf-step-number">3</span><div><strong>{tr('Как часто создавать файлы?','How often to create files?')}</strong><small>{tr('Время нового файла и срок хранения не связаны.','File period and retention are separate settings.')}</small></div></div>
          <div class="rf-easy-grid">
            <label>{tr('Новый файл','New file')}<select bind:value={draft.rotate_hours} onchange={mark}>
              <option value={24}>{tr('Каждые сутки','Every day')}</option><option value={12}>{tr('Каждые 12 часов','Every 12 hours')}</option><option value={6}>{tr('Каждые 6 часов','Every 6 hours')}</option><option value={1}>{tr('Каждый час','Every hour')}</option>
            </select></label>
            <label>{tr('Хранить журналы','Keep logs')}<select bind:value={draft.keep_days} onchange={mark}>
              <option value={7}>{tr('7 дней','7 days')}</option><option value={30}>{tr('30 дней','30 days')}</option><option value={90}>{tr('90 дней','90 days')}</option><option value={365}>{tr('1 год','1 year')}</option>
              {#if ![7,30,90,365].includes(draft.keep_days)}<option value={draft.keep_days}>{draft.keep_days} {tr('дней','days')}</option>{/if}
            </select></label>
            <label>{tr('Ограничение по объёму, МБ','Size limit, MB')}<input type="number" min="10" max="10240" bind:value={draft.max_mb} oninput={mark}/></label>
          </div>
          <p class="rf-hint">{tr('Например: раз в 6 часов, хранить 30 дней. Старые файлы очищаются каждый час.','Example: new file every 6 hours, retain for 30 days. Old files are cleaned hourly.')}</p>
          <div class="rf-file-example"><small>{tr('Пример названия файла','Example filename')}</small><code>{`syslog-${namedDevices[0]?.[1] || draft.head_ip || 'router'}${draft.rotate_hours === 24 ? '' : draft.rotate_hours === 12 ? '-00-12' : draft.rotate_hours === 6 ? '-00-06' : '-00-01'}.log`}</code></div>
        </div>
      </div>
      <button type="button" class="button rf-advanced-toggle" onclick={()=>simpleMode=!simpleMode}>{simpleMode?tr('⚙ Дополнительные настройки','⚙ Advanced settings'):tr('Скрыть дополнительные настройки','Hide advanced settings')}</button>
      {#if !simpleMode}
      <div class="rf-draft-heading"><strong>{tr('Черновик дополнения RouterForge','RouterForge sidecar draft')}</strong><span>{tr('Правила RouterForge сохраняются отдельно: основной конфиг не заменяется, в него однократно добавляется строка подключения.','RouterForge rules are stored separately; the main config is not replaced, only one include directive is added.')}</span></div>
      <p class="rf-syslog-msg rf-ok">{tr('P45.3: не перезаписываем сторонние правила, не открываем /dev/log повторно, не запускаем службу автоматически.','P45.3: foreign rules are preserved; /dev/log is never opened twice; the service is not started automatically.')}</p>
      <div class="rf-fields">
        <label>{tr('Адрес прослушивания (если создаём свой сетевой источник)','Listen address (new network source only)')}<select bind:value={draft.bind} onchange={mark}><option value="0.0.0.0">0.0.0.0 (LAN)</option><option value="127.0.0.1">127.0.0.1 (local)</option></select></label>
        <label>{tr('Порт','Port')}<input type="number" min="1024" max="65535" bind:value={draft.port} oninput={mark} /></label>
        <label>{tr('Хранение, дней','Retention, days')}<input type="number" min="1" max="365" bind:value={draft.keep_days} oninput={mark} /></label>
        <label>{tr('Общий лимит, МБ','Total limit, MB')}<input type="number" min="10" max="10240" bind:value={draft.max_mb} oninput={mark} /></label>
      </div>
      <div class="rf-checks">
        <label><input type="checkbox" bind:checked={draft.udp} onchange={mark}/> UDP</label>
        <label><input type="checkbox" bind:checked={draft.tcp} onchange={mark}/> TCP</label>
        <label><input type="checkbox" bind:checked={draft.local} onchange={mark}/> {tr('Локальные события через существующий источник','Local events via existing source')}</label>
      </div>
      <div class="rf-fields rf-source-fields">
        <label>{tr('Сетевой источник из существующего конфига','Existing remote source')}
          <select bind:value={draft.remote_source} onchange={mark}><option value="">{tr('Создать новый (если нет конфликтов)','Create new (if conflict-free)')}</option>{#each status.source_names||[] as name}<option value={name}>{name}</option>{/each}</select>
        </label>
        <label>{tr('Локальный источник Entware / Keenetic','Existing local source')}
          <select bind:value={draft.local_source} onchange={mark}><option value="">{tr('Не выбран','Not selected')}</option>{#each status.source_names||[] as name}<option value={name}>{name}</option>{/each}</select>
        </label>
        <label>{tr('IPv4 головного Keenetic','Main Keenetic IPv4')}
          <input bind:value={draft.head_ip} oninput={mark} placeholder="192.168.10.1" />
        </label>
      </div>
      <p class="rf-hint">{tr('Существующие источники переиспользуются без второго прослушивания порта или /dev/log. Если источник принимает сообщения уже сейчас, RouterForge только добавляет маршруты записи.','Existing sources are reused without binding the port or /dev/log a second time. RouterForge adds destinations for received messages.')}</p>
      <label class="rf-block">{tr('Разрешённые сети (IPv4 CIDR, через запятую)','Allowed networks (IPv4 CIDRs, comma separated)')}
        <input value={draft.allowed.join(', ')} oninput={(e)=>{draft.allowed=e.currentTarget.value.split(',').map(x=>x.trim()).filter(Boolean);mark();}} placeholder="192.168.0.0/16, 10.0.0.0/8" />
      </label>
      <label class="rf-block">{tr('Каталог журналов','Log directory')}<input bind:value={draft.root} oninput={mark} placeholder="/opt/var/log/routerforge-syslog" /></label>
      <p class="rf-hint">{tr('Доступны отдельный каталог Entware и /opt/mnt/УСТРОЙСТВО/routerforge-syslog. Хранилище должно быть примонтировано; запись во внутреннюю память при исчезновении USB запрещена.','Use the dedicated Entware directory or /opt/mnt/DEVICE/routerforge-syslog. Storage must be mounted; no fallback writes to internal flash.')}</p>
      {#if !status.storage_ready}<p class="rf-syslog-msg rf-warning">{status.storage_error || tr('Накопитель недоступен','Storage unavailable')}</p>{/if}
      {/if}
      <div class="rf-actions"><button class="button primary" onclick={prepare} disabled={!!busy||planBusy||!status.binary_ready||!draft.root||!selectedStorage}>{planBusy?'…':tr('Предпросмотр и проверка плана','Preview & preflight')}</button></div>
      {#if plan}
        <div class="rf-plan">
          <strong>{tr('Готово к сохранению — служба не запускается','Ready to save — service is not started')}</strong>
          <p>{tr('Основной файл:','Main config:')} <b>{plan.parent_action==='unchanged'?tr('не изменится','unchanged'):tr('добавится безопасное подключение дополнения','a safe add-on include is added')}</b></p>
          {#if !simpleMode}<p class="mono">{plan.include_line}</p>{/if}
          <p>{tr('Отдельный файл RouterForge:','Separate RouterForge file:')} <span class="mono">{plan.sidecar_path}</span></p>
          {#if !simpleMode}<pre class="rf-config-preview">{plan.sidecar_content}</pre>{/if}
          <label class="rf-danger"><input type="checkbox" bind:checked={acceptAdoption}/> {tr('Я проверил план и разрешаю добавить только дополнение RouterForge; основную конфигурацию не заменять.','I reviewed the plan and approve only the RouterForge sidecar; do not replace the main config.')}</label>
          <button class="button primary" onclick={apply} disabled={!!busy||!acceptAdoption}>{busy==='apply'?'…':tr('Проверить синтаксис и сохранить дополнение','Validate syntax & save sidecar')}</button>
        </div>
      {/if}
      <p class="rf-hint">{tr('При сохранении проверяется полный кандидат через syslog-ng -s, создаются резервные копии и публикуется только дополнение и строка подключения. Применение — только после ручного START/RESTART.','On save, full candidate syntax is checked with syslog-ng -s, backups are created, and only the sidecar and include directive are published. Activation requires manual START/RESTART.')}</p>
    </section>

    {#if status.managed}
      <section class="panel rf-syslog-space">
        <div class="rf-syslog-heading"><div><strong>{tr('Журналы устройств','Device journals')}</strong><span>{files.length} {tr('файлов (последние 250)','files (latest 250)')}</span></div><button class="button" onclick={() => refresh()} disabled={!!busy}>↻</button></div>
        {#if summary}
          <div class="rf-syslog-status rf-inventory">
            <div><small>{tr('Всего файлов','Total files')}</small><strong>{summary.files}</strong></div>
            <div><small>{tr('Размер журналов','Log data')}</small><strong>{(summary.total_bytes/1048576).toFixed(1)} MiB</strong></div>
            <div><small>{tr('Период','Date range')}</small><strong>{summary.oldest_date||'—'} — {summary.latest_date||'—'}</strong></div>
            <div><small>{tr('Очистка раз в час','Hourly cleanup')}</small><strong>{summary.cleanup_files} {tr('файлов','files')} · {(summary.cleanup_bytes/1048576).toFixed(1)} MiB</strong></div>
          </div>
          <div class="rf-source-inventory">
            {#each summary.sources||[] as source}
              <div><strong class="mono">{source.source==='entware'?'Entware':source.source}</strong><span>{source.files} {tr('файлов','files')} · {(source.bytes/1048576).toFixed(1)} MiB · {source.latest}</span></div>
            {/each}
          </div>
          <p class="rf-hint">{tr('Кандидаты на очистку показаны справочно. Очистка выполняется автоматически раз в час. Файлы текущего дня защищены; другие каталоги не затрагиваются.','Cleanup candidates are informational. Auto cleanup runs hourly. Current-day files and unrelated directories are protected.')}</p>
        {/if}
        <div class="rf-log-layout">
          <div class="rf-log-files">
            {#each files as file (`${file.date}/${file.filename}`)}
              <button class:selected={chosen?.date===file.date&&chosen?.filename===file.filename} onclick={() => selectFile(file)}>
                <strong>{file.display || file.source}</strong><small>{file.date}{file.interval ? ` · ${file.interval}` : ''} · {(file.size/1024).toFixed(1)} KB</small>
              </button>
            {/each}
            {#if !files.length}<div class="rf-syslog-empty">{tr('Журналы пока пусты','No journals yet')}</div>{/if}
          </div>
          <div class="rf-log-reader">
            <div class="rf-actions rf-log-toolbar"><input type="search" bind:value={filter} placeholder={tr('Поиск по строкам','Search lines')} /><button class="button" onclick={reloadTail} disabled={!chosen}>↻</button>
              {#if chosen}<a class="button" href={routerforgeSyslogExportURL(chosen.date,chosen.source)}>{tr('Скачать','Export')}</a>{/if}
            </div>
            <div class="rf-log-lines">{#each visibleLines as line,i (i)}<div>{line}</div>{/each}{#if !visibleLines.length}<span>{tr('Нет записей','No entries')}</span>{/if}</div>
          </div>
        </div>
        <p class="rf-hint">{tr('Очистка управляемых журналов выполняется ежечасно по сроку и лимиту. Текущие и чужие файлы не удаляются.','Managed journals are cleaned hourly by retention and size limits. Current-day and foreign files are never removed.')}</p>
      </section>
    {/if}
  {/if}
</div>

<style>
.rf-syslog{display:grid;gap:12px;min-width:0}.rf-syslog-space{padding:14px;display:grid;gap:12px}.rf-syslog-space p{margin:0}.rf-syslog-status{padding:12px;display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px}.rf-syslog-status>div{display:grid;gap:4px;border:1px solid var(--rf-border,#333);border-radius:7px;padding:10px;min-width:0}.rf-syslog-status small,.rf-hint,.rf-syslog-heading span{color:var(--rf-muted,#a2adb8);font-size:11px}.rf-syslog-status strong{font-size:13px;overflow-wrap:anywhere}.rf-syslog-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.rf-syslog-heading>div{display:grid;gap:4px}.rf-fields{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.rf-fields label,.rf-block{min-width:0;display:grid;gap:5px;font-size:12px;color:var(--rf-muted,#aaa)}.rf-fields input,.rf-fields select,.rf-block input,.rf-log-toolbar input{min-width:0;width:100%;box-sizing:border-box;border:1px solid var(--rf-border,#333);background:var(--rf-surface-2,#15191e);border-radius:6px;color:var(--rf-text,#fff);padding:8px}.rf-checks,.rf-actions{display:flex;gap:9px;flex-wrap:wrap;align-items:center}.rf-checks label,.rf-danger{display:flex;gap:6px;align-items:center;font-size:12px}.rf-danger{color:var(--warn,#e9b459)}.rf-syslog-msg{padding:10px;border:1px solid var(--rf-border,#333);border-radius:6px;overflow-wrap:anywhere;font-size:12px}.rf-error{color:var(--error,#ff6565)}.rf-ok{color:var(--good,#7ad49e)}.rf-warning{color:var(--warn,#e9b459)}.rf-syslog-empty{color:var(--rf-muted,#aaa);padding:16px}.rf-log-layout{display:grid;grid-template-columns:minmax(180px,260px) minmax(0,1fr);min-height:230px;border:1px solid var(--rf-border,#333);border-radius:7px;overflow:hidden}.rf-log-files{overflow:auto;max-height:360px;border-right:1px solid var(--rf-border,#333)}.rf-log-files button{display:grid;width:100%;text-align:left;gap:4px;border:0;border-bottom:1px solid var(--rf-border,#333);background:transparent;color:inherit;padding:10px;cursor:pointer}.rf-log-files button.selected{background:var(--rf-accent-soft,rgba(56,189,248,.15))}.rf-log-files small{color:var(--rf-muted,#aaa)}.rf-log-reader{display:grid;grid-template-rows:auto 1fr;min-width:0}.rf-log-toolbar{padding:8px;border-bottom:1px solid var(--rf-border,#333)}.rf-log-toolbar input{flex:1;width:auto}.rf-log-lines{padding:10px;font:11px/1.55 var(--font-mono,monospace);overflow:auto;max-height:315px;white-space:pre-wrap;word-break:break-word}.rf-chip{font-size:10px}.rf-actions a.button{text-decoration:none}@media(max-width:900px){.rf-fields{grid-template-columns:repeat(2,minmax(0,1fr))}.rf-syslog-status{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.rf-log-layout{grid-template-columns:1fr}.rf-log-files{max-height:170px;border-right:0;border-bottom:1px solid var(--rf-border,#333)}.rf-fields{grid-template-columns:1fr}}
  .rf-service-controls{display:grid;gap:10px;padding:12px 14px;min-width:0}
  .rf-service-row{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:12px;min-width:0}
  .rf-service-title{display:grid;gap:4px;min-width:0}
  .rf-service-title strong{font-size:14px}
  .rf-service-title span{color:var(--rf-muted,#a2adb8);font-size:11px}
  .rf-service-actions{display:flex!important;flex-flow:row wrap!important;align-items:center!important;justify-content:flex-end!important;gap:7px}
  .rf-service-actions>.button{width:auto!important;min-width:85px;flex:0 0 auto!important;padding:8px 12px}
  .rf-existing-toolbar{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;padding:10px;border:1px solid var(--rf-border,#333);border-radius:7px}
  .rf-existing-toolbar>div{display:grid;gap:3px}
  .rf-existing-toolbar strong,.rf-draft-heading strong{font-size:13px}
  .rf-existing-toolbar small,.rf-draft-heading span{font-size:11px;color:var(--rf-muted,#a2adb8)}
  .rf-existing-preview{display:grid;gap:8px;padding:10px;border:1px solid var(--rf-border,#333);border-radius:7px;min-width:0}
  .rf-preview-meta{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap;color:var(--rf-muted,#a2adb8);font-size:11px;overflow-wrap:anywhere}
  .rf-preview-meta strong{color:var(--warn,#e9b459)}
  .rf-config-preview{box-sizing:border-box;margin:0;padding:12px;background:var(--rf-bg,#0b0d10);border:1px solid var(--rf-border,#333);color:var(--rf-text,#fff);font:11px/1.55 var(--font-mono,monospace);white-space:pre;overflow:auto;max-height:420px;min-width:0}
  .rf-draft-heading{display:grid;gap:4px;padding-top:5px}
  .rf-plan{padding:12px;display:grid;gap:10px;min-width:0;border:1px solid var(--rf-border,#333);border-radius:7px}
  .rf-plan p{margin:0;color:var(--rf-muted,#aaa);font-size:12px}
  .rf-plan .mono{font-size:11px;overflow-wrap:anywhere}
  .rf-source-fields{grid-template-columns:repeat(3,minmax(0,1fr))}
  @media(max-width:850px){.rf-source-fields{grid-template-columns:1fr}}
  .rf-inventory{padding:0}
  .rf-source-inventory{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:7px}
  .rf-source-inventory>div{display:grid;gap:3px;padding:9px;border:1px solid var(--rf-border,#333);border-radius:6px}
  .rf-source-inventory span{font-size:11px;color:var(--rf-muted,#aaa)}

  .rf-file-example{display:flex;gap:8px;align-items:baseline;flex-wrap:wrap;min-width:0}
  .rf-file-example code{min-width:0;overflow-wrap:anywhere;word-break:break-word}
  .rf-file-example small{color:var(--rf-muted,#aaa)}
  .rf-easy-shell{display:grid;gap:12px;min-width:0}
  .rf-easy-head{display:grid;gap:4px}.rf-easy-head strong{font-size:15px}.rf-easy-head span{color:var(--rf-muted,#aaa);font-size:12px}
  .rf-easy-step{display:grid;gap:12px;padding:14px;border:1px solid var(--rf-border,#333);border-radius:9px;min-width:0}
  .rf-step-title{display:flex;align-items:center;gap:10px}.rf-step-title>div{display:grid;gap:3px}.rf-step-title strong{font-size:14px}.rf-step-title small{font-size:11px;color:var(--rf-muted,#aaa)}
  .rf-step-number{display:grid;place-items:center;width:28px;height:28px;flex:0 0 28px;border-radius:50%;background:var(--rf-accent-soft,rgba(56,189,248,.10));color:var(--rf-accent,#38bdf8);font-weight:700}
  .rf-radio-pills{display:flex;gap:7px;flex-wrap:wrap}.rf-radio-pills button{padding:9px 15px;border-radius:7px;border:1px solid var(--rf-border,#333);background:var(--rf-surface-2,#171b21);color:var(--rf-text,#fff);cursor:pointer}.rf-radio-pills button.chosen{border-color:var(--rf-accent,#38bdf8);background:var(--rf-accent-soft,rgba(56,189,248,.1))}
  .rf-easy-step select,.rf-easy-step input{min-width:0;box-sizing:border-box;width:100%;padding:9px;border:1px solid var(--rf-border,#333);border-radius:6px;background:var(--rf-surface-2,#171b21);color:var(--rf-text,#fff)}
  .rf-easy-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.rf-easy-grid label{display:grid;gap:5px;color:var(--rf-muted,#aaa);font-size:12px}
  .rf-device-list{display:grid;gap:5px}.rf-device-row{display:grid;grid-template-columns:minmax(90px,170px) 1fr auto;gap:8px;align-items:center;font-size:12px}.rf-device-add{display:grid;grid-template-columns:minmax(110px,180px) 1fr auto;gap:8px}
  .rf-advanced-toggle{width:fit-content}
  @media(max-width:650px){.rf-easy-grid{grid-template-columns:1fr}.rf-device-add{grid-template-columns:1fr}.rf-device-row{grid-template-columns:1fr auto}.rf-device-row .mono{grid-column:1 / -1}}
  .rf-smb-wizard{display:grid;gap:10px;padding:12px;border:1px solid var(--rf-border,#333);border-radius:8px}
  .rf-smb-title{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap}
  .rf-smb-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}
  .rf-smb-fields label{display:grid;gap:5px;font-size:12px;color:var(--rf-muted,#aaa)}
  .rf-smb-fields input{width:100%;min-width:0;box-sizing:border-box;border:1px solid var(--rf-border,#333);background:var(--rf-surface-2,#15191e);color:var(--rf-text,#fff);border-radius:6px;padding:9px}
  .rf-smb-remember{font-size:12px;display:flex;align-items:flex-start;gap:8px}
  @media(max-width:600px){.rf-smb-fields{grid-template-columns:1fr}}
</style>
