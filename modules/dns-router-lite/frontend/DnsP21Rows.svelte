<script>
  // P21 row rendering belongs exclusively to Svelte. The parent supplies
  // immutable view models and guarded draft-only commands.
  let {kind, rows=[], busy=false, onAction=()=>{}}=$props();
</script>
{#each rows as row (row.key)}
  <div class="p21-device-row" data-p21-row={kind} data-p21-key={row.key}>
    <div class="p21-device-copy">
      <strong>{row.title}</strong>
      <small>{row.description}</small>
    </div>
    <div class="actions">
      {#each row.actions as action (action.name)}
        <button class="button" type="button" disabled={busy} onclick={()=>onAction(kind,action.name,row.key)}>{action.label}</button>
      {/each}
    </div>
  </div>
{/each}
