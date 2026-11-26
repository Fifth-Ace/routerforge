<script>
  import { onDestroy } from 'svelte';
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let locale = 'ru';
  export let snapshot = null;
  export let artifact = '';
  export let parentBusy = false;
  export let onRefresh = async () => {};
  const admin = '/api/modules/admin/maintenance/';
  const foundation = '/api/platform/foundation/';
  const nfqws = '/api/modules/nfqws-manager/backups';
  let tr;
  $: tr = locale === 'ru' ? ((ru,en)=>ru) : ((ru,en)=>en);
  let busy=false, alive=true, controller=null, error='', notice='', result=null;
  let nativePlan=null, policyPlan=null, nfPlan=null, nfFiles=[], nfID='';
  let mounts=[], mountID='', listing=null, archive='', key='', showKey=false;
  $: chosen=snapshot?.artifacts?.find(a=>a.id===artifact);
  $: sourceKey=(snapshot?.id||'')+':'+artifact;
  $: dnsSource=chosen?.source_path==='/opt/etc/routerforge/dns-router/config.json';
  $: policySource=chosen?.source_path==='/opt/etc/routerforge/policy-catalog-v1.json';
  $: if(nativePlan && nativePlan.source!==sourceKey)nativePlan=null;
  $: if(policyPlan && policyPlan.source!==sourceKey)policyPlan=null;
  $: validKey=/^[0-9a-fA-F]{64}$/.test(key);
  $: locked=busy||parentBusy;
  async function req(path,body,headers={}){
    const c=new AbortController();controller=c;const timeout=setTimeout(()=>c.abort(),50000);
    try{
      const r=await fetch(path,{method:body===undefined?'GET':'POST',credentials:'same-origin',cache:'no-store',redirect:'error',signal:c.signal,...(body===undefined?{}:{headers:{'Content-Type':'application/json',...headers},body:JSON.stringify(body)})});
      if(r.status===401)markUnauthorized();
      const d=await r.json();if(!r.ok){const e=new Error(d.error||d.state||`HTTP ${r.status}`);e.payload=d;throw e;}return d;
    }finally{clearTimeout(timeout);if(controller===c)controller=null;}
  }
  async function run(fn){if(locked||!alive)return;busy=true;error='';notice='';result=null;try{await fn()}catch(e){if(alive){error=String(e.message||e);result=e.payload||null;}}finally{if(alive)busy=false;}}
  async function dnsPreview(){nativePlan=null;const source=sourceKey;const q={snapshot_id:snapshot.id,artifact_id:artifact,confirm:'RECOVERY_DNS_PREVIEW'};await run(async()=>{const p=await req(admin+'recovery-dns-preview',q);if(alive&&source===sourceKey)nativePlan={source,p};});}
  async function dnsRestore(){const p=nativePlan?.p;if(!p?.can_commit||nativePlan.source!==sourceKey||!confirm(tr('Восстановить сохранённую DNS-политику? Создаётся защитный снимок. Служба не запускается и правила не применяются автоматически.','Restore the saved DNS policy? A safety snapshot is created. The service is not started and rules are not applied automatically.')))return;nativePlan=null;await run(async()=>{result=await req(admin+'recovery-dns-restore',{snapshot_id:p.snapshot_id,artifact_id:p.artifact_id,token:p.token,confirm:'RECOVERY_DNS_RESTORE'});if(!alive)return;notice=tr('Сохранённая DNS-политика проверена. Активная редакция отдельно.','Saved DNS policy verified. Active revision is separate.');await onRefresh();});}
  async function policyPreview(){policyPlan=null;const source=sourceKey;const q={snapshot_id:snapshot.id,artifact_id:artifact,confirm:'RECOVERY_POLICY_DRAFT'};await run(async()=>{const restored=await req(admin+'recovery-policy-draft',q);const current=await req(foundation+'catalog');const p=await req(foundation+'preview',restored.catalog,{'X-RF-Foundation':'1','If-Match':`"${current.current.etag}"`});if(alive&&source===sourceKey)policyPlan={source,p,catalog:restored.catalog,etag:current.current.etag};});}
  async function policyRestore(){const plan=policyPlan;if(!plan||plan.source!==sourceKey||!confirm(tr('Сохранить редакцию общего каталога из снимка? Предыдущая редакция сохранится в штатной истории. Привязки и нативные списки автоматически не синхронизируются.','Save the catalogue revision from this snapshot? The previous revision remains in native history. Bindings and native lists are not automatically synchronized.')))return;policyPlan=null;await run(async()=>{const s=await req(foundation+'save',plan.catalog,{'X-RF-Foundation':'1','If-Match':`"${plan.etag}"`,'X-RF-Preview':plan.p.candidate.etag,'X-RF-Confirm':'SAVE_DRAFT'});const current=await req(foundation+'catalog');if(current.current.etag!==s.current.etag)throw new Error('CATALOGUE_READBACK_CONFLICT');if(alive)notice=tr('Каталог сохранён; откат доступен через штатную историю редакций.','Catalogue saved; rollback uses native revision history.');});}
  async function loadNF(){nfPlan=null;await run(async()=>{const x=await req(nfqws);if(!alive)return;nfFiles=Array.isArray(x.files)?x.files:[];if(!nfFiles.some(f=>f.id===nfID))nfID='';});}
  async function nfPreview(){nfPlan=null;const id=nfID;await run(async()=>{const p=await req(nfqws+'/recovery-preview?'+new URLSearchParams({id}));if(alive&&nfID===id)nfPlan=p;});}
  async function nfRestore(){const p=nfPlan;if(!p?.can_commit||p.id!==nfID||!confirm(tr('Восстановить пользовательский список NFQWS? Защитная копия будет создана без ротации. Работающая служба может читать этот файл; автоматического рестарта нет.','Restore this NFQWS user list? A safety copy is created without pruning. A running service may read the file; no automatic restart.')))return;nfPlan=null;await run(async()=>{result=await req(nfqws+'/recovery-save',{id:p.id,token:p.token,confirm:'NFQWS_RECOVERY_LIST_SAVE'});if(!alive)return;notice=tr('Список сохранён и проверен. Активное поведение не проверено.','List saved and verified. Runtime behavior not verified.');const x=await req(nfqws);if(alive)nfFiles=x.files||[];});}
  async function mountLoad(){listing=null;archive='';await run(async()=>{const x=await req(admin+'recovery-storage');if(!alive)return;mounts=x.mounts||[];if(!mounts.some(m=>m.id===mountID))mountID='';});}
  async function mountList(){listing=null;archive='';const id=mountID;await run(async()=>{const x=await req(admin+'recovery-storage-list',{mount_id:id,confirm:'RECOVERY_STORAGE_LIST'});if(alive&&mountID===id)listing=x;});}
  function generateKey(){key=Array.from(crypto.getRandomValues(new Uint8Array(32)),b=>b.toString(16).padStart(2,'0')).join('');showKey=true;}
  async function exportSnapshot(){if(!snapshot?.id||!validKey||!mountID||!confirm(tr('Ключ сохранён отдельно? Без него импорт невозможен. Зашифрованная копия будет записана только на выбранное подключённое хранилище.','Have you saved the key separately? Import is impossible without it. The encrypted copy will be written only to the selected mounted storage.')))return;const q={snapshot_id:snapshot.id,mount_id:mountID,key,confirm:'RECOVERY_STORAGE_EXPORT'};await run(async()=>{result=await req(admin+'recovery-storage-export',q);if(alive){notice=tr('Зашифрованная копия записана и проверена.','Encrypted copy written and verified.');listing=null;archive='';}});}
  async function importSnapshot(){if(!archive||!validKey||!mountID||!confirm(tr('Импортировать копию как новый локальный снимок? Ни один рабочий файл не восстанавливается.','Import the copy as a new local snapshot? No working file is restored.')))return;const q={file:archive,mount_id:mountID,key,confirm:'RECOVERY_STORAGE_IMPORT'};await run(async()=>{result=await req(admin+'recovery-storage-import',q);if(alive){notice=tr('Локальный снимок импортирован. Настройки не изменены.','Local snapshot imported. Settings unchanged.');await onRefresh();}});}
  onDestroy(()=>{alive=false;controller?.abort();key='';});
</script>
<section class="extended panel">
  <div class="headline"><strong>{tr('Нативное восстановление и внешние копии','Native recovery and external copies')}</strong><span>{tr('E06.2 · без автоматической активации','E06.2 · no automatic activation')}</span></div>
  {#if error}<p role="alert" class="err">{error} · {tr('Не повторяйте запись вслепую. Проверьте исход операции.','Do not repeat writes blindly. Inspect the outcome.')}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if busy}<p role="status">{tr('Выполняется операция…','Working…')}</p>{/if}
  {#if result}<div class="result"><strong>{result.state||result.error||(result.ok?'OK':'RESULT')}</strong>{#if result.file}<small>{tr('Файл','File')}: <code>{result.file}</code></small>{/if}{#if result.sha256}<small>SHA256: <code>{result.sha256}</code></small>{/if}{#if result.safety_snapshot||result.safety_backup}<small>{tr('Защитная копия','Safety copy')}: <code>{result.safety_snapshot||result.safety_backup}</code></small>{/if}{#if result.snapshot?.id}<small>{tr('Новый снимок','New snapshot')}: <code>{result.snapshot.id}</code></small>{/if}</div>{/if}
  <details open={dnsSource||policySource}>
    <summary>{tr('DNS Router и объекты политик','DNS Router and policy objects')}</summary>
    <p>{tr('Выберите dns-router/config.json или policy-catalog-v1.json в локальном снимке выше. Конфигурация безопасности и привязки не восстанавливаются как сырые файлы.','Select dns-router/config.json or policy-catalog-v1.json in the local snapshot above. Security settings and bindings are not restored as raw files.')}</p>
    <div class="actions"><button class="button" disabled={locked||!dnsSource} onclick={dnsPreview}>{tr('Проверить DNS-политику','Preview DNS policy')}</button><button class="button" disabled={locked||!policySource} onclick={policyPreview}>{tr('Проверить каталог','Preview catalogue')}</button></div>
    {#if nativePlan}<div class="result"><strong>{tr('Сохранённая DNS-политика','Saved DNS policy')}</strong><small>{tr('Текущая редакция','Current revision')}: <code>{nativePlan.p.native.current_revision}</code></small><small>{tr('Редакция снимка','Snapshot revision')}: <code>{nativePlan.p.native.target_revision}</code></small><span>{tr('Серверов / правил / списков','Upstreams / rules / lists')}: {nativePlan.p.native.upstreams} / {nativePlan.p.native.rules} / {nativePlan.p.native.lists}</span><button class="button" disabled={locked||!nativePlan.p.can_commit} onclick={dnsRestore}>{tr('Восстановить сохранённую политику','Restore saved policy')}</button></div>{/if}
    {#if policyPlan}<div class="result"><strong>{tr('Изменения каталога','Catalogue changes')}</strong><span>+ {policyPlan.p.added?.length||0} · Δ {policyPlan.p.changed?.length||0} · − {policyPlan.p.removed?.length||0}</span><small>{tr('Откат через штатную историю редакций.','Rollback through native revision history.')}</small><button class="button" disabled={locked} onclick={policyRestore}>{tr('Сохранить редакцию из снимка','Save snapshot revision')}</button></div>{/if}
    <small>{tr('DNS: только ручные списки, неизменные параметры службы, без активного managed ownership и постоянных привязок.','DNS: manual lists only, unchanged service settings, no managed ownership or persistent bindings.')}</small>
  </details>
  <details>
    <summary>{tr('NFQWS · родные резервные копии','NFQWS · native backups')}</summary><button class="button" disabled={locked} onclick={loadNF}>{tr('Загрузить перечень','Load inventory')}</button>
    <label>{tr('Резервная копия','Backup')}<select bind:value={nfID} disabled={locked} onchange={()=>nfPlan=null}><option value="">—</option>{#each nfFiles as f}<option value={f.id}>{f.kind} · {f.label} · {f.created_at} · {f.state}</option>{/each}</select></label>
    <button class="button" disabled={locked||!nfID||nfFiles.find(f=>f.id===nfID)?.kind!=='list'} onclick={nfPreview}>{tr('Проверить восстановление списка','Preview list restore')}</button>
    {#if nfPlan}<div class="result"><strong>{nfPlan.target}</strong><small>SHA256 {tr('до','before')}: <code>{nfPlan.current_sha256}</code></small><small>SHA256 {tr('после','after')}: <code>{nfPlan.target_sha256}</code></small><button class="button" disabled={locked||!nfPlan.can_commit} onclick={nfRestore}>{tr('Восстановить список','Restore list')}</button></div>{/if}
    <small>{tr('Конфигурации, профили и бинарные объекты остаются в штатном Backup Center NFQWS. Защищённые списки и rf-binding-* здесь не восстанавливаются.','Configs, profiles and binary objects remain in the native NFQWS Backup Center. Protected lists and rf-binding-* are not restored here.')}</small>
  </details>
  <details>
    <summary>{tr('USB / SMB · зашифрованные копии','USB / SMB · encrypted copies')}</summary>
    <p>{tr('Только уже подключённые тома. Подключите SMB штатным мастером, затем обновите перечень. При недоступности тома запись в другое место запрещена.','Existing mounted volumes only. Connect SMB with the existing wizard, then refresh inventory. If the volume is unavailable, no fallback writes are allowed.')}</p>
    <div class="actions"><button class="button" disabled={locked} onclick={mountLoad}>{tr('Найти хранилища','Find storage')}</button><button class="button" disabled={locked||!mountID} onclick={mountList}>{tr('Проверить и показать копии','Check and list copies')}</button></div>
    <label>{tr('Хранилище','Storage')}<select bind:value={mountID} disabled={locked} onchange={()=>{listing=null;archive='';}}><option value="">—</option>{#each mounts as m}<option value={m.id}>{m.kind.toUpperCase()} · {m.point} · {m.fs}</option>{/each}</select></label>
    {#if listing}<small>{tr('Свободно / всего','Free / total')}: {Math.floor(listing.free_bytes/1048576)} / {Math.floor(listing.total_bytes/1048576)} MiB {listing.limited?'· LIMITED':''}</small><label>{tr('Копия для импорта','Copy to import')}<select bind:value={archive} disabled={locked}><option value="">—</option>{#each listing.files as f}<option value={f}>{f}</option>{/each}</select></label>{/if}
    <label>{tr('Ключ AES-256 (64 hex-символа)','AES-256 key (64 hex characters)')}{#if showKey}<input type="text" bind:value={key} autocomplete="off" spellcheck="false" disabled={locked}/>{:else}<input type="password" bind:value={key} autocomplete="off" disabled={locked}/>{/if}</label>
    <div class="actions"><button class="button" disabled={locked} onclick={generateKey}>{tr('Сгенерировать ключ','Generate key')}</button><button class="button" disabled={locked} onclick={()=>showKey=!showKey}>{showKey?tr('Скрыть','Hide'):tr('Показать','Show')}</button><button class="button" disabled={locked} onclick={()=>{key='';showKey=false;}}>{tr('Очистить','Clear')}</button></div>
    <small>{tr('Сохраните случайный ключ отдельно. Не используйте обычный пароль. Передача ключа идёт в текущей сессии RouterForge; для SMB-копий предпочтителен доверенный LAN или HTTPS. Ключ не сохраняется в браузере или конфигурации.','Save the random key separately. Do not use a normal password. The key travels in the current RouterForge session; prefer trusted LAN or HTTPS for SMB copies. The key is not persisted in browser or config.')}</small>
    <div class="actions"><button class="button primary" disabled={locked||!mountID||!validKey||!snapshot?.id} onclick={exportSnapshot}>{tr('Экспорт выбранного снимка','Export selected snapshot')}</button><button class="button" disabled={locked||!mountID||!validKey||!archive} onclick={importSnapshot}>{tr('Импорт в Config Vault','Import into Config Vault')}</button></div>
  </details>
</section>
<style>
 .extended{display:grid;gap:10px;padding:14px;min-width:0}.headline,.result,label{display:grid;gap:6px;min-width:0}.headline span,small{color:var(--rf-muted,#8d98a4);font-size:12px}.actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.extended details,.result{border:1px solid var(--rf-border,#29313a);border-radius:6px;padding:10px;overflow-wrap:anywhere}.extended details>*+*{margin-top:8px}.extended p{margin:0}.extended input,.extended select{width:100%;min-width:0}.result button{justify-self:start}.err{color:var(--danger,#e66)}
</style>
