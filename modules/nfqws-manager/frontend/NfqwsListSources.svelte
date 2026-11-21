<script>
  import { rfUi } from './rf-localization.js';
  import {nfqwsListSources} from './NfqwsListSourceModel.js';
  import {backupBytes} from './NfqwsBackupModel.js';
  let {snapshot={},onAction=()=>{}}=$props();
  let pending=$state(false);
  let view=$derived(nfqwsListSources(snapshot));
  async function run(name,id='') {
    if(pending||view.loading)return;
    pending=true;
    try{await onAction(name,id);}finally{pending=false;}
  }
</script>
<div class="blob-shell" data-native-svelte-list-sources="true">
  <div class="blob-hero">
    <div>
      <div class="blob-title-row"><h2 style="margin:0">{rfUi("Списки NFQWS Menu","NFQWS Menu lists")}</h2><span id="listSourceState" class="config-badge" class:active={view.state==='SYNCED'}>{view.loading?'LOADING':view.state}</span></div>
      <div class="muted small" style="margin-top:5px">{rfUi("Локальные списки и готовые списки из NFQWS Menu. Перед заменой RouterForge создаёт резервную копию.","Local lists and prepared lists from NFQWS Menu. RouterForge creates a backup before replacing a file.")}</div>
    </div>
    <div class="toolbar"><button id="refreshListSources" disabled={pending||view.loading} onclick={()=>run('refresh')}>{rfUi("Обновить sources","Refresh sources")}</button></div>
  </div>
  <div class="blob-summary">
    <div class="blob-stat"><span>{rfUi("Локально","Local")}</span><strong id="listSourceLocalCount">{view.localCount}</strong></div>
    <div class="blob-stat"><span>NFQWS Menu</span><strong id="listSourceUpstreamCount">{view.upstreamCount}</strong></div>
    <div class="blob-stat"><span>{rfUi("Нужно установить/обновить","Install or update required")}</span><strong id="listSourceMissingCount">{view.changedCount}</strong></div>
    <div class="blob-stat"><span>{rfUi("Используются","In use")}</span><strong id="listSourceReferencedCount">{view.referencedCount}</strong></div>
  </div>
  <div class="blob-layout">
    <article class="panel">
      <div class="panel-head"><div><h2>{rfUi("Локальные списки","Local lists")}</h2><div id="listSourceRootMeta" class="section-kicker mono">{view.root}</div></div></div>
      <div id="listSourceLocalList" class="blob-list">
        {#if !view.local.length}<div class="empty">{rfUi("Локальные .list не найдены.","No local .list files found.")}</div>{/if}
        {#each view.local as item (item.name)}
          <div class="blob-row"><div><div class="blob-name">{item.name} {#if item.protected}<span class="config-badge">CORE</span>{/if}</div>
          <div class="blob-meta">{backupBytes(item.size)} · sha256 {String(item.sha256||'').slice(0,16)}…</div>
          <div class="blob-refs">{item.referenced ? 'refs: '+(item.references||[]).join(', ') : 'unreferenced'}</div></div>
          <div class="blob-actions"><span class="blob-state ok">LOCAL</span><button type="button" disabled={pending||view.loading} onclick={()=>run('open',item.name)}>{rfUi("Открыть","Open")}</button></div></div>
        {/each}
      </div>
    </article>
    <article class="panel">
      <div class="panel-head"><div><h2>{rfUi("Списки NFQWS Menu","NFQWS Menu lists")}</h2><div id="listSourceUpstreamMeta" class="section-kicker">{view.upstreamMeta}</div></div></div>
      <div id="listSourceUpstreamList" class="blob-deps">
        {#if view.error}
          <div class="empty bad-text" role="alert">{view.state==='RATE LIMITED'?'GitHub API rate limited':'Upstream unavailable'}: {view.error} {rfUi("— локальные списки остаются доступны.","— local lists remain available.")}</div>
        {:else if !view.upstream.length}<div class="empty">{rfUi("Upstream manifest не содержит strategies/lists/*.list.","The upstream manifest does not include strategies/lists/*.list.")}</div>
        {:else}
          {#each view.upstream as item (item.name)}
            <div class="blob-dep-row"><div><div class="blob-name">{item.name}</div>
              <div class="blob-meta">expected {String(item.sha256||'').slice(0,16)}…{#if item.installed} · local {String(item.installed.sha256||'').slice(0,16)}…{#if item.installed.protected} · protected{/if}{/if}</div></div>
              <div class="blob-actions"><span class={'blob-state '+item.installState}>{item.installState==='ok'?'VERIFIED':item.installState==='update'?'UPDATE':'MISSING'}</span>
              {#if item.installState==='ok'}<button type="button" disabled={pending||view.loading} onclick={()=>run('open',item.name)}>{rfUi("Открыть","Open")}</button>
              {:else}<button type="button" disabled={pending||view.loading||!view.ref} onclick={()=>run('install',item.name)}>{item.installState==='update'?'Заменить':'Установить'}</button>{/if}</div>
            </div>
          {/each}
        {/if}
      </div>
    </article>
  </div>
  <div class="blob-note">{rfUi("SHA256, exact ref и существующий файл проверяются до установки. Эти действия не меняют рабочий nfqws2.conf автоматически.","SHA256, the exact source revision, and the existing file are verified before installation. This action does not automatically change the active nfqws2.conf.")}</div>
</div>
