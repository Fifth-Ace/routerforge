<script>
  import { onDestroy } from 'svelte';

  export let workspace;
  export let locale = 'ru';
  export let onclose = () => {};

  let frameReady = false;
  let frameURL = '';
  let revealTimer;

  $: item = workspace?.item || {};
  $: url = workspace?.url || '';
  $: probe = workspace?.probe || {};
  $: title = locale === 'ru' ? '\u0412\u043D\u0435\u0448\u043D\u0438\u0439 Web UI' : 'External Web UI';
  $: externalLabel = locale === 'ru' ? '\u041E\u0442\u043A\u0440\u044B\u0442\u044C \u043E\u0442\u0434\u0435\u043B\u044C\u043D\u043E' : 'Open externally';
  $: closeLabel = locale === 'ru' ? '\u0417\u0430\u043A\u0440\u044B\u0442\u044C' : 'Close';
  $: loadingLabel = locale === 'ru' ? '\u0417\u0430\u0433\u0440\u0443\u0437\u043A\u0430 Web UI\u2026' : 'Loading Web UI\u2026';

  $: if (url !== frameURL) {
    frameURL = url;
    frameReady = false;
    if (revealTimer) {
      clearTimeout(revealTimer);
      revealTimer = undefined;
    }
  }

  function handleFrameLoad() {
    if (revealTimer) clearTimeout(revealTimer);
    revealTimer = setTimeout(() => {
      frameReady = true;
      revealTimer = undefined;
    }, 120);
  }

  function handleKeydown(event) {
    if (event.key === 'Escape') onclose();
  }

  onDestroy(() => {
    if (revealTimer) clearTimeout(revealTimer);
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if workspace && url}
  <div class="web-workspace-overlay" role="presentation" onclick={(event) => event.currentTarget === event.target && onclose()}>
    <section class="web-workspace" role="dialog" aria-modal="true" aria-label={`${title}: ${item.name || item.id || ''}`}>
      <header class="web-workspace-head">
        <div>
          <div class="web-workspace-title"><strong>{item.name || item.id || title}</strong><span class="mono">:{item?.web?.port || probe.port || '-'}</span></div>
          <div class="web-workspace-meta mono">runtime-probe HTTP {probe.status_code || '-'} | {probe.frame_header_policy || 'unknown'}</div>
        </div>
        <div class="web-workspace-actions">
          <a class="button" target="_blank" rel="noopener noreferrer" href={url}>{externalLabel}</a>
          <button class="button" type="button" onclick={onclose}>{closeLabel}</button>
        </div>
      </header>

      <div class="web-frame-shell" class:ready={frameReady}>
        <div class="web-frame-loading" aria-hidden={frameReady}>
          <div class="web-frame-loading-content">
            <span class="web-frame-spinner" aria-hidden="true"></span>
            <div>
              <strong>{item.name || item.id || title}</strong>
              <small>{loadingLabel}</small>
            </div>
          </div>
        </div>

        <iframe
          class:ready={frameReady}
          title={item.name || title}
          src={url}
          sandbox="allow-downloads allow-forms allow-popups allow-same-origin allow-scripts"
          referrerpolicy="same-origin"
          onload={handleFrameLoad}
        ></iframe>
      </div>
    </section>
  </div>
{/if}

<style>
  .web-workspace-overlay {
    position: fixed;
    inset: 0;
    z-index: 1200;
    display: grid;
    place-items: center;
    padding: 2vh 2vw;
    background: rgba(5, 10, 18, .72);
    backdrop-filter: blur(8px);
    animation: web-workspace-overlay-in 160ms ease-out both;
  }

  .web-workspace {
    width: min(96vw, 1680px);
    height: 94vh;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    overflow: hidden;
    border: 1px solid var(--rf-border, var(--border));
    border-radius: 16px;
    background: var(--rf-card, var(--panel));
    box-shadow: 0 24px 80px rgba(0, 0, 0, .45);
    transform-origin: 50% 48%;
    animation: web-workspace-panel-in 220ms cubic-bezier(.2, .8, .2, 1) both;
  }

  .web-workspace-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: .8rem 1rem;
    border-bottom: 1px solid var(--rf-border, var(--border));
  }

  .web-workspace-title {
    display: flex;
    align-items: baseline;
    gap: .55rem;
  }

  .web-workspace-meta {
    margin-top: .22rem;
    color: var(--rf-muted, var(--muted));
    font-size: .76rem;
  }

  .web-workspace-actions {
    display: flex;
    align-items: center;
    gap: .55rem;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .web-frame-shell {
    position: relative;
    min-height: 0;
    overflow: hidden;
    background: var(--rf-card, var(--panel));
  }

  .web-frame-loading {
    position: absolute;
    inset: 0;
    z-index: 1;
    display: grid;
    place-items: center;
    padding: 2rem;
    background: var(--rf-card, var(--panel));
    opacity: 1;
    pointer-events: auto;
    transition: opacity 180ms ease;
  }

  .web-frame-shell.ready .web-frame-loading {
    opacity: 0;
    pointer-events: none;
  }

  .web-frame-loading-content {
    display: flex;
    align-items: center;
    gap: .8rem;
    color: var(--rf-muted, var(--muted));
  }

  .web-frame-loading-content > div {
    display: grid;
    gap: .2rem;
  }

  .web-frame-loading-content strong {
    color: inherit;
    font-size: .92rem;
  }

  .web-frame-loading-content small {
    color: var(--rf-muted, var(--muted));
    font-size: .78rem;
  }

  .web-frame-spinner {
    width: 24px;
    height: 24px;
    flex: 0 0 24px;
    box-sizing: border-box;
    border: 2px solid var(--rf-border, var(--border));
    border-top-color: currentColor;
    border-radius: 50%;
    animation: web-frame-spin 720ms linear infinite;
  }

  iframe {
    width: 100%;
    height: 100%;
    display: block;
    border: 0;
    background: transparent;
    opacity: 0;
    transition: opacity 180ms ease 30ms;
  }

  iframe.ready {
    opacity: 1;
  }

  @keyframes web-workspace-overlay-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes web-workspace-panel-in {
    from {
      opacity: 0;
      transform: translateY(10px) scale(.985);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @keyframes web-frame-spin {
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .web-workspace-overlay,
    .web-workspace,
    .web-frame-spinner {
      animation: none;
    }

    .web-frame-loading,
    iframe {
      transition: none;
    }
  }

  @media (max-width: 760px) {
    .web-workspace-overlay { padding: 0; }
    .web-workspace {
      width: 100vw;
      height: 100vh;
      border: 0;
      border-radius: 0;
    }
    .web-workspace-head {
      align-items: flex-start;
      flex-direction: column;
    }
    .web-workspace-actions { width: 100%; justify-content: flex-start; }
  }
</style>
