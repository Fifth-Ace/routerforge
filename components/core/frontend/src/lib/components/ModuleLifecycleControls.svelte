<script>
  import { moduleLifecycleAction } from '$lib/api.js';
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

  const text = (ru, en) => locale === 'ru' ? ru : en;

  function question(action) {
    const name = item?.name || item?.id || 'module';
    if (action === 'restart') return text(`Перезапустить ${name}?`, `Restart ${name}?`);
    if (action === 'stop') return text(
      `Остановить ${name}? После перезагрузки роутера модуль снова запустится.`,
      `Stop ${name}? The module will start again after the router reboots.`
    );
    if (action === 'disable') return text(
      `Отключить ${name}? Модуль будет остановлен и не запустится после перезагрузки, пока вы не включите его снова.`,
      `Disable ${name}? It will stop and stay disabled after reboot until you enable it again.`
    );
    if (action === 'enable') return text(`Включить и запустить ${name}?`, `Enable and start ${name}?`);
    return text(`Запустить ${name}?`, `Start ${name}?`);
  }

  function successText(action) {
    const labels = locale === 'ru'
      ? { start:'запущен', restart:'перезапущен', stop:'остановлен', disable:'отключён', enable:'включён' }
      : { start:'started', restart:'restarted', stop:'stopped', disable:'disabled', enable:'enabled' };
    return `${item?.name || item?.id}: ${labels[action] || action}`;
  }

  function busyText(action) {
    if (!action) return '';
    const labels = locale === 'ru'
      ? { start:'Запуск…', restart:'Перезапуск…', stop:'Остановка…', disable:'Отключаем…', enable:'Включаем…' }
      : { start:'Starting…', restart:'Restarting…', stop:'Stopping…', disable:'Disabling…', enable:'Enabling…' };
    return labels[action] || text('Выполняется…','Working…');
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
        text:error?.payload?.detail || error?.payload?.error || error?.message || text('Ошибка управления модулем','Module lifecycle action failed')
      });
    } finally {
      busyAction = '';
    }
  }
</script>

{#if available}
  <details class="module-lifecycle-menu" bind:open={menuOpen}>
    <summary
      class="button compact module-lifecycle-trigger"
      class:primary={item.disabled || !item.service_running}
      aria-label={text('Управление модулем','Module controls')}
    >
      <span>{busyAction ? busyText(busyAction) : text('Управление','Manage')}</span>
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
          <span>{text('Включить','Enable')}</span>
          <small>{text('Запустить и включить автозапуск','Start and restore autostart')}</small>
        </button>
      {:else if item.service_running}
        <button
          class="module-lifecycle-item"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('restart')}
        >
          <span>{text('Перезапустить','Restart')}</span>
          <small>{text('Перезапустить модуль сейчас','Restart the module now')}</small>
        </button>

        <button
          class="module-lifecycle-item"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('stop')}
        >
          <span>{text('Остановить','Stop')}</span>
          <small>{text('Запустится снова после перезагрузки','Starts again after reboot')}</small>
        </button>

        <button
          class="module-lifecycle-item danger"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('disable')}
        >
          <span>{text('Отключить','Disable')}</span>
          <small>{text('Не запускать после перезагрузки','Keep disabled after reboot')}</small>
        </button>
      {:else}
        <button
          class="module-lifecycle-item primary"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('start')}
        >
          <span>{text('Запустить','Start')}</span>
          <small>{text('Запустить модуль сейчас','Start the module now')}</small>
        </button>

        <button
          class="module-lifecycle-item danger"
          type="button"
          disabled={disabled || Boolean(busyAction)}
          onclick={() => run('disable')}
        >
          <span>{text('Отключить','Disable')}</span>
          <small>{text('Не запускать после перезагрузки','Keep disabled after reboot')}</small>
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
