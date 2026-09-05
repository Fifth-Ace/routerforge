<script>
  import { page } from '$app/stores';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  const items = [
    { href: '/dns', key: 'dns.nav.overview' },
    { href: '/dns/servers', key: 'dns.nav.servers' },
    { href: '/dns/routing', key: 'dns.nav.routing' },
    { href: '/dns/traffic', key: 'dns.nav.traffic' },
    { href: '/dns/tools', key: 'dns.nav.tools' }
  ];

  $: locale = $settings.locale || 'ru';
  $: current = $page.url.pathname;
  const active = (href) => href === '/dns' ? current === href : current === href || current.startsWith(`${href}/`);
</script>

<nav class="dns-module-nav" aria-label="RouterForge DNS">
  <span class="dns-module-nav-title mono">DNS</span>
  {#each items as item}
    <a href={item.href} class:active={active(item.href)}>{t(locale, item.key)}</a>
  {/each}
</nav>
