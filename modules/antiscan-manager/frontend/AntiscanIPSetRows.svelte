<script>
  let { rows = [] } = $props();
</script>
{#each rows as row, index (index)}
  <tr><td class="mono strong">{row.name}</td><td>{row.description}</td>
    <td><span class="state {row.stateClass}">{row.state}</span>
      {#if row.countUnavailable}<small class="row-note">Число записей недоступно</small>
      {:else if row.error}<small class="row-error">{row.error}</small>{/if}
    </td><td class="mono">{row.count}</td>
  </tr>
{/each}
