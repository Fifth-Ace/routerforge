<script>
  import { rfUi } from './rf-localization.js';
  import {onMount,untrack} from 'svelte';
  import {draft,readDraft,commitDraft,feedback,formsSnapshot} from './DnsRuntimeStore.svelte.js';
  import {createDraftEditor} from './DnsRuntimeDraftEditorLogic.js';
  import DnsRuntimeNative from './DnsRuntimeNative.svelte';
  let {scope}=$props();
  let upstreams=$state([]);
  let editingId=$state('');
  let poolID=$state('');
  let poolTransport=$state('dot');
  let poolAddress=$state('');
  let poolSNI=$state('');
  let poolPath=$state('');
  let poolFallbacks=$state('');
  let ruleID=$state('');
  let domain=$state('');
  let match=$state('suffix');
  let upstream=$state('');
  let priority=$state(100);
  let listenerMode=$state('loopback');
  let listenerAddress=$state('127.0.0.1:55353');
  let listenerACL=$state('');
  let listenerEdited=$state(false);
  let error=$state('');
  let stopped=false;
  const byId=id=>document.getElementById(id);
  const editor=createDraftEditor({
    readDraft,commit:commitDraft,notify:feedback
  });
  function apply(kind,fields,id=''){
    if(stopped)return false;
    error='';
    try{editor.apply(kind,fields,id);return true;}
    catch(e){error=String(e?.message||e);feedback(error);return false;}
  }
  function snapshot(event){
    const data=event?.detail||formsSnapshot();
    if(!data)return;
    upstreams=Array.isArray(data.upstreams)?data.upstreams:[];
    if(!upstreams.some(x=>x.id===upstream))upstream=upstreams[0]?.id||'';
    if(data.listener && !listenerEdited){
      listenerMode=data.listener.mode||'loopback';
      listenerAddress=data.listener.listen||'127.0.0.1:55353';
      listenerACL=(data.listener.allowed_clients||[]).join(',');
    }
  }
  function changeTransport(event){
    const next=event.currentTarget.value;
    if(next!==poolTransport){poolAddress='';poolSNI='';poolPath='';poolFallbacks='';}
    poolTransport=next;
  }
  function resetEditor(){
    editingId='';error='';
    if(scope==='upstream'){
      poolID='';poolTransport='dot';poolAddress='';poolSNI='';poolPath='';poolFallbacks='';
    }else if(scope==='rule'){
      ruleID='';domain='';match='suffix';priority=100;
    }
  }
  function onEdit(event){
    const data=event?.detail;
    if(!data||data.scope!==scope||!data.item)return;
    const item=data.item;
    editingId=String(item.id||'');error='';
    if(scope==='upstream'){
      poolID=editingId;poolTransport=item.transport||'dot';poolAddress=item.address||'';
      poolSNI=item.server_name||'';poolPath=item.path||'';
      poolFallbacks=(item.fallbacks||[]).join(', ');
    }else if(scope==='rule'){
      ruleID=editingId;domain=item.domain||'';match=item.match||'suffix';
      upstream=item.upstream||'';priority=item.priority??100;
    }
  }
  function saveUpstream(event){
    event.preventDefault();
    const success=apply('upstream',{
      id:poolID,transport:poolTransport,address:poolAddress,
      server_name:poolSNI,path:poolPath,fallbacks:poolFallbacks
    },editingId);
    if(success)resetEditor();
  }
  function saveRule(event){
    event.preventDefault();
    const success=apply('rule',{id:ruleID,domain,match,upstream,priority},editingId);
    if(success)resetEditor();
  }
  function saveListener(event){
    event.preventDefault();
    if(apply('listener',{mode:listenerMode,listen:listenerAddress,allowed_clients:listenerACL}))listenerEdited=false;
  }
  $effect(()=>{void draft.generation;untrack(snapshot);});
  $effect(()=>{const action=draft.edit;if(action.scope===scope)untrack(()=>onEdit({detail:action}));});
  onMount(()=>()=>{stopped=true;});
</script>

{#if scope==='upstream'}
<div class="form-grid fields-upstream" data-native-svelte-upstream-form="true">
  <label><span>{rfUi("Протокол","Protocol")}</span>
    <select value={poolTransport} onchange={changeTransport}>
      <option value="dot">DoT (TLS)</option>
      <option value="doh">DoH (HTTPS)</option>
      <option value="doq">DoQ (QUIC)</option>
      <option value="udp">{rfUi("DNS UDP (без шифрования)","DNS UDP (unencrypted)")}</option>
      <option value="tcp">{rfUi("DNS TCP (без шифрования)","DNS TCP (unencrypted)")}</option>
      <option value="native">Keenetic Native — System / PolicyN</option>
    </select>
  </label>
  {#if poolTransport==='native'}
    <DnsRuntimeNative bind:value={poolAddress} />
    <form class="form-grid span-all" onsubmit={saveUpstream}>
      <label><span>ID Native DNS</span><input required maxlength="64" readonly={!!editingId} bind:value={poolID} placeholder="system-main"></label>
      <label><span>{rfUi("Выбранный селектор","Selected DNS selector")}</span><input required readonly bind:value={poolAddress} placeholder={rfUi("Выбери System / PolicyN выше","Select System / PolicyN above")}></label>
      <div class="form-actions"><button class="button primary" type="submit" disabled={!poolAddress}>{editingId?'Сохранить Native DNS':'Добавить Native DNS в черновик'}</button>{#if editingId}<button type="button" onclick={resetEditor}>{rfUi("Отменить","Cancel")}</button>{/if}</div>
    </form>
  {:else}
    <form class="form-grid span-all" onsubmit={saveUpstream}>
      <label><span>{rfUi("ID пула","Pool ID")}</span><input required maxlength="64" readonly={!!editingId} placeholder="cloudflare" bind:value={poolID}></label>
      <label><span>{rfUi("Адрес","Address")}</span><input required placeholder="1.1.1.1:853" bind:value={poolAddress}></label>
      <label><span>TLS SNI (DoT/DoH/DoQ)</span><input placeholder="dns.example.com" bind:value={poolSNI}></label>
      <label><span>DoH path</span><input placeholder="/dns-query" bind:value={poolPath}></label>
      <label><span>{rfUi("Резервные ID через запятую","Fallback IDs, comma-separated")}</span><input placeholder="secondary, backup" bind:value={poolFallbacks}></label>
      <div class="form-actions"><button class="button primary" type="submit">{editingId?'Сохранить DNS-пул':'+ Добавить пул'}</button>{#if editingId}<button type="button" onclick={resetEditor}>{rfUi("Отменить","Cancel")}</button>{/if}</div>
    </form>
  {/if}
</div>
{:else if scope==='rule'}
  {#if !upstreams.length}<p class="boundary-copy">{rfUi("Для доменных правил сначала добавь DNS-пул.","Add a DNS pool before defining domain rules.")}</p>{/if}
  <form class="form-grid fields-rule" data-native-svelte-rule-form="true" onsubmit={saveRule}>
    <label><span>{rfUi("ID правила","Rule ID")}</span><input required maxlength="64" readonly={!!editingId} placeholder="main-site" bind:value={ruleID}></label>
    <label><span>{rfUi("Домен","Domain")}</span><input required placeholder="example.com" bind:value={domain}></label>
    <label><span>{rfUi("Совпадение","Match type")}</span><select bind:value={match}><option value="suffix">{rfUi("Домен и поддомены","Domain and subdomains")}</option><option value="exact">{rfUi("Только домен","Exact domain only")}</option><option value="subdomains">{rfUi("Только поддомены","Subdomains only")}</option></select></label>
    <label><span>{rfUi("DNS-пул","DNS pool")}</span><select required bind:value={upstream}>{#each upstreams as item (item.id)}<option value={item.id}>{item.id}</option>{/each}</select></label>
    <label><span>{rfUi("Приоритет","Priority")}</span><input type="number" min="0" max="1000000" bind:value={priority}></label>
    <div class="form-actions"><button class="button primary" type="submit" disabled={!upstreams.length}>{editingId?'Сохранить правило':'+ Добавить правило'}</button>{#if editingId}<button type="button" onclick={resetEditor}>{rfUi("Отменить","Cancel")}</button>{/if}</div>
  </form>
{:else}
  <form class="form-grid" data-native-svelte-listener-form="true" onsubmit={saveListener}>
    <label><span>{rfUi("Режим","Mode")}</span><select bind:value={listenerMode} onchange={()=>{listenerEdited=true;}}><option value="loopback">{rfUi("Только loopback","Loopback only")}</option><option value="lan">{rfUi("Приватный IPv4 LAN + ACL","Private LAN IPv4 + ACL")}</option></select></label>
    <label><span>{rfUi("Адрес и порт (не 53)","Address and port (not 53)")}</span><input required placeholder="127.0.0.1:55353" bind:value={listenerAddress} oninput={()=>{listenerEdited=true;}}></label>
    <label class="span-all"><span>{rfUi("Разрешённые клиенты для LAN, через запятую","Allowed LAN clients, comma-separated")}</span><input placeholder="192.168.10.1,127.0.0.1" bind:value={listenerACL} oninput={()=>{listenerEdited=true;}}></label>
    <div class="form-actions"><button class="button primary" type="submit">{rfUi("Добавить в черновик","Add to draft")}</button></div>
  </form>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
