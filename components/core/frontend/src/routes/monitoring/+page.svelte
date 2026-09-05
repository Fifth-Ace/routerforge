<script>
  import { onMount } from 'svelte';
  import { replaceState } from '$app/navigation';
  import { catalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { getModule } from '$lib/api.js';
  import { bytes, fmtDuration, fmtInt } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';

  const moduleDefs = [
    { id: 'system', nameKey: 'monitoring.modules.system', short: 'SYS', package: 'routerforge-system' },
    { id: 'thermal', nameKey: 'monitoring.modules.thermal', short: 'TMP', package: 'routerforge-thermal' },
    { id: 'storage', nameKey: 'monitoring.modules.storage', short: 'DSK', package: 'routerforge-storage' },
    { id: 'network', nameKey: 'monitoring.modules.network', short: 'NET', package: 'routerforge-network' },
  ];

  let tab = 'system';
  let loading = false;
  let errorText = '';
  let data = {};
  let timer = null;
  let interfaceSort = { key: 'display_name', dir: 'asc' };
  let systemInterfaceSort = { key: 'name', dir: 'asc' };
  let routeSort = { key: 'destination', dir: 'asc' };

  $: locale = $settings.locale || 'ru';
  $: advanced = $settings.uiLevel === 'advanced';
  $: modules = $catalog.modules || [];
  $: installedDefs = moduleDefs.filter((definition) => modules.some((item) => item.id === definition.id && item.installed));
  $: currentModule = modules.find((item) => item.id === tab);
  $: definition = installedDefs.find((item) => item.id === tab) || installedDefs[0] || moduleDefs[0];
  $: visibleInterfaces = sortInterfaces(data.interfaces?.interfaces || [], interfaceSort);
  $: systemInterfaces = sortInterfaces(data.interfaces?.system_interfaces || [], systemInterfaceSort);
  $: sortedRoutes = sortRoutes(data.routes?.routes || [], routeSort);

  $: if (installedDefs.length && !installedDefs.some((item) => item.id === tab)) {
    tab = installedDefs[0].id;
    data = {};
    errorText = '';
    if (typeof window !== 'undefined') {
      Promise.resolve().then(() => {
        replaceState(`/monitoring?tab=${encodeURIComponent(tab)}`, {});
        startTimer();
        loadCurrent();
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
  const moduleName = (item) => t(locale, item?.nameKey || 'monitoring.title');

  async function loadSystem() {
    const [summary, cpu, memory] = await Promise.all([
      getModule('system', 'summary'), getModule('system', 'cpu'), getModule('system', 'memory')
    ]);
    return { summary, cpu, memory };
  }

  async function loadThermal() {
    return { thermal: await getModule('thermal', 'sensors') };
  }

  async function loadStorage() {
    return { storage: await getModule('storage', 'storage') };
  }

  async function loadNetwork() {
    const [summary, interfaces, routes] = await Promise.all([
      getModule('network', 'summary'), getModule('network', 'interfaces'), getModule('network', 'routes')
    ]);
    return { summary, interfaces, routes };
  }

  async function loadProfiling() {
    return { profiling: await getModule('profiling', 'status') };
  }

  async function loadCurrent(showLoading = true) {
    if (showLoading) loading = true;
    try {
      if (tab === 'system') data = await loadSystem();
      if (tab === 'thermal') data = await loadThermal();
      if (tab === 'storage') data = await loadStorage();
      if (tab === 'network') data = await loadNetwork();
      if (tab === 'profiling') data = await loadProfiling();
      errorText = '';
    } catch (error) {
      data = {};
      errorText = error?.payload?.error || error?.message || t(locale, 'common.unavailable');
    } finally {
      if (showLoading) loading = false;
    }
  }

  function startTimer() {
    clearInterval(timer);
    const interval = tab === 'thermal' ? 10000 : tab === 'profiling' ? 5000 : 3000;
    timer = setInterval(() => {
      if (!document.hidden && !errorText) loadCurrent(false);
    }, interval);
  }

  function selectTab(id) {
    if (tab === id) return;
    tab = id;
    replaceState(`/monitoring?tab=${encodeURIComponent(id)}`, {});
    startTimer();
    loadCurrent();
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
    const initial = new URLSearchParams(location.search).get('tab');
    if (initial && moduleDefs.some((item) => item.id === initial)) tab = initial;
    loadCurrent();
    startTimer();
    return () => clearInterval(timer);
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
      {@const catalogItem = modules.find((entry) => entry.id === item.id)}
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

  {:else if tab === 'thermal' && data.thermal}
    <div class="module-meta-strip mono">
      <span>{t(locale, 'monitoring.thermal.sensors')} <strong>{data.thermal.sensor_count || 0}</strong></span>
      <span>{t(locale, 'monitoring.thermal.cache')} <strong>{data.thermal.cache_seconds || 0}s</strong></span>
      <span>{t(locale, 'monitoring.thermal.smartctl')} <strong class={data.thermal.optional_smartctl ? 'good' : 'muted'}>{data.thermal.optional_smartctl ? t(locale, 'common.available') : t(locale, 'common.optional')}</strong></span>
    </div>
    <section class="module-thermal-grid">
      {#if (data.thermal.sensors || []).length}
        {#each data.thermal.sensors as sensor (sensor.id)}
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
        <div class="panel-head"><div><strong>Core profiling</strong><span>pprof listener</span></div><span class="state-chip {data.profiling.running ? 'good' : data.profiling.enabled ? 'warn' : 'neutral'}">{data.profiling.running ? 'RUNNING' : data.profiling.enabled ? 'ERROR' : 'DISABLED'}</span></div>
        <div class="info-row"><div><strong>Listen</strong><span>loopback only</span></div><div class="info-value mono">{data.profiling.listen}</div></div>
        <div class="info-row"><div><strong>Slow request</strong><span>logging threshold</span></div><div class="info-value mono">{data.profiling.slow_ms} ms</div></div>
        <div class="info-row"><div><strong>Mode</strong><span>security boundary</span></div><div class="info-value good">{data.profiling.mode}</div></div>
        <div class="info-row"><div><strong>Error</strong><span>listener startup</span></div><div class="info-value">{data.profiling.error || t(locale, 'common.none')}</div></div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>SSH access</strong><span>pprof is not exposed to LAN</span></div></div>
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
</style>
