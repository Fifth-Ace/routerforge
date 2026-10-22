<script>
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import { page } from '$app/stores';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';
  $: view = $page.url.searchParams.get('tab') || 'system';
  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
</script>
<svelte:head><title>RouterForge — {t(locale, 'nav.monitoring')}</title></svelte:head>
<ModuleFrame moduleId="monitoring" {view} />