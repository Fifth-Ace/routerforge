<script>
  import { catalogCategoryKey } from '$lib/catalogPresentation.js';
  export let item={};
  const shapes={
    system:'M4 5h16v14H4z M8 9l3 3-3 3 M13 15h3',
    network:'M3 12h4l3-7 4 14 3-7h4',
    dns:'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z M2 12h20 M12 2c5 5 5 15 0 20 M12 2c-5 5-5 15 0 20',
    routing:'M5 7h14l-4-4 M19 7l-4 4 M19 17H5l4-4 M5 17l4 4',
    proxy:'M7 5h10v6H7z M3 15h8v5H3z M15 15h6v5h-6z M12 11v2 M7 15v-2h11v2',
    adaptation:'M3 7h18 M3 12h18 M3 17h18 M8 5v4 M16 10v4 M11 15v4',
    automation:'M13 2 4 13h7l-1 9 10-12h-7z',
    security:'M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11z M8 12l3 3 5-6',
    other:'M4 7 12 3l8 4v10l-8 4-8-4z M4 7l8 4 8-4 M12 11v10',
    core:'M4 4h16v16H4z M9 9h6v6H9z M8 2v2 M16 2v2 M8 20v2 M16 20v2 M2 8h2 M2 16h2 M20 8h2 M20 16h2'
  };
  const official={ 'routerforge-core':'core',admin:'system',monitoring:'network',dns:'dns','dns-router':'routing','network-tools':'network','nfqws-manager':'adaptation','antiscan-manager':'security','port-access-manager':'security','crowdsec-manager':'security',profiling:'network' };
  $: category=item.kind==='module' && official[item.id] ? official[item.id] : catalogCategoryKey(item);
</script>
<svg viewBox="0 0 24 24" width="21" height="21" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={shapes[category] || shapes.other}/></svg>
