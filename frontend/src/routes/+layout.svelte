<script>
  import '@fontsource/inter/400.css';
  import '@fontsource/inter/500.css';
  import '@fontsource/inter/600.css';
  import '@fontsource/inter/700.css';
  import '@fontsource/roboto-mono/400.css';
  import '@fontsource/roboto-mono/500.css';
  import '@fontsource/roboto-mono/600.css';
  import '../app.css';
  import '../admin-shell.css';
  import '../routerforge-beta.css';

  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import AppHeader from '$lib/components/AppHeader.svelte';
  import AppShell from '$lib/components/AppShell.svelte';
  import AuthGate from '$lib/components/AuthGate.svelte';
  import { snapshot, startSnapshotStream } from '$lib/stores/snapshot.js';
  import { settings } from '$lib/stores/settings.js';
  import { catalog, catalogOnline } from '$lib/stores/catalog.js';
  import { authState, refreshAuth } from '$lib/stores/auth.js';
  import { t } from '$lib/i18n/index.js';

  let stopStream = null;
  let lastInterval = 0;
  let panelAccess = false;
  let redirecting = false;

  $: locale = $settings.locale || 'ru';

  function reconcileStream() {
    if (!panelAccess) {
      stopStream?.(); stopStream = null;
      return;
    }
    if (stopStream) return;
    stopStream = startSnapshotStream(lastInterval || 2000);
  }

  $: modules = $catalog.modules || [];
  $: adminInstalled = modules.some((item) => item.id === 'admin' && item.installed);
  $: dnsInstalled = modules.some((item) => item.id === 'dns' && item.installed);
  $: monitoringInstalled = modules.some((item) =>
    ['system', 'thermal', 'storage', 'network'].includes(item.id) && item.installed
  );
  $: path = $page.url.pathname;
  $: protectedPathMissing =
    (path === '/manage' && !adminInstalled)
    || ((path === '/monitoring' || path.startsWith('/monitoring/')) && !monitoringInstalled)
    || ((path === '/dns' || path.startsWith('/dns/')) && !dnsInstalled);

  $: if ($catalogOnline && protectedPathMissing && !redirecting) {
    redirecting = true;
    goto('/catalog', { replaceState: true }).finally(() => { redirecting = false; });
  }

  onMount(() => {
    const unsubscribeSettings = settings.subscribe((value) => {
      const interval = Number(value.refreshMs || 2000);
      if (interval === lastInterval) return;
      lastInterval = interval;
      if (stopStream) { stopStream(); stopStream = null; }
      reconcileStream();
    });

    const unsubscribeAuth = authState.subscribe((value) => {
      panelAccess = Boolean(value.ready && (!value.required || value.authenticated));
      reconcileStream();
    });

    refreshAuth().catch(() => {});
    const authTimer = setInterval(() => refreshAuth().catch(() => {}), 60000);

    return () => {
      clearInterval(authTimer);
      unsubscribeSettings();
      unsubscribeAuth();
      stopStream?.();
    };
  });
</script>

{#if $authState.ready && (!$authState.required || $authState.authenticated)}
  <AppHeader />
  <main class="app-main">
    <AppShell>
      {#if $snapshot.discovery_error || $snapshot.capture_error || $snapshot.client_registry_error || $snapshot.client_capture_error}
        <div class="global-alerts shell-alerts">
          {#if $snapshot.discovery_error}<div class="global-alert">{t(locale, 'errors.dnsDiscovery')}: {$snapshot.discovery_error}</div>{/if}
          {#if $snapshot.capture_error}<div class="global-alert">{t(locale, 'errors.dnsCapture')}: {$snapshot.capture_error}</div>{/if}
          {#if $snapshot.client_registry_error}<div class="global-alert">{t(locale, 'errors.dnsClients')}: {$snapshot.client_registry_error}</div>{/if}
          {#if $snapshot.client_capture_error}<div class="global-alert">{t(locale, 'errors.dnsClientCapture')}: {$snapshot.client_capture_error}</div>{/if}
        </div>
      {/if}
      <slot />
    </AppShell>
  </main>
{:else}
  <AuthGate />
{/if}
