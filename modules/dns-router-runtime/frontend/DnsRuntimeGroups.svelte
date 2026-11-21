<script>
  import { rfUi } from './rf-localization.js';
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
    {#if !groups.length}<p class="dr-p5-empty">{rfUi("Групп пока нет. Добавь DNS-пулы выше, затем создай строгую группу.","No groups yet. Add DNS pools above, then create a strict group.")}</p>{/if}
    {#each groups as group (group.id)}
      <div class="dr-p5-list-row">
        <div class="dr-p5-list-copy"><strong>{group.name}</strong><span>{(group.servers||[]).join(' → ')} · {group.encrypted_only?'только шифрование':'legacy/совместимость'} · fail-closed</span></div>
        <div class="dr-p5-list-actions"><button class="button danger" type="button" onclick={()=>remove(group)} disabled={rules.some(r=>r.group_id===group.id)||lists.some(l=>l.group_id===group.id)}>{rfUi("Удалить","Delete")}</button></div>
      </div>
    {/each}
  </div>
  <form id="drGroupForm" class="form-grid" onsubmit={submit}>
    <label><span>{rfUi("ID группы","Group ID")}</span><input required maxlength="40" pattern="[a-zA-Z0-9_-]+" placeholder="cloudflare-only" bind:value={id}></label>
    <label><span>{rfUi("Название","Name")}</span><input required maxlength="80" placeholder={rfUi("Cloudflare · только свои DNS","Cloudflare · custom DNS only")} bind:value={name}></label>
    <label class="span-all"><input type="checkbox" bind:checked={encryptedOnly}> {rfUi("Только зашифрованные DNS (DoT/DoH/DoQ); запрет UDP/TCP и неявных резервов","Encrypted DNS only (DoT/DoH/DoQ); no UDP/TCP or implicit fallback")}</label>
    <label class="span-all"><span>{rfUi("DNS-серверы в порядке приоритета — ID через запятую","DNS servers by priority — comma-separated IDs")}</span><input id="drGroupServers" required placeholder="cloudflare-doh, cloudflare-dot" bind:value={servers}><small>{rfUi("Указанные DNS-пулы:","Selected DNS pools:")} {upstreams.length}{rfUi(". Неявные резервы не применяются.",". Implicit fallbacks are not used.")}</small></label>
    <div class="form-actions"><button class="button primary" type="submit">{rfUi("+ Создать DNS-группу","+ Create DNS group")}</button></div>
  </form>
  {#if error}<p class="notice bad" role="alert">{error}</p>{/if}
  <p class="dr-p6-note">{rfUi("Группы задают строгую цепочку DNS. Пока Keenetic не зарегистрировал доменную зону, правило не управляет запросами клиентов.","Groups define strict DNS chains. Until Keenetic registers the domain zone, the rule does not govern client requests.")}</p>
</div>
