<script>
  import { rfUi } from './rf-localization.js';
  import {backupBytes} from './NfqwsBackupModel.js';
  import {blobSummary} from './NfqwsCatalogBlobModel.js';
  let {snapshot={},onAction=()=>{}}=$props();
  let pending=$state(false);
  let files=$derived(Array.isArray(snapshot.files)?snapshot.files:[]);
  let deps=$derived(Array.isArray(snapshot.dependencies)?snapshot.dependencies:[]);
  let busy=$derived(pending||snapshot.busy||snapshot.loading);
  let stats=$derived(blobSummary(files,deps));
  async function run(name,id=''){
    if(busy)return;
    pending=true;
    try{await onAction(name,id);}finally{pending=false;}
  }
</script>
<div class="blob-shell" data-svelte-owned="nfqws-blobs">
  <div class="blob-hero"><div><div class="blob-title-row"><h2 style="margin:0">{rfUi("TLS-заготовки","TLS templates")}</h2><span id="blobRuntimeBadge" class="config-badge">{rfUi("ФАЙЛЫ","FILES")}</span></div><div class="muted small" style="margin-top:5px">{rfUi("Локальные заготовки и файлы, необходимые выбранной стратегии. Перед установкой RouterForge проверяет их целостность.","Local templates and files required by the selected strategy. RouterForge verifies their integrity before installation.")}</div></div><div class="toolbar"><button id="refreshBlobs" disabled={busy} onclick={()=>run('refresh')}>{rfUi("Обновить inventory","Refresh inventory")}</button></div></div>
  <div class="blob-summary"><div class="blob-stat"><span>{rfUi("Установлено","Installed")}</span><strong id="blobInstalledCount">{stats.installed}</strong></div><div class="blob-stat"><span>{rfUi("Требуется","Required")}</span><strong id="blobRequiredCount">{stats.required}</strong></div><div class="blob-stat"><span>Missing / update</span><strong id="blobMissingCount">{stats.missing}</strong></div><div class="blob-stat"><span>Referenced</span><strong id="blobReferencedCount">{stats.referenced}</strong></div></div>
  <div class="blob-layout"><article class="panel"><div class="panel-head"><div><h2>{rfUi("Локальные blobs","Local blobs")}</h2><div id="blobRootMeta" class="section-kicker mono">{snapshot.root||'/opt/etc/nfqws2/blobs'}</div></div></div>
    <div id="blobInventoryList" class="blob-list">
      {#if snapshot.error}<div class="empty bad-text" role="alert">{snapshot.error}</div>
      {:else if !files.length}<div class="empty">{snapshot.loading?'Загружаем inventory…':'Blob-файлы не найдены.'}</div>{/if}
      {#each files as file (file.name)}<div class="blob-row"><div><div class="blob-name">{file.name}</div><div class="blob-meta">{backupBytes(file.size)} · sha256 {String(file.sha256||'').slice(0,16)}…</div><div class="blob-refs">{file.referenced?'refs: '+(file.references||[]).join(', '):'unreferenced'}</div></div><div class="blob-actions"><span class="blob-state ok">INSTALLED</span><button type="button" disabled={busy} onclick={()=>run('validate',file.name)}>ClientHello?</button><button type="button" class="danger" disabled={busy||file.referenced} title={file.referenced?'Referenced blob cannot be deleted':''} onclick={()=>run('delete',file.name)}>{rfUi("Удалить","Delete")}</button></div></div>{/each}
    </div></article>
    <article class="panel"><div class="panel-head"><div><h2>{rfUi("Зависимости Catalog","Catalog dependencies")}</h2><div id="blobCatalogMeta" class="section-kicker">{snapshot.selectedCatalog?`${snapshot.selectedCatalog.name} · ${snapshot.selectedCatalog.kind==='upstream'?'exact ref '+String(snapshot.selectedCatalog.ref||'').slice(0,12):snapshot.selectedCatalog.source}`:'Выберите config в Catalog'}</div></div><button id="openBlobCatalog" disabled={busy} onclick={()=>run('catalog')}>{rfUi("Открыть Catalog","Open Catalog")}</button></div>
      <div id="blobDependencyList" class="blob-deps">
        {#if !snapshot.selectedCatalog}<div class="empty">{rfUi("Выберите стратегию в Catalog, чтобы проверить её blobs.","Choose a strategy in Catalog to inspect its blobs.")}</div>
        {:else if !deps.length}<div class="empty">{rfUi("У выбранного config явные --blob dependencies не найдены.","No explicit --blob dependencies were found in the selected configuration.")}</div>{/if}
        {#each deps as item (item.name)}<div class="blob-dep-row"><div><div class="blob-name">{item.name||item.dep.name}</div><div class="blob-meta">symbol={item.dep.name}{item.expected?' · expected '+item.expected.sha256.slice(0,16)+'…':' · upstream hash unavailable'}{item.installed?' · local '+String(item.installed.sha256||'').slice(0,16)+'…':''}</div></div><div class="blob-actions"><span class={'blob-state '+item.state}>{item.state==='ok'?'VERIFIED':item.state==='update'?'UPDATE':'MISSING'}</span>{#if item.state!=='ok'}<button type="button" disabled={busy||item.item.kind!=='upstream'||!item.expected} onclick={()=>run('install',item.name)}>{item.state==='update'?'Заменить':'Установить'}</button>{/if}</div></div>{/each}
      </div>
    </article></div>
  <div class="blob-note"><b>{rfUi("Ограничения изменений:","Change boundaries:")}</b> {rfUi("установка/замена пишет только blob-файл через atomic write и safety backup. RouterForge","installation or replacement writes only the blob file with an atomic write and safety backup. RouterForge")} <b>{rfUi("не","does not")}</b> {rfUi("выполняет restart/reload и не утверждает, что running nfqws уже использует новый файл. Удаление запрещено, пока blob упоминается active или stored config. Координированное применение config + dependencies + verify/rollback остаётся за безопасным применением настроек.","restarts or reloads the service, and does not claim the running nfqws process already uses the new file. Deletion is blocked while the blob is referenced by active or stored configurations. Coordinated application, dependency verification, and rollback remain part of Smart Apply.")}</div>
</div>
