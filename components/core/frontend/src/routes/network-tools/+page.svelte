<script>
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';
  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
</script>

<svelte:head><title>{t(locale, 'moduleFrame.networkToolsTitle')}</title></svelte:head>

<ModuleFrame moduleId="network-tools" />
