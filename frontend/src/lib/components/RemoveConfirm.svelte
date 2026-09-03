<script>
  export let item;
  export let busy = false;
  export let oncancel = () => {};
  export let onconfirm = () => {};

  let typed = '';
  $: matches = typed === item?.name;
  $: packages = item?.install?.packages || item?.detection?.packages || [];
</script>

{#if item}
  <div class="modal-overlay danger-overlay">
    <div class="remove-confirm-modal" role="dialog" aria-modal="true" aria-labelledby="remove-title" tabindex="-1">
      <div class="remove-confirm-head">
        <div><span class="state-chip error">DESTRUCTIVE</span><h2 id="remove-title">Удалить {item.name}?</h2></div>
        <button class="icon-button" type="button" aria-label="Закрыть" disabled={busy} onclick={oncancel}>×</button>
      </div>
      <p>Будет выполнен <code>opkg remove</code> только для allowlisted пакета DNS Monitor. Конфигурация Core и другие модули не затрагиваются.</p>
      {#if packages.length}<div class="remove-package mono">PACKAGE · {packages.join(', ')}</div>{/if}
      <label class="remove-confirm-input">
        <span>Для подтверждения введи точное имя модуля:</span>
        <strong>{item.name}</strong>
        <input bind:value={typed} autocomplete="off" spellcheck="false" disabled={busy}/>
      </label>
      <div class="remove-confirm-actions">
        <button class="button" type="button" disabled={busy} onclick={oncancel}>Отмена</button>
        <button class="button danger" type="button" disabled={busy || !matches} onclick={() => onconfirm(typed)}>{busy ? 'Удаление…' : 'Удалить модуль'}</button>
      </div>
    </div>
  </div>
{/if}
