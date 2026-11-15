<script>
  import { settings } from '$lib/stores/settings.js';
  import ReleaseNotesBrowser from '$lib/components/ReleaseNotesBrowser.svelte';
  let locale = $derived($settings.locale === 'en' ? 'en' : 'ru');
</script>
<svelte:head><title>RouterForge — {locale==='ru'?'Что нового':'What’s new'}</title></svelte:head>
<!-- P41: use the native Core page and header layout; do not create a second shell. -->
<div class="page rf-release-page">
  <div class="page-head rf-release-heading">
    <div>
      <span class="page-kicker">ROUTERFORGE / RELEASE NOTES</span>
      <h1>{locale==='ru'?'Что нового':'What’s new'}</h1>
    </div>
  </div>
  <section class="rf-release-view"><ReleaseNotesBrowser {locale}/></section>
</div>
<style>
  /* Core owns the page padding, title scale, and header rhythm. */
  .rf-release-page {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    min-width: 0;
    gap: 0;
  }
  .rf-release-heading.page-head { margin-bottom: 14px; }
  .rf-release-view {
    min-width: 0;
    width: 100%;
    height: calc(100dvh - var(--header-height, 48px) - 170px);
    min-height: 400px;
    border: 1px solid var(--rf-border, var(--border));
    border-radius: 12px;
    overflow: hidden;
  }
  @media (max-width: 760px) {
    .rf-release-view {
      height: calc(100dvh - var(--header-height, 48px) - 154px);
      min-height: 320px;
    }
  }
</style>
