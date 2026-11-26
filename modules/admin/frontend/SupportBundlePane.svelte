<script>
  import { onMount, onDestroy } from 'svelte';
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let locale = 'ru';
  const base = '/api/modules/admin/maintenance/';
  const sectionIDs = ['system','packages','modules','foundation','logs'];
  const moduleIDs = ['admin','dns','monitoring','network-tools','nfqws-manager','dns-router'];
  let tr; $: tr = locale === 'ru' ? ((ru,en)=>ru) : ((ru,en)=>en);
  let sectionLabels; $: sectionLabels = {system:tr('Система и ресурсы','System and resources'),packages:tr('Версии пакетов','Package versions'),modules:tr('Состояние модулей','Module health'),foundation:tr('Каталог и события','Catalogue and events'),logs:tr('Сводка журналов','Log summary')};
  let stateLabels; $: stateLabels = {ok:tr('Собрано','Collected'),partial:tr('Частично','Partial'),skipped:tr('Пропущено','Skipped')};
  let reasonLabels; $: reasonLabels = {PARTIAL:tr('Часть данных недоступна','Some data unavailable'),NOT_AVAILABLE:tr('Источник недоступен','Source unavailable'),NO_LOG:tr('Локальный журнал не найден','Local log unavailable'),TIMEOUT:tr('Тайм-аут','Timeout'),INVALID_SOURCE:tr('Неподдерживаемый источник','Unsupported source')};
  let selected = [...sectionIDs], modules = [...moduleIDs], preview = null, saved = null, bundles = [], maxFiles = 8;
  let busy = false, error = '', alive = true, controller = null;
  $: canPreview = selected.length > 0 && (!selected.includes('modules') || modules.length > 0);
  function invalidate(){ preview = null; }
  async function req(path,body){
    const c = new AbortController(); controller = c;
    const timeout = setTimeout(()=>c.abort(),25000);
    try{
      const response=await fetch(base+path,{method:body?'POST':'GET',credentials:'same-origin',cache:'no-store',redirect:'error',signal:c.signal,...(body?{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{})});
      if(response.status===401)markUnauthorized();
      const data=await response.json();if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);return data;
    }finally{clearTimeout(timeout);if(controller===c)controller=null;}
  }
  async function run(fn){if(busy||!alive)return;busy=true;error='';try{await fn();}catch(e){if(alive)error=String(e.message||e);}finally{if(alive)busy=false;}}
  async function load(){const r=await req('support-bundles');if(alive){bundles=r.bundles||[];maxFiles=r.max_files||8;}}
  function selectAll(){selected=[...sectionIDs];modules=[...moduleIDs];invalidate();}
  async function makePreview(){
    preview=null;saved=null;
    await run(async()=>{const p=await req('support-preview',{sections:selected,modules:selected.includes('modules')?modules:[],confirm:'SUPPORT_PREVIEW'});if(alive)preview=p;});
  }
  async function save(){
    const p=preview;if(!p)return;
    if(Date.now()>=Date.parse(p.expires_at)){preview=null;error=tr('Предпросмотр устарел. Соберите заново.','Preview expired. Collect again.');return;}
    if(!confirm(tr('Сохранить именно этот диагностический ZIP? Никакие данные не отправляются наружу.','Save exactly this diagnostic ZIP? No data is sent externally.')))return;
    preview=null;
    await run(async()=>{const r=await req('support-bundle',{confirm:'SUPPORT_BUNDLE',preview_token:p.token});if(!alive)return;saved=r;if(r.sha256!==p.sha256)throw new Error('ZIP_SHA256_MISMATCH');await load();});
  }
  async function remove(bundle){if(!confirm(tr('Удалить только этот диагностический архив?','Delete only this diagnostic archive?')+'\n'+bundle.name))return;await run(async()=>{await req('support-delete',{name:bundle.name,confirm:'DELETE_SUPPORT_BUNDLE'});if(!alive)return;if(saved?.bundle?.name===bundle.name)saved=null;await load();});}
  const isV2 = name => /^routerforge-support-v2-\d+\.zip$/.test(name||'');
  const downloadURL = path => base+'support-bundle/download?path='+encodeURIComponent(path);
  function reviewLegacy(event,bundle){if(!isV2(bundle.name)&&!confirm(tr('Старый архив: маскирование неизвестно. Скачивайте для локальной проверки, не отправляйте без проверки.','Legacy archive: redaction unverified. Download for local inspection; do not share without review.')))event.preventDefault();}
  onMount(()=>{void run(load)});
  onDestroy(()=>{alive=false;controller?.abort()});
</script>
<div class="support-v2">
  <div class="head"><div><strong>{tr('Диагностический архив','Support Bundle')}</strong><small>{tr('V2 · только разрешённые метаданные · локальный ZIP','V2 · allowlisted metadata only · local ZIP')}</small></div><button class="button" disabled={busy} onclick={selectAll}>{tr('Выбрать всё','Select all')}</button></div>
  <p>{tr('Сырые логи, конфигурации, адреса, имена хостов, аргументы процессов, ключи и идентификаторы не включаются. Версии и числовые показатели сохраняются.','Raw logs, configurations, addresses, hostnames, process arguments, keys and identifiers are excluded. Versions and numeric measurements are retained.')}</p>
  <div class="choices">{#each sectionIDs as id}<label><input type="checkbox" value={id} bind:group={selected} disabled={busy} onchange={invalidate}/>{sectionLabels[id]}</label>{/each}</div>
  {#if selected.includes('modules')}<div class="choices module-choices">{#each moduleIDs as id}<label><input type="checkbox" value={id} bind:group={modules} disabled={busy} onchange={invalidate}/>{id}</label>{/each}</div><small>{tr('Защищённые модули: только версии установленных пакетов, без новых runtime-запросов.','Protected modules: installed package versions only, no new runtime probes.')}</small>{/if}
  <div class="actions"><button class="button primary" disabled={busy||!canPreview} onclick={makePreview}>{tr('Собрать предпросмотр','Collect preview')}</button><button class="button" disabled={busy} onclick={()=>run(load)}>{tr('Обновить архивы','Refresh archives')}</button>{#if busy}<span role="status">{tr('Выполняется…','Working…')}</span>{/if}</div>
  {#if error}<p role="alert" class="err">{error} · {tr('При неопределённом результате сначала обновите список архивов.','If the outcome is uncertain, refresh the archive inventory first.')}</p>{/if}
  {#if preview}<section class="preview"><strong>{tr('Готовый предпросмотр ZIP','Prepared ZIP preview')}</strong><small>{preview.size} B · {tr('Действует до','Expires at')} {new Date(preview.expires_at).toLocaleTimeString()}</small><small>SHA256 <code>{preview.sha256}</code></small>
    {#each preview.manifest.sources as item}<div class="line"><div><strong>{sectionLabels[item.id]||item.id}</strong><small>{item.name||'—'} · {item.bytes} B {item.reason?' · '+(reasonLabels[item.reason]||item.reason):''}</small>{#if item.sha256}<small>SHA256 <code>{item.sha256}</code></small>{/if}</div><span>{stateLabels[item.state]||item.state}</span></div>{/each}
    <small>{tr('Дополнительно: manifest.json и README.txt. Список исключений включён в манифест.','Also includes manifest.json and README.txt. Exclusions are listed in the manifest.')}</small>
    <button class="button primary" disabled={busy||bundles.length>=maxFiles} onclick={save}>{tr('Сохранить этот ZIP','Save this ZIP')}</button><small>{tr('Один подготовленный архив на роутер, срок 2 минуты. Новый предпросмотр заменяет предыдущий.','One prepared archive per router, valid for two minutes. New previews replace the previous one.')}</small></section>{/if}
  {#if saved}<section class="preview"><strong>{tr('Архив сохранён и проверен','Archive saved and verified')}</strong><small>SHA256 <code>{saved.sha256}</code></small><a class="button" href={downloadURL(saved.bundle.path)}>{tr('Скачать ZIP','Download ZIP')}</a></section>{/if}
  <div class="head"><strong>{tr('Локальные архивы','Local archives')} · {bundles.length}/{maxFiles}</strong><small>{tr('Временное хранилище /tmp','Temporary storage /tmp')}</small></div>
  {#if bundles.length>=maxFiles}<p class="err">{tr('Лимит достигнут. Удалите выбранный архив вручную. Старые копии не вытесняются автоматически.','Retention limit reached. Delete a selected archive explicitly. Old archives are not automatically evicted.')}</p>{/if}
  {#each bundles as bundle (bundle.name)}<div class="line"><div><strong>{bundle.name}</strong><small>{bundle.size} B · {new Date(bundle.modified_at).toLocaleString()} · {isV2(bundle.name)?'V2':tr('LEGACY / маскирование не подтверждено','LEGACY / redaction unverified')}</small></div><div class="actions"><a class="button" href={downloadURL(bundle.path)} onclick={(event)=>reviewLegacy(event,bundle)}>{tr('Скачать','Download')}</a><button class="button" disabled={busy} onclick={()=>remove(bundle)}>{tr('Удалить','Delete')}</button></div></div>{:else}<small>{tr('Архивов пока нет.','No archives yet.')}</small>{/each}
  <small>{tr('Перед передачей архива третьим лицам проверьте его содержимое. Сводка событий не доказывает причину сбоя.','Review the archive before sharing. Event summaries do not establish the cause of an incident.')}</small>
</div>
<style>
  .support-v2{display:grid;gap:12px;padding:12px;min-width:0}.head,.actions,.line,.choices{display:flex;gap:8px;flex-wrap:wrap;align-items:center}.head,.line{justify-content:space-between}.head>div,.line>div:first-child,.preview{display:grid;gap:5px;min-width:0}.choices label{display:flex;gap:5px;align-items:center;padding:6px 8px;border:1px solid var(--rf-border,#29313a);border-radius:6px}.module-choices{padding-left:10px}.preview{border:1px solid var(--rf-border,#29313a);border-radius:6px;padding:12px}.preview .button{justify-self:start}.line{border-bottom:1px solid var(--rf-border,#29313a);padding:7px 0}.support-v2 p{margin:0}.support-v2 small{color:var(--rf-muted,#8d98a4);overflow-wrap:anywhere}.support-v2 code,.line strong{overflow-wrap:anywhere}.err{color:var(--danger,#e66)}
</style>
