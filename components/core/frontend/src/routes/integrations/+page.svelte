<script>
  import { page } from '$app/stores';
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import { catalog } from '$lib/stores/catalog.js';
  import { settings } from '$lib/stores/settings.js';
  import { integrationProviders, manageableExternalIntegrations } from '$lib/integrations.js';

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  $: modules = $catalog.modules || [];
  $: integrations = $catalog.integrations || [];
  $: providers = integrationProviders(modules, integrations);
  $: externals = manageableExternalIntegrations(modules, integrations);
  $: open = $page.url.searchParams.get('open') || '';
  $: activeProvider = providers.find((item) => item.id === open) || null;

  const text = (ru, en) => locale === 'ru' ? ru : en;

  function managerHref(item) {
    return item?.presentation?.integration?.href || `/integrations?open=${encodeURIComponent(item.id)}`;
  }

  function externalHref(item) {
    if (!item?.web) return '';
    return `/apps?tab=integrations&open=${encodeURIComponent(item.id)}`;
  }

  function targetNames(item) {
    const ids = item?.presentation?.integration?.target_ids || [];
    return ids.map((id) => integrations.find((x) => x.id === id)?.name || id).join(', ');
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
              {#if externalHref(item)}
                <a class="button" href={externalHref(item)}>{text('Открыть интерфейс','Open interface')}</a>
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

<style>
  .integrations-hub{display:grid;gap:22px}
  .integration-card-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:12px}
  .integration-card{padding:14px;display:flex;min-height:190px;flex-direction:column;justify-content:space-between;gap:14px}
  .integration-card h3{margin:10px 0 6px}
  .integration-card p{margin:0 0 8px;color:var(--rf-muted,#8d98a4);line-height:1.45}
  .integration-card small{color:var(--rf-muted,#8d98a4)}
  .integration-managed-note{font-size:12px;color:var(--rf-muted,#8d98a4)}
  .integration-workspace{display:grid;gap:12px}
  .integration-workspace-head{display:flex;align-items:center;gap:14px;margin:0}
  .integration-workspace-head>div{display:grid;gap:3px}
  .integration-workspace-head span{color:var(--rf-muted,#8d98a4);font-size:12px}
</style>
