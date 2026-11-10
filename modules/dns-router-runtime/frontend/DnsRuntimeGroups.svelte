<script>
  import {untrack} from 'svelte';
  import {draft,readDraft,commitDraft,feedback} from './DnsRuntimeStore.svelte.js';
  import {createPolicyManager} from './DnsRuntimePolicyLogic.js';
  let groups=$state([]),upstreams=$state([]),rules=$state([]),lists=$state([]);
  let id=$state(''),name=$state(''),servers=$state(''),encryptedOnly=$state(true);
  let error=$state('');
  const notify=feedback;
  const manager=createPolicyManager({
    readDraft,commit:commitDraft,
    postPreview:async()=>{throw Error('Group form does not preview lists');},
    confirmAction:message=>window.confirm(message),notify,createId:()=>''
  });
  function refresh(){
    const s=readDraft();
    if(!s?.config)return;
    groups=Array.isArray(s.config.dns_groups)?s.config.dns_groups:[];
    upstreams=Array.isArray(s.config.upstreams)?s.config.upstreams:[];
    rules=Array.isArray(s.config.rules)?s.config.rules:[];
    lists=Array.isArray(s.config.lists)?s.config.lists:[];
  }
  function submit(event){
    event.preventDefault();error='';
    try{
      manager.addGroup({id,name,servers,encrypted_only:encryptedOnly});
      id='';name='';servers='';encryptedOnly=true;
    }catch(e){error=String(e?.message||e);notify(error);}
  }
  function remove(group){
    error='';
    try{manager.removeGroup(group.id);}catch(e){error=String(e?.message||e);notify(error);}
  }
  $effect(()=>{void draft.generation;untrack(refresh);});
</script>
<div class="dr-p5-content" data-native-svelte-dns-groups="true">
  <div id="drGroupCards" class="dr-p5-list-rows">
    {#if !groups.length}<p class="dr-p5-empty">Групп пока нет. Добавь DNS-пулы выше, затем создай строгую группу.</p>{/if}
    {#each groups as group (group.id)}
      <div class="dr-p5-list-row">
        <div class="dr-p5-list-copy"><strong>{group.name}</strong><span>{(group.servers||[]).join(' → ')} · {group.encrypted_only?'только шифрование':'legacy/совместимость'} · fail-closed</span></div>
        <div class="dr-p5-list-actions"><button class="button danger" type="button" onclick={()=>remove(group)} disabled={rules.some(r=>r.group_id===group.id)||lists.some(l=>l.group_id===group.id)}>Удалить</button></div>
      </div>
    {/each}
  </div>
  <form id="drGroupForm" class="form-grid" onsubmit={submit}>
    <label><span>ID группы</span><input required maxlength="40" pattern="[a-zA-Z0-9_-]+" placeholder="cloudflare-only" bind:value={id}></label>
    <label><span>Название</span><input required maxlength="80" placeholder="Cloudflare · только свои DNS" bind:value={name}></label>
    <label class="span-all"><input type="checkbox" bind:checked={encryptedOnly}> Только зашифрованные DNS (DoT/DoH/DoQ); запрет UDP/TCP и неявных резервов</label>
    <label class="span-all"><span>DNS-серверы в порядке приоритета — ID через запятую</span><input id="drGroupServers" required placeholder="cloudflare-doh, cloudflare-dot" bind:value={servers}><small>Указанные DNS-пулы: {upstreams.length}. Неявные резервы не применяются.</small></label>
    <div class="form-actions"><button class="button primary" type="submit">+ Создать DNS-группу</button></div>
  </form>
  {#if error}<p class="notice bad" role="alert">{error}</p>{/if}
  <p class="dr-p6-note">Группы задают строгую цепочку DNS. Пока Keenetic не зарегистрировал доменную зону, правило не управляет запросами клиентов.</p>
</div>
