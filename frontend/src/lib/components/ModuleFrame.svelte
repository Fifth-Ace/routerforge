<script>
  import { settings } from '$lib/stores/settings.js';

  export let moduleId = '';
  export let view = 'overview';

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  $: src = `/api/modules/${encodeURIComponent(moduleId)}/ui/index.html?locale=${encodeURIComponent(locale)}&view=${encodeURIComponent(view)}`;
</script>

<div class="routerforge-module-frame">
  <iframe
    title={`RouterForge ${moduleId}`}
    src={src}
    loading="eager"
    referrerpolicy="same-origin"
  ></iframe>
</div>

<style>
  .routerforge-module-frame {
    width: 100%;
    min-height: calc(100vh - 11rem);
  }
  iframe {
    display: block;
    width: 100%;
    height: calc(100vh - 10.5rem);
    min-height: 720px;
    border: 0;
    background: transparent;
  }
  @media (max-width: 760px) {
    iframe { min-height: 900px; height: calc(100vh - 8rem); }
  }
</style>
