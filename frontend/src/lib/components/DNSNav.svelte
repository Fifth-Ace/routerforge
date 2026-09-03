<script>
  import { page } from '$app/stores';

  const items = [
    { href: '/dns', label: 'Обзор' },
    { href: '/dns/servers', label: 'Серверы' },
    { href: '/dns/routing', label: 'Маршрутизация' },
    { href: '/dns/traffic', label: 'Трафик' },
    { href: '/dns/tools', label: 'Инструменты' }
  ];

  $: current = $page.url.pathname;
  const active = (href) => href === '/dns' ? current === href : current === href || current.startsWith(`${href}/`);
</script>

<nav class="dns-module-nav" aria-label="RouterForge DNS">
  <span class="dns-module-nav-title mono">DNS</span>
  {#each items as item}
    <a href={item.href} class:active={active(item.href)}>{item.label}</a>
  {/each}
</nav>
