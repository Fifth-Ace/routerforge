<script>
  import { rfUi } from './rf-localization.js';
  let {kind, snapshot, onChange, name='', onNameChange=()=>{}, busy=false} = $props();
  const domainModes = [
    {id:'MODE_LIST',title:rfUi('Только мой список',"My list only"),description:'user.list'},
    {id:'MODE_AUTO',title:rfUi('Автоматически',"Automatically"),description:'user.list + auto.list'},
    {id:'MODE_ALL',title:rfUi('Весь трафик',"All traffic"),description:rfUi('всё − exclude.list',"all except exclude.list")}
  ];
  const policies = [
    {id:'none',title:rfUi('Без политики',"No policy"),description:rfUi('все устройства',"all devices")},
    {id:'include',title:rfUi('Только политика',"Policy only"),description:rfUi('только POLICY_NAME',"POLICY_NAME only")},
    {id:'exclude',title:rfUi('Исключить политику',"Exclude policy"),description:rfUi('кроме POLICY_NAME',"except POLICY_NAME")}
  ];
</script>
{#if kind==='domain'}
  <div class="mode-grid" data-svelte-owned="nfqws-domain-mode">
    {#each domainModes as item (item.id)}
      <label class="mode-card" class:active={snapshot.domain===item.id} data-mode={item.id}>
        <input type="radio" name="flowMode" value={item.id} checked={snapshot.domain===item.id} disabled={busy} onchange={()=>onChange('domain',item.id)} />
        <strong>{item.title}</strong><span>{item.description}</span>
      </label>
    {/each}
  </div>
{:else if kind==='policy'}
  <div class="policy-grid" data-svelte-owned="nfqws-policy-mode">
    {#each policies as item (item.id)}
      <label class="policy-card" class:active={snapshot.policy===item.id} data-policy-mode={item.id}>
        <input type="radio" name="policyMode" value={item.id} checked={snapshot.policy===item.id} disabled={busy} onchange={()=>onChange('policy',item.id)} />
        <strong>{item.title}</strong><span>{item.description}</span>
      </label>
    {/each}
  </div>
  <div id="policyNameRow" class="form-row policy-name-compact" style:display={snapshot.policy==='none'?'none':'grid'}>
    <div class="setup-label"><strong>{rfUi("Имя политики","Policy name")}</strong>{rfUi("Точное имя политики Keenetic.","Exact Keenetic policy name.")}</div>
    <div class="form-control"><input id="flowPolicyName" class="setup-input" spellcheck="false" value={name} oninput={e=>onNameChange(e.currentTarget.value)} disabled={busy} /></div>
  </div>
{/if}
