<script>
  import { onMount } from 'svelte';
  import { replaceState } from '$app/navigation';
  import { catalog } from '$lib/stores/catalog.js';
  import { getModule } from '$lib/api.js';
  import { bytes, fmtDuration, fmtInt } from '$lib/utils.js';

  const moduleDefs = [
    { id: 'system', name: 'System Monitor', short: 'SYS', package: 'routerforge-system' },
    { id: 'thermal', name: 'Thermal Monitor', short: 'TMP', package: 'routerforge-thermal' },
    { id: 'storage', name: 'Storage Monitor', short: 'DSK', package: 'routerforge-storage' },
    { id: 'network', name: 'Network Monitor', short: 'NET', package: 'routerforge-network' },
  ];

  let tab = 'system';
  let loading = false;
  let errorText = '';
  let data = {};
  let timer = null;

  $: modules = $catalog.modules || [];
  $: installedDefs = moduleDefs.filter((definition) => modules.some((item) => item.id === definition.id && item.installed));
  $: currentModule = modules.find((item) => item.id === tab);
  $: definition = installedDefs.find((item) => item.id === tab) || installedDefs[0] || moduleDefs[0];
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
      errorText = error?.payload?.error || error?.message || 'Модуль недоступен';
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

  onMount(() => {
    const initial = new URLSearchParams(location.search).get('tab');
    if (initial && moduleDefs.some((item) => item.id === initial)) tab = initial;
    loadCurrent();
    startTimer();
    return () => clearInterval(timer);
  });
</script>

<svelte:head><title>RouterForge — Мониторинг</title></svelte:head>

<div class="page modules-page">
  <div class="page-head">
    <div>
      <h1>Мониторинг</h1>
      <p>Единый центр телеметрии RouterForge. Показываются только установленные monitoring providers.</p>
    </div>
    <span class="state-chip {currentModule?.service_running ? 'good' : currentModule?.installed ? 'warn' : 'info'}">
      {currentModule?.service_running ? 'MODULE ONLINE' : currentModule?.installed ? 'INSTALLED / OFFLINE' : 'OPTIONAL'}
    </span>
  </div>

  <div class="module-selector">
    {#each installedDefs as item}
      {@const catalogItem = modules.find((entry) => entry.id === item.id)}
      <button class:active={tab === item.id} onclick={() => selectTab(item.id)}>
        <span class="module-selector-icon mono">{item.short}</span>
        <span><strong>{item.name}</strong><small>{catalogItem?.service_running ? 'ONLINE' : catalogItem?.installed ? 'INSTALLED' : 'NOT INSTALLED'}</small></span>
        <i class="status-dot {catalogItem?.service_running ? 'good' : catalogItem?.installed ? 'warn' : 'muted'}"></i>
      </button>
    {/each}
  </div>

  {#if errorText}
    <section class="panel module-offline">
      <div class="panel-head">
        <div><strong>{definition.name} недоступен</strong><span>Core продолжает работать независимо от optional modules.</span></div>
        <span class="state-chip info">OPTIONAL IPK</span>
      </div>
      <div class="module-install-hint">
        <p>{errorText}</p>
        <code>opkg install {definition.package}</code>
        <span>После установки helper запускается автоматически и появляется здесь без изменения Core.</span>
      </div>
    </section>
  {:else if loading}
    <section class="panel"><div class="empty">Читаю {definition.name}…</div></section>

  {:else if tab === 'system' && data.summary}
    <section class="metric-grid module-metric-grid">
      <div class="metric-card"><span>HOST</span><strong>{data.summary.hostname || '—'}</strong><small>{data.summary.kernel || '—'} · {data.summary.architecture || '—'}</small></div>
      <div class="metric-card"><span>LOAD 1 / 5 / 15</span><strong>{Number(data.summary.load_1 || 0).toFixed(2)}</strong><small>{Number(data.summary.load_5 || 0).toFixed(2)} · {Number(data.summary.load_15 || 0).toFixed(2)}</small></div>
      <div class="metric-card"><span>RAM</span><strong>{Number(data.memory?.used_pct || 0).toFixed(1)}%</strong><small>{bytes(Number(data.memory?.used_kb || 0) * 1024)} / {bytes(Number(data.memory?.total_kb || 0) * 1024)}</small></div>
      <div class="metric-card"><span>UPTIME</span><strong>{fmtDuration(data.summary.uptime_seconds || 0)}</strong><small>{data.summary.cpu_count || 0} cores · {data.summary.process_count || 0} processes</small></div>
    </section>

    <div class="two-col">
      <section class="panel">
        <div class="panel-head"><div><strong>CPU по ядрам</strong><span>rolling {data.cpu?.window_seconds || 5} сек</span></div><span class="state-chip {data.cpu?.ready ? 'good' : 'info'}">{data.cpu?.ready ? 'LIVE' : 'SAMPLING'}</span></div>
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
        <div class="panel-head"><div><strong>Memory</strong><span>/proc/meminfo</span></div></div>
        <div class="info-row"><div><strong>Available</strong><span>Доступно процессам</span></div><div class="info-value good">{bytes(Number(data.memory?.available_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>Cached</strong><span>Page cache + reclaimable</span></div><div class="info-value">{bytes(Number(data.memory?.cached_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>Swap</strong><span>Использовано / всего</span></div><div class="info-value">{bytes((Number(data.memory?.swap_total_kb || 0) - Number(data.memory?.swap_free_kb || 0)) * 1024)} / {bytes(Number(data.memory?.swap_total_kb || 0) * 1024)}</div></div>
      </section>
    </div>

  {:else if tab === 'thermal' && data.thermal}
    <div class="module-meta-strip mono">
      <span>SENSORS <strong>{data.thermal.sensor_count || 0}</strong></span>
      <span>CACHE <strong>{data.thermal.cache_seconds || 0}s</strong></span>
      <span>SMARTCTL <strong class={data.thermal.optional_smartctl ? 'good' : 'muted'}>{data.thermal.optional_smartctl ? 'AVAILABLE' : 'OPTIONAL'}</strong></span>
    </div>
    <section class="module-thermal-grid">
      {#if (data.thermal.sensors || []).length}
        {#each data.thermal.sensors as sensor (sensor.id)}
          <article class="module-thermal-card">
            <div class="module-thermal-head"><span class="status-dot {tempClass(sensor)}"></span><div><strong>{sensor.name}</strong><span>{sensor.category}</span></div></div>
            <div class="module-temp {tempClass(sensor)}">{Number(sensor.temp_c).toFixed(1)}°C</div>
            <div class="module-temp-scale"><span style={`width:${Math.min(100, Number(sensor.temp_c || 0) / Number(sensor.critical_c || 100) * 100)}%`}></span></div>
            <div class="module-thermal-foot mono"><span>warn {sensor.warn_c}°</span><span>crit {sensor.critical_c}°</span></div>
            <code title={sensor.source}>{sensor.source}</code>
          </article>
        {/each}
      {:else}
        <section class="panel"><div class="empty">Реальные температурные датчики не обнаружены</div></section>
      {/if}
    </section>

  {:else if tab === 'storage' && data.storage}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Mounted storage</strong><span>statfs + passive /proc/diskstats rates</span></div><span class="state-chip info">NO BENCHMARKS</span></div>
      <div class="table-scroll"><table><thead><tr><th>Mount</th><th>Device</th><th>FS</th><th>Usage</th><th>Space</th><th>Read</th><th>Write</th><th>IOPS R/W</th></tr></thead><tbody>
        {#each data.storage.mounts || [] as item (item.mount)}
          <tr><td><strong>{item.mount}</strong></td><td class="mono">{item.device}</td><td>{item.fs_type}</td><td><strong>{Number(item.used_pct || 0).toFixed(1)}%</strong><div class="progress"><span style={`width:${pct(item.used_pct)}%`}></span></div></td><td class="mono">{bytes(item.used_bytes)} / {bytes(item.total_bytes)}</td><td class="mono good">{rate(item.read_bps)}</td><td class="mono accent-text">{rate(item.write_bps)}</td><td class="mono">{Number(item.read_ops || 0).toFixed(1)} / {Number(item.write_ops || 0).toFixed(1)}</td></tr>
        {/each}
      </tbody></table></div>
    </section>

    <section class="module-card-grid">
      {#each data.storage.disks || [] as disk (disk.name)}
        <article class="module-detail-card">
          <div><strong>{disk.model || disk.name}</strong><span class="mono">{disk.path}</span></div>
          <dl>
            <div><dt>Size</dt><dd>{bytes(disk.size_bytes)}</dd></div>
            <div><dt>Type</dt><dd>{disk.rotational ? 'HDD' : 'Flash / SSD'}</dd></div>
            <div><dt>Read</dt><dd>{rate(disk.rate?.read_bps)}</dd></div>
            <div><dt>Write</dt><dd>{rate(disk.rate?.write_bps)}</dd></div>
          </dl>
        </article>
      {/each}
    </section>

  {:else if tab === 'network' && data.summary}
    <section class="metric-grid module-metric-grid">
      <div class="metric-card"><span>RX</span><strong>{rate(data.summary.rx_bps)}</strong><small>aggregate excluding loopback</small></div>
      <div class="metric-card"><span>TX</span><strong>{rate(data.summary.tx_bps)}</strong><small>aggregate excluding loopback</small></div>
      <div class="metric-card"><span>CONNTRACK</span><strong>{fmtInt(data.summary.conntrack_count || 0)}</strong><small>max {fmtInt(data.summary.conntrack_max || 0)}</small></div>
      <div class="metric-card"><span>ERRORS / DROPS</span><strong>{fmtInt(data.summary.errors || 0)} / {fmtInt(data.summary.drops || 0)}</strong><small>{data.summary.interface_count || 0} interfaces</small></div>
    </section>

    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Interfaces</strong><span>kernel counters + sysfs metadata</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>Interface</th><th>State</th><th>Addresses</th><th>Speed</th><th>RX</th><th>TX</th><th>Errors</th><th>Drops</th></tr></thead><tbody>
        {#each data.interfaces?.interfaces || [] as iface (iface.name)}
          <tr><td><strong>{iface.name}</strong><div class="cell-sub mono">{iface.mac || ''}</div></td><td><span class="state-chip {iface.oper_state === 'up' ? 'good' : 'neutral'}">{iface.oper_state || '—'}</span></td><td class="mono module-addresses">{(iface.addresses || []).join(' · ') || '—'}</td><td>{iface.speed_mbps > 0 ? `${iface.speed_mbps} Mbps` : '—'}<div class="cell-sub">{iface.duplex || ''}</div></td><td class="mono good">{rate(iface.rate?.rx_bps)}</td><td class="mono accent-text">{rate(iface.rate?.tx_bps)}</td><td class="mono">{fmtInt(Number(iface.rx_errors || 0) + Number(iface.tx_errors || 0))}</td><td class="mono">{fmtInt(Number(iface.rx_drops || 0) + Number(iface.tx_drops || 0))}</td></tr>
        {/each}
      </tbody></table></div>
    </section>

    <section class="panel table-panel">
      <div class="panel-head"><div><strong>IPv4 routes</strong><span>/proc/net/route</span></div></div>
      <div class="table-scroll"><table><thead><tr><th>Interface</th><th>Destination</th><th>Mask</th><th>Gateway</th><th>Metric</th><th>Flags</th></tr></thead><tbody>
        {#each data.routes?.routes || [] as route (`${route.interface}-${route.destination}-${route.gateway}-${route.metric}`)}
          <tr><td><strong>{route.interface}</strong></td><td class="mono">{route.destination}</td><td class="mono">{route.mask}</td><td class="mono">{route.gateway}</td><td class="mono">{route.metric}</td><td class="mono">{route.flags}</td></tr>
        {/each}
      </tbody></table></div>
    </section>

  {:else if tab === 'profiling' && data.profiling}
    <div class="two-col">
      <section class="panel">
        <div class="panel-head"><div><strong>Core profiling</strong><span>отдельный pprof listener, не :2233</span></div><span class="state-chip {data.profiling.running ? 'good' : data.profiling.enabled ? 'warn' : 'neutral'}">{data.profiling.running ? 'RUNNING' : data.profiling.enabled ? 'ERROR' : 'DISABLED'}</span></div>
        <div class="info-row"><div><strong>Listen</strong><span>Только loopback</span></div><div class="info-value mono">{data.profiling.listen}</div></div>
        <div class="info-row"><div><strong>Slow request</strong><span>Порог логирования</span></div><div class="info-value mono">{data.profiling.slow_ms} ms</div></div>
        <div class="info-row"><div><strong>Mode</strong><span>Security boundary</span></div><div class="info-value good">{data.profiling.mode}</div></div>
        <div class="info-row"><div><strong>Error</strong><span>Listener startup</span></div><div class="info-value">{data.profiling.error || 'нет'}</div></div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>SSH access</strong><span>pprof не публикуется в LAN</span></div></div>
        <div class="module-code-block mono">ssh -L 6061:127.0.0.1:6061 root@ROUTER</div>
        <div class="module-code-block mono">go tool pprof http://127.0.0.1:6061/debug/pprof/heap</div>
        <div class="module-note">Пакет намеренно не открывает внешний порт. Не-loopback listen блокируется Core.</div>
      </section>
    </div>
  {/if}
</div>
