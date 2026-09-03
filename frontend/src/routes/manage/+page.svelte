<script>
  import { onMount } from 'svelte';
  import { getAdminProcesses, getAdminPorts, getAdminServices, getAdminPackages } from '$lib/api.js';
  import { bytes } from '$lib/utils.js';

  let tab = 'processes';
  let processes = [];
  let ports = [];
  let services = [];
  let packages = [];
  let search = '';
  let loading = false;
  let errorText = '';

  const tabs = [
    ['processes', 'Процессы'],
    ['ports', 'Порты'],
    ['services', 'Службы'],
    ['packages', 'Пакеты']
  ];

  $: q = search.trim().toLowerCase();
  $: filteredProcesses = processes.filter((p) => !q || `${p.pid} ${p.name} ${p.user} ${p.command}`.toLowerCase().includes(q));
  $: filteredPorts = ports.filter((p) => !q || `${p.protocol} ${p.local_address} ${p.local_port} ${p.process} ${p.pid}`.toLowerCase().includes(q));
  $: filteredServices = services.filter((s) => !q || `${s.name} ${s.path}`.toLowerCase().includes(q));
  $: filteredPackages = packages.filter((p) => !q || `${p.name} ${p.version} ${p.architecture}`.toLowerCase().includes(q));

  async function load(next = tab) {
    loading = true;
    errorText = '';
    try {
      if (next === 'processes') processes = (await getAdminProcesses()).processes || [];
      if (next === 'ports') ports = (await getAdminPorts()).ports || [];
      if (next === 'services') services = (await getAdminServices()).services || [];
      if (next === 'packages') packages = (await getAdminPackages()).packages || [];
    } catch (error) {
      errorText = error?.payload?.error || error?.message || 'RouterForge Control недоступен';
    } finally {
      loading = false;
    }
  }

  function selectTab(next) {
    tab = next;
    search = '';
    load(next);
  }

  onMount(() => {
    load();
    const timer = setInterval(() => {
      if (!document.hidden && (tab === 'processes' || tab === 'ports')) load(tab);
    }, 4000);
    return () => clearInterval(timer);
  });
</script>

<svelte:head><title>RouterForge — Управление</title></svelte:head>

<div class="page admin-page">
  <div class="page-head">
    <div>
      <h1>Управление</h1>
      <p>Процессы, listening sockets, Entware services и установленные пакеты. Системная телеметрия вынесена в «Мониторинг».</p>
    </div>
    <span class="state-chip info">CONTROL / BETA</span>
  </div>

  <div class="admin-safety-banner">
    <div class="admin-safety-main">
      <strong>ROUTERFORGE CONTROL</strong>
      <span>Диагностика и инвентаризация. Опасные действия пока заблокированы до отдельного permission layer.</span>
    </div>
    <div class="admin-lock-groups mono">
      <span class="admin-lock-group"><em>INSPECT</em><strong>ENABLED</strong></span>
      <span class="admin-lock-group"><em>MUTATE</em><strong>LOCKED</strong></span>
    </div>
  </div>

  <div class="subtabs admin-tabs">
    {#each tabs as [id, label]}
      <button class:active={tab === id} onclick={() => selectTab(id)}>{label}</button>
    {/each}
  </div>

  <div class="toolbar">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder="Поиск…"/></div>
    <button class="button" onclick={() => load(tab)} disabled={loading}>↻ Обновить</button>
  </div>

  {#if errorText}
    <section class="panel"><div class="empty">{errorText}</div></section>
  {:else if loading && !processes.length && !ports.length && !services.length && !packages.length}
    <section class="panel"><div class="empty">Читаю RouterForge Control…</div></section>
  {:else if tab === 'processes'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Процессы</strong><span>Top 300 по RSS</span></div><span class="state-chip info">{filteredProcesses.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>PID</th><th>Process</th><th>User</th><th>State</th><th>RSS</th><th>VmSize</th><th>Threads</th><th>Command</th></tr></thead><tbody>
        {#each filteredProcesses as p (p.pid)}
          <tr><td class="mono">{p.pid}</td><td><strong>{p.name}</strong></td><td>{p.user}</td><td><span class="pill">{p.state || '—'}</span></td><td class="mono">{bytes(Number(p.rss_kb || 0) * 1024)}</td><td class="mono">{bytes(Number(p.vmsize_kb || 0) * 1024)}</td><td class="mono">{p.threads || 0}</td><td class="mono admin-command">{p.command}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'ports'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Listening sockets</strong><span>/proc/net/tcp* + udp*</span></div><span class="state-chip info">{filteredPorts.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>Proto</th><th>Local</th><th>State</th><th>PID</th><th>Process</th></tr></thead><tbody>
        {#each filteredPorts as p (`${p.protocol}-${p.local_address}-${p.local_port}-${p.inode}`)}
          <tr><td><span class="pill accent">{p.protocol}</span></td><td class="mono">{p.local_address}:{p.local_port}</td><td>{p.state}</td><td class="mono">{p.pid || '—'}</td><td>{p.process || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'services'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Entware services</strong><span>Состояние init scripts без их запуска</span></div><span class="state-chip info">{filteredServices.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>Service</th><th>State</th><th>Executable</th><th>Init script</th><th>Detected by</th></tr></thead><tbody>
        {#each filteredServices as s (s.path)}
          <tr><td><strong>{s.name}</strong></td><td><span class="state-chip {s.running ? 'good' : 'neutral'}">{s.running ? 'RUNNING' : 'NOT DETECTED'}</span></td><td>{s.executable ? 'yes' : 'no'}</td><td class="mono">{s.path}</td><td class="cell-sub">{s.running_source || '—'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {:else if tab === 'packages'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>Entware packages</strong><span>/opt/lib/opkg/status</span></div><span class="state-chip info">{filteredPackages.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>Package</th><th>Version</th><th>Architecture</th><th>Status</th></tr></thead><tbody>
        {#each filteredPackages as p (p.name)}
          <tr><td><strong>{p.name}</strong></td><td class="mono">{p.version || '—'}</td><td>{p.architecture || '—'}</td><td>{p.status || 'installed'}</td></tr>
        {/each}
      </tbody></table></div>
    </section>
  {/if}
</div>
