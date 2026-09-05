<script>
  import { onDestroy } from 'svelte';
  import { settings } from '$lib/stores/settings.js';

  export let moduleId = '';
  export let view = 'overview';

  let state = 'checking';
  let retryTimer = null;
  let probeGeneration = 0;

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  $: src = `/api/modules/${encodeURIComponent(moduleId)}/ui/index.html?locale=${encodeURIComponent(locale)}&view=${encodeURIComponent(view)}`;
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
    try {
      const response = await fetch(`/api/modules/${encodeURIComponent(id)}/health`, {
        cache: 'no-store',
        credentials: 'same-origin',
        headers: { Accept: 'application/json' }
      });

      if (generation !== probeGeneration) return;

      if (response.ok) {
        state = 'ready';
        return;
      }

      if (response.status === 404) {
        state = 'not-installed';
        return;
      }

      if (response.status === 503) {
        let payload = null;
        try {
          payload = await response.json();
        } catch {
          payload = null;
        }

        if (generation !== probeGeneration) return;

        if (payload?.installed === false) {
          state = 'not-installed';
          return;
        }
      }

      state = 'reconnecting';
    } catch {
      if (generation !== probeGeneration) return;
      state = 'reconnecting';
    }

    clearRetry();
    retryTimer = setTimeout(() => probe(id, generation), 800);
  }

  onDestroy(() => {
    ++probeGeneration;
    clearRetry();
  });
</script>

<div class="routerforge-module-frame">
  {#if state === 'ready'}
    <iframe
      title={`RouterForge ${moduleId}`}
      src={src}
      loading="eager"
      referrerpolicy="same-origin"
    ></iframe>
  {:else}
    <div class="module-state" role="status" aria-live="polite">
      {#if state === 'not-installed'}
        <strong>{locale === 'en' ? 'Module is not installed' : 'Модуль не установлен'}</strong>
        <span>{locale === 'en' ? 'Install it from RouterForge Marketplace to open this page.' : 'Установите его через RouterForge Marketplace, чтобы открыть эту страницу.'}</span>
      {:else}
        <span class="spinner" aria-hidden="true"></span>
        <strong>{locale === 'en' ? 'Reconnecting to module…' : 'Переподключение к модулю…'}</strong>
        <span>{locale === 'en' ? 'RouterForge is waiting for the module runtime to become ready.' : 'RouterForge ждёт готовности runtime модуля.'}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .routerforge-module-frame {
    width: 100%;
    min-height: calc(100vh - 11rem);
  }
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
  .module-state span {
    max-width: 42rem;
    opacity: 0.72;
  }
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
    height: calc(100vh - 10.5rem);
    min-height: 720px;
    border: 0;
    background: transparent;
  }
  @keyframes module-spin {
    to { transform: rotate(360deg); }
  }
  @media (prefers-reduced-motion: reduce) {
    .spinner { animation: none; }
  }
  @media (max-width: 760px) {
    iframe { min-height: 900px; height: calc(100vh - 8rem); }
    .module-state { min-height: min(900px, calc(100vh - 8rem)); }
  }
</style>
