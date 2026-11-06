<script>
  import {onMount} from 'svelte';
  let {scope}=$props();
  let upstreams=$state([]);
  let poolID=$state('');
  let poolTransport=$state('dot');
  let poolAddress=$state('');
  let poolSNI=$state('');
  let poolPath=$state('');
  let poolFallbacks=$state('');
  let listenerMode=$state('loopback');
  let listenerAddress=$state('127.0.0.1:55353');
  let listenerACL=$state('');
  let listenerEdited=$state(false);
  let ruleID=$state('');
  let domain=$state('');
  let match=$state('suffix');
  let upstream=$state('');
  let priority=$state(100);
  const byId=id=>document.getElementById(id);
  function snapshot(event){
    const data=event?.detail||window.__rfDNSRuntimeFormsSnapshot?.();
    if(!data)return;
    upstreams=data.upstreams||[];
    if(!upstreams.some(x=>x.id===upstream))upstream=upstreams[0]?.id||'';
    if(data.listener && !listenerEdited){
      listenerMode=data.listener.mode||'loopback';
      listenerAddress=data.listener.listen||'127.0.0.1:55353';
      listenerACL=(data.listener.allowed_clients||[]).join(',');
    }
  }
  function submitLegacy(id,fields){
    const form=byId(id);
    if(!form)throw Error('DNS_RUNTIME_LEGACY_FORM_MISSING: '+id);
    for(const [name,value] of Object.entries(fields)){
      const input=form.elements.namedItem(name);
      if(!input)throw Error('DNS_RUNTIME_LEGACY_FIELD_MISSING: '+name);
      input.value=String(value);
    }
    form.requestSubmit();
  }
  function saveUpstream(event){
    event.preventDefault();
    if(poolTransport==='native')return;
    const id=poolID.trim();
    if(!id||!poolAddress.trim())return;
    if(upstreams.some(x=>x.id===id)){
      const feedback=byId('feedback');
      if(feedback)feedback.textContent='ID пула уже существует';
      return;
    }
    submitLegacy('upstreamForm',{
      id,transport:poolTransport,address:poolAddress.trim(),
      server_name:poolSNI.trim(),path:poolPath.trim(),fallbacks:poolFallbacks.trim()
    });
    poolID='';poolAddress='';poolSNI='';poolPath='';poolFallbacks='';
  }
  function selectPoolTransport(event){
    poolTransport=event.currentTarget.value;
    const legacy=byId('upstreamForm');
    if(!legacy)return;
    const select=legacy.elements.namedItem('transport');
    select.value=poolTransport;
    select.dispatchEvent(new Event('change',{bubbles:true}));
    legacy.style.display=poolTransport==='native'?'':'none';
  }
  function onLegacyTransport(event){
    if(event.target?.name!=='transport')return;
    poolTransport=event.target.value;
    byId('upstreamForm').style.display=poolTransport==='native'?'':'none';
  }
  function saveRule(event){
    event.preventDefault();
    if(!upstream)return;
    const id=ruleID.trim();
    if(!id||!domain.trim())return;
    submitLegacy('ruleForm',{id,domain:domain.trim(),match,upstream,priority});
    const message=byId('feedback')?.textContent||'';
    if(!message.includes('уже существует')){ruleID='';domain='';priority=100;}
  }
  function saveListener(event){
    event.preventDefault();
    submitLegacy('listenerForm',{mode:listenerMode,listen:listenerAddress.trim(),allowed_clients:listenerACL});
    listenerEdited=false;
  }
  onMount(()=>{
    const forms=[scope==='upstream'?'upstreamForm':scope==='rule'?'ruleForm':'listenerForm'].map(byId);
    if(forms.some(f=>!f))throw Error('DNS_RUNTIME_LEGACY_FORMS_MISSING');
    const old=forms.map(f=>f.style.display);
    forms.forEach(f=>{f.style.display='none';});
    if(scope==='upstream')forms[0].addEventListener('change',onLegacyTransport);
    window.__rfDNSRuntimeSvelteFormsLoaded=true;
    window.addEventListener('routerforge:dns-runtime-form-state',snapshot);
    snapshot();
    return ()=>{
      window.removeEventListener('routerforge:dns-runtime-form-state',snapshot);
      if(scope==='upstream')forms[0].removeEventListener('change',onLegacyTransport);
      forms.forEach((f,i)=>{f.style.display=old[i];});
      window.__rfDNSRuntimeSvelteFormsLoaded=false;
    };
  });
</script>

{#if scope==='upstream'}
<div class="form-grid fields-upstream" data-native-svelte-upstream-form="true">
  <label><span>Протокол</span>
    <select value={poolTransport} onchange={selectPoolTransport}>
      <option value="dot">DoT (TLS)</option>
      <option value="doh">DoH (HTTPS)</option>
      <option value="doq">DoQ (QUIC)</option>
      <option value="native">Keenetic Native — штатный редактор</option>
    </select>
  </label>
  {#if poolTransport==='native'}
    <p class="boundary-copy span-all">Для Keenetic Native используется штатный селектор System / PolicyN ниже. Он не заменяется и работает только по явному действию.</p>
  {:else}
    <form class="form-grid span-all" onsubmit={saveUpstream}>
      <label><span>ID пула</span><input required maxlength="64" placeholder="cloudflare" bind:value={poolID}></label>
      <label><span>Адрес</span><input required placeholder="1.1.1.1:853" bind:value={poolAddress}></label>
      <label><span>TLS SNI (DoT/DoH/DoQ)</span><input placeholder="dns.example.com" bind:value={poolSNI}></label>
      <label><span>DoH path</span><input placeholder="/dns-query" bind:value={poolPath}></label>
      <label><span>Резервные ID через запятую</span><input placeholder="secondary, backup" bind:value={poolFallbacks}></label>
      <div class="form-actions"><button class="button primary" type="submit">+ Добавить пул</button></div>
    </form>
  {/if}
</div>
{:else if scope==='rule'}
{#if !upstreams.length}
<p class="boundary-copy">Для доменных правил сначала добавь DNS-пул.</p>
{/if}
<form class="form-grid fields-rule" data-native-svelte-rule-form="true" onsubmit={saveRule}>
  <label><span>ID правила</span><input required maxlength="64" placeholder="main-site" bind:value={ruleID}></label>
  <label><span>Домен</span><input required placeholder="example.com" bind:value={domain}></label>
  <label><span>Совпадение</span><select bind:value={match}><option value="suffix">Домен и поддомены</option><option value="exact">Только домен</option><option value="subdomains">Только поддомены</option></select></label>
  <label><span>DNS-пул</span><select required bind:value={upstream}>{#each upstreams as item (item.id)}<option value={item.id}>{item.id}</option>{/each}</select></label>
  <label><span>Приоритет</span><input type="number" min="0" max="1000000" bind:value={priority}></label>
  <div class="form-actions"><button class="button primary" type="submit" disabled={!upstreams.length}>+ Добавить правило</button></div>
</form>

{:else}
<form class="form-grid" data-native-svelte-listener-form="true" onsubmit={saveListener}>
  <label><span>Режим</span><select bind:value={listenerMode} onchange={()=>{listenerEdited=true;}}><option value="loopback">Только loopback</option><option value="lan">Приватный IPv4 LAN + ACL</option></select></label>
  <label><span>Адрес и порт (не 53)</span><input required placeholder="127.0.0.1:55353" bind:value={listenerAddress} oninput={()=>{listenerEdited=true;}}></label>
  <label class="span-all"><span>Разрешённые клиенты для LAN, через запятую</span><input placeholder="192.168.10.1,127.0.0.1" bind:value={listenerACL} oninput={()=>{listenerEdited=true;}}></label>
  <div class="form-actions"><button class="button primary" type="submit">Добавить в черновик</button></div>
</form>
{/if}
