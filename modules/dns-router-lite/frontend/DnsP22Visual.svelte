<script>
  import {presentP22Route} from './DnsP22Route.js';
  // Pure reactive view of an explicit Evaluate result. No DOM observer,
  // duplicate navigation handler or implicit network operation.
  let {result=null,mac='',devices=[]}=$props();
  const english=document.documentElement.lang==='en'||new URLSearchParams(location.search).get('lang')==='en';
  let view=$derived(presentP22Route(result,mac,devices,english));
</script>
<div id="p22RouteSummary" class="p22-route-summary" role="status" aria-live="polite">
  {#if view}
    {#each view.steps as step,i}
      {#if i}<span class="p22-route-arrow">→</span>{/if}
      <span class="p22-route-chip">{step}</span>
    {/each}
    <small class="p22-route-disclaimer">{view.note}</small>
  {:else}
    <small>{english?'Select a device and domain to explain the configuration. No traffic is sent.':'Укажи устройство и домен, затем проверь маршрут. Реальный трафик не отправляется.'}</small>
  {/if}
</div>
