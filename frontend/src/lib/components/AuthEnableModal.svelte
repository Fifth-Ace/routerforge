<script>
  export let busy = false;
  export let error = '';
  export let oncancel = () => {};
  export let onconfirm = () => {};

  let password = '';

  function submit(event) {
    event.preventDefault();
    if (!busy && password) onconfirm(password);
  }
</script>

<div class="modal-overlay">
  <div class="auth-config-modal" role="dialog" aria-modal="true" aria-labelledby="auth-enable-title" tabindex="-1">
    <div class="remove-confirm-head">
      <div><span class="state-chip warn">SECURITY</span><h2 id="auth-enable-title">Включить обязательный вход?</h2></div>
      <button class="icon-button" type="button" aria-label="Закрыть" disabled={busy} onclick={oncancel}>×</button>
    </div>
    <p>Сначала проверим root-пароль Entware. После включения UI и все API, кроме login/status/health, будут закрыты сессией.</p>
    <form class="auth-form compact-form" onsubmit={submit}>
      <label><span>Пользователь</span><input value="root" readonly autocomplete="username"/></label>
      <label><span>Root-пароль Entware</span><input bind:value={password} type="password" autocomplete="current-password" autofocus/></label>
      {#if error}<div class="auth-error">{error}</div>{/if}
      <div class="remove-confirm-actions">
        <button class="button" type="button" disabled={busy} onclick={oncancel}>Отмена</button>
        <button class="button primary" type="submit" disabled={busy || !password}>{busy ? 'Проверка…' : 'Проверить и включить'}</button>
      </div>
    </form>
  </div>
</div>
