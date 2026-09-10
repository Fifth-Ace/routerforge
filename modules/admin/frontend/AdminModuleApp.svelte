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
    getAdminFiles,
    getModule,
    readAdminFile
  } from '$lib/api.js';
  import { startSerialPolling } from '$lib/polling.js';
  import { bytes } from '$lib/utils.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

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

  $: locale = $settings.locale || 'ru';
  $: copy = locale === 'ru' ? {
    files: 'Файлы',
    terminal: 'Терминал',
    run: 'Выполнить',
    clear: 'Очистить',
    terminalHint: 'Команды выполняются через /bin/sh -lc от root, cwd разрешён только внутри /opt или /tmp. Таймаут 15 секунд.',
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
    noPreview: 'Предпросмотр доступен только для UTF-8 текстовых файлов до 256 KiB.'
  } : {
    files: 'Files',
    terminal: 'Terminal',
    run: 'Run',
    clear: 'Clear',
    terminalHint: 'Commands run through /bin/sh -lc as root; cwd is restricted to /opt or /tmp. Timeout is 15 seconds.',
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
    noPreview: 'Preview supports UTF-8 text files up to 256 KiB only.'
  };

  $: tabs = [
    ['processes', t(locale, 'manage.tabs.processes')],
    ['ports', t(locale, 'manage.tabs.ports')],
    ['services', t(locale, 'manage.tabs.services')],
    ['packages', t(locale, 'manage.tabs.packages')],
    ['files', copy.files],
    ['terminal', copy.terminal]
  ];

  $: q = search.trim().toLowerCase();
  $: filteredProcesses = processes.filter((p) => !q || `${p.pid} ${p.name} ${p.user} ${p.command}`.toLowerCase().includes(q));
  $: filteredPorts = ports.filter((p) => !q || `${p.protocol} ${p.local_address} ${p.local_port} ${p.process} ${p.pid}`.toLowerCase().includes(q));
  $: filteredServices = services.filter((s) => !q || `${s.id} ${s.name} ${s.path}`.toLowerCase().includes(q));
  $: filteredPackages = packages.filter((p) => !q || `${p.name} ${p.version} ${p.architecture}`.toLowerCase().includes(q));
  $: filteredFiles = fileEntries.filter((entry) => !q || `${entry.name} ${entry.path} ${entry.kind}`.toLowerCase().includes(q));
  $: editorDirty = selectedFile && editorContent !== editorOriginal;

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
    if (!selectedFile || !editorDirty) return;
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
          <div><strong class="mono">{selectedFile.path}</strong><span>{bytes(selectedFile.size || 0)} · {selectedFile.mode} {editorDirty ? `· ${copy.dirty}` : ''}</span></div>
          <div class="actions-cell">
            <button class="button" onclick={saveEditor} disabled={!editorDirty || editorBusy}>{copy.save}</button>
            <button class="button" onclick={() => { selectedFile = null; editorContent = ''; editorOriginal = ''; }}>{copy.close}</button>
          </div>
        </div>
        <textarea class="file-editor mono" bind:value={editorContent} spellcheck="false"></textarea>
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
</style>
