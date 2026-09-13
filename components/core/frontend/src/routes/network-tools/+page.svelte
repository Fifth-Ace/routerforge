<script>
  import ModuleFrame from '$lib/components/ModuleFrame.svelte';
  import { settings } from '$lib/stores/settings.js';
  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
</script>

<svelte:head><title>{locale === 'ru' ? '\u0421\u0435\u0442\u0435\u0432\u044b\u0435 \u0438\u043d\u0441\u0442\u0440\u0443\u043c\u0435\u043d\u0442\u044b' : 'Network Tools'}</title></svelte:head>

<ModuleFrame moduleId="network-tools" />
