<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  $: locale = $settings.locale || 'ru';
  onMount(() => goto('/monitoring', { replaceState: true }));
</script>
<svelte:head><title>RouterForge</title></svelte:head>
<div class="page"><section class="panel"><div class="empty">{t(locale, 'redirect.redirecting')}</div></section></div>
