<script>
  import { settings } from '$lib/stores/settings.js';
  import ReleaseNotesBrowser from '$lib/components/ReleaseNotesBrowser.svelte';
  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
</script>
<svelte:head><title>RouterForge — {locale==='ru'?'Что нового':'What’s new'}</title></svelte:head>
<div class="rf-release-page">
  <div class="rf-release-heading"><span>ROUTERFORGE / RELEASE NOTES</span><h1>{locale==='ru'?'Что нового':'What’s new'}</h1></div>
  <section class="rf-release-view"><ReleaseNotesBrowser {locale}/></section>
</div>
<style>
  .rf-release-page{display:grid;gap:16px;min-height:calc(100vh - 140px)}
  .rf-release-heading span{font-size:11px;color:var(--rf-muted,var(--muted))}.rf-release-heading h1{margin:6px 0 0}
  .rf-release-view{height:calc(100vh - 185px);min-height:400px;border:1px solid var(--rf-border,var(--border));border-radius:12px;overflow:hidden}
  @media(max-width:760px){.rf-release-view{height:calc(100vh - 150px);min-height:320px}}
</style>
