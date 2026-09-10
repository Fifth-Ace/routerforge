<script>
  import { onMount } from 'svelte';
  import {
    adminFileChmod, adminFileCopy, adminFileDelete, adminFileDownloadURL,
    adminFileMkdir, adminFileMove, adminFileWrite, getAdminFiles, getAdminFileVolumes, readAdminFile
  } from '$lib/api.js';
  import { bytes } from '$lib/utils.js';
  import FileEditorDrawer from './FileEditorDrawer.svelte';
  import FilePropertiesModal from './FilePropertiesModal.svelte';

  export let locale = 'ru';

  const VIEW_KEY = 'routerforge.admin.files.view';
  const LEFT_KEY = 'routerforge.admin.files.left.path';
  const RIGHT_KEY = 'routerforge.admin.files.right.path';
  const EXPLORER_KEY = 'routerforge.admin.files.explorer.path';
  const WRITE_LIMIT = 128 * 1024;

  let volumes = [
    { id: 'entware', label: 'Entware', mount: '/opt', read_only: false, kind: 'entware' },
    { id: 'temp', label: 'Temporary', mount: '/tmp', read_only: false, kind: 'temporary' }
  ];
  let explorerVolumeRoot = '/opt';
  let treeState = {};

  let view = 'commander';
  let busyCount = 0;
  let errorText = '';
  let actionText = '';
  let active = 'left';

  let left = makePanel('/opt');
  let right = makePanel('/opt/etc');

  let explorerPath = '/opt';
  let explorerEntries = [];
  let explorerSearch = '';
  let explorerSort = 'name';
  let explorerSortDir = 1;

  let selectedFile = null;
  let editorContent = '';
  let editorOriginal = '';
  let editorBusy = false;
  let editorForceReadOnly = false;
  let propertiesEntry = null;
  let propertiesReload = null;

  $: busy = busyCount > 0;
  $: editorDirty = !!selectedFile && editorContent !== editorOriginal;
  $: editorReadOnly = editorForceReadOnly || (!!selectedFile && Number(selectedFile.size || 0) > WRITE_LIMIT);
  $: explorerFiltered = sortEntries(
    explorerEntries.filter((entry) => !explorerSearch.trim() || `${entry.name} ${entry.path}`.toLowerCase().includes(explorerSearch.trim().toLowerCase())),
    explorerSort,
    explorerSortDir
  );

  $: copy = locale === 'ru' ? {
    commander: 'Commander', explorer: 'Explorer', refresh: 'Обновить',
    folder: 'Папка', file: 'Файл', copy: 'Копировать', move: 'Переместить', rename: 'Переименовать',
    remove: 'Удалить', chmod: 'Права', download: 'Скачать', actions: 'Действия', name: 'Имя',
    size: 'Размер', modified: 'Изменён', mode: 'Права', search: 'Фильтр…', path: 'Путь',
    noSelection: 'Сначала выбери объект в активной панели.', regularOnly: 'Копирование каталогов backend пока не поддерживает.',
    destination: 'Путь назначения', viewFile: 'Просмотр', edit: 'Правка', save: 'Сохранить', close: 'Закрыть',
    previous: 'Назад', next: 'Вперёд', up: 'Вверх', swap: 'Поменять панели', quick: 'Дерево папок',
    selected: 'выбрано', items: 'объектов', empty: 'Папка пуста', open: 'Открыть',
    volume: 'Диск / том', properties: 'Свойства'
  } : {
    commander: 'Commander', explorer: 'Explorer', refresh: 'Refresh',
    folder: 'Folder', file: 'File', copy: 'Copy', move: 'Move', rename: 'Rename',
    remove: 'Delete', chmod: 'Mode', download: 'Download', actions: 'Actions', name: 'Name',
    size: 'Size', modified: 'Modified', mode: 'Mode', search: 'Filter…', path: 'Path',
    noSelection: 'Select an item in the active panel first.', regularOnly: 'Directory copy is not supported by the backend yet.',
    destination: 'Destination path', viewFile: 'View', edit: 'Edit', save: 'Save', close: 'Close',
    previous: 'Back', next: 'Forward', up: 'Up', swap: 'Swap panels', quick: 'Folder tree',
    selected: 'selected', items: 'items', empty: 'Folder is empty', open: 'Open',
    volume: 'Disk / volume', properties: 'Properties'
  };

  function makePanel(path) {
    return { path, volumeRoot: rootForPath(path), pathInput: path, entries: [], selected: null, filter: '', sort: 'name', sortDir: 1, history: [path], historyIndex: 0 };
  }

  function volumeForPath(path) {
    const clean = String(path || '/');
    return [...volumes]
      .filter((volume) => clean === volume.mount || clean.startsWith(`${volume.mount.replace(/\/$/, '')}/`))
      .sort((a, b) => b.mount.length - a.mount.length)[0] || volumes[0];
  }

  function rootForPath(path) { return volumeForPath(path)?.mount || '/opt'; }
  function pathWritable(path) { return !(volumeForPath(path)?.read_only ?? true); }

  function err(error) { return error?.payload?.error || error?.message || String(error); }
  function join(parentPath, name) { return `${parentPath.replace(/\/+$/, '')}/${name}` || `/${name}`; }
  function parentWithin(path, root = rootForPath(path)) {
    const clean = path.replace(/\/+$/, '') || '/';
    const boundary = root.replace(/\/+$/, '') || '/';
    if (clean === boundary) return boundary;
    const pos = clean.lastIndexOf('/');
    const value = pos <= 0 ? '/' : clean.slice(0, pos);
    if (boundary === '/') return value;
    return value === boundary || value.startsWith(`${boundary}/`) ? value : boundary;
  }
  function base(path) { const clean = path.replace(/\/+$/, ''); return clean.slice(clean.lastIndexOf('/') + 1); }

  function panelFor(side) { return side === 'left' ? left : right; }
  function setPanel(side, next) { if (side === 'left') left = next; else right = next; }
  function activePanel() { return panelFor(active); }
  function otherPanel() { return panelFor(active === 'left' ? 'right' : 'left'); }
  function panelEntries(panel) {
    const q = panel.filter.trim().toLowerCase();
    const filtered = q ? panel.entries.filter((entry) => `${entry.name} ${entry.path}`.toLowerCase().includes(q)) : panel.entries;
    return sortEntries(filtered, panel.sort, panel.sortDir);
  }

  function sortEntries(entries, key, dir) {
    return [...entries].sort((a, b) => {
      if (a.kind === 'directory' && b.kind !== 'directory') return -1;
      if (a.kind !== 'directory' && b.kind === 'directory') return 1;
      let av; let bv;
      if (key === 'size') { av = Number(a.size || 0); bv = Number(b.size || 0); }
      else if (key === 'modified') { av = Date.parse(a.modified_at || 0) || 0; bv = Date.parse(b.modified_at || 0) || 0; }
      else if (key === 'mode') { av = a.mode || ''; bv = b.mode || ''; }
      else { av = (a.name || '').toLowerCase(); bv = (b.name || '').toLowerCase(); }
      if (av < bv) return -1 * dir;
      if (av > bv) return 1 * dir;
      return 0;
    });
  }

  function cycleSort(side, key) {
    const panel = panelFor(side);
    const sortDir = panel.sort === key ? panel.sortDir * -1 : 1;
    setPanel(side, { ...panel, sort: key, sortDir });
  }

  function cycleExplorerSort(key) {
    const same = explorerSort === key;
    explorerSort = key;
    explorerSortDir = same ? explorerSortDir * -1 : 1;
  }

  function setView(next) {
    view = next;
    localStorage.setItem(VIEW_KEY, next);
    if (next === 'explorer') loadExplorer(explorerPath);
  }

  function setAction(value) {
    actionText = value;
    setTimeout(() => { if (actionText === value) actionText = ''; }, 3500);
  }

  function rememberPanelPath(side, path) {
    localStorage.setItem(side === 'left' ? LEFT_KEY : RIGHT_KEY, path);
  }

  async function loadPanel(side, path, historyMode = 'push') {
    busyCount += 1; errorText = '';
    try {
      const result = await getAdminFiles(path);
      const current = panelFor(side);
      const resolved = result.path || path;
      let history = current.history;
      let historyIndex = current.historyIndex;
      if (historyMode === 'push' && current.path !== resolved) {
        history = [...history.slice(0, historyIndex + 1), resolved];
        historyIndex = history.length - 1;
      }
      setPanel(side, {
        ...current,
        path: resolved,
        volumeRoot: rootForPath(resolved),
        pathInput: resolved,
        entries: result.entries || [],
        selected: null,
        history,
        historyIndex
      });
      rememberPanelPath(side, resolved);
    } catch (error) { errorText = err(error); }
    finally { busyCount -= 1; }
  }

  async function navigateHistory(side, delta) {
    const panel = panelFor(side);
    const nextIndex = panel.historyIndex + delta;
    if (nextIndex < 0 || nextIndex >= panel.history.length) return;
    const target = panel.history[nextIndex];
    setPanel(side, { ...panel, historyIndex: nextIndex });
    await loadPanel(side, target, 'history');
  }

  async function submitPath(side) {
    const panel = panelFor(side);
    const target = panel.pathInput.trim();
    if (!target) return;
    await loadPanel(side, target);
  }

  async function loadExplorer(path = explorerPath) {
    busyCount += 1; errorText = '';
    try {
      const result = await getAdminFiles(path);
      explorerPath = result.path || path;
      explorerEntries = result.entries || [];
      explorerVolumeRoot = rootForPath(explorerPath);
      localStorage.setItem(EXPLORER_KEY, explorerPath);
      await ensureTreePath(explorerPath);
    } catch (error) { errorText = err(error); }
    finally { busyCount -= 1; }
  }

  function select(side, entry) {
    active = side;
    const panel = panelFor(side);
    setPanel(side, { ...panel, selected: entry });
  }

  async function openEntry(side, entry, forceReadOnly = false) {
    if (entry.kind === 'directory') {
      if (side === 'explorer') return loadExplorer(entry.path);
      return loadPanel(side, entry.path);
    }
    if (entry.kind !== 'file') return;
    editorBusy = true; errorText = '';
    try {
      const result = await readAdminFile(entry.path);
      selectedFile = result;
      editorContent = result.content || '';
      editorOriginal = editorContent;
      editorForceReadOnly = forceReadOnly;
    } catch (error) { errorText = err(error); }
    finally { editorBusy = false; }
  }

  async function saveEditor() {
    if (!selectedFile || !editorDirty || editorReadOnly) return;
    editorBusy = true; errorText = '';
    try {
      const result = await adminFileWrite({
        path: selectedFile.path, confirm_path: selectedFile.path, content: editorContent, create: false,
        expected_size: selectedFile.size, expected_mtime_ns: selectedFile.mtime_ns
      });
      const fresh = await readAdminFile(result.path || selectedFile.path);
      selectedFile = fresh;
      editorContent = fresh.content || '';
      editorOriginal = editorContent;
      setAction(`${copy.save}: ${fresh.path}`);
      await refreshAll();
    } catch (error) { errorText = err(error); }
    finally { editorBusy = false; }
  }

  async function createFolder(path, reload) {
    const name = prompt(copy.folder); if (!name) return;
    const target = join(path, name);
    try { await adminFileMkdir({ path: target, confirm_path: target }); setAction(`${copy.folder}: ${target}`); await reload(); }
    catch (error) { errorText = err(error); }
  }

  async function createFile(path, reload) {
    const name = prompt(copy.file); if (!name) return;
    const target = join(path, name);
    try {
      await adminFileWrite({ path: target, confirm_path: target, content: '', create: true });
      setAction(`${copy.file}: ${target}`); await reload();
    } catch (error) { errorText = err(error); }
  }

  async function renameEntry(entry, reload) {
    const destination = prompt(copy.rename, entry.path); if (!destination || destination === entry.path) return;
    if (!confirm(`${entry.path}\n→ ${destination}?`)) return;
    try {
      await adminFileMove({
        source: entry.path, destination, confirm_source: entry.path, confirm_destination: destination,
        expected_size: entry.size, expected_mtime_ns: entry.mtime_ns
      });
      setAction(`${copy.rename}: ${destination}`); await reload();
    } catch (error) { errorText = err(error); }
  }

  async function deleteEntry(entry, reload) {
    if (!confirm(`${copy.remove}: ${entry.path}?`)) return;
    try {
      await adminFileDelete({ path: entry.path, confirm_path: entry.path, expected_size: entry.size, expected_mtime_ns: entry.mtime_ns });
      setAction(`${copy.remove}: ${entry.path}`); await reload();
    } catch (error) { errorText = err(error); }
  }

  function showProperties(entry, reload) {
    propertiesEntry = entry;
    propertiesReload = reload;
  }

  async function applyPropertiesMode(mode) {
    const entry = propertiesEntry;
    if (!entry) return;
    await adminFileChmod({
      path: entry.path, confirm_path: entry.path, mode,
      expected_size: entry.size, expected_mtime_ns: entry.mtime_ns
    });
    setAction(`${copy.chmod}: ${entry.path} → ${mode}`);
    propertiesEntry = null;
    if (propertiesReload) await propertiesReload();
  }

  async function changePanelVolume(side, mount) {
    const panel = panelFor(side);
    setPanel(side, { ...panel, volumeRoot: mount, history: [mount], historyIndex: 0 });
    await loadPanel(side, mount, 'history');
  }

  async function changeExplorerVolume(mount) {
    explorerVolumeRoot = mount;
    treeState = {};
    await loadExplorer(mount);
  }

  async function loadTreeChildren(path) {
    const current = treeState[path];
    if (current?.loaded) return;
    try {
      const result = await getAdminFiles(path);
      const children = (result.entries || []).filter((entry) => entry.kind === 'directory');
      treeState = { ...treeState, [path]: { loaded: true, expanded: current?.expanded ?? true, children } };
    } catch (error) { errorText = err(error); }
  }

  async function toggleTree(path) {
    const current = treeState[path] || { loaded: false, expanded: false, children: [] };
    if (!current.loaded) {
      await loadTreeChildren(path);
      const fresh = treeState[path] || current;
      treeState = { ...treeState, [path]: { ...fresh, expanded: true } };
      return;
    }
    treeState = { ...treeState, [path]: { ...current, expanded: !current.expanded } };
  }

  async function ensureTreePath(path) {
    const root = rootForPath(path);
    await loadTreeChildren(root);
    const parts = path.slice(root.length).split('/').filter(Boolean);
    let current = root;
    for (const part of parts) {
      const node = treeState[current] || { loaded: false, expanded: false, children: [] };
      treeState = { ...treeState, [current]: { ...node, expanded: true } };
      const next = `${current.replace(/\/$/, '')}/${part}`;
      await loadTreeChildren(next);
      current = next;
    }
  }

  function treeRows(root) {
    const rows = [];
    const walk = (path, label, depth) => {
      const state = treeState[path] || { loaded: false, expanded: false, children: [] };
      rows.push({ path, label, depth, expanded: state.expanded, loaded: state.loaded, hasChildren: !state.loaded || state.children.length > 0 });
      if (!state.expanded) return;
      for (const child of state.children) walk(child.path, child.name, depth + 1);
    };
    const volume = volumes.find((item) => item.mount === root);
    walk(root, volume?.label || root, 0);
    return rows;
  }

  async function transfer(kind) {
    const sourcePanel = activePanel();
    const destinationPanel = otherPanel();
    const entry = sourcePanel.selected;
    if (!entry) { errorText = copy.noSelection; return; }
    if (kind === 'copy' && entry.kind !== 'file') { errorText = copy.regularOnly; return; }
    const proposed = join(destinationPanel.path, entry.name);
    const destination = prompt(copy.destination, proposed);
    if (!destination || destination === entry.path) return;
    if (!confirm(`${copy[kind]}:\n${entry.path}\n→ ${destination}?`)) return;
    try {
      if (kind === 'copy') {
        await adminFileCopy({
          source: entry.path, destination, confirm_source: entry.path, confirm_destination: destination,
          expected_size: entry.size, expected_mtime_ns: entry.mtime_ns
        });
      } else {
        await adminFileMove({
          source: entry.path, destination, confirm_source: entry.path, confirm_destination: destination,
          expected_size: entry.size, expected_mtime_ns: entry.mtime_ns
        });
      }
      setAction(`${copy[kind]}: ${destination}`);
      await Promise.all([loadPanel('left', left.path, 'history'), loadPanel('right', right.path, 'history')]);
    } catch (error) { errorText = err(error); }
  }

  function swapPanels() {
    const oldLeft = left;
    left = right;
    right = oldLeft;
    rememberPanelPath('left', left.path);
    rememberPanelPath('right', right.path);
    active = active === 'left' ? 'right' : 'left';
  }

  async function refreshAll() {
    if (view === 'commander') await Promise.all([loadPanel('left', left.path, 'history'), loadPanel('right', right.path, 'history')]);
    else await loadExplorer(explorerPath);
  }

  async function invokeSelected(action) {
    const panel = activePanel();
    const entry = panel.selected;
    if (!entry && !['mkdir'].includes(action)) { errorText = copy.noSelection; return; }
    if (action === 'view') return openEntry(active, entry, true);
    if (action === 'edit') return openEntry(active, entry, false);
    if (action === 'copy') return transfer('copy');
    if (action === 'move') return transfer('move');
    if (action === 'mkdir') return createFolder(panel.path, () => loadPanel(active, panel.path, 'history'));
    if (action === 'delete') return deleteEntry(entry, () => loadPanel(active, panel.path, 'history'));
    if (action === 'properties') return showProperties(entry, () => loadPanel(active, panel.path, 'history'));
  }

  function selectRelative(delta) {
    const panel = activePanel();
    const entries = panelEntries(panel);
    if (!entries.length) return;
    let index = panel.selected ? entries.findIndex((item) => item.path === panel.selected.path) : -1;
    index = Math.max(0, Math.min(entries.length - 1, index + delta));
    select(active, entries[index]);
  }

  function handleCommanderKey(event) {
    if (view !== 'commander' || selectedFile) return;
    const tag = event.target?.tagName?.toLowerCase();
    if (tag === 'input' || tag === 'textarea') return;
    if (event.key === 'Tab') { event.preventDefault(); active = active === 'left' ? 'right' : 'left'; return; }
    if (event.ctrlKey && event.key.toLowerCase() === 'u') { event.preventDefault(); swapPanels(); return; }
    if (event.key === 'ArrowDown') { event.preventDefault(); selectRelative(1); return; }
    if (event.key === 'ArrowUp') { event.preventDefault(); selectRelative(-1); return; }
    if (event.key === 'Home') { event.preventDefault(); selectRelative(-999999); return; }
    if (event.key === 'End') { event.preventDefault(); selectRelative(999999); return; }
    if (event.key === 'PageDown') { event.preventDefault(); selectRelative(12); return; }
    if (event.key === 'PageUp') { event.preventDefault(); selectRelative(-12); return; }
    if (event.key === 'Backspace') { event.preventDefault(); loadPanel(active, parentWithin(activePanel().path, activePanel().volumeRoot)); return; }
    if (event.altKey && event.key === 'Enter') { event.preventDefault(); invokeSelected('properties'); return; }
    if (event.key === 'Enter' && activePanel().selected) { event.preventDefault(); openEntry(active, activePanel().selected); return; }
    if (event.key === 'F3') { event.preventDefault(); invokeSelected('view'); return; }
    if (event.key === 'F4') { event.preventDefault(); invokeSelected('edit'); return; }
    if (event.key === 'F5') { event.preventDefault(); invokeSelected('copy'); return; }
    if (event.key === 'F6') { event.preventDefault(); invokeSelected('move'); return; }
    if (event.key === 'F7') { event.preventDefault(); invokeSelected('mkdir'); return; }
    if (event.key === 'F8') { event.preventDefault(); invokeSelected('delete'); }
  }

  function breadcrumbs(path) {
    const root = rootForPath(path);
    if (path === root) return [{ label: root, path: root }];
    const rest = path.slice(root.length).split('/').filter(Boolean);
    const crumbs = [{ label: root, path: root }];
    let current = root;
    for (const part of rest) { current = `${current}/${part}`; crumbs.push({ label: part, path: current }); }
    return crumbs;
  }

  onMount(() => {
    let disposed = false;
    (async () => {
      try {
        const result = await getAdminFileVolumes();
        if (!disposed && Array.isArray(result.volumes) && result.volumes.length) volumes = result.volumes;
      } catch (error) { errorText = err(error); }
      if (disposed) return;
    const stored = localStorage.getItem(VIEW_KEY);
    if (stored === 'explorer' || stored === 'commander') view = stored;
    const leftPath = localStorage.getItem(LEFT_KEY) || '/opt';
    const rightPath = localStorage.getItem(RIGHT_KEY) || '/opt/etc';
    explorerPath = localStorage.getItem(EXPLORER_KEY) || leftPath;
    left = makePanel(leftPath);
    right = makePanel(rightPath);
    loadPanel('left', leftPath, 'history');
    loadPanel('right', rightPath, 'history');
    explorerVolumeRoot = rootForPath(explorerPath);
    if (view === 'explorer') loadExplorer(explorerPath);
    else loadTreeChildren(explorerVolumeRoot);
    })();
    window.addEventListener('keydown', handleCommanderKey);
    return () => {
      disposed = true;
      window.removeEventListener('keydown', handleCommanderKey);
    };
  });
</script>

<section class="fm-shell">
  <div class="fm-toolbar">
    <div class="view-toggle">
      <button class:active={view === 'commander'} onclick={() => setView('commander')}>◫ {copy.commander}</button>
      <button class:active={view === 'explorer'} onclick={() => setView('explorer')}>☷ {copy.explorer}</button>
    </div>
    <button onclick={refreshAll} disabled={busy}>↻ {copy.refresh}</button>
    {#if view === 'commander'}
      <button onclick={swapPanels} title="Ctrl+U">⇄ {copy.swap}</button>
    {/if}
    <span class="spacer"></span>
    {#if busy}<span class="busy-pill">WORKING…</span>{/if}
  </div>

  {#if actionText}<div class="fm-ok">{actionText}</div>{/if}
  {#if errorText}<div class="fm-error">{errorText}</div>{/if}

  {#if view === 'commander'}
    <div class="commander-grid">
      {#each [['left', left], ['right', right]] as [side, panel]}
        <section class:active-panel={active === side} class="commander-panel" onclick={() => active = side}>
          <div class="panel-nav">
            <button title={copy.previous} onclick={() => navigateHistory(side, -1)} disabled={panel.historyIndex <= 0}>‹</button>
            <button title={copy.next} onclick={() => navigateHistory(side, 1)} disabled={panel.historyIndex >= panel.history.length - 1}>›</button>
            <button title={copy.up} onclick={() => loadPanel(side, parentWithin(panel.path, panel.volumeRoot))}>↑</button>
            <select class="volume-select" aria-label={copy.volume} onchange={(event) => changePanelVolume(side, event.currentTarget.value)} value={panel.volumeRoot}>
              {#each volumes as volume}<option value={volume.mount}>{volume.label}{volume.read_only ? ' · RO' : ''}</option>{/each}
            </select>
            <form class="path-form" onsubmit={(event) => { event.preventDefault(); submitPath(side); }}>
              <input class="path-input mono" bind:value={panel.pathInput} aria-label={copy.path}/>
            </form>
          </div>

          <div class="panel-filter-row">
            <input bind:value={panel.filter} placeholder={copy.search}/>
            <button class:sort-active={panel.sort === 'name'} onclick={() => cycleSort(side, 'name')}>Name{panel.sort === 'name' ? (panel.sortDir === 1 ? ' ↑' : ' ↓') : ''}</button>
            <button class:sort-active={panel.sort === 'size'} onclick={() => cycleSort(side, 'size')}>Size{panel.sort === 'size' ? (panel.sortDir === 1 ? ' ↑' : ' ↓') : ''}</button>
            <button class:sort-active={panel.sort === 'modified'} onclick={() => cycleSort(side, 'modified')}>Date{panel.sort === 'modified' ? (panel.sortDir === 1 ? ' ↑' : ' ↓') : ''}</button>
          </div>

          <div class="commander-head"><span>{copy.name}</span><span>{copy.size}</span><span>{copy.modified}</span><span>{copy.mode}</span></div>
          <div class="commander-list">
            <button class="file-row parent-row" onclick={() => loadPanel(side, parentWithin(panel.path, panel.volumeRoot))}><span>📁 ..</span><span>—</span><span>—</span><span>—</span></button>
            {#each panelEntries(panel) as entry (entry.path)}
              <button class:selected={panel.selected?.path === entry.path} class="file-row" onclick={() => select(side, entry)} ondblclick={() => openEntry(side, entry)}>
                <span class="entry-name">{entry.kind === 'directory' ? '📁' : entry.kind === 'file' ? '📄' : '↗'} {entry.name}</span>
                <span class="mono">{entry.kind === 'file' ? bytes(entry.size || 0) : '—'}</span>
                <span class="mono date-cell">{entry.modified_at ? new Date(entry.modified_at).toLocaleString() : '—'}</span>
                <span class="mono">{entry.mode || ''}</span>
              </button>
            {/each}
            {#if panelEntries(panel).length === 0}<div class="empty-row">{copy.empty}</div>{/if}
          </div>

          <div class="panel-status mono">
            <span>{panelEntries(panel).length} {copy.items}</span>
            <span class="spacer"></span>
            {#if panel.selected}<span class="selected-info">{base(panel.selected.path)}{panel.selected.kind === 'file' ? ` · ${bytes(panel.selected.size || 0)}` : ''}</span>{/if}
          </div>
        </section>
      {/each}
    </div>

    <div class="commander-keys">
      <button onclick={() => invokeSelected('view')}><kbd>F3</kbd> {copy.viewFile}</button>
      <button onclick={() => invokeSelected('edit')}><kbd>F4</kbd> {copy.edit}</button>
      <button onclick={() => invokeSelected('copy')}><kbd>F5</kbd> {copy.copy}</button>
      <button onclick={() => invokeSelected('move')}><kbd>F6</kbd> {copy.move}</button>
      <button onclick={() => invokeSelected('mkdir')}><kbd>F7</kbd> MkDir</button>
      <button class="danger" onclick={() => invokeSelected('delete')}><kbd>F8</kbd> {copy.remove}</button>
      <button onclick={() => invokeSelected('properties')}><kbd>Alt+Enter</kbd> {copy.properties}</button>
      <span class="spacer"></span><span class="key-hint mono">TAB panel · Ctrl+U swap · Enter open · Backspace up</span>
    </div>
  {:else}
    <div class="explorer-shell">
      <aside class="quick-tree">
        <div class="tree-volume">
          <label>{copy.volume}</label>
          <select value={explorerVolumeRoot} onchange={(event) => changeExplorerVolume(event.currentTarget.value)}>
            {#each volumes as volume}<option value={volume.mount}>{volume.label}{volume.read_only ? ' · RO' : ''}</option>{/each}
          </select>
        </div>
        <div class="quick-title">{copy.quick}</div>
        <div class="folder-tree">
          {#each treeRows(explorerVolumeRoot) as node (node.path)}
            <div class:tree-active={explorerPath === node.path} class="tree-row" style={`padding-left:${0.45 + node.depth * 0.85}rem`}>
              <button class="tree-toggle" onclick={() => toggleTree(node.path)} disabled={!node.hasChildren}>{node.hasChildren ? (node.expanded ? '▾' : '▸') : '·'}</button>
              <button class="tree-name" onclick={() => loadExplorer(node.path)} title={node.path}>📁 {node.label}</button>
            </div>
          {/each}
        </div>
      </aside>

      <section class="explorer-main">
        <div class="explorer-toolbar">
          <button title={copy.up} onclick={() => loadExplorer(parentWithin(explorerPath, explorerVolumeRoot))}>↑</button>
          <button title={copy.refresh} onclick={() => loadExplorer(explorerPath)}>↻</button>
          <button onclick={() => createFolder(explorerPath, () => loadExplorer(explorerPath))}>＋ {copy.folder}</button>
          <button onclick={() => createFile(explorerPath, () => loadExplorer(explorerPath))}>＋ {copy.file}</button>
          <span class="spacer"></span>
          <div class="search-box">⌕ <input bind:value={explorerSearch} placeholder={copy.search}/></div>
        </div>

        <div class="breadcrumbs">
          {#each breadcrumbs(explorerPath) as crumb, index}
            {#if index > 0}<span>›</span>{/if}
            <button onclick={() => loadExplorer(crumb.path)}>{crumb.label}</button>
          {/each}
        </div>

        <div class="explorer-table">
          <div class="explorer-head">
            <button onclick={() => cycleExplorerSort('name')}>{copy.name}</button>
            <button onclick={() => cycleExplorerSort('size')}>{copy.size}</button>
            <button onclick={() => cycleExplorerSort('modified')}>{copy.modified}</button>
            <button onclick={() => cycleExplorerSort('mode')}>{copy.mode}</button>
            <span>{copy.actions}</span>
          </div>
          <button class="explorer-row parent-row" onclick={() => loadExplorer(parentWithin(explorerPath, explorerVolumeRoot))}><span>📁 ..</span><span>—</span><span>—</span><span>—</span><span></span></button>
          {#each explorerFiltered as entry (entry.path)}
            <div class="explorer-row">
              <button class="entry-open" onclick={() => openEntry('explorer', entry)}>
                <span class="file-icon">{entry.kind === 'directory' ? '📁' : entry.kind === 'file' ? '📄' : '↗'}</span>
                <span><strong>{entry.name}</strong><small>{entry.path}</small></span>
              </button>
              <span class="mono">{entry.kind === 'file' ? bytes(entry.size || 0) : '—'}</span>
              <span class="mono">{entry.modified_at ? new Date(entry.modified_at).toLocaleString() : '—'}</span>
              <span class="mono permission">{entry.mode || ''}</span>
              <span class="row-actions">
                {#if entry.kind === 'file'}
                  <button title={copy.viewFile} onclick={() => openEntry('explorer', entry, true)}>◉</button>
                  <button title={copy.edit} onclick={() => openEntry('explorer', entry, false)}>✎</button>
                  <a title={copy.download} href={adminFileDownloadURL(entry.path)}>↓</a>
                {/if}
                <button title={copy.rename} onclick={() => renameEntry(entry, () => loadExplorer(explorerPath))}>↷</button>
                <button title={copy.properties} onclick={() => showProperties(entry, () => loadExplorer(explorerPath))}>ⓘ</button>
                <button title={copy.remove} class="danger" onclick={() => deleteEntry(entry, () => loadExplorer(explorerPath))}>×</button>
              </span>
            </div>
          {/each}
          {#if explorerFiltered.length === 0}<div class="empty-row explorer-empty">{copy.empty}</div>{/if}
        </div>
        <div class="explorer-status mono"><span>{explorerFiltered.length} / {explorerEntries.length} {copy.items}</span><span class="spacer"></span><span>{explorerPath}</span></div>
      </section>
    </div>
  {/if}
</section>

{#if selectedFile}
  <FileEditorDrawer
    file={selectedFile}
    bind:content={editorContent}
    dirty={editorDirty}
    readOnly={editorReadOnly}
    busy={editorBusy}
    locale={locale}
    onSave={saveEditor}
    onClose={() => { selectedFile = null; editorContent = ''; editorOriginal = ''; editorForceReadOnly = false; }}
  />
{/if}

{#if propertiesEntry}
  <FilePropertiesModal
    entry={propertiesEntry}
    writable={pathWritable(propertiesEntry.path)}
    locale={locale}
    onClose={() => { propertiesEntry = null; propertiesReload = null; }}
    onEdit={() => { const entry = propertiesEntry; propertiesEntry = null; if (entry?.kind === 'file') openEntry('explorer', entry, false); }}
    onApplyMode={applyPropertiesMode}
  />
{/if}

<style>
  .fm-shell{border:1px solid var(--border-color,rgba(127,127,127,.22));border-radius:.72rem;overflow:hidden;background:#0c1219;color:#d7e0e8}
  .fm-toolbar,.panel-nav,.panel-filter-row,.panel-status,.commander-keys,.explorer-toolbar,.breadcrumbs,.explorer-status{display:flex;align-items:center;gap:.42rem}
  .fm-toolbar{padding:.58rem .68rem;border-bottom:1px solid #22303c;background:#101821}
  button,input,select,.search-box,a{font:inherit;color:inherit;border:1px solid #2a3946;background:#141f2a;border-radius:.38rem}
  button,select,a{padding:.34rem .5rem;cursor:pointer}button:hover,a:hover{background:#1b2a36;border-color:#466073}button:disabled{opacity:.35;cursor:not-allowed}
  input{padding:.38rem .5rem;outline:0}input:focus{border-color:#5de4c7;box-shadow:0 0 0 1px rgba(93,228,199,.18)}
  .danger{border-color:#583941!important;color:#ff9da4!important}.spacer{flex:1}.mono{font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace}
  .view-toggle{display:flex}.view-toggle button{border-radius:0}.view-toggle button:first-child{border-radius:.38rem 0 0 .38rem}.view-toggle button:last-child{border-radius:0 .38rem .38rem 0}.view-toggle button.active{background:#15392d;border-color:#39705c;color:#79e7ba}
  .busy-pill{color:#ffd866;font:700 .58rem/1 "Roboto Mono",monospace;letter-spacing:.08em}.fm-ok,.fm-error{padding:.45rem .7rem;font-size:.74rem}.fm-ok{color:#79e7ba;background:#10241d}.fm-error{color:#ff9da4;background:#2a1419}

  .commander-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:1px;background:#273643;min-height:35rem}
  .commander-panel{min-width:0;background:#0c131b;outline:2px solid transparent;outline-offset:-2px}.commander-panel.active-panel{outline-color:#5de4c7}
  .panel-nav{padding:.48rem;border-bottom:1px solid #1f2c37;background:#101821}.panel-nav button{min-width:2rem}.panel-nav select{width:2.4rem;padding:.34rem .25rem}.path-form{flex:1;min-width:0}.path-input{box-sizing:border-box;width:100%;background:#0b1218;color:#b8f3df}
  .panel-filter-row{padding:.38rem .48rem;border-bottom:1px solid #1c2934;background:#0e161f}.panel-filter-row input{min-width:0;flex:1}.panel-filter-row button{padding:.26rem .38rem;font-size:.65rem;color:#748697}.panel-filter-row button.sort-active{color:#8bc8ff;border-color:#345064}
  .commander-head,.file-row{display:grid;grid-template-columns:minmax(0,1fr) 6.5rem 10.5rem 8rem;gap:.45rem;align-items:center}.commander-head{padding:.38rem .55rem;background:#111c26;color:#6f8293;font-size:.62rem;font-weight:800;text-transform:uppercase;letter-spacing:.04em;border-bottom:1px solid #22303c}
  .commander-list{height:52vh;min-height:25rem;overflow:auto;padding:.18rem}.file-row{box-sizing:border-box;width:100%;border:0;border-radius:.26rem;padding:.32rem .42rem;background:transparent;text-align:left;color:#cbd5df;font-size:.74rem}.file-row:hover{background:#13232d}.file-row.selected{background:#16513a;color:#eafff4;box-shadow:inset 3px 0 #5de4c7}.entry-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.date-cell{font-size:.67rem;color:#7e91a2}.parent-row{opacity:.75}
  .panel-status{min-height:1.9rem;padding:0 .5rem;border-top:1px solid #1f2c37;background:#0f1821;color:#687b8c;font-size:.61rem}.selected-info{color:#9fb0bf;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .commander-keys{padding:.42rem .5rem;background:#0a1016;border-top:1px solid #263542}.commander-keys button{display:flex;gap:.32rem;align-items:center;padding:.3rem .46rem}.commander-keys kbd{border:0;background:#23313e;color:#8bc8ff;border-radius:.2rem;padding:.08rem .24rem;font:700 .58rem/1 "Roboto Mono",monospace}.key-hint{color:#647586;font-size:.59rem}

  .explorer-shell{display:grid;grid-template-columns:13rem minmax(0,1fr);min-height:36rem;background:#0c131b}.quick-tree{padding:.7rem .55rem;border-right:1px solid #22303c;background:#0e161e;display:flex;flex-direction:column;gap:.18rem}.quick-title{padding:.25rem .45rem .5rem;color:#607384;font-size:.62rem;font-weight:800;text-transform:uppercase;letter-spacing:.08em}.quick-tree button{display:grid;grid-template-columns:1.25rem minmax(0,1fr);gap:.06rem .35rem;border:0;text-align:left;background:transparent;padding:.42rem}.quick-tree button small{grid-column:2;color:#536574;font-size:.56rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.quick-tree button.active{background:#13372c;color:#86eabd}.quick-tree button.active small{color:#699b88}.quick-icon{color:#5de4c7}
  .explorer-main{min-width:0;display:grid;grid-template-rows:auto auto 1fr auto}.explorer-toolbar{padding:.55rem .62rem;border-bottom:1px solid #22303c;background:#101821}.search-box{display:flex;align-items:center;gap:.3rem;padding:0 .42rem;background:#0b1218}.search-box input{border:0;background:transparent;box-shadow:none;padding:.35rem 0;min-width:14rem}
  .breadcrumbs{padding:.42rem .65rem;border-bottom:1px solid #1f2c37;background:#0e161f;color:#637687;overflow:auto;white-space:nowrap}.breadcrumbs button{border:0;background:transparent;padding:.18rem .25rem;color:#91b7d0}.breadcrumbs button:hover{color:#c8ecff;background:#15232d}
  .explorer-table{overflow:auto;max-height:58vh}.explorer-head,.explorer-row{display:grid;grid-template-columns:minmax(16rem,1fr) 6.5rem 11rem 8.5rem 10.5rem;gap:.5rem;align-items:center;padding:.42rem .62rem;border-bottom:1px solid #1b2731}.explorer-head{position:sticky;top:0;z-index:3;background:#111b25;color:#687b8c;font-size:.61rem;font-weight:800;text-transform:uppercase;letter-spacing:.04em}.explorer-head button{border:0;background:transparent;padding:0;text-align:left;color:inherit;font-weight:inherit;text-transform:inherit}.explorer-row{font-size:.73rem;min-height:2.2rem}.explorer-row:hover{background:#101f29}.explorer-row>button{border:0;background:transparent;text-align:left;padding:0}.entry-open{display:flex;align-items:center;gap:.5rem;min-width:0}.entry-open>span:last-child{display:flex;min-width:0;flex-direction:column}.entry-open strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.75rem}.entry-open small{color:#516574;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font: .56rem/1.2 "Roboto Mono",monospace}.file-icon{width:1.2rem;text-align:center}.permission{color:#8bc8ff}.row-actions{display:flex;justify-content:flex-end;gap:.2rem}.row-actions button,.row-actions a{display:grid;place-items:center;width:1.65rem;height:1.65rem;padding:0;text-decoration:none}.explorer-status{padding:.35rem .62rem;border-top:1px solid #22303c;background:#0e161f;color:#607384;font-size:.59rem;min-width:0}.explorer-status span:last-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.empty-row{padding:1rem;color:#566978;text-align:center;font-size:.7rem}.explorer-empty{border-bottom:1px solid #1b2731}


  .volume-select,.tree-volume select{max-width:12rem;background:#0e161e;color:#c9d4df;border:1px solid #30404d;border-radius:.35rem;padding:.34rem .42rem;font:600 .62rem "Roboto Mono",monospace}.tree-volume{padding:.15rem .35rem .65rem;border-bottom:1px solid #22303c;margin-bottom:.45rem}.tree-volume label{display:block;color:#607384;font-size:.55rem;font-weight:800;text-transform:uppercase;letter-spacing:.07em;margin-bottom:.28rem}.tree-volume select{width:100%;max-width:none}
  .folder-tree{overflow:auto;max-height:54vh;padding:.1rem 0}.tree-row{display:flex;align-items:center;min-width:0;border-radius:.3rem}.tree-row:hover{background:rgba(93,228,199,.06)}.tree-row.tree-active{background:#13372c}.tree-toggle{flex:none;width:1.25rem;padding:.25rem 0;border:0;background:transparent;color:#71879a}.tree-name{min-width:0;flex:1;border:0;background:transparent;text-align:left;padding:.32rem .2rem;color:#b9c7d3;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.tree-active .tree-name{color:#86eabd}
  @media(max-width:980px){.commander-head,.file-row{grid-template-columns:minmax(0,1fr) 6rem 7rem}.commander-head span:nth-child(3),.file-row span:nth-child(3){display:none}.explorer-head,.explorer-row{grid-template-columns:minmax(12rem,1fr) 6rem 8rem 9rem}.explorer-head>*:nth-child(3),.explorer-row>*:nth-child(3){display:none}.quick-tree{width:auto}.explorer-shell{grid-template-columns:10.5rem minmax(0,1fr)}}
  @media(max-width:760px){.commander-grid{grid-template-columns:1fr}.commander-panel:not(.active-panel){display:none}.key-hint{display:none}.explorer-shell{grid-template-columns:1fr}.quick-tree{display:none}.explorer-head,.explorer-row{grid-template-columns:minmax(10rem,1fr) 6rem 9rem}.explorer-head>*:nth-child(4),.explorer-row>*:nth-child(4){display:none}.search-box input{min-width:7rem}.commander-keys{overflow:auto}}
</style>
