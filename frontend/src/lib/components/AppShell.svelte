<script>
  import { onMount } from 'svelte';
  import { catalog, catalogOnline, startCatalogPolling } from '$lib/stores/catalog.js';
  import { adminSummary, adminOnline, startAdminPolling } from '$lib/stores/admin.js';

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
    + modules.filter((x) => x.managed && x.service_running).length;
  $: available = all.filter((x) => x.state === 'available').length;
  $: adminModule = modules.find((x) => x.id === 'admin');
  $: mem = $adminSummary?.memory;
  $: ramPct = mem?.total_kb ? Number(mem.used_kb || 0) / Number(mem.total_kb) * 100 : 0;

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
    return item.installed && !item.service_running ? 'warn' : 'muted';
  }
</script>

<div class="global-shell">
  <aside class="global-rail global-rail-left">
    <div class="rail-main">
      <section class="rail-block">
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
      </section>

      <section class="rail-block">
        <div class="rail-section-label">Registry Summary</div>
        <div class="rail-summary-row"><span>External installed</span><strong>{externalInstalled}</strong></div>
        <div class="rail-summary-row"><span>Services running</span><strong>{servicesRunning}</strong></div>
        <div class="rail-summary-row"><span>Available</span><strong>{available}</strong></div>
        <div class="rail-summary-row"><span>Catalog entries</span><strong>{all.length}</strong></div>
      </section>
    </div>

    <div class="rail-bottom-stack">
      <section class="rail-status-card mono">
        <div class="rail-section-label">Core Telemetry</div>
        {#if $adminOnline && $adminSummary}
          <div><span>host</span><strong>{$adminSummary.hostname || '—'}</strong></div>
          <div><span>load.1m</span><strong>{Number($adminSummary.load_1 || 0).toFixed(2)}</strong></div>
          <div><span>ram.used</span><strong>{ramPct.toFixed(0)}%</strong></div>
          <div><span>processes</span><strong>{$adminSummary.process_count || 0}</strong></div>
        {:else}
          <div><span>admin</span><strong class={adminModule?.installed ? 'warn' : 'muted'}>{adminModule?.installed ? 'OFFLINE' : 'OPTIONAL'}</strong></div>
          <div><span>telemetry</span><strong class="muted">N/A</strong></div>
        {/if}
      </section>

      <section class="rail-status-card mono">
        <div class="rail-section-label">Marketplace Safety</div>
        <div><span>Catalog API</span><strong class={$catalogOnline ? 'good' : 'bad'}>{$catalogOnline ? 'ONLINE' : 'OFFLINE'}</strong></div>
        <div><span>Mutation API</span><strong>DISABLED</strong></div>
        <div><span>Mode</span><strong>READ-ONLY</strong></div>
      </section>
    </div>
  </aside>

  <div class="global-content">
    <slot />
  </div>
</div>
