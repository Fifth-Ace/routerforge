<script>
  export let workspace;
  export let locale = 'ru';
  export let onclose = () => {};

  $: item = workspace?.item || {};
  $: url = workspace?.url || '';
  $: probe = workspace?.probe || {};
  $: title = locale === 'ru' ? '\u0412\u043D\u0435\u0448\u043D\u0438\u0439 Web UI' : 'External Web UI';
  $: externalLabel = locale === 'ru' ? '\u041E\u0442\u043A\u0440\u044B\u0442\u044C \u043E\u0442\u0434\u0435\u043B\u044C\u043D\u043E' : 'Open externally';
  $: closeLabel = locale === 'ru' ? '\u0417\u0430\u043A\u0440\u044B\u0442\u044C' : 'Close';

  function handleKeydown(event) {
    if (event.key === 'Escape') onclose();
  }
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

      <div class="web-frame-shell">
        <iframe
          title={item.name || title}
          src={url}
          sandbox="allow-downloads allow-forms allow-popups allow-same-origin allow-scripts"
          referrerpolicy="same-origin"
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
    min-height: 0;
    background: #fff;
  }

  iframe {
    width: 100%;
    height: 100%;
    display: block;
    border: 0;
    background: #fff;
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
