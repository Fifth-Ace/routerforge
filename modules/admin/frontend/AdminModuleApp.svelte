<script>
  import { onMount } from 'svelte';
  import {
    adminFileChmod,
    adminFileDelete,
    adminFileDownloadURL,
    adminFileMkdir,
    adminFileMove,
    adminFileWrite,
    adminProcessSignal,
    adminServiceAction,
    adminTerminalRun,
    adminMaintenanceBackup,
    adminMaintenanceRestore,
    configureAdminWatchdog,
    createAdminSnapshot,
    deleteAdminSnapshot,
    adminNetworkToolRun,
    getAdminMaintenanceBackups,
    getAdminMaintenanceLogs,
    getAdminSnapshots,
    getAdminWatchdogs,
    getAdminMaintenanceTasks,
    getAdminIntegrations,
    getAdminFiles,
    getModule,
    readAdminFile
  } from '$lib/api.js';
  import { startSerialPolling } from '$lib/polling.js';
  import { bytes } from '$lib/utils.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  const FILE_EDITOR_WRITE_LIMIT = 128 * 1024;

  let tab = 'processes';
  let processes = [];
  let ports = [];
  let services = [];
  let packages = [];
  let search = '';
  let loading = false;
  let errorText = '';
  let actionText = '';

  let filePath = '/opt';
  let fileEntries = [];
  let fileLoading = false;
  let selectedFile = null;
  let editorContent = '';
  let editorOriginal = '';
  let editorBusy = false;

  let terminalCommand = '';
  let terminalCwd = '/opt';
  let terminalLines = [];
  let terminalBusy = false;

  let maintenanceLogs = '';
  let maintenanceLogSource = '';
  let maintenanceTasks = [];
  let maintenanceBusy = false;
  let maintenanceBackups = [];
  let restoreBusyPath = '';
  let watchdogs = [];
  let watchdogBusyID = '';
  let snapshots = [];
  let snapshotBusy = false;

  let networkTool = 'ping';
  let networkHost = '1.1.1.1';
  let networkPort = 443;
  let networkResult = null;
  let networkBusy = false;

  let integrations = [];
  let integrationsBusy = false;

  $: locale = $settings.locale || 'ru';
  $: copy = locale === 'ru' ? {
    files: 'Файлы',
    terminal: 'Терминал',
    run: 'Выполнить',
    clear: 'Очистить',
    terminalHint: 'Команды выполняются через /bin/sh -lc от root, cwd разрешён только внутри /opt или /tmp. Таймаут 15 секунд.',
    maintenance: 'Обслуживание',
    networkTools: 'Сеть',
    logs: 'Логи',
    tasks: 'Cron / задачи',
    backup: 'Создать backup',
    backupConfirm: 'Создать config-only архив RouterForge в /tmp/routerforge-backups?',
    restore: 'Восстановить',
    restoreConfirm: 'ВОССТАНОВИТЬ КОНФИГ из выбранного backup? Перед заменой RouterForge автоматически создаст safety backup текущего конфига. Автоперезапуска не будет.',
    backups: 'Backup / Restore',
    restoreMode: 'Restore меняет только /opt/etc/routerforge. Старый /opt/share/routerforge из legacy backup игнорируется.',
    watchdogs: 'Watchdogs',
    watchdogHint: 'Автозапуск только известных интеграций. По умолчанию выключен; 30 сек проверка, cooldown 5 мин, максимум 3 попытки/час.',
    enable: 'Включить',
    disable: 'Выключить',
    attempts: 'Попытки/час',
    snapshots: 'Диагностические snapshots',
    snapshotHint: 'Снимок summary/processes/services/ports/storage/thermal/integrations. Хранятся в /tmp, максимум 32.',
    createSnapshot: 'Создать snapshot',
    networkHint: 'Ping, traceroute, DNS lookup и TCP connect test без shell-интерполяции.',
    integrations: 'Интеграции',
    integrationsHint: 'Автообнаружение nfqws2, AWG Manager и AdGuard Home: бинарники, сервисы, процессы и listening-порты.',
    detected: 'Обнаружено',
    notDetected: 'Не обнаружено',
    running: 'Работает',
    stopped: 'Остановлено',
    service: 'Сервис',
    processesLabel: 'PID',
    portsLabel: 'Порты',
    pathsLabel: 'Пути',
    path: 'Путь',
    up: 'Вверх',
    open: 'Открыть',
    download: 'Скачать',
    rename: 'Переименовать',
    remove: 'Удалить',
    chmod: 'Права',
    mkdir: 'Новая папка',
    newFile: 'Новый файл',
    save: 'Сохранить',
    close: 'Закрыть',
    actions: 'Действия',
    processConfirm: 'Отправить сигнал процессу',
    serviceConfirm: 'Выполнить действие с сервисом',
    deleteConfirm: 'Удалить объект? Рекурсивное удаление отключено.',
    renamePrompt: 'Полный новый путь',
    chmodPrompt: 'Права, например 0644',
    mkdirPrompt: 'Имя новой папки',
    filePrompt: 'Имя нового файла',
    empty: 'Пусто',
    dirty: 'Есть несохранённые изменения',
    mutationLocked: 'Изменения требуют активной root-сессии RouterForge.',
    noPreview: 'Предпросмотр доступен только для UTF-8 текстовых файлов до 256 KiB.',
    editorReadOnly: 'Только просмотр: редактирование ограничено 128 KiB.'
  } : {
    files: 'Files',
    terminal: 'Terminal',
    run: 'Run',
    clear: 'Clear',
    terminalHint: 'Commands run through /bin/sh -lc as root; cwd is restricted to /opt or /tmp. Timeout is 15 seconds.',
    maintenance: 'Maintenance',
    networkTools: 'Network',
    logs: 'Logs',
    tasks: 'Cron / tasks',
    backup: 'Create backup',
    backupConfirm: 'Create a config-only RouterForge archive in /tmp/routerforge-backups?',
    restore: 'Restore',
    restoreConfirm: 'RESTORE CONFIG from the selected backup? RouterForge creates a safety backup of the current config before the swap. No automatic restart.',
    backups: 'Backup / Restore',
    restoreMode: 'Restore changes /opt/etc/routerforge only. Legacy /opt/share/routerforge content is ignored.',
    watchdogs: 'Watchdogs',
    watchdogHint: 'Auto-start is limited to known integrations. Disabled by default; 30s checks, 5m cooldown, max 3 attempts/hour.',
    enable: 'Enable',
    disable: 'Disable',
    attempts: 'Attempts/hour',
    snapshots: 'Diagnostic snapshots',
    snapshotHint: 'Captures summary/processes/services/ports/storage/thermal/integrations. Stored in /tmp, max 32.',
    createSnapshot: 'Create snapshot',
    networkHint: 'Ping, traceroute, DNS lookup and TCP connect test without shell interpolation.',
    integrations: 'Integrations',
    integrationsHint: 'Auto-detection for nfqws2, AWG Manager and AdGuard Home: binaries, services, processes and listening ports.',
    detected: 'Detected',
    notDetected: 'Not detected',
    running: 'Running',
    stopped: 'Stopped',
    service: 'Service',
    processesLabel: 'PID',
    portsLabel: 'Ports',
    pathsLabel: 'Paths',
    path: 'Path',
    up: 'Up',
    open: 'Open',
    download: 'Download',
    rename: 'Rename',
    remove: 'Delete',
    chmod: 'Mode',
    mkdir: 'New folder',
    newFile: 'New file',
    save: 'Save',
    close: 'Close',
    actions: 'Actions',
    processConfirm: 'Send signal to process',
    serviceConfirm: 'Run service action',
    deleteConfirm: 'Delete object? Recursive delete is disabled.',
    renamePrompt: 'Full destination path',
    chmodPrompt: 'Mode, for example 0644',
    mkdirPrompt: 'New folder name',
    filePrompt: 'New file name',
    empty: 'Empty',
    dirty: 'Unsaved changes',
    mutationLocked: 'Mutations require an active RouterForge root session.',
    noPreview: 'Preview supports UTF-8 text files up to 256 KiB only.',
    editorReadOnly: 'Read-only preview: editing is limited to 128 KiB.'
  };

  $: tabs = [
    ['processes', t(locale, 'manage.tabs.processes')],
    ['ports', t(locale, 'manage.tabs.ports')],
    ['services', t(locale, 'manage.tabs.services')],
    ['packages', t(locale, 'manage.tabs.packages')],
    ['files', copy.files],
    ['terminal', copy.terminal],
    ['maintenance', copy.maintenance],
    ['network-tools', copy.networkTools],
    ['integrations', copy.integrations]
  ];

  $: q = search.trim().toLowerCase();
  $: filteredProcesses = processes.filter((p) => !q || `${p.pid} ${p.name} ${p.user} ${p.command}`.toLowerCase().includes(q));
  $: filteredPorts = ports.filter((p) => !q || `${p.protocol} ${p.local_address} ${p.local_port} ${p.process} ${p.pid}`.toLowerCase().includes(q));
  $: filteredServices = services.filter((s) => !q || `${s.id} ${s.name} ${s.path}`.toLowerCase().includes(q));
  $: filteredPackages = packages.filter((p) => !q || `${p.name} ${p.version} ${p.architecture}`.toLowerCase().includes(q));
  $: filteredFiles = fileEntries.filter((entry) => !q || `${entry.name} ${entry.path} ${entry.kind}`.toLowerCase().includes(q));
  $: editorDirty = selectedFile && editorContent !== editorOriginal;
  $: editorReadOnly = !!selectedFile && Number(selectedFile.size || 0) > FILE_EDITOR_WRITE_LIMIT;

  function setAction(message) {
    actionText = message || '';
    if (message) setTimeout(() => {
      if (actionText === message) actionText = '';
    }, 5000);
  }

  function errorMessage(error) {
    return error?.payload?.error || error?.message || t(locale, 'errors.controlUnavailable');
  }

  async function load(next = tab) {
    if (next === 'files') return loadFiles(filePath);
    if (next === 'terminal') return;
    if (next === 'maintenance') return loadMaintenance();
    if (next === 'network-tools') return;
    if (next === 'integrations') return loadIntegrations();
    loading = true;
    errorText = '';
    try {
      if (next === 'processes') processes = (await getModule('admin', 'processes')).processes || [];
      if (next === 'ports') ports = (await getModule('admin', 'ports')).ports || [];
      if (next === 'services') services = (await getModule('admin', 'services')).services || [];
      if (next === 'packages') packages = (await getModule('admin', 'packages')).packages || [];
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  function selectTab(next) {
    tab = next;
    search = '';
    errorText = '';
    actionText = '';
    load(next);
  }

  async function mutateProcess(p, signal) {
    if (!confirm(`${copy.processConfirm} PID ${p.pid}: ${signal}?`)) return;
    try {
      await adminProcessSignal(p.pid, signal);
      setAction(`PID ${p.pid}: ${signal} — OK`);
      await load('processes');
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function mutateService(service, action) {
    const id = service.id || service.name;
    if (!id || !confirm(`${copy.serviceConfirm} ${id}: ${action}?`)) return;
    try {
      await adminServiceAction(id, action);
      setAction(`${id}: ${action} — OK`);
      await load('services');
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  function joinPath(parent, name) {
    if (parent === '/') return `/${name}`;
    return `${parent.replace(/\/+$/, '')}/${name}`;
  }

  function parentPath(path) {
    const clean = path.replace(/\/+$/, '');
    const pos = clean.lastIndexOf('/');
    if (pos <= 0) return '/';
    return clean.slice(0, pos);
  }

  async function loadFiles(path = filePath) {
    fileLoading = true;
    errorText = '';
    try {
      const result = await getAdminFiles(path);
      filePath = result.path || path;
      fileEntries = result.entries || [];
      selectedFile = null;
      editorContent = '';
      editorOriginal = '';
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      fileLoading = false;
    }
  }

  async function openEntry(entry) {
    if (entry.kind === 'directory') {
      await loadFiles(entry.path);
      return;
    }
    if (entry.kind !== 'file') {
      errorText = copy.noPreview;
      return;
    }
    editorBusy = true;
    errorText = '';
    try {
      const result = await readAdminFile(entry.path);
      selectedFile = result;
      editorContent = result.content || '';
      editorOriginal = editorContent;
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      editorBusy = false;
    }
  }

  async function saveEditor() {
    if (!selectedFile || !editorDirty || editorReadOnly) return;
    editorBusy = true;
    errorText = '';
    try {
      const result = await adminFileWrite({
        path: selectedFile.path,
        confirm_path: selectedFile.path,
        content: editorContent,
        create: false,
        expected_size: selectedFile.size,
        expected_mtime_ns: selectedFile.mtime_ns
      });
      setAction(`${copy.save}: ${selectedFile.path} — OK`);
      const fresh = await readAdminFile(result.path || selectedFile.path);
      selectedFile = fresh;
      editorContent = fresh.content || '';
      editorOriginal = editorContent;
      await refreshFilesKeepingEditor();
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      editorBusy = false;
    }
  }

  async function refreshFilesKeepingEditor() {
    const active = selectedFile?.path;
    const result = await getAdminFiles(filePath);
    fileEntries = result.entries || [];
    if (active && selectedFile) {
      const found = fileEntries.find((entry) => entry.path === active);
      if (found) selectedFile = { ...selectedFile, size: found.size, mtime_ns: found.mtime_ns, modified_at: found.modified_at, mode: found.mode };
    }
  }

  async function createDirectory() {
    const name = prompt(copy.mkdirPrompt, '');
    if (!name || name.includes('/') || name === '.' || name === '..') return;
    const path = joinPath(filePath, name);
    try {
      await adminFileMkdir({ path, confirm_path: path });
      setAction(`${copy.mkdir}: ${path} — OK`);
      await loadFiles(filePath);
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function createFile() {
    const name = prompt(copy.filePrompt, '');
    if (!name || name.includes('/') || name === '.' || name === '..') return;
    const path = joinPath(filePath, name);
    try {
      await adminFileWrite({ path, confirm_path: path, content: '', create: true });
      setAction(`${copy.newFile}: ${path} — OK`);
      await loadFiles(filePath);
      const entry = fileEntries.find((item) => item.path === path);
      if (entry) await openEntry(entry);
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function moveEntry(entry) {
    const destination = prompt(copy.renamePrompt, entry.path);
    if (!destination || destination === entry.path) return;
    if (!confirm(`${copy.rename}: ${entry.path}\n→ ${destination}?`)) return;
    try {
      await adminFileMove({
        source: entry.path,
        destination,
        confirm_source: entry.path,
        confirm_destination: destination,
        expected_size: entry.size,
        expected_mtime_ns: entry.mtime_ns
      });
      setAction(`${copy.rename}: ${entry.path} → ${destination}`);
      await loadFiles(filePath);
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function deleteEntry(entry) {
    if (!confirm(`${copy.deleteConfirm}\n${entry.path}`)) return;
    try {
      await adminFileDelete({
        path: entry.path,
        confirm_path: entry.path,
        expected_size: entry.size,
        expected_mtime_ns: entry.mtime_ns
      });
      setAction(`${copy.remove}: ${entry.path} — OK`);
      await loadFiles(filePath);
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function chmodEntry(entry) {
    const mode = prompt(copy.chmodPrompt, '0644');
    if (!mode) return;
    if (!confirm(`${copy.chmod}: ${entry.path} → ${mode}?`)) return;
    try {
      await adminFileChmod({
        path: entry.path,
        confirm_path: entry.path,
        mode,
        expected_size: entry.size,
        expected_mtime_ns: entry.mtime_ns
      });
      setAction(`${copy.chmod}: ${entry.path} → ${mode}`);
      await loadFiles(filePath);
    } catch (error) {
      errorText = errorMessage(error);
    }
  }

  async function runTerminal() {
    const command = terminalCommand.trim();
    if (!command || terminalBusy) return;
    if (!confirm(`root@routerforge:${terminalCwd}$ ${command}`)) return;

    terminalBusy = true;
    errorText = '';
    terminalLines = [...terminalLines, { kind: 'command', text: `root@routerforge:${terminalCwd}$ ${command}` }];
    try {
      const result = await adminTerminalRun(command, terminalCwd);
      const output = result.output || '';
      if (output) terminalLines = [...terminalLines, { kind: 'output', text: output }];
      terminalLines = [...terminalLines, {
        kind: result.ok ? 'status' : 'error',
        text: `[exit ${result.exit_code}${result.timed_out ? ' · timeout' : ''}${result.output_truncated ? ' · truncated' : ''}]`
      }];
      terminalCommand = '';
    } catch (error) {
      const message = errorMessage(error);
      terminalLines = [...terminalLines, { kind: 'error', text: message }];
      errorText = message;
    } finally {
      terminalBusy = false;
    }
  }

  async function loadMaintenance() {
    maintenanceBusy = true;
    errorText = '';
    try {
      const [logs, tasks, backups, watchdogResult, snapshotResult] = await Promise.all([
        getAdminMaintenanceLogs(),
        getAdminMaintenanceTasks(),
        getAdminMaintenanceBackups(),
        getAdminWatchdogs(),
        getAdminSnapshots()
      ]);
      maintenanceLogs = logs.content || '';
      maintenanceLogSource = logs.source || '';
      maintenanceTasks = tasks.tasks || [];
      maintenanceBackups = backups.backups || [];
      watchdogs = watchdogResult.watchdogs || [];
      snapshots = snapshotResult.snapshots || [];
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      maintenanceBusy = false;
    }
  }

  async function createBackup() {
    if (!confirm(copy.backupConfirm)) return;
    maintenanceBusy = true;
    errorText = '';
    try {
      const result = await adminMaintenanceBackup();
      setAction(`${copy.backup}: ${result.path} (${bytes(result.size || 0)})`);
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      maintenanceBusy = false;
    }
  }

  async function restoreBackup(backup) {
    if (!backup?.valid || restoreBusyPath) return;
    if (!confirm(`${copy.restoreConfirm}\n\n${backup.path}`)) return;
    restoreBusyPath = backup.path;
    errorText = '';
    try {
      const result = await adminMaintenanceRestore(backup.path);
      setAction(`${copy.restore}: ${result.restored_root} — OK · safety: ${result.safety_backup}`);
      await loadMaintenance();
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      restoreBusyPath = '';
    }
  }

  async function setWatchdog(watchdog, enabled) {
    if (watchdogBusyID) return;
    const action = enabled ? copy.enable : copy.disable;
    if (!confirm(`${action} watchdog: ${watchdog.name}?`)) return;
    watchdogBusyID = watchdog.id;
    errorText = '';
    try {
      await configureAdminWatchdog(watchdog.id, enabled);
      await loadMaintenance();
      setAction(`${copy.watchdogs}: ${watchdog.name} — ${action}`);
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      watchdogBusyID = '';
    }
  }

  async function makeSnapshot() {
    if (snapshotBusy) return;
    snapshotBusy = true;
    errorText = '';
    try {
      const result = await createAdminSnapshot();
      setAction(`${copy.createSnapshot}: ${result.path} (${bytes(result.size || 0)})`);
      await loadMaintenance();
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      snapshotBusy = false;
    }
  }

  async function removeSnapshot(snapshot) {
    if (snapshotBusy) return;
    if (!confirm(`${copy.delete}: ${snapshot.path}?`)) return;
    snapshotBusy = true;
    errorText = '';
    try {
      await deleteAdminSnapshot(snapshot.path);
      setAction(`${copy.delete}: ${snapshot.path}`);
      await loadMaintenance();
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      snapshotBusy = false;
    }
  }

  async function runNetworkTool() {
    if (!networkHost.trim() || networkBusy) return;
    networkBusy = true;
    networkResult = null;
    errorText = '';
    try {
      networkResult = await adminNetworkToolRun(networkTool, networkHost.trim(), Number(networkPort || 0));
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      networkBusy = false;
    }
  }

  async function loadIntegrations() {
    integrationsBusy = true;
    errorText = '';
    try {
      const result = await getAdminIntegrations();
      integrations = result.integrations || [];
    } catch (error) {
      errorText = errorMessage(error);
    } finally {
      integrationsBusy = false;
    }
  }

  onMount(() => {
    load(tab);
    const stopPolling = startSerialPolling(() => {
      if (!document.hidden && (tab === 'processes' || tab === 'ports')) {
        return load(tab);
      }
    }, 4000);
    return stopPolling;
  });
</script>

<svelte:head><title>RouterForge — {t(locale, 'manage.pageTitle')}</title></svelte:head>

<div class="page admin-page">
  <div class="page-head">
    <div><h1>{t(locale, 'manage.pageTitle')}</h1><p>{t(locale, 'manage.subtitle')}</p></div>
    <span class="state-chip info">CONTROL / DEV</span>
  </div>

  <div class="admin-safety-banner">
    <div class="admin-safety-main"><strong>ROUTERFORGE CONTROL</strong><span>{copy.mutationLocked}</span></div>
    <div class="admin-lock-groups mono">
      <span class="admin-lock-group"><em>{t(locale, 'manage.inspect')}</em><strong>{t(locale, 'manage.enabled')}</strong></span>
      <span class="admin-lock-group"><em>{t(locale, 'manage.mutate')}</em><strong>GUARDED</strong></span>
    </div>
  </div>

  <div class="subtabs admin-tabs">
    {#each tabs as [id, label]}
      <button class:active={tab === id} onclick={() => selectTab(id)}>{label}</button>
    {/each}
  </div>

  <div class="toolbar">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder={t(locale, 'common.search')}/></div>
    <button class="button" onclick={() => load(tab)} disabled={loading || fileLoading}>↻ {t(locale, 'common.refresh')}</button>
  </div>

  {#if actionText}<div class="ui-action-ok">{actionText}</div>{/if}
  {#if errorText}<div class="ui-action-error">{errorText}</div>{/if}

  {#if tab === 'processes'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.tabs.processes')}</strong><span>{t(locale, 'manage.topRss')}</span></div><span class="state-chip info">{filteredProcesses.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>PID</th><th>{t(locale, 'manage.columns.process')}</th><th>{t(locale, 'manage.columns.user')}</th><th>{t(locale, 'manage.columns.state')}</th><th>RSS</th><th>{t(locale, 'manage.columns.command')}</th><th>{copy.actions}</th></tr></thead><tbody>
        {#each filteredProcesses as p (p.pid)}
          <tr>
            <td class="mono">{p.pid}</td><td><strong>{p.name}</strong></td><td>{p.user}</td><td><span class="pill">{p.state || '—'}</span></td><td class="mono">{bytes(Number(p.rss_kb || 0) * 1024)}</td><td class="mono admin-command">{p.command}</td>
            <td class="actions-cell">
              <button class="mini" onclick={() => mutateProcess(p, 'TERM')}>TERM</button>
              <button class="mini" onclick={() => mutateProcess(p, 'HUP')}>HUP</button>
              <button class="mini danger" onclick={() => mutateProcess(p, 'KILL')}>KILL</button>
            </td>
          </tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'ports'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.listeningSockets')}</strong><span>/proc/net/tcp* + udp*</span></div><span class="state-chip info">{filteredPorts.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>{t(locale, 'manage.columns.proto')}</th><th>{t(locale, 'manage.columns.local')}</th><th>{t(locale, 'manage.columns.state')}</th><th>PID</th><th>{t(locale, 'manage.columns.process')}</th></tr></thead><tbody>
        {#each filteredPorts as p (`${p.protocol}-${p.local_address}-${p.local_port}-${p.inode}`)}
          <tr><td><span class="pill accent">{p.protocol}</span></td><td class="mono">{p.local_address}:{p.local_port}</td><td>{p.state}</td><td class="mono">{p.pid || '—'}</td><td>{p.process || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'services'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.services')}</strong><span>{t(locale, 'manage.servicesHint')}</span></div><span class="state-chip info">{filteredServices.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>{t(locale, 'manage.columns.service')}</th><th>{t(locale, 'manage.columns.state')}</th><th>{t(locale, 'manage.columns.initScript')}</th><th>{copy.actions}</th></tr></thead><tbody>
        {#each filteredServices as s (s.path)}
          <tr>
            <td><strong>{s.name}</strong><div class="cell-sub mono">{s.id || ''}</div></td>
            <td><span class="state-chip {s.running ? 'good' : 'neutral'}">{s.running ? t(locale, 'common.running').toUpperCase() : t(locale, 'common.notDetected').toUpperCase()}</span></td>
            <td class="mono">{s.path}</td>
            <td class="actions-cell">
              <button class="mini" disabled={!s.executable} onclick={() => mutateService(s, 'start')}>START</button>
              <button class="mini" disabled={!s.executable} onclick={() => mutateService(s, 'restart')}>RESTART</button>
              <button class="mini danger" disabled={!s.executable} onclick={() => mutateService(s, 'stop')}>STOP</button>
            </td>
          </tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'packages'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.packages')}</strong><span>/opt/lib/opkg/status</span></div><span class="state-chip info">{filteredPackages.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>{t(locale, 'manage.columns.package')}</th><th>{t(locale, 'manage.columns.version')}</th><th>{t(locale, 'manage.columns.architecture')}</th><th>{t(locale, 'manage.columns.status')}</th></tr></thead><tbody>
        {#each filteredPackages as p (p.name)}
          <tr><td><strong>{p.name}</strong></td><td class="mono">{p.version || '—'}</td><td>{p.architecture || '—'}</td><td>{p.status || 'installed'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'files'}
    <section class="panel files-panel">
      <div class="file-toolbar">
        <button class="button" onclick={() => loadFiles(parentPath(filePath))} disabled={filePath === '/opt' || filePath === '/tmp'}>↑ {copy.up}</button>
        <input class="path-input mono" bind:value={filePath} onkeydown={(event) => event.key === 'Enter' && loadFiles(filePath)} aria-label={copy.path}/>
        <button class="button" onclick={() => loadFiles(filePath)}>↻</button>
        <button class="button" onclick={createDirectory}>＋ {copy.mkdir}</button>
        <button class="button" onclick={createFile}>＋ {copy.newFile}</button>
      </div>

      {#if fileLoading}
        <div class="empty">{t(locale, 'manage.loading')}</div>
      {:else}
        <div class="table-scroll"><table><thead><tr><th>{copy.files}</th><th>Type</th><th>Size</th><th>Mode</th><th>Modified</th><th>{copy.actions}</th></tr></thead><tbody>
          {#each filteredFiles as entry (entry.path)}
            <tr>
              <td><button class="file-name" onclick={() => openEntry(entry)}>{entry.kind === 'directory' ? '📁' : entry.kind === 'file' ? '📄' : '↗'} {entry.name}</button></td>
              <td>{entry.kind}</td><td class="mono">{entry.kind === 'file' ? bytes(entry.size || 0) : '—'}</td><td class="mono">{entry.mode}</td><td class="mono">{new Date(entry.modified_at).toLocaleString()}</td>
              <td class="actions-cell">
                {#if entry.kind === 'file'}<a class="mini link" href={adminFileDownloadURL(entry.path)}>{copy.download}</a>{/if}
                <button class="mini" onclick={() => moveEntry(entry)}>{copy.rename}</button>
                <button class="mini" onclick={() => chmodEntry(entry)}>{copy.chmod}</button>
                <button class="mini danger" onclick={() => deleteEntry(entry)}>{copy.remove}</button>
              </td>
            </tr>
          {/each}
          {#if filteredFiles.length === 0}<tr><td colspan="6" class="empty">{copy.empty}</td></tr>{/if}
        </tbody></table></div>
      {/if}
    </section>

    {#if selectedFile}
      <section class="panel editor-panel">
        <div class="panel-head">
          <div><strong class="mono">{selectedFile.path}</strong><span>{bytes(selectedFile.size || 0)} · {selectedFile.mode} {editorReadOnly ? `· ${copy.editorReadOnly}` : (editorDirty ? `· ${copy.dirty}` : '')}</span></div>
          <div class="actions-cell">
            <button class="button" onclick={saveEditor} disabled={!editorDirty || editorBusy || editorReadOnly}>{copy.save}</button>
            <button class="button" onclick={() => { selectedFile = null; editorContent = ''; editorOriginal = ''; }}>{copy.close}</button>
          </div>
        </div>
        <textarea class="file-editor mono" bind:value={editorContent} spellcheck="false" readonly={editorReadOnly}></textarea>
      </section>
    {/if}
  {:else if tab === 'terminal'}
    <section class="panel terminal-panel">
      <div class="panel-head">
        <div><strong>{copy.terminal}</strong><span>{copy.terminalHint}</span></div>
        <button class="button" onclick={() => { terminalLines = []; }}>{copy.clear}</button>
      </div>
      <div class="terminal-output mono">
        {#if terminalLines.length === 0}
          <div class="terminal-muted">RouterForge Terminal BASE · /bin/sh</div>
        {/if}
        {#each terminalLines as line}
          <pre class:terminal-command={line.kind === 'command'} class:terminal-error={line.kind === 'error'}>{line.text}</pre>
        {/each}
      </div>
      <div class="terminal-controls">
        <input class="path-input mono terminal-cwd" bind:value={terminalCwd} aria-label="cwd"/>
        <input class="path-input mono terminal-input" bind:value={terminalCommand} onkeydown={(event) => event.key === 'Enter' && runTerminal()} placeholder="command" aria-label="command"/>
        <button class="button" onclick={runTerminal} disabled={terminalBusy || !terminalCommand.trim()}>{terminalBusy ? '…' : copy.run}</button>
      </div>
    </section>
  {:else if tab === 'maintenance'}
    <section class="panel">
      <div class="panel-head">
        <div><strong>{copy.maintenance}</strong><span>Logs · Cron · Backup BASE</span></div>
        <div class="actions-cell">
          <button class="button" onclick={loadMaintenance} disabled={maintenanceBusy}>↻ {t(locale, 'common.refresh')}</button>
          <button class="button" onclick={createBackup} disabled={maintenanceBusy}>{copy.backup}</button>
        </div>
      </div>
      <div class="maintenance-grid">
        <div class="maintenance-card">
          <strong>{copy.logs}</strong>
          <span class="cell-sub mono">{maintenanceLogSource || 'not detected'}</span>
          <pre class="maintenance-pre mono">{maintenanceLogs || copy.empty}</pre>
        </div>
        <div class="maintenance-card">
          <strong>{copy.tasks}</strong>
          <div class="maintenance-task-list mono">
            {#each maintenanceTasks as task}
              <div><span>{task.source}</span><pre>{task.line}</pre></div>
            {/each}
            {#if maintenanceTasks.length === 0}<div>{copy.empty}</div>{/if}
          </div>
        </div>
      </div>
      <div class="maintenance-backups">
        <div class="maintenance-backup-head">
          <div><strong>{copy.backups}</strong><span>{copy.restoreMode}</span></div>
        </div>
        {#if maintenanceBackups.length === 0}
          <div class="maintenance-backup-empty">{copy.empty}</div>
        {:else}
          {#each maintenanceBackups as backup}
            <div class="maintenance-backup-row">
              <div>
                <strong class="mono">{backup.name}</strong>
                <span class="cell-sub mono">
                  {bytes(backup.size || 0)} · config {bytes(backup.config_bytes || 0)} · {backup.config_entries || 0} entries
                  {backup.ignored_entries ? ` · ignored ${backup.ignored_entries}` : ''}
                  {backup.error ? ` · ${backup.error}` : ''}
                </span>
              </div>
              <div class="actions-cell">
                <span class:state-running={backup.valid} class:state-stopped={!backup.valid}>{backup.valid ? 'VALID' : 'INVALID'}</span>
                <button class="button" onclick={() => restoreBackup(backup)} disabled={!backup.valid || !!restoreBusyPath}>
                  {restoreBusyPath === backup.path ? '…' : copy.restore}
                </button>
              </div>
            </div>
          {/each}
        {/if}
      </div>

      <div class="maintenance-extra">
        <div class="maintenance-extra-head">
          <div><strong>{copy.watchdogs}</strong><span>{copy.watchdogHint}</span></div>
        </div>
        <div class="watchdog-grid">
          {#each watchdogs as watchdog}
            <div class="watchdog-card">
              <div>
                <strong>{watchdog.name}</strong>
                <span class="cell-sub mono">
                  {watchdog.detected ? (watchdog.running ? copy.running : copy.stopped) : copy.notDetected}
                  · {copy.attempts}: {watchdog.attempts_last_hour || 0}
                  {watchdog.service_id ? ` · ${watchdog.service_id}` : ''}
                </span>
              </div>
              <button
                class="button"
                onclick={() => setWatchdog(watchdog, !watchdog.enabled)}
                disabled={!!watchdogBusyID || (!watchdog.auto_start_available && !watchdog.enabled)}
              >
                {watchdogBusyID === watchdog.id ? '…' : (watchdog.enabled ? copy.disable : copy.enable)}
              </button>
            </div>
          {/each}
        </div>
      </div>

      <div class="maintenance-extra">
        <div class="maintenance-extra-head">
          <div><strong>{copy.snapshots}</strong><span>{copy.snapshotHint}</span></div>
          <button class="button" onclick={makeSnapshot} disabled={snapshotBusy}>
            {snapshotBusy ? '…' : copy.createSnapshot}
          </button>
        </div>
        {#if snapshots.length === 0}
          <div class="maintenance-backup-empty">{copy.empty}</div>
        {:else}
          {#each snapshots as snapshot}
            <div class="maintenance-backup-row">
              <div>
                <strong class="mono">{snapshot.name}</strong>
                <span class="cell-sub mono">{bytes(snapshot.size || 0)} · {snapshot.modified_at}</span>
              </div>
              <button class="button danger" onclick={() => removeSnapshot(snapshot)} disabled={snapshotBusy}>{copy.delete}</button>
            </div>
          {/each}
        {/if}
      </div>
    </section>
  {:else if tab === 'network-tools'}
    <section class="panel">
      <div class="panel-head"><div><strong>{copy.networkTools}</strong><span>{copy.networkHint}</span></div></div>
      <div class="network-controls">
        <select class="path-input" bind:value={networkTool}>
          <option value="ping">Ping</option>
          <option value="traceroute">Traceroute</option>
          <option value="dns">DNS lookup</option>
          <option value="tcp">TCP connect</option>
        </select>
        <input class="path-input mono" bind:value={networkHost} placeholder="host"/>
        {#if networkTool === 'tcp'}<input class="path-input mono network-port" type="number" min="1" max="65535" bind:value={networkPort}/>{/if}
        <button class="button" onclick={runNetworkTool} disabled={networkBusy || !networkHost.trim()}>{networkBusy ? '…' : copy.run}</button>
      </div>
      {#if networkResult}
        <div class="network-result mono">
          <pre>{JSON.stringify(networkResult, null, 2)}</pre>
        </div>
      {/if}
    </section>
  {:else if tab === 'integrations'}
    <section class="panel">
      <div class="panel-head">
        <div><strong>{copy.integrations}</strong><span>{copy.integrationsHint}</span></div>
        <button class="button" onclick={loadIntegrations} disabled={integrationsBusy}>↻ {t(locale, 'common.refresh')}</button>
      </div>
      <div class="integration-grid">
        {#each integrations as integration}
          <article class="integration-card">
            <div class="integration-title">
              <strong>{integration.name}</strong>
              <span class:state-running={integration.running} class:state-stopped={!integration.running}>
                {integration.detected ? (integration.running ? copy.running : copy.stopped) : copy.notDetected}
              </span>
            </div>
            <dl>
              <dt>{copy.detected}</dt><dd>{integration.detected ? 'YES' : 'NO'}</dd>
              <dt>{copy.service}</dt><dd class="mono">{integration.service_id || '—'}</dd>
              <dt>{copy.processesLabel}</dt><dd class="mono">{(integration.process_pids || []).join(', ') || '—'}</dd>
              <dt>{copy.portsLabel}</dt><dd class="mono">{(integration.ports || []).join(', ') || '—'}</dd>
              <dt>{copy.pathsLabel}</dt>
              <dd class="mono integration-paths">
                {#each integration.paths || [] as path}<div>{path}</div>{/each}
                {#if !(integration.paths || []).length}—{/if}
              </dd>
            </dl>
          </article>
        {/each}
      </div>
    </section>
  {/if}
</div>

<style>
  .ui-action-ok,.ui-action-error{margin:.75rem 0;padding:.7rem .9rem;border-radius:.65rem;font-weight:600}
  .ui-action-ok{background:rgba(48,190,120,.12);border:1px solid rgba(48,190,120,.35)}
  .ui-action-error{background:rgba(230,75,75,.12);border:1px solid rgba(230,75,75,.35)}
  .actions-cell{display:flex;gap:.35rem;flex-wrap:wrap;align-items:center}
  .mini{font:inherit;font-size:.78rem;padding:.32rem .5rem;border:1px solid var(--border-color,rgba(127,127,127,.3));border-radius:.45rem;background:transparent;color:inherit;cursor:pointer}
  .mini:hover{background:rgba(127,127,127,.12)}
  .mini.danger{border-color:rgba(230,75,75,.45)}
  .mini:disabled{opacity:.4;cursor:not-allowed}
  .mini.link{text-decoration:none;display:inline-block}
  .file-toolbar{display:flex;gap:.5rem;align-items:center;flex-wrap:wrap;padding:1rem}
  .path-input{flex:1;min-width:18rem;padding:.55rem .7rem;border-radius:.5rem;border:1px solid var(--border-color,rgba(127,127,127,.3));background:rgba(0,0,0,.08);color:inherit}
  .file-name{border:0;background:none;color:inherit;font:inherit;font-weight:600;cursor:pointer;text-align:left;padding:0}
  .file-name:hover{text-decoration:underline}
  .editor-panel{margin-top:1rem}
  .file-editor{width:100%;min-height:26rem;resize:vertical;box-sizing:border-box;border:0;border-top:1px solid var(--border-color,rgba(127,127,127,.25));background:rgba(0,0,0,.1);color:inherit;padding:1rem;line-height:1.45;tab-size:2}
  .terminal-panel{overflow:hidden}
  .terminal-output{min-height:28rem;max-height:55vh;overflow:auto;padding:1rem;background:#0b0d10;color:#d7e1ea}
  .terminal-output pre{margin:0 0 .55rem;white-space:pre-wrap;word-break:break-word;font:inherit}
  .terminal-command{color:#8fd3ff}
  .terminal-error{color:#ff8f8f}
  .terminal-muted{opacity:.55}
  .terminal-controls{display:flex;gap:.5rem;padding:1rem;border-top:1px solid var(--border-color,rgba(127,127,127,.25))}
  .terminal-cwd{flex:0 0 12rem;min-width:8rem}
  .terminal-input{flex:1}
  .maintenance-grid{display:grid;grid-template-columns:1fr 1fr;gap:1rem;padding:1rem}
  .maintenance-card{min-width:0}
  .maintenance-card>strong{display:block;margin-bottom:.5rem}
  .maintenance-pre,.maintenance-task-list{min-height:22rem;max-height:45vh;overflow:auto;background:#0b0d10;color:#d7e1ea;padding:.8rem;border-radius:.55rem;white-space:pre-wrap;word-break:break-word}
  .maintenance-task-list pre{white-space:pre-wrap;margin:.2rem 0 .75rem}
  .maintenance-task-list span{opacity:.6}
  .network-controls{display:flex;gap:.5rem;padding:1rem;flex-wrap:wrap}
  .network-controls select{flex:0 0 10rem}
  .network-controls input{flex:1}
  .network-port{max-width:8rem}
  .network-result{margin:0 1rem 1rem;background:#0b0d10;color:#d7e1ea;padding:1rem;border-radius:.55rem;min-height:20rem}
  .network-result pre{white-space:pre-wrap;word-break:break-word;margin:0}
  @media(max-width:900px){.maintenance-grid{grid-template-columns:1fr}}
  .integration-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1rem;padding:1rem}
  .integration-card{border:1px solid var(--border-color,rgba(127,127,127,.25));border-radius:.7rem;padding:1rem;min-width:0}
  .integration-title{display:flex;align-items:center;justify-content:space-between;gap:.75rem;margin-bottom:1rem}
  .integration-title span{font-size:.8rem;font-weight:700}
  .state-running{color:#42c77a}
  .state-stopped{opacity:.6}
  .integration-card dl{display:grid;grid-template-columns:6rem 1fr;gap:.45rem .7rem;margin:0}
  .integration-card dt{opacity:.6}
  .integration-card dd{margin:0;min-width:0;word-break:break-word}
  .integration-paths div{margin-bottom:.2rem}
  @media(max-width:1000px){.integration-grid{grid-template-columns:1fr}}
  .maintenance-backups{margin:0 1rem 1rem;border:1px solid var(--border-color,rgba(127,127,127,.25));border-radius:.65rem;overflow:hidden}
  .maintenance-backup-head,.maintenance-backup-row{display:flex;justify-content:space-between;gap:1rem;align-items:center;padding:.8rem 1rem;border-bottom:1px solid var(--border-color,rgba(127,127,127,.18))}
  .maintenance-backup-head>div,.maintenance-backup-row>div:first-child{min-width:0;display:flex;flex-direction:column;gap:.2rem}
  .maintenance-backup-head span{opacity:.65}
  .maintenance-backup-row:last-child{border-bottom:0}
  .maintenance-backup-empty{padding:1rem;opacity:.6}
  .maintenance-extra{margin:0 1rem 1rem;border:1px solid var(--border-color,rgba(127,127,127,.25));border-radius:.65rem;overflow:hidden}
  .maintenance-extra-head{display:flex;justify-content:space-between;align-items:center;gap:1rem;padding:.8rem 1rem;border-bottom:1px solid var(--border-color,rgba(127,127,127,.18))}
  .maintenance-extra-head>div{display:flex;flex-direction:column;gap:.2rem}
  .maintenance-extra-head span{opacity:.65}
  .watchdog-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.75rem;padding:1rem}
  .watchdog-card{display:flex;justify-content:space-between;align-items:center;gap:.75rem;border:1px solid var(--border-color,rgba(127,127,127,.18));border-radius:.55rem;padding:.75rem;min-width:0}
  .watchdog-card>div{display:flex;flex-direction:column;gap:.2rem;min-width:0}
  @media(max-width:1000px){.watchdog-grid{grid-template-columns:1fr}}
</style>
