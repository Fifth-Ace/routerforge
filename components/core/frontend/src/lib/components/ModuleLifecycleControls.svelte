<script>
  import { moduleLifecycleAction } from '$lib/api.js';
  import { t } from '$lib/i18n/index.js';
  import { refreshCatalog } from '$lib/stores/catalog.js';

  export let item;
  export let locale = 'ru';
  export let disabled = false;
  export let onnotice = () => {};

  let busyAction = '';
  let menuOpen = false;

  $: available = Boolean(
    item?.kind === 'module'
    && item?.installed
    && item?.lifecycle_managed
    && item?.id !== 'routerforge-core'
  );


  function question(action) {
    const name = item?.name || item?.id || 'module';
    if (action === 'restart') return t(locale, 'moduleLifecycle.confirmRestart', {name});
    if (action === 'stop') return t(locale, 'moduleLifecycle.confirmStop', {name});
    if (action === 'disable') return t(locale, 'moduleLifecycle.confirmDisable', {name});
    if (action === 'enable') return t(locale, 'moduleLifecycle.confirmEnable', {name});
    return t(locale, 'moduleLifecycle.confirmStart', {name});
  }

  function successText(action) {
    const labels = { start:'successStart', restart:'successRestart', stop:'successStop', disable:'successDisable', enable:'successEnable' };
    return `${item?.name || item?.id}: ${labels[action] ? t(locale, 'moduleLifecycle.' + labels[action]) : action}`;
  }

  function busyText(action) {
    if (!action) return '';
    const labels = { start:'busyStart', restart:'busyRestart', stop:'busyStop', disable:'busyDisable', enable:'busyEnable' };
    return labels[action] ? t(locale, 'moduleLifecycle.' + labels[action]) : t(locale, 'moduleLifecycle.working');
  }

  async function run(action) {
    if (!available || disabled || busyAction) return;
    menuOpen = false;
    if (!window.confirm(question(action))) return;

    busyAction = action;
    try {
      await moduleLifecycleAction(item.id, action);
      await refreshCatalog();
      onnotice({ cls:'good', text:successText(action) });
    } catch (error) {
      onnotice({
        cls:'error',
        text:error?.payload?.detail || error?.payload?.error || error?.message || t(locale, 'moduleLifecycle.error')
      });
    } finally {
      busyAction = '';
    }
  }
</script>

{#if available}
  <details class="module-lifecycle-menu" bind:open={menuOpen}>
    <summary
      class="button module-lifecycle-trigger"
      class:primary={item.disabled || !item.service_running}
      aria-label={t(locale, 'moduleLifecycle.controls')}
    >
      <span>{busyAction ? busyText(busyAction) : t(locale, 'moduleLifecycle.manage')}</span>
      <span class="module-lifecycle-chevron" aria-hidden="true">▾</span>
    </summary>

    <div class="module-lifecycle-dropdown">
      {#if item.disabled}
        <button
          class="module-lifecycle-item primary"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('enable')}
        >
          <span>{t(locale, 'moduleLifecycle.enable')}</span>
          <small>{t(locale, 'moduleLifecycle.enableHint')}</small>
        </button>
      {:else if item.service_running}
        <button
          class="module-lifecycle-item"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('restart')}
        >
          <span>{t(locale, 'moduleLifecycle.restart')}</span>
          <small>{t(locale, 'moduleLifecycle.restartHint')}</small>
        </button>

        <button
          class="module-lifecycle-item"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('stop')}
        >
          <span>{t(locale, 'moduleLifecycle.stop')}</span>
          <small>{t(locale, 'moduleLifecycle.stopHint')}</small>
        </button>

        <button
          class="module-lifecycle-item danger"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('disable')}
        >
          <span>{t(locale, 'moduleLifecycle.disable')}</span>
          <small>{t(locale, 'moduleLifecycle.disableHint')}</small>
        </button>
      {:else}
        <button
          class="module-lifecycle-item primary"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('start')}
        >
          <span>{t(locale, 'moduleLifecycle.start')}</span>
          <small>{t(locale, 'moduleLifecycle.startHint')}</small>
        </button>

        <button
          class="module-lifecycle-item danger"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('disable')}
        >
          <span>{t(locale, 'moduleLifecycle.disable')}</span>
          <small>{t(locale, 'moduleLifecycle.disableHint')}</small>
        </button>
      {/if}
    </div>
  </details>
{/if}

<style>
  .module-lifecycle-menu {
    position:relative;
    display:inline-block;
  }

  .module-lifecycle-menu > summary {
    list-style:none;
  }

  .module-lifecycle-menu > summary::-webkit-details-marker {
    display:none;
  }

  .module-lifecycle-trigger {
    display:inline-flex;
    align-items:center;
    gap:.42rem;
    white-space:nowrap;
  }

  .module-lifecycle-chevron {
    color:var(--rf-muted,var(--muted));
    font-size:.72rem;
    line-height:1;
    transition:transform .14s ease;
  }

  .module-lifecycle-menu[open] .module-lifecycle-chevron {
    transform:rotate(180deg);
  }

  .module-lifecycle-dropdown {
    position:absolute;
    z-index:80;
    right:0;
    bottom:calc(100% + .4rem);
    width:min(280px,calc(100vw - 2rem));
    display:grid;
    gap:.22rem;
    padding:.38rem;
    border:1px solid var(--rf-border-strong,var(--rf-border,var(--border)));
    border-radius:.55rem;
    background:var(--rf-surface,var(--panel));
    box-shadow:0 14px 36px rgba(0,0,0,.34);
  }

  .module-lifecycle-item {
    appearance:none;
    width:100%;
    display:grid;
    gap:.16rem;
    padding:.56rem .62rem;
    border:1px solid transparent;
    border-radius:.42rem;
    background:transparent;
    color:var(--rf-text,var(--text));
    text-align:left;
    font:inherit;
    cursor:pointer;
  }

  .module-lifecycle-item:hover:not(:disabled) {
    border-color:var(--rf-border,var(--border));
    background:var(--rf-hover,var(--hover));
  }

  .module-lifecycle-item:disabled {
    opacity:.55;
    cursor:not-allowed;
  }

  .module-lifecycle-item span {
    font-size:.78rem;
    font-weight:700;
  }

  .module-lifecycle-item small {
    color:var(--rf-muted,var(--muted));
    font-size:.68rem;
    line-height:1.35;
  }

  .module-lifecycle-item.primary span {
    color:var(--rf-accent,var(--accent));
  }

  .module-lifecycle-item.danger span {
    color:var(--bad,#f85149);
  }

  @media (max-width:760px) {
    .module-lifecycle-dropdown {
      right:auto;
      left:0;
    }
  }
</style>
