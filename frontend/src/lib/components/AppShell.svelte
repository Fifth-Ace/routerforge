<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline, startCatalogPolling } from '$lib/stores/catalog.js';
  import { adminSummary, adminOnline, startAdminPolling } from '$lib/stores/admin.js';
  import { fmtDuration } from '$lib/utils.js';

  let stopCatalog = null;
  let stopAdmin = null;

  onMount(() => {
    stopCatalog = startCatalogPolling();
    stopAdmin = startAdminPolling();
    return () => {
      stopCatalog?.();
      stopAdmin?.();
    };
  });

  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: all = [...integrations, ...modules];
  $: externalInstalled = integrations.filter((x) => x.state === 'installed_external').length;
  $: servicesRunning = integrations.filter((x) => x.service_running).length
    + modules.filter((x) => x.id === 'admin' && x.service_running).length;
  $: available = all.filter((x) => x.state === 'available').length;
  $: adminModule = modules.find((x) => x.id === 'admin');
  $: mem = $adminSummary?.memory;
  $: memText = mem?.total_kb ? `${(Number(mem.used_kb || 0) / 1024).toFixed(0)} / ${(Number(mem.total_kb || 0) / 1024).toFixed(0)} MB` : '—';

  function stateLabel(item) {
    if (item.state === 'installed' && item.managed) return item.service_running ? 'ACTIVE' : 'INSTALLED';
    if (item.state === 'installed') return 'BUILT-IN';
    if (item.state === 'installed_external') return item.service_running ? 'ACTIVE' : 'INSTALLED';
    if (item.state === 'planned') return 'PLANNED';
    if (item.state === 'available') return 'AVAILABLE';
    return String(item.state || 'UNKNOWN').toUpperCase();
  }

  function stateClass(item) {
    if (item.state === 'installed' && item.managed) return item.service_running ? 'good' : 'warn';
    if (item.state === 'installed') return 'good';
    if (item.state === 'installed_external' && item.service_running) return 'good';
    if (item.state === 'available') return 'info';
    if (item.state === 'planned') return 'muted';
    return item.service_running === false && item.installed ? 'warn' : 'muted';
  }
</script>

<div class="global-shell">
  <aside class="global-rail global-rail-left">
    <div>
      <div class="rail-section-label">Core Modules Tree</div>
      <div class="global-module-tree mono">
        <div class="global-tree-root">
          <span class="status-dot good"></span>
          <strong>DNS Monitor Core</strong>
        </div>

        {#each modules as module, index (module.id)}
          <div class="global-tree-row">
            <span class="tree-branch">{index === modules.length - 1 ? '└─' : '├─'}</span>
            <span class="status-dot {stateClass(module)}"></span>
            <span class="global-tree-name">{module.name}</span>
            <span class="global-tree-state {stateClass(module)}">[{stateLabel(module)}]</span>
          </div>
        {/each}
      </div>

      <div class="rail-section">
        <div class="rail-section-label">Registry Summary</div>
        <div class="rail-summary-row"><span>External installed</span><strong>{externalInstalled}</strong></div>
        <div class="rail-summary-row"><span>Services running</span><strong>{servicesRunning}</strong></div>
        <div class="rail-summary-row"><span>Available</span><strong>{available}</strong></div>
        <div class="rail-summary-row"><span>Catalog entries</span><strong>{all.length}</strong></div>
      </div>
    </div>

    <div class="rail-status-card mono">
      <div class="rail-section-label">Marketplace Safety</div>
      <div><span>Catalog API</span><strong class={$catalogOnline ? 'good' : 'bad'}>{$catalogOnline ? 'ONLINE' : 'OFFLINE'}</strong></div>
      <div><span>Mutation API</span><strong>DISABLED</strong></div>
      <div><span>Mode</span><strong>READ-ONLY</strong></div>
    </div>
  </aside>

  <div class="global-content">
    <slot />
  </div>

  <aside class="global-rail global-rail-right">
    <div>
      <div class="rail-section-label">Admin Module</div>
      <div class="rail-status-card mono">
        <div><span>Package</span><strong class={adminModule?.installed ? 'good' : 'info'}>{adminModule?.installed ? 'INSTALLED' : 'OPTIONAL'}</strong></div>
        <div><span>Service</span><strong class={$adminOnline ? 'good' : adminModule?.installed ? 'warn' : 'muted'}>{$adminOnline ? 'ONLINE' : adminModule?.installed ? 'STOPPED' : 'NOT INSTALLED'}</strong></div>
        <div><span>Mutation API</span><strong>DISABLED</strong></div>
      </div>
    </div>

    <div class="rail-section">
      <div class="rail-section-label">System Quick View</div>
      {#if $adminOnline && $adminSummary}
        <div class="rail-summary-row"><span>Host</span><strong>{$adminSummary.hostname || '—'}</strong></div>
        <div class="rail-summary-row"><span>Kernel</span><strong class="rail-ellipsis">{$adminSummary.kernel || '—'}</strong></div>
        <div class="rail-summary-row"><span>CPU</span><strong>{$adminSummary.cpu_count || 0} cores</strong></div>
        <div class="rail-summary-row"><span>Load</span><strong>{Number($adminSummary.load_1 || 0).toFixed(2)}</strong></div>
        <div class="rail-summary-row"><span>RAM</span><strong>{memText}</strong></div>
        <div class="rail-summary-row"><span>Processes</span><strong>{$adminSummary.process_count || 0}</strong></div>
        <div class="rail-summary-row"><span>Uptime</span><strong>{fmtDuration($adminSummary.uptime_seconds || 0)}</strong></div>
      {:else}
        <div class="rail-empty">
          {adminModule?.installed ? 'Admin helper не отвечает' : 'Установи dns-monitor-admin — здесь появится системная телеметрия.'}
        </div>
      {/if}
    </div>

    <div class="rail-status-card mono rail-bottom-card">
      <div class="rail-section-label">Safety Boundary</div>
      <div><span>Processes</span><strong>READ</strong></div>
      <div><span>Services</span><strong>READ</strong></div>
      <div><span>Packages</span><strong>READ</strong></div>
      <div><span>Terminal</span><strong class="muted">LOCKED</strong></div>
      <div><span>Writes</span><strong class="muted">LOCKED</strong></div>
    </div>
  </aside>
</div>
