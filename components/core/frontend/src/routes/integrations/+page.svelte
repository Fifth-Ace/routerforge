<script>
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import ExternalWebWorkspace from '$lib/components/ExternalWebWorkspace.svelte';
  import { probeCatalogWeb } from '$lib/api.js';
  import { authState } from '$lib/stores/auth.js';
  import { catalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
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

  const text = (ru, en) => locale === 'ru' ? ru : en;

  function managerHref(item) {
    return item?.presentation?.integration?.href || `/integrations?open=${encodeURIComponent(item.id)}`;
  }

  function targetNames(item) {
    const ids = item?.presentation?.integration?.target_ids || [];
    return ids.map((id) => integrations.find((x) => x.id === id)?.name || id).join(', ');
  }

  function webBlockedText(reason) {
    if (reason === 'session-cookie-cross-port') {
      return text(
        'Встроенный Web UI заблокирован: cookie сессии RouterForge не изолируются по TCP-порту.',
        'Embedded Web UI is blocked because RouterForge session cookies are not isolated by TCP port.'
      );
    }
    if (reason === 'mixed-content') {
      return text(
        'Встроенный режим заблокирован: HTTPS RouterForge не может встраивать HTTP Web UI.',
        'Embedded mode is blocked because HTTPS RouterForge cannot embed an HTTP Web UI.'
      );
    }
    return text(
      'Внешний Web UI сейчас нельзя безопасно открыть внутри RouterForge.',
      'The external Web UI cannot be safely embedded in RouterForge right now.'
    );
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
        webNotice = text(
          'Web UI найден, но приложение запрещает встраивание через X-Frame-Options/CSP.',
          'Web UI was detected, but the application blocks embedding through X-Frame-Options/CSP.'
        );
        return;
      }

      if (!safe) {
        webNotice = text(
          'Runtime web-probe не подтвердил безопасный iframe.',
          'The runtime web probe did not confirm a safe iframe.'
        );
        return;
      }

      const resolvedURL = catalogWebResolvedURL(item, probe, $authState);
      if (!resolvedURL) {
        webNotice = text(
          'Не найден безопасный Web UI listener, доступный браузеру.',
          'No safe browser-reachable Web UI listener was resolved.'
        );
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
      webNotice = error?.payload?.error || error?.message || 'web probe failed';
    } finally {
      webProbeBusyId = '';
    }
  }

  async function closeExternalWeb() {
    webWorkspace = null;
    webNotice = '';
    handledDeepLink = '';
    if (open && !activeProvider) {
      await goto('/integrations', { replaceState:true, keepFocus:true, noScroll:true });
    }
  }
</script>

<svelte:head><title>RouterForge — {text('Интеграции','Integrations')}</title></svelte:head>

{#if activeProvider}
  <div class="page integration-workspace">
    <div class="integration-workspace-head">
      <a class="button" href="/integrations">← {text('Все интеграции','All integrations')}</a>
      <div>
        <strong>{activeProvider.presentation?.integration?.label || activeProvider.name}</strong>
        <span>{text('RouterForge extension для внешней интеграции','RouterForge extension for an external integration')}</span>
      </div>
    </div>
    <ModuleFrame moduleId={activeProvider.id} />
  </div>
{:else}
  <div class="page integrations-hub">
    <div class="page-head">
      <div>
        <span class="routerforge-eyebrow mono">ROUTERFORGE / INTEGRATIONS</span>
        <h1>{text('Интеграции','Integrations')}</h1>
        <p>{text(
          'Внешние приложения и устанавливаемые надстройки RouterForge для управления ими.',
          'External applications and installable RouterForge extensions that manage them.'
        )}</p>
      </div>
    </div>

    {#if webNotice}
      <div class="integration-web-notice">{webNotice}</div>
    {/if}

    {#if providers.length}
      <section>
        <div class="catalog-section-head">
          <div>
            <h2>{text('Расширения RouterForge','RouterForge extensions')}</h2>
            <p>{text('Отдельные устанавливаемые модули. Они не являются частью Управления или Core.','Separate installable modules. They are not part of Control or Core.')}</p>
          </div>
        </div>
        <div class="integration-card-grid">
          {#each providers as item (item.id)}
            <article class="panel integration-card">
              <div>
                <span class="state-chip good">{text('ГОТОВО','READY')}</span>
                <h3>{item.presentation?.integration?.label || item.name}</h3>
                <p>{item.description}</p>
                <small class="mono">{text('Цель','Target')}: {targetNames(item)}</small>
              </div>
              <a class="button primary" href={managerHref(item)}>{text('Открыть менеджер','Open manager')}</a>
            </article>
          {/each}
        </div>
      </section>
    {/if}

    {#if externals.length}
      <section>
        <div class="catalog-section-head">
          <div>
            <h2>{text('Внешние интеграции','External integrations')}</h2>
            <p>{text('Установленные сторонние приложения, которыми здесь действительно можно управлять или открывать их интерфейс.','Installed third-party applications that can actually be managed or opened here.')}</p>
          </div>
        </div>
        <div class="integration-card-grid">
          {#each externals as item (item.id)}
            <article class="panel integration-card">
              <div>
                <span class="state-chip {item.service_running ? 'good' : 'neutral'}">{item.service_running ? text('РАБОТАЕТ','RUNNING') : text('ОБНАРУЖЕНО','DETECTED')}</span>
                <h3>{item.name}</h3>
                <p>{item.description}</p>
                <small class="mono">{item.category || 'Integration'}{item.version ? ` · v${item.version}` : ''}</small>
              </div>
              {#if item.web}
                <button
                  class="button"
                  type="button"
                  disabled={Boolean(webProbeBusyId)}
                  onclick={() => openEmbeddedWeb(item)}
                >
                  {webProbeBusyId === item.id ? text('Проверка…','Checking…') : text('Открыть интерфейс','Open interface')}
                </button>
              {:else}
                <span class="integration-managed-note">{text('Управляется через расширение RouterForge выше','Managed through the RouterForge extension above')}</span>
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
