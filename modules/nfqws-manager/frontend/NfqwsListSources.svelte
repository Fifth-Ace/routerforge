<script>
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
      <div class="blob-title-row"><h2 style="margin:0">Списки NFQWS Menu</h2><span id="listSourceState" class="config-badge" class:active={view.state==='SYNCED'}>{view.loading?'LOADING':view.state}</span></div>
      <div class="muted small" style="margin-top:5px">Локальные списки и готовые списки из NFQWS Menu. Перед заменой RouterForge создаёт резервную копию.</div>
    </div>
    <div class="toolbar"><button id="refreshListSources" disabled={pending||view.loading} onclick={()=>run('refresh')}>Обновить sources</button></div>
  </div>
  <div class="blob-summary">
    <div class="blob-stat"><span>Локально</span><strong id="listSourceLocalCount">{view.localCount}</strong></div>
    <div class="blob-stat"><span>NFQWS Menu</span><strong id="listSourceUpstreamCount">{view.upstreamCount}</strong></div>
    <div class="blob-stat"><span>Нужно установить/обновить</span><strong id="listSourceMissingCount">{view.changedCount}</strong></div>
    <div class="blob-stat"><span>Используются</span><strong id="listSourceReferencedCount">{view.referencedCount}</strong></div>
  </div>
  <div class="blob-layout">
    <article class="panel">
      <div class="panel-head"><div><h2>Локальные списки</h2><div id="listSourceRootMeta" class="section-kicker mono">{view.root}</div></div></div>
      <div id="listSourceLocalList" class="blob-list">
        {#if !view.local.length}<div class="empty">Локальные .list не найдены.</div>{/if}
        {#each view.local as item (item.name)}
          <div class="blob-row"><div><div class="blob-name">{item.name} {#if item.protected}<span class="config-badge">CORE</span>{/if}</div>
          <div class="blob-meta">{backupBytes(item.size)} · sha256 {String(item.sha256||'').slice(0,16)}…</div>
          <div class="blob-refs">{item.referenced ? 'refs: '+(item.references||[]).join(', ') : 'unreferenced'}</div></div>
          <div class="blob-actions"><span class="blob-state ok">LOCAL</span><button type="button" disabled={pending||view.loading} onclick={()=>run('open',item.name)}>Открыть</button></div></div>
        {/each}
      </div>
    </article>
    <article class="panel">
      <div class="panel-head"><div><h2>Списки NFQWS Menu</h2><div id="listSourceUpstreamMeta" class="section-kicker">{view.upstreamMeta}</div></div></div>
      <div id="listSourceUpstreamList" class="blob-deps">
        {#if view.error}
          <div class="empty bad-text" role="alert">{view.state==='RATE LIMITED'?'GitHub API rate limited':'Upstream unavailable'}: {view.error} — локальные списки остаются доступны.</div>
        {:else if !view.upstream.length}<div class="empty">Upstream manifest не содержит strategies/lists/*.list.</div>
        {:else}
          {#each view.upstream as item (item.name)}
            <div class="blob-dep-row"><div><div class="blob-name">{item.name}</div>
              <div class="blob-meta">expected {String(item.sha256||'').slice(0,16)}…{#if item.installed} · local {String(item.installed.sha256||'').slice(0,16)}…{#if item.installed.protected} · protected{/if}{/if}</div></div>
              <div class="blob-actions"><span class={'blob-state '+item.installState}>{item.installState==='ok'?'VERIFIED':item.installState==='update'?'UPDATE':'MISSING'}</span>
              {#if item.installState==='ok'}<button type="button" disabled={pending||view.loading} onclick={()=>run('open',item.name)}>Открыть</button>
              {:else}<button type="button" disabled={pending||view.loading||!view.ref} onclick={()=>run('install',item.name)}>{item.installState==='update'?'Заменить':'Установить'}</button>{/if}</div>
            </div>
          {/each}
        {/if}
      </div>
    </article>
  </div>
  <div class="blob-note">SHA256, exact ref и существующий файл проверяются до установки. Эти действия не меняют рабочий nfqws2.conf автоматически.</div>
</div>
