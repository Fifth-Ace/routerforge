<script>
  import { onMount } from 'svelte';
  import { getAdminProcesses, getAdminPorts, getAdminServices, getAdminPackages } from '$lib/api.js';
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

  $: locale = $settings.locale || 'ru';
  $: tabs = [
    ['processes', t(locale, 'manage.tabs.processes')],
    ['ports', t(locale, 'manage.tabs.ports')],
    ['services', t(locale, 'manage.tabs.services')],
    ['packages', t(locale, 'manage.tabs.packages')]
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
      errorText = error?.payload?.error || error?.message || t(locale, 'errors.controlUnavailable');
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

<svelte:head><title>RouterForge — {t(locale, 'manage.pageTitle')}</title></svelte:head>

<div class="page admin-page">
  <div class="page-head">
    <div><h1>{t(locale, 'manage.pageTitle')}</h1><p>{t(locale, 'manage.subtitle')}</p></div>
    <span class="state-chip info">CONTROL / BETA</span>
  </div>

  <div class="admin-safety-banner">
    <div class="admin-safety-main"><strong>ROUTERFORGE CONTROL</strong><span>{t(locale, 'manage.safety')}</span></div>
    <div class="admin-lock-groups mono">
      <span class="admin-lock-group"><em>{t(locale, 'manage.inspect')}</em><strong>{t(locale, 'manage.enabled')}</strong></span>
      <span class="admin-lock-group"><em>{t(locale, 'manage.mutate')}</em><strong>{t(locale, 'manage.locked')}</strong></span>
    </div>
  </div>

  <div class="subtabs admin-tabs">
    {#each tabs as [id, label]}<button class:active={tab === id} onclick={() => selectTab(id)}>{label}</button>{/each}
  </div>

  <div class="toolbar">
    <div class="search-control flex"><span>⌕</span><input bind:value={search} placeholder={t(locale, 'common.search')}/></div>
    <button class="button" onclick={() => load(tab)} disabled={loading}>↻ {t(locale, 'common.refresh')}</button>
  </div>

  {#if errorText}
    <section class="panel"><div class="empty">{errorText}</div></section>
  {:else if loading && !processes.length && !ports.length && !services.length && !packages.length}
    <section class="panel"><div class="empty">{t(locale, 'manage.loading')}</div></section>
  {:else if tab === 'processes'}
    <section class="panel table-panel">
      <div class="panel-head"><div><strong>{t(locale, 'manage.tabs.processes')}</strong><span>{t(locale, 'manage.topRss')}</span></div><span class="state-chip info">{filteredProcesses.length}</span></div>
      <div class="table-scroll"><table><thead><tr><th>PID</th><th>{t(locale, 'manage.columns.process')}</th><th>{t(locale, 'manage.columns.user')}</th><th>{t(locale, 'manage.columns.state')}</th><th>RSS</th><th>VmSize</th><th>{t(locale, 'manage.columns.threads')}</th><th>{t(locale, 'manage.columns.command')}</th></tr></thead><tbody>
        {#each filteredProcesses as p (p.pid)}
          <tr><td class="mono">{p.pid}</td><td><strong>{p.name}</strong></td><td>{p.user}</td><td><span class="pill">{p.state || '—'}</span></td><td class="mono">{bytes(Number(p.rss_kb || 0) * 1024)}</td><td class="mono">{bytes(Number(p.vmsize_kb || 0) * 1024)}</td><td class="mono">{p.threads || 0}</td><td class="mono admin-command">{p.command}</td></tr>
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
      <div class="table-scroll"><table><thead><tr><th>{t(locale, 'manage.columns.service')}</th><th>{t(locale, 'manage.columns.state')}</th><th>{t(locale, 'manage.columns.executable')}</th><th>{t(locale, 'manage.columns.initScript')}</th><th>{t(locale, 'manage.columns.detectedBy')}</th></tr></thead><tbody>
        {#each filteredServices as s (s.path)}
          <tr><td><strong>{s.name}</strong></td><td><span class="state-chip {s.running ? 'good' : 'neutral'}">{s.running ? t(locale, 'common.running').toUpperCase() : t(locale, 'common.notDetected').toUpperCase()}</span></td><td>{s.executable ? t(locale, 'common.yes') : t(locale, 'common.no')}</td><td class="mono">{s.path}</td><td class="cell-sub">{s.running_source || '—'}</td></tr>
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
  {/if}
</div>
