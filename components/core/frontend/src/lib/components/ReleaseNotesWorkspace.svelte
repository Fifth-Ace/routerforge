<script>
  import { onMount } from 'svelte';
  import { parseReleaseNote } from '$lib/release-notes.js';

  export let locale = 'ru';
  export let onclose = () => {};

  let index = { current_stable:'', releases:[] };
  let selectedVersion = '';
  let blocks = [];
  let loading = true;
  let error = '';

  $: selected = (index.releases || []).find((item) => item.version === selectedVersion) || null;

  const text = (ru, en) => locale === 'ru' ? ru : en;

  onMount(() => {
    void loadIndex();
  });

  async function loadIndex() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/release-notes/index.json', { cache:'no-store' });
      if (!response.ok) throw new Error(`release notes index HTTP ${response.status}`);
      index = await response.json();

      const available = new Set((index.releases || []).map((item) => item.version));
      selectedVersion = available.has(index.current_stable)
        ? index.current_stable
        : index.releases?.[0]?.version || '';

      if (selectedVersion) await loadVersion(selectedVersion);
    } catch (cause) {
      error = cause?.message || String(cause);
    } finally {
      loading = false;
    }
  }

  async function loadVersion(version) {
    const item = (index.releases || []).find((candidate) => candidate.version === version);
    if (!item) return;

    loading = true;
    error = '';
    try {
      const response = await fetch(`/release-notes/${encodeURIComponent(item.file)}`, { cache:'no-store' });
      if (!response.ok) throw new Error(`release note HTTP ${response.status}`);
      selectedVersion = version;
      blocks = parseReleaseNote(await response.text());
    } catch (cause) {
      blocks = [];
      error = cause?.message || String(cause);
    } finally {
      loading = false;
    }
  }

  function handleKeydown(event) {
    if (event.key === 'Escape') onclose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="release-workspace-overlay" role="presentation" onclick={(event) => event.currentTarget === event.target && onclose()}>
  <section class="release-workspace" role="dialog" aria-modal="true" aria-label={text('Что нового','What’s new')}>
    <header class="release-workspace-head">
      <div>
        <div class="release-workspace-title">
          <strong>{text('Что нового','What’s new')}</strong>
          {#if index.current_stable}<span class="state-chip good">STABLE {index.current_stable}</span>{/if}
        </div>
        <div class="release-workspace-meta mono">ROUTERFORGE / RELEASE NOTES</div>
      </div>
      <button class="button" type="button" onclick={onclose}>{text('Закрыть','Close')}</button>
    </header>

    <div class="release-workspace-body">
      <aside class="release-workspace-list">
        <div class="release-list-head">
          <strong>{text('Версии','Versions')}</strong>
          <span class="mono">{index.releases?.length || 0}</span>
        </div>

        {#each index.releases || [] as item (item.version)}
          <button
            type="button"
            class:active={item.version === selectedVersion}
            onclick={() => loadVersion(item.version)}
          >
            <span>
              <strong>RouterForge {item.version}</strong>
              <small>{item.title || `RouterForge ${item.version}`}</small>
            </span>
            {#if item.version === index.current_stable}<em>STABLE</em>{/if}
          </button>
        {/each}
      </aside>

      <section class="release-workspace-document">
        {#if loading}
          <div class="release-state">{text('Загрузка патчноута…','Loading release note…')}</div>
        {:else if error}
          <div class="release-state error">{error}</div>
        {:else if !selected}
          <div class="release-state">{text('Патчноуты не найдены.','No release notes found.')}</div>
        {:else}
          <div class="release-document-head">
            <div>
              <span class="mono">{selected.file}</span>
              <strong>{selected.title || `RouterForge ${selected.version}`}</strong>
            </div>
            <span class="version-badge">v{selected.version}</span>
          </div>

          <article class="release-markdown">
            {#each blocks as block}
              {#if block.type === 'heading'}
                {#if block.level <= 1}
                  <h2>{block.text}</h2>
                {:else if block.level === 2}
                  <h3>{block.text}</h3>
                {:else}
                  <h4>{block.text}</h4>
                {/if}
              {:else if block.type === 'paragraph'}
                <p>{block.text}</p>
              {:else if block.type === 'code'}
                <pre><code>{block.text}</code></pre>
              {:else if block.type === 'list'}
                <ul>
                  {#each block.items as item}<li>{item}</li>{/each}
                </ul>
              {:else if block.type === 'quote'}
                <blockquote>{block.text}</blockquote>
              {:else if block.type === 'table'}
                <div class="release-table-wrap">
                  <table>
                    <thead><tr>{#each block.headers as cell}<th>{cell}</th>{/each}</tr></thead>
                    <tbody>
                      {#each block.rows as row}
                        <tr>{#each row as cell}<td>{cell}</td>{/each}</tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            {/each}
          </article>
        {/if}
      </section>
    </div>
  </section>
</div>

<style>
  .release-workspace-overlay {
    position: fixed;
    inset: 0;
    z-index: 1200;
    display: grid;
    place-items: center;
    padding: 2vh 2vw;
    background: rgba(5,10,18,.72);
    backdrop-filter: blur(8px);
    animation: release-overlay-in 160ms ease-out both;
  }

  .release-workspace {
    width: min(96vw,1680px);
    height: 94vh;
    display: grid;
    grid-template-rows: auto minmax(0,1fr);
    overflow: hidden;
    border: 1px solid var(--rf-border,var(--border));
    border-radius: 16px;
    background: var(--rf-card,var(--panel));
    box-shadow: 0 24px 80px rgba(0,0,0,.45);
    animation: release-panel-in 220ms cubic-bezier(.2,.8,.2,1) both;
  }

  .release-workspace-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: .8rem 1rem;
    border-bottom: 1px solid var(--rf-border,var(--border));
  }

  .release-workspace-title {
    display: flex;
    align-items: center;
    gap: .65rem;
    flex-wrap: wrap;
  }

  .release-workspace-meta {
    margin-top: .22rem;
    color: var(--rf-muted,var(--muted));
    font-size: .76rem;
  }

  .release-workspace-body {
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(220px,280px) minmax(0,1fr);
    overflow: hidden;
  }

  .release-workspace-list {
    min-height: 0;
    overflow: auto;
    padding: 8px;
    border-right: 1px solid var(--rf-border,var(--border));
    background: var(--rf-surface,var(--panel));
  }

  .release-list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 9px 10px;
    color: var(--rf-muted,var(--muted));
  }

  .release-workspace-list button {
    appearance: none;
    width: 100%;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    padding: 10px;
    text-align: left;
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
  }

  .release-workspace-list button:hover {
    background: var(--rf-hover,var(--hover));
  }

  .release-workspace-list button.active {
    border-color: var(--rf-accent-border,var(--border));
    background: var(--rf-accent-soft,rgba(56,189,248,.08));
  }

  .release-workspace-list button span {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .release-workspace-list button small {
    overflow: hidden;
    color: var(--rf-muted,var(--muted));
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .release-workspace-list button em {
    color: var(--rf-accent,var(--accent));
    font-size: 10px;
    font-style: normal;
    font-weight: 700;
  }

  .release-workspace-document {
    min-width: 0;
    min-height: 0;
    overflow: auto;
    background: var(--rf-card,var(--panel));
  }

  .release-document-head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--rf-border,var(--border));
    background: var(--rf-card,var(--panel));
  }

  .release-document-head > div {
    display: grid;
    gap: 4px;
  }

  .release-document-head span.mono {
    color: var(--rf-muted,var(--muted));
    font-size: 11px;
  }

  .release-document-head strong {
    font-size: 18px;
  }

  .release-markdown {
    padding: 18px 20px 28px;
    line-height: 1.6;
  }

  .release-markdown h2 { margin: 0 0 16px; font-size: 25px; }
  .release-markdown h3 { margin: 24px 0 10px; font-size: 19px; }
  .release-markdown h4 { margin: 20px 0 8px; font-size: 15px; }
  .release-markdown p { margin: 0 0 13px; color: var(--rf-text,var(--text)); }
  .release-markdown ul { margin: 0 0 15px; padding-left: 22px; }
  .release-markdown li { margin: 5px 0; }
  .release-markdown pre {
    margin: 12px 0 16px;
    padding: 13px 14px;
    overflow: auto;
    border: 1px solid var(--rf-border,var(--border));
    border-radius: 8px;
    background: var(--rf-bg,var(--bg));
    font-family: var(--font-mono,monospace);
    font-size: 12px;
  }

  .release-markdown blockquote {
    margin: 12px 0 16px;
    padding: 8px 12px;
    border-left: 3px solid var(--rf-accent,var(--accent));
    background: var(--rf-accent-soft,rgba(56,189,248,.08));
    color: var(--rf-muted,var(--muted));
  }

  .release-table-wrap {
    margin: 12px 0 18px;
    overflow: auto;
    border: 1px solid var(--rf-border,var(--border));
    border-radius: 8px;
  }

  .release-table-wrap table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .release-table-wrap th,
  .release-table-wrap td {
    padding: 8px 10px;
    border-bottom: 1px solid var(--rf-border,var(--border));
    text-align: left;
    white-space: nowrap;
  }

  .release-table-wrap th {
    background: var(--rf-surface-2,var(--surface-2));
  }

  .release-table-wrap tr:last-child td {
    border-bottom: 0;
  }

  .release-state {
    padding: 28px;
    color: var(--rf-muted,var(--muted));
  }

  .release-state.error {
    color: #ff8b8b;
  }

  @keyframes release-overlay-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes release-panel-in {
    from {
      opacity: 0;
      transform: translateY(10px) scale(.985);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .release-workspace-overlay,
    .release-workspace {
      animation: none;
    }
  }

  @media (max-width: 760px) {
    .release-workspace-overlay { padding: 0; }
    .release-workspace {
      width: 100vw;
      height: 100vh;
      border: 0;
      border-radius: 0;
    }
    .release-workspace-head {
      align-items: flex-start;
      flex-direction: column;
    }
    .release-workspace-body {
      grid-template-columns: 1fr;
      grid-template-rows: auto minmax(0,1fr);
    }
    .release-workspace-list {
      display: flex;
      overflow-x: auto;
      border-right: 0;
      border-bottom: 1px solid var(--rf-border,var(--border));
    }
    .release-list-head { display: none; }
    .release-workspace-list button {
      min-width: 180px;
      width: auto;
    }
  }
</style>
