<script>
  import { onMount } from 'svelte';
  import { settings } from '$lib/stores/settings.js';
  import { getAppLegal } from '$lib/api.js';

  let legal = { agreement_version:'', license:'MIT', as_is:true, text:'' };
  let loading = true;
  let error = '';
  $: locale = $settings.locale || 'ru';

  onMount(async () => {
    try { legal = await getAppLegal(locale); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { loading = false; }
  });
</script>

<svelte:head><title>RouterForge — {locale === 'ru' ? 'Пользовательское соглашение' : 'User Agreement'}</title></svelte:head>

<div class="page legal-page">
  <div class="page-head">
    <div>
      <span class="routerforge-eyebrow mono">ROUTERFORGE / LEGAL</span>
      <h1>{locale === 'ru' ? 'Пользовательское соглашение' : 'User Agreement'}</h1>
      <p>MIT · AS IS · {legal.agreement_version || '2026-09-12'}</p>
    </div>
    <a class="button" href="/apps">{locale === 'ru' ? 'Центр приложений' : 'App Center'}</a>
  </div>

  {#if loading}
    <section class="panel legal-panel">{locale === 'ru' ? 'Загрузка…' : 'Loading…'}</section>
  {:else if error}
    <section class="panel legal-panel legal-error">{error}</section>
  {:else}
    <section class="panel legal-panel"><pre>{legal.text}</pre></section>
  {/if}
</div>

<style>
  .legal-page{max-width:1100px;margin:0 auto}.legal-panel{padding:1.1rem}.legal-panel pre{margin:0;white-space:pre-wrap;font:inherit;font-size:.9rem;line-height:1.58;color:var(--rf-text,var(--text))}.legal-error{color:#ff8b8b}
</style>
