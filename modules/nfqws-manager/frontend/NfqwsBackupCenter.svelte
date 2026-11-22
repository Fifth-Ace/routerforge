<script>
  import {backupBytes,backupProjection} from './NfqwsBackupModel.js';
  let {snapshot={}, onAction=()=>{}} = $props();
  let search = $state('');
  let pending = $state(false);
  let data = $derived(backupProjection(snapshot,search));
  async function invoke(action,id='') {
    if (pending || data.loading) return;
    pending=true;
    try { await onAction(action,id); }
    finally { pending=false; }
  }
</script>
<div class="backup-center-shell" data-native-svelte-backups="true">
  <div class="backup-center-hero">
    <div>
      <div class="backup-center-title-row"><h2 style="margin:0">Backup Center</h2><span class="config-badge active">PERSISTENT</span><span id="backupCenterState" class="config-badge" class:active={!data.loading&&!data.error&&data.total>0}>{data.error?'ERROR':data.loading?'LOADING':data.total?'READY':'EMPTY'}</span></div>
      <div class="muted small" style="margin-top:5px">Постоянные snapshots до config/list/blob/library mutation. Inventory, diff и restore живут на роутере и переживают reboot.</div>
      <div id="backupCenterRoot" class="section-kicker mono" style="margin-top:7px">{data.root}</div>
    </div>
    <div class="toolbar"><button id="refreshBackups" disabled={pending||data.loading} onclick={()=>invoke('refresh')}>Обновить</button><button id="pruneBackups" class="danger" disabled={pending||data.loading} onclick={()=>invoke('prune')}>Очистить старые…</button></div>
  </div>
  <div class="backup-center-summary">
    <div class="backup-center-stat"><span>Persistent</span><strong id="backupPersistentCount">{data.total}</strong></div>
    <div class="backup-center-stat"><span>Changed / missing</span><strong id="backupChangedCount">{data.changed}</strong></div>
    <div class="backup-center-stat"><span>Restorable</span><strong id="backupRestorableCount">{data.restorable}</strong></div>
    <div class="backup-center-stat"><span>Temporary safety</span><strong id="backupCount">{data.safetyCount}</strong></div>
  </div>
  <div class="backup-center-layout">
    <article class="panel">
      <div class="panel-head"><div><h2>История</h2><div class="section-kicker">до 64 persistent snapshots</div></div></div>
      <input id="backupCenterSearch" aria-label="Поиск persistent backups" class="search" style="width:100%;max-width:none" placeholder="label / target / kind / sha…" bind:value={search}/>
      <div id="backupCenterList" class="backup-center-list">
        {#if data.error}<div class="empty bad-text" role="alert">{data.error}</div>
        {:else if !data.files.length}<div class="empty">{data.loading?'Загружаем persistent inventory…':'Persistent backups не найдены.'}</div>
        {:else}
          {#each data.files as item (item.id)}
            <button type="button" class="backup-center-item" class:selected={data.selected===item.id} disabled={pending||data.loading} aria-pressed={data.selected===item.id} onclick={()=>invoke('select',item.id)}>
              <div><div class="backup-center-name">{item.label}</div><div class="backup-center-meta">{String(item.kind||'').toUpperCase()} · {item.created_at||'—'} · {item.target||'unbound'} · {backupBytes(item.size)} · {String(item.sha256||'').slice(0,12)}…</div></div>
              <span class="backup-center-state" class:changed={item.state==='CHANGED'}>{item.state||'UNKNOWN'}</span>
            </button>
          {/each}
        {/if}
      </div>
    </article>
    <article class="panel">
      <div id="backupCenterDetail" class:backup-center-detail={!!data.entry} class:backup-center-detail-empty={!data.entry}>
        {#if data.detailError}<div class="bad-text" role="alert">{data.detailError}</div>
        {:else if data.detailLoading}Загрузка backup и текущего target…
        {:else if !data.entry}Выберите backup слева. Restore сначала создаёт новый persistent snapshot текущего target и только потом меняет файл.
        {:else}
          <div class="backup-center-detail-head">
            <div><div class="backup-center-detail-name">{data.entry.label||data.entry.id}</div><div class="backup-center-detail-meta">id={data.entry.id||'—'} · target={data.entry.target||'—'}</div></div>
            <div class="toolbar"><span class="backup-center-state">{data.entry.state||'—'}</span><button id="restoreSelectedBackup" class="primary" disabled={pending||data.loading||!data.entry.restorable} onclick={()=>invoke('restore')}>Restore</button><button id="deleteSelectedBackup" class="danger" disabled={pending||data.loading} onclick={()=>invoke('delete')}>Удалить backup</button></div>
          </div>
          <div class="backup-center-kv"><div>Kind</div><div>{data.entry.kind||'—'}</div><div>Created</div><div>{data.entry.created_at||'—'}</div><div>Backup SHA256</div><div class="mono">{data.entry.sha256||'—'}</div><div>Current SHA256</div><div class="mono">{data.entry.current_sha256||'—'}</div><div>Size / mode</div><div>{backupBytes(data.entry.size)} · {Number(data.entry.mode||0).toString(8)}</div></div>
          {#if data.isText}
            <div><div class="section-kicker" style="margin-bottom:6px">Current → backup diff</div><pre class="backup-center-diff">{#each data.diff as line,i (i)}<span class={line.c}>{line.t}</span>{'\n'}{/each}{#if !data.diff.length}No textual differences{/if}</pre></div>
            <div><div class="section-kicker" style="margin-bottom:6px">Backup content</div><pre class="backup-center-preview">{snapshot.detail.content}</pre></div>
          {:else}
            <div class="section-kicker" style="margin-bottom:6px">Binary backup</div><div class="backup-center-preview">Binary payload · {backupBytes(data.entry.size)} · SHA256 {data.entry.sha256||'—'}</div>
          {/if}
        {/if}
      </div>
    </article>
  </div>
  <div class="backup-center-note"><b>Safety boundary:</b> config restore → verify SHA → controlled restart только если nfqws2 был запущен → verify running; при первой ошибке возвращается предыдущий файл и runtime. List/blob/library restore не перезапускает сервис. Старые временные safety-копии в <span class="mono">/tmp</span> остаются rollback-слоем и по-прежнему ограничены 8 файлами.</div>
</div>
