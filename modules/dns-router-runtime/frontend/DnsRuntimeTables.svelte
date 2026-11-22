<script>
  import {draft,removeRow,editRow,feedback} from './DnsRuntimeStore.svelte.js';
  let {target}=$props();
  let upstream=$derived(target==='upstreamList');
  let columns=$derived(upstream?['ID','Протокол','Адрес','SNI','DoH path','Резерв']:['ID','Домен','Совпадение','Пул','Приоритет']);
  let items=$derived(upstream?(draft.config.upstreams||[]):(draft.config.rules||[]));
  let rows=$derived(items.map(item=>upstream?
    [item.id,item.transport,item.address,item.server_name||'—',item.path||'—',(item.fallbacks||[]).join(', ')||'—']:
    [item.id,item.domain,item.match,item.upstream,item.priority]));
  let query=$derived(draft.ruleSearch.trim().toLowerCase());
  let error=$state('');
  function remove(index){
    error='';try{removeRow(upstream?'upstream':'rule',index);}
    catch(e){error=String(e?.message||e);feedback(error);}
  }
  function edit(index){
    error='';try{editRow(upstream?'upstream':'rule',items[index]);
      document.getElementById('rf-dns-runtime-svelte-'+(upstream?'upstream':'rule')+'-form')?.scrollIntoView({behavior:'smooth',block:'start'});
    }catch(e){error=String(e?.message||e);feedback(error);}
  }
</script>

{#if error}<p role="alert" class="notice">{error}</p>{/if}
<table>
  <thead><tr>{#each columns as column,i (i)}<th>{column}</th>{/each}<th>Действие</th></tr></thead>
  <tbody>
    {#each rows as values,index (index)}
      <tr hidden={!!query && !values.some(v=>String(v??'').toLowerCase().includes(query))} style:display={!!query && !values.some(v=>String(v??'').toLowerCase().includes(query))?'none':''}>
        {#each values as value,j (j)}<td>{String(value ?? '—')}</td>{/each}
        <td><button type="button" class="button dr-p5-edit" onclick={()=>edit(index)}>Изменить</button> <button type="button" onclick={()=>remove(index)}>Удалить</button></td>
      </tr>
    {/each}
  </tbody>
</table>
