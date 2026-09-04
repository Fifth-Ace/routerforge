<script>
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings, themes } from '$lib/stores/settings.js';
  import { authState, setAuthRequired } from '$lib/stores/auth.js';
  import { getSystem } from '$lib/api.js';
  import { fmtInt, fmtDuration, fmtAgo } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
  import AuthEnableModal from '$lib/components/AuthEnableModal.svelte';

  let system = {};
  let showEnableAuth = false;
  let authBusy = false;
  let authError = '';

  $: locale = $settings.locale || 'ru';
  $: themeCards = [
    ['forge', t(locale, 'settings.themes.forgeTitle'), t(locale, 'settings.themes.forgeDesc')],
    ['midnight', t(locale, 'settings.themes.midnightTitle'), t(locale, 'settings.themes.midnightDesc')],
    ['graphite', t(locale, 'settings.themes.graphiteTitle'), t(locale, 'settings.themes.graphiteDesc')],
    ['custom', t(locale, 'settings.themes.customTitle'), t(locale, 'settings.themes.customDesc')]
  ];
  $: accentOptions = [
    ['#38bdf8', t(locale, 'settings.colors.blue')],
    ['#22d3ee', t(locale, 'settings.colors.cyan')],
    ['#60a5fa', t(locale, 'settings.colors.azure')],
    ['#34d399', t(locale, 'settings.colors.green')],
    ['#f59e0b', t(locale, 'settings.colors.amber')],
    ['#d7824b', t(locale, 'settings.colors.copper')]
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
      authError = error?.payload?.error || error?.message || t(locale, 'settings.enableAuthError');
    } finally {
      authBusy = false;
    }
  }

  async function disableAuth() {
    if (authBusy) return;
    if (!window.confirm(t(locale, 'settings.disableAuthConfirm'))) return;
    authBusy = true;
    authError = '';
    try {
      await setAuthRequired(false);
    } catch (error) {
      authError = error?.payload?.error || error?.message || t(locale, 'settings.disableAuthError');
    } finally {
      authBusy = false;
    }
  }

  function previewPalette(id) {
    if (id === 'custom') {
      return { background: $settings.background, text: $settings.text, surface: $settings.background, border: $settings.text };
    }
    return themes[id];
  }
</script>

<svelte:head><title>RouterForge — {t(locale, 'settings.pageTitle')}</title></svelte:head>

<div class="page">
  <div class="page-head">
    <div><h1>{t(locale, 'settings.pageTitle')}</h1><p>{t(locale, 'settings.subtitle')}</p></div>
    <span class="page-kicker mono">{t(locale, 'settings.kicker')}</span>
  </div>

  <div class="settings-layout">
    <aside class="system-card panel">
      <div class="panel-head"><div><strong>{t(locale, 'settings.system')}</strong><span>RouterForge</span></div><span class="state-chip {$snapshot.capture_error ? 'error' : 'good'}">{$snapshot.capture_error ? 'ERROR' : 'OK'}</span></div>
      <div class="system-row"><span>RouterForge</span><strong>v{$snapshot.version || '—'}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.status')}</span><strong>{primaryDown ? 'ERROR' : primaryDegraded ? 'DEGRADED' : 'OK'}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.primaryDns')}</span><strong>{fmtInt(primaryUpstreams, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.uptime')}</span><strong>{fmtDuration($snapshot.uptime_seconds, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.requests')}</span><strong>{fmtInt($snapshot.total_requests, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.discovery')}</span><strong>{fmtAgo($snapshot.last_discovery, locale)}</strong></div>
      <div class="system-row"><span>RAM</span><strong>{system?.rss_kb ? `${(system.rss_kb / 1024).toFixed(1)} MB` : '—'}</strong></div>
    </aside>

    <div class="settings-stack">
      <section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.general')}</div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.uiLevel')}</strong><span>{t(locale, 'settings.uiLevelHint')}</span></div>
          <div class="setting-control-slot"><select value={$settings.uiLevel} onchange={(e) => update('uiLevel', e.currentTarget.value)}><option value="normal">{t(locale, 'common.normal')}</option><option value="advanced">{t(locale, 'common.advanced')}</option></select></div>
        </div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.uiScale')}</strong><span>{t(locale, 'settings.uiScaleHint')}</span></div>
          <div class="setting-control-slot"><select value={$settings.uiScale} onchange={(e) => update('uiScale', e.currentTarget.value)}><option value="auto">{t(locale, 'settings.autoRecommended')}</option><option value="100">100%</option><option value="115">115%</option><option value="125">125%</option><option value="140">140%</option></select></div>
        </div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.liveUpdate')}</strong><span>{t(locale, 'settings.liveUpdateHint')}</span></div>
          <div class="setting-control-slot"><select value={$settings.refreshMs} onchange={(e) => update('refreshMs', Number(e.currentTarget.value))}><option value="1000">{t(locale, 'common.seconds', { count: 1 })}</option><option value="2000">{t(locale, 'common.seconds', { count: 2 })}</option><option value="5000">{t(locale, 'common.seconds', { count: 5 })}</option><option value="10000">{t(locale, 'common.seconds', { count: 10 })}</option></select></div>
        </div>
      </section>

      <section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.security')}</div>
        <div class="setting-row security-setting-row">
          <div><strong>{t(locale, 'settings.requireAuth')}</strong><span>{t(locale, 'settings.requireAuthHint')}</span></div>
          <div class="security-control setting-control-slot">
            <span class="state-chip {$authState.required ? 'good' : 'neutral'}">{$authState.required ? 'ON' : 'OFF'}</span>
            <button class="security-switch" class:on={$authState.required} type="button" aria-label={$authState.required ? t(locale, 'settings.disableAuthAria') : t(locale, 'settings.enableAuthAria')} aria-pressed={$authState.required} disabled={authBusy} onclick={requestAuthToggle}><span></span></button>
          </div>
        </div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.session')}</strong><span>{t(locale, 'settings.sessionHint')}</span></div><code>{t(locale, 'common.hours', { count: $authState.session_hours || 12 })}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.backend')}</strong><span>{t(locale, 'settings.backendHint')}</span></div><code>entware-root</code></div>
        {#if authError}<div class="settings-security-error">{authError}</div>{/if}
      </section>

      <section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.appearance')}</div>
        <div class="appearance-intro">{t(locale, 'settings.appearanceIntro')}</div>
        <div class="appearance-subtitle">{t(locale, 'settings.theme')}</div>
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

        <div class="appearance-subtitle">{t(locale, 'settings.accent')}</div>
        <div class="accent-picker-row">
          <div class="accent-presets" aria-label={t(locale, 'settings.presetAccents')}>
            {#each accentOptions as [color, label]}
              <button type="button" class="accent-swatch" class:active={$settings.accent === color} style={`--swatch:${color}`} title={label} aria-label={label} onclick={() => update('accent', color)}><span></span></button>
            {/each}
          </div>
          <label class="custom-accent"><span>{t(locale, 'settings.customColor')}</span><input type="color" value={$settings.accent} oninput={(e) => update('accent', e.currentTarget.value)}/><code>{$settings.accent}</code></label>
        </div>

        <div class="appearance-options-grid">
          <label><span>{t(locale, 'settings.density')}</span><select value={$settings.density} onchange={(e) => update('density', e.currentTarget.value)}><option value="compact">{t(locale, 'settings.densityCompact')}</option><option value="normal">{t(locale, 'settings.densityNormal')}</option><option value="comfortable">{t(locale, 'settings.densityComfortable')}</option></select></label>
          <label><span>{t(locale, 'settings.radius')}</span><select value={$settings.radius} onchange={(e) => update('radius', e.currentTarget.value)}><option value="sharp">{t(locale, 'settings.radiusSharp')}</option><option value="default">{t(locale, 'settings.radiusDefault')}</option><option value="soft">{t(locale, 'settings.radiusSoft')}</option></select></label>
          <label><span>{t(locale, 'settings.brandCopper')}</span><select value={$settings.brandMode} onchange={(e) => update('brandMode', e.currentTarget.value)}><option value="brand-only">{t(locale, 'settings.brandOnly')}</option><option value="extended">{t(locale, 'settings.brandExtended')}</option></select></label>
        </div>

        {#if $settings.theme === 'custom'}
          <div class="color-grid custom-theme-colors">
            <label><span>{t(locale, 'settings.background')}</span><input type="color" value={$settings.background} oninput={(e) => update('background', e.currentTarget.value)}/><code>{$settings.background}</code></label>
            <label><span>{t(locale, 'settings.text')}</span><input type="color" value={$settings.text} oninput={(e) => update('text', e.currentTarget.value)}/><code>{$settings.text}</code></label>
          </div>
        {/if}
      </section>

      <section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.monitoring')}</div>
        <div class="setting-row static"><div><strong>Discovery</strong><span>{t(locale, 'settings.discoveryHint')}</span></div><code>{t(locale, 'common.seconds', { count: 60 })}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.healthCheck')}</strong><span>{t(locale, 'settings.healthCheckHint')}</span></div><code>{t(locale, 'common.seconds', { count: 30 })}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.history')}</strong><span>{t(locale, 'settings.historyHint')}</span></div><code>{t(locale, 'common.hours', { count: 24 })}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.webPort')}</strong><span>{t(locale, 'settings.webPortHint')}</span></div><code>2233</code></div>
      </section>
    </div>
  </div>
</div>

{#if showEnableAuth}
  <AuthEnableModal busy={authBusy} error={authError} oncancel={() => { if (!authBusy) { showEnableAuth = false; authError = ''; } }} onconfirm={enableAuth}/>
{/if}
