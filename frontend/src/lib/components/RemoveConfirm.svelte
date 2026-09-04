<script>
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  export let item;
  export let busy = false;
  export let oncancel = () => {};
  export let onconfirm = () => {};

  let typed = '';
  $: locale = $settings.locale || 'ru';
  $: matches = typed === item?.name;
  $: packages = item?.remove?.packages || item?.install?.packages || item?.detection?.packages || [];
</script>

{#if item}
  <div class="modal-overlay danger-overlay">
    <div class="remove-confirm-modal" role="dialog" aria-modal="true" aria-labelledby="remove-title" tabindex="-1">
      <div class="remove-confirm-head">
        <div><span class="state-chip error">{t(locale, 'marketplace.remove.destructive')}</span><h2 id="remove-title">{t(locale, 'marketplace.remove.title', { name: item.name })}</h2></div>
        <button class="icon-button" type="button" aria-label={t(locale, 'common.close')} disabled={busy} onclick={oncancel}>×</button>
      </div>
      <p>{t(locale, 'marketplace.remove.description')}</p>
      {#if packages.length}<div class="remove-package mono">PACKAGE · {packages.join(', ')}</div>{/if}
      <label class="remove-confirm-input">
        <span>{t(locale, 'marketplace.remove.confirmPrompt')}</span>
        <strong>{item.name}</strong>
        <input bind:value={typed} autocomplete="off" spellcheck="false" disabled={busy}/>
      </label>
      <div class="remove-confirm-actions">
        <button class="button" type="button" disabled={busy} onclick={oncancel}>{t(locale, 'common.cancel')}</button>
        <button class="button danger" type="button" disabled={busy || !matches} onclick={() => onconfirm(typed)}>{busy ? t(locale, 'common.removing') : t(locale, 'common.remove')}</button>
      </div>
    </div>
  </div>
{/if}
