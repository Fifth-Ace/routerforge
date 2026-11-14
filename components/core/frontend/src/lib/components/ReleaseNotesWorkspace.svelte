<script>
  import ReleaseNotesBrowser from './ReleaseNotesBrowser.svelte';
  export let locale = 'ru';
  export let onclose = () => {};
  function handleKeydown(event) { if(event.key === 'Escape') onclose(); }
</script>
<svelte:window onkeydown={handleKeydown}/>
<div class="rf-notes-overlay" role="presentation" onclick={(event)=>event.currentTarget===event.target&&onclose()}>
  <section class="rf-notes-dialog" role="dialog" aria-modal="true" aria-label={locale==='ru'?'Что нового':'What’s new'}>
    <header class="rf-notes-head">
      <div><strong>{locale==='ru'?'Что нового':'What’s new'}</strong><small>ROUTERFORGE / RELEASE NOTES</small></div>
      <button type="button" class="button" onclick={onclose}>{locale==='ru'?'Закрыть':'Close'}</button>
    </header>
    <div class="rf-notes-panel"><ReleaseNotesBrowser {locale}/></div>
  </section>
</div>
<style>
  .rf-notes-overlay{position:fixed;inset:0;z-index:1200;display:grid;place-items:center;padding:2vh 2vw;background:rgba(5,10,18,.72);backdrop-filter:blur(8px)}
  .rf-notes-dialog{width:min(96vw,1680px);height:94vh;min-height:0;display:grid;grid-template-rows:auto minmax(0,1fr);border:1px solid var(--rf-border,var(--border));border-radius:16px;background:var(--rf-card,var(--panel));box-shadow:0 24px 80px rgba(0,0,0,.45);overflow:hidden}
  .rf-notes-head{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:12px 16px;border-bottom:1px solid var(--rf-border,var(--border))}
  .rf-notes-head div{display:grid;gap:3px}.rf-notes-head strong{font-size:19px}.rf-notes-head small{font-size:11px;color:var(--rf-muted,var(--muted))}
  .rf-notes-panel{min-height:0;min-width:0}
  @media(max-width:760px){.rf-notes-overlay{padding:0}.rf-notes-dialog{width:100vw;height:100vh;border:0;border-radius:0}}
</style>
