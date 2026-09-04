<script>
  import { authState, loginAuth, refreshAuth } from '$lib/stores/auth.js';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  let username = 'root';
  let password = '';
  let busy = false;
  let error = '';

  $: locale = $settings.locale || 'ru';

  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      await loginAuth(username.trim(), password);
      password = '';
    } catch (e) {
      error = e?.payload?.error || e?.message || t(locale, 'auth.loginError');
    } finally {
      busy = false;
    }
  }

  async function retry() {
    error = '';
    try { await refreshAuth(); } catch {}
  }
</script>

<div class="auth-gate">
  <div class="auth-card routerforge-auth-card">
    <div class="auth-brand routerforge-auth-brand">
      <img class="routerforge-auth-mark" src="/routerforge-mark.png" alt=""/>
      <div><strong>RouterForge</strong><span>Router Console · Beta</span></div>
    </div>

    {#if !$authState.ready}
      <div class="auth-loading mono">{t(locale, 'auth.securityStatus')}</div>
    {:else if $authState.error}
      <h1>{t(locale, 'auth.coreUnavailable')}</h1>
      <p>{t(locale, 'auth.cannotCheck')}</p>
      <div class="auth-error">{$authState.error}</div>
      <button class="button primary" type="button" onclick={retry}>{t(locale, 'common.retry')}</button>
    {:else}
      <div class="auth-kicker mono">ENTWARE ROOT AUTH</div>
      <h1>{t(locale, 'auth.signIn')}</h1>
      <p>{t(locale, 'auth.usesRoot')}</p>

      <form class="auth-form" onsubmit={submit}>
        <label><span>{t(locale, 'auth.user')}</span><input bind:value={username} autocomplete="username" spellcheck="false"/></label>
        <label><span>{t(locale, 'auth.password')}</span><input bind:value={password} type="password" autocomplete="current-password" autofocus/></label>
        {#if error}<div class="auth-error">{error}</div>{/if}
        <button class="button primary auth-submit" type="submit" disabled={busy || !username.trim()}>{busy ? t(locale, 'auth.checking') : t(locale, 'auth.login')}</button>
      </form>
      <div class="auth-foot mono">SESSION {$authState.session_hours || 12}H · HTTPONLY · SAMESITE STRICT</div>
    {/if}
  </div>
</div>
