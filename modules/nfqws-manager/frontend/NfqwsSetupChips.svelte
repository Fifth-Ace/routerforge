<script>
  let {items=[]}=$props();
</script>
<div class="setup-chips" data-svelte-owned="nfqws-setup-summary-chips">
  {#each items as item,i (i)}<span class="setup-chip" class:good={item.cls==='good'} class:warn={item.cls==='warn'}>{item.label}</span>{/each}
</div>
