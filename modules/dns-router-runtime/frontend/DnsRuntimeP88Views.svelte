<script>
  import {draft,simple,nativeDNS,choices,groups,selectPolicy,chooseDNS,editDNS} from './DnsRuntimeStore.svelte.js';
  import {policyCards,inventoryCards,groupedCards} from './DnsRuntimeP88ViewModel.js';
  let {scope}=$props();
  let available=$derived(Boolean(simple.inventory?.available));
  let instances=$derived(available?(simple.inventory.instances||[]):[]);
  let policies=$derived(policyCards(instances,simple.catalog,simple.policy));
  let inventory=$derived(inventoryCards(nativeDNS()));
  let items=$derived(groupedCards(groups(),choices().map(x=>x.value),simple.editingId,simple.policy));
  function execute(name,id){
    if(name==='selectPolicy')selectPolicy(id);
    else if(name==='chooseDNS')chooseDNS(id,message=>window.confirm(message));
    else if(name==='editDNS')editDNS(id,message=>window.confirm(message));
    else throw Error('DNS_RUNTIME_P88_ACTION_UNKNOWN');
  }
</script>

{#if scope==='policies'}
  <div id="drSimplePolicies" class="dr-simple-policies" data-p88-native-policies="true">
    {#if !available}
      <p>Список политик пока недоступен. Нажми «Обновить».</p>
    {:else}
      {#each policies as item (item.name)}
        <button type="button" class={item.selected ? 'dr-simple-policy selected' : 'dr-simple-policy'}
                aria-pressed={String(item.selected)} onclick={()=>execute('selectPolicy',item.name)}>
          <strong>{item.title}</strong><span>{item.subtitle}</span><small>{item.route}</small>
        </button>
      {/each}
    {/if}
  </div>
{:else if scope==='inventory'}
  <div id="drSimpleDNSInventory" class="dr-p89-dns-catalog" data-p88-native-inventory="true">
    {#if !inventory.length}
      <p>В этой политике DNS endpoints не обнаружены.</p>
    {:else}
      {#each inventory as item (item.key)}
        <div class="dr-p89-dns-item">
          <strong>{item.title}</strong><span>{item.meta}</span><small>{item.details}</small>
        </div>
      {/each}
    {/if}
  </div>
{:else if scope==='rules'}
  <div id="drSimpleRuleList" class="dr-simple-rules" data-p88-native-rules="true">
    {#if !groups.length}
      <p>DNS ещё не добавлены. Выбери DNS в левой колонке и добавь список доменов.</p>
    {:else}
      {#each items as group (group.id)}
        <section class={group.editing ? 'dr-ux-dns-card dr-ux-dns-editing' : 'dr-ux-dns-card'}>
          <div class="dr-ux-dns-heading">
            <div class="dr-ux-dns-title">
              <strong>{group.title}</strong><small>{group.details}</small>
              <small class="dr-ux-dns-scope">{group.scope}</small>
            </div>
            <span class="dr-ux-dns-count">{group.count} доменов</span>
          </div>
          <div class="dr-ux-dns-actions">
            <button type="button" class="button" disabled={!group.available} onclick={()=>execute('chooseDNS',group.id)}>＋ Добавить домены</button>
            <button type="button" class="button" disabled={!group.canEdit} onclick={()=>execute('editDNS',group.id)}>Редактировать список</button>
          </div>
          <details class="dr-ux-domains">
            <summary>{group.count?'Показать список ('+group.count+')':'Список пуст'}</summary>
            <pre>{group.domains||'Доменов пока нет.'}</pre>
            {#if group.count&&group.hasExtended}
              <p>Есть расширенные правила: изменения доступны в расширенном режиме.</p>
            {/if}
          </details>
        </section>
      {/each}
    {/if}
  </div>
{/if}
