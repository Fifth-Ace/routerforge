<script>
  import { moduleLifecycleAction } from '$lib/api.js';
  import { refreshCatalog } from '$lib/stores/catalog.js';

  export let item;
  export let locale = 'ru';
  export let disabled = false;
  export let onnotice = () => {};

  let busyAction = '';

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

  async function run(action) {
    if (!available || disabled || busyAction) return;
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
  {#if item.disabled}
    <button class="button compact primary" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('enable')}>
      {busyAction === 'enable' ? text('Включаем…','Enabling…') : text('Включить','Enable')}
    </button>
  {:else if item.service_running}
    <button class="button compact" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('restart')}>
      {busyAction === 'restart' ? text('Перезапуск…','Restarting…') : text('Перезапустить','Restart')}
    </button>
    <button class="button compact" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('stop')}>
      {busyAction === 'stop' ? text('Остановка…','Stopping…') : text('Остановить','Stop')}
    </button>
    <button class="button compact danger-subtle" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('disable')}>
      {busyAction === 'disable' ? text('Отключаем…','Disabling…') : text('Отключить','Disable')}
    </button>
  {:else}
    <button class="button compact primary" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('start')}>
      {busyAction === 'start' ? text('Запуск…','Starting…') : text('Запустить','Start')}
    </button>
    <button class="button compact danger-subtle" type="button" disabled={disabled || Boolean(busyAction)} onclick={() => run('disable')}>
      {busyAction === 'disable' ? text('Отключаем…','Disabling…') : text('Отключить','Disable')}
    </button>
  {/if}
{/if}
