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
  import TerminalPane from './TerminalPane.svelte';
  import FileManagerPane from './FileManagerPane.svelte';

  const FILE_EDITOR_WRITE_LIMIT = 128 * 1024;

  let tab = 'processes';
  let processes = [];
  let ports = [];
  let services = [];
  let packages = [];
  let search = '';
  let loading = false;
  let packageLoading = false;
  let loadEpoch = 0;
  let packageRenderLimit = 160;
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
  $: matchingPackages = packages.filter((p) => !q || `${p.name} ${p.version} ${p.architecture}`.toLowerCase().includes(q));
  $: filteredPackages = matchingPackages.slice(0, packageRenderLimit);
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
    const epoch = ++loadEpoch;
    if (next === 'files') return;
    if (next === 'terminal') return;
    if (next === 'maintenance') return loadMaintenance();
    if (next === 'network-tools') return;
    if (next === 'integrations') return loadIntegrations();

    if (next === 'packages') {
      packageLoading = true;
      packageRenderLimit = 160;
    } else {
      loading = true;
    }
    errorText = '';
    try {
      if (next === 'processes') {
        const result = await getModule('admin', 'processes');
        if (epoch === loadEpoch && tab === next) processes = result.processes || [];
      }
      if (next === 'ports') {
        const result = await getModule('admin', 'ports');
        if (epoch === loadEpoch && tab === next) ports = result.ports || [];
      }
      if (next === 'services') {
        const result = await getModule('admin', 'services');
        if (epoch === loadEpoch && tab === next) services = result.services || [];
      }
      if (next === 'packages') {
        const result = await getModule('admin', 'packages');
        if (epoch === loadEpoch && tab === next) packages = result.packages || [];
      }
    } catch (error) {
      if (epoch === loadEpoch && tab === next) errorText = errorMessage(error);
    } finally {
      if (epoch === loadEpoch) {
        loading = false;
        packageLoading = false;
      }
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
      await loadMaintenance();
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

  {#if tab !== 'files' && tab !== 'terminal' && tab !== 'maintenance'}
    <div class="toolbar">
      <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder={t(locale, 'common.search')}/></div>
      <button class="button" onclick={() => load(tab)} disabled={loading || fileLoading}>↻ {t(locale, 'common.refresh')}</button>
    </div>
  {/if}

  {#if actionText}<div class="ui-action-ok">{actionText}</div>{/if}
  {#if errorText}<div class="ui-action-error">{errorText}</div>{/if}

  {#if tab === 'processes'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.tabs.processes')}</strong><span>{t(locale, 'manage.topRss')}</span></div><span class="state-chip info">{filteredProcesses.length}</span></div>
      <div class="table-scroll"><table class="process-table"><thead><tr><th>{t(locale, 'manage.columns.process')}</th><th>PID</th><th>{t(locale, 'manage.columns.user')}</th><th>{t(locale, 'manage.columns.state')}</th><th>RSS</th><th>{copy.actions}</th></tr></thead><tbody>
        {#each filteredProcesses as p (p.pid)}
          <tr>
            <td class="process-primary">
              <strong>{p.name}</strong>
              <div class="cell-sub mono process-command" title={p.command}>{p.command || p.name}</div>
            </td>
            <td class="mono">{p.pid}</td>
            <td>{p.user}</td>
            <td><span class="pill">{p.state || '—'}</span></td>
            <td class="mono">{bytes(Number(p.rss_kb || 0) * 1024)}</td>
            <td class="actions-cell process-actions">
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
      {#if packageLoading}
        <div class="empty">{t(locale, 'manage.loading')}</div>
      {:else}
        <div class="table-scroll"><table><thead><tr><th>{t(locale, 'manage.columns.package')}</th><th>{t(locale, 'manage.columns.version')}</th><th>{t(locale, 'manage.columns.architecture')}</th><th>{t(locale, 'manage.columns.status')}</th></tr></thead><tbody>
          {#each filteredPackages as p}
            <tr><td><strong>{p.name}</strong></td><td class="mono">{p.version || '—'}</td><td>{p.architecture || '—'}</td><td>{p.status || 'installed'}</td></tr>
          {/each}
        </tbody></table></div>
        {#if matchingPackages.length > filteredPackages.length}
          <div class="package-more">
            <span>{filteredPackages.length} / {matchingPackages.length}</span>
            <button class="button" onclick={() => { packageRenderLimit += 160; }}>＋160</button>
          </div>
        {/if}
      {/if}
    </section>
  {:else if tab === 'files'}
    <FileManagerPane locale={locale} />
  {:else if tab === 'terminal'}
    <TerminalPane locale={locale} />
  {:else if tab === 'maintenance'}
    <section class="maintenance-shell">
      <div class="maintenance-commandbar">
        <div class="maintenance-command-copy">
          <span class="maintenance-kicker mono">ROUTERFORGE / SYSTEM CARE</span>
          <strong>{copy.maintenance}</strong>
          <span>{copy.restoreMode}</span>
        </div>
        <div class="maintenance-actions">
          {#if maintenanceBusy}<span class="maintenance-busy mono">WORKING…</span>{/if}
          <button class="button" onclick={loadMaintenance} disabled={maintenanceBusy}>↻ {t(locale, 'common.refresh')}</button>
          <button class="button primary" onclick={createBackup} disabled={maintenanceBusy}>{copy.backup}</button>
        </div>
      </div>

      <div class="maintenance-summary">
        <div class="maintenance-stat"><span>{copy.logs}</span><strong>{maintenanceLogSource ? 'READY' : '—'}</strong><small class="mono" title={maintenanceLogSource}>{maintenanceLogSource || 'not detected'}</small></div>
        <div class="maintenance-stat"><span>{copy.tasks}</span><strong>{maintenanceTasks.length}</strong><small>cron entries</small></div>
        <div class="maintenance-stat"><span>{copy.backups}</span><strong>{maintenanceBackups.length}</strong><small>/tmp/routerforge-backups</small></div>
        <div class="maintenance-stat"><span>{copy.watchdogs}</span><strong>{watchdogs.filter((item) => item.enabled).length}</strong><small>{watchdogs.length} detected/configured</small></div>
        <div class="maintenance-stat"><span>{copy.snapshots}</span><strong>{snapshots.length}</strong><small>/tmp · max 32</small></div>
      </div>

      <div class="maintenance-workbench">
        <section class="maintenance-section maintenance-log-section">
          <div class="maintenance-section-head">
            <div><strong>{copy.logs}</strong><span class="mono">{maintenanceLogSource || 'not detected'}</span></div>
            <span class="state-chip neutral">LOG</span>
          </div>
          <pre class="maintenance-pre mono">{maintenanceLogs || copy.empty}</pre>
        </section>

        <section class="maintenance-section maintenance-task-section">
          <div class="maintenance-section-head">
            <div><strong>{copy.tasks}</strong><span>Configured cron entries</span></div>
            <span class="state-chip info">{maintenanceTasks.length}</span>
          </div>
          <div class="maintenance-task-list mono">
            {#each maintenanceTasks as task}
              <div class="maintenance-task-row"><span>{task.source}</span><pre>{task.line}</pre></div>
            {/each}
            {#if maintenanceTasks.length === 0}<div class="maintenance-empty">{copy.empty}</div>{/if}
          </div>
        </section>
      </div>

      <section class="maintenance-section maintenance-wide-section">
        <div class="maintenance-section-head">
          <div><strong>{copy.backups}</strong><span>{copy.restoreMode}</span></div>
          <span class="state-chip info">{maintenanceBackups.length}</span>
        </div>
        {#if maintenanceBackups.length === 0}
          <div class="maintenance-empty">{copy.empty}</div>
        {:else}
          <div class="maintenance-list">
            {#each maintenanceBackups as backup}
              <div class="maintenance-list-row">
                <div class="maintenance-list-copy">
                  <strong class="mono">{backup.name}</strong>
                  <span class="cell-sub mono">
                    {bytes(backup.size || 0)} · config {bytes(backup.config_bytes || 0)} · {backup.config_entries || 0} entries
                    {backup.ignored_entries ? ` · ignored ${backup.ignored_entries}` : ''}
                    {backup.error ? ` · ${backup.error}` : ''}
                  </span>
                </div>
                <div class="maintenance-row-actions">
                  <span class="state-chip {backup.valid ? 'good' : 'error'}">{backup.valid ? 'VALID' : 'INVALID'}</span>
                  <button class="button" onclick={() => restoreBackup(backup)} disabled={!backup.valid || !!restoreBusyPath}>
                    {restoreBusyPath === backup.path ? '…' : copy.restore}
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </section>

      <div class="maintenance-secondary-grid">
        <section class="maintenance-section">
          <div class="maintenance-section-head">
            <div><strong>{copy.watchdogs}</strong><span>{copy.watchdogHint}</span></div>
            <span class="state-chip info">{watchdogs.length}</span>
          </div>
          {#if watchdogs.length === 0}
            <div class="maintenance-empty">{copy.empty}</div>
          {:else}
            <div class="maintenance-list compact-list">
              {#each watchdogs as watchdog}
                <div class="watchdog-row">
                  <div class="maintenance-list-copy">
                    <strong>{watchdog.name}</strong>
                    <span class="cell-sub mono">
                      {watchdog.detected ? (watchdog.running ? copy.running : copy.stopped) : copy.notDetected}
                      · {copy.attempts}: {watchdog.attempts_last_hour || 0}
                      {watchdog.service_id ? ` · ${watchdog.service_id}` : ''}
                    </span>
                  </div>
                  <div class="maintenance-row-actions">
                    <span class="state-chip {watchdog.running ? 'good' : 'neutral'}">{watchdog.running ? 'RUN' : 'IDLE'}</span>
                    <button class="button" onclick={() => setWatchdog(watchdog, !watchdog.enabled)} disabled={!!watchdogBusyID || (!watchdog.auto_start_available && !watchdog.enabled)}>
                      {watchdogBusyID === watchdog.id ? '…' : (watchdog.enabled ? copy.disable : copy.enable)}
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </section>

        <section class="maintenance-section">
          <div class="maintenance-section-head">
            <div><strong>{copy.snapshots}</strong><span>{copy.snapshotHint}</span></div>
            <button class="button" onclick={makeSnapshot} disabled={snapshotBusy}>{snapshotBusy ? '…' : copy.createSnapshot}</button>
          </div>
          {#if snapshots.length === 0}
            <div class="maintenance-empty">{copy.empty}</div>
          {:else}
            <div class="maintenance-list compact-list">
              {#each snapshots as snapshot}
                <div class="maintenance-list-row">
                  <div class="maintenance-list-copy">
                    <strong class="mono">{snapshot.name}</strong>
                    <span class="cell-sub mono">{bytes(snapshot.size || 0)} · {snapshot.modified_at}</span>
                  </div>
                  <button class="button danger" onclick={() => removeSnapshot(snapshot)} disabled={snapshotBusy}>{copy.delete}</button>
                </div>
              {/each}
            </div>
          {/if}
        </section>
      </div>
    </section>  {:else if tab === 'network-tools'}
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
  .ui-action-ok,.ui-action-error{margin:.75rem 0;padding:.7rem .9rem;border-radius:var(--rf-radius-control,.65rem);font-weight:600}
  .ui-action-ok{color:var(--good,#2ea043);background:color-mix(in srgb,var(--good,#2ea043) 10%,var(--rf-surface,#12151a));border:1px solid color-mix(in srgb,var(--good,#2ea043) 36%,transparent)}
  .ui-action-error{color:var(--bad,#f85149);background:color-mix(in srgb,var(--bad,#f85149) 10%,var(--rf-surface,#12151a));border:1px solid color-mix(in srgb,var(--bad,#f85149) 36%,transparent)}
  .actions-cell{display:flex;gap:.35rem;flex-wrap:wrap;align-items:center}td.actions-cell{display:table-cell;white-space:nowrap;vertical-align:middle}td.actions-cell>.mini,td.actions-cell>.link{margin:.15rem .3rem .15rem 0}td.actions-cell>:last-child{margin-right:0}
  .mini{font:inherit;font-size:.78rem;padding:.32rem .5rem;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-control,.45rem);background:var(--rf-surface-2,#171b21);color:var(--rf-text,#f5f7fa);cursor:pointer}.mini:hover{background:var(--rf-hover,#1d2229);border-color:var(--rf-accent-border,rgba(56,189,248,.30))}.mini.danger{border-color:rgba(248,81,73,.42);color:var(--bad,#f85149)}.mini:disabled{opacity:.4;cursor:not-allowed}.mini.link{text-decoration:none;display:inline-block}
  .process-table{table-layout:fixed}.process-primary{width:48%}.process-command{max-width:46rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.process-actions{white-space:nowrap}.process-table tbody td{border-bottom:1px solid var(--rf-border,#29313a)}
  .package-more{display:flex;justify-content:flex-end;align-items:center;gap:.75rem;padding:.7rem 1rem;border-top:1px solid var(--rf-border,#29313a);font-size:.82rem;color:var(--rf-muted,#8d98a4)}
  .file-toolbar{display:flex;gap:.5rem;align-items:center;flex-wrap:wrap;padding:1rem}.path-input{flex:1;min-width:18rem;padding:.55rem .7rem;border-radius:var(--rf-radius-control,.5rem);border:1px solid var(--rf-border,#29313a);background:var(--rf-surface-2,#171b21);color:var(--rf-text,#f5f7fa)}.file-name{border:0;background:none;color:inherit;font:inherit;font-weight:600;cursor:pointer;text-align:left;padding:0}.file-name:hover{text-decoration:underline}
  .terminal-panel{overflow:hidden}.terminal-output{min-height:28rem;max-height:55vh;overflow:auto;padding:1rem;background:var(--rf-bg,#0b0d10);color:var(--rf-text,#f5f7fa)}.terminal-output pre{margin:0 0 .55rem;white-space:pre-wrap;word-break:break-word;font:inherit}.terminal-command{color:var(--rf-accent,#38bdf8)}.terminal-error{color:var(--bad,#f85149)}.terminal-muted{opacity:.55}.terminal-controls{display:flex;gap:.5rem;padding:1rem;border-top:1px solid var(--rf-border,#29313a)}.terminal-cwd{flex:0 0 12rem;min-width:8rem}.terminal-input{flex:1}
  .network-controls{display:flex;gap:.5rem;padding:1rem;flex-wrap:wrap}.network-controls select{flex:0 0 10rem}.network-controls input{flex:1}.network-port{max-width:8rem}.network-result{margin:0 1rem 1rem;background:var(--rf-bg,#0b0d10);color:var(--rf-text,#f5f7fa);padding:1rem;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-control,.55rem);min-height:20rem}.network-result pre{white-space:pre-wrap;word-break:break-word;margin:0}
  .integration-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1rem;padding:1rem}.integration-card{border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-card,.7rem);padding:1rem;min-width:0;background:var(--rf-surface-2,#171b21)}.integration-title{display:flex;align-items:center;justify-content:space-between;gap:.75rem;margin-bottom:1rem}.integration-title span{font-size:.8rem;font-weight:700}.state-running{color:var(--good,#2ea043)}.state-stopped{color:var(--rf-muted,#8d98a4)}.integration-card dl{display:grid;grid-template-columns:6rem 1fr;gap:.45rem .7rem;margin:0}.integration-card dt{color:var(--rf-muted,#8d98a4)}.integration-card dd{margin:0;min-width:0;word-break:break-word}.integration-paths div{margin-bottom:.2rem}

  .maintenance-shell{display:grid;gap:14px;min-width:0}
  .maintenance-commandbar{min-height:72px;padding:12px 14px;display:flex;align-items:center;justify-content:space-between;gap:18px;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-panel,6px);background:var(--rf-surface,#12151a)}
  .maintenance-command-copy{min-width:0;display:grid;gap:3px}.maintenance-command-copy>strong{color:var(--rf-text,#f5f7fa);font-size:var(--ui-panel-title,14px)}.maintenance-command-copy>span:last-child{max-width:70rem;color:var(--rf-muted,#8d98a4);font-size:var(--ui-xs,12px);line-height:1.4}.maintenance-kicker{color:var(--rf-accent,#38bdf8);font-size:var(--ui-micro,11px);font-weight:700;letter-spacing:.08em}.maintenance-actions{display:flex;align-items:center;justify-content:flex-end;gap:8px;flex-wrap:wrap}.maintenance-busy{color:var(--warn,#d29922);font-size:var(--ui-micro,11px);letter-spacing:.06em}
  .maintenance-summary{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.maintenance-stat{min-width:0;padding:11px 12px;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-panel,6px);background:var(--rf-surface,#12151a);display:grid;gap:4px}.maintenance-stat>span{color:var(--rf-muted,#8d98a4);font-size:var(--ui-micro,11px);text-transform:uppercase;letter-spacing:.045em}.maintenance-stat>strong{color:var(--rf-text,#f5f7fa);font:650 18px/1.1 var(--font-mono,"Roboto Mono",monospace)}.maintenance-stat>small{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-muted,#8d98a4);font-size:10px}
  .maintenance-workbench{display:grid;grid-template-columns:minmax(0,1.45fr) minmax(320px,.75fr);gap:14px}.maintenance-secondary-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:14px}
  .maintenance-section{min-width:0;overflow:hidden;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-panel,6px);background:var(--rf-surface,#12151a)}.maintenance-section-head{min-height:50px;padding:9px 12px;display:flex;align-items:center;justify-content:space-between;gap:12px;border-bottom:1px solid var(--rf-border,#29313a);background:color-mix(in srgb,var(--rf-surface,#12151a) 76%,var(--rf-bg,#0b0d10))}.maintenance-section-head>div{min-width:0}.maintenance-section-head strong{display:block;color:var(--rf-text,#f5f7fa);font-size:var(--ui-panel-title,14px)}.maintenance-section-head div>span{display:block;margin-top:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-muted,#8d98a4);font-size:var(--ui-micro,11px)}
  .maintenance-pre{box-sizing:border-box;width:100%;height:clamp(18rem,34vh,27rem);margin:0;padding:12px 14px;overflow:auto;border:0;background:var(--rf-bg,#0b0d10);color:color-mix(in srgb,var(--rf-text,#f5f7fa) 88%,var(--rf-muted,#8d98a4));white-space:pre-wrap;word-break:break-word;line-height:1.52;scrollbar-color:var(--rf-border-strong,#36414d) var(--rf-bg,#0b0d10)}
  .maintenance-task-list{height:clamp(18rem,34vh,27rem);overflow:auto;background:var(--rf-bg,#0b0d10);scrollbar-color:var(--rf-border-strong,#36414d) var(--rf-bg,#0b0d10)}.maintenance-task-row{padding:10px 12px;border-bottom:1px solid var(--rf-border,#29313a)}.maintenance-task-row:last-child{border-bottom:0}.maintenance-task-row>span{display:block;color:var(--rf-accent,#38bdf8);font-size:10px}.maintenance-task-row pre{margin:5px 0 0;color:var(--rf-text,#f5f7fa);white-space:pre-wrap;word-break:break-word;font:inherit;line-height:1.45}
  .maintenance-list{display:grid}.maintenance-list-row,.watchdog-row{min-height:58px;padding:9px 12px;display:flex;align-items:center;justify-content:space-between;gap:14px;border-bottom:1px solid var(--rf-border,#29313a)}.maintenance-list-row:last-child,.watchdog-row:last-child{border-bottom:0}.maintenance-list-row:hover,.watchdog-row:hover{background:var(--rf-accent-soft,rgba(56,189,248,.10))}.maintenance-list-copy{min-width:0;display:grid;gap:3px}.maintenance-list-copy>strong{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-text,#f5f7fa)}.maintenance-list-copy>.cell-sub{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-muted,#8d98a4)}.maintenance-row-actions{display:flex;align-items:center;justify-content:flex-end;gap:8px;flex:0 0 auto}.maintenance-empty{min-height:74px;padding:16px;display:grid;place-items:center;color:var(--rf-muted,#8d98a4);background:var(--rf-bg,#0b0d10);font-size:var(--ui-xs,12px)}.compact-list{max-height:22rem;overflow:auto;scrollbar-color:var(--rf-border-strong,#36414d) var(--rf-surface,#12151a)}
  .button.danger{border-color:rgba(248,81,73,.38)!important;color:var(--bad,#f85149)!important}.button.danger:hover{background:color-mix(in srgb,var(--bad,#f85149) 10%,var(--rf-surface,#12151a))!important}
  @media(max-width:1200px){.maintenance-summary{grid-template-columns:repeat(3,minmax(0,1fr))}.maintenance-workbench{grid-template-columns:1fr}.maintenance-secondary-grid{grid-template-columns:1fr}}
  @media(max-width:900px){.integration-grid{grid-template-columns:1fr}.maintenance-summary{grid-template-columns:repeat(2,minmax(0,1fr))}}
  @media(max-width:680px){.maintenance-commandbar{align-items:flex-start;flex-direction:column}.maintenance-actions{width:100%;justify-content:flex-start}.maintenance-summary{grid-template-columns:1fr}.maintenance-list-row,.watchdog-row{align-items:flex-start;flex-direction:column}.maintenance-row-actions{width:100%;justify-content:flex-start}.maintenance-pre,.maintenance-task-list{height:22rem}}
</style>
