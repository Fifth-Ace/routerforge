<script>
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let catalogueETag = '';
  export let groups = [];
  export let locale = 'ru';
  export let disabled = false;
  let rows = [], loaded = false, busy = false, error = '', notice = '', target = '', groupId = '', operation = 'attach', plan = null;
  let oldETag = '', tr, existing;
  $: tr = locale === 'en' ? (ru,en)=>en : (ru,en)=>ru;
  $: if (catalogueETag !== oldETag) { oldETag = catalogueETag; plan = null; loaded = false; rows = []; }
  $: existing = rows.find(row=>row.target===target);
  const base='/api/platform/foundation/bindings';
  async function request(path, body=null, commit=false) {
    const res=await fetch(base+path,{cache:'no-store',credentials:'same-origin',redirect:'error',...(body?{method:'POST',headers:{'Content-Type':'application/json','X-RF-Foundation':'1',...(commit?{'X-RF-Confirm':'BINDING_COMMIT'}:{})},body:JSON.stringify(body)}:{})});
    if(res.status===401) markUnauthorized();
    const data=await res.json();if(!res.ok)throw Error(data.error||`HTTP ${res.status}`);return data;
  }
  async function run(fn){if(busy||disabled)return;busy=true;error='';notice='';try{await fn()}catch(e){plan=null;error=String(e.message||e)}finally{busy=false}}
  async function readRows(){const data=await request('');if(data.schemaVersion!==1||!Array.isArray(data.bindings)||data.catalogueETag!==catalogueETag)throw Error('REVISION_CONFLICT');rows=data.bindings;loaded=true;}
  function selectRow(row){target=row.target;groupId=row.groupId;operation='inspect';plan=null;error='';notice='';}
  function clear(){plan=null;error='';notice='';}
  function reset(){target='';groupId='';operation='attach';clear();}
  function params(){return {operation,target:target.trim(),groupId,catalogueETag}}
  async function inspect(){plan=null;const body=params();await run(async()=>{const data=await request('/preview',body);if(data.catalogueETag!==catalogueETag||data.target!==body.target||data.operation!==body.operation||data.groupId!==body.groupId||!/^[a-f0-9]{64}$/.test(data.token))throw Error('INVALID_PREVIEW');plan=data;});}
  async function commit(){if(!plan?.canCommit||disabled||busy)return;const p=plan;const warning=p.operation==='sync'||p.operation==='rollback'?tr('Сохранить показанный состав в нативный список? Это может повлиять на работающий NFQWS. DNS-сервис не перезапускается.','Save the shown membership to the native list? This may affect a running NFQWS instance. The DNS service is not restarted.'):p.operation==='detach'?tr('Удалить только привязку и её журнал восстановления? Сам список останется.','Remove only this binding and its recovery journal? The list stays intact.'):p.operation==='rebase'?tr('Принять текущую редакцию списка и удалить предыдущую точку rollback?','Accept current list revision and discard the previous rollback checkpoint?'):p.operation==='reconcile'?tr('Подтвердить наблюдаемое состояние? Перед подтверждением убедитесь, что предыдущая операция завершилась.','Confirm observed state? Make sure the previous operation has finished first.'):tr('Подтвердить изменение метаданных привязки?','Confirm binding metadata change?');if(!confirm(warning))return;
    await run(async()=>{await request('/commit',{operation:p.operation,target:p.target,groupId:p.groupId,catalogueETag:p.catalogueETag,token:p.token},true);plan=null;await readRows();notice=tr('Изменение подтверждено. Состояние активных правил не проверялось.','Change confirmed. Active rule state was not verified.');if(p.operation==='detach')reset();else operation='inspect';});}
  function statusLabel(status){switch(status){case 'saved_in_sync':return tr('Сохранённый список совпадает','Saved list matches');case 'needs_sync':return tr('Требуется синхронизация','Synchronization required');case 'native_conflict':return tr('Нативный список изменён вне привязки','Native list changed outside binding');case 'ready':return tr('Готово к сохранению','Ready to save');case 'pending_after_observed':return tr('Наблюдается ожидаемая новая редакция','Expected new revision observed');case 'pending_before_observed':return tr('Наблюдается исходная редакция','Original revision observed');case 'pending_conflict':return tr('Неизвестная редакция после операции','Unknown revision after operation');case 'rebase_metadata_only':return tr('Принять текущую редакцию без записи списка','Accept current revision without list write');case 'detach_metadata_only':return tr('Удаление только метаданных привязки','Remove binding metadata only');default:return status}}
</script>
<section class="panel rf-binding-panel">
  <div class="panel-head"><div><strong>{tr('Постоянные привязки списков','Persistent list bindings')}</strong><span>{tr('Изменения применяются только после подтверждения','Changes require explicit confirmation')}</span></div></div>
  <div class="rf-binding-body">
  <details class="rf-binding-help"><summary>{tr('Требования к спискам и синхронизации','List and synchronization requirements')}</summary><p>{tr('Привязка хранит контрольную редакцию существующего списка. Изменения каталога не применяются автоматически. Создайте отдельный список в родном модуле: dns.rf-binding-name или nfqws.rf-binding-name.list. DNS: локальный список без автообновления; NFQWS: непустой отсортированный список доменов, по одному на строку, без комментариев и дублей.','A binding tracks the checkpoint of an existing list. Catalogue changes never apply automatically. Create a dedicated list in its native module: dns.rf-binding-name or nfqws.rf-binding-name.list. DNS: local list without auto-updates; NFQWS: nonempty sorted domain list, one per line, without comments or duplicates.')}</p></details>
  <div class="rf-bind-actions"><button type="button" class="button" disabled={busy||disabled||!catalogueETag} onclick={()=>run(readRows)}>{tr('Обновить привязки','Refresh bindings')}</button><button type="button" class="button primary" disabled={busy||disabled} onclick={reset}>{tr('Новая привязка','New binding')}</button></div>
  {#if loaded}
    {#each rows as row (row.target)}<div class="rf-bind-row"><div><strong>{row.target}</strong><small>{row.groupId} · {row.count} · SHA {row.sha256.slice(0,12)} · {row.pending?tr('Незавершённая операция','Pending operation'):tr('Контрольная редакция','Checkpoint')}</small></div><button type="button" class="button" disabled={busy||disabled} onclick={()=>selectRow(row)}>{tr('Выбрать','Select')}</button></div>{:else}<div class="empty">{tr('Постоянных привязок пока нет','No persistent bindings yet')}</div>{/each}
    <div class="rf-bind-fields">
      <label>{tr('Целевой список','Target list')}<input bind:value={target} maxlength="96" disabled={busy||disabled||operation!=='attach'} placeholder="dns.rf-binding-name" oninput={clear}/></label>
      <label>{tr('Сохранённая группа','Saved group')}<select bind:value={groupId} disabled={busy||disabled||operation!=='attach'} onchange={clear}><option value="">—</option>{#each groups.filter(g=>g.type==='domain-group') as g (g.id)}<option value={g.id}>{g.name} ({g.id})</option>{/each}</select></label>
      <label>{tr('Операция','Operation')}<select bind:value={operation} disabled={busy||disabled} onchange={clear}><option value="attach">{tr('Создать привязку','Attach')}</option>{#if existing}<option value="inspect">{tr('Проверить','Inspect')}</option><option value="sync">{tr('Синхронизировать сохранённый список','Sync saved list')}</option>{#if existing.rollbackAvailable}<option value="rollback">{tr('Вернуть предыдущий состав','Restore previous membership')}</option>{/if}<option value="rebase">{tr('Принять ручные изменения','Accept manual changes')}</option>{#if existing.pending}<option value="reconcile">{tr('Проверить незавершённую операцию','Reconcile pending operation')}</option>{/if}<option value="detach">{tr('Отвязать (не менять список)','Detach (keep list)')}</option>{/if}</select></label>
    </div>
    <button type="button" class="button primary" disabled={busy||disabled||!target.trim()||!groupId||!catalogueETag} onclick={inspect}>{tr('Предпросмотр / проверка','Preview / inspect')}</button>
  {/if}
  {#if busy}<p role="status">{tr('Выполняется операция…','Operation in progress…')}</p>{/if}
  {#if error}<p role="alert">{error} · {tr('Обновите привязки и проверьте состояние. Не повторяйте сохранение вслепую.','Refresh bindings and inspect state. Do not blindly repeat a save.')}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if plan}
    <p><strong>{statusLabel(plan.status)}</strong> · {tr('Активные правила не проверены','Active rules not verified')}</p>
    {#if plan.runtime==='SAVED_REVISION_ACTIVE'}<p>{tr('Служба DNS сообщила активную сохранённую редакцию (без проверки трафика).','DNS service reported the saved revision active (traffic not tested).')}</p>{:else if plan.runtime==='OTHER_REVISION_ACTIVE'}<p>{tr('DNS-служба использует другую редакцию.','DNS service uses another revision.')}</p>{:else if plan.runtime==='INACTIVE'}<p>{tr('DNS-служба остановлена.','DNS service is inactive.')}</p>{/if}
    {#if plan.before}<p>{tr('До','Before')}: {plan.before.values.length} · {plan.before.sha256.slice(0,12)}</p>{/if}
    {#if plan.after}<p>{tr('После','After')}: {plan.after.values.length}</p>{/if}
    {#if plan.added.length||plan.removed.length}<details open><summary>{tr('Изменения состава','Membership changes')} +{plan.added.length} / −{plan.removed.length}</summary><pre>{plan.added.map(v=>'+ '+v).concat(plan.removed.map(v=>'− '+v)).join('\n')}</pre></details>{/if}
    {#if plan.canCommit}<button type="button" class="button" disabled={busy||disabled} onclick={commit}>{tr('Подтвердить именно этот план','Confirm this exact plan')}</button>{/if}
  {/if}
  </div>
</section>
<style>
 .rf-binding-panel{margin:0;min-width:0;overflow-wrap:anywhere}
 .rf-binding-panel .panel-head {min-height:40px;padding:10px 12px;flex-wrap:wrap}
 .rf-binding-body {padding:12px;display:grid;gap:10px;min-width:0}
 .rf-binding-body p {margin:0;line-height:1.5}
 .rf-binding-help {border:1px solid var(--border);border-radius:5px;padding:8px 10px;color:var(--muted);font-size:12px}
 .rf-binding-help summary {cursor:pointer;font-weight:600}
 .rf-binding-help p {margin-top:8px}
 .rf-bind-actions,.rf-bind-row{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
 .rf-bind-row{justify-content:space-between;border-bottom:1px solid var(--border);padding:8px 0}
 .rf-bind-row>div{min-width:0;flex:1}
 .rf-bind-row small{display:block;color:var(--muted);margin-top:4px}
 .rf-bind-fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,200px),1fr));gap:10px}
 .rf-bind-fields label{display:grid;gap:5px;min-width:0;color:var(--muted);font-size:12px}
 .rf-bind-fields input,.rf-bind-fields select{width:100%;max-width:none;min-width:0;min-height:30px;height:auto;padding:7px 9px;border:1px solid var(--border);border-radius:5px;background:var(--surface-2);color:var(--text);font:inherit}
 .rf-binding-panel pre{max-height:240px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;background:var(--bg);padding:8px;border-radius:5px;border:1px solid var(--border)}
 .rf-binding-panel button:disabled{opacity:.5;cursor:not-allowed}
 .rf-binding-panel button:focus-visible,.rf-binding-panel summary:focus-visible,.rf-binding-panel input:focus-visible,.rf-binding-panel select:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
 .rf-binding-panel [role=alert]{color:var(--bad)}
</style>
