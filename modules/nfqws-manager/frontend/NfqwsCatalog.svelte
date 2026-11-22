<script>
  import {catalogVisible} from './NfqwsCatalogBlobModel.js';
  let {snapshot={},onAction=()=>{}}=$props();
  let search=$state('');
  let filter=$state('all');
  let pending=$state(false);
  let items=$derived(Array.isArray(snapshot.items)?snapshot.items:[]);
  let selected=$derived(snapshot.detail||null);
  let visible=$derived(catalogVisible(items,filter,search));
  let model=$derived(selected?.compatibility||null);
  let deps=$derived(model?.deps||{lists:[],blobs:[],lua:[]});
  let changed=$derived((selected?.diff||[]).filter(x=>x.state!=='same').length);
  async function run(name,id=''){
    if(pending||snapshot.loading||snapshot.selecting)return;
    pending=true;
    try{await onAction(name,id);}finally{pending=false;}
  }
  const sourceLabel=item=>item.kind==='upstream'?'UPSTREAM':item.kind==='active'?'ACTIVE':'LOCAL';
</script>
<div class="catalog-shell" data-svelte-owned="nfqws-catalog">
  <div class="catalog-hero">
    <div><div class="catalog-title-row"><h2 style="margin:0">Каталог стратегий</h2><span class="config-badge active">ТОЛЬКО ПРОСМОТР</span></div>
      <div class="muted small" style="margin-top:5px">Сохранённые и готовые конфигурации NFQWS. Просмотр не меняет рабочие настройки.</div></div>
    <div class="toolbar"><button id="refreshCatalog" type="button" disabled={pending||snapshot.loading} onclick={()=>run('refresh')}>Обновить источники</button></div>
  </div>
  <div class="catalog-sources">
    <div class="catalog-source-card"><div class="catalog-source-name">Рабочая и сохранённые конфигурации</div><div id="catalogLocalSource" class="catalog-source-meta">{items.filter(x=>x.kind!=='upstream').length} items · active canonical + Config Library</div></div>
    <div class="catalog-source-card"><div class="catalog-source-name">rndnaame/nfqws-menu</div>
      <div id="catalogUpstreamSource" class="catalog-source-meta"><span class:good={snapshot.upstream?.state==='ready'} class:warn={snapshot.upstream?.state==='stale'||snapshot.upstream?.state==='loading'} class:bad={snapshot.upstream?.state==='error'} class="catalog-source-status">{String(snapshot.upstream?.state||'idle').toUpperCase()}</span> · {items.filter(x=>x.kind==='upstream').length} configs · ref {String(snapshot.upstream?.sha||snapshot.upstream?.ref||'main').slice(0,12)}{snapshot.upstream?.error?' · '+snapshot.upstream.error:''}</div>
    </div>
  </div>
  <div class="catalog-layout">
    <article class="panel catalog-browser">
      <input id="catalogSearch" aria-label="Поиск стратегии" class="search" placeholder="Стратегия / source / ref…" bind:value={search}/>
      <div class="catalog-filters"><button id="catalogAll" class:active={filter==='all'} aria-pressed={filter==='all'} onclick={()=>filter='all'}>Все</button><button id="catalogLocal" class:active={filter==='local'} aria-pressed={filter==='local'} onclick={()=>filter='local'}>Локальные</button><button id="catalogUpstream" class:active={filter==='upstream'} aria-pressed={filter==='upstream'} onclick={()=>filter='upstream'}>NFQWS Menu</button></div>
      <div id="catalogList" class="catalog-list">
        {#if !visible.length}<div class="empty">{snapshot.loading?'Загружаем каталог…':'По текущему фильтру стратегий нет.'}</div>{/if}
        {#each visible as item (item.id)}
          <button type="button" class="catalog-item" class:selected={snapshot.selected===item.id} aria-pressed={snapshot.selected===item.id} disabled={pending||snapshot.loading} onclick={()=>run('select',item.id)}>
            <div><div class="catalog-item-name">{item.name}</div><div class="catalog-item-meta">{sourceLabel(item)} · {item.ref||'local'}{item.sha256?' · '+item.sha256.slice(0,10):''}</div></div><span class="config-badge" class:active={item.kind!=='upstream'}>{sourceLabel(item)}</span>
          </button>
        {/each}
      </div>
    </article>
    <article class="panel catalog-detail">
      <div id="catalogDetail" class:catalog-empty={!selected}>
        {#if snapshot.error}<div class="bad-text" role="alert">{snapshot.error}</div>
        {:else if snapshot.selecting}<div class="empty" role="status">Загрузка preview и semantic analysis…</div>
        {:else if !selected}<div class="catalog-empty">Выберите стратегию слева. Просмотр не меняет рабочую конфигурацию.</div>
        {:else}
          <div class="catalog-detail-head"><div><div class="catalog-detail-name">{selected.item.name}</div><div class="catalog-detail-meta">source={selected.item.source} · ref={selected.item.ref||'local'} · path={selected.item.path||'—'}<br/>sha256={selected.sha256||selected.item.sha256||'—'} · integrity={String(selected.integrity||'unknown').toUpperCase()}</div></div>
            <div class="toolbar"><span class={'catalog-verdict '+String(model?.status||'').toLowerCase()}>{model?.status||'—'}</span><button id="catalogToSandbox" disabled={pending} onclick={()=>run('sandbox')}>В Sandbox · HTTPS</button><button id="catalogSmartApply" class="primary" disabled={pending||model?.status==='BLOCKED'} onclick={()=>run('smartApply')}>Smart Apply</button>{#if deps.blobs.length}<button id="catalogOpenBlobs" disabled={pending} onclick={()=>run('blobs')}>Blobs · {deps.blobs.length}</button>{/if}{#if selected.item.kind==='local'}<button id="catalogOpenLocal" disabled={pending} onclick={()=>run('local')}>Открыть в Configs</button>{/if}</div></div>
          <div class="catalog-summary"><div class="catalog-stat"><span>Diff assignments</span><strong>{changed}</strong></div><div class="catalog-stat"><span>Lists</span><strong>{deps.lists.length}{selected.missingLists?' / '+selected.missingLists+' missing':''}</strong></div><div class="catalog-stat"><span>Blobs</span><strong>{deps.blobs.length}</strong></div><div class="catalog-stat"><span>Lua</span><strong>{deps.lua.length}</strong></div></div>
          <div><div class="section-kicker" style="margin-bottom:6px">Compatibility</div><div class="catalog-checks">{#each model?.checks||[] as c,i (i)}<div class={'catalog-check '+c.severity}><span class="catalog-check-mark">{c.severity==='good'?'✓':c.severity==='info'?'i':'!'}</span><div><div class="catalog-check-title">{c.title}</div><div class="catalog-check-detail">{c.detail}</div></div></div>{/each}</div></div>
          <div><div class="section-kicker" style="margin-bottom:6px">Dependencies</div><div class="catalog-deps">{#each deps.lists as name (name)}<span class="catalog-dep">LIST {name}</span>{/each}{#each deps.blobs as blob (blob.name)}<span class="catalog-dep">BLOB {blob.name}</span>{/each}{#each deps.lua as lua (lua)}<span class="catalog-dep">LUA {lua}</span>{/each}{#if deps.tcp}<span class="catalog-dep">TCP {deps.tcp}</span>{/if}{#if deps.udp}<span class="catalog-dep">UDP {deps.udp}</span>{/if}{#if !deps.lists.length&&!deps.blobs.length&&!deps.lua.length&&!deps.tcp&&!deps.udp}<span class="muted small">Явные dependencies не найдены.</span>{/if}</div></div>
          <div><div class="section-kicker" style="margin-bottom:6px">Semantic diff vs active canonical</div><div class="catalog-diff"><div class="catalog-diff-row head"><span>Assignment</span><span>State</span><span>Active</span><span>Candidate</span></div>{#each selected.diff||[] as row (row.key)}<div class="catalog-diff-row"><b>{row.key}</b><span class={'catalog-diff-state '+row.state}>{String(row.state).toUpperCase()}</span><span class="catalog-diff-value">{row.active||'—'}</span><span class="catalog-diff-value">{row.candidate||'—'}</span></div>{/each}</div></div>
          <div><div class="section-kicker" style="margin-bottom:6px">Config preview · read-only</div><pre class="catalog-preview">{selected.content}</pre></div>
        {/if}
      </div>
    </article>
  </div>
  <div class="catalog-note">Перед использованием RouterForge проверяет целостность полученных файлов.</div>
</div>
