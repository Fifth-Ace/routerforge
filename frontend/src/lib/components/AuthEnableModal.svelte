<script>
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  export let busy = false;
  export let error = '';
  export let oncancel = () => {};
  export let onconfirm = () => {};

  let password = '';
  $: locale = $settings.locale || 'ru';

  function submit(event) {
    event.preventDefault();
    if (!busy && password) onconfirm(password);
  }
</script>

<div class="modal-overlay">
  <div class="auth-config-modal" role="dialog" aria-modal="true" aria-labelledby="auth-enable-title" tabindex="-1">
    <div class="remove-confirm-head">
      <div><span class="state-chip warn">SECURITY</span><h2 id="auth-enable-title">{t(locale, 'auth.enableTitle')}</h2></div>
      <button class="icon-button" type="button" aria-label={t(locale, 'auth.close')} disabled={busy} onclick={oncancel}>×</button>
    </div>
    <p>{t(locale, 'auth.enableDescription')}</p>
    <form class="auth-form compact-form" onsubmit={submit}>
      <label><span>{t(locale, 'auth.user')}</span><input value="root" readonly autocomplete="username"/></label>
      <label><span>{t(locale, 'auth.rootPassword')}</span><input bind:value={password} type="password" autocomplete="current-password" autofocus/></label>
      {#if error}<div class="auth-error">{error}</div>{/if}
      <div class="remove-confirm-actions">
        <button class="button" type="button" disabled={busy} onclick={oncancel}>{t(locale, 'common.cancel')}</button>
        <button class="button primary" type="submit" disabled={busy || !password}>{busy ? t(locale, 'auth.checking') : t(locale, 'auth.verifyEnable')}</button>
      </div>
    </form>
  </div>
</div>
