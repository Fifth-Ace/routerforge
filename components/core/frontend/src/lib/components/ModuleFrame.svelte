<script>
  import { onDestroy } from 'svelte';
  import { settings } from '$lib/stores/settings.js';
  import { withRequestTimeout } from '$lib/http.js';

  export let moduleId = '';
  export let view = 'overview';

  let state = 'checking';
  let retryTimer = null;
  let probeGeneration = 0;
  let frame = null;
  let frameHeight = 760;
  let moduleUIRevision = Date.now().toString(36);

  function visibleWorkspaceHeight() {
    if (!frame || typeof window === 'undefined') return 720;
    const top = frame.getBoundingClientRect().top;
    return Math.max(720, Math.floor(window.innerHeight - top - 8));
  }

  function usesVisibleWorkspaceFloor() {
    return moduleId === 'admin' || moduleId === 'monitoring';
  }

  function applyFrameHeight(reportedHeight = frameHeight) {
    const reported = Number(reportedHeight || 0);
    const floor = usesVisibleWorkspaceFloor() ? visibleWorkspaceHeight() : 0;
    const next = Math.max(reported, floor);
    if (Number.isFinite(next) && next >= 360 && next <= 12000) {
      frameHeight = Math.ceil(next);
    }
  }

  function refreshVisibleWorkspaceHeight() {
    if (!usesVisibleWorkspaceFloor()) return;
    applyFrameHeight(frameHeight);
  }

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  $: moduleThemeQuery = moduleId === 'network-tools'
    ? `&theme=${encodeURIComponent($settings.theme || 'forge')}&accent=${encodeURIComponent($settings.accent || '#38bdf8')}&background=${encodeURIComponent($settings.background || '#0b0d10')}&text=${encodeURIComponent($settings.text || '#f5f7fa')}&density=${encodeURIComponent($settings.density || 'normal')}&radius=${encodeURIComponent($settings.radius || 'default')}`
    : '';
  $: src = `/api/modules/${encodeURIComponent(moduleId)}/ui/index.html?locale=${encodeURIComponent(locale)}&view=${encodeURIComponent(view)}&rev=${encodeURIComponent(moduleUIRevision)}${moduleThemeQuery}`;
  $: restartProbe(moduleId);

  function clearRetry() {
    if (retryTimer !== null) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
  }

  function restartProbe(id) {
    clearRetry();
    const generation = ++probeGeneration;
    if (!id) {
      state = 'not-installed';
      return;
    }
    state = 'checking';
    probe(id, generation);
  }

  async function probe(id, generation) {
    const path = `/api/modules/${encodeURIComponent(id)}/health`;
    try {
      const { response, payload } = await withRequestTimeout(path, 5000, async (signal) => {
        const response = await fetch(path, {
          cache: 'no-store',
          credentials: 'same-origin',
          headers: { Accept: 'application/json' },
          signal
        });
        let payload = null;
        if (response.status === 503) {
          try { payload = await response.json(); } catch { payload = null; }
        }
        return { response, payload };
      });

      if (generation !== probeGeneration) return;
      if (response.ok) {
        state = 'ready';
        return;
      }
      if (response.status === 404 || (response.status === 503 && payload?.installed === false)) {
        state = 'not-installed';
        return;
      }
      state = 'reconnecting';
    } catch {
      if (generation !== probeGeneration) return;
      state = 'reconnecting';
    }

    clearRetry();
    retryTimer = setTimeout(() => probe(id, generation), 800);
  }

  function moduleMessage(event) {
    if (event.origin !== window.location.origin || !frame || event.source !== frame.contentWindow) return;
    const data = event.data;
    if (!data || data.type !== 'routerforge-module-height' || data.moduleId !== moduleId) return;
    const height = Number(data.height || 0);
    if (!Number.isFinite(height) || height < 360 || height > 12000) return;
    applyFrameHeight(height);
  }

  window.addEventListener('message', moduleMessage);
  window.addEventListener('resize', refreshVisibleWorkspaceHeight);

  onDestroy(() => {
    ++probeGeneration;
    clearRetry();
    window.removeEventListener('message', moduleMessage);
    window.removeEventListener('resize', refreshVisibleWorkspaceHeight);
  });
</script>

<div class="routerforge-module-frame">
  {#if state === 'ready'}
    <iframe
      bind:this={frame}
      title={`RouterForge ${moduleId}`}
      src={src}
      loading="eager"
      onload={refreshVisibleWorkspaceHeight}
      referrerpolicy="same-origin"
      style={`height:${frameHeight}px`}
    ></iframe>
  {:else}
    <div class="module-state" role="status" aria-live="polite">
      {#if state === 'not-installed'}
        <strong>{locale === 'en' ? 'Module is not installed' : 'Модуль не установлен'}</strong>
        <span>{locale === 'en' ? 'Install it from RouterForge App Center to open this workspace.' : 'Установите его через Центр приложений RouterForge, чтобы открыть рабочую область.'}</span>
      {:else}
        <span class="spinner" aria-hidden="true"></span>
        <strong>{locale === 'en' ? 'Reconnecting to module…' : 'Переподключение к модулю…'}</strong>
        <span>{locale === 'en' ? 'RouterForge is waiting for the module runtime to become ready.' : 'RouterForge ждёт готовности runtime модуля.'}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .routerforge-module-frame { width: 100%; min-height: calc(100vh - 11rem); }
  .module-state {
    display: grid;
    justify-items: center;
    align-content: center;
    gap: 0.55rem;
    min-height: min(720px, calc(100vh - 11rem));
    padding: 2rem;
    box-sizing: border-box;
    text-align: center;
    color: inherit;
  }
  .module-state span { max-width: 42rem; opacity: 0.72; }
  .spinner {
    width: 1.55rem;
    height: 1.55rem;
    border: 0.16rem solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: module-spin 0.8s linear infinite;
    opacity: 0.72;
  }
  iframe {
    display: block;
    width: 100%;
    min-height: 720px;
    border: 0;
    background: transparent;
  }
  @keyframes module-spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
  @media (max-width: 760px) {
    iframe { min-height: 900px; }
    .module-state { min-height: min(900px, calc(100vh - 8rem)); }
  }
</style>
