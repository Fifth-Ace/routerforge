<script>
  import {onMount, flushSync} from 'svelte';
  let {target} = $props();
  let columns = $state([]);
  let rows = $state([]);
  let remove = null;
  let error = $state('');

  function removeRow(index) {
    if (typeof remove !== 'function') {
      error = 'Редактор DNS недоступен. Перезагрузи страницу.';
      return;
    }
    error = '';
    remove(index); // Guarded business owner validates all reference constraints.
  }

  onMount(() => {
    let mounted = true;
    const update = (event) => {
      if (!mounted || event.detail?.target !== target) return;
      flushSync(() => {
        columns = Array.isArray(event.detail.columns) ? event.detail.columns : [];
        rows = Array.isArray(event.detail.rows) ? event.detail.rows : [];
        remove = event.detail.onRemove;
      });
    };
    window.addEventListener('routerforge:dns-runtime-table', update);
    return () => {
      mounted = false;
      window.removeEventListener('routerforge:dns-runtime-table', update);
      remove = null;
    };
  });
</script>

{#if error}<p role="alert" class="notice">{error}</p>{/if}
<table>
  <thead><tr>{#each columns as column,i (i)}<th>{column}</th>{/each}<th>Действие</th></tr></thead>
  <tbody>
    {#each rows as values,index (index)}
      <tr>
        {#each values as value,j (j)}<td>{String(value ?? '—')}</td>{/each}
        <td><button type="button" onclick={() => removeRow(index)}>Удалить</button></td>
      </tr>
    {/each}
  </tbody>
</table>
