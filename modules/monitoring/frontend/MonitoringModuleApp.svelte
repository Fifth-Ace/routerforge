<script>
  import { onMount } from 'svelte';
  import { replaceState } from '$app/navigation';
  import { catalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { getModule, preflightEntwarePackage, entwarePackageAction } from '$lib/api.js';
  import { startSerialPolling } from '$lib/polling.js';
  import { createReadSessions } from '$lib/readSessions.js';
  import { bytes, fmtDuration, fmtInt } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
  import EventsPane from './EventsPane.svelte';

  const moduleDefs = [
    { id: 'system', nameKey: 'monitoring.modules.system', short: 'SYS', package: 'routerforge-monitoring' },
    { id: 'processes', nameKey: 'monitoring.modules.processes', short: 'PRC', package: 'routerforge-monitoring' },
    { id: 'thermal', nameKey: 'monitoring.modules.thermal', short: 'TMP', package: 'routerforge-monitoring' },
    { id: 'storage', nameKey: 'monitoring.modules.storage', short: 'DSK', package: 'routerforge-monitoring' },
    { id: 'network', nameKey: 'monitoring.modules.network', short: 'NET', package: 'routerforge-monitoring' },
    { id: 'events', short: 'EVT', package: 'routerforge-monitoring' },
  ];

  let tab = 'system';
  let loading = false;
  let errorText = '';
  let data = {};
  let stopPolling = null;
  let mounted = false;
  const viewReads = createReadSessions();
  let interfaceSort = { key: 'display_name', dir: 'asc' };
  let systemInterfaceSort = { key: 'name', dir: 'asc' };
  let routeSort = { key: 'destination', dir: 'asc' };
  let processSort = { key: 'cpu', dir: 'desc' };
  let processQuery = '';
  let doctorExpanded = '';
  let doctorDrivers = {};
  let doctorBusy = {};
  let doctorSmartByDevice = {};
  let doctorError = {};
  let doctorInstallBusy = false;
  let doctorInstallError = '';
  let doctorInstallNotice = '';
  // The user owns this disclosure state, not the 3-second storage poll.
  let storageTechnicalOpen = false;
  let storageTechnicalInitialized = false;

  $: locale = $settings.locale || 'ru';
  $: advanced = $settings.uiLevel === 'advanced';
  $: modules = $catalog.modules || [];
  $: monitoringModule = modules.find((item) => item.id === 'monitoring');
  $: installedDefs = monitoringModule?.installed ? moduleDefs : [];
  $: currentModule = monitoringModule;
  $: definition = installedDefs.find((item) => item.id === tab) || installedDefs[0] || moduleDefs[0];
  $: visibleInterfaces = sortInterfaces(data.interfaces?.interfaces || [], interfaceSort);
  $: systemInterfaces = sortInterfaces(data.interfaces?.system_interfaces || [], systemInterfaceSort);
  $: sortedRoutes = sortRoutes(data.routes?.routes || [], routeSort);
  $: processItems = data.processes?.processes || [];
  $: filteredProcessItems = filterProcesses(processItems, processQuery);
  $: sortedProcessItems = sortProcesses(filteredProcessItems, processSort);


  $: if (installedDefs.length && !installedDefs.some((item) => item.id === tab)) {
    tab = installedDefs[0].id;
    data = {};
    errorText = '';
    if (typeof window !== 'undefined') {
      Promise.resolve().then(() => {
        if (!mounted) return;
        replaceState(`/monitoring?tab=${encodeURIComponent(tab)}`, {});
        startTimer(true);
      });
    }
  }

  const pct = (n) => Math.max(0, Math.min(100, Number(n || 0)));
  const rate = (n) => {
    const value = Number(n || 0);
    if (value < 1024) return `${value.toFixed(0)} B/s`;
    if (value < 1048576) return `${(value / 1024).toFixed(1)} KB/s`;
    return `${(value / 1048576).toFixed(1)} MB/s`;
  };
  const cpuText = (n) => Number(n || 0) > 0 && Number(n) < 1 ? '<1%' : `${Number(n || 0).toFixed(1)}%`;
  const tempClass = (sensor) => sensor.status === 'critical' ? 'error' : sensor.status === 'warn' ? 'warn' : 'good';
  const moduleName = (item) => item?.id === 'events' ? t(locale, 'monitoring.events.title') : t(locale, item?.nameKey || 'monitoring.title');

  async function loadSystem(signal) {
    const [summary, cpu, memory] = await Promise.all([
      getModule('system', 'summary', signal), getModule('system', 'cpu', signal), getModule('system', 'memory', signal)
    ]);
    return { summary, cpu, memory };
  }

  async function loadProcesses(signal) {
    return { processes: await getModule('system', 'processes', signal) };
  }

  async function loadThermal(signal) {
    const [thermal, storageDoctor] = await Promise.all([
      getModule('thermal', 'sensors', signal),
      getModule('storage', 'doctor', signal).catch(() => null)
    ]);
    return { thermal, storageDoctor };
  }

  async function loadStorage(signal) {
    const storage = await getModule('storage', 'storage', signal);
    let doctor = null;
    try { doctor = await getModule('storage', 'doctor', signal); } catch (_) { /* older backend: keep original Storage working */ }
    return { storage, doctor };
  }

  async function loadNetwork(signal) {
    const [summary, interfaces, routes] = await Promise.all([
      getModule('network', 'summary', signal), getModule('network', 'interfaces', signal), getModule('network', 'routes', signal)
    ]);
    return { summary, interfaces, routes };
  }

  async function loadProfiling(signal) {
    return { profiling: await getModule('profiling', 'status', signal) };
  }

  async function loadCurrent(showLoading = true) {
    const currentTab = tab;
    if (currentTab === 'events') {
      viewReads.cancel('active-tab');
      loading = false; errorText = ''; data = {};
      return;
    }
    const read = viewReads.begin('active-tab');
    if (showLoading) loading = true;
    try {
      let result = {};
      if (currentTab === 'system') result = await loadSystem(read.signal);
      if (currentTab === 'processes') result = await loadProcesses(read.signal);
      if (currentTab === 'thermal') result = await loadThermal(read.signal);
      if (currentTab === 'storage') result = await loadStorage(read.signal);
      if (currentTab === 'network') result = await loadNetwork(read.signal);
      if (currentTab === 'profiling') result = await loadProfiling(read.signal);
      if (!read.isCurrent() || tab !== currentTab) return;
      // Preserve manual open/close across subsequent polls and tab changes.
      if (currentTab === 'storage' && !storageTechnicalInitialized) {
        storageTechnicalOpen = !result.doctor;
        storageTechnicalInitialized = true;
      }
      data = result;
      errorText = '';
    } catch (error) {
      if (!read.isCurrent() || tab !== currentTab) return;
      data = {};
      errorText = error?.payload?.error || error?.message || t(locale, 'common.unavailable');
    } finally {
      if (read.isCurrent() && showLoading) loading = false;
      read.finish();
    }
  }

  function startTimer(showLoading = false) {
    if (tab === 'events') { if (stopPolling) stopPolling(); stopPolling = null; viewReads.cancel('active-tab'); loading = false; errorText = ''; return; }
    if (stopPolling) stopPolling();
    viewReads.cancel('active-tab');
    const interval = tab === 'processes' ? 1000 : tab === 'thermal' ? 10000 : tab === 'profiling' ? 5000 : 3000;
    let first = true;
    stopPolling = startSerialPolling(() => {
      if (document.hidden) return;
      const currentShowLoading = first && showLoading;
      first = false;
      return loadCurrent(currentShowLoading);
    }, interval);
  }

  function selectTab(id) {
    if (tab === id) return;
    tab = id;
    replaceState(`/monitoring?tab=${encodeURIComponent(id)}`, {});
    startTimer(true);
  }

  function nextSort(current, key) {
    if (current.key === key) return { key, dir: current.dir === 'asc' ? 'desc' : 'asc' };
    return { key, dir: 'asc' };
  }

  function sortArrow(sort, key) {
    if (sort.key !== key) return '↕';
    return sort.dir === 'asc' ? '▲' : '▼';
  }

  function compareValues(left, right, dir) {
    const factor = dir === 'desc' ? -1 : 1;
    if (typeof left === 'number' || typeof right === 'number') {
      return (Number(left || 0) - Number(right || 0)) * factor;
    }
    return String(left ?? '').localeCompare(String(right ?? ''), locale === 'ru' ? 'ru' : 'en', { numeric: true, sensitivity: 'base' }) * factor;
  }

  function interfaceSortValue(item, key) {
    if (key === 'display_name') return item.display_name || item.name || '';
    if (key === 'name') return item.name || '';
    if (key === 'state') return item.logical_state || item.oper_state || '';
    if (key === 'address') return item.primary_address || '';
    if (key === 'speed') return Number(item.speed_mbps || 0);
    if (key === 'rx') return Number(item.rate?.rx_bps || 0);
    if (key === 'tx') return Number(item.rate?.tx_bps || 0);
    if (key === 'errors') return Number(item.rx_errors || 0) + Number(item.tx_errors || 0);
    if (key === 'drops') return Number(item.rx_drops || 0) + Number(item.tx_drops || 0);
    return item.name || '';
  }

  function sortInterfaces(items, sort) {
    return [...items].sort((a, b) => {
      const primary = compareValues(interfaceSortValue(a, sort.key), interfaceSortValue(b, sort.key), sort.dir);
      return primary || compareValues(a.display_name || a.name, b.display_name || b.name, 'asc');
    });
  }

  function routeSortValue(item, key) {
    if (key === 'metric') return Number(item.metric || 0);
    return item[key] || '';
  }

  function sortRoutes(items, sort) {
    return [...items].sort((a, b) => {
      const primary = compareValues(routeSortValue(a, sort.key), routeSortValue(b, sort.key), sort.dir);
      return primary || compareValues(a.interface, b.interface, 'asc');
    });
  }

  function processSortValue(item, key) {
    if (key === 'cpu') return Number(item.cpu_pct || 0);
    if (key === 'memory') return Number(item.rss_kb || 0);
    if (key === 'pid') return Number(item.pid || 0);
    if (key === 'ppid') return Number(item.ppid || 0);
    if (key === 'threads') return Number(item.threads || 0);
    if (key === 'user') return item.user || '';
    if (key === 'state') return item.state || '';
    return item.name || item.command || '';
  }

  function filterProcesses(items, query) {
    const needle = String(query || '').trim().toLowerCase();
    if (!needle) return [...items];
    return items.filter((item) => [item.name, item.command, item.user, item.state, item.pid, item.ppid]
      .some((value) => String(value ?? '').toLowerCase().includes(needle)));
  }

  function sortProcesses(items, sort) {
    return [...items].sort((a, b) => {
      const primary = compareValues(processSortValue(a, sort.key), processSortValue(b, sort.key), sort.dir);
      return primary || compareValues(a.pid, b.pid, 'asc');
    });
  }

  function processStateName(item) {
    const key = ({ R: 'running', S: 'sleeping', D: 'diskSleep', T: 'stopped', t: 'tracing', Z: 'zombie', I: 'idle' })[String(item.state || '')] || 'unknown';
    return t(locale, `monitoring.processes.states.${key}`);
  }

  function processStateClass(item) {
    const state = String(item.state || '');
    if (state === 'R') return 'good';
    if (state === 'D' || state === 'Z') return 'warn';
    return 'neutral';
  }

  function interfaceState(item) {
    const value = String(item.logical_state || item.oper_state || 'unknown').toLowerCase();
    if (value === 'up') return t(locale, 'common.up');
    if (value === 'down') return t(locale, 'common.down');
    return t(locale, 'common.unknown');
  }

  function interfaceStateClass(item) {
    const value = String(item.logical_state || item.oper_state || '').toLowerCase();
    return value === 'down' ? 'neutral' : 'good';
  }

  function thermalName(sensor) {
    const role = sensor.role || 'other';
    const index = Number(sensor.sensor_index || 0) + 1;
    return t(locale, `monitoring.thermal.roles.${role}`, {
      index,
      detail: sensor.detail || index,
      name: sensor.name || role
    });
  }

  function thermalCategory(sensor) {
    return t(locale, `monitoring.thermal.categories.${sensor.category || 'other'}`);
  }

  function doctorTr(ru,en) { return locale === 'ru' ? ru : en; }
  function doctorSmartStatus(s) {
    if(!s) return doctorTr('Ещё не проверялось','Not checked yet');
    const values = {available:doctorTr('Данные получены','Data available'),asleep:doctorTr('Диск находится в режиме сна','Drive is asleep'),unsupported:doctorTr('Устройство не поддерживает проверку','Device does not support the check'),unavailable:doctorTr('Данные недоступны','Data unavailable'),busy:doctorTr('Диск не ответил вовремя','Drive did not respond'),error:doctorTr('Проверка не удалась','Check failed')};
    if(s.status!=='available') return values[s.status] || doctorTr('Результат недоступен','Result unavailable');
    if(s.health==='ok') return doctorTr('Критических признаков не найдено','No critical indicators found');
    if(s.health==='warning') return doctorTr('Есть признаки возможных проблем','Possible issues detected');
    if(s.health==='critical') return doctorTr('Обнаружены серьёзные проблемы','Serious problems detected');
    return doctorTr('Состояние не определено','Health unknown');
  }
  function doctorHistoryName(kind) {
    const names = {
      usb_disconnect: doctorTr('USB отключился','USB disconnected'),
      usb_reset: doctorTr('Перезапуск USB','USB reset'),
      io_error: doctorTr('Ошибка чтения или записи','I/O error'),
      fs_error: doctorTr('Ошибка файловой системы','Filesystem error'),
      read_only: doctorTr('Диск перешёл в режим чтения','Filesystem read-only'),
      disappeared: doctorTr('Накопитель отключился','Mount disappeared'),
      appeared: doctorTr('Накопитель подключился','Mount appeared'),
      writable_again: doctorTr('Запись восстановлена','Writable again'),
      overheat: doctorTr('Высокая температура','Overtemperature'),
      cooled: doctorTr('Температура нормализовалась','Temperature returned to normal'),
      smart_degradation: doctorTr('Ухудшение SMART','SMART degradation'),
      smart_critical: doctorTr('SMART: критическое состояние','SMART critical')
    };
    return names[kind] || kind;
  }
  function doctorEventName(type) {
    const names={appeared:doctorTr('Подключён','Mounted'),disappeared:doctorTr('Отключён','Unmounted'),read_only:doctorTr('Перешёл в только чтение','Became read-only'),writable_again:doctorTr('Запись восстановлена','Writable again')};
    return names[type] || type;
  }
  async function doctorInstallSmart() {
    if (doctorInstallBusy || data.doctor?.smartctl_installed) return;
    doctorInstallBusy = true;
    doctorInstallError = '';
    doctorInstallNotice = '';
    try {
      const plan = await preflightEntwarePackage('smartmontools', 'install');
      if (!plan?.allowed) throw new Error(plan?.reason || doctorTr('Установка недоступна через Центр приложений.','Installation is not available through App Center.'));
      if (!confirm(doctorTr('Установить дополнительный пакет smartmontools из Entware? Изменится список пакетов, но диски не будут проверяться или изменяться автоматически.','Install the optional smartmontools Entware package? Installed packages will change, but no drive scans or modifications will start automatically.'))) return;
      await entwarePackageAction('smartmontools', 'install');
      doctorInstallNotice = doctorTr('Запрос передан Центру приложений. Статус установки обновится автоматически.','The request was sent to App Center. Installation status will update automatically.');
      await loadCurrent(false);
    } catch (error) {
      doctorInstallError = error?.payload?.error || error?.message || doctorTr('Не удалось выполнить установку.','Installation failed.');
    } finally {
      doctorInstallBusy = false;
    }
  }
  async function doctorCheckSmart(device) {
    if(doctorBusy[device]) return;
    doctorBusy={...doctorBusy,[device]:true};doctorError={...doctorError,[device]:''};
    const name=String(device).replace(/^\/dev\//,'');
    const driver=doctorDrivers[device]||'auto';
    const ctl = new AbortController();
    const timeout = setTimeout(()=>ctl.abort(),12000);
    try {
      const url=`/api/modules/storage/doctor-smart?device=${encodeURIComponent(name)}&driver=${encodeURIComponent(driver)}`;
      const response=await fetch(url,{credentials:'same-origin',cache:'no-store',signal:ctl.signal});
      const result=await response.json();
      if(!response.ok) throw new Error(result.error||doctorTr('Не удалось проверить SMART','SMART check failed'));
      doctorSmartByDevice={...doctorSmartByDevice,[device]:result};doctorExpanded=device;
    } catch(error){doctorError={...doctorError,[device]:error?.message||String(error)};}
    finally{clearTimeout(timeout);doctorBusy={...doctorBusy,[device]:false};}
  }

  function storageMountName(item) {
    if (item.mount === '/opt') return t(locale, 'monitoring.storage.entware');
    if (item.mount === '/storage') return t(locale, 'monitoring.storage.internal');
    return item.mount;
  }

  function storageType(disk) {
    const transport = String(disk.transport || '').toLowerCase();
    if (transport === 'usb') return t(locale, 'monitoring.storage.usb');
    if (transport === 'nvme') return t(locale, 'monitoring.storage.nvme');
    if (transport === 'mmc') return t(locale, 'monitoring.storage.mmc');
    if (transport === 'ata') return t(locale, 'monitoring.storage.ata');
    return t(locale, 'monitoring.storage.storageDevice');
  }

  onMount(() => {
    mounted = true;
    const params = new URLSearchParams(location.search);
    const initial = params.get('view') || params.get('tab');
    if (initial && moduleDefs.some((item) => item.id === initial)) tab = initial;
    startTimer(true);
    return () => {
      mounted = false;
      if (stopPolling) stopPolling();
      viewReads.dispose();
    };
  });
</script>

<svelte:head><title>RouterForge — {t(locale, 'monitoring.title')}</title></svelte:head>

<div class="page modules-page">
  <div class="page-head">
    <div>
      <h1>{t(locale, 'monitoring.title')}</h1>
      <p>{t(locale, 'monitoring.subtitle')}</p>
    </div>
    <span class="state-chip {currentModule?.service_running ? 'good' : currentModule?.installed ? 'warn' : 'info'}">
      {currentModule?.service_running ? t(locale, 'monitoring.moduleOnline') : currentModule?.installed ? t(locale, 'monitoring.installedOffline') : t(locale, 'monitoring.optional')}
    </span>
  </div>

  <div class="module-selector">
    {#each installedDefs as item}
      {@const catalogItem = monitoringModule}
      <button class:active={tab === item.id} onclick={() => selectTab(item.id)}>
        <span class="module-selector-icon mono">{item.short}</span>
        <span><strong>{moduleName(item)}</strong><small>{catalogItem?.service_running ? t(locale, 'common.online') : catalogItem?.installed ? t(locale, 'common.installed') : t(locale, 'common.optional')}</small></span>
        <i class="status-dot {catalogItem?.service_running ? 'good' : catalogItem?.installed ? 'warn' : 'muted'}"></i>
      </button>
    {/each}
  </div>

  {#if errorText}
    <section class="panel module-offline">
      <div class="panel-head">
        <div><strong>{t(locale, 'monitoring.unavailableTitle', { name: moduleName(definition) })}</strong><span>{t(locale, 'monitoring.unavailableHint')}</span></div>
        <span class="state-chip info">{t(locale, 'monitoring.optionalIpk')}</span>
      </div>
      <div class="module-install-hint">
        <p>{errorText}</p>
        <code>opkg install {definition.package}</code>
        <span>{t(locale, 'monitoring.installHint')}</span>
      </div>
    </section>
  {:else if loading}
    <section class="panel"><div class="empty">{t(locale, 'monitoring.loading', { name: moduleName(definition) })}</div></section>

  {:else if tab === 'events'}
    <EventsPane />

  {:else if tab === 'system' && data.summary}
    <section class="metric-grid module-metric-grid">
      <div class="metric-card"><span>{t(locale, 'monitoring.system.host')}</span><strong>{data.summary.hostname || '—'}</strong><small>{data.summary.kernel || '—'} · {data.summary.architecture || '—'}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.system.load')}</span><strong>{Number(data.summary.load_1 || 0).toFixed(2)}</strong><small>{Number(data.summary.load_5 || 0).toFixed(2)} · {Number(data.summary.load_15 || 0).toFixed(2)}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.system.ram')}</span><strong>{Number(data.memory?.used_pct || 0).toFixed(1)}%</strong><small>{bytes(Number(data.memory?.used_kb || 0) * 1024)} / {bytes(Number(data.memory?.total_kb || 0) * 1024)}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.system.uptime')}</span><strong>{fmtDuration(data.summary.uptime_seconds || 0, locale)}</strong><small>{t(locale, 'monitoring.system.coresProcesses', { cores: data.summary.cpu_count || 0, processes: data.summary.process_count || 0 })}</small></div>
    </section>

    <div class="two-col">
      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'monitoring.system.cpuByCore')}</strong><span>{t(locale, 'monitoring.system.rolling', { seconds: data.cpu?.window_seconds || 5 })}</span></div><span class="state-chip {data.cpu?.ready ? 'good' : 'info'}">{data.cpu?.ready ? t(locale, 'common.live') : t(locale, 'common.sampling')}</span></div>
        <div class="module-cpu-list">
          {#each data.cpu?.cpus || [] as core (core.name)}
            <div class="module-cpu-row">
              <div><strong>{core.name}</strong><span>{cpuText(core.usage_pct)}</span></div>
              <div class="progress"><span style={`width:${Math.max(core.usage_pct > 0 ? 1 : 0, pct(core.usage_pct))}%`}></span></div>
            </div>
          {/each}
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'monitoring.system.memory')}</strong><span>/proc/meminfo</span></div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.system.available')}</strong><span>{t(locale, 'monitoring.system.availableHint')}</span></div><div class="info-value good">{bytes(Number(data.memory?.available_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.system.cached')}</strong><span>{t(locale, 'monitoring.system.cachedHint')}</span></div><div class="info-value">{bytes(Number(data.memory?.cached_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.system.swap')}</strong><span>{t(locale, 'monitoring.system.swapHint')}</span></div><div class="info-value">{bytes((Number(data.memory?.swap_total_kb || 0) - Number(data.memory?.swap_free_kb || 0)) * 1024)} / {bytes(Number(data.memory?.swap_total_kb || 0) * 1024)}</div></div>
      </section>
    </div>

  {:else if tab === 'processes' && data.processes}
        <section class="metric-grid module-metric-grid process-metric-grid">
      <div class="metric-card"><span>{t(locale, 'monitoring.processes.total')}</span><strong>{fmtInt(data.processes.total || processItems.length)}</strong><small>{t(locale, 'monitoring.processes.visible', { visible: fmtInt(filteredProcessItems.length), total: fmtInt(data.processes.total || processItems.length) })}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.processes.refresh')}</span><strong>{Number(data.processes.interval_ms || 1000) / 1000}s</strong><small>{t(locale, 'monitoring.processes.cpuHint')}</small></div>
    </section>

    <section class="panel table-panel process-manager-panel">
      <div class="panel-head process-manager-head">
        <div><strong>{t(locale, 'monitoring.processes.title')}</strong><span>{t(locale, 'monitoring.processes.subtitle')}</span></div>
        <div class="process-manager-toolbar">
          <input class="process-search" type="search" value={processQuery} oninput={(event) => processQuery = event.currentTarget.value} placeholder={t(locale, 'monitoring.processes.search')} aria-label={t(locale, 'monitoring.processes.search')} />
          <span class="state-chip {data.processes.ready ? 'good' : 'info'}">{data.processes.ready ? t(locale, 'common.live') : t(locale, 'common.sampling')}</span>
        </div>
      </div>

      {#if sortedProcessItems.length}
        <div class="table-scroll process-table-scroll"><table class="sortable-table process-table"><thead><tr>
          <th><button onclick={() => processSort = nextSort(processSort, 'name')}>{t(locale, 'monitoring.processes.columns.process')} <span>{sortArrow(processSort, 'name')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'pid')}>PID <span>{sortArrow(processSort, 'pid')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'user')}>{t(locale, 'monitoring.processes.columns.user')} <span>{sortArrow(processSort, 'user')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'state')}>{t(locale, 'monitoring.processes.columns.state')} <span>{sortArrow(processSort, 'state')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'cpu')}>CPU <span>{sortArrow(processSort, 'cpu')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'memory')}>{t(locale, 'monitoring.processes.columns.ram')} <span>{sortArrow(processSort, 'memory')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'threads')}>{t(locale, 'monitoring.processes.columns.threads')} <span>{sortArrow(processSort, 'threads')}</span></button></th>
          <th><button onclick={() => processSort = nextSort(processSort, 'ppid')}>PPID <span>{sortArrow(processSort, 'ppid')}</span></button></th>
        </tr></thead><tbody>
          {#each sortedProcessItems as proc (proc.pid)}
            <tr>
              <td><strong>{proc.name || '—'}</strong><div class="cell-sub mono process-command" title={proc.command}>{proc.command || '—'}</div></td>
              <td class="mono">{proc.pid}</td>
              <td>{proc.user || proc.uid}</td>
              <td><span class="state-chip {processStateClass(proc)}" title={proc.state}>{processStateName(proc)}</span></td>
              <td><div class="process-usage"><div><strong>{cpuText(proc.cpu_pct)}</strong></div><div class="progress"><span style={`width:${pct(proc.cpu_pct)}%`}></span></div></div></td>
              <td><div class="process-usage"><div><strong>{bytes(Number(proc.rss_kb || 0) * 1024)}</strong><span>{Number(proc.memory_pct || 0).toFixed(1)}%</span></div><div class="progress"><span style={`width:${Math.max(proc.memory_pct > 0 ? 1 : 0, pct(proc.memory_pct))}%`}></span></div></div></td>
              <td class="mono">{fmtInt(proc.threads || 0)}</td>
              <td class="mono">{proc.ppid || 0}</td>
            </tr>
          {/each}
        </tbody></table></div>
      {:else}
        <div class="empty">{t(locale, 'monitoring.processes.none')}</div>
      {/if}
    </section>

  {:else if tab === 'thermal' && data.thermal}
    <div class="module-meta-strip mono">
      <span>{t(locale, 'monitoring.thermal.sensors')} <strong>{data.thermal.sensor_count || 0}</strong></span>
      <span>{t(locale, 'monitoring.thermal.cache')} <strong>{data.thermal.cache_seconds || 0}s</strong></span>
      <span>{t(locale, 'monitoring.thermal.smartctl')} <strong class={data.thermal.optional_smartctl ? 'good' : 'muted'}>{data.thermal.optional_smartctl ? t(locale, 'common.available') : t(locale, 'common.optional')}</strong></span>
    </div>
    {#if data.storageDoctor}
      <section class="panel rf-thermal-disks">
        <div class="panel-head"><div><strong>{doctorTr('Температура накопителей','Drive temperatures')}</strong><span>{doctorTr('Отдельно для каждого диска. Показания из датчика устройства или последней ручной проверки SMART.','Per-drive temperature from device sensors or the last manual SMART check.')}</span></div></div>
        {#if data.storageDoctor.disks?.length}
          <div class="rf-thermal-disk-grid">
            {#each data.storageDoctor.disks as disk (disk.device)}
              {@const measurement = data.storageDoctor.temperatures?.[disk.device]}
              {@const previous = (data.storageDoctor.temperature_history?.[disk.device] || []).filter((item) => new Date(item.at).getTime() > Date.now() - 86400000)}
              <article class="rf-thermal-disk-card">
                <div class="rf-thermal-disk-header"><strong>{disk.model || disk.vendor || disk.device}</strong><small>{disk.device} · {storageType(disk)}</small></div>
                <div class="rf-thermal-disk-reading" class:rf-thermal-hot={measurement && measurement.celsius >= 65}>
                  {#if measurement}<strong>{Number(measurement.celsius).toFixed(1)} °C</strong>{:else}<strong>— °C</strong>{/if}
                  <span>{!measurement ? doctorTr('Нет доступного датчика','No temperature sensor available') : measurement.celsius >= 75 ? doctorTr('Сильный нагрев','High temperature') : measurement.celsius >= 65 ? doctorTr('Повышенная температура','Elevated temperature') : doctorTr('Температура получена','Temperature available')}</span>
                </div>
                {#if measurement}<small>{measurement.source} · {new Date(measurement.at).toLocaleString(locale)}</small>{/if}
                {#if previous.length > 1}
                  <small>{doctorTr('За последние 24 часа','Last 24 hours')}: {Math.min(...previous.map((p) => p.celsius)).toFixed(1)}–{Math.max(...previous.map((p) => p.celsius)).toFixed(1)} °C · {previous.length} {doctorTr('измерений','readings')}</small>
                {:else}<small>{doctorTr('История появится после получения нескольких измерений.','History will appear after several readings.')}</small>{/if}
                {#if !measurement && data.storageDoctor.smartctl_installed}
                  <button type="button" class="button" disabled={!!doctorBusy[disk.device]} onclick={() => doctorCheckSmart(disk.device)}>{doctorBusy[disk.device] ? doctorTr('Проверяем…','Checking…') : doctorTr('Проверить температуру вручную','Read temperature manually')}</button>
                {:else if !measurement}<small>{doctorTr('Обычная USB-флешка может не иметь температурного датчика.','Ordinary USB flash drives may not provide temperature data.')}</small>{/if}
                {#if doctorError[disk.device]}<small class="rf-doctor-bad">{doctorError[disk.device]}</small>{/if}
              </article>
            {/each}
          </div>
          <p class="rf-doctor-note">{doctorTr('Автоматический опрос SMART отключён: спящие HDD не пробуждаются ради температуры. Для SMART-измерения нажми кнопку вручную.','Automatic SMART polling is disabled: sleeping HDDs are not woken up for temperature. Use the manual check for SMART readings.')}</p>
        {:else}<p class="rf-doctor-note">{doctorTr('Диски не найдены.','No drives detected.')}</p>{/if}
      </section>
    {/if}
    <section class="module-thermal-grid">
      {#if (data.thermal.sensors || []).length}
        {#each data.thermal.sensors.filter((sensor) => !data.storageDoctor || sensor.category !== "storage") as sensor (sensor.id)}
          <article class="module-thermal-card">
            <div class="module-thermal-head"><span class="status-dot {tempClass(sensor)}"></span><div><strong>{thermalName(sensor)}</strong><span>{thermalCategory(sensor)}</span></div></div>
            <div class="module-temp {tempClass(sensor)}">{Number(sensor.temp_c).toFixed(1)}°C</div>
            <div class="module-temp-scale"><span style={`width:${Math.min(100, Number(sensor.temp_c || 0) / Number(sensor.critical_c || 100) * 100)}%`}></span></div>
            <div class="module-thermal-foot mono"><span>{t(locale, 'monitoring.thermal.warning')} {sensor.warn_c}°</span><span>{t(locale, 'monitoring.thermal.critical')} {sensor.critical_c}°</span></div>
            {#if advanced}<code title={sensor.source}>{sensor.source}</code>{/if}
          </article>
        {/each}
      {:else}
        <section class="panel"><div class="empty">{t(locale, 'monitoring.thermal.none')}</div></section>
      {/if}
    </section>

  {:else if tab === 'storage' && data.storage}
    {#if data.doctor}
      <section class="panel rf-doctor-shell rf-doctor-friendly">
        <div class="rf-doctor-intro">
          <div>
            <strong>{doctorTr('Ваши накопители','Your drives')}</strong>
            <p>{doctorTr('Здесь видно, подключены ли диски, хватает ли на них места и работает ли Entware.','See whether drives are connected, how much space is free and whether Entware is working.')}</p>
          </div>
          <span class="rf-doctor-count">{data.doctor.disks?.length || 0} {doctorTr('шт.','drives')}</span>
        </div>
        <div class="rf-doctor-health" class:rf-doctor-health-warn={!data.doctor.entware.mounted || !data.doctor.entware.writable || !data.doctor.entware.init_present || !data.doctor.entware.package_db_present}>
          <div class="rf-doctor-health-icon">{data.doctor.entware.mounted && data.doctor.entware.writable && data.doctor.entware.init_present && data.doctor.entware.package_db_present ? '✓' : '!'}</div>
          <div class="rf-doctor-health-copy">
            <strong>{data.doctor.entware.mounted ? (data.doctor.entware.writable && data.doctor.entware.init_present && data.doctor.entware.package_db_present ? doctorTr('Entware работает','Entware is available') : doctorTr('Entware требует внимания','Entware needs attention')) : doctorTr('Накопитель Entware не подключён','Entware drive is not connected')}</strong>
            <span>{!data.doctor.entware.mounted ? doctorTr('Проверь подключение USB-накопителя с программами роутера.','Check the USB drive containing the router applications.') : !data.doctor.entware.writable ? doctorTr('Файловая система доступна только для чтения. Новые данные могут не сохраняться.','The filesystem is read-only. New data may not be saved.') : !data.doctor.entware.init_present || !data.doctor.entware.package_db_present ? doctorTr('Диск подключён, но часть файлов Entware не найдена. Подробности — ниже.','The disk is connected, but some Entware files are missing. See details below.') : doctorTr('Диск с программами подключён и доступен для записи. Это не оценка его физического здоровья.','The application drive is connected and writable. This is not a hardware health assessment.')}</span>
          </div>
          <span class="rf-doctor-pill" class:rf-doctor-pill-warn={!data.doctor.entware.mounted || !data.doctor.entware.writable || !data.doctor.entware.init_present || !data.doctor.entware.package_db_present}>{data.doctor.entware.mounted && data.doctor.entware.writable && data.doctor.entware.init_present && data.doctor.entware.package_db_present ? doctorTr('Доступен','Available') : doctorTr('Внимание','Attention')}</span>
        </div>

        {#if !data.doctor.smartctl_installed}
          <div class="rf-doctor-install" aria-label={doctorTr('Дополнительная проверка дисков','Optional drive health check')}>
            <div class="rf-doctor-install-copy">
              <strong>{doctorTr('Хотите проверить внутреннее состояние диска?','Want to check a drive’s internal health?')}</strong>
              <p>{doctorTr('Установите необязательный инструмент smartmontools. Он помогает прочитать сведения о состоянии HDD и SSD. У обычных USB-флешек эти данные могут отсутствовать.','Install the optional smartmontools utility to read HDD and SSD health data. Ordinary USB flash drives may not provide this information.')}</p>
              <small>{doctorTr('Без него объём, подключение и состояние Entware уже отображаются.','Disk space, connection and Entware status work without it.')}</small>
            </div>
            <button type="button" class="button primary rf-doctor-install-button" disabled={doctorInstallBusy} onclick={doctorInstallSmart}>
              {doctorInstallBusy ? doctorTr('Проверяем установку…','Checking installation…') : doctorTr('Установить проверку дисков','Install drive health check')}
            </button>
          </div>
        {:else}
          <div class="rf-doctor-ready"><span aria-hidden="true">✓</span> {doctorTr('Дополнительная проверка дисков доступна. Запускается только по вашему нажатию.','Advanced drive health checks are available and run only when requested.')}</div>
        {/if}
        {#if doctorInstallNotice}<p class="rf-doctor-feedback" role="status">{doctorInstallNotice}</p>{/if}
        {#if doctorInstallError}<p class="rf-doctor-warning" role="alert">{doctorInstallError}</p>{/if}

        {#if (data.doctor.disks || []).length}
          <div class="rf-doctor-grid">
            {#each data.doctor.disks as disk (disk.device)}
              {@const primaryMount = disk.mounts?.find(m => m.mount === '/opt') || disk.mounts?.[0]}
              <article class="rf-doctor-disk rf-doctor-drive">
                <div class="rf-doctor-drive-head">
                  <div class="rf-doctor-drive-title">
                    <span class="rf-doctor-drive-icon" aria-hidden="true">▣</span>
                    <div>
                      <strong>{disk.model || disk.vendor || doctorTr('Подключённый накопитель','Connected drive')}</strong>
                      <small>{storageType(disk)}{disk.entware ? ` · ${doctorTr('Здесь установлена Entware','Contains Entware')}` : ''}</small>
                    </div>
                  </div>
                  <strong class="rf-doctor-drive-size">{bytes(disk.size_bytes)}</strong>
                </div>
                <div class="rf-doctor-drive-state" class:rf-doctor-drive-danger={disk.read_only || disk.mounts?.some(m=>m.read_only) || !disk.mounts?.length}>
                  <span class="rf-doctor-state-dot"></span>
                  {disk.mounts?.length ? (disk.read_only || disk.mounts.some(m=>m.read_only) ? doctorTr('Подключён, но запись ограничена','Connected, write access limited') : doctorTr('Подключён и доступен','Connected and available')) : doctorTr('Не смонтирован','Not mounted')}
                </div>
                {#if primaryMount}
                  <div class="rf-doctor-space-label"><span>{doctorTr('Занято','Used')} {Number(primaryMount.used_pct || 0).toFixed(1)}%</span><strong>{doctorTr('Свободно','Free')} {bytes(primaryMount.available_bytes)}</strong></div>
                  <div class="rf-doctor-space-track" role="progressbar" aria-label={doctorTr('Заполнение диска','Drive usage')} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(pct(primaryMount.used_pct))}><span style={`width:${pct(primaryMount.used_pct)}%`}></span></div>
                {:else}
                  <p class="rf-doctor-note">{doctorTr('Нет подключённых разделов: занятое место пока неизвестно.','No mounted partitions: used space is unavailable.')}</p>
                {/if}
                <div class="rf-doctor-smart-summary">
                  <span>{doctorTr('Проверка состояния устройства','Hardware health check')}</span>
                  <strong class:rf-doctor-good={doctorSmartByDevice[disk.device]?.health==='ok'} class:rf-doctor-bad={doctorSmartByDevice[disk.device]?.health==='critical'}>
                    {data.doctor.smartctl_installed ? doctorSmartStatus(doctorSmartByDevice[disk.device]) : doctorTr('Нужна установка дополнения','Optional utility not installed')}
                  </strong>
                </div>
                <div class="rf-doctor-controls">
                  {#if data.doctor.smartctl_installed}
                    <button type="button" class="button" disabled={!!doctorBusy[disk.device]} onclick={()=>doctorCheckSmart(disk.device)}>{doctorBusy[disk.device] ? doctorTr('Проверяем…','Checking…') : doctorTr('Проверить диск','Check drive')}</button>
                  {/if}
                  <button type="button" class="button" aria-expanded={doctorExpanded===disk.device} onclick={()=>doctorExpanded=doctorExpanded===disk.device ? '' : disk.device}>{doctorExpanded===disk.device ? doctorTr('Скрыть детали','Hide details') : doctorTr('Подробнее о диске','Drive details')}</button>
                </div>
                {#if doctorError[disk.device]}<p class="rf-doctor-warning" role="alert">{doctorError[disk.device]}</p>{/if}
                {#if doctorExpanded===disk.device}
                  <div class="rf-doctor-details">
                    <strong>{doctorTr('Сведения о диске','Drive information')}</strong>
                    <div class="rf-doctor-mount"><span>{doctorTr('Устройство','Device')}</span><strong>{disk.device}</strong></div>
                    <div class="rf-doctor-mount"><span>{doctorTr('Разделы','Partitions')}</span><strong>{disk.partitions?.join(', ') || '—'}</strong></div>
                    {#each disk.mounts || [] as mount (`detail:${mount.mount}`)}
                      <div class="rf-doctor-mount"><span>{mount.mount} · {mount.fs_type}</span><strong>{mount.read_only ? doctorTr('Только чтение','Read-only') : doctorTr('Чтение и запись','Read/write')} · {mount.inodes_total != null ? `${mount.inodes_free}/${mount.inodes_total} inode` : doctorTr('Нет данных inode','No inode data')}</strong></div>
                    {/each}
                    {#if data.doctor.smartctl_installed}
                      <details class="rf-doctor-bridge"><summary>{doctorTr('Если диск не проходит проверку: настройки совместимости','Compatibility settings (if the check fails)')}</summary>
                        <label>{doctorTr('Тип подключения USB / SMART','USB / SMART device type')}
                          <select value={doctorDrivers[disk.device] || 'auto'} onchange={(event)=>{doctorDrivers={...doctorDrivers,[disk.device]:event.currentTarget.value};}}>
                            {#each ['auto','sat','sat,12','scsi','nvme','ata','usbjmicron','usbcypress','usbsunplus','sntjmicron','sntasmedia','sntrealtek'] as driver}<option value={driver}>{driver}</option>{/each}
                          </select>
                        </label>
                        <small>{doctorTr('По умолчанию используется автоматический режим. Меняйте его только если проверка не дала результата.','Automatic mode is recommended. Change only if a check could not get results.')}</small>
                      </details>
                    {/if}
                    {#if doctorSmartByDevice[disk.device]}
                      {@const smart = doctorSmartByDevice[disk.device]}
                      <strong>{doctorTr('Результат проверки','Health check result')}</strong>
                      <div class="rf-doctor-mount"><span>{doctorTr('Проверено','Checked')}</span><strong>{smart.checked_at ? new Date(smart.checked_at).toLocaleString(locale) : '—'}{smart.cached ? doctorTr(' · из кэша',' · cached') : ''}</strong></div>
                      <div class="rf-doctor-mount"><span>{doctorTr('Температура','Temperature')}</span><strong>{smart.temperature_c == null ? doctorTr('Нет данных','Unavailable') : `${smart.temperature_c} °C`}</strong></div>
                      <div class="rf-doctor-mount"><span>{doctorTr('Время работы','Power-on time')}</span><strong>{smart.power_hours == null ? doctorTr('Нет данных','Unavailable') : `${smart.power_hours} ${doctorTr('ч','h')}`}</strong></div>
                      {#if smart.note && locale !== 'ru'}<small>{smart.note}</small>{/if}
                      {#if smart.attributes?.length}
                        <details class="rf-doctor-bridge"><summary>{doctorTr('Технические показатели SMART','Technical SMART attributes')} ({smart.attributes.length})</summary>
                          <div class="table-scroll"><table><thead><tr><th>ID</th><th>{doctorTr('Показатель','Attribute')}</th><th>Value</th><th>Worst</th><th>Thresh</th><th>Raw</th></tr></thead><tbody>
                            {#each smart.attributes as attr (attr.id)}<tr><td>{attr.id}</td><td>{attr.name}</td><td>{attr.value ?? '—'}</td><td>{attr.worst ?? '—'}</td><td>{attr.thresh ?? '—'}</td><td>{attr.raw?.value == null ? '—' : String(attr.raw.value)}</td></tr>{/each}
                          </tbody></table></div>
                        </details>
                      {/if}
                      {#if smart.nvme && Object.keys(smart.nvme).length}
                        <details class="rf-doctor-bridge"><summary>{doctorTr('Показатели NVMe','NVMe data')}</summary>
                          {#each Object.entries(smart.nvme) as [key,value] (key)}<div class="rf-doctor-mount"><span>{key.replaceAll('_',' ')}</span><strong>{String(value)}</strong></div>{/each}
                        </details>
                      {/if}
                    {:else}
                      <small>{doctorTr('Аппаратное состояние пока неизвестно. Проверка запускается вручную и может не поддерживаться USB-флешками.','Hardware health is unknown until a manual check; some USB flash drives do not support it.')}</small>
                    {/if}
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {:else}
          <p class="rf-doctor-note">{doctorTr('Подключённые физические диски пока не обнаружены.','No physical drives detected yet.')}</p>
        {/if}
        <details class="rf-doctor-incidents">
          <summary>{doctorTr('История подключений и отключений','Connection and disconnection history')}{data.doctor.incidents?.length ? ` · ${data.doctor.incidents.length}` : ''}</summary>
          {#if data.doctor.incidents?.length}
            {#each data.doctor.incidents.slice(0,10) as event (`${event.at}:${event.kind}:${event.mount}`)}<div class="rf-doctor-mount"><span>{doctorEventName(event.kind)} · {event.mount}</span><small>{new Date(event.at).toLocaleString(locale)}</small></div>{/each}
          {:else}<p class="rf-doctor-note">{doctorTr('За время работы этой диагностики изменений подключения не замечено.','No connection changes have been observed during this monitoring session.')}</p>{/if}
          <small>{doctorTr('Это не полная история диска: события доступны только с момента запуска этой диагностики. Журнал системных ошибок пока не подключён.','This is not the complete disk history: only events observed during this diagnostic session are shown. Kernel error logs are not included yet.')}</small>
        </details>
        <details class="rf-doctor-incidents">
          <summary>{doctorTr('История ошибок накопителей','Storage error history')} · {data.doctor.history?.length || 0}</summary>
          {#if data.doctor.history?.length}
            {#each data.doctor.history.slice(0,32) as event (event.id)}
              <div class="rf-doctor-mount"><span><strong class:rf-doctor-bad={event.severity==='critical'}>{doctorHistoryName(event.kind)}</strong> · {event.device || doctorTr('Накопитель','Drive')}</span><small>{new Date(event.at).toLocaleString(locale)}</small></div>
              {#if advanced && event.detail}<small>{event.detail}</small>{/if}
            {/each}
          {:else}<p class="rf-doctor-note">{doctorTr('Ошибок, обнаруженных мониторингом, пока нет. Это не гарантирует отсутствие прежних неполадок.','No errors observed by Monitoring yet. This does not guarantee a flawless historical record.')}</p>{/if}
          <small>{data.doctor.kernel_history_available ? doctorTr('Системный журнал доступен. Время события — момент обнаружения записи.','Kernel log available. Timestamps show when entries were observed.') : doctorTr('Системный журнал недоступен: отображаются только события, которые видел RouterForge.','Kernel log unavailable: only events observed by RouterForge are shown.')}</small>
        </details>
      </section>
    {/if}
    <details class="rf-doctor-extras" bind:open={storageTechnicalOpen}>
      <summary>{doctorTr('Техническая статистика дисков и файловых систем','Technical disk and filesystem statistics')}</summary>
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'monitoring.storage.title')}</strong><span>{t(locale, 'monitoring.storage.subtitle')}</span></div></div>
      {#if (data.storage.mounts || []).length}
        <div class="table-scroll"><table><thead><tr><th>{t(locale, 'monitoring.storage.mount')}</th><th>{t(locale, 'monitoring.storage.device')}</th><th>{t(locale, 'monitoring.storage.filesystem')}</th><th>{t(locale, 'monitoring.storage.usage')}</th><th>{t(locale, 'monitoring.storage.space')}</th><th>{t(locale, 'monitoring.storage.read')}</th><th>{t(locale, 'monitoring.storage.write')}</th></tr></thead><tbody>
          {#each data.storage.mounts || [] as item (item.mount)}
            <tr><td><strong>{storageMountName(item)}</strong><div class="cell-sub mono">{item.mount}</div></td><td class="mono">{item.device}</td><td>{item.fs_type}</td><td><strong>{Number(item.used_pct || 0).toFixed(1)}%</strong><div class="progress"><span style={`width:${pct(item.used_pct)}%`}></span></div></td><td class="mono">{bytes(item.used_bytes)} / {bytes(item.total_bytes)}</td><td class="mono good">{rate(item.read_bps)}</td><td class="mono accent-text">{rate(item.write_bps)}</td></tr>
          {/each}
        </tbody></table></div>
      {:else}
        <div class="empty">{t(locale, 'monitoring.storage.noUserStorage')}</div>
      {/if}
    </section>

    {#if (data.storage.disks || []).length}
      <div class="section-caption">{t(locale, 'monitoring.storage.physical')}</div>
      <section class="module-card-grid">
        {#each data.storage.disks || [] as disk (disk.name)}
          <article class="module-detail-card">
            <div><strong>{disk.model || disk.vendor || disk.name}</strong><span class="mono">{disk.path}</span></div>
            <dl>
              <div><dt>{t(locale, 'monitoring.storage.size')}</dt><dd>{bytes(disk.size_bytes)}</dd></div>
              <div><dt>{t(locale, 'monitoring.storage.type')}</dt><dd>{storageType(disk)}</dd></div>
              <div><dt>{t(locale, 'monitoring.storage.read')}</dt><dd>{rate(disk.rate?.read_bps)}</dd></div>
              <div><dt>{t(locale, 'monitoring.storage.write')}</dt><dd>{rate(disk.rate?.write_bps)}</dd></div>
            </dl>
          </article>
        {/each}
      </section>
    {/if}

    {#if advanced}
      <details class="panel advanced-details">
        <summary>{t(locale, 'monitoring.storage.systemDetails')}</summary>
        <div class="advanced-details-body">
          <strong>{t(locale, 'monitoring.storage.systemMounts')}</strong>
          <div class="table-scroll"><table><thead><tr><th>{t(locale, 'monitoring.storage.mount')}</th><th>{t(locale, 'monitoring.storage.device')}</th><th>{t(locale, 'monitoring.storage.filesystem')}</th><th>{t(locale, 'monitoring.storage.usage')}</th><th>{t(locale, 'monitoring.storage.iops')}</th></tr></thead><tbody>
            {#each data.storage.system_mounts || [] as item (`${item.device}-${item.mount}`)}
              <tr><td class="mono">{item.mount}</td><td class="mono">{item.device}</td><td>{item.fs_type}</td><td>{Number(item.used_pct || 0).toFixed(1)}%</td><td class="mono">{Number(item.read_ops || 0).toFixed(1)} / {Number(item.write_ops || 0).toFixed(1)}</td></tr>
            {/each}
          </tbody></table></div>
          <strong>{t(locale, 'monitoring.storage.systemDisks')}</strong>
          <div class="advanced-device-list mono">
            {#each data.storage.system_disks || [] as disk (disk.name)}
              <span>{disk.path} · {bytes(disk.size_bytes)}{disk.model ? ` · ${disk.model}` : ''}</span>
            {/each}
          </div>
        </div>
      </details>
    {/if}
    </details>

  {:else if tab === 'network' && data.summary}
    <section class="metric-grid module-metric-grid">
      <div class="metric-card"><span>{t(locale, 'monitoring.network.activeInterfaces')}</span><strong>{fmtInt(data.summary.interface_count || 0)}</strong><small>{advanced ? t(locale, 'monitoring.network.systemCount', { count: fmtInt(data.summary.system_interface_count || 0, locale) }) : ''}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.network.conntrack')}</span><strong>{fmtInt(data.summary.conntrack_count || 0)}</strong><small>{t(locale, 'monitoring.network.max', { value: fmtInt(data.summary.conntrack_max || 0) })}</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.network.errors')}</span><strong>{fmtInt(data.summary.errors || 0)}</strong><small>RX + TX</small></div>
      <div class="metric-card"><span>{t(locale, 'monitoring.network.drops')}</span><strong>{fmtInt(data.summary.drops || 0)}</strong><small>RX + TX</small></div>
    </section>

    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'monitoring.network.interfaces')}</strong><span>{t(locale, 'monitoring.network.interfacesHint')}</span></div></div>
      {#if visibleInterfaces.length}
        <div class="table-scroll"><table class="sortable-table"><thead><tr>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'display_name')}>{t(locale, 'monitoring.network.name')} <span>{sortArrow(interfaceSort, 'display_name')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'name')}>{t(locale, 'monitoring.network.interface')} <span>{sortArrow(interfaceSort, 'name')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'state')}>{t(locale, 'monitoring.network.state')} <span>{sortArrow(interfaceSort, 'state')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'address')}>{t(locale, 'monitoring.network.address')} <span>{sortArrow(interfaceSort, 'address')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'speed')}>{t(locale, 'monitoring.network.speed')} <span>{sortArrow(interfaceSort, 'speed')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'rx')}>{t(locale, 'monitoring.network.rx')} <span>{sortArrow(interfaceSort, 'rx')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'tx')}>{t(locale, 'monitoring.network.tx')} <span>{sortArrow(interfaceSort, 'tx')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'errors')}>{t(locale, 'monitoring.network.errorsColumn')} <span>{sortArrow(interfaceSort, 'errors')}</span></button></th>
          <th><button onclick={() => interfaceSort = nextSort(interfaceSort, 'drops')}>{t(locale, 'monitoring.network.dropsColumn')} <span>{sortArrow(interfaceSort, 'drops')}</span></button></th>
        </tr></thead><tbody>
          {#each visibleInterfaces as iface (iface.name)}
            <tr>
              <td><strong>{iface.display_name || iface.name}</strong>{#if advanced && iface.keenetic_id}<div class="cell-sub mono">{iface.keenetic_id}</div>{/if}</td>
              <td><strong class="mono">{iface.name}</strong>{#if advanced && iface.mac}<div class="cell-sub mono">{iface.mac}</div>{/if}</td>
              <td><span class="state-chip {interfaceStateClass(iface)}">{interfaceState(iface)}</span></td>
              <td class="mono">{iface.primary_address || '—'}</td>
              <td>{iface.speed_mbps > 0 ? `${iface.speed_mbps} Mbps` : '—'}{#if iface.duplex}<div class="cell-sub">{iface.duplex}</div>{/if}</td>
              <td class="mono good">{rate(iface.rate?.rx_bps)}</td>
              <td class="mono accent-text">{rate(iface.rate?.tx_bps)}</td>
              <td class="mono">{fmtInt(Number(iface.rx_errors || 0) + Number(iface.tx_errors || 0))}</td>
              <td class="mono">{fmtInt(Number(iface.rx_drops || 0) + Number(iface.tx_drops || 0))}</td>
            </tr>
          {/each}
        </tbody></table></div>
      {:else}
        <div class="empty">{t(locale, 'monitoring.network.noActive')}</div>
      {/if}
    </section>

    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'monitoring.network.routes')}</strong><span>/proc/net/route</span></div></div>
      <div class="table-scroll"><table class="sortable-table"><thead><tr>
        <th><button onclick={() => routeSort = nextSort(routeSort, 'interface')}>{t(locale, 'monitoring.network.interface')} <span>{sortArrow(routeSort, 'interface')}</span></button></th>
        <th><button onclick={() => routeSort = nextSort(routeSort, 'destination')}>{t(locale, 'monitoring.network.destination')} <span>{sortArrow(routeSort, 'destination')}</span></button></th>
        <th><button onclick={() => routeSort = nextSort(routeSort, 'mask')}>{t(locale, 'monitoring.network.mask')} <span>{sortArrow(routeSort, 'mask')}</span></button></th>
        <th><button onclick={() => routeSort = nextSort(routeSort, 'gateway')}>{t(locale, 'monitoring.network.gateway')} <span>{sortArrow(routeSort, 'gateway')}</span></button></th>
        <th><button onclick={() => routeSort = nextSort(routeSort, 'metric')}>{t(locale, 'monitoring.network.metric')} <span>{sortArrow(routeSort, 'metric')}</span></button></th>
        {#if advanced}<th>{t(locale, 'monitoring.network.flags')}</th>{/if}
      </tr></thead><tbody>
        {#each sortedRoutes as route (`${route.interface}-${route.destination}-${route.gateway}-${route.metric}`)}
          <tr><td><strong>{route.interface}</strong></td><td class="mono">{route.destination}</td><td class="mono">{route.mask}</td><td class="mono">{route.gateway}</td><td class="mono">{route.metric}</td>{#if advanced}<td class="mono">{route.flags}</td>{/if}</tr>
        {/each}
      </tbody></table></div>
    </section>

    {#if advanced}
      <details class="panel advanced-details">
        <summary>{t(locale, 'monitoring.network.systemInterfaces')}</summary>
        <div class="advanced-details-body">
          <p class="module-note">{t(locale, 'monitoring.network.systemHint')}</p>
          <div class="table-scroll"><table class="sortable-table"><thead><tr>
            <th><button onclick={() => systemInterfaceSort = nextSort(systemInterfaceSort, 'name')}>{t(locale, 'monitoring.network.interface')} <span>{sortArrow(systemInterfaceSort, 'name')}</span></button></th>
            <th><button onclick={() => systemInterfaceSort = nextSort(systemInterfaceSort, 'state')}>{t(locale, 'monitoring.network.state')} <span>{sortArrow(systemInterfaceSort, 'state')}</span></button></th>
            <th><button onclick={() => systemInterfaceSort = nextSort(systemInterfaceSort, 'address')}>{t(locale, 'monitoring.network.address')} <span>{sortArrow(systemInterfaceSort, 'address')}</span></button></th>
            <th><button onclick={() => systemInterfaceSort = nextSort(systemInterfaceSort, 'rx')}>{t(locale, 'monitoring.network.rx')} <span>{sortArrow(systemInterfaceSort, 'rx')}</span></button></th>
            <th><button onclick={() => systemInterfaceSort = nextSort(systemInterfaceSort, 'tx')}>{t(locale, 'monitoring.network.tx')} <span>{sortArrow(systemInterfaceSort, 'tx')}</span></button></th>
          </tr></thead><tbody>
            {#each systemInterfaces as iface (iface.name)}
              <tr><td><strong class="mono">{iface.name}</strong>{#if iface.display_name && iface.display_name !== iface.name}<div class="cell-sub">{iface.display_name}</div>{/if}</td><td>{iface.oper_state || '—'}</td><td class="mono module-addresses">{(iface.addresses || []).join(' · ') || '—'}</td><td class="mono good">{rate(iface.rate?.rx_bps)}</td><td class="mono accent-text">{rate(iface.rate?.tx_bps)}</td></tr>
            {/each}
          </tbody></table></div>
        </div>
      </details>
    {/if}

  {:else if tab === 'profiling' && data.profiling}
    <div class="two-col">
      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'monitoring.profiling.title')}</strong><span>{t(locale, 'monitoring.profiling.listener')}</span></div><span class="state-chip {data.profiling.running ? 'good' : data.profiling.enabled ? 'warn' : 'neutral'}">{data.profiling.running ? t(locale, 'monitoring.profiling.running') : data.profiling.enabled ? t(locale, 'monitoring.profiling.errorState') : t(locale, 'monitoring.profiling.disabled')}</span></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.profiling.listen')}</strong><span>{t(locale, 'monitoring.profiling.loopbackOnly')}</span></div><div class="info-value mono">{data.profiling.listen}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.profiling.slowRequest')}</strong><span>{t(locale, 'monitoring.profiling.loggingThreshold')}</span></div><div class="info-value mono">{data.profiling.slow_ms} ms</div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.profiling.mode')}</strong><span>{t(locale, 'monitoring.profiling.securityBoundary')}</span></div><div class="info-value good">{data.profiling.mode}</div></div>
        <div class="info-row"><div><strong>{t(locale, 'monitoring.profiling.errorLabel')}</strong><span>{t(locale, 'monitoring.profiling.listenerStartup')}</span></div><div class="info-value">{data.profiling.error || t(locale, 'common.none')}</div></div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>{t(locale, 'monitoring.profiling.sshAccess')}</strong><span>{t(locale, 'monitoring.profiling.lanRestricted')}</span></div></div>
        <div class="module-code-block mono">ssh -L 6061:127.0.0.1:6061 root@ROUTER</div>
        <div class="module-code-block mono">go tool pprof http://127.0.0.1:6061/debug/pprof/heap</div>
      </section>
    </div>
  {/if}
</div>

<style>
  .sortable-table th button {
    appearance: none;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    font-weight: inherit;
    text-align: left;
    padding: 0;
    cursor: pointer;
    white-space: nowrap;
  }

  .sortable-table th button:hover {
    color: var(--rf-text, var(--text));
  }

  .sortable-table th button span {
    display: inline-block;
    min-width: 1.15em;
    margin-left: .2rem;
    color: var(--rf-muted, var(--muted));
    font-size: .75em;
  }

  .section-caption {
    margin: .45rem 0 .7rem;
    color: var(--rf-muted, var(--muted));
    font-size: .78rem;
    font-weight: 700;
    letter-spacing: .08em;
    text-transform: uppercase;
  }

  .advanced-details {
    margin-top: 1rem;
    overflow: hidden;
  }

  .advanced-details > summary {
    cursor: pointer;
    padding: 1rem 1.1rem;
    font-weight: 700;
    color: var(--rf-text, var(--text));
    user-select: none;
  }

  .advanced-details > summary::marker {
    color: var(--rf-accent, var(--accent));
  }

  .advanced-details-body {
    display: grid;
    gap: .9rem;
    padding: 0 1.1rem 1.1rem;
  }

  .advanced-device-list {
    display: grid;
    gap: .35rem;
    color: var(--rf-muted, var(--muted));
    font-size: .82rem;
  }
  .process-metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .process-manager-head {
    gap: 1rem;
    align-items: center;
  }

  .process-manager-toolbar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: .65rem;
    min-width: min(30rem, 48vw);
  }

  .process-search {
    width: min(25rem, 42vw);
    min-width: 13rem;
    box-sizing: border-box;
    border: 1px solid var(--rf-border, var(--border));
    border-radius: .65rem;
    background: var(--rf-surface-2, var(--surface-2));
    color: var(--rf-text, var(--text));
    padding: .58rem .72rem;
    font: inherit;
    outline: none;
  }

  .process-search:focus {
    border-color: var(--rf-accent, var(--accent));
  }

  .process-manager-panel {
    overflow-anchor: none;
  }

  .process-table-scroll {
    max-height: min(68vh, 720px);
    overflow: auto;
    overscroll-behavior: contain;
    overflow-anchor: none;
    scrollbar-gutter: stable;
  }

  .process-table-scroll .process-table,
  .process-table-scroll .process-table tbody,
  .process-table-scroll .process-table tr,
  .process-table-scroll .process-table td {
    overflow-anchor: none;
  }

  .process-table-scroll .process-table thead th {
    position: sticky;
    top: 0;
    z-index: 3;
  }

  .process-table {
    width: 100%;
    min-width: 760px;
    table-layout: fixed;
  }

  .process-table th,
  .process-table td {
    min-width: 0;
  }

  .process-table th:nth-child(1) { width: 28%; }
  .process-table th:nth-child(2) { width: 7%; }
  .process-table th:nth-child(3) { width: 10%; }
  .process-table th:nth-child(4) { width: 12%; }
  .process-table th:nth-child(5) { width: 14%; }
  .process-table th:nth-child(6) { width: 16%; }
  .process-table th:nth-child(7) { width: 7%; }
  .process-table th:nth-child(8) { width: 6%; }

  .process-table td {
    vertical-align: middle;
  }

  .process-table td:first-child > strong {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .process-command {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .process-usage {
    display: grid;
    gap: .32rem;
    min-width: 0;
  }

  .process-usage > div:first-child {
    display: flex;
    justify-content: space-between;
    gap: .6rem;
    white-space: nowrap;
  }

  .process-usage > div:first-child span {
    color: var(--rf-muted, var(--muted));
    font-size: .76rem;
  }

  @media (max-width: 760px) {
    .process-manager-head {
      align-items: stretch;
    }

    .process-manager-toolbar {
      width: 100%;
      min-width: 0;
      justify-content: space-between;
    }

    .process-search {
      width: 100%;
      min-width: 0;
    }
  }
  .rf-doctor-shell{padding:14px;display:grid;gap:12px}
  .rf-doctor-entware,.rf-doctor-disk,.rf-doctor-incidents{border:1px solid var(--rf-border,#333);border-radius:8px;padding:12px;display:grid;gap:8px;min-width:0}
  .rf-doctor-entware small,.rf-doctor-disk small,.rf-doctor-incidents small,.rf-doctor-note{color:var(--rf-muted,#9ba5b1);font-size:12px}
  .rf-doctor-good{color:var(--good,#4fd498)}.rf-doctor-bad{color:var(--error,#ff6d6d)}
  .rf-doctor-warning{color:var(--warn,#eeb45e);font-size:12px;margin:0;overflow-wrap:anywhere}
  .rf-doctor-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,340px),1fr));gap:12px;min-width:0}
  .rf-doctor-top,.rf-doctor-mount,.rf-doctor-smart-summary{display:flex;justify-content:space-between;align-items:center;gap:10px}
  .rf-doctor-top>div{display:grid;gap:4px;min-width:0}.rf-doctor-top small{overflow-wrap:anywhere}
  .rf-doctor-mount{font-size:12px;padding:4px 0;overflow-wrap:anywhere}.rf-doctor-mount strong{text-align:right;flex-shrink:0}
  .rf-doctor-mounts,.rf-doctor-details{display:grid;gap:4px;min-width:0}
  .rf-doctor-smart-summary{border-top:1px solid var(--rf-border,#333);padding-top:9px;font-size:13px}
  .rf-doctor-controls{display:flex;flex-wrap:wrap;gap:8px;align-items:end}.rf-doctor-controls label{font-size:11px;display:grid;gap:5px;color:var(--rf-muted,#aaa)}
  .rf-doctor-controls select{background:var(--rf-surface-2,#15191e);border:1px solid var(--rf-border,#333);color:var(--rf-text,#fff);padding:7px;border-radius:6px}
  .rf-doctor-details{padding-top:8px;border-top:1px solid var(--rf-border,#333)}.rf-doctor-details table{font-size:11px}
  @media(max-width:600px){.rf-doctor-top,.rf-doctor-mount{align-items:flex-start;flex-wrap:wrap}.rf-doctor-mount strong{text-align:left}}
  /* P46: friendly drive overview. Existing advanced tables remain intact. */
  .rf-doctor-friendly{gap:16px}
  .rf-doctor-intro{display:flex;align-items:center;justify-content:space-between;gap:16px}
  .rf-doctor-intro>div{min-width:0}.rf-doctor-intro strong{font-size:19px;line-height:1.3}
  .rf-doctor-intro p{margin:6px 0 0;color:var(--rf-muted,#9ba5b1);font-size:13px;line-height:1.5}
  .rf-doctor-count{white-space:nowrap;border:1px solid var(--rf-border,#333);padding:7px 11px;border-radius:30px;color:var(--rf-muted,#9ba5b1);font-size:12px}
  .rf-doctor-health{border:1px solid rgba(53,190,118,.38);background:rgba(53,190,118,.065);border-radius:10px;padding:15px;display:flex;gap:12px;align-items:flex-start}
  .rf-doctor-health-warn{border-color:rgba(235,174,79,.43);background:rgba(235,174,79,.06)}
  .rf-doctor-health-icon{display:grid;place-items:center;width:31px;height:31px;flex-shrink:0;border-radius:50%;font-weight:700;color:var(--good,#4fd498);background:rgba(53,190,118,.13)}
  .rf-doctor-health-warn .rf-doctor-health-icon{color:var(--warn,#eeb45e);background:rgba(235,174,79,.12)}
  .rf-doctor-health-copy{display:grid;gap:5px;min-width:0;flex:1}.rf-doctor-health-copy strong{font-size:15px}.rf-doctor-health-copy span{font-size:12px;line-height:1.55;color:var(--rf-muted,#aab1b9)}
  .rf-doctor-pill{flex-shrink:0;font-size:11px;padding:6px 9px;background:rgba(53,190,118,.1);border-radius:20px;color:var(--good,#4fd498)}
  .rf-doctor-pill-warn{background:rgba(235,174,79,.1);color:var(--warn,#eeb45e)}
  .rf-doctor-install{border:1px solid var(--rf-border,#333);border-radius:10px;padding:15px;display:flex;justify-content:space-between;gap:16px;align-items:center;background:var(--rf-surface-2,#16191e)}
  .rf-doctor-install-copy{display:grid;gap:6px;max-width:620px}.rf-doctor-install-copy strong{font-size:14px}.rf-doctor-install-copy p{font-size:12px;color:var(--rf-muted,#abb3bd);line-height:1.6;margin:0}
  .rf-doctor-install-copy small{font-size:11px;color:var(--rf-muted,#9ba5b1)}
  .rf-doctor-install-button{min-height:40px;white-space:normal;text-align:center;flex-shrink:0}
  .rf-doctor-ready{font-size:12px;color:var(--rf-muted,#aab2bc);display:flex;gap:7px;align-items:center}.rf-doctor-ready>span{color:var(--good,#4fd498)}
  .rf-doctor-feedback{font-size:12px;color:var(--good,#4fd498);margin:0}
  .rf-doctor-drive{padding:16px;gap:13px;border-radius:10px}
  .rf-doctor-drive-head{display:flex;justify-content:space-between;gap:12px;align-items:center;min-width:0}
  .rf-doctor-drive-title{display:flex;align-items:center;gap:11px;min-width:0}.rf-doctor-drive-title>div{min-width:0;display:grid;gap:4px}
  .rf-doctor-drive-title strong{font-size:15px;overflow-wrap:anywhere}.rf-doctor-drive-title small{font-size:11px;line-height:1.4}
  .rf-doctor-drive-icon{width:36px;height:36px;flex-shrink:0;display:grid;place-items:center;border-radius:9px;background:var(--rf-surface-2,#20242a);color:var(--rf-muted,#aab1b9);font-size:21px}
  .rf-doctor-drive-size{font-size:15px;white-space:nowrap;font-variant-numeric:tabular-nums}
  .rf-doctor-drive-state{display:flex;align-items:center;gap:7px;font-size:12px;color:var(--good,#4fd498)}
  .rf-doctor-state-dot{display:inline-block;width:7px;height:7px;flex-shrink:0;border-radius:50%;background:currentColor}
  .rf-doctor-drive-danger{color:var(--warn,#eeb45e)}
  .rf-doctor-space-label{display:flex;justify-content:space-between;align-items:center;gap:8px;font-size:12px;font-variant-numeric:tabular-nums}
  .rf-doctor-space-label span{color:var(--rf-muted,#aab1b9)}
  .rf-doctor-space-track{background:var(--rf-surface-2,#292d32);height:9px;border-radius:10px;overflow:hidden}
  .rf-doctor-space-track>span{display:block;height:100%;background:var(--good,#48bb80);border-radius:10px;transition:width .3s ease}
  .rf-doctor-drive .rf-doctor-smart-summary{align-items:flex-start}.rf-doctor-drive .rf-doctor-smart-summary span{color:var(--rf-muted,#aab1b9)}
  .rf-doctor-drive .rf-doctor-smart-summary strong{max-width:60%;text-align:right;font-size:12px}
  .rf-doctor-drive .rf-doctor-controls{align-items:center;gap:8px}
  .rf-doctor-drive .rf-doctor-controls .button{min-height:35px}
  .rf-doctor-details{gap:9px;padding-top:13px}.rf-doctor-details>strong{font-size:13px}
  .rf-doctor-bridge{border:1px solid var(--rf-border,#333);border-radius:7px;padding:9px;margin-top:4px;min-width:0}
  .rf-doctor-bridge summary{cursor:pointer;font-size:12px;color:var(--rf-muted,#b1b8c0)}
  .rf-doctor-bridge label{display:grid;gap:5px;padding:9px 0;font-size:12px}
  .rf-doctor-bridge select{max-width:240px;background:var(--rf-surface-2,#15191e);color:var(--rf-text,#fff);border:1px solid var(--rf-border,#333);padding:8px;border-radius:6px}
  .rf-doctor-bridge small{font-size:11px}
  .rf-doctor-incidents{padding:12px 14px}.rf-doctor-incidents summary{cursor:pointer;font-size:13px;font-weight:650}
  .rf-doctor-incidents[open] summary{margin-bottom:12px}
  .rf-doctor-extras{border:1px solid var(--rf-border,#333);border-radius:9px;padding:12px;display:grid;gap:12px}
  .rf-doctor-extras>summary{cursor:pointer;font-size:13px;color:var(--rf-muted,#aab1b9);font-weight:650}
  .rf-doctor-extras[open]>summary{margin-bottom:10px}
  @media(max-width:720px){.rf-doctor-install{flex-direction:column;align-items:stretch}.rf-doctor-install-button{width:100%}.rf-doctor-health{flex-wrap:wrap}.rf-doctor-pill{margin-left:42px}.rf-doctor-drive-head{align-items:flex-start}.rf-doctor-space-label{flex-wrap:wrap}}
  .rf-thermal-disks{display:grid;gap:12px}
  .rf-thermal-disk-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,245px),1fr));gap:12px}
  .rf-thermal-disk-card{display:grid;gap:8px;padding:14px;border:1px solid var(--rf-border,#38404c);border-radius:12px;min-width:0}
  .rf-thermal-disk-header{display:grid;gap:3px}.rf-thermal-disk-header strong{overflow-wrap:anywhere}.rf-thermal-disk-header small,.rf-thermal-disk-card small{color:var(--rf-muted,#9ba5b1);font-size:12px}
  .rf-thermal-disk-reading{display:flex;align-items:baseline;justify-content:space-between;gap:12px;flex-wrap:wrap}
  .rf-thermal-disk-reading strong{font-size:26px;line-height:1.2;font-variant-numeric:tabular-nums}.rf-thermal-disk-reading span{font-size:12px;color:var(--rf-muted,#9ba5b1)}
  .rf-thermal-hot strong,.rf-thermal-hot span{color:var(--warn,#ebba66)}
</style>
