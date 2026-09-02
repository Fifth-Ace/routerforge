<script>
  import { onMount } from 'svelte';
  import {
    getAdminSummary, getAdminCPU, getAdminProcesses, getAdminPorts,
    getAdminServices, getAdminPackages, getAdminStorage, getAdminThermal
  } from '$lib/api.js';
  import { fmtInt, fmtDuration, bytes } from '$lib/utils.js';

  let available = null;
  let errorText = '';
  let tab = 'overview';
  let summary = null;
  let cpu = [];
  let processes = [];
  let ports = [];
  let services = [];
  let packages = [];
  let storage = [];
  let sensors = [];
  let loading = false;
  let search = '';

  const tabs = [
    ['overview', 'Обзор'],
    ['processes', 'Процессы'],
    ['ports', 'Порты'],
    ['services', 'Службы'],
    ['packages', 'Пакеты'],
    ['storage', 'Хранилища'],
    ['thermal', 'Температуры']
  ];

  $: filteredProcesses = processes.filter((p) => {
    const q = search.trim().toLowerCase();
    return !q || `${p.pid} ${p.name} ${p.user} ${p.command}`.toLowerCase().includes(q);
  });
  $: filteredPorts = ports.filter((p) => {
    const q = search.trim().toLowerCase();
    return !q || `${p.protocol} ${p.local_address} ${p.local_port} ${p.process} ${p.pid}`.toLowerCase().includes(q);
  });
  $: filteredServices = services.filter((s) => {
    const q = search.trim().toLowerCase();
    return !q || `${s.name} ${s.path}`.toLowerCase().includes(q);
  });
  $: filteredPackages = packages.filter((p) => {
    const q = search.trim().toLowerCase();
    return !q || `${p.name} ${p.version} ${p.architecture}`.toLowerCase().includes(q);
  });

  async function loadOverview() {
    loading = true;
    try {
      summary = await getAdminSummary();
      const cpuData = await getAdminCPU();
      cpu = cpuData.cpus || [];
      available = true;
      errorText = '';
    } catch (error) {
      available = false;
      errorText = error?.payload?.error || error?.message || 'dns-monitor-admin недоступен';
    } finally {
      loading = false;
    }
  }

  async function selectTab(next) {
    tab = next;
    search = '';
    if (available === false && next !== 'overview') return;
    loading = true;
    try {
      if (next === 'overview') {
        await loadOverview();
        return;
      }
      if (next === 'processes') processes = (await getAdminProcesses()).processes || [];
      if (next === 'ports') ports = (await getAdminPorts()).ports || [];
      if (next === 'services') services = (await getAdminServices()).services || [];
      if (next === 'packages') packages = (await getAdminPackages()).packages || [];
      if (next === 'storage') storage = (await getAdminStorage()).storage || [];
      if (next === 'thermal') sensors = (await getAdminThermal()).sensors || [];
      available = true;
    } catch (error) {
      available = false;
      errorText = error?.payload?.error || error?.message || 'dns-monitor-admin недоступен';
    } finally {
      loading = false;
    }
  }

  const pct = (n) => Math.max(0, Math.min(100, Number(n || 0)));
  const sectorsBytes = (n) => Number(n || 0) * 512;

  onMount(() => { loadOverview(); });
</script>

<svelte:head><title>DNS Monitor — Админ</title></svelte:head>

<div class="page admin-page">
  <div class="page-head">
    <div>
      <h1>Admin Tools</h1>
      <p>Опциональный системный модуль DNS Monitor. Сейчас только чтение — никаких скрытых shell-команд и мутаций.</p>
    </div>
    <span class="state-chip {available === true ? 'good' : available === false ? 'warn' : 'info'}">
      {available === true ? 'MODULE ONLINE' : available === false ? 'MODULE OFFLINE' : 'CHECKING'}
    </span>
  </div>

  <div class="admin-safety-banner">
    <div><strong>READ-ONLY ADMIN</strong><span>CPU · RAM · процессы · порты · службы · opkg · storage · thermal</span></div>
    <div class="mono"><span>TERMINAL</span><strong>LOCKED</strong><span>WRITES</span><strong>LOCKED</strong></div>
  </div>

  <div class="subtabs admin-tabs">
    {#each tabs as [id, label]}
      <button class:active={tab === id} onclick={() => selectTab(id)}>{label}</button>
    {/each}
  </div>

  {#if available === false}
    <section class="panel admin-unavailable">
      <div class="panel-head">
        <div><strong>Admin module не запущен</strong><span>Core работает нормально; системная админка вынесена в отдельный пакет.</span></div>
        <span class="state-chip warn">OPTIONAL MODULE</span>
      </div>
      <div class="admin-install-hint">
        <p>{errorText}</p>
        <code>opkg install dns-monitor-admin</code>
        <span>Для dev-теста ставим отдельный IPK `dns-monitor-admin_0.2.0-dev_aarch64-3.10.ipk`.</span>
      </div>
    </section>
  {:else if loading && !summary}
    <section class="panel"><div class="empty">Читаю системную телеметрию…</div></section>
  {:else if tab === 'overview' && summary}
    <section class="metric-grid admin-metric-grid">
      <div class="metric-card"><span>Host</span><strong class="admin-host">{summary.hostname || '—'}</strong><small>{summary.kernel || '—'} · {summary.architecture || '—'}</small></div>
      <div class="metric-card"><span>Load 1 / 5 / 15</span><strong>{Number(summary.load_1 || 0).toFixed(2)}</strong><small>{Number(summary.load_5 || 0).toFixed(2)} · {Number(summary.load_15 || 0).toFixed(2)}</small></div>
      <div class="metric-card"><span>RAM</span><strong>{summary.memory?.used_pct ? `${Number(summary.memory.used_pct).toFixed(1)}%` : '—'}</strong><small>{bytes(Number(summary.memory?.used_kb || 0) * 1024)} / {bytes(Number(summary.memory?.total_kb || 0) * 1024)}</small></div>
      <div class="metric-card"><span>Processes</span><strong>{fmtInt(summary.process_count || 0)}</strong><small>{summary.cpu_count || 0} CPU cores · uptime {fmtDuration(summary.uptime_seconds || 0)}</small></div>
    </section>

    <div class="two-col admin-overview-grid">
      <section class="panel">
        <div class="panel-head"><div><strong>CPU по ядрам</strong><span>220 ms sample из /proc/stat</span></div><span class="state-chip good">{cpu.length} CORES</span></div>
        <div class="cpu-list">
          {#each cpu as core (core.name)}
            <div class="cpu-row">
              <div><strong>{core.name}</strong><span>{Number(core.usage || 0).toFixed(1)}%</span></div>
              <div class="progress"><span style={`width:${pct(core.usage)}%`}></span></div>
            </div>
          {/each}
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><div><strong>Memory</strong><span>/proc/meminfo</span></div><span class="state-chip info">{Number(summary.memory?.used_pct || 0).toFixed(1)}%</span></div>
        <div class="info-row"><div><strong>Total</strong><span>Физическая RAM</span></div><div class="info-value">{bytes(Number(summary.memory?.total_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>Available</strong><span>Доступно ядру и процессам</span></div><div class="info-value good">{bytes(Number(summary.memory?.available_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>Cached</strong><span>Page cache + reclaimable</span></div><div class="info-value">{bytes(Number(summary.memory?.cached_kb || 0) * 1024)}</div></div>
        <div class="info-row"><div><strong>Swap</strong><span>Использовано / всего</span></div><div class="info-value">{bytes((Number(summary.memory?.swap_total_kb || 0) - Number(summary.memory?.swap_free_kb || 0)) * 1024)} / {bytes(Number(summary.memory?.swap_total_kb || 0) * 1024)}</div></div>
      </section>
    </div>
  {:else if tab === 'processes'}
    <div class="toolbar">
      <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="PID, процесс, пользователь, команда…"/></div>
      <span class="panel-meta">{filteredProcesses.length} / {processes.length}</span>
    </div>
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Процессы</strong><span>Top 300 по RSS</span></div><span class="state-chip info">READ ONLY</span></div>
      <div class="table-scroll"><table><thead><tr><th>PID</th><th>Process</th><th>User</th><th>State</th><th>RSS</th><th>VmSize</th><th>Threads</th><th>Command</th></tr></thead><tbody>
        {#each filteredProcesses as p (p.pid)}
          <tr><td class="mono">{p.pid}</td><td><strong>{p.name}</strong></td><td>{p.user}</td><td><span class="pill">{p.state || '—'}</span></td><td class="mono">{bytes(Number(p.rss_kb || 0) * 1024)}</td><td class="mono">{bytes(Number(p.vmsize_kb || 0) * 1024)}</td><td class="mono">{p.threads || 0}</td><td class="mono admin-command">{p.command}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'ports'}
    <div class="toolbar"><div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Порт, адрес, процесс, PID…"/></div><span class="panel-meta">{filteredPorts.length} sockets</span></div>
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Listening sockets</strong><span>/proc/net/tcp* + udp*</span></div><span class="state-chip info">READ ONLY</span></div>
      <div class="table-scroll"><table><thead><tr><th>Proto</th><th>Local</th><th>State</th><th>PID</th><th>Process</th></tr></thead><tbody>
        {#each filteredPorts as p (`${p.protocol}-${p.local_address}-${p.local_port}-${p.inode}`)}
          <tr><td><span class="pill accent">{p.protocol}</span></td><td class="mono">{p.local_address}:{p.local_port}</td><td>{p.state}</td><td class="mono">{p.pid || '—'}</td><td>{p.process || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'services'}
    <div class="toolbar"><div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Служба или init script…"/></div><span class="panel-meta">{filteredServices.length} init scripts</span></div>
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Entware services</strong><span>Определение running без запуска init scripts</span></div><span class="state-chip info">READ ONLY</span></div>
      <div class="table-scroll"><table><thead><tr><th>Service</th><th>State</th><th>Executable</th><th>Init script</th><th>Detected by</th></tr></thead><tbody>
        {#each filteredServices as s (s.path)}
          <tr><td><strong>{s.name}</strong></td><td><span class="state-chip {s.running ? 'good' : 'neutral'}">{s.running ? 'RUNNING' : 'NOT DETECTED'}</span></td><td>{s.executable ? 'yes' : 'no'}</td><td class="mono">{s.path}</td><td class="cell-sub">{s.running_source || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'packages'}
    <div class="toolbar"><div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Пакет, версия, архитектура…"/></div><span class="panel-meta">{filteredPackages.length} packages</span></div>
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Entware packages</strong><span>/opt/lib/opkg/status</span></div><span class="state-chip info">READ ONLY</span></div>
      <div class="table-scroll"><table><thead><tr><th>Package</th><th>Version</th><th>Architecture</th><th>Status</th></tr></thead><tbody>
        {#each filteredPackages as p (p.name)}
          <tr><td><strong>{p.name}</strong></td><td class="mono">{p.version}</td><td class="mono">{p.architecture || '—'}</td><td class="cell-sub">{p.status || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'storage'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Storage / mounts</strong><span>statfs + /proc/diskstats counters</span></div><span class="state-chip info">NO BENCHMARKS</span></div>
      <div class="table-scroll"><table><thead><tr><th>Mount</th><th>Device</th><th>FS</th><th>Used</th><th>Space</th><th>Reads</th><th>Writes</th><th>Read I/O</th><th>Write I/O</th></tr></thead><tbody>
        {#each storage as s (s.mount)}
          <tr><td><strong>{s.mount}</strong></td><td class="mono">{s.device}</td><td>{s.fs_type}</td><td><strong>{Number(s.used_pct || 0).toFixed(1)}%</strong><div class="progress"><span style={`width:${pct(s.used_pct)}%`}></span></div></td><td class="mono">{bytes(s.used_bytes)} / {bytes(s.total_bytes)}</td><td class="mono">{fmtInt(s.reads || 0)}</td><td class="mono">{fmtInt(s.writes || 0)}</td><td class="mono">{bytes(sectorsBytes(s.read_sectors))}</td><td class="mono">{bytes(sectorsBytes(s.write_sectors))}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'thermal'}
    <section class="thermal-grid">
      {#if sensors.length}
        {#each sensors as sensor (sensor.id)}
          <article class="thermal-card">
            <div><span class="status-dot {Number(sensor.temp_c) >= 80 ? 'error' : Number(sensor.temp_c) >= 70 ? 'warn' : 'good'}"></span><strong>{sensor.name}</strong></div>
            <span class="thermal-value">{Number(sensor.temp_c).toFixed(1)}°C</span>
            <code>{sensor.source}</code>
          </article>
        {/each}
      {:else}
        <section class="panel"><div class="empty">Реальные thermal/hwmon датчики не обнаружены</div></section>
      {/if}
    </section>
  {/if}
</div>
