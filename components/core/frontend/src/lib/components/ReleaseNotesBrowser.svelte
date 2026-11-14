<script>
  import { onMount } from 'svelte';
  import { parseReleaseNote } from '$lib/release-notes.js';
  import { mergeNoteIndexes } from '$lib/release-notes-feed.js';

  export let locale = 'ru';
  let mounted = false;
  let channel = 'stable';
  let releases = [];
  let selectedVersion = '';
  let format = 'summary';
  let blocks = [];
  let error = '';
  let loading = true;
  let notice = '';
  let remoteAvailable = false;
  let generation = 0;
  const text = (ru,en) => locale === 'en' ? en : ru;

  $: visible = releases.filter(item => item.channel === channel);
  $: selected = releases.find(item => item.channel === channel && item.version === selectedVersion) || null;

  async function getIndex(url) {
    const response = await fetch(url, { cache:'no-store' });
    if (!response.ok) throw Error(`index HTTP ${response.status}`);
    const payload = await response.json();
    if (!Array.isArray(payload?.releases)) throw Error('invalid release notes index');
    return payload;
  }
  function pickVersion(requested = '') {
    const candidates = releases.filter(item=>item.channel===channel);
    const preferred = candidates.find(item=>item.version===requested);
    const chosen = preferred || candidates[0];
    selectedVersion = chosen?.version || '';
    format = 'summary';
    if (selectedVersion) void loadNote(); else {loading=false;blocks=[];}
  }
  onMount(() => {
    mounted = true;
    const params = new URLSearchParams(window.location.search);
    channel = params.get('channel')==='beta' ? 'beta' : 'stable';
    const requested = params.get('version') || '';
    if (requested.includes('-beta.') && !params.has('channel')) channel='beta';
    void loadCatalog(requested);
    return () => {mounted=false;generation+=1;};
  });
  async function loadCatalog(requested = '') {
    error=''; loading=true;
    const results = await Promise.allSettled([
      getIndex('/release-notes/index.json'),
      getIndex('/api/release-notes/catalog')
    ]);
    if(!mounted) return;
    const local = results[0].status==='fulfilled' ? results[0].value : {releases:[]};
    const remote = results[1].status==='fulfilled' ? results[1].value : {releases:[]};
    remoteAvailable = Boolean(remote.releases.length);
    releases=mergeNoteIndexes(local,remote);
    if (!releases.length) error=text('История обновлений недоступна.','Release history unavailable.');
    notice=remoteAvailable ? '' : text('Показан встроенный архив. Для новых выпусков требуется доступ к GitHub.','Showing bundled archive. GitHub access is required for new releases.');
    pickVersion(requested);
  }
  function changeChannel(next) {
    if(channel===next) return;
    channel=next; pickVersion();
  }
  function changeVersion(version) {
    selectedVersion=version; format='summary'; void loadNote();
  }
  function changeFormat(next) { format=next; void loadNote(); }
  async function loadNote() {
    const item = releases.find(entry => entry.channel===channel && entry.version===selectedVersion) || null;
    if(!item) {loading=false;blocks=[];return;}
    const seq=++generation;
    loading=true;error='';blocks=[];
    const available=format==='summary'?item.summary:item.story;
    if(!available) {loading=false;return;}
    const file = format==='summary'?item.summaryFile:item.storyFile;
    const fallbackFile = format==='summary'?item.localSummaryFile:item.localStoryFile;
    const remoteHasNote = item.source==='remote' && (format==='summary'?item.remoteSummary:item.remoteStory);
    const url = remoteHasNote
      ? `/api/release-notes/text?channel=${encodeURIComponent(item.channel)}&version=${encodeURIComponent(item.version)}&kind=${format}`
      : `/release-notes/${encodeURIComponent(file || fallbackFile)}`;
    try {
      let response=await fetch(url,{cache:'no-store'});
      if(!response.ok && item.source==='remote' && fallbackFile && url.startsWith('/api/')) {
        response=await fetch(`/release-notes/${encodeURIComponent(fallbackFile)}`,{cache:'no-store'});
      }
      if(!response.ok) throw Error(`HTTP ${response.status}`);
      const markdown=await response.text();
      if(!mounted || seq!==generation) return;
      blocks=parseReleaseNote(markdown);
    } catch(e) {
      if(!mounted || seq!==generation) return;
      error=text('Не удалось загрузить выбранный патчноут: ','Unable to load release notes: ')+(e?.message||String(e));
    } finally {if(mounted && seq===generation) loading=false;}
  }
</script>

<div class="rf-notes">
  <div class="rf-notes-toolbar" role="tablist" aria-label={text('Каналы обновлений','Release channels')}>
    <button type="button" role="tab" aria-selected={channel==='stable'} class:active={channel==='stable'} onclick={()=>changeChannel('stable')}>{text('Релизы','Releases')}</button>
    <button type="button" role="tab" aria-selected={channel==='beta'} class:active={channel==='beta'} onclick={()=>changeChannel('beta')}>{text('Бета-версии','Beta versions')}</button>
    <span class="rf-notes-counter">{visible.length} {text('выпусков','versions')}</span>
  </div>
  {#if notice}<div class="rf-notes-notice">{notice}</div>{/if}
  <div class="rf-notes-body">
    <nav class="rf-notes-list" aria-label={text('Версии RouterForge','RouterForge versions')}>
      {#each visible as item (`${item.channel}-${item.version}`)}
        <button type="button" class:active={selectedVersion===item.version} aria-current={selectedVersion===item.version?'true':undefined} onclick={()=>changeVersion(item.version)}>
          <strong>RouterForge {item.version}</strong>
          <small>{item.source==='remote'?text('Опубликованные заметки','Published release notes'):text('Локальный архив','Local archive')}</small>
        </button>
      {/each}
      {#if !visible.length}<div class="rf-notes-placeholder">{text('Выпусков пока нет.','No versions yet.')}</div>{/if}
    </nav>
    <section class="rf-notes-content" aria-live="polite">
      {#if selected}
        <header class="rf-notes-title">
          <div><small>ROUTERFORGE / {selected.channel.toUpperCase()}</small><strong>{selected.title}</strong></div>
          <span>v{selected.version}</span>
        </header>
        {#if channel==='stable'}
          <div class="rf-notes-formats" role="tablist" aria-label={text('Формат заметок','Release notes format')}>
            <button type="button" role="tab" aria-selected={format==='summary'} class:active={format==='summary'} onclick={()=>changeFormat('summary')}>{text('Кратко об изменениях','At a glance')}</button>
            <button type="button" role="tab" aria-selected={format==='story'} class:active={format==='story'} onclick={()=>changeFormat('story')}>{text('Графомания повышенной ненужности','Unnecessarily verbose chronicles')}</button>
          </div>
        {/if}
        {#if loading}<p class="rf-notes-placeholder">{text('Загрузка заметок…','Loading notes…')}</p>
        {:else if error}<p class="rf-notes-error">{error}</p>
        {:else if !(format==='summary'?selected.summary:selected.story)}
          <p class="rf-notes-placeholder">{format==='summary' ? text('Краткая сводка для этого выпуска пока не подготовлена. Полный авторский патчноут доступен во второй вкладке.','No short summary for this release. The full original note is on the next tab.') : text('Авторский патчноут для этого выпуска пока недоступен.','The long-form notes are not available for this release.')}</p>
        {:else}
          <article class="rf-notes-markdown">
            {#each blocks as block}
              {#if block.type==='heading'}
                {#if block.level<=1}<h2>{block.text}</h2>{:else if block.level===2}<h3>{block.text}</h3>{:else}<h4>{block.text}</h4>{/if}
              {:else if block.type==='paragraph'}<p>{block.text}</p>
              {:else if block.type==='code'}<pre><code>{block.text}</code></pre>
              {:else if block.type==='list'}<ul>{#each block.items as item}<li>{item}</li>{/each}</ul>
              {:else if block.type==='quote'}<blockquote>{block.text}</blockquote>
              {:else if block.type==='table'}
                <div class="rf-notes-table"><table><thead><tr>{#each block.headers as cell}<th>{cell}</th>{/each}</tr></thead><tbody>{#each block.rows as row}<tr>{#each row as cell}<td>{cell}</td>{/each}</tr>{/each}</tbody></table></div>
              {/if}
            {/each}
          </article>
        {/if}
      {:else}<p class="rf-notes-placeholder">{text('Выбери выпуск.','Choose a release.')}</p>{/if}
    </section>
  </div>
</div>

<style>
  .rf-notes{min-height:0;min-width:0;height:100%;display:flex;flex-direction:column;color:var(--rf-text,var(--text));background:var(--rf-card,var(--panel))}
  .rf-notes-toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:8px;padding:10px 12px;border-bottom:1px solid var(--rf-border,var(--border))}
  .rf-notes-toolbar button,.rf-notes-formats button{font:inherit;background:transparent;color:inherit;border:1px solid var(--rf-border,var(--border));border-radius:8px;padding:8px 12px;cursor:pointer;text-align:left}
  .rf-notes-toolbar button.active,.rf-notes-formats button.active{border-color:var(--rf-accent,var(--accent));background:var(--rf-accent-soft,rgba(56,189,248,.08));font-weight:650}
  .rf-notes-counter{margin-left:auto;color:var(--rf-muted,var(--muted));font-size:12px}
  .rf-notes-notice{padding:8px 14px;color:var(--rf-muted,var(--muted));font-size:12px;border-bottom:1px solid var(--rf-border,var(--border))}
  .rf-notes-body{flex:1;min-height:0;min-width:0;display:grid;grid-template-columns:minmax(220px,280px) minmax(0,1fr)}
  .rf-notes-list{overflow:auto;padding:8px;display:flex;flex-direction:column;gap:4px;background:var(--rf-surface,var(--panel));border-right:1px solid var(--rf-border,var(--border))}
  .rf-notes-list button{font:inherit;display:grid;gap:3px;text-align:left;cursor:pointer;padding:11px 12px;color:inherit;border:1px solid transparent;border-radius:8px;background:transparent}
  .rf-notes-list button.active{background:var(--rf-accent-soft,rgba(56,189,248,.08));border-color:var(--rf-accent,var(--accent))}
  .rf-notes-list button small,.rf-notes-title small{font-size:11px;color:var(--rf-muted,var(--muted))}
  .rf-notes-content{overflow:auto;min-height:0;min-width:0}
  .rf-notes-title{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:14px 18px;border-bottom:1px solid var(--rf-border,var(--border))}
  .rf-notes-title div{display:grid;gap:4px}.rf-notes-title strong{font-size:18px}.rf-notes-title span{font-family:monospace;font-size:11px;color:var(--rf-muted,var(--muted))}
  .rf-notes-formats{display:flex;gap:8px;flex-wrap:wrap;padding:12px 18px;border-bottom:1px solid var(--rf-border,var(--border))}
  .rf-notes-placeholder,.rf-notes-error{padding:22px;color:var(--rf-muted,var(--muted))}.rf-notes-error{color:#ff8b8b}
  .rf-notes-markdown{padding:18px 20px 26px;line-height:1.65}.rf-notes-markdown h2{font-size:22px;margin:0 0 16px}.rf-notes-markdown h3{font-size:18px;margin:22px 0 9px}.rf-notes-markdown h4{font-size:15px;margin:18px 0 8px}.rf-notes-markdown p{margin:0 0 12px}.rf-notes-markdown ul{padding-left:22px;margin:0 0 14px}.rf-notes-markdown li{margin:4px 0}.rf-notes-markdown pre{padding:12px;overflow:auto;border:1px solid var(--rf-border,var(--border));border-radius:8px}.rf-notes-markdown blockquote{padding:9px 13px;margin:12px 0;border-left:3px solid var(--rf-accent,var(--accent));background:var(--rf-accent-soft,rgba(56,189,248,.08))}
  .rf-notes-table{overflow-x:auto}.rf-notes-table table{border-collapse:collapse;font-size:13px}.rf-notes-table th,.rf-notes-table td{padding:8px;border:1px solid var(--rf-border,var(--border));white-space:nowrap}
  @media(max-width:760px){.rf-notes-body{display:flex;flex-direction:column}.rf-notes-list{flex-direction:row;flex:0 0 auto;max-height:92px;overflow-x:auto;border-right:0;border-bottom:1px solid var(--rf-border,var(--border))}.rf-notes-list button{min-width:180px}.rf-notes-content{flex:1}.rf-notes-counter{display:none}}
</style>
