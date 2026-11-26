<script>
  import { onMount, onDestroy } from 'svelte';
  import RecoveryExtended from './RecoveryExtended.svelte';
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let locale = 'ru';
  const base = '/api/modules/admin/maintenance/';
  let vault = null, archives = [], selected = '', artifact = '', compare = '', plan = null;
  let busy = false, error = '', result = null, notice = '', alive = true, controller = null;
  let tr;
  $: tr = locale === 'ru' ? ((ru,en)=>ru) : ((ru,en)=>en);
  $: snapshot = vault?.snapshots?.find(x => x.id === selected);
  $: previous = vault?.snapshots?.find(x => x.id === compare);
  $: comparison = previous && snapshot ? compareManifests(previous,snapshot) : [];
  function compareManifests(a,b) {
    const x = new Map(a.artifacts.map(v=>[v.source_path,v]));
    const y = new Map(b.artifacts.map(v=>[v.source_path,v]));
    return [...new Set([...x.keys(),...y.keys()])].sort().map(path=>{
      const l=x.get(path), r=y.get(path);
      return {path,status:!l?'added':!r?'removed':l.sha256!==r.sha256||l.mode!==r.mode?'changed':'unchanged'};
    });
  }
  let status;
  $: status = s => ({added:tr('Добавлен','Added'),removed:tr('Удалён','Removed'),changed:tr('Изменён','Changed'),unchanged:tr('Без изменений','Unchanged'),missing:tr('Отсутствует','Missing'),verified:tr('Проверено','Verified'),READY:tr('Готово к подтверждению','Ready for confirmation'),UNSUPPORTED_TARGET:tr('Только просмотр','Inspection only'),MISSING_TARGET:tr('Восстановление отсутствующего файла недоступно','Missing-file restore unavailable'),NO_CHANGE:tr('Изменений нет','No changes')})[s]||s;
  async function req(path,body) {
    controller = new AbortController();
    const timeout = setTimeout(()=>controller?.abort(),45000);
    try {
      const response = await fetch(base+path,{method:body?'POST':'GET',credentials:'same-origin',cache:'no-store',redirect:'error',signal:controller.signal,...(body?{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{})});
      if(response.status===401)markUnauthorized();
      const data=await response.json();
      if(!response.ok){const e=new Error(data.error||data.state||`HTTP ${response.status}`);e.payload=data;throw e;}
      return data;
    } finally {clearTimeout(timeout);controller=null;}
  }
  async function run(fn) {
    if(busy||!alive)return;busy=true;error='';notice='';
    try{await fn();}catch(e){if(alive){error=String(e.message||e);if(e.payload?.safety_snapshot)result=e.payload;}}finally{if(alive)busy=false;}
  }
  async function load() {
    const next=await req('config-vault');
    if(!alive)return;vault=next;plan=null;
    if(!vault.snapshots.some(x=>x.id===selected)){selected=vault.snapshots[0]?.id||'';artifact='';}
    if(!vault.snapshots.some(x=>x.id===compare))compare='';
  }
  async function capture() {
    if(!confirm(tr('Создать локальный снимок Config Vault? Старые снимки не будут удалены.','Create a local Config Vault snapshot? Existing snapshots will not be deleted.')))return;
    await run(async()=>{const x=await req('config-vault-capture',{confirm:'SNAPSHOT',reason:'Recovery Center manual snapshot',no_prune:true});if(!alive)return;await load();selected=x.snapshot.id;artifact='';notice=tr('Снимок сохранён','Snapshot saved');});
  }
  async function inspect() {plan=null;await run(async()=>{const p=await req('recovery-inspect?'+new URLSearchParams({id:selected,artifact}));if(alive)plan=p;});}
  async function restore() {
    if(!plan?.item.restore_allowed||!confirm(tr('Восстановить только выбранный файл? Перед записью будет создан защитный снимок. Службы не перезапускаются.','Restore only the selected file? A safety snapshot will be captured first. Services will not restart.')))return;
    const p=plan;plan=null;result=null;
    await run(async()=>{const out=await req('recovery-restore',{snapshot_id:p.snapshot_id,artifact_id:p.item.id,token:p.token,confirm:'RECOVERY_RESTORE_ONE'});if(!alive)return;result=out;await load();notice=tr('Файл сохранён и проверен. Активация отдельно.','File saved and verified. Activation is separate.');});
  }
  function showSafety(){if(!result?.safety_snapshot)return;selected=result.safety_snapshot;artifact='';plan=null;compare='';}
  function legacy(){return run(async()=>{const r=await req('backups');if(alive)archives=r.backups||[];});}
  onMount(()=>{void run(load)});
  onDestroy(()=>{alive=false;controller?.abort()});
</script>
<section class="recovery panel">
  <div class="head"><div><strong>{tr('Центр восстановления','Recovery Center')}</strong><small>{tr('Config Vault · локальные снимки · без автоматической активации','Config Vault · local snapshots · no automatic activation')}</small></div><div class="actions"><button class="button" disabled={busy} onclick={()=>run(load)}>{tr('Обновить','Refresh')}</button><button class="button primary" disabled={busy||!vault} onclick={capture}>{tr('Создать снимок','Capture snapshot')}</button></div></div>
  {#if error}<p role="alert" class="err">{error} · {tr('При неопределённом результате проверьте состояние. Не повторяйте запись вслепую.','If the result is uncertain, inspect state. Do not retry blindly.')}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if busy}<p role="status">{tr('Выполняется операция…','Working…')}</p>{/if}
  {#if vault}
    <div class="stats"><span>{tr('Снимков','Snapshots')}: <b>{vault.snapshots.length} / {vault.retention.max_snapshots}</b></span><span>{tr('Хранилище','Storage')}: <b>{vault.root}</b></span><span>{tr('Лимит','Limit')}: <b>{Math.round(vault.retention.max_store_bytes/1048576)} MiB</b></span></div>
    <p>{tr('Снимки содержат метаданные файлов, а не гарантированно рабочую конфигурацию. Содержимое не выгружается в браузер.','Snapshots describe saved files, not guaranteed working configurations. File contents are not sent to the browser.')}</p>
    {#if vault.snapshots.length}
      <div class="selectors"><label>{tr('Снимок','Snapshot')}<select bind:value={selected} disabled={busy} onchange={()=>{artifact='';plan=null;compare='';}}>{#each vault.snapshots as s}<option value={s.id}>{s.created_at} · {s.reason||s.id} · {s.artifacts.length}</option>{/each}</select></label><label>{tr('Сравнить с редакцией','Compare with revision')}<select bind:value={compare} disabled={busy}><option value="">—</option>{#each vault.snapshots.filter(s=>s.id!==selected) as s}<option value={s.id}>{s.created_at} · {s.id}</option>{/each}</select></label></div>
      {#if snapshot}<div class="meta"><code>{snapshot.id}</code> · {snapshot.component} · {snapshot.total_bytes} B {#if vault.state.last_working===snapshot.id}· LAST WORKING{/if}<small>{tr('Версия Admin','Admin version')}: {snapshot.version||tr('не записана','not recorded')}</small></div>
        <div class="files">{#each snapshot.artifacts as a}<label class="file"><input type="radio" name="recovery-artifact" value={a.id} bind:group={artifact} disabled={busy} onchange={()=>plan=null}/><span>{a.source_path}<small>{a.size} B · SHA256 <code>{a.sha256}</code></small></span></label>{:else}<p>{tr('Пустой снимок','Empty snapshot')}</p>{/each}</div>
        <button class="button" disabled={busy||!artifact} onclick={inspect}>{tr('Проверить файл и сравнить с текущим','Verify file and compare with current')}</button>
      {/if}
      {#if comparison.length}<details open><summary>{tr('Сравнение метаданных редакций','Revision metadata comparison')}</summary>{#each comparison as item}<div class="diff"><span>{item.path}</span><strong>{status(item.status)}</strong></div>{/each}</details>{/if}
      {#if plan}<div class="preview"><strong>{plan.item.path}</strong><div>{tr('Целостность объекта','Object integrity')}: {status(plan.item.integrity)} · {status(plan.item.status)}</div><small>{tr('Текущий SHA256','Current SHA256')}: <code>{plan.item.current_sha256||'—'}</code></small><small>{tr('SHA256 снимка','Snapshot SHA256')}: <code>{plan.item.snapshot_sha256}</code></small><div>{status(plan.item.reason)}</div>{#if plan.before && plan.after}<div>{tr('Размер журнала','Log size')}: {plan.before.max_mb} → {plan.after.max_mb} MiB</div><div>{tr('Ротация (копий)','Rotation (copies)')}: {plan.before.backups} → {plan.after.backups}</div>{/if}<button class="button" disabled={busy||!plan.item.restore_allowed} onclick={restore}>{tr('Восстановить только этот файл','Restore this file only')}</button></div>{/if}
      {#if result}<div class="preview"><strong>{result.state}</strong><small>{tr('Защитный снимок','Safety snapshot')}: {result.safety_snapshot||'—'}</small><small>{tr('Проверка записи','Readback')}: {result.readback_verified?'PASS':'NOT VERIFIED'}</small><button class="button" disabled={busy||!result.safety_snapshot} onclick={showSafety}>{tr('Открыть защитный снимок для отката','Open safety snapshot for rollback')}</button></div>{/if}
    {:else}<p>{tr('Снимков пока нет. Создайте первый вручную.','No snapshots yet. Capture the first one manually.')}</p>{/if}
  {/if}
  <p class="limit">{tr('Выборочная запись в E06.1 доступна только для существующего logging.json в стандартном режиме Entware. Связанные состояния, настройки безопасности и произвольные файлы — только просмотр метаданных.','E06.1 selective writes support only existing logging.json in standard Entware mode. Coupled state, security settings and arbitrary files are metadata-only.')}</p>
  <details><summary>{tr('Существующие архивы обслуживания','Existing maintenance archives')}</summary><button class="button" disabled={busy} onclick={legacy}>{tr('Загрузить перечень','Load inventory')}</button>{#each archives as a}<div class="diff"><span>{a.name} · {a.modified_at} · {a.size} B</span><strong>{a.valid?'VALID':'UNVERIFIED'}</strong></div>{/each}<p>{tr('Для архивов используется прежний механизм обслуживания; новых действий восстановления здесь нет.','Archives retain their existing maintenance workflow; no new restore actions here.')}</p></details>
</section>
<RecoveryExtended {locale} {snapshot} {artifact} parentBusy={busy} onRefresh={()=>run(load)} />
<style>
  .recovery{display:grid;gap:12px;padding:14px;min-width:0}.head,.actions,.stats,.diff{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.head,.diff{justify-content:space-between}.head>div:first-child,.meta,.preview,.file span{display:grid;gap:5px;min-width:0}.recovery small,.limit{color:var(--rf-muted,#8d98a4);overflow-wrap:anywhere}.selectors{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:10px}.selectors label{display:grid;gap:5px;min-width:0}.selectors select{min-width:0;width:100%}.files{display:grid;gap:4px;max-height:320px;overflow:auto}.file{display:flex;gap:8px;align-items:flex-start;padding:6px;border-bottom:1px solid var(--rf-border,#29313a);overflow-wrap:anywhere}.file input{flex:none}.preview,details{border:1px solid var(--rf-border,#29313a);border-radius:6px;padding:10px;overflow-wrap:anywhere}.preview button{justify-self:start}.diff{padding:6px 0;border-bottom:1px solid var(--rf-border,#29313a)}.err{color:var(--danger,#e66)}.stats{font-size:12px}.recovery p{margin:0}.recovery code{overflow-wrap:anywhere}
</style>
