<script>
  import {onMount} from 'svelte';
  import {shadow, refreshShadow, controlShadow, beginShadowView} from './DnsRuntimeShadowState.svelte.js';
  import {shell} from './DnsRuntimeStore.svelte.js';
  let lite=$derived(shell.lite);
  onMount(()=>{const end=beginShadowView();void refreshShadow();return end;}); // read-only GET, never start/apply
</script>
<div class="action-bar" aria-label="Shadow DNS">
  <button type="button" class="button" id="shadowStatusButton" disabled={shadow.busy} onclick={refreshShadow}>↻ Статус</button>
  <button type="button" class="button primary" id="shadowStartButton" disabled={shadow.busy || lite} onclick={() => controlShadow('start')}>▶ Запустить</button>
  <button type="button" class="button" id="shadowApplyButton" disabled={shadow.busy || lite} onclick={() => controlShadow('apply')}>Применить правила</button>
  <button type="button" class="button danger" id="shadowStopButton" disabled={shadow.busy || lite} onclick={() => controlShadow('stop')}>■ Остановить</button>
</div>
