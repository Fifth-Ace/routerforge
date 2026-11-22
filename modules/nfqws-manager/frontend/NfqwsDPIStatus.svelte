<script>
  import {dpiStatusProjection} from './NfqwsDPIStatusModel.js';
  let {snapshot=null,loading=false,error='',mode='summary'}=$props();
  const view=$derived(dpiStatusProjection(snapshot,loading,error));
</script>
{#if mode==='summary'}
  <div id="dpiModuleSummary" class="dpi-runtime-strip" data-svelte-owned="nfqws-dpi-status">
    {#each view.summary as item,i (i)}
      <span class={`dpi-runtime-pill ${item.kind}`}>{item.text}</span>
    {/each}
  </div>
{:else if mode==='keenetic'}
  <div id="dpiDetectorStatus" class="diagnostics" data-svelte-owned="nfqws-dpi-keenetic-status">
    {#each view.items as item,i (i)}
      <div class={`diagnostic ${item.kind}`}><div class="diagnostic-icon">{item.kind==='good'?'✓':item.kind==='info'?'i':'!'}</div><div><div class="diagnostic-title">{item.title}</div><div class="diagnostic-detail">{item.detail}</div></div></div>
    {:else}
      <div class="empty">Открой вкладку — проверим только наличие локального dpi-detector.</div>
    {/each}
  </div>
{/if}
