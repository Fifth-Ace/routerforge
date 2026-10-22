<script>
  import { onMount } from 'svelte';
  import '../../routerforge-settings-p1.css';
  let activeSettingsTab = 'system';
  function onSettingsTabKeydown(event) {
    const tabs = Array.from(event.currentTarget.parentElement.querySelectorAll('[role="tab"]'));
    const index = tabs.indexOf(event.currentTarget);
    if (index < 0) return;
    let next = index;
    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = tabs.length - 1;
    else return;
    event.preventDefault();
    activeSettingsTab = ['system', 'interface', 'logs', 'security'][next];
    tabs[next].focus();
  }
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings, themes } from '$lib/stores/settings.js';
  import { catalog, refreshCatalog } from '$lib/stores/catalog.js';
  import { authState, setAuthRequired } from '$lib/stores/auth.js';
  import { getSystem } from '$lib/api.js';
  import { fmtInt, fmtDuration, fmtAgo } from '$lib/utils.js';
  import { t } from '$lib/i18n/index.js';
  import AuthEnableModal from '$lib/components/AuthEnableModal.svelte';
  import RfToggle from '$lib/components/RfToggle.svelte';

  let system = {};
  let showEnableAuth = false;
  let authBusy = false;
  let authError = '';
  let loggingMode = 'entware';
  let loggingCustomPath = '';
  let loggingActivePath = '';
  let loggingMaxMB = 1;
  let loggingBackups = 2;
  let loggingRAMAvailable = false;
  let loggingBusy = false;
  let loggingError = '';
  let loggingSaved = '';
  let telemetryEnabled = true;
  let telemetrySaving = false;
  let telemetryStatus = {};
  let telemetryError = '';
  let telemetryNotice = '';
  async function loadTelemetry() {
    try {
      const response = await fetch('/api/settings/telemetry', { credentials:'same-origin', cache:'no-store' });
      if (!response.ok) throw new Error('HTTP ' + response.status);
      telemetryStatus = await response.json();
      telemetryEnabled = !!telemetryStatus.diagnostics_enabled;
    } catch (err) {
      telemetryError = String(err?.message || err);
    }
  }
  async function changeTelemetry(enabled) {
    if (telemetrySaving) return;
    telemetrySaving = true;
    telemetryError = '';
    telemetryNotice = '';
    try {
      const response = await fetch('/api/settings/telemetry', {
        method:'POST', credentials:'same-origin',
        headers:{ 'Content-Type':'application/json', Accept:'application/json' },
        body:JSON.stringify({ diagnostics_enabled: !!enabled })
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || ('HTTP ' + response.status));
      telemetryStatus = data;
      telemetryEnabled = !!data.diagnostics_enabled;
      telemetryNotice = telemetryEnabled ? t(locale, 'settings.l1a.telemetryOn') : t(locale, 'settings.l1a.telemetryOff');
    } catch (err) {
      telemetryError = String(err?.message || err);
      await loadTelemetry();
    } finally {
      telemetrySaving = false;
    }
  }
  async function loadLogging() {
    try {
      const response = await fetch('/api/settings/logging', { credentials: 'same-origin', cache: 'no-store' });
      if (!response.ok) throw new Error('HTTP ' + response.status);
      const data = await response.json();
      loggingMode = data.settings?.mode || 'entware';
      loggingCustomPath = data.settings?.custom_path || '';
      loggingActivePath = data.active_path || '';
      loggingMaxMB = data.settings?.max_mb || 1;
      loggingBackups = data.settings?.backups || 2;
      loggingRAMAvailable = !!data.ram_available;
    } catch (err) {
      loggingError = String(err.message || err);
    }
  }
  async function saveLogging() {
    if (loggingBusy) return;
    loggingBusy = true;
    loggingError = '';
    loggingSaved = '';
    try {
      const response = await fetch('/api/settings/logging', {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ mode: loggingMode, custom_path: loggingCustomPath.trim(), max_mb: Number(loggingMaxMB), backups: Number(loggingBackups) })
      });
      const reply = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(reply.error || ('HTTP ' + response.status));
      loggingActivePath = reply.active_path || '';
      loggingSaved = t(locale, 'settings.l1a.loggingSaved');
    } catch (err) {
      loggingError = String(err.message || err);
    } finally { loggingBusy = false; }
  }

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
  $: dnsInstalled = ($catalog?.modules || []).some((item) => item?.id === 'dns' && item?.installed);

  const dnsPresets = {
    strict: { dnsHealthWindowMin:5, dnsHealthFailCount:5, dnsHealthFailRate:1, dnsHealthLatencyP95Ms:1000, dnsHealthLatencySamples:20, dnsHealthDownRequests:3, dnsHealthDownWindowSec:60, dnsHealthHistoryMin:60 },
    normal: { dnsHealthWindowMin:5, dnsHealthFailCount:8, dnsHealthFailRate:1.5, dnsHealthLatencyP95Ms:1200, dnsHealthLatencySamples:20, dnsHealthDownRequests:4, dnsHealthDownWindowSec:60, dnsHealthHistoryMin:60 },
    unstable: { dnsHealthWindowMin:5, dnsHealthFailCount:10, dnsHealthFailRate:2, dnsHealthLatencyP95Ms:1500, dnsHealthLatencySamples:20, dnsHealthDownRequests:5, dnsHealthDownWindowSec:60, dnsHealthHistoryMin:60 }
  };

  function setDNSPreset(id) {
    if (id === 'custom') {
      settings.update((current) => ({ ...current, dnsHealthPreset:'custom' }));
      return;
    }
    const preset = dnsPresets[id] || dnsPresets.unstable;
    settings.update((current) => ({ ...current, ...preset, dnsHealthPreset:id }));
  }

  function updateDNS(key, value) {
    settings.update((current) => ({ ...current, [key]:value, dnsHealthPreset:'custom' }));
  }
  onMount(async () => {
    await Promise.all([
      getSystem().then((value) => { system = value; }).catch(() => {}),
      refreshCatalog().catch(() => {}),
      loadLogging(),
      loadTelemetry()
    ]);
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

  <div class="rf-p1-settings">
    <nav class="rf-p1-tabs" role="tablist" aria-label={t(locale, 'settings.l1a.tabsAria')}>
      <button type="button" id="rf-p1-tab-system" role="tab" aria-controls="rf-p1-pane-system" aria-selected={activeSettingsTab === 'system'} tabindex={activeSettingsTab === 'system' ? 0 : -1} onkeydown={onSettingsTabKeydown} class:active={activeSettingsTab === 'system'} onclick={() => activeSettingsTab = 'system'}>{t(locale, 'settings.l1a.tabSystem')}</button>
      <button type="button" id="rf-p1-tab-interface" role="tab" aria-controls="rf-p1-pane-interface" aria-selected={activeSettingsTab === 'interface'} tabindex={activeSettingsTab === 'interface' ? 0 : -1} onkeydown={onSettingsTabKeydown} class:active={activeSettingsTab === 'interface'} onclick={() => activeSettingsTab = 'interface'}>{t(locale, 'settings.l1a.tabInterface')}</button>
      <button type="button" id="rf-p1-tab-logs" role="tab" aria-controls="rf-p1-pane-logs" aria-selected={activeSettingsTab === 'logs'} tabindex={activeSettingsTab === 'logs' ? 0 : -1} onkeydown={onSettingsTabKeydown} class:active={activeSettingsTab === 'logs'} onclick={() => activeSettingsTab = 'logs'}>{t(locale, 'settings.l1a.tabLogs')}</button>
      <button type="button" id="rf-p1-tab-security" role="tab" aria-controls="rf-p1-pane-security" aria-selected={activeSettingsTab === 'security'} tabindex={activeSettingsTab === 'security' ? 0 : -1} onkeydown={onSettingsTabKeydown} class:active={activeSettingsTab === 'security'} onclick={() => activeSettingsTab = 'security'}>{t(locale, 'settings.l1a.tabSecurity')}</button>
    </nav>

    <section id="rf-p1-pane-system" class="rf-p1-pane" class:inactive={activeSettingsTab !== 'system'} role="tabpanel" aria-labelledby="rf-p1-tab-system">
      <div class="rf-p1-pane-intro"><h2>{t(locale, 'settings.l1a.systemIntroTitle')}</h2><p>{t(locale, 'settings.l1a.systemIntroDescription')}</p></div>
<section class="system-card panel rf-p1-system-card">
      <div class="panel-head"><div><strong>{t(locale, 'settings.system')}</strong><span>RouterForge</span></div><span class="state-chip {$snapshot.capture_error ? 'error' : 'good'}">{$snapshot.capture_error ? 'ERROR' : 'OK'}</span></div>
      <div class="system-row"><span>RouterForge</span><strong>v{$snapshot.version || '—'}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.status')}</span><strong>{primaryDown ? 'ERROR' : primaryDegraded ? 'DEGRADED' : 'OK'}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.primaryDns')}</span><strong>{fmtInt(primaryUpstreams, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.uptime')}</span><strong>{fmtDuration($snapshot.uptime_seconds, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.requests')}</span><strong>{fmtInt($snapshot.total_requests, locale)}</strong></div>
      <div class="system-row"><span>{t(locale, 'settings.discovery')}</span><strong>{fmtAgo($snapshot.last_discovery, locale)}</strong></div>
      <div class="system-row"><span>RAM</span><strong>{system?.rss_kb ? `${(system.rss_kb / 1024).toFixed(1)} MB` : '—'}</strong></div>

<div class="rf-p1-statistics">
<section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.l1a.statisticsTitle')}</div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.l1a.installsTitle')}</strong><span>{t(locale, 'settings.l1a.installsHint')}</span></div>
          <div class="setting-control-slot"><span class="state-chip neutral">{t(locale, 'settings.l1a.installsBadge')}</span></div>
        </div>
        <div class="setting-row rf-p1b-telemetry-row">
          <div><strong>{t(locale, 'settings.l1a.telemetryTitle')}</strong><span>{t(locale, 'settings.l1a.telemetryHint')}</span></div>
          <div class="setting-control-slot"><RfToggle checked={telemetryEnabled} busy={telemetrySaving} ariaLabel={t(locale, 'settings.l1a.telemetryAria')} onToggle={(enabled) => changeTelemetry(enabled)} /></div>
        </div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.l1a.collectorTitle')}</strong><span>{t(locale, 'settings.l1a.collectorHint')}</span></div><code>{t(locale, telemetryStatus.collector_configured ? 'settings.l1a.collectorConfigured' : 'settings.l1a.collectorNotConfigured')}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.l1a.registrationTitle')}</strong><span>{t(locale, 'settings.l1a.registrationHint')}</span></div><code>{t(locale, telemetryStatus.registration_confirmed ? 'settings.l1a.registrationConfirmed' : 'settings.l1a.registrationPending')}</code></div>
        {#if telemetryError}<div class="settings-security-error">{telemetryError}</div>{/if}
        {#if telemetryNotice}<div class="settings-security-success">{telemetryNotice}</div>{/if}
      </section>
</div>
</section>
      {#if dnsInstalled}
<section class="settings-section panel">
          <div class="settings-title">DNS</div>
          <div class="appearance-intro">
            {t(locale, 'settings.l1a.dnsThresholdIntro')}
          </div>

          <div class="setting-row">
            <div>
              <strong>{t(locale, 'settings.l1a.dnsSensitivityTitle')}</strong>
              <span>{t(locale, 'settings.l1a.dnsSensitivityHint')}</span>
            </div>
            <div class="setting-control-slot">
              <select value={$settings.dnsHealthPreset} onchange={(e) => setDNSPreset(e.currentTarget.value)}>
                <option value="strict">{t(locale, 'settings.l1a.dnsPresetStrict')}</option>
                <option value="normal">{t(locale, 'settings.l1a.dnsPresetNormal')}</option>
                <option value="unstable">{t(locale, 'settings.l1a.dnsPresetUnstable')}</option>
                <option value="custom">{t(locale, 'settings.l1a.dnsPresetCustom')}</option>
              </select>
            </div>
          </div>

          <div class="setting-row static">
            <div><strong>{t(locale, 'settings.l1a.dnsPolicyTitle')}</strong><span>{t(locale, 'settings.l1a.dnsPolicyHint')}</span></div>
            <code>{$settings.dnsHealthFailCount}+ / {$settings.dnsHealthFailRate}% · p95 {$settings.dnsHealthLatencyP95Ms} ms</code>
          </div>

          {#if $settings.dnsHealthPreset === 'custom'}
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsWindowTitle')}</strong><span>{t(locale, 'settings.l1a.dnsWindowHint')}</span></div>
              <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="1" max="15" step="1" value={$settings.dnsHealthWindowMin} onchange={(e) => updateDNS('dnsHealthWindowMin', Number(e.currentTarget.value))}/><span>min</span></label></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsMinimumFailuresTitle')}</strong><span>{t(locale, 'settings.l1a.dnsMinimumFailuresHint')}</span></div>
              <div class="setting-control-slot"><input type="number" min="1" max="1000" step="1" value={$settings.dnsHealthFailCount} onchange={(e) => updateDNS('dnsHealthFailCount', Number(e.currentTarget.value))}/></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsFailureRateTitle')}</strong><span>{t(locale, 'settings.l1a.dnsFailureRateHint')}</span></div>
              <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="0.1" max="50" step="0.1" value={$settings.dnsHealthFailRate} onchange={(e) => updateDNS('dnsHealthFailRate', Number(e.currentTarget.value))}/><span>%</span></label></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsLatencyTitle')}</strong><span>{t(locale, 'settings.l1a.dnsLatencyHint')}</span></div>
              <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="100" max="10000" step="50" value={$settings.dnsHealthLatencyP95Ms} onchange={(e) => updateDNS('dnsHealthLatencyP95Ms', Number(e.currentTarget.value))}/><span>ms</span></label></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsMinimumSamplesTitle')}</strong><span>{t(locale, 'settings.l1a.dnsMinimumSamplesHint')}</span></div>
              <div class="setting-control-slot"><input type="number" min="1" max="1000" step="1" value={$settings.dnsHealthLatencySamples} onchange={(e) => updateDNS('dnsHealthLatencySamples', Number(e.currentTarget.value))}/></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsDownAttemptsTitle')}</strong><span>{t(locale, 'settings.l1a.dnsDownAttemptsHint')}</span></div>
              <div class="setting-control-slot"><input type="number" min="1" max="100" step="1" value={$settings.dnsHealthDownRequests} onchange={(e) => updateDNS('dnsHealthDownRequests', Number(e.currentTarget.value))}/></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsDownWindowTitle')}</strong><span>{t(locale, 'settings.l1a.dnsDownWindowHint')}</span></div>
              <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="30" max="300" step="30" value={$settings.dnsHealthDownWindowSec} onchange={(e) => updateDNS('dnsHealthDownWindowSec', Number(e.currentTarget.value))}/><span>s</span></label></div>
            </div>
            <div class="setting-row">
              <div><strong>{t(locale, 'settings.l1a.dnsIssueMemoryTitle')}</strong><span>{t(locale, 'settings.l1a.dnsIssueMemoryHint')}</span></div>
              <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="15" max="60" step="5" value={$settings.dnsHealthHistoryMin} onchange={(e) => updateDNS('dnsHealthHistoryMin', Number(e.currentTarget.value))}/><span>min</span></label></div>
            </div>
          {/if}
        </section>
      {/if}
<section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.monitoring')}</div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.l1a.cpuWarningTitle')}</strong><span>{t(locale, 'settings.l1a.cpuWarningHint')}</span></div>
          <div class="setting-control-slot"><label class="temperature-threshold"><input type="number" min="50" max="89" step="1" value={$settings.cpuTempWarning} onchange={(e) => update('cpuTempWarning', Number(e.currentTarget.value))}/><span>&#176;C</span></label></div>
        </div>



      </section>
    </section>

    <section id="rf-p1-pane-interface" class="rf-p1-pane" class:inactive={activeSettingsTab !== 'interface'} role="tabpanel" aria-labelledby="rf-p1-tab-interface">
      <div class="rf-p1-pane-intro"><h2>{t(locale, 'settings.l1a.interfaceIntroTitle')}</h2><p>{t(locale, 'settings.l1a.interfaceIntroDescription')}</p></div>
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
    </section>

    <section id="rf-p1-pane-logs" class="rf-p1-pane" class:inactive={activeSettingsTab !== 'logs'} role="tabpanel" aria-labelledby="rf-p1-tab-logs">
      <div class="rf-p1-pane-intro"><h2>{t(locale, 'settings.l1a.logsIntroTitle')}</h2><p>{t(locale, 'settings.l1a.logsIntroDescription')}</p></div>
<section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.l1a.logsTitle')}</div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.l1a.storageTitle')}</strong><span>{t(locale, 'settings.l1a.storageHint')}</span></div>
          <div class="setting-control-slot">
            <select bind:value={loggingMode}>
              <option value="entware">Entware — /opt/var/log</option>
              <option value="ram" disabled={!loggingRAMAvailable}>{t(locale, 'settings.l1a.storageRam')}</option><option value="custom">{t(locale, 'settings.l1a.storageCustom')}</option>
            </select>
          </div>
        </div>
        {#if loggingMode === 'custom'}
          <div class="setting-row">
            <div><strong>{t(locale, 'settings.l1a.customPathTitle')}</strong><span>{t(locale, 'settings.l1a.customPathHint')}</span></div>
            <div class="setting-control-slot"><input type="text" bind:value={loggingCustomPath} placeholder="/opt/var/log/routerforge-custom.log" autocomplete="off" spellcheck="false" /></div>
          </div>
        {/if}
        <div class="setting-row static"><div><strong>{t(locale, 'settings.l1a.activeLogTitle')}</strong><span>{t(locale, 'settings.l1a.activeLogHint')}</span></div><code>{loggingActivePath || '—'}</code></div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.l1a.maxSizeTitle')}</strong><span>{t(locale, 'settings.l1a.maxSizeHint')}</span></div>
          <div class="setting-control-slot"><select bind:value={loggingMaxMB}><option value={1}>1 {t(locale, 'settings.l1a.mbUnit')}</option><option value={2}>2 {t(locale, 'settings.l1a.mbUnit')}</option><option value={4}>4 {t(locale, 'settings.l1a.mbUnit')}</option><option value={8}>8 {t(locale, 'settings.l1a.mbUnit')}</option><option value={16}>16 {t(locale, 'settings.l1a.mbUnit')}</option></select></div>
        </div>
        <div class="setting-row">
          <div><strong>{t(locale, 'settings.l1a.backupsTitle')}</strong><span>{t(locale, 'settings.l1a.backupsHint')}</span></div>
          <div class="setting-control-slot"><select bind:value={loggingBackups}><option value={1}>1</option><option value={2}>2</option><option value={3}>3</option><option value={4}>4</option><option value={5}>5</option></select></div>
        </div>
        <div class="setting-row"><div><strong>{t(locale, 'settings.l1a.saveLoggingTitle')}</strong><span>{t(locale, 'settings.l1a.saveLoggingHint')}</span></div><div class="setting-control-slot"><button type="button" onclick={saveLogging} disabled={loggingBusy}>{t(locale, 'settings.l1a.save')}</button></div></div>
        {#if loggingError}<div class="settings-security-error">{loggingError}</div>{/if}
        {#if loggingSaved}<div class="settings-security-success">{loggingSaved}</div>{/if}
      </section>
    </section>

    <section id="rf-p1-pane-security" class="rf-p1-pane" class:inactive={activeSettingsTab !== 'security'} role="tabpanel" aria-labelledby="rf-p1-tab-security">
      <div class="rf-p1-pane-intro"><h2>{t(locale, 'settings.l1a.securityIntroTitle')}</h2><p>{t(locale, 'settings.l1a.securityIntroDescription')}</p></div>
<section class="settings-section panel">
        <div class="settings-title">{t(locale, 'settings.security')}</div>
        <div class="setting-row security-setting-row">
          <div><strong>{t(locale, 'settings.requireAuth')}</strong><span>{t(locale, 'settings.requireAuthHint')}</span></div>
          <div class="security-control setting-control-slot">
            <RfToggle checked={$authState.required} busy={authBusy} ariaLabel={t(locale, 'settings.requireAuth')} onToggle={() => requestAuthToggle()} />
          </div>
        </div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.session')}</strong><span>{t(locale, 'settings.sessionHint')}</span></div><code>{t(locale, 'common.hours', { count: $authState.session_hours || 12 })}</code></div>
        <div class="setting-row static"><div><strong>{t(locale, 'settings.backend')}</strong><span>{t(locale, 'settings.backendHint')}</span></div><code>entware-root</code></div>
        {#if authError}<div class="settings-security-error">{authError}</div>{/if}
      </section>
    </section>
  </div>
</div>

{#if showEnableAuth}
  <AuthEnableModal busy={authBusy} error={authError} oncancel={() => { if (!authBusy) { showEnableAuth = false; authError = ''; } }} onconfirm={enableAuth}/>
{/if}
