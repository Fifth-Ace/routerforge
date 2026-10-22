<script>
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import ExternalWebWorkspace from '$lib/components/ExternalWebWorkspace.svelte';
  import { probeCatalogWeb } from '$lib/api.js';
  import { authState } from '$lib/stores/auth.js';
  import { catalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';
  import {
    catalogWebProbeStatusAllowed,
    catalogWebResolvedURL,
    catalogWebSecurityDecision
  } from '$lib/utils.js';
  import { integrationProviders, manageableExternalIntegrations } from '$lib/integrations.js';

  let webWorkspace = null;
  let webProbeBusyId = '';
  let webNotice = '';
  let handledDeepLink = '';

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: providers = integrationProviders(modules, integrations);
  $: externals = manageableExternalIntegrations(modules, integrations);
  $: open = $page.url.searchParams.get('open') || '';
  $: activeProvider = providers.find((item) => item.id === open) || null;
  $: requestedExternal = !activeProvider
    ? externals.find((item) => item.id === open) || null
    : null;

  $: if (!open) handledDeepLink = '';
  $: if (requestedExternal && requestedExternal.id !== handledDeepLink) {
    handledDeepLink = requestedExternal.id;
    void openEmbeddedWeb(requestedExternal, false);
  }


  function managerHref(item) {
    return item?.presentation?.integration?.href || `/integrations?open=${encodeURIComponent(item.id)}`;
  }

  function targetNames(item) {
    const ids = item?.presentation?.integration?.target_ids || [];
    return ids.map((id) => integrations.find((x) => x.id === id)?.name || id).join(', ');
  }

  function webBlockedText(reason) {
    if (reason === 'session-cookie-cross-port') {
      return t(locale, 'integrationsUi.embedCookie');
    }
    if (reason === 'mixed-content') {
      return t(locale, 'integrationsUi.embedMixedContent');
    }
    return t(locale, 'integrationsUi.embedUnavailable');
  }

  async function openEmbeddedWeb(item, syncURL = true) {
    if (!item?.web || webProbeBusyId) return;

    webNotice = '';
    const decision = catalogWebSecurityDecision(item, $authState);
    if (!decision.embedAllowed) {
      webNotice = webBlockedText(decision.reason);
      return;
    }

    webProbeBusyId = item.id;
    try {
      const probe = await probeCatalogWeb(item.id);
      const safe = probe?.reachable === true
        && catalogWebProbeStatusAllowed(item, probe)
        && probe?.redirect === false
        && ['embedded-supported', 'probe-required'].includes(probe?.mode)
        && probe?.embed === true
        && probe?.frame_header_policy === 'no-blocking-header-detected';

      if (probe?.reachable === true && probe?.frame_header_policy === 'blocked') {
        webNotice = t(locale, 'integrationsUi.embedPolicyBlocked');
        return;
      }

      if (!safe) {
        webNotice = t(locale, 'integrationsUi.embedProbeUnsafe');
        return;
      }

      const resolvedURL = catalogWebResolvedURL(item, probe, $authState);
      if (!resolvedURL) {
        webNotice = t(locale, 'integrationsUi.embedNoListener');
        return;
      }

      webWorkspace = { item, url:resolvedURL, probe };

      if (syncURL && open !== item.id) {
        await goto(`/integrations?open=${encodeURIComponent(item.id)}`, {
          keepFocus:true,
          noScroll:true
        });
        handledDeepLink = item.id;
      }
    } catch (error) {
      webNotice = error?.payload?.error || error?.message || t(locale, 'integrationsUi.probeFailed');
    } finally {
      webProbeBusyId = '';
    }
  }

  async function closeExternalWeb() {
    webWorkspace = null;
    webNotice = '';

    if (open && !activeProvider) {
      // Keep the current deep-link marked as handled until goto removes ?open=.
      // Clearing it first lets the reactive deep-link block reopen the workspace
      // while navigation is still in flight.
      handledDeepLink = open;
      await goto('/integrations', { replaceState:true, keepFocus:true, noScroll:true });
      return;
    }

    handledDeepLink = '';
  }
</script>

<svelte:head><title>RouterForge — {t(locale, 'integrationsUi.title')}</title></svelte:head>

{#if activeProvider}
  <div class="page integration-workspace">
    <div class="integration-workspace-head">
      <a class="button" href="/integrations">← {t(locale, 'integrationsUi.allIntegrations')}</a>
      <div>
        <strong>{activeProvider.presentation?.integration?.label || activeProvider.name}</strong>
        <span>{t(locale, 'integrationsUi.managerDescription')}</span>
      </div>
    </div>
    <ModuleFrame moduleId={activeProvider.id} />
  </div>
{:else}
  <div class="page integrations-hub">
    <div class="page-head">
      <div>
        <h1>{t(locale, 'integrationsUi.title')}</h1>
        <p>{t(locale, 'integrationsUi.intro')}</p>
      </div>
    </div>

    {#if webNotice}
      <div class="integration-web-notice">{webNotice}</div>
    {/if}

    {#if providers.length}
      <section>
        <div class="catalog-section-head">
          <div>
            <h2>{t(locale, 'integrationsUi.managerHeading')}</h2>
            <p>{t(locale, 'integrationsUi.managerHint')}</p>
          </div>
        </div>
        <div class="integration-card-grid">
          {#each providers as item (item.id)}
            <article class="panel integration-card">
              <div>
                <span class="state-chip good">{t(locale, 'integrationsUi.ready')}</span>
                <h3>{item.presentation?.integration?.label || item.name}</h3>
                <p>{item.description}</p>
                <small class="mono">{t(locale, 'integrationsUi.target')}: {targetNames(item)}</small>
              </div>
              <a class="button primary" href={managerHref(item)}>{t(locale, 'integrationsUi.openManager')}</a>
            </article>
          {/each}
        </div>
      </section>
    {/if}

    {#if externals.length}
      <section>
        <div class="catalog-section-head">
          <div>
            <h2>{t(locale, 'integrationsUi.externalHeading')}</h2>
            <p>{t(locale, 'integrationsUi.externalHint')}</p>
          </div>
        </div>
        <div class="integration-card-grid">
          {#each externals as item (item.id)}
            <article class="panel integration-card">
              <div>
                <span class="state-chip {item.service_running ? 'good' : 'neutral'}">{item.service_running ? t(locale, 'integrationsUi.running') : t(locale, 'integrationsUi.detected')}</span>
                <h3>{item.name}</h3>
                <p>{item.description}</p>
                <small class="mono">{item.category || t(locale, 'integrationsUi.categoryFallback')}{item.version ? ` · v${item.version}` : ''}</small>
              </div>
              {#if item.web}
                <button
                  class="button"
                  type="button"
                  disabled={Boolean(webProbeBusyId)}
                  onclick={() => openEmbeddedWeb(item)}
                >
                  {webProbeBusyId === item.id ? t(locale, 'integrationsUi.checking') : t(locale, 'integrationsUi.openInterface')}
                </button>
              {:else}
                <span class="integration-managed-note">{t(locale, 'integrationsUi.managedThroughManager')}</span>
              {/if}
            </article>
          {/each}
        </div>
      </section>
    {/if}
  </div>
{/if}

{#if webWorkspace}
  <ExternalWebWorkspace workspace={webWorkspace} {locale} onclose={closeExternalWeb} />
{/if}

<style>
  .integrations-hub{display:grid;gap:22px}
  .integration-card-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:12px}
  .integration-card{padding:14px;display:flex;min-height:190px;flex-direction:column;justify-content:space-between;gap:14px}
  .integration-card h3{margin:10px 0 6px}
  .integration-card p{margin:0 0 8px;color:var(--rf-muted,#8d98a4);line-height:1.45}
  .integration-card small{color:var(--rf-muted,#8d98a4)}
  .integration-managed-note{font-size:12px;color:var(--rf-muted,#8d98a4)}
  .integration-web-notice{padding:10px 12px;border:1px solid var(--rf-accent-border,var(--border));border-radius:8px;background:var(--rf-accent-soft,rgba(56,189,248,.08));font-size:13px}
  .integration-workspace{display:grid;gap:12px}
  .integration-workspace-head{display:flex;align-items:center;gap:14px;margin:0}
  .integration-workspace-head>div{display:grid;gap:3px}
  .integration-workspace-head span{color:var(--rf-muted,#8d98a4);font-size:12px}
</style>
