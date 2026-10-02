<script>
  import { onMount } from 'svelte';
  import { settings } from '$lib/stores/settings.js';
  import { parseReleaseNote } from '$lib/release-notes.js';

  let index = { current_stable:'', releases:[] };
  let selectedVersion = '';
  let blocks = [];
  let loading = true;
  let error = '';

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
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

      const requested = new URLSearchParams(window.location.search).get('version') || '';
      const available = new Set((index.releases || []).map((item) => item.version));
      selectedVersion = available.has(requested)
        ? requested
        : available.has(index.current_stable)
          ? index.current_stable
          : index.releases?.[0]?.version || '';

      if (selectedVersion) await loadVersion(selectedVersion, false);
    } catch (cause) {
      error = cause?.message || String(cause);
    } finally {
      loading = false;
    }
  }

  async function loadVersion(version, syncURL = true) {
    const item = (index.releases || []).find((candidate) => candidate.version === version);
    if (!item) return;

    loading = true;
    error = '';
    try {
      const response = await fetch(`/release-notes/${encodeURIComponent(item.file)}`, { cache:'no-store' });
      if (!response.ok) throw new Error(`release note HTTP ${response.status}`);
      const markdown = await response.text();

      selectedVersion = version;
      blocks = parseReleaseNote(markdown);

      if (syncURL) {
        const url = new URL(window.location.href);
        url.searchParams.set('version', version);
        window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}`);
      }
    } catch (cause) {
      blocks = [];
      error = cause?.message || String(cause);
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head><title>RouterForge — {text('Что нового','What’s new')}</title></svelte:head>

<div class="page release-notes-page">
  <div class="page-head">
    <div>
      <span class="routerforge-eyebrow mono">ROUTERFORGE / RELEASE NOTES</span>
      <h1>{text('Что нового','What’s new')}</h1>
      <p>{text(
        'Патчноуты поставляются вместе с RouterForge Core и доступны без GitHub.',
        'Release notes ship with RouterForge Core and remain available without GitHub.'
      )}</p>
    </div>
    {#if index.current_stable}
      <span class="state-chip good">STABLE {index.current_stable}</span>
    {/if}
  </div>

  <div class="release-layout">
    <aside class="panel release-list">
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

    <section class="panel release-document">
      {#if loading}
        <div class="release-empty">{text('Загрузка патчноута…','Loading release note…')}</div>
      {:else if error}
        <div class="release-error">{error}</div>
      {:else if !selected}
        <div class="release-empty">{text('Патчноуты не найдены.','No release notes found.')}</div>
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
</div>

<style>
  .release-notes-page{display:grid;gap:18px}
  .release-layout{display:grid;grid-template-columns:minmax(220px,280px) minmax(0,1fr);gap:14px;align-items:start}
  .release-list{padding:8px;display:grid;gap:5px;position:sticky;top:76px}
  .release-list-head{display:flex;align-items:center;justify-content:space-between;padding:8px 9px 10px;color:var(--rf-muted,var(--muted))}
  .release-list button{appearance:none;border:1px solid transparent;border-radius:8px;background:transparent;color:inherit;padding:10px;text-align:left;display:flex;gap:8px;align-items:center;justify-content:space-between;cursor:pointer}
  .release-list button:hover{background:var(--rf-hover,var(--hover))}
  .release-list button.active{border-color:var(--rf-accent-border,var(--border));background:var(--rf-accent-soft,rgba(56,189,248,.08))}
  .release-list button span{display:grid;gap:3px;min-width:0}
  .release-list button small{color:var(--rf-muted,var(--muted));overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .release-list button em{font-style:normal;font-size:10px;color:var(--rf-accent,var(--accent));font-weight:700}
  .release-document{overflow:hidden}
  .release-document-head{padding:14px 16px;border-bottom:1px solid var(--rf-border,var(--border));display:flex;justify-content:space-between;gap:16px;align-items:center}
  .release-document-head>div{display:grid;gap:4px}
  .release-document-head span.mono{font-size:11px;color:var(--rf-muted,var(--muted))}
  .release-document-head strong{font-size:18px}
  .release-markdown{padding:18px 20px 28px;line-height:1.6}
  .release-markdown h2{font-size:25px;margin:0 0 16px}
  .release-markdown h3{font-size:19px;margin:24px 0 10px}
  .release-markdown h4{font-size:15px;margin:20px 0 8px}
  .release-markdown p{margin:0 0 13px;color:var(--rf-text,var(--text))}
  .release-markdown ul{margin:0 0 15px;padding-left:22px}
  .release-markdown li{margin:5px 0}
  .release-markdown pre{margin:12px 0 16px;padding:13px 14px;border:1px solid var(--rf-border,var(--border));border-radius:8px;overflow:auto;background:var(--rf-bg,var(--bg));font-family:var(--font-mono,monospace);font-size:12px}
  .release-markdown blockquote{margin:12px 0 16px;padding:8px 12px;border-left:3px solid var(--rf-accent,var(--accent));background:var(--rf-accent-soft,rgba(56,189,248,.08));color:var(--rf-muted,var(--muted))}
  .release-table-wrap{overflow:auto;margin:12px 0 18px;border:1px solid var(--rf-border,var(--border));border-radius:8px}
  .release-table-wrap table{width:100%;border-collapse:collapse;font-size:13px}
  .release-table-wrap th,.release-table-wrap td{padding:8px 10px;text-align:left;border-bottom:1px solid var(--rf-border,var(--border));white-space:nowrap}
  .release-table-wrap th{background:var(--rf-surface-2,var(--surface-2))}
  .release-table-wrap tr:last-child td{border-bottom:0}
  .release-empty,.release-error{padding:28px;color:var(--rf-muted,var(--muted))}
  .release-error{color:#ff8b8b}
  @media(max-width:900px){
    .release-layout{grid-template-columns:1fr}
    .release-list{position:static;display:flex;overflow:auto}
    .release-list-head{display:none}
    .release-list button{min-width:180px}
  }
</style>
