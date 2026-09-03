<script>
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings, themes } from '$lib/stores/settings.js';
  import { authState, setAuthRequired } from '$lib/stores/auth.js';
  import { getSystem } from '$lib/api.js';
  import { fmtInt, fmtDuration, fmtAgo } from '$lib/utils.js';
  import AuthEnableModal from '$lib/components/AuthEnableModal.svelte';

  let system = {};
  let showEnableAuth = false;
  let authBusy = false;
  let authError = '';

  const themeCards = [
    ['forge', 'RouterForge Dark', 'Нейтральный графитовый фундамент. Рекомендуется.'],
    ['midnight', 'Midnight', 'Холодная тёмно-синяя палитра для сетевой консоли.'],
    ['graphite', 'Graphite', 'Более нейтральный серо-графитовый интерфейс.'],
    ['custom', 'Custom', 'Свой фон и текст; остальные поверхности вычисляются автоматически.']
  ];

  const accentOptions = [
    ['#38bdf8', 'Blue'],
    ['#22d3ee', 'Cyan'],
    ['#60a5fa', 'Azure'],
    ['#34d399', 'Green'],
    ['#f59e0b', 'Amber'],
    ['#d7824b', 'Copper']
  ];

  const update = (key, value) => settings.update((current) => ({ ...current, [key]: value }));
  $: primaryDown = Number($snapshot.primary_active_down ?? $snapshot.active_down ?? 0);
  $: primaryDegraded = Number($snapshot.primary_active_degraded ?? $snapshot.active_degraded ?? 0);
  $: primaryUpstreams = Number($snapshot.primary_upstream_count ?? $snapshot.upstream_count ?? 0);

  onMount(async () => {
    try { system = await getSystem(); } catch {}
  });

  function requestAuthToggle() {
    authError = '';
    if ($authState.required) disableAuth();
    else showEnableAuth = true;
  }

  async function enableAuth(password) {
    if (authBusy) return;
    authBusy = true;
    authError = '';
    try {
      await setAuthRequired(true, 'root', password);
      showEnableAuth = false;
    } catch (error) {
      authError = error?.payload?.error || error?.message || 'Не удалось включить авторизацию';
    } finally {
      authBusy = false;
    }
  }

  async function disableAuth() {
    if (authBusy) return;
    if (!window.confirm('Отключить обязательную авторизацию RouterForge? Панель и API снова будут доступны без входа из локальной сети.')) return;
    authBusy = true;
    authError = '';
    try {
      await setAuthRequired(false);
    } catch (error) {
      authError = error?.payload?.error || error?.message || 'Не удалось отключить авторизацию';
    } finally {
      authBusy = false;
    }
  }

  function previewPalette(id) {
    if (id === 'custom') {
      return {
        background: $settings.background,
        text: $settings.text,
        surface: $settings.background,
        border: $settings.text
      };
    }
    return themes[id];
  }
</script>

<svelte:head><title>RouterForge — Настройки</title></svelte:head>

<div class="page">
  <div class="page-head">
    <div><h1>Настройки</h1><p>Интерфейс, безопасность, live-обновления и единая дизайн-система RouterForge.</p></div>
    <span class="page-kicker mono">LOCAL UI SETTINGS</span>
  </div>

  <div class="settings-layout">
    <aside class="system-card panel">
      <div class="panel-head"><div><strong>Система</strong><span>RouterForge</span></div><span class="state-chip {$snapshot.capture_error ? 'error' : 'good'}">{$snapshot.capture_error ? 'ERROR' : 'OK'}</span></div>
      <div class="system-row"><span>RouterForge</span><strong>v{$snapshot.version || '—'}</strong></div>
      <div class="system-row"><span>Статус</span><strong>{primaryDown ? 'ERROR' : primaryDegraded ? 'DEGRADED' : 'OK'}</strong></div>
      <div class="system-row"><span>Основные DNS</span><strong>{fmtInt(primaryUpstreams)}</strong></div>
      <div class="system-row"><span>Uptime</span><strong>{fmtDuration($snapshot.uptime_seconds)}</strong></div>
      <div class="system-row"><span>Запросы</span><strong>{fmtInt($snapshot.total_requests)}</strong></div>
      <div class="system-row"><span>Discovery</span><strong>{fmtAgo($snapshot.last_discovery)}</strong></div>
      <div class="system-row"><span>RAM</span><strong>{system?.rss_kb ? `${(system.rss_kb / 1024).toFixed(1)} MB` : '—'}</strong></div>
    </aside>

    <div class="settings-stack">
      <section class="settings-section panel">
        <div class="settings-title">Общие</div>
        <div class="setting-row">
          <div><strong>Уровень интерфейса</strong><span>Обычный показывает основные рабочие показатели. Расширенный добавляет диагностические поля.</span></div>
          <div class="setting-control-slot"><select value={$settings.uiLevel} onchange={(e) => update('uiLevel', e.currentTarget.value)}><option value="normal">Обычный</option><option value="advanced">Расширенный</option></select></div>
        </div>
        <div class="setting-row">
          <div><strong>Масштаб интерфейса</strong><span>Auto автоматически делает UI крупнее на 2K/4K без browser zoom.</span></div>
          <div class="setting-control-slot"><select value={$settings.uiScale} onchange={(e) => update('uiScale', e.currentTarget.value)}><option value="auto">Auto · рекомендуется</option><option value="100">100%</option><option value="115">115%</option><option value="125">125%</option><option value="140">140%</option></select></div>
        </div>
        <div class="setting-row">
          <div><strong>Live update</strong><span>Частота SSE snapshot. При проблемах SSE автоматически включается polling.</span></div>
          <div class="setting-control-slot"><select value={$settings.refreshMs} onchange={(e) => update('refreshMs', Number(e.currentTarget.value))}><option value="1000">1 сек</option><option value="2000">2 сек</option><option value="5000">5 сек</option><option value="10000">10 сек</option></select></div>
        </div>
      </section>

      <section class="settings-section panel">
        <div class="settings-title">Безопасность</div>
        <div class="setting-row security-setting-row">
          <div>
            <strong>Требовать авторизацию</strong>
            <span>Вход под <code>root</code> с паролем Entware. Проверяется <code>/opt/etc/shadow</code> с fallback на <code>/opt/etc/passwd</code>; отдельный пароль RouterForge не хранится.</span>
          </div>
          <div class="security-control setting-control-slot">
            <span class="state-chip {$authState.required ? 'good' : 'neutral'}">{$authState.required ? 'REQUIRED' : 'OFF'}</span>
            <button class="security-switch" class:on={$authState.required} type="button" aria-label={$authState.required ? 'Отключить обязательную авторизацию' : 'Включить обязательную авторизацию'} aria-pressed={$authState.required} disabled={authBusy} onclick={requestAuthToggle}><span></span></button>
          </div>
        </div>
        <div class="setting-row static"><div><strong>Сессия</strong><span>HttpOnly cookie, SameSite=Strict. После перезапуска Core требуется войти снова.</span></div><code>{$authState.session_hours || 12} часов</code></div>
        <div class="setting-row static"><div><strong>Backend</strong><span>Учётные данные системного root Entware, без копирования хеша в конфиг RouterForge.</span></div><code>entware-root</code></div>
        {#if authError}<div class="settings-security-error">{authError}</div>{/if}
      </section>

      <section class="settings-section panel">
        <div class="settings-title">Внешний вид</div>
        <div class="appearance-intro">Рабочий акцент интерфейса отделён от фирменной меди RouterForge. По умолчанию медь остаётся только в логотипе и брендовых деталях.</div>

        <div class="appearance-subtitle">Тема</div>
        <div class="theme-grid">
          {#each themeCards as [id, title, description]}
            {@const palette = previewPalette(id)}
            <button class="theme-card" class:active={$settings.theme === id} onclick={() => update('theme', id)}>
              <h3>{title}</h3><p>{description}</p>
              <div class="theme-preview" style={`background:${palette.background};color:${palette.text}`}>
                <i style={`width:25%;background:${$settings.accent}`}></i>
                <i style={`width:80%;background:${palette.text};opacity:.14`}></i>
                <i style={`background:${palette.surface || palette.background};border:1px solid ${palette.border || palette.text}`}></i>
              </div>
            </button>
          {/each}
        </div>

        <div class="appearance-subtitle">Акцент интерфейса</div>
        <div class="accent-picker-row">
          <div class="accent-presets" aria-label="Предустановленные акценты">
            {#each accentOptions as [color, label]}
              <button type="button" class="accent-swatch" class:active={$settings.accent === color} style={`--swatch:${color}`} title={label} aria-label={label} onclick={() => update('accent', color)}><span></span></button>
            {/each}
          </div>
          <label class="custom-accent"><span>Свой цвет</span><input type="color" value={$settings.accent} oninput={(e) => update('accent', e.currentTarget.value)}/><code>{$settings.accent}</code></label>
        </div>

        <div class="appearance-options-grid">
          <label><span>Плотность</span><select value={$settings.density} onchange={(e) => update('density', e.currentTarget.value)}><option value="compact">Компактная</option><option value="normal">Обычная</option><option value="comfortable">Свободная</option></select></label>
          <label><span>Скругления</span><select value={$settings.radius} onchange={(e) => update('radius', e.currentTarget.value)}><option value="sharp">Строгие</option><option value="default">Стандартные</option><option value="soft">Мягкие</option></select></label>
          <label><span>Фирменная медь</span><select value={$settings.brandMode} onchange={(e) => update('brandMode', e.currentTarget.value)}><option value="brand-only">Только бренд</option><option value="extended">Расширенно</option></select></label>
        </div>

        {#if $settings.theme === 'custom'}
          <div class="color-grid custom-theme-colors">
            <label><span>Фон</span><input type="color" value={$settings.background} oninput={(e) => update('background', e.currentTarget.value)}/><code>{$settings.background}</code></label>
            <label><span>Текст</span><input type="color" value={$settings.text} oninput={(e) => update('text', e.currentTarget.value)}/><code>{$settings.text}</code></label>
          </div>
        {/if}
      </section>

      <section class="settings-section panel">
        <div class="settings-title">Мониторинг</div>
        <div class="setting-row static"><div><strong>Discovery</strong><span>Основной DNS, System DoT/DoH и служебные policy-context Keenetic.</span></div><code>60 сек</code></div>
        <div class="setting-row static"><div><strong>Health-check</strong><span>Основные resolver'ы и policy-context только при наличии маршрута наружу.</span></div><code>30 сек</code></div>
        <div class="setting-row static"><div><strong>История</strong><span>Minute buckets хранятся только в RAM.</span></div><code>24 часа</code></div>
        <div class="setting-row static"><div><strong>Веб-порт</strong><span>Панель RouterForge.</span></div><code>2233</code></div>
      </section>
    </div>
  </div>
</div>

{#if showEnableAuth}
  <AuthEnableModal busy={authBusy} error={authError} oncancel={() => { if (!authBusy) { showEnableAuth = false; authError = ''; } }} onconfirm={enableAuth}/>
{/if}
