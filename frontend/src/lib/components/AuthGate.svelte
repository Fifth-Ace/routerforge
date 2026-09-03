<script>
  import { authState, loginAuth, refreshAuth } from '$lib/stores/auth.js';

  let username = 'root';
  let password = '';
  let busy = false;
  let error = '';

  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      await loginAuth(username.trim(), password);
      password = '';
    } catch (e) {
      error = e?.payload?.error || e?.message || 'Ошибка входа';
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
      <div class="auth-loading mono">SECURITY STATUS…</div>
    {:else if $authState.error}
      <h1>Core недоступен</h1>
      <p>Не удалось проверить режим авторизации.</p>
      <div class="auth-error">{$authState.error}</div>
      <button class="button primary" type="button" onclick={retry}>Повторить</button>
    {:else}
      <div class="auth-kicker mono">ENTWARE ROOT AUTH</div>
      <h1>Вход в RouterForge</h1>
      <p>Используется root-пароль Entware.</p>

      <form class="auth-form" onsubmit={submit}>
        <label><span>Пользователь</span><input bind:value={username} autocomplete="username" spellcheck="false"/></label>
        <label><span>Пароль</span><input bind:value={password} type="password" autocomplete="current-password" autofocus/></label>
        {#if error}<div class="auth-error">{error}</div>{/if}
        <button class="button primary auth-submit" type="submit" disabled={busy || !username.trim()}>{busy ? 'Проверка…' : 'Войти'}</button>
      </form>
      <div class="auth-foot mono">SESSION {$authState.session_hours || 12}H · HTTPONLY · SAMESITE STRICT</div>
    {/if}
  </div>
</div>
